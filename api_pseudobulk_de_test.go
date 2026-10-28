package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/pseudobulkde"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestPseudobulkDETruthReplayExports(t *testing.T) {
	a, h, req, dir := pseudobulkFixtureFor(t, "pseudobulk-de-study")
	start := func(path string, v any) string {
		t.Helper()
		rec := postJSON(t, h, path, string(mustJSON(v)))
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body)
		}
		var started struct{ Artifact, Job string }
		if e := json.Unmarshal(rec.Body.Bytes(), &started); e != nil {
			t.Fatal(e)
		}
		if started.Job != "" {
			waitForJob(t, a, started.Job, 2*time.Minute)
		}
		return started.Artifact
	}
	parent := start("/api/pseudobulk", req)
	options := pseudobulkde.Options{Type: "Artificial T cells", Time: "baseline", Reference: "1", Comparison: ">1"}
	requestBody := pseudobulkDERequest{Pseudobulk: parent, Options: options}
	preview := postJSON(t, h, "/api/pseudobulk-de", string(mustJSON(map[string]any{"pseudobulk": parent, "options": options, "start": false})))
	if preview.Code != 200 {
		t.Fatal(preview.Code, preview.Body)
	}
	var planned struct {
		Artifact string
		Ready    bool
	}
	json.Unmarshal(preview.Body.Bytes(), &planned)
	if planned.Ready {
		t.Fatal("preview ran a fit")
	}
	id := start("/api/pseudobulk-de", requestBody)
	if id != planned.Artifact {
		t.Fatal("preview identity")
	}
	var doc pseudobulkDEDocument
	m, _, e := a.readCheckedResult(id, pseudobulkDEKind, &doc)
	if e != nil {
		t.Fatal(e)
	}
	r := doc.Analysis
	if len(r.Observations) != 56 || r.Family != 38 || r.Tested < 35 || r.Design.ResidualDF != 27 {
		t.Fatal("accounting", len(r.Observations), r.Family, r.Tested, r.Design.ResidualDF)
	}
	f, e := os.Open(filepath.Join(dir, "truth/counts.tsv"))
	if e != nil {
		t.Fatal(e)
	}
	rd := csv.NewReader(f)
	rd.Comma = '\t'
	truth, e := rd.ReadAll()
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	counts := map[[3]string]uint64{}
	for _, v := range truth[1:] {
		count, e := strconv.ParseUint(v[3], 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		counts[[3]string{v[0], v[1], v[2]}] = count
	}
	for _, row := range r.Rows {
		for j, v := range row.Counts {
			o := r.Observations[j]
			if counts[[3]string{o.Sample, o.Bin, row.Feature.ID}] != v {
				t.Fatal("generating count differs", o, row.Feature.ID)
			}
		}
	}
	for j := 0; j < 4; j++ {
		if r.Rows[j].P != r.Rows[j].P || j < 2 && r.Rows[j].Log2Fold <= 0 || j >= 2 && r.Rows[j].Log2Fold >= 0 {
			t.Fatal("positive control direction", j)
		}
	}
	t.Logf("positive example: %d/%d fits; first four log2 effects %.3f %.3f %.3f %.3f", r.Tested, r.Family, r.Rows[0].Log2Fold, r.Rows[1].Log2Fold, r.Rows[2].Log2Fold, r.Rows[3].Log2Fold)
	if other := start("/api/pseudobulk-de", requestBody); other != id {
		t.Fatal("reuse identity")
	}
	for _, bad := range []string{`{}`, `{"extra":true}`, string(mustJSON(requestBody)) + ` {}`, fmt.Sprintf(`{"pseudobulk":%q,"options":{"type":"Artificial T cells","timepoint":"baseline","reference":"1","comparison":">1","extra":true}}`, parent)} {
		if v := postJSON(t, h, "/api/pseudobulk-de", bad); v.Code == 200 {
			t.Fatal("accepted", bad)
		}
	}
	// A new analysis of a checked raw-count parent does not need expression sources.
	if e = os.RemoveAll(dir); e != nil {
		t.Fatal(e)
	}
	if _, e = a.capturePseudobulkDE(context.Background(), requestBody); e != nil {
		t.Fatal("source-free new analysis", e)
	}
	parentPath, _ := a.artifacts.Path(parent, "pseudobulk.json")
	if e = os.WriteFile(parentPath, []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = a.capturePseudobulkDE(context.Background(), requestBody); e == nil {
		t.Fatal("accepted changed parent")
	}
	before := mustJSON(doc)
	resultPath, _ := a.artifacts.Path(id, "pseudobulk-de.json")
	if e = os.RemoveAll(filepath.Dir(resultPath)); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(parentPath); e != nil {
		t.Fatal(e)
	}
	var spec pseudobulkDESpec
	if e = json.Unmarshal(m.Lineage.Job.Spec, &spec); e != nil {
		t.Fatal(e)
	}
	work, e := a.jobs.Remake(pseudobulkDEKind, m.Lineage.Job.Spec)
	if e != nil {
		t.Fatal(e)
	}
	job, _, e := a.jobs.Submit(pseudobulkDEKind, "Replay", id+"-replay", spec, work)
	if e != nil {
		t.Fatal(e)
	}
	waitForJob(t, a, job.ID, 2*time.Minute)
	var back pseudobulkDEDocument
	if _, _, e = a.readCheckedResult(id, pseudobulkDEKind, &back); e != nil || !bytes.Equal(before, mustJSON(back)) {
		t.Fatal("parent/source-free replay differs", e)
	}
	for _, accept := range []bool{false, true} {
		path := "/api/pseudobulk-de/" + id + "/download"
		if accept {
			path += "?accept=1"
		}
		v := request(t, h, http.MethodGet, path, nil, nil)
		if !accept {
			if v.Code != 409 {
				t.Fatal("terms", v.Code)
			}
			continue
		}
		if v.Code != 200 {
			t.Fatal("export", v.Code, v.Body)
		}
		z, e := zip.NewReader(bytes.NewReader(v.Body.Bytes()), int64(v.Body.Len()))
		if e != nil {
			t.Fatal(e)
		}
		found := map[string]bool{}
		for _, f := range z.File {
			found[f.Name] = true
			if f.Name == "genes.csv" || f.Name == "counts.csv" || f.Name == "design.csv" {
				stream, _ := f.Open()
				raw, _ := io.ReadAll(stream)
				stream.Close()
				rows, e := csv.NewReader(bytes.NewReader(raw)).ReadAll()
				want := 41
				if f.Name == "design.csv" {
					want = 57
				}
				if e != nil || len(rows) != want {
					t.Fatal("incomplete CSV", f.Name, len(rows), e)
				}
			}
		}
		for _, name := range []string{"pseudobulk-de.json", "manifest.json", "parent-manifest.json", "raw-evidence.json", "genes.csv", "counts.csv", "design.csv", "NOTICE.txt"} {
			if !found[name] {
				t.Fatal("missing", name)
			}
		}
	}
	if e = os.WriteFile(resultPath, []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if v := request(t, h, http.MethodGet, "/api/pseudobulk-de/"+id, nil, nil); v.Code != 409 {
		t.Fatal("checksum", v.Code)
	}
}

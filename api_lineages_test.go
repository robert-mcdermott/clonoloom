package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/examples"
)

func TestLineageAnalysisIdentityPagesAndExport(t *testing.T) {
	a, h := testApp(t)
	// A generated reference still has its actual bytes pinned by prepare/work.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, simulatedGermlines, "reference"), 0755)
	reference := filepath.Join(dir, simulatedGermlines, "reference/germlines.fasta")
	fasta := ">IGHV1-1*01\nAAA\n>IGHJ1*01\nGGG\n"
	os.WriteFile(reference, []byte(fasta), 0600)
	a.examples = &examples.Library{Dir: dir, Generated: []examples.Generated{{Source: examples.Source{Name: simulatedGermlines, License: "Artificial"}, Marker: "reference/germlines.fasta", Files: func(string) [][]string { return nil }}}}
	path := filepath.Join(t.TempDir(), "input.tsv")
	os.WriteFile(path, []byte("sequence_id\tlocus\tv_call\tj_call\tjunction\tproductive\tsequence\trev_comp\tv_cigar\tj_cigar\n"+"a\tIGH\tIGHV1-1*01\tIGHJ1*01\tAAACCCGGG\tT\tAAACCCGGG\tF\t3M6S\t6S3M\n"+"b\tIGH\tIGHV1-1*01\tIGHJ1*01\tAAACCCGGG\tT\tAAACCCGGG\tF\t\t\n"), 0600)
	var ds datasetSummary
	json.Unmarshal(postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`).Body.Bytes(), &ds)
	var family struct{ Job, Artifact string }
	json.Unmarshal(postJSON(t, h, "/api/families", `{"dataset":`+jsonString(ds.ID)+`,"families":{"scope":"sample","threshold":0}}`).Body.Bytes(), &family)
	waitForJob(t, a, family.Job, 10*time.Second)
	body := `{"dataset":` + jsonString(ds.ID) + `,"germlines":{"families":` + jsonString(family.Artifact) + `,"reference":` + jsonString(simulatedGermlines) + `}}`
	var job struct {
		Job, Artifact string
		Ready         bool
	}
	started := postJSON(t, h, "/api/germlines", body)
	json.Unmarshal(started.Body.Bytes(), &job)
	if job.Job == "" {
		t.Fatal(started.Body)
	}
	waitForJob(t, a, job.Job, 10*time.Second)
	var result struct {
		Result   germlineDocument
		Artifact artifacts.Manifest
	}
	json.Unmarshal(request2(t, h, "/api/germlines/"+job.Artifact), &result)
	if result.Result.Reconstructed != 1 || len(result.Result.Rows) != 2 || len(result.Artifact.Lineage.Parents) != 3 || result.Result.Rows[0].Family != result.Result.Families[0].Family {
		t.Fatalf("%+v", result)
	}

	body = `{"dataset":` + jsonString(ds.ID) + `,"lineages":{"germlines":` + jsonString(job.Artifact) + `}}`
	var startedTree struct {
		Job, Artifact string
		Ready         bool
	}
	response := postJSON(t, h, "/api/lineages", body)
	if err := json.Unmarshal(response.Body.Bytes(), &startedTree); err != nil || startedTree.Job == "" {
		t.Fatal(response.Body, err)
	}
	waitForJob(t, a, startedTree.Job, 10*time.Second)
	var tree struct {
		Result   lineageDocument
		Artifact artifacts.Manifest
	}
	json.Unmarshal(request2(t, h, "/api/lineages/"+startedTree.Artifact), &tree)
	if tree.Result.Built != 1 || len(tree.Result.Members) != 2 || len(tree.Artifact.Lineage.Parents) != 2 || tree.Result.Options.GermlineSHA256 == "" {
		t.Fatal(tree)
	}
	if tree.Result.Trees[0].Sites != 6 || tree.Result.Trees[0].Score != 0 || len(tree.Result.Trees[0].Nodes) != 1 {
		t.Fatal(tree.Result.Trees)
	}
	for _, kind := range []string{"trees", "rows"} {
		var page struct {
			Rows   []json.RawMessage
			Result lineageDocument
			Count  int
		}
		json.Unmarshal(request2(t, h, "/api/lineages/"+startedTree.Artifact+"/page?kind="+kind+"&limit=1"), &page)
		if len(page.Rows) != 1 || page.Result.Trees != nil || page.Result.Members != nil {
			t.Fatal(page)
		}
		if kind == "trees" && bytes.Contains(page.Rows[0], []byte(`"nodes"`)) {
			t.Fatal("summary leaked full tree")
		}
	}
	detail := request2(t, h, "/api/lineages/"+startedTree.Artifact+"/page?kind=detail&tree="+tree.Result.Trees[0].ID)
	if !bytes.Contains(detail, []byte(`"nodes"`)) {
		t.Fatal(string(detail))
	}
	for _, query := range []string{"limit=201", "offset=-1", "kind=bad", "offset=x"} {
		if r := request(t, h, http.MethodGet, "/api/lineages/"+startedTree.Artifact+"/page?"+query, nil, nil); r.Code != 400 {
			t.Fatal(r.Code)
		}
	}
	var filtered struct{ Count int }
	json.Unmarshal(request2(t, h, "/api/lineages/"+startedTree.Artifact+"/page?kind=rows&node=absent"), &filtered)
	if filtered.Count != 0 {
		t.Fatal(filtered)
	}
	exported := request(t, h, http.MethodGet, "/api/export?what=result&artifact="+startedTree.Artifact, nil, nil)
	if exported.Code != 200 {
		t.Fatal(exported.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(exported.Body.Bytes()), int64(exported.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range z.File {
		seen[f.Name] = true
		opened, _ := f.Open()
		raw, _ := io.ReadAll(opened)
		opened.Close()
		if len(raw) == 0 {
			t.Fatal("empty export", f.Name)
		}
		if f.Name == "panel.fasta" && !bytes.Contains(raw, []byte("AAAGGG")) {
			t.Fatal(string(raw))
		}
		if f.Name == "NOTICE.txt" && !bytes.Contains(raw, []byte("not bootstrap")) {
			t.Fatal(string(raw))
		}
	}
	for _, name := range []string{"trees.csv", "nodes.csv", "members.csv", "panel.fasta", "trees.nwk", "lineages.json", "manifest.json", "NOTICE.txt"} {
		if !seen[name] {
			t.Fatal("missing", name)
		}
	}
	changed := strings.TrimSuffix(body, "}") + `,"filter":{"productiveOnly":true},"definition":{"resolution":"gene"},"subjects":{"x":"y"},"options":{"seed":999}}`
	var cached struct {
		Artifact string
		Ready    bool
	}
	json.Unmarshal(postJSON(t, h, "/api/lineages", changed).Body.Bytes(), &cached)
	if cached.Artifact != startedTree.Artifact || !cached.Ready {
		t.Fatal(cached)
	}
	name := tree.Result.Samples[0]
	annotated := strings.TrimSuffix(body, "}") + `,"start":false,"study":[{"name":` + jsonString(name) + `,"timepoint":" day 14 ","tissue":"blood","subject":"ignored"}]}`
	var fresh struct {
		Artifact string
		Ready    bool
	}
	json.Unmarshal(postJSON(t, h, "/api/lineages", annotated).Body.Bytes(), &fresh)
	if fresh.Artifact == "" || fresh.Artifact == cached.Artifact || fresh.Ready {
		t.Fatal(fresh)
	}
	annotated = strings.ReplaceAll(annotated, `"subject":"ignored"`, `"subject":"different"`)
	json.Unmarshal(postJSON(t, h, "/api/lineages", annotated).Body.Bytes(), &cached)
	if cached.Artifact != fresh.Artifact {
		t.Fatal("irrelevant metadata changed identity")
	}
	// Run the annotated identity and verify trimmed values reach source rows and nodes.
	var annotatedJob struct{ Job, Artifact string }
	json.Unmarshal(postJSON(t, h, "/api/lineages", strings.Replace(annotated, `"start":false`, `"start":true`, 1)).Body.Bytes(), &annotatedJob)
	if annotatedJob.Job == "" {
		t.Fatal("annotated job missing")
	}
	waitForJob(t, a, annotatedJob.Job, 10*time.Second)
	var pinned struct{ Result lineageDocument }
	json.Unmarshal(request2(t, h, "/api/lineages/"+annotatedJob.Artifact), &pinned)
	for _, m := range pinned.Result.Members {
		if m.Timepoint != "day 14" || m.Tissue != "blood" {
			t.Fatal(m)
		}
	}
	if pinned.Result.Trees[0].Nodes[0].Annotations[0].Timepoint != "day 14" {
		t.Fatal(pinned.Result.Trees)
	}
	var historical struct{ Result lineageDocument }
	json.Unmarshal(request2(t, h, "/api/lineages/"+startedTree.Artifact), &historical)
	if historical.Result.Members[0].Timepoint != "" {
		t.Fatal("old metadata changed")
	}
	for _, study := range []string{`[{"name":"unknown"}]`, `[{"name":` + jsonString(name) + `},{"name":` + jsonString(name) + `}]`} {
		r := postJSON(t, h, "/api/lineages", strings.TrimSuffix(body, "}")+`,"study":`+study+`}`)
		if r.Code != 400 {
			t.Fatal(r.Code, r.Body)
		}
	}
	os.Remove(reference)
	parentPath, _ := a.artifacts.Path(job.Artifact, "germlines.json")
	original, _ := os.ReadFile(parentPath)
	os.WriteFile(parentPath, append(original, ' '), 0600)
	if r := postJSON(t, h, "/api/lineages", body); r.Code != 400 || !strings.Contains(r.Body.String(), "checksum") {
		t.Fatal(r.Code, r.Body)
	}
	if r := request(t, h, http.MethodGet, "/api/lineages/"+startedTree.Artifact+"/page", nil, nil); r.Code != 200 {
		t.Fatal(r.Body)
	}
	spec := analysisSpec{Dataset: ds.ID, Lineages: &tree.Result.Options}
	if err := a.lineageWork(lineageAnalysis, spec)(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}
}

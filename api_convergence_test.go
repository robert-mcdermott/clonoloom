package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/convergence"
	"clonoloom/internal/motifs"
	"clonoloom/internal/simulate"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func convergenceFixture(t *testing.T) (*app, http.Handler, convergenceRequest, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "study")
	sum, err := simulate.Run(context.Background(), simulate.Presets["convergence-study"], dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, f := range sum.Files {
		if !strings.Contains(f, "/") && (strings.HasSuffix(f, ".tsv") || strings.HasSuffix(f, ".airr.json")) {
			paths = append(paths, filepath.Join(dir, f))
		}
	}
	a, h := testApp(t)
	resp := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": paths})))
	var ds datasetSummary
	if resp.Code != 200 || json.Unmarshal(resp.Body.Bytes(), &ds) != nil {
		t.Fatal(resp.Code, resp.Body)
	}
	dataset, _ := a.datasets.get(ds.ID)
	dataset.Credits = []credit{{Title: "Convergence fixture", License: "Fixture terms", LicenseClass: "terms", Citation: "Fixture source"}}
	var subjects []convergence.Subject
	raw, _ := os.ReadFile(filepath.Join(dir, "truth/subjects.json"))
	if err = json.Unmarshal(raw, &subjects); err != nil {
		t.Fatal(err)
	}
	var arms map[string][]string
	raw, _ = os.ReadFile(filepath.Join(dir, "truth/arms.json"))
	if err = json.Unmarshal(raw, &arms); err != nil {
		t.Fatal(err)
	}
	body := motifsRequest{Dataset: ds.ID, Definition: clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}, Query: arms["query"], Background: arms["background"], Subjects: map[string]string{}, Options: motifs.Options{MinK: 4, MaxK: 4}}
	for _, s := range subjects {
		for _, sample := range s.Samples {
			body.Subjects[sample] = s.ID
		}
	}
	resp = postJSON(t, h, "/api/motifs", string(mustJSON(body)))
	var start struct{ Artifact, Job string }
	if resp.Code != 200 || json.Unmarshal(resp.Body.Bytes(), &start) != nil {
		t.Fatal(resp.Code, resp.Body)
	}
	waitForJob(t, a, start.Job, 120*time.Second)
	var p motifsDocument
	if _, _, err = a.readCheckedResult(start.Artifact, motifsKind, &p); err != nil {
		t.Fatal(err)
	}
	req := convergenceRequest{Motifs: start.Artifact, Selection: convergence.Selection{Species: "human", Subjects: subjects, Samples: append(append([]string{}, arms["query"]...), arms["background"]...), Groups: []string{}, Alleles: []string{"HLA-A*02:01", "HLA-B*07:02", "HLA-DRB1*15:01"}}}
	for _, g := range p.Result.Groups {
		if g.Kind == "local" && g.Motif == "QGHT" && g.Length == 17 && g.Start == 5 || g.Kind == "global" && g.Allowed == "ILMV" {
			req.Selection.Groups = append(req.Selection.Groups, g.ID)
		}
	}
	if len(req.Selection.Groups) != 2 {
		t.Fatal("planted groups", len(req.Selection.Groups))
	}
	return a, h, req, dir
}
func TestConvergenceTruthReplayAndExports(t *testing.T) {
	a, h, req, dir := convergenceFixture(t)
	s, err := a.captureConvergence(req)
	if err != nil {
		t.Fatal(err)
	}
	r, err := convergence.Compute(context.Background(), s.Input)
	if err != nil {
		t.Fatal(err)
	}
	if r.ObservedSubjects != 32 || r.Family != 6 {
		t.Fatal(r.ObservedSubjects, r.Family)
	}
	// Independently aggregate literal planted truth, by subject, never sample count.
	raw, _ := os.ReadFile(filepath.Join(dir, "truth/receptors.tsv"))
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	truth := map[string]map[string]bool{"local QGHT": {}, "global ILMV": {}}
	for _, row := range rows[1:] {
		if set := truth[row[6]]; set != nil {
			set[row[1]] = true
		}
	}
	for _, g := range r.Recurrence {
		key := "local QGHT"
		if strings.Contains(g.Label, "global") {
			key = "global ILMV"
		}
		if len(g.Subjects) != len(truth[key]) {
			t.Fatal("subject recurrence", g)
		}
		for _, id := range g.Subjects {
			if !truth[key][id] {
				t.Fatal(id)
			}
		}
	}
	positive := false
	for _, x := range r.Associations {
		for _, g := range r.Recurrence {
			if g.ID == x.Group && strings.Contains(g.Label, "QGHT") && x.Allele == "HLA-A*02:01" {
				if x.N != 28 || x.Carriers != 20 || x.GroupSubjects != 18 || x.Shared != 17 || x.Excluded != 4 || float64(x.Q) > .05 {
					t.Fatal("positive control", x)
				}
				positive = true
				t.Logf("planted association p=%g BY=%g; %d/%d carriers, %d/%d non-carriers", x.P, x.Q, x.Carrier.Matches, x.Carrier.Total, x.Noncarrier.Matches, x.Noncarrier.Total)
			}
		}
	}
	if !positive {
		t.Fatal("missing positive control")
	}
	resp := postJSON(t, h, "/api/convergence", string(mustJSON(req)))
	var start struct {
		Artifact, Job string
		Ready         bool
	}
	if resp.Code != 200 || json.Unmarshal(resp.Body.Bytes(), &start) != nil {
		t.Fatal(resp.Code, resp.Body)
	}
	waitForJob(t, a, start.Job, 60*time.Second)
	var doc convergenceDocument
	m, _, err := a.readCheckedResult(start.Artifact, convergenceKind, &doc)
	if err != nil {
		t.Fatal(err)
	}
	resp = postJSON(t, h, "/api/convergence", string(mustJSON(req)))
	if resp.Code != 200 || !strings.Contains(resp.Body.String(), `"ready":true`) {
		t.Fatal("ready reuse", resp.Body)
	}
	// Parent tampering is refused on new capture; replay uses its recorded snapshot.
	parentPath, _ := a.artifacts.Path(req.Motifs, "motifs.json")
	parentBytes, err := os.ReadFile(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(parentPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.captureConvergence(req); err == nil {
		t.Fatal("tampered parent accepted")
	}
	if err = os.WriteFile(parentPath, parentBytes, 0600); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*convergenceRequest){func(r *convergenceRequest) { r.Selection.Groups = []string{"unknown"} }, func(r *convergenceRequest) { r.Selection.Samples = append(r.Selection.Samples, "unknown") }, func(r *convergenceRequest) { r.Selection.Species = "mouse" }} {
		var changed convergenceRequest
		json.Unmarshal(mustJSON(req), &changed)
		mutate(&changed)
		if _, err = a.captureConvergence(changed); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if rr := postJSON(t, h, "/api/convergence", `{"unknown":true}`); rr.Code != 400 {
		t.Fatal(rr.Code)
	}
	if err = os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	// Recreate through the registered durable handler with both parent and sources absent.
	if err = os.RemoveAll(filepath.Dir(parentPath)); err != nil {
		t.Fatal(err)
	}
	resultPath, _ := a.artifacts.Path(m.ID, "convergence.json")
	if err = os.RemoveAll(filepath.Dir(resultPath)); err != nil {
		t.Fatal(err)
	}
	work, err := a.jobs.Remake(convergenceKind, m.Lineage.Job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	var recorded convergenceSpec
	if err = json.Unmarshal(m.Lineage.Job.Spec, &recorded); err != nil {
		t.Fatal(err)
	}
	j, _, err := a.jobs.Submit(convergenceKind, "Replay convergence", m.ID, recorded, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, 60*time.Second)
	var back convergenceDocument
	if _, _, err = a.readCheckedResult(m.ID, convergenceKind, &back); err != nil || !bytes.Equal(mustJSON(doc), mustJSON(back)) {
		t.Fatal("replay drift", err)
	}
	response := request(t, h, http.MethodGet, "/api/convergence/"+m.ID+"/download", nil, nil)
	if response.Code != 409 {
		t.Fatal("provider terms", response.Code)
	}
	response = request(t, h, http.MethodGet, "/api/convergence/"+m.ID+"/download?accept=1", nil, nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		rd, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		files[f.Name], e = io.ReadAll(rd)
		rd.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"convergence.json", "manifest.json", "subjects.json", "associations.csv", "recurrence.csv", "NOTICE.txt"} {
		if len(files[name]) == 0 {
			t.Fatal(name)
		}
	}
	table, err := csv.NewReader(bytes.NewReader(files["associations.csv"])).ReadAll()
	if err != nil || len(table) != 7 {
		t.Fatal(err, len(table))
	}
	if !strings.Contains(string(files["NOTICE.txt"]), "Fixture source") {
		t.Fatal("terms lost")
	}
	path, _ := a.artifacts.Path(m.ID, "convergence.json")
	os.WriteFile(path, []byte("{}"), 0600)
	if rr := request(t, h, http.MethodGet, "/api/convergence/"+m.ID, nil, nil); rr.Code != 409 {
		t.Fatal("checksum accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := a.convergenceWork(s)(ctx, nil); e != context.Canceled {
		t.Fatal(e)
	}
	bad := s
	bad.SHA256 = fmt.Sprint("changed")
	if bad.check() == nil {
		t.Fatal("changed capture provenance")
	}
}

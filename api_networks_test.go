package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNetworksAnalysisAndExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trb.tsv")
	if err := os.WriteFile(path, []byte(tcrdistTSV), 0600); err != nil {
		t.Fatal(err)
	}
	a, h := testApp(t)
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	var ds datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &ds)
	base := `"dataset":` + jsonString(ds.ID) + `,"definition":{"junction":"aa","v":"gene","j":"gene"}`
	body := `{` + base + `,"networks":{"metric":"tcrdist","cutoff":24,"cdr3Only":true,"drawNodes":1}}`
	started := postJSON(t, h, "/api/networks", body)
	var job struct{ Job, Artifact string }
	if err := json.Unmarshal(started.Body.Bytes(), &job); err != nil || job.Job == "" {
		t.Fatalf("%s %v", started.Body, err)
	}
	waitForJob(t, a, job.Job, 10*time.Second)
	var read struct{ Result networkDocument }
	if err := json.Unmarshal(request2(t, h, "/api/networks/"+job.Artifact), &read); err != nil {
		t.Fatal(err)
	}
	doc := read.Result
	if len(doc.Clonotypes) == 0 || len(doc.Graph.Edges) == 0 || len(doc.Drawing.Points) != 1 || len(doc.Graph.Membership) != len(doc.Clonotypes) {
		t.Fatalf("%+v", doc)
	}
	again := postJSON(t, h, "/api/networks", body)
	var cached struct {
		Ready    bool
		Artifact string
	}
	json.Unmarshal(again.Body.Bytes(), &cached)
	if !cached.Ready || cached.Artifact != job.Artifact {
		t.Fatal(again.Body)
	}
	exported := request(t, h, http.MethodGet, "/api/export?what=result&artifact="+job.Artifact, nil, nil)
	if exported.Code != 200 {
		t.Fatal(exported.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(exported.Body.Bytes()), int64(exported.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, f := range z.File {
		if filepath.Ext(f.Name) != ".csv" {
			continue
		}
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		rows, e := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if e != nil {
			t.Fatal(e)
		}
		seen[filepath.Base(f.Name)] = len(rows) - 1
	}
	if seen["clusters.csv"] != len(doc.Clonotypes) || seen["edges.csv"] != len(doc.Graph.Edges) {
		t.Fatal(seen)
	}
	for _, options := range []string{`{"metric":"bad"}`, `{"metric":"hamming","cutoff":3}`, `{"arms":"bad"}`, `{"drawNodes":2001}`, `{"maxEdges":-1}`} {
		r := postJSON(t, h, "/api/networks", `{`+base+`,"networks":`+options+`}`)
		if r.Code != 400 {
			t.Fatal(options, r.Code)
		}
	}
	spec := analysisSpec{Dataset: ds.ID, Definition: doc.Definition, Networks: &doc.Options}
	spec, _ = spec.normalized(1)
	first := spec.artifactID("net-")
	spec.Subjects = map[string]string{"sample": "person"}
	if first == spec.artifactID("net-") {
		t.Fatal("subjects absent from lineage")
	}
}

func TestNetworkDefinitionAndLoopRequirements(t *testing.T) {
	_, h := testApp(t)
	for _, body := range []string{
		`{"dataset":"test","definition":{"vendor":true},"networks":{}}`,
		`{"dataset":"test","definition":{"junction":"nt"},"networks":{"metric":"tcrdist","cdr3Only":true}}`,
		`{"dataset":"test","definition":{"v":"none"},"networks":{"sameV":true}}`,
		`{"dataset":"test","definition":{"j":"family"},"networks":{"sameJ":true}}`,
	} {
		r := postJSON(t, h, "/api/networks", body)
		if r.Code != 400 {
			t.Fatalf("%s: %d %s", body, r.Code, r.Body)
		}
	}
}

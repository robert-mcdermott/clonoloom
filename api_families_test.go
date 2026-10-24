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
	"strings"
	"testing"
	"time"
)

func TestFamiliesAnalysisPagesAndExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bcr.tsv")
	data := "sequence_id\trepertoire_id\tlocus\tv_call\tj_call\tjunction\tproductive\tduplicate_count\n" +
		"a\ts1\tIGH\tIGHV1-1*01\tIGHJ1*01\tAAAAAAAAAAAA\tT\t2\n" +
		"b\ts1\tIGH\tIGHV1-1*02\tIGHJ1*01\tCAAAAAAAAAAA\tT\t3\n" +
		"c\ts2\tIGH\tIGHV1-1*01\tIGHJ1*01\tCCAAAAAAAAAA\tT\t4\n" +
		"d\ts2\tIGH\tIGHV1-1*01\tIGHJ1*01\tCCAAAAANAAAA\tT\t5\n"
	if e := os.WriteFile(path, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	a, h := testApp(t)
	var ds datasetSummary
	json.Unmarshal(postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`).Body.Bytes(), &ds)
	body := `{"dataset":` + jsonString(ds.ID) + `,"subjects":{"s1":"P1","s2":"P1"},"families":{"mode":"heavy","threshold":0.09}}`
	var job struct{ Job, Artifact string }
	started := postJSON(t, h, "/api/families", body)
	json.Unmarshal(started.Body.Bytes(), &job)
	if job.Job == "" {
		t.Fatal(started.Body)
	}
	waitForJob(t, a, job.Job, 10*time.Second)
	var read struct{ Result familyDocument }
	if e := json.Unmarshal(request2(t, h, "/api/families/"+job.Artifact), &read); e != nil {
		t.Fatal(e)
	}
	doc := read.Result
	if len(doc.Families) != 1 || doc.Eligible != 3 || len(doc.Observations) != 4 || *doc.Families[0].Samples[0].Abundance != 5 {
		t.Fatalf("%+v", doc)
	}
	var cached struct {
		Ready    bool
		Artifact string
	}
	json.Unmarshal(postJSON(t, h, "/api/families", body).Body.Bytes(), &cached)
	if !cached.Ready || cached.Artifact != job.Artifact {
		t.Fatal(cached)
	}
	for _, change := range []string{strings.Replace(body, "0.09", "0", 1), strings.Replace(body, "P1", "P2", 1), strings.Replace(body, `"mode":"heavy"`, `"mode":"heavy","scope":"sample"`, 1), strings.Replace(body, `"families":`, `"filter":{"excludeSamples":["s2"]},"families":`, 1)} {
		var out struct{ Artifact string }
		json.Unmarshal(postJSON(t, h, "/api/families", strings.TrimSuffix(change, "}")+`,"start":false}`).Body.Bytes(), &out)
		if out.Artifact == "" || out.Artifact == job.Artifact {
			t.Fatal("identity did not include options/subjects/filter", change, out)
		}
	}
	for _, kind := range []string{"families", "nearest", "members"} {
		var page struct {
			Count  int
			Rows   []json.RawMessage
			Result familyDocument
		}
		if e := json.Unmarshal(request2(t, h, "/api/families/"+job.Artifact+"/page?kind="+kind+"&limit=1"), &page); e != nil {
			t.Fatal(e)
		}
		if len(page.Rows) != 1 || page.Result.Observations != nil || page.Result.Representatives != nil || page.Result.Families != nil {
			t.Fatal(page)
		}
	}
	for _, query := range []string{"limit=201", "offset=-1", "limit=x", "kind=bad"} {
		if r := request(t, h, http.MethodGet, "/api/families/"+job.Artifact+"/page?"+query, nil, nil); r.Code != 400 {
			t.Fatal(r.Code)
		}
	}
	exported := request(t, h, http.MethodGet, "/api/export?what=result&artifact="+job.Artifact, nil, nil)
	if exported.Code != 200 {
		t.Fatal(exported.Body)
	}
	z, e := zip.NewReader(bytes.NewReader(exported.Body.Bytes()), int64(exported.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]int{}
	for _, f := range z.File {
		if filepath.Ext(f.Name) != ".csv" {
			if f.Name == "families.json" || f.Name == "manifest.json" {
				seen[f.Name] = 1
			}
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
	if seen["families.json"] != 1 || seen["manifest.json"] != 1 || seen["members.csv"] != 4 || seen["nearest.csv"] != 3 || seen["families.csv"] != 2 {
		t.Fatal(seen)
	}
	// Null means diagnostic, and an explicitly empty subject mapping never restores metadata.
	for _, opts := range []string{`{"threshold":null}`, `{"threshold":0}`} {
		r := postJSON(t, h, "/api/families", `{"dataset":`+jsonString(ds.ID)+`,"subjects":{},"families":`+opts+`}`)
		json.Unmarshal(r.Body.Bytes(), &job)
		waitForJob(t, a, job.Job, 10*time.Second)
		json.Unmarshal(request2(t, h, "/api/families/"+job.Artifact), &read)
		if read.Result.Eligible != 0 {
			t.Fatal("missing subjects were inferred")
		}
	}
	for _, opts := range []string{`{"threshold":-1}`, `{"mode":"paired"}`, `{"scope":"all"}`} {
		r := postJSON(t, h, "/api/families", `{"dataset":`+jsonString(ds.ID)+`,"families":`+opts+`}`)
		if r.Code != 400 {
			t.Fatal(r.Body)
		}
	}
}

func TestFamilyMetadataAndDefinitionIndependence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rows.tsv")
	meta := filepath.Join(dir, "repertoires.airr.json")
	if e := os.WriteFile(path, []byte("sequence_id\trepertoire_id\tlocus\tv_call\tj_call\tjunction\tproductive\na\ts\tIGH\tIGHV1-1*01\tIGHJ1*01\tAAAAAAAAAAAA\tT\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(meta, []byte(`{"Repertoire":[{"repertoire_id":"s","subject":{"subject_id":"P"}}]}`), 0600); e != nil {
		t.Fatal(e)
	}
	a, h := testApp(t)
	var ds datasetSummary
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`,`+jsonString(meta)+`]}`)
	if e := json.Unmarshal(opened.Body.Bytes(), &ds); e != nil || ds.ID == "" {
		t.Fatal(opened.Body, e)
	}
	body := `{"dataset":` + jsonString(ds.ID) + `,"families":{"threshold":0}}`
	var job struct{ Job, Artifact string }
	started := postJSON(t, h, "/api/families", body)
	json.Unmarshal(started.Body.Bytes(), &job)
	if job.Job == "" {
		t.Fatal(started.Body)
	}
	waitForJob(t, a, job.Job, 10*time.Second)
	var read struct{ Result familyDocument }
	json.Unmarshal(request2(t, h, "/api/families/"+job.Artifact), &read)
	if read.Result.Subjects["s"] != "P" || read.Result.Eligible != 1 {
		t.Fatal(read)
	}
	for _, extra := range []string{`,"subjects":{"s":" P "}`, `,"definition":{"junction":"nt","loci":["TRB"]}`, `,"options":{"seed":777,"replicates":20}`} {
		var cached struct {
			Ready    bool
			Artifact string
		}
		r := postJSON(t, h, "/api/families", strings.TrimSuffix(body, "}")+extra+`}`)
		json.Unmarshal(r.Body.Bytes(), &cached)
		if !cached.Ready || cached.Artifact != job.Artifact {
			t.Fatal("unrelated settings changed family identity", r.Body)
		}
	}
	r := postJSON(t, h, "/api/families", strings.TrimSuffix(body, "}")+`,"subjects":{}}`)
	json.Unmarshal(r.Body.Bytes(), &job)
	waitForJob(t, a, job.Job, 10*time.Second)
	json.Unmarshal(request2(t, h, "/api/families/"+job.Artifact), &read)
	if read.Result.Eligible != 0 || read.Result.Excluded["sample has no subject"] != 1 {
		t.Fatal("explicit empty subjects restored metadata", read)
	}
}

package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/artifacts"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsotypeIdentityPagesAndExport(t *testing.T) {
	a, h := testApp(t)
	path := filepath.Join(t.TempDir(), "input.tsv")
	os.WriteFile(path, []byte("sequence_id\tlocus\tv_call\tj_call\tjunction\tproductive\tc_call\tduplicate_count\n"+
		"a\tIGH\tIGHV1-1*01\tIGHJ1*01\tAAACCCGGG\tT\tIGHM*01\t0\n"+
		"b\tIGH\tIGHV1-1*01\tIGHJ1*01\tAAACCCGGG\tT\tIGHG1*01\t\n"+
		"c\tIGH\tIGHV1-1*01\tIGHJ1*01\tAAACCCGGG\tT\t\t2\n"), 0600)
	var ds datasetSummary
	json.Unmarshal(postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`).Body.Bytes(), &ds)
	var family struct{ Job, Artifact string }
	json.Unmarshal(postJSON(t, h, "/api/families", `{"dataset":`+jsonString(ds.ID)+`,"families":{"scope":"sample","threshold":0}}`).Body.Bytes(), &family)
	waitForJob(t, a, family.Job, 10*time.Second)
	body := `{"dataset":` + jsonString(ds.ID) + `,"isotypes":{"families":` + jsonString(family.Artifact) + `}}`
	var job struct {
		Job, Artifact string
		Ready         bool
	}
	response := postJSON(t, h, "/api/isotypes", body)
	if json.Unmarshal(response.Body.Bytes(), &job) != nil || job.Job == "" {
		t.Fatal(response.Body)
	}
	waitForJob(t, a, job.Job, 10*time.Second)
	var result struct {
		Result   isotypeDocument
		Artifact artifacts.Manifest
	}
	json.Unmarshal(request2(t, h, "/api/isotypes/"+job.Artifact), &result)
	r := result.Result
	if r.Unit != "heavy-chain rows" || r.AbundanceUnit != "" || r.Assigned != 3 || r.ResolvedClass != 2 || r.ResolvedGene != 2 || len(r.Pairs) != 1 || len(result.Artifact.Lineage.Parents) != 2 || r.Options.FamilySHA256 == "" {
		t.Fatalf("%+v", result)
	}
	for _, kind := range []string{"usage", "families", "pairs", "observations"} {
		var page struct {
			Rows   []json.RawMessage
			Result isotypeDocument
			Count  int
		}
		json.Unmarshal(request2(t, h, "/api/isotypes/"+job.Artifact+"/page?kind="+kind+"&limit=1"), &page)
		if len(page.Rows) != 1 || page.Result.Observations != nil || page.Result.Usage != nil || page.Result.Families != nil || page.Result.Pairs != nil {
			t.Fatal(page)
		}
	}
	for _, query := range []string{"limit=201", "offset=-1", "kind=bad", "offset=x", "level=bad"} {
		if res := request(t, h, http.MethodGet, "/api/isotypes/"+job.Artifact+"/page?"+query, nil, nil); res.Code != 400 {
			t.Fatal(res.Code, query)
		}
	}
	for _, query := range []string{"kind=observations&family=absent", "sample=absent", "kind=families&left=IGHM&right=IGHA1"} {
		var page struct{ Count int }
		json.Unmarshal(request2(t, h, "/api/isotypes/"+job.Artifact+"/page?"+query), &page)
		if page.Count != 0 {
			t.Fatal(query, page)
		}
	}
	var matched struct{ Count int }
	json.Unmarshal(request2(t, h, "/api/isotypes/"+job.Artifact+"/page?kind=families&left=IGHG1&right=IGHM"), &matched)
	if matched.Count != 1 {
		t.Fatal(matched)
	}
	exported := request(t, h, http.MethodGet, "/api/export?what=result&artifact="+job.Artifact, nil, nil)
	if exported.Code != 200 {
		t.Fatal(exported.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(exported.Body.Bytes()), int64(exported.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, file := range z.File {
		seen[file.Name] = true
		if file.Name == "usage.csv" {
			f, _ := file.Open()
			rows, err := csv.NewReader(f).ReadAll()
			f.Close()
			if err != nil || len(rows) != 13 || len(rows[0]) != 16 {
				t.Fatal(err, len(rows))
			}
			foundZero, foundMissing := false, false
			for _, row := range rows[1:] {
				if row[0] == "" && row[4] == "gene" {
					if row[14] != "heavy-chain rows" || row[15] != r.AbundanceUnit || row[7] != "3" || row[10] != "" || row[11] != "" || row[13] != "1" {
						t.Fatal(row)
					}
					if row[5] == "IGHM" {
						foundZero = row[9] == "0"
					}
					if row[5] == "IGHG1" {
						foundMissing = row[9] == ""
					}
				}
			}
			if !foundZero || !foundMissing {
				t.Fatal("missing/zero lost")
			}
		}
	}
	for _, name := range []string{"usage.csv", "families.csv", "cooccurrence.csv", "observations.csv", "sources.csv", "isotypes.json", "manifest.json", "NOTICE.txt"} {
		if !seen[name] {
			t.Fatal(name)
		}
	}
	changed := strings.TrimSuffix(body, "}") + `,"filter":{"productiveOnly":true},"definition":{"resolution":"gene"},"subjects":{"x":"y"},"options":{"seed":999}}`
	var cached struct {
		Artifact string
		Ready    bool
	}
	json.Unmarshal(postJSON(t, h, "/api/isotypes", changed).Body.Bytes(), &cached)
	if cached.Artifact != job.Artifact || !cached.Ready {
		t.Fatal(cached)
	}
	annotated := strings.TrimSuffix(body, "}") + `,"study":[{"name":"input","timepoint":"day 1","tissue":"blood"}],"start":false}`
	// Use the imported sample name, rather than assuming the file naming convention.
	annotated = strings.ReplaceAll(annotated, `"name":"input"`, `"name":`+jsonString(r.Samples[0]))
	var alternative struct{ Artifact string }
	resp := postJSON(t, h, "/api/isotypes", annotated)
	json.Unmarshal(resp.Body.Bytes(), &alternative)
	if resp.Code != 200 || alternative.Artifact == job.Artifact || alternative.Artifact == "" {
		t.Fatal(resp.Code, resp.Body)
	}
	parentPath, _ := a.artifacts.Path(family.Artifact, "families.json")
	original, _ := os.ReadFile(parentPath)
	os.WriteFile(parentPath, append(original, ' '), 0600)
	if res := postJSON(t, h, "/api/isotypes", body); res.Code != 400 || !strings.Contains(res.Body.String(), "checksum") {
		t.Fatal(res.Code, res.Body)
	}
	if res := request(t, h, http.MethodGet, "/api/isotypes/"+job.Artifact+"/page", nil, nil); res.Code != 200 {
		t.Fatal(res.Body)
	}
	if err := a.isotypeWork(isotypeAnalysis, analysisSpec{Dataset: ds.ID, Isotypes: &r.Options})(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}
}

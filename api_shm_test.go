package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
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

func TestSHMAnalysisIdentityPagesAndExport(t *testing.T) {
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

	shmBody := `{"dataset":` + jsonString(ds.ID) + `,"shm":{"germlines":` + jsonString(job.Artifact) + `}}`
	var startedSHM struct {
		Job, Artifact string
		Ready         bool
	}
	response := postJSON(t, h, "/api/shm", shmBody)
	if err := json.Unmarshal(response.Body.Bytes(), &startedSHM); err != nil || startedSHM.Job == "" {
		t.Fatal(response.Body, err)
	}
	waitForJob(t, a, startedSHM.Job, 10*time.Second)
	var shm struct {
		Result   shmDocument
		Artifact artifacts.Manifest
	}
	json.Unmarshal(request2(t, h, "/api/shm/"+startedSHM.Artifact), &shm)
	if shm.Result.Evaluated != 1 || len(shm.Result.Rows) != 2 || len(shm.Artifact.Lineage.Parents) != 2 || shm.Result.Options.GermlineSHA256 == "" || shm.Result.Germlines.Files[0].SHA256 == "" {
		t.Fatal(shm)
	}
	for _, kind := range []string{"summaries", "rows"} {
		var page struct {
			Rows   []json.RawMessage
			Result shmDocument
			Count  int
		}
		json.Unmarshal(request2(t, h, "/api/shm/"+startedSHM.Artifact+"/page?kind="+kind+"&limit=1"), &page)
		if len(page.Rows) != 1 || page.Result.Rows != nil || page.Result.Summaries != nil {
			t.Fatal(page)
		}
	}
	for _, query := range []string{"limit=201", "offset=-1", "kind=bad", "offset=x"} {
		if r := request(t, h, http.MethodGet, "/api/shm/"+startedSHM.Artifact+"/page?"+query, nil, nil); r.Code != 400 {
			t.Fatal(r.Code)
		}
	}
	var filtered struct{ Count int }
	json.Unmarshal(request2(t, h, "/api/shm/"+startedSHM.Artifact+"/page?kind=rows&family=absent"), &filtered)
	if filtered.Count != 0 {
		t.Fatal(filtered)
	}
	export := request(t, h, http.MethodGet, "/api/export?what=result&artifact="+startedSHM.Artifact, nil, nil)
	if export.Code != 200 {
		t.Fatal(export.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(export.Body.Bytes()), int64(export.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, file := range z.File {
		seen[file.Name] = true
		if file.Name == "shm.csv" {
			f, _ := file.Open()
			raw, _ := io.ReadAll(f)
			f.Close()
			rows, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
			if err != nil || len(rows) != 21 || len(rows[0]) != 30 {
				t.Fatal(err, len(rows), rows[0])
			}
			missing, measured := 0, 0
			for _, r := range rows[1:] {
				if r[14] == "all" {
					if r[11] != "" {
						if r[16] != "" || r[18] != "" {
							t.Fatal(r)
						}
						missing++
					} else {
						if r[16] != "6" || r[17] != "0" || r[18] != "0" {
							t.Fatal(r)
						}
						measured++
					}
				}
			}
			if missing != 1 || measured != 1 {
				t.Fatal(missing, measured)
			}
		}
	}
	for _, name := range []string{"shm.csv", "summaries.csv", "shm.json", "manifest.json", "NOTICE.txt"} {
		if !seen[name] {
			t.Fatal("missing", name)
		}
	}
	// Active QC, clonotype definitions, subjects and diversity settings do not change a derived result.
	changed := strings.TrimSuffix(shmBody, "}") + `,"filter":{"productiveOnly":true},"definition":{"resolution":"gene"},"subjects":{"x":"y"},"options":{"seed":999}}`
	var cached struct {
		Artifact string
		Ready    bool
	}
	json.Unmarshal(postJSON(t, h, "/api/shm", changed).Body.Bytes(), &cached)
	if cached.Artifact != startedSHM.Artifact || !cached.Ready {
		t.Fatal(cached)
	}
	// Neither reference files nor the parent need to remain available to inspect the saved SHM result.
	os.Remove(reference)
	parentPath, _ := a.artifacts.Path(job.Artifact, "germlines.json")
	original, _ := os.ReadFile(parentPath)
	os.WriteFile(parentPath, append(original, ' '), 0600)
	if r := postJSON(t, h, "/api/shm", shmBody); r.Code != 400 || !strings.Contains(r.Body.String(), "checksum") {
		t.Fatal(r.Code, r.Body)
	}
	if r := request(t, h, http.MethodGet, "/api/shm/"+startedSHM.Artifact+"/page", nil, nil); r.Code != 200 {
		t.Fatal(r.Body)
	}
	// Resumed work rechecks the parent's checksum, independently of request preparation.
	spec := analysisSpec{Dataset: ds.ID, SHM: &shm.Result.Options}
	if err := a.shmWork(shmAnalysis, spec)(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}
}

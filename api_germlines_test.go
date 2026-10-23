package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
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

	"clonoloom/internal/artifacts"
	"clonoloom/internal/examples"
)

func TestGermlinesAnalysisIdentityPagesAndExport(t *testing.T) {
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
	for _, kind := range []string{"families", "rows"} {
		var page struct {
			Rows   []json.RawMessage
			Result germlineDocument
			Count  int
		}
		json.Unmarshal(request2(t, h, "/api/germlines/"+job.Artifact+"/page?kind="+kind+"&limit=1"), &page)
		if len(page.Rows) != 1 || page.Result.Rows != nil || page.Result.Families != nil {
			t.Fatal(page)
		}
	}
	for _, query := range []string{"limit=201", "offset=-1", "kind=bad", "offset=x"} {
		if r := request(t, h, http.MethodGet, "/api/germlines/"+job.Artifact+"/page?"+query, nil, nil); r.Code != 400 {
			t.Fatal(r.Code)
		}
	}
	export := request(t, h, http.MethodGet, "/api/export?what=result&artifact="+job.Artifact, nil, nil)
	if export.Code != 200 {
		t.Fatal(export.Body)
	}
	z, e := zip.NewReader(bytes.NewReader(export.Body.Bytes()), int64(export.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for _, f := range z.File {
		seen[f.Name] = true
		if f.Name == "germlines.csv" || f.Name == "germlines.fasta" {
			handle, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(handle)
			handle.Close()
			if err != nil {
				t.Fatal(err)
			}
			if f.Name == "germlines.csv" {
				rows, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
				if err != nil || len(rows) != 3 || len(rows[0]) != 15 {
					t.Fatal(err, rows)
				}
				success := 0
				for _, row := range rows[1:] {
					if row[11] == "" {
						success++
						if row[13] != "AAANNNGGG" {
							t.Fatal(row)
						}
					}
				}
				if success != 1 {
					t.Fatal(rows)
				}
			} else if strings.Count(string(raw), ">") != 1 || !strings.Contains(string(raw), "AAANNNGGG") {
				t.Fatal(string(raw))
			}
		}
	}
	for _, name := range []string{"germlines.json", "germlines.csv", "germlines.fasta", "manifest.json"} {
		if !seen[name] {
			t.Fatal(seen)
		}
	}
	// Irrelevant current QC/definition/subjects must not change inherited membership or identity.
	changed := strings.TrimSuffix(body, "}") + `,"filter":{"excludeSamples":["input"]},"definition":{"junction":"aa"},"subjects":{"input":"other"}}`
	var cached struct {
		Artifact string
		Ready    bool
	}
	json.Unmarshal(postJSON(t, h, "/api/germlines", changed).Body.Bytes(), &cached)
	if !cached.Ready || cached.Artifact != job.Artifact {
		t.Fatal(cached)
	}
	var spec analysisSpec
	json.Unmarshal(result.Artifact.Lineage.Job.Spec, &spec)
	os.WriteFile(reference, []byte(strings.Replace(fasta, "AAA", "AAC", 1)), 0600)
	// Resume refuses a reference changed after job submission; a fresh request gets a new ID.
	err := a.germlineWork(germlineAnalysis, spec)(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatal(err)
	}
	changed = strings.TrimSuffix(body, "}") + `,"start":false}`
	json.Unmarshal(postJSON(t, h, "/api/germlines", changed).Body.Bytes(), &cached)
	if cached.Ready || cached.Artifact == job.Artifact {
		t.Fatal(cached)
	}
	// Saved result inspection does not need the reference or the dataset to be reopened.
	os.Remove(reference)
	if r := request(t, h, http.MethodGet, "/api/germlines/"+job.Artifact+"/page", nil, nil); r.Code != 200 {
		t.Fatal(r.Body)
	}
	os.WriteFile(reference, []byte(fasta), 0600)
	fp, _ := a.artifacts.Path(family.Artifact, "families.json")
	raw, _ := os.ReadFile(fp)
	os.WriteFile(fp, append(raw, ' '), 0600)
	if r := postJSON(t, h, "/api/germlines", body); r.Code != 400 || !strings.Contains(r.Body.String(), "checksum") {
		t.Fatal(r.Body)
	}
}

func TestGermlineCatalogChecksActualBytes(t *testing.T) {
	a, _ := testApp(t)
	dir := t.TempDir()
	folder := filepath.Join(dir, humanGermlines)
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(">IGHV1-1*01\nAAA\n")
	path := filepath.Join(folder, "segments.fasta")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	source := examples.Source{Name: humanGermlines, Files: []examples.File{{Path: "segments.fasta", Size: int64(len(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}}}
	a.references = &examples.Library{Dir: dir, Sources: []examples.Source{source}, Offline: true}
	if ref, files, err := a.germlineReference(humanGermlines); err != nil || len(ref) != 1 || files[0].SHA256 != source.Files[0].SHA256 {
		t.Fatal(ref, files, err)
	}
	if _, _, err := a.germlineReference("../../segments.fasta"); err == nil {
		t.Fatal("arbitrary reference accepted")
	}
	if err := os.WriteFile(path, bytes.ReplaceAll(raw, []byte("AAA"), []byte("AAC")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.germlineReference(humanGermlines); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}
}

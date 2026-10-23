package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/examples"
	"clonoloom/internal/track"
)

func TestSpecificityAnalysisAndExport(t *testing.T) {
	a, h := testApp(t)
	path := filepath.Join(t.TempDir(), "trb.tsv")
	if e := os.WriteFile(path, []byte(tcrdistTSV), 0600); e != nil {
		t.Fatal(e)
	}
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	var ds datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &ds)
	base := `"dataset":` + jsonString(ds.ID) + `,"definition":{"junction":"aa","v":"gene","j":"gene"}`
	if r := postJSON(t, h, "/api/specificity", `{`+base+`}`); r.Code != 400 || !strings.Contains(r.Body.String(), "no-database") {
		t.Fatal(r.Code, r.Body)
	}
	raw := []byte("gene\tcdr3\tv.segm\tj.segm\tspecies\tmhc.a\tmhc.b\tmhc.class\tantigen.epitope\tantigen.gene\tantigen.species\treference.id\tvdjdb.score\nTRB\tCASSLAGGPEAFF\tTRBV5-1\tTRBJ2-7\tHomoSapiens\tA\tB\tI\tPEPTIDE\tprotein\tSimulated\tclonoloom-test\t3\n")
	dir := filepath.Join(a.references.Dir, "vdjdb")
	os.MkdirAll(dir, 0700)
	file := filepath.Join(dir, "vdjdb.txt")
	os.WriteFile(file, raw, 0600)
	a.references.Sources = []examples.Source{{Name: "vdjdb", License: "simulated", Citation: "ClonoLoom simulated data", Files: []examples.File{{Path: "vdjdb.txt", Size: int64(len(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}}}}
	body := `{` + base + `,"specificity":{"genes":true}}`
	r := postJSON(t, h, "/api/specificity", body)
	var job struct{ Job, Artifact string }
	json.Unmarshal(r.Body.Bytes(), &job)
	if job.Job == "" {
		t.Fatal(r.Body)
	}
	waitForJob(t, a, job.Job, 10*time.Second)
	var read struct{ Result specificityDocument }
	json.Unmarshal(request2(t, h, "/api/specificity/"+job.Artifact), &read)
	if read.Result.Database.SHA256 == "" || read.Result.Options.Database != "vdjdb" || len(read.Result.Samples) == 0 || len(read.Result.Matches) != 1 || len(read.Result.Epitopes) != 1 {
		t.Fatal(read)
	}
	again := postJSON(t, h, "/api/specificity", body)
	if !strings.Contains(again.Body.String(), `"ready":true`) {
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
	found := map[string]bool{}
	for _, f := range z.File {
		found[filepath.Base(f.Name)] = true
	}
	if !found["matches.csv"] || !found["epitopes.csv"] {
		t.Fatal(found)
	}
	for _, opts := range []string{`{"metric":"wrong"}`, `{"database":"/tmp/private"}`, `{"radius":-1}`, `{"species":"unknown"}`, `{"minScore":4}`} {
		r := postJSON(t, h, "/api/specificity", `{`+base+`,"specificity":`+opts+`}`)
		if r.Code != 400 {
			t.Fatal(opts, r.Code, r.Body)
		}
	}
	spec := analysisSpec{Dataset: ds.ID, Definition: read.Result.Definition, Specificity: &read.Result.Options}
	spec, _ = spec.normalized(1)
	first := spec.artifactID("spc-")
	spec.Specificity.SHA256 = "different"
	if spec.artifactID("spc-") == first {
		t.Fatal("database checksum missing from identity")
	}
	// A changed file of the same length must fail, including on lookups of kept results.
	raw[0] = 'X'
	os.WriteFile(file, raw, 0600)
	if r := postJSON(t, h, "/api/specificity", body); r.Code != 400 {
		t.Fatal("accepted changed reference", r.Body)
	}
}

func TestSpecificityPublishedDownload(t *testing.T) {
	if os.Getenv("CLONOLOOM_VDJDB_DOWNLOAD") != "1" {
		t.Skip("maintainer-only on-request reference download")
	}
	library := newReferences(t.TempDir(), false)
	if _, err := library.Download(context.Background(), "vdjdb", false, nil); err != examples.ErrTerms {
		t.Fatal("terms were not required", err)
	}
	path, err := library.Download(context.Background(), "vdjdb", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(path, "vdjdb.txt"))
	if err != nil || info.Size() != 239910782 {
		t.Fatal(info, err)
	}
	t.Log("published archive members downloaded and verified by size, CRC and SHA-256")
}

func TestSpecificityTrackingIdentity(t *testing.T) {
	s := analysisSpec{Dataset: "dataset", Track: &track.Choice{By: track.ByList, List: []string{"clonotype"}}}
	s, err := s.normalized(trackingAnalysis.version)
	if err != nil {
		t.Fatal(err)
	}
	ordinary := s.artifactID("trk-")
	s.Track.Evidence = &track.Evidence{Artifact: "spc-one", Epitope: "PEPTIDE", Antigen: "protein", Species: "Simulated"}
	first := s.artifactID("trk-")
	s.Track.Evidence.Artifact = "spc-two"
	if first == ordinary || first == s.artifactID("trk-") {
		t.Fatal("tracking evidence missing from artifact identity")
	}
}

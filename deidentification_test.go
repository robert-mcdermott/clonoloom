package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/export"
	"clonoloom/internal/qc"
)

// Artificial identity canaries; the sequences/counts are small export controls, not biology.
func deidFixture(t *testing.T) (*app, http.Handler, deidRequest, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "PRIVATE-file-canary.tsv")
	raw := "sequence_id\tcell_id\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\trepertoire_id\tsequence\tjunction\n" +
		"PRIVATE-seq-1\tPRIVATE-cell-1\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\t3\tPRIVATE-sample-A\tACGTN\tTGTGCC\n" +
		"PRIVATE-seq-2\tPRIVATE-cell-1\tTRBV6-2*01\tTRBJ1-1*01\tCASSPGQGAYEQYF\tT\t0\tPRIVATE-sample-A\tACGT\tTGTGCC\n" +
		"PRIVATE-seq-1\tPRIVATE-cell-1\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\t5\tPRIVATE-sample-B\tACGTN\tTGTGCC\n" +
		"PRIVATE-seq-3\t\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tF\t\tPRIVATE-sample-C\t\t\n"
	if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	a, h := testApp(t)
	t.Cleanup(a.jobs.Close)
	opened := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": []string{path}})))
	if opened.Code != 200 {
		t.Fatal(opened.Body.String())
	}
	var s datasetSummary
	if e := json.Unmarshal(opened.Body.Bytes(), &s); e != nil {
		t.Fatal(e)
	}
	yes := true
	r := deidRequest{Dataset: s.ID, What: "tables", DropText: &yes, DropHLA: &yes, Definition: clonotype.Definition{Junction: "aa", V: "gene", J: "gene", Name: "PRIVATE-definition-label"}}
	for i, sample := range s.Samples {
		subject := "PRIVATE-subject-one"
		if i == 2 {
			subject = ""
		}
		r.Annotations = append(r.Annotations, deidAnnotation{sample.Name, map[string]string{"subject": subject, "timepoint": "day 14", "condition": "PRIVATE-free-text", "hla": "HLA-A*02:01"}})
	}
	return a, h, r, path
}
func deidRows(t *testing.T, s string, comma rune) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(s))
	r.Comma = comma
	rows, e := r.ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	return rows
}

func TestDeidentificationScientificEvidenceAndPrivateMap(t *testing.T) {
	a, _, r, path := deidFixture(t)
	before, _ := os.ReadFile(path)
	ds, entry, _, e := a.clonotypeTable(clonotypeRequest{Dataset: r.Dataset, Definition: r.Definition, Filter: r.Filter})
	if e != nil {
		t.Fatal(e)
	}
	var nativeAIRR, nativeClones bytes.Buffer
	keep, _ := qc.Apply(ds.Result.Table, r.Filter)
	if e = export.Rearrangements(&nativeAIRR, ds.Result.Table, keep, entry.table); e != nil {
		t.Fatal(e)
	}
	names := []string{}
	for _, s := range ds.Result.Table.Samples {
		names = append(names, s.Name)
	}
	if e = export.Clonotypes(&nativeClones, entry.table, names); e != nil {
		t.Fatal(e)
	}
	p, e := a.prepareDeidentified(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	files := unzip(t, p.Bundle)
	if len(files) != 5 {
		t.Fatalf("unexpected files: %v", files)
	}
	for name, raw := range files {
		for _, canary := range []string{"PRIVATE-", path, ds.ID, ds.Result.Sources[0].SHA256, "HLA-A*02:01"} {
			if strings.Contains(raw, canary) {
				t.Fatalf("identity disclosed in %s: %s", name, canary)
			}
		}
	}
	if replayDigest([]byte(files["transformation.json"])) != p.Preview.Fingerprint {
		t.Fatal("record fingerprint")
	}
	for _, f := range p.Preview.Record.Files {
		if replayDigest([]byte(files[f.Path])) != f.SHA256 || int64(len(files[f.Path])) != f.Size {
			t.Fatal("inventory mismatch")
		}
	}
	original := deidRows(t, nativeAIRR.String(), '\t')
	transformed := deidRows(t, files["rearrangements.tsv"], '\t')
	if len(original) != len(transformed) || !reflect.DeepEqual(original[0], transformed[0]) {
		t.Fatal("row/header mismatch")
	}
	at := map[string]int{}
	for i, key := range original[0] {
		at[key] = i
	}
	for row := 1; row < len(original); row++ {
		for col, key := range original[0] {
			if key != "sequence_id" && key != "cell_id" && key != "repertoire_id" && original[row][col] != transformed[row][col] {
				t.Fatalf("changed scientific field %s row %d", key, row)
			}
		}
	}
	if transformed[1][at["cell_id"]] != transformed[2][at["cell_id"]] || transformed[1][at["cell_id"]] == transformed[3][at["cell_id"]] || transformed[4][at["cell_id"]] != "" {
		t.Fatal("cell membership/missingness changed")
	}
	if transformed[1][at["sequence_id"]] == transformed[3][at["sequence_id"]] {
		t.Fatal("sequence scope crossed samples")
	}
	oldClones, newClones := deidRows(t, nativeClones.String(), ','), deidRows(t, files["clonotypes.csv"], ',')
	if !reflect.DeepEqual(oldClones[1:], newClones[1:]) {
		t.Fatal("clonotype evidence changed")
	}
	sampleRows := deidRows(t, files["samples.csv"], ',')
	if sampleRows[1][1] != sampleRows[2][1] || sampleRows[3][1] != "" || p.Preview.Record.Subjects != 1 || p.Preview.Record.MissingSubjects != 1 {
		t.Fatal("subject correspondence changed")
	}
	for i, row := range sampleRows[1:] {
		if newClones[0][len(newClones[0])-3+i] != row[0] {
			t.Fatal("sample headers do not join")
		}
	}
	var private struct {
		Fingerprint string        `json:"bundleFingerprint"`
		Bindings    []deidBinding `json:"bindings"`
	}
	if e = json.Unmarshal(p.IdentityMap, &private); e != nil {
		t.Fatal(e)
	}
	if private.Fingerprint != p.Preview.Fingerprint || !strings.Contains(string(p.IdentityMap), path) || !strings.Contains(string(p.IdentityMap), ds.Result.Sources[0].SHA256) {
		t.Fatal("private provenance missing")
	}
	for _, binding := range private.Bindings {
		if binding.Kind == "cell" && binding.Scope == names[0] && binding.Original == "PRIVATE-cell-1" && binding.Pseudonym != transformed[1][at["cell_id"]] {
			t.Fatal("wrong correspondence")
		}
	}
	again, e := a.prepareDeidentified(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	if again.Preview.Subjects[0].Pseudonym == p.Preview.Subjects[0].Pseudonym {
		t.Fatal("pseudonyms reused across preparations")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("source mutated")
	}
	var afterAIRR bytes.Buffer
	export.Rearrangements(&afterAIRR, ds.Result.Table, keep, entry.table)
	if afterAIRR.String() != nativeAIRR.String() {
		t.Fatal("native scientific evidence mutated")
	}
}

func TestDeidentificationChoicesRefusalsAndFiltering(t *testing.T) {
	a, _, r, _ := deidFixture(t)
	no := false
	t.Run("explicit choices", func(t *testing.T) {
		bad := r
		bad.DropHLA = nil
		if _, e := a.prepareDeidentified(context.Background(), bad); e == nil {
			t.Fatal("implicit choice")
		}
	})
	t.Run("unknown field", func(t *testing.T) {
		r.Annotations[0].Fields["rawMetadata"] = "PRIVATE"
		defer delete(r.Annotations[0].Fields, "rawMetadata")
		if _, e := a.prepareDeidentified(context.Background(), r); e == nil {
			t.Fatal("unknown field")
		}
	})
	t.Run("retained annotation", func(t *testing.T) {
		r.DropText = &no
		r.DropHLA = &no
		p, e := a.prepareDeidentified(context.Background(), r)
		if e != nil {
			t.Fatal(e)
		}
		samples := unzip(t, p.Bundle)["samples.csv"]
		if !strings.Contains(samples, "PRIVATE-free-text") || !strings.Contains(samples, "HLA-A*02:01") {
			t.Fatal("explicitly retained fields lost")
		}
		r.Annotations[0].Fields["condition"] = "subject PRIVATE-subject-one"
		if _, e = a.prepareDeidentified(context.Background(), r); e == nil {
			t.Fatal("known identity in annotation")
		}
		r.Annotations[0].Fields["condition"] = "HLA-A*02:01"
		yes := true
		r.DropHLA = &yes
		if _, e = a.prepareDeidentified(context.Background(), r); e == nil {
			t.Fatal("HLA in retained text")
		}
		r.Annotations[0].Fields["condition"] = "PRIVATE-free-text"
		r.DropText = &yes
	})
	t.Run("QC aliases", func(t *testing.T) {
		filtered := r
		filtered.Filter = qc.Filter{ProductiveOnly: true, ExcludeSamples: []string{r.Annotations[1].Sample}}
		p, e := a.prepareDeidentified(context.Background(), filtered)
		if e != nil {
			t.Fatal(e)
		}
		if p.Preview.Record.Rows != 2 || !strings.HasPrefix(p.Preview.Record.Filter.ExcludeSamples[0], "sample-") {
			t.Fatal("QC not preserved")
		}
		filtered.Filter.Loci = []string{"PRIVATE-custom-locus"}
		if _, e = a.prepareDeidentified(context.Background(), filtered); e == nil {
			t.Fatal("unmapped QC text")
		}
	})
	t.Run("restricted", func(t *testing.T) {
		ds, _ := a.datasets.get(r.Dataset)
		ds.Credits = []credit{{Title: "Provider study", LicenseClass: "terms", Citation: "Required citation"}}
		defer func() { ds.Credits = nil }()
		if _, e := a.prepareDeidentified(context.Background(), r); e == nil {
			t.Fatal("missing acknowledgment")
		}
		accepted := r
		accepted.Accept = true
		p, e := a.prepareDeidentified(context.Background(), accepted)
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(unzip(t, p.Bundle)["NOTICE.txt"], "THESE DATA MAY NOT BE SHARED") {
			t.Fatal("lost provider restrictions")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := a.prepareDeidentified(ctx, r); e == nil {
		t.Fatal("cancelled export succeeded")
	}
}

func TestDeidentificationUnmappedEvidenceRefused(t *testing.T) {
	a, h, r, path := deidFixture(t)
	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	lines[0] += "\tgermline_database"
	for i := 1; i < len(lines); i++ {
		lines[i] += "\tPRIVATE-unmapped-provenance"
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
	opened := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": []string{path}})))
	var s datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &s)
	r.Dataset = s.ID
	if _, e := a.prepareDeidentified(context.Background(), r); e == nil || !strings.Contains(e.Error(), "no de-identification contract") {
		t.Fatalf("unmapped evidence: %v", e)
	}
	r.What = "clonotypes"
	p, e := a.prepareDeidentified(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	if len(unzip(t, p.Bundle)) != 4 {
		t.Fatal("clonotype-only scope differs")
	}
}

func TestDeidentificationHTTPSeparatePinnedDownloads(t *testing.T) {
	_, h, r, _ := deidFixture(t)
	prepare := func() deidPreview {
		t.Helper()
		rec := postJSON(t, h, "/api/deidentification/prepare", string(mustJSON(r)))
		if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		var p deidPreview
		json.Unmarshal(rec.Body.Bytes(), &p)
		return p
	}
	p := prepare()
	base := "/api/deidentification/" + p.Token + "?expected=" + p.Fingerprint
	for _, part := range []string{"bundle", "identity-map"} {
		rec := request(t, h, "GET", base+"&part="+part, nil, nil)
		if rec.Code != 200 || rec.Header().Get("X-ClonoLoom-Deidentification") != p.Fingerprint || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("download %s %d", part, rec.Code)
		}
		if part == "bundle" && len(unzip(t, rec.Body.Bytes())) != 5 {
			t.Fatal("map in bundle")
		}
	}
	for _, path := range []string{base, base + "&part=all", "/api/deidentification/" + p.Token + "?part=bundle"} {
		if request(t, h, "GET", path, nil, nil).Code == 200 {
			t.Fatal("unpinned/implicit download")
		}
	}
	prepare()
	if request(t, h, "GET", base+"&part=bundle", nil, nil).Code != 404 {
		t.Fatal("old preparation remained")
	}
	for _, body := range []string{`{"what":"tables","what":"clonotypes"}`, `{"unknown":true}`, string([]byte{'{', '"', 'x', '"', ':', '"', 255, '"', '}'})} {
		if postJSON(t, h, "/api/deidentification/prepare", body).Code != 400 {
			t.Fatal("invalid request accepted")
		}
	}
}
func TestDeidentificationBoundsAndAliasFraming(t *testing.T) {
	a, e := newDeidAliases()
	if e != nil {
		t.Fatal(e)
	}
	one, _ := a.alias("cell", "a:b", "c")
	two, _ := a.alias("cell", "a", "b:c")
	again, _ := a.alias("cell", "a:b", "c")
	if one == two || one != again {
		t.Fatal("ambiguous alias framing")
	}
	if got, e := a.alias("cell", "a", ""); e != nil || got != "" {
		t.Fatal("invented missing identity")
	}
	if _, e = a.alias("cell", "a", strings.Repeat("x", 8193)); e == nil {
		t.Fatal("unbounded ID")
	}
	a.entries = make([]deidBinding, deidMaxBindings)
	if _, e = a.alias("cell", "new", "new"); e == nil {
		t.Fatal("unbounded correspondence")
	}
	b := &deidBuffer{ctx: context.Background(), limit: 3}
	if _, e = b.WriteString("four"); e == nil || b.Len() != 0 {
		t.Fatal("unbounded output")
	}
}

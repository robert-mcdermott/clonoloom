package validation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

type contigJSONReference struct {
	SHA256  string
	Columns map[string][]string
}

func compareJSONContigs(t *testing.T, path string, want contigJSONReference) {
	t.Helper()
	r, err := reads.Open(context.Background(), []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sources[0].SHA256 != want.SHA256 || string(r.Unit) != "cells" {
		t.Fatal("source/unit differs")
	}
	got := columns(r.Table)
	for i := 0; i < r.Table.Len(); i++ {
		umi := missing
		if len(r.Table.UMIs) > 0 {
			umi = numberText(r.Table.UMIs[i])
		}
		got["umis"] = append(got["umis"], umi)
		got["sequence"] = append(got["sequence"], orMissing(r.Table.EvidenceValue(i, 0)))
		got["vj_in_frame"] = append(got["vj_in_frame"], flagText(r.Table.VJInFrame[i]))
		got["stop_codon"] = append(got["stop_codon"], flagText(r.Table.StopCodon[i]))
	}
	for k, v := range want.Columns {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("%s differs", k)
		}
	}
	data, _ := os.ReadFile(path)
	var raw []json.RawMessage
	json.Unmarshal(data, &raw)
	for i, v := range raw {
		if r.Sources[0].Native.Rows[i].Text != string(v) {
			t.Fatalf("record %d source changed", i)
		}
	}
}

func TestCellRangerJSONIndependentValues(t *testing.T) {
	data, err := os.ReadFile("reference/cellranger_json.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref map[string]contigJSONReference
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	for name, want := range ref {
		t.Run(name, func(t *testing.T) { compareJSONContigs(t, filepath.Join("../testdata/cellranger-json", name), want) })
	}
}

func TestCellRangerJSONPublicReference(t *testing.T) {
	path, reference := os.Getenv("CLONOLOOM_CONTIG_JSON"), os.Getenv("CLONOLOOM_CONTIG_REFERENCE")
	if path == "" || reference == "" {
		t.Skip("optional explicitly downloaded public JSON/CSV comparison")
	}
	data, err := os.ReadFile(reference)
	if err != nil {
		t.Fatal(err)
	}
	var want contigJSONReference
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	compareJSONContigs(t, path, want)
}

func TestCellRangerJSONDistinctCellAbundance(t *testing.T) {
	r, err := reads.Open(context.Background(), []string{"../testdata/cellranger-json/simulated_all_contig_annotations.json"})
	if err != nil {
		t.Fatal(err)
	}
	keep := make([]bool, r.Table.Len())
	for i := range keep {
		keep[i] = r.Table.IsCell[i] == rep.True
	}
	d := clonotype.Definition{Chains: []string{"TRA", "TRB"}, Junction: "aa", V: "gene", J: "gene", Cells: "primary", RequireAll: true, ProductiveOnly: true}
	result, err := clonotype.Compute(r.Table, keep, d)
	if err != nil {
		t.Fatal(err)
	}
	if result.Cells != 2 || len(result.Clonotypes) != 2 || result.SampleTotals[0] != 2 {
		t.Fatalf("expected two declared cells, not summed support: %+v", result)
	}
	for _, c := range result.Clonotypes {
		if c.Total != 1 {
			t.Fatal("contig support became cell abundance", c.Total)
		}
	}
	// The lower-UMI second alpha remains source evidence; primary-chain selection
	// is the explicit existing definition, not a reader-created pairing flag.
	if r.Table.Len() != 8 || len(r.Sources[0].Native.Rows) != 8 {
		t.Fatal("import filtered source contigs")
	}
}

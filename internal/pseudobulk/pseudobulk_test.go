package pseudobulk

import (
	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func fixture(t *testing.T) Input {
	t.Helper()
	b, e := os.ReadFile("../../validation/reference/pseudobulk.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct{ Input Input }
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	return v.Input
}
func TestEvidenceRefusals(t *testing.T) {
	for name, change := range map[string]func(*Input){
		"duplicate subject visit": func(in *Input) { in.Samples[1].Annotation.Time = in.Samples[0].Annotation.Time },
		"missing subject":         func(in *Input) { in.Samples[0].Annotation.Subject = "" },
		"missing time":            func(in *Input) { in.Samples[0].Annotation.Time = "" },
		"duplicate expression":    func(in *Input) { in.Samples[1].Expression = in.Samples[0].Expression },
		"duplicate barcode":       func(in *Input) { in.Samples[0].Cells[1].Barcode = in.Samples[0].Cells[0].Barcode },
		"feature modality":        func(in *Input) { in.Features[0].Type = "Antibody Capture" },
		"duplicate feature":       func(in *Input) { in.Features[1].ID = in.Features[0].ID },
		"count overflow":          func(in *Input) { in.Samples[0].Cells[0].Counts[0].Count = MaxCount + 1 },
		"library overflow":        func(in *Input) { in.Samples[0].Cells[0].Counts[0].Count = MaxCount },
		"negative feature":        func(in *Input) { in.Samples[0].Cells[0].Counts[0].Feature = -1 },
		"bad category":            func(in *Input) { in.Samples[0].Cells[0].Bin = "invented" },
		"bad minimum":             func(in *Input) { in.Options.MinCells = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			in := fixture(t)
			change(&in)
			if _, e := Compute(context.Background(), in); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Compute(ctx, fixture(t)); e == nil {
		t.Fatal("ignored cancellation")
	}
}
func TestThresholdPreservesCounts(t *testing.T) {
	in := fixture(t)
	r, e := Compute(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	in.Options.MinCells = 1000
	next, e := Compute(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	for i, g := range r.Groups {
		a, _ := json.Marshal(g.Counts)
		b, _ := json.Marshal(next.Groups[i].Counts)
		if string(a) != string(b) || g.Cells != next.Groups[i].Cells {
			t.Fatal("threshold changed evidence")
		}
		if next.Groups[i].Eligible {
			t.Fatal("wrong eligibility")
		}
	}
}

func TestCaptureMembershipAndModalities(t *testing.T) {
	n := 1
	p := &phenotype.Result{Options: phenotype.Options{Population: "receptor", Breakpoints: []int{1}}, Columns: []string{"type"}, Cells: []phenotype.Cell{{Barcode: "a", Values: []string{"A"}, Clone: "c", Size: &n, Bin: "1"}, {Barcode: "b", Values: []string{"A"}}}}
	m := &expression.Matrix{Features: []expression.Feature{{ID: "g", Name: "same", Type: "Gene Expression"}, {ID: "adt", Name: "same", Type: "Antibody Capture"}}, Barcodes: []string{"a", "b"}, Indptr: []int{0, 2, 2}, Indices: []uint32{0, 1}, Counts: []uint64{7, 99}}
	features, cells, bins, e := Capture(m, p, Options{Annotation: "type", MinCells: 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(features) != 1 || len(cells) != 2 || cells[0].Counts[0].Count != 7 || len(cells[0].Counts) != 1 || cells[1].Reason != "Unavailable unique clonotype size" || len(bins) != 2 {
		t.Fatal("capture", features, cells, bins)
	}
	p.Options.Population = "mapped"
	if _, _, _, e = Capture(m, p, Options{Annotation: "type"}); e == nil {
		t.Fatal("mapped accepted")
	}
	p.Options.Population = "receptor"
	if _, _, _, e = Capture(m, p, Options{Annotation: "missing"}); e == nil {
		t.Fatal("absent column accepted")
	}
	p.Cells[0].Barcode = "wrong"
	if _, _, _, e = Capture(m, p, Options{Annotation: "type"}); e == nil {
		t.Fatal("wrong barcode accepted")
	}
}

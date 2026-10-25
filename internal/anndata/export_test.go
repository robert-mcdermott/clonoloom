package anndata

import (
	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fixture() (*expression.Input, *phenotype.Result, *expression.AnalysisResult) {
	a, b := [2]float64{1.5, -2}, [2]float64{3, 4}
	size := 2
	in := &expression.Input{ID: "fixture", Matrix: &expression.Matrix{Barcodes: []string{"β-1", "B-1", "ZERO-1"}, Features: []expression.Feature{{ID: "g1", Name: "Shared", Type: "Gene Expression"}, {ID: "g2", Name: "Shared", Type: "Gene Expression"}, {ID: "ab1", Name: "抗体", Type: "Antibody Capture"}}, Indptr: []int{0, 2, 3, 3}, Indices: []uint32{0, 2, 1}, Counts: []uint64{16777217, 7, 3}}, Cells: &expression.CellAnnotations{ByBarcode: map[string]int{"β-1": 0}, Rows: []expression.CellAnnotation{{Barcode: "β-1", Position: &b}}}}
	in.AnnData = &expression.AnnDataEvidence{VariableColumns: []string{"class / source", "nullable"}, VariableValues: [][]string{{"gene", ""}, {"gene", "é"}, {"protein", "yes"}}}
	p := &phenotype.Result{Sample: "S01", Columns: []string{"state / supplied"}, Cells: []phenotype.Cell{{Barcode: "B-1", Values: []string{""}, Position: &b, Status: "Ambiguous clonotype", Chains: "Multi-chain", Candidates: []string{"c1", "c2"}}, {Barcode: "ZERO-1", Values: []string{"Missing annotation"}, Status: "No receptor evidence"}, {Barcode: "β-1", Values: []string{"Mémoire"}, Position: &a, Clone: "c1", Size: &size, Status: "Assigned", Chains: "Paired", Bin: "2"}}}
	m := &expression.AnalysisResult{InputCells: 3, Components: 2, Cells: []expression.AnalysisCell{{Barcode: "B-1", Scores: []float64{3, 4}, Position: &b}, {Barcode: "β-1", Scores: []float64{-1, 2}, Position: &a}}, Excluded: []expression.AnalysisCell{{Barcode: "ZERO-1", Reason: "no positive gene counts"}}}
	return in, p, m
}
func TestExportFixture(t *testing.T) {
	in, p, m := fixture()
	raw, err := Export(context.Background(), in, p, m, map[string]string{"provenance_json": "{\"sample\":\"S01\",\"definition\":\"paired\"}"})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CLONOLOOM_TEST_ANNDATA_EXPORT") == "" {
		ref, err := os.ReadFile("../../validation/reference/anndata_export.json")
		if err != nil {
			t.Fatal(err)
		}
		var oracle struct{ SHA256 string }
		if err = json.Unmarshal(ref, &oracle); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != oracle.SHA256 {
			t.Fatal("export differs from independently read AnnData fixture")
		}
	}
	raw2, err := Export(context.Background(), in, p, m, map[string]string{"provenance_json": "{\"sample\":\"S01\",\"definition\":\"paired\"}"})
	if err != nil || !reflect.DeepEqual(raw, raw2) {
		t.Fatal("nondeterministic export", err)
	}
	path := filepath.Join(t.TempDir(), "export.h5ad")
	if out := os.Getenv("CLONOLOOM_TEST_ANNDATA_EXPORT"); out != "" {
		path = out
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	back, err := expression.OpenSelected(context.Background(), path, "", &expression.AnnDataSelection{Layer: "X", Embedding: "X_clonoloom_umap", FeatureName: "feature_name", FeatureType: "feature_type"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Matrix.Barcodes, in.Matrix.Barcodes) || !reflect.DeepEqual(back.Matrix.Features, in.Matrix.Features) || !reflect.DeepEqual(back.Matrix.Counts, in.Matrix.Counts) || !reflect.DeepEqual(back.Matrix.Indices, in.Matrix.Indices) || !reflect.DeepEqual(back.Matrix.Indptr, in.Matrix.Indptr) {
		t.Fatal("counts, IDs or modalities changed")
	}
	if back.Cells.Rows[2].Position != nil || *back.Cells.Rows[0].Position != *p.Cells[2].Position {
		t.Fatal("map alignment or missingness changed")
	}
}
func TestExportRejectsInconsistentEvidence(t *testing.T) {
	for _, change := range []func(*expression.Input, *phenotype.Result, *expression.AnalysisResult){
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			in.Matrix.Barcodes[1] = "β-1"
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			in.Matrix.Counts[0] = expression.MaxCount + 1
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) { in.Matrix.Indptr[1] = 4 },
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			p.Cells[0].Barcode = "wrong"
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) { m.Cells[0].Scores = nil },
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			m.Cells[0].Position = &[2]float64{0, 0}
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			m.Excluded[0].Barcode = "wrong"
		},
	} {
		in, p, m := fixture()
		change(in, p, m)
		if _, err := Export(context.Background(), in, p, m, nil); err == nil {
			t.Fatal("accepted inconsistent evidence")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	in, p, m := fixture()
	if _, err := Export(ctx, in, p, m, nil); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestExportBoundsAndWideAnnotations(t *testing.T) {
	for _, change := range []func(*expression.Input, *phenotype.Result, *expression.AnalysisResult){
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			in.Matrix.Barcodes = make([]string, phenotype.MaxCells+1)
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			in.Matrix.Counts = make([]uint64, MaxEntries+1)
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			p.Columns = make([]string, 33)
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			in.AnnData.VariableColumns = make([]string, 127)
		},
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) { m.Components = 1001 },
		func(in *expression.Input, p *phenotype.Result, m *expression.AnalysisResult) {
			in.Cells.ByBarcode["β-1"] = 999
		},
	} {
		in, p, m := fixture()
		change(in, p, m)
		if _, err := Export(context.Background(), in, p, m, nil); err == nil {
			t.Fatal("accepted unsupported dimensions")
		}
	}
	in, p, m := fixture()
	p.Columns = make([]string, 32)
	for i := range p.Columns {
		p.Columns[i] = "clonoloom_clone_id"
	}
	for i := range p.Cells {
		p.Cells[i].Values = make([]string, 32)
		p.Cells[i].Values[31] = "literal"
	}
	raw, err := Export(context.Background(), in, p, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wide.h5ad")
	os.WriteFile(path, raw, 0600)
	back, err := expression.OpenSelected(context.Background(), path, "", &expression.AnnDataSelection{Layer: "X", FeatureName: "feature_name", FeatureType: "feature_type"})
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Cells.Coverage.Columns) != 40 {
		t.Fatal("generated and supplied columns collide")
	}
}
func TestExportSuppliedCoordinatesAndNoReceptors(t *testing.T) {
	in, p, _ := fixture()
	for i := range p.Cells {
		c := &p.Cells[i]
		c.Position = nil
		c.Clone = ""
		c.Size = nil
		c.Status = "No receptor evidence"
		c.Chains = "No receptor evidence"
		c.Bin = ""
	}
	p.Cells[2].Position = in.Cells.Rows[0].Position
	raw, err := Export(context.Background(), in, p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "supplied.h5ad")
	os.WriteFile(path, raw, 0600)
	back, err := expression.OpenSelected(context.Background(), path, "", &expression.AnnDataSelection{Layer: "X", Embedding: "X_supplied", FeatureName: "feature_name", FeatureType: "feature_type"})
	if err != nil {
		t.Fatal(err)
	}
	if back.Cells.Rows[0].Position == nil || back.Cells.Rows[1].Position != nil {
		t.Fatal("supplied coordinate missingness")
	}
	p.Cells[0].Position = &[2]float64{0, 0}
	if _, err = Export(context.Background(), in, p, nil, nil); err == nil {
		t.Fatal("invented supplied coordinate")
	}
}

package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/qc"
	"clonoloom/internal/reads"
)

func TestPhenotypesAgreeWithIndependentSQL(t *testing.T) {
	var ref struct {
		Sources map[string]string
		Inputs  map[string][]phenotype.Cell
		Results []struct {
			Sample, Population string
			Cells              []struct {
				Barcode string
				Clone   *string
				Size    *int
				Bin     string
			}
			Groups       []phenotype.Group
			Associations []phenotype.Association
		}
	}
	raw, err := os.ReadFile("reference/phenotypes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "testdata", "phenotypes")
	for name, want := range ref.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("reference source changed", name)
		}
	}
	ds, err := reads.OpenInputsWith(context.Background(), []reads.Input{{Path: filepath.Join(dir, "S1.tsv")}, {Path: filepath.Join(dir, "S2.tsv")}}, reads.ReadInputs)
	if err != nil {
		t.Fatal(err)
	}
	f := qc.Filter{ProductiveOnly: true}
	keep, _ := qc.Apply(ds.Table, f)
	ct, err := clonotype.Compute(ds.Table, keep, clonotype.Definition{Chains: []string{"TRB"}})
	if err != nil {
		t.Fatal(err)
	}
	junction := map[string]string{}
	for _, c := range ct.Clonotypes {
		junction[c.ID] = c.Chains[0].Junction
	}
	for _, want := range ref.Results {
		sample := -1
		for i, s := range ds.Table.Samples {
			if s.Name == want.Sample {
				sample = i
			}
		}
		if sample < 0 {
			t.Fatal(ds.Table.Samples)
		}
		got, err := phenotype.Compute(context.Background(), ds.Table, ct, f, sample, ref.Inputs[want.Sample], []string{"state"}, phenotype.Options{Population: want.Population, Annotation: "state"})
		if err != nil {
			t.Fatal(err)
		}
		cells := map[string]phenotype.Cell{}
		for _, c := range got.Cells {
			cells[c.Barcode] = c
		}
		for _, w := range want.Cells {
			g := cells[w.Barcode]
			clone := ""
			if w.Clone != nil {
				clone = *w.Clone
			}
			if junction[g.Clone] != clone || !reflect.DeepEqual(g.Size, w.Size) || g.Bin != w.Bin {
				t.Fatalf("%s/%s %s got %+v want %+v", want.Sample, want.Population, w.Barcode, g, w)
			}
		}
		for i, w := range want.Groups {
			g := got.Groups[i]
			if g.Value != w.Value || g.Cells != w.Cells || g.Mapped != w.Mapped || g.Assigned != w.Assigned || g.Sized != w.Sized || g.Expanded != w.Expanded || g.Clonotypes != w.Clonotypes {
				t.Fatal(g, w)
			}
			if w.Sized > 0 && (g.ExpandedShare == nil || *g.ExpandedShare != float64(w.Expanded)/float64(w.Sized)) {
				t.Fatal("expansion denominator", g, w)
			}
		}
		found := map[[2]string]phenotype.Association{}
		for _, g := range got.Associations {
			found[[2]string{junction[g.Clone], g.Value}] = g
		}
		if len(found) != len(want.Associations) {
			t.Fatal("association count")
		}
		for _, w := range want.Associations {
			g := found[[2]string{w.Clone, w.Value}]
			if g.Cells != w.Cells || g.Mapped != w.Mapped || g.CloneExpression != w.CloneExpression || g.GroupExpression != w.GroupExpression || g.CloneShare != float64(w.Cells)/float64(w.CloneExpression) || g.GroupShare != float64(w.Cells)/float64(w.GroupExpression) {
				t.Fatal(g, w)
			}
		}
	}
}

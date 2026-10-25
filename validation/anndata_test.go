package validation

import (
	"clonoloom/internal/expression"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAnnDataIndependentReferences(t *testing.T) {
	var ref struct {
		Valid map[string]struct {
			ObservationColumns []string      `json:"obs_columns"`
			ObservationValues  [][]string    `json:"obs_values"`
			VariableColumns    []string      `json:"var_columns"`
			VariableValues     [][]string    `json:"var_values"`
			Positions          []*[2]float64 `json:"positions"`
			Entries            int           `json:"entries"`

			Selection          expression.AnnDataSelection
			SHA256             string
			Barcodes, Features []string
			Dense              [][]uint64
			GeneUMIs           []uint64 `json:"gene_umis"`
			Genes              []int
			AIRRBarcodes       []string `json:"airr_barcodes"`
			AIRR               [][]map[string]any
		}
		Invalid map[string]struct {
			Selection expression.AnnDataSelection
			Error     string
		}
	}
	raw, e := os.ReadFile("reference/anndata.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &ref); e != nil {
		t.Fatal(e)
	}
	for name, w := range ref.Valid {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "testdata", "anndata", name)
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			sum := sha256.Sum256(raw)
			if hex.EncodeToString(sum[:]) != w.SHA256 {
				t.Fatal("independent source hash changed")
			}
			in, e := expression.OpenSelected(context.Background(), path, "", &w.Selection)
			if e != nil {
				t.Fatal(e)
			}
			m := in.Matrix
			if !reflect.DeepEqual(m.Barcodes, w.Barcodes) || !reflect.DeepEqual(m.GeneUMIs, w.GeneUMIs) || !reflect.DeepEqual(m.Genes, w.Genes) {
				t.Fatalf("counts/barcodes differ: %+v", m)
			}
			dense := make([][]uint64, len(m.Barcodes))
			for c := range dense {
				dense[c] = make([]uint64, len(m.Features))
				for j := m.Indptr[c]; j < m.Indptr[c+1]; j++ {
					dense[c][m.Indices[j]] = m.Counts[j]
				}
			}
			if !reflect.DeepEqual(dense, w.Dense) {
				t.Fatal("dense values differ", dense, w.Dense)
			}
			for i, f := range m.Features {
				if f.ID != w.Features[i] {
					t.Fatal("feature index differs")
				}
			}
			if m.Entries != w.Entries || !reflect.DeepEqual(in.Cells.Coverage.Columns, w.ObservationColumns) || !reflect.DeepEqual(in.AnnData.VariableColumns, w.VariableColumns) || !reflect.DeepEqual(in.AnnData.VariableValues, w.VariableValues) {
				t.Fatal("annotation schema/values or entries differ")
			}
			for i, row := range in.Cells.Rows {
				if !reflect.DeepEqual(row.Values, w.ObservationValues[i]) || !reflect.DeepEqual(row.Position, w.Positions[i]) {
					t.Fatalf("observation/map row %d differs: %+v", i, row)
				}
			}
			a := in.AnnData.AIRR
			at := 0
			for i, chains := range w.AIRR {
				for j, chain := range chains {
					if at >= len(a.Rows) {
						t.Fatal("missing chain")
					}
					row := a.Rows[at]
					at++
					if row.Barcode != w.AIRRBarcodes[i] || row.Chain != j+1 {
						t.Fatal("AIRR row alignment")
					}
					for k, field := range a.Fields {
						v := chain[field]
						got := row.Values[k]
						if v == nil {
							if got != nil {
								t.Fatal("missing AIRR field replaced")
							}
						} else if got == nil || *got != fmt.Sprint(v) {
							t.Fatalf("AIRR field %s: %v != %v", field, got, v)
						}
					}
				}
			}
			if at != a.Chains {
				t.Fatal("extra AIRR chains")
			}
			if in.AnnData.Embedding.Axes != [2]string{"UMAP 1", "UMAP 2"} {
				t.Fatal("axis provenance")
			}
			if len(m.Barcodes) > 0 {
				if in.Cells.Rows[1].Values[0] != "" || in.Cells.Rows[1].Values[1] != "" || in.Cells.Rows[1].Values[3] != "" {
					t.Fatal("missing annotations changed")
				}
				if in.Cells.Coverage.Mapped != 3 {
					t.Fatal("missing coordinates changed")
				}
			}
		})
	}
	for name, w := range ref.Invalid {
		t.Run(name, func(t *testing.T) {
			_, e := expression.OpenSelected(context.Background(), filepath.Join("..", "testdata", "anndata", name), "", &w.Selection)
			if e == nil || !strings.Contains(e.Error(), w.Error) {
				t.Fatalf("want %q; got %v", w.Error, e)
			}
		})
	}
}

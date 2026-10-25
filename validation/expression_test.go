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

	"clonoloom/internal/expression"
	"clonoloom/internal/reads"
)

func TestExpressionAgreesWithSciPy(t *testing.T) {
	var ref struct {
		Sources        map[string]string `json:"sources"`
		Matrix         [][]uint64        `json:"featureByCell"`
		UMIs           []uint64          `json:"geneUMIs"`
		Genes          []int             `json:"genes"`
		Matched        int               `json:"matched"`
		ExpressionOnly int               `json:"expressionOnly"`
		ReceptorOnly   int               `json:"receptorOnly"`
		Missing        int               `json:"missingBarcodeRows"`
	}
	raw, err := os.ReadFile("reference/expression.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "testdata", "expression-mex")
	for name, want := range ref.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("reference source changed: %s", name)
		}
	}
	input, err := expression.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	m := input.Matrix
	if !reflect.DeepEqual(m.GeneUMIs, ref.UMIs) || !reflect.DeepEqual(m.Genes, ref.Genes) {
		t.Fatal(m.GeneUMIs, m.Genes)
	}
	dense := make([][]uint64, len(m.Features))
	for f := range dense {
		dense[f] = make([]uint64, len(m.Barcodes))
	}
	for c := range m.Barcodes {
		for i := m.Indptr[c]; i < m.Indptr[c+1]; i++ {
			dense[m.Indices[i]][c] = m.Counts[i]
		}
	}
	if !reflect.DeepEqual(dense, ref.Matrix) {
		t.Fatal(dense)
	}
	repertoire, err := reads.OpenInputsWith(context.Background(), []reads.Input{{Path: filepath.Join(dir, "receptors.tsv")}}, reads.ReadInputs)
	if err != nil {
		t.Fatal(err)
	}
	links, err := expression.Link(context.Background(), m, repertoire.Table, 0)
	if err != nil || links.Matched != ref.Matched || links.ExpressionOnly != ref.ExpressionOnly || links.ReceptorOnly != ref.ReceptorOnly || links.MissingBarcodeRows != ref.Missing {
		t.Fatal(links, err)
	}
}

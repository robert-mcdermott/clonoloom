package expression

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnnDataIdentityAndMissingEvidence(t *testing.T) {
	sel := AnnDataSelection{Layer: "layers/counts", Embedding: "X_umap", AIRRKey: "airr", FeatureName: "gene_name", FeatureType: "feature_type"}
	path := "../../testdata/anndata/optional-cell.h5ad"
	in, e := OpenSelected(context.Background(), path, "", &sel)
	if e != nil {
		t.Fatal(e)
	}
	if in.AnnData.AIRR.MissingCells != 1 || in.AnnData.AIRR.CellRows[1].Chains != nil || in.AnnData.AIRR.CellRows[3].Chains == nil || *in.AnnData.AIRR.CellRows[3].Chains != 0 {
		t.Fatal("missing versus empty chain list lost")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	moved := filepath.Join(t.TempDir(), "moved.h5ad")
	if e = os.WriteFile(moved, raw, 0600); e != nil {
		t.Fatal(e)
	}
	copy, e := OpenSelected(context.Background(), moved, "", &sel)
	if e != nil || copy.ID != in.ID {
		t.Fatal(e, "relocation changed identity")
	}
	sel.Embedding = "X_tsne"
	other, e := OpenSelected(context.Background(), path, "", &sel)
	if e != nil || other.ID == in.ID || other.AnnData.Embedding.Axes[0] != "t-SNE 1" {
		t.Fatal(e, "embedding selection not pinned")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = OpenSelected(ctx, path, "", &sel)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestMuDataRequiresExplicitSelections(t *testing.T) {
	path := "../../testdata/anndata/multimodal.h5mu"
	for _, s := range []AnnDataSelection{{Layer: "X"}, {Modality: "gex"}, {Modality: "gex", Layer: "X", AIRRKey: "airr"}, {Modality: "gex", Layer: "X", Embedding: "../X"}, {Modality: "gex", Layer: "X", AIRRModality: "receptors"}, {Modality: "missing", Layer: "X"}} {
		if _, e := OpenSelected(context.Background(), path, "", &s); e == nil {
			t.Fatal("accepted ambiguous/invalid selection", s)
		}
	}
	s := AnnDataSelection{Modality: "gex", Layer: "X", AIRRKey: "airr", AIRRModality: "receptors"}
	in, e := OpenSelected(context.Background(), path, "", &s)
	if e != nil {
		t.Fatal(e)
	}
	a := in.AnnData.AIRR
	if a.Matched != 2 || a.AIRROnly != 1 || a.ExpressionOnly != 2 || a.Rows[0].Barcode != "CELL3" {
		t.Fatal(a)
	}
}
func FuzzAnnData(f *testing.F) {
	for _, name := range []string{"gzip-csr.h5ad", "multimodal.h5mu", "empty.h5ad"} {
		b, e := os.ReadFile("../../testdata/anndata/" + name)
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2<<20 {
			t.Skip()
		}
		_, _ = ReadAnnData(context.Background(), bytes.NewReader(b), int64(len(b)), AnnDataSelection{Layer: "layers/counts", Embedding: "X_umap", AIRRKey: "airr"})
	})
}

func TestAnnDataTruncation(t *testing.T) {
	b, e := os.ReadFile("../../testdata/anndata/gzip-csr.h5ad")
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{0, 8, 12, 48, 512, len(b) / 2, len(b) - 1} {
		_, e := ReadAnnData(context.Background(), bytes.NewReader(b[:n]), int64(n), AnnDataSelection{Layer: "layers/counts", Embedding: "X_umap", AIRRKey: "airr"})
		if e == nil {
			t.Fatalf("truncated file accepted at %d", n)
		}
	}
}

func TestAnnDataDoesNotReplaceSourceObservationsWithTSV(t *testing.T) {
	path := "../../testdata/anndata/gzip-csr.h5ad"
	tsv := filepath.Join(t.TempDir(), "cells.tsv")
	if e := os.WriteFile(tsv, []byte("barcode\tstate\nCELL1\treplacement\n"), 0600); e != nil {
		t.Fatal(e)
	}
	_, e := OpenSelected(context.Background(), path, tsv, &AnnDataSelection{Layer: "X"})
	if e == nil || !strings.Contains(e.Error(), "omit the external cell TSV") {
		t.Fatal("source annotations could be replaced", e)
	}
}

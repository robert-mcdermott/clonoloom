package validation

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/expression"
)

func TestExpressionHDF5AgreesWithH5pyAndSciPy(t *testing.T) {
	var ref struct {
		Valid map[string]struct {
			SHA256   string               `json:"sha256"`
			Matrix   [][]uint64           `json:"featureByCell"`
			UMIs     []uint64             `json:"geneUMIs"`
			Genes    []int                `json:"genes"`
			Entries  int                  `json:"entries"`
			Barcodes []string             `json:"barcodes"`
			Features []expression.Feature `json:"features"`
		} `json:"valid"`
		Invalid map[string]string `json:"invalid"`
	}
	raw, err := os.ReadFile("reference/expression_hdf5.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "testdata", "expression-hdf5")
	for name, want := range ref.Valid {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(raw)
			if hex.EncodeToString(hash[:]) != want.SHA256 {
				t.Fatal("independent source changed")
			}
			in, err := expression.Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			m := in.Matrix
			if !reflect.DeepEqual(m.Features, want.Features) || !reflect.DeepEqual(m.Barcodes, want.Barcodes) || !reflect.DeepEqual(m.GeneUMIs, want.UMIs) || !reflect.DeepEqual(m.Genes, want.Genes) || m.Entries != want.Entries {
				t.Fatal("metadata/totals disagree", m)
			}
			dense := make([][]uint64, len(m.Features))
			for i := range dense {
				dense[i] = make([]uint64, len(m.Barcodes))
			}
			for c := range m.Barcodes {
				for i := m.Indptr[c]; i < m.Indptr[c+1]; i++ {
					dense[m.Indices[i]][c] = m.Counts[i]
				}
			}
			if !reflect.DeepEqual(dense, want.Matrix) {
				t.Fatal("HDF5 values disagree with h5py/SciPy")
			}
		})
	}
	for name, message := range ref.Invalid {
		t.Run(name, func(t *testing.T) {
			_, err := expression.Open(context.Background(), filepath.Join(dir, name))
			if err == nil || !strings.Contains(err.Error(), message) {
				t.Fatalf("wanted %q, got %v", message, err)
			}
		})
	}
}

func TestExpressionHDF5PublicReference(t *testing.T) {
	path, reference := os.Getenv("CLONOLOOM_HDF5"), os.Getenv("CLONOLOOM_HDF5_REFERENCE")
	if path == "" && reference == "" {
		t.Skip("optional maintainer-selected public HDF5 source/reference")
	}
	if path == "" || reference == "" {
		t.Fatal("set both CLONOLOOM_HDF5 and CLONOLOOM_HDF5_REFERENCE")
	}
	var ref struct {
		Source                                                     string
		Features, Cells, Entries                                   int
		Counts, Indices, Indptr, GeneUMIs, Genes, Labels, Barcodes string
	}
	raw, err := os.ReadFile(reference)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	in, err := expression.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	m := in.Matrix
	if in.Sources[0].SHA256 != ref.Source || len(m.Features) != ref.Features || len(m.Barcodes) != ref.Cells || m.Entries != ref.Entries {
		t.Fatal("public source or dimensions differ")
	}
	digest := func(want string, put func(func([]byte))) {
		t.Helper()
		h := sha256.New()
		put(func(b []byte) { h.Write(b) })
		if hex.EncodeToString(h.Sum(nil)) != want {
			t.Fatal("independent public HDF5 value hash differs")
		}
	}
	digest(ref.Counts, func(put func([]byte)) {
		for _, v := range m.Counts {
			put(binary.LittleEndian.AppendUint64(nil, v))
		}
	})
	digest(ref.Indices, func(put func([]byte)) {
		for _, v := range m.Indices {
			put(binary.LittleEndian.AppendUint32(nil, v))
		}
	})
	digest(ref.Indptr, func(put func([]byte)) {
		for _, v := range m.Indptr {
			put(binary.LittleEndian.AppendUint64(nil, uint64(v)))
		}
	})
	digest(ref.GeneUMIs, func(put func([]byte)) {
		for _, v := range m.GeneUMIs {
			put(binary.LittleEndian.AppendUint64(nil, v))
		}
	})
	digest(ref.Genes, func(put func([]byte)) {
		for _, v := range m.Genes {
			put(binary.LittleEndian.AppendUint64(nil, uint64(v)))
		}
	})
	digest(ref.Labels, func(put func([]byte)) {
		for _, v := range m.Features {
			for _, s := range []string{v.ID, v.Name, v.Type} {
				put([]byte(s + "\x00"))
			}
		}
	})
	digest(ref.Barcodes, func(put func([]byte)) {
		for _, s := range m.Barcodes {
			put([]byte(s + "\x00"))
		}
	})
}

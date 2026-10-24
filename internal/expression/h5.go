package expression

// 10x's feature-barcode HDF5 contract is CSC (features by cells), not a dense matrix:
// https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/outputs/cr-outputs-h5-matrices
// Exact counts, duplicate rejection and bounded resources follow the MEX reader's policies.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"

	"clonoloom/internal/hdf5"
)

func Read10xHDF5(ctx context.Context, r io.ReaderAt, size int64) (*Matrix, error) {
	f, err := hdf5.Open(ctx, r, size)
	if err != nil {
		return nil, err
	}
	shape, err := f.Integers("/matrix/shape", 2)
	if err != nil {
		return nil, err
	}
	if len(shape) != 2 || shape[0] > MaxFeatures || shape[1] > MaxCells {
		return nil, fmt.Errorf("10x HDF5 shape must be [features, cells], within 200000 features and 2000000 cells")
	}
	ids, err := f.Strings("/matrix/features/id", shape[0])
	if err != nil {
		return nil, err
	}
	names, err := f.Strings("/matrix/features/name", shape[0])
	if err != nil {
		return nil, err
	}
	types, err := f.Strings("/matrix/features/feature_type", shape[0])
	if err != nil {
		return nil, err
	}
	barcodes, err := f.Strings("/matrix/barcodes", shape[1])
	if err != nil {
		return nil, err
	}
	if len(ids) != int(shape[0]) || len(names) != len(ids) || len(types) != len(ids) || len(barcodes) != int(shape[1]) {
		return nil, fmt.Errorf("10x HDF5 shape does not match feature/barcode lengths")
	}
	m := &Matrix{Barcodes: barcodes, Features: make([]Feature, len(ids))}
	seen := map[string]bool{}
	labelBytes := 0
	for i, id := range ids {
		if !validLabel(id) || !validLabel(names[i]) || !validLabel(types[i]) || seen[id] {
			return nil, fmt.Errorf("10x HDF5 invalid feature field or duplicate ID at %d", i)
		}
		seen[id] = true
		labelBytes += len(id) + len(names[i]) + len(types[i])
		if labelBytes > maxLabelBytes {
			return nil, fmt.Errorf("10x HDF5 feature labels exceed 64 MiB")
		}
		m.Features[i] = Feature{id, names[i], types[i]}
	}
	seen = map[string]bool{}
	for _, b := range barcodes {
		if !validLabel(b) || seen[b] {
			return nil, fmt.Errorf("10x HDF5 invalid or duplicate barcode")
		}
		seen[b] = true
	}
	pointers, err := f.Integers("/matrix/indptr", shape[1]+1)
	if err != nil {
		return nil, err
	}
	if len(pointers) != len(barcodes)+1 || pointers[0] != 0 {
		return nil, fmt.Errorf("10x HDF5 indptr must have cells+1 entries and start at zero")
	}
	entries := pointers[len(pointers)-1]
	if entries > MaxEntries {
		return nil, fmt.Errorf("10x HDF5 exceeds 10000000 coordinates")
	}
	m.Indptr = make([]int, len(pointers))
	for i, p := range pointers {
		if p > entries || (i > 0 && p < pointers[i-1]) {
			return nil, fmt.Errorf("10x HDF5 indptr is not monotone within the stored entries")
		}
		m.Indptr[i] = int(p)
	}
	indices, err := f.Integers("/matrix/indices", entries)
	if err != nil {
		return nil, err
	}
	counts, err := f.Integers("/matrix/data", entries)
	if err != nil {
		return nil, err
	}
	if uint64(len(indices)) != entries || len(counts) != len(indices) {
		return nil, fmt.Errorf("10x HDF5 CSC arrays disagree with indptr")
	}
	m.Entries = int(entries)
	m.Indices = make([]uint32, len(indices))
	m.Counts = counts
	m.GeneUMIs = make([]uint64, len(barcodes))
	m.Genes = make([]int, len(barcodes))
	for i, index := range indices {
		if index >= shape[0] || counts[i] > MaxCount {
			return nil, fmt.Errorf("10x HDF5 index outside features or count exceeds 2^53-1")
		}
		m.Indices[i] = uint32(index)
	}
	// CSC writers may leave feature indices unsorted. Sort paired values per cell, rejecting
	// duplicates (including explicit zeros) rather than silently summing them.
	for cell := range barcodes {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		lo, hi := m.Indptr[cell], m.Indptr[cell+1]
		if hi-lo > len(ids) {
			return nil, fmt.Errorf("10x HDF5 duplicate coordinates")
		}
		block := cscBlock{m.Indices[lo:hi], m.Counts[lo:hi]}
		sort.Sort(block)
		for i := lo; i < hi; i++ {
			if i > lo && m.Indices[i] == m.Indices[i-1] {
				return nil, fmt.Errorf("10x HDF5 duplicate coordinate for barcode %q", barcodes[cell])
			}
			if m.Features[m.Indices[i]].Type == "Gene Expression" {
				v := m.Counts[i]
				if v > MaxCount-m.GeneUMIs[cell] {
					return nil, fmt.Errorf("10x HDF5 gene-expression UMI total exceeds 2^53-1")
				}
				m.GeneUMIs[cell] += v
				if v > 0 {
					m.Genes[cell]++
				}
			}
		}
	}
	return m, ctx.Err()
}

type cscBlock struct {
	indices []uint32
	counts  []uint64
}

func (b cscBlock) Len() int           { return len(b.indices) }
func (b cscBlock) Less(i, j int) bool { return b.indices[i] < b.indices[j] }
func (b cscBlock) Swap(i, j int) {
	b.indices[i], b.indices[j] = b.indices[j], b.indices[i]
	b.counts[i], b.counts[j] = b.counts[j], b.counts[i]
}

func openHDF5(ctx context.Context, path string) (*Input, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > hdf5.MaxFile {
		return nil, fmt.Errorf("HDF5 source must be a regular file no larger than 1 GiB")
	}
	// Hash before and after random-access parsing. A concurrently changed source is never
	// associated with a matrix fingerprint from a different read of its bytes.
	fingerprint := func() (string, error) {
		h := sha256.New()
		n, e := io.Copy(h, contextReader{ctx, io.NewSectionReader(f, 0, stat.Size()+1)})
		if e != nil {
			return "", e
		}
		if n != stat.Size() {
			return "", fmt.Errorf("HDF5 source changed size while reading")
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	before, err := fingerprint()
	if err != nil {
		return nil, err
	}
	m, err := Read10xHDF5(ctx, f, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	after, err := fingerprint()
	if err != nil {
		return nil, err
	}
	now, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if before != after || now.Size() != stat.Size() || !now.ModTime().Equal(stat.ModTime()) {
		return nil, fmt.Errorf("HDF5 source changed while reading; reopen it")
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("clonoloom-expression-hdf5/1\n%d\n%s", stat.Size(), before)))
	return &Input{ID: "x-" + hex.EncodeToString(h[:]), Path: path, Sources: []Source{{Role: "matrix-hdf5", Path: path, Size: stat.Size(), SHA256: before}}, Matrix: m}, nil
}

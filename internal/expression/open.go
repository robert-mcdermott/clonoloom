package expression

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
)

const ReaderVersion = 1

type Source struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Input struct {
	ID      string           `json:"id"`
	Path    string           `json:"path"`
	Sources []Source         `json:"sources"`
	Matrix  *Matrix          `json:"-"`
	Cells   *CellAnnotations `json:"-"`
}

// Open reads just the selected MEX directory. It never chooses between raw and filtered data,
// walks a tree, writes to the source, or silently chooses one of two possible companions.
func Open(ctx context.Context, dir string) (*Input, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, fmt.Errorf("choose a MEX directory")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	in := &Input{Path: abs}
	roles := []string{"matrix", "features", "barcodes"}
	candidates := [][]string{{"matrix.mtx", "matrix.mtx.gz"}, {"features.tsv", "features.tsv.gz", "genes.tsv", "genes.tsv.gz"}, {"barcodes.tsv", "barcodes.tsv.gz"}}
	readers := make([]io.Reader, 3)
	hashes := make([]hash.Hash, 3)
	limits := make([]*io.LimitedReader, 3)
	legacy := false
	for i, names := range candidates {
		var path string
		for _, name := range names {
			p := filepath.Join(abs, name)
			info, e := os.Stat(p)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return nil, e
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("%s is not a regular file", p)
			}
			if path != "" {
				return nil, fmt.Errorf("%s: ambiguous %s companions", abs, roles[i])
			}
			path = p
		}
		if path == "" {
			return nil, fmt.Errorf("%s: missing %s companion", abs, roles[i])
		}
		f, e := os.Open(path)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		stat, e := f.Stat()
		if e != nil {
			return nil, e
		}
		if !stat.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file", path)
		}
		if stat.Size() > 1<<30 {
			return nil, fmt.Errorf("%s exceeds the 1 GiB source limit", path)
		}
		h := sha256.New()
		hashes[i] = h
		limits[i] = &io.LimitedReader{R: f, N: stat.Size() + 1}
		readers[i] = io.TeeReader(limits[i], h)
		in.Sources = append(in.Sources, Source{Role: roles[i], Path: path, Size: stat.Size()})
		if i == 1 {
			name := filepath.Base(path)
			legacy = name == "genes.tsv" || name == "genes.tsv.gz"
		}
	}
	in.Matrix, err = ReadMEX(ctx, readers[0], readers[1], readers[2], legacy, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}
	// The identity is positional by role and covers actual bytes, including compression. Relocation
	// preserves identity; a changed companion, including its modality or order, changes identity.
	h := sha256.New()
	fmt.Fprintf(h, "clonoloom-expression/%d legacy=%t\n", ReaderVersion, legacy)
	for i := range in.Sources {
		s := &in.Sources[i]
		if limits[i].N != 1 {
			return nil, fmt.Errorf("%s changed size while reading; reopen it", s.Path)
		}
		s.SHA256 = hex.EncodeToString(hashes[i].Sum(nil))
		raw, _ := json.Marshal(struct {
			Role, SHA256 string
			Size         int64
		}{s.Role, s.SHA256, s.Size})
		h.Write(raw)
	}
	in.ID = "x-" + hex.EncodeToString(h.Sum(nil))
	return in, nil
}

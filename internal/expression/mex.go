// Package expression reads sparse single-cell expression evidence independently of receptor rows.
// Format references: 10x Genomics, Cell Ranger Feature Barcode Matrices (MEX Format),
// https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/cr-outputs-mex-matrices;
// Boisvert et al. (1996), Matrix Market Exchange Formats, NISTIR 5935,
// https://math.nist.gov/MatrixMarket/formats.html.
// The cell-compressed layout follows the sparse approach in SituCraft's internal/xenium;
// this streaming reader is ClonoLoom's own implementation. Strict duplicate rejection and
// exact nonnegative integer counts are ClonoLoom policy, not general Matrix Market rules.
package expression

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxFeatures          = 200000
	MaxCells             = 2000000
	MaxEntries           = 10000000
	MaxCount      uint64 = 1<<53 - 1 // exact in JSON consumers, including per-cell sums
	maxTextBytes         = 512 << 20
	maxLineBytes         = 1 << 20
	maxLabelBytes        = 64 << 20
)

type Feature struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Matrix is compressed by cell, with ascending feature indices within each cell. An absent
// coordinate is a measured zero only within this matrix; an absent barcode is not a zero cell.
type Matrix struct {
	Features []Feature
	Barcodes []string
	Indptr   []int
	Indices  []uint32
	Counts   []uint64
	GeneUMIs []uint64
	Genes    []int
	Entries  int // coordinates in the file, including explicit zero values
}

type lineReader struct {
	ctx   context.Context
	scan  *bufio.Scanner
	limit *io.LimitedReader
	name  string
	n     int
	close func() error
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func newLines(ctx context.Context, r io.Reader, name string) (*lineReader, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	br := bufio.NewReader(contextReader{ctx, r})
	magic, _ := br.Peek(2)
	var close func() error
	var input io.Reader = br
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		input, close = gz, gz.Close
	}
	limit := &io.LimitedReader{R: input, N: maxTextBytes + 1}
	scanner := bufio.NewScanner(limit)
	scanner.Buffer(make([]byte, 4096), maxLineBytes)
	return &lineReader{ctx: ctx, scan: scanner, limit: limit, name: name, close: close}, nil
}
func (l *lineReader) next() bool {
	if l.ctx.Err() != nil {
		return false
	}
	if !l.scan.Scan() {
		return false
	}
	l.n++
	return true
}
func (l *lineReader) err() error {
	if err := l.ctx.Err(); err != nil {
		return err
	}
	if l.limit.N == 0 {
		return fmt.Errorf("%s exceeds the 512 MiB decompressed limit", l.name)
	}
	if err := l.scan.Err(); err != nil {
		return fmt.Errorf("%s, near line %d: %w", l.name, l.n+1, err)
	}
	return nil
}
func (l *lineReader) bad(message string) error {
	return fmt.Errorf("%s, line %d: %s", l.name, l.n, message)
}
func validLabel(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && utf8.ValidString(s) && !strings.ContainsAny(s, "\x00\r\n\t")
}

// ReadMEX accepts plain or gzip streams. legacy means a two-column genes.tsv; current features
// require three columns. progress reports matrix coordinates consumed, never guessed percentages.
func ReadMEX(ctx context.Context, matrix, features, barcodes io.Reader, legacy bool, progress func(int)) (*Matrix, error) {
	m := &Matrix{}
	fl, err := newLines(ctx, features, "features")
	if err != nil {
		return nil, err
	}
	defer func() {
		if fl.close != nil {
			fl.close()
		}
	}()
	seen := map[string]bool{}
	bytes := 0
	for fl.next() {
		fields := strings.Split(fl.scan.Text(), "\t")
		want := 3
		if legacy {
			want = 2
		}
		if len(fields) != want {
			return nil, fl.bad(fmt.Sprintf("expected %d tab-separated fields", want))
		}
		for _, f := range fields {
			if !validLabel(f) {
				return nil, fl.bad("empty or invalid feature field")
			}
		}
		if seen[fields[0]] {
			return nil, fl.bad("duplicate feature ID " + strconv.Quote(fields[0]))
		}
		if len(m.Features) == MaxFeatures {
			return nil, fl.bad("more than 200,000 features")
		}
		bytes += len(fl.scan.Text())
		if bytes > maxLabelBytes {
			return nil, fl.bad("feature labels exceed 64 MiB")
		}
		seen[fields[0]] = true
		typ := "Gene Expression"
		if !legacy {
			typ = fields[2]
		}
		m.Features = append(m.Features, Feature{fields[0], fields[1], typ})
	}
	if err = fl.err(); err != nil {
		return nil, err
	}
	bl, err := newLines(ctx, barcodes, "barcodes")
	if err != nil {
		return nil, err
	}
	defer func() {
		if bl.close != nil {
			bl.close()
		}
	}()
	seen = map[string]bool{}
	bytes = 0
	for bl.next() {
		s := bl.scan.Text()
		if !validLabel(s) {
			return nil, bl.bad("expected one nonempty barcode without surrounding whitespace")
		}
		if seen[s] {
			return nil, bl.bad("duplicate barcode " + strconv.Quote(s))
		}
		if len(m.Barcodes) == MaxCells {
			return nil, bl.bad("more than 2,000,000 barcodes")
		}
		bytes += len(s)
		if bytes > maxLabelBytes {
			return nil, bl.bad("barcode labels exceed 64 MiB")
		}
		seen[s] = true
		m.Barcodes = append(m.Barcodes, s)
	}
	if err = bl.err(); err != nil {
		return nil, err
	}
	ml, err := newLines(ctx, matrix, "matrix")
	if err != nil {
		return nil, err
	}
	defer func() {
		if ml.close != nil {
			ml.close()
		}
	}()
	if !ml.next() {
		if err := ml.err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("matrix: missing Matrix Market header")
	}
	if ml.scan.Text() != "%%MatrixMarket matrix coordinate integer general" {
		return nil, ml.bad("expected %%MatrixMarket matrix coordinate integer general (raw integer counts)")
	}
	type entry struct {
		feature, cell uint32
		value         uint64
	}
	var entries []entry
	declared := -1
	for ml.next() {
		text := strings.TrimSpace(ml.scan.Text())
		if text == "" || strings.HasPrefix(text, "%") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) != 3 {
			return nil, ml.bad("expected three integers")
		}
		r, e1 := strconv.ParseUint(fields[0], 10, 64)
		c, e2 := strconv.ParseUint(fields[1], 10, 64)
		v, e3 := strconv.ParseUint(fields[2], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, ml.bad("invalid nonnegative integer: " + strconv.Quote(text))
		}
		if declared < 0 {
			if r != uint64(len(m.Features)) || c != uint64(len(m.Barcodes)) {
				return nil, ml.bad("dimensions do not match the feature and barcode files")
			}
			if v > MaxEntries {
				return nil, ml.bad("more than 10,000,000 coordinates")
			}
			declared = int(v)
			continue
		}
		if len(entries) >= declared {
			return nil, ml.bad("more coordinates than declared")
		}
		if r < 1 || c < 1 || r > uint64(len(m.Features)) || c > uint64(len(m.Barcodes)) {
			return nil, ml.bad("coordinate outside the matrix")
		}
		if v > MaxCount {
			return nil, ml.bad("count exceeds the exact integer limit (2^53-1)")
		}
		entries = append(entries, entry{uint32(r - 1), uint32(c - 1), v})
		if progress != nil && len(entries)%65536 == 0 {
			progress(len(entries))
		}
	}
	if err = ml.err(); err != nil {
		return nil, err
	}
	if declared < 0 || len(entries) != declared {
		return nil, fmt.Errorf("matrix: read %d coordinates; declared %d", len(entries), declared)
	}
	m.Entries = declared
	m.Indptr = make([]int, len(m.Barcodes)+1)
	for i, e := range entries {
		if i%65536 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		m.Indptr[int(e.cell)+1]++
	}
	for i := 1; i < len(m.Indptr); i++ {
		m.Indptr[i] += m.Indptr[i-1]
	}
	ordered := make([]entry, len(entries))
	next := append([]int(nil), m.Indptr...)
	for i, e := range entries {
		if i%65536 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		ordered[next[e.cell]] = e
		next[e.cell]++
	}
	entries = nil
	m.Indices = make([]uint32, 0, len(ordered))
	m.Counts = make([]uint64, 0, len(ordered))
	m.GeneUMIs = make([]uint64, len(m.Barcodes))
	m.Genes = make([]int, len(m.Barcodes))
	for cell := range m.Barcodes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		block := ordered[m.Indptr[cell]:m.Indptr[cell+1]]
		if len(block) > len(m.Features) {
			return nil, fmt.Errorf("matrix: too many coordinates for barcode %q (duplicate features)", m.Barcodes[cell])
		}
		sort.Slice(block, func(i, j int) bool { return block[i].feature < block[j].feature })
		for i, e := range block {
			if i > 0 && block[i-1].feature == e.feature {
				return nil, fmt.Errorf("matrix: duplicate coordinate for feature %q, barcode %q", m.Features[e.feature].ID, m.Barcodes[cell])
			}
			m.Indices = append(m.Indices, e.feature)
			m.Counts = append(m.Counts, e.value)
			if m.Features[e.feature].Type == "Gene Expression" {
				if e.value > MaxCount-m.GeneUMIs[cell] {
					return nil, fmt.Errorf("matrix: gene-expression UMI total exceeds 2^53-1 for barcode %q", m.Barcodes[cell])
				}
				m.GeneUMIs[cell] += e.value
				if e.value > 0 {
					m.Genes[cell]++
				}
			}
		}
	}
	if progress != nil {
		progress(declared)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

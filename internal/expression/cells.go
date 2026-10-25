package expression

// Cell annotations and coordinates are supplied evidence, joined by exact barcode. This TSV
// contract and its missing-value/limit rules are ClonoLoom policies, not a clustering method.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const MaxMapPoints = 20000

type CellAnnotation struct {
	Barcode    string      `json:"barcode"`
	Values     []string    `json:"values"`
	Position   *[2]float64 `json:"position"`
	Expression bool        `json:"expression"`
}
type CellCoverage struct {
	Columns            []string `json:"columns"`
	Rows               int      `json:"rows"`
	Matched            int      `json:"matched"`
	AnnotationOnly     int      `json:"annotationOnly"`
	WithoutAnnotation  int      `json:"withoutAnnotation"`
	Mapped             int      `json:"mapped"`
	WithoutCoordinates int      `json:"withoutCoordinates"`
}
type CellAnnotations struct {
	Coverage  CellCoverage
	Rows      []CellAnnotation
	ByBarcode map[string]int
}

func ReadCells(ctx context.Context, r io.Reader, m *Matrix) (*CellAnnotations, error) {
	l, err := newLines(ctx, r, "cell annotations")
	if err != nil {
		return nil, err
	}
	defer func() {
		if l.close != nil {
			l.close()
		}
	}()
	if !l.next() {
		if err := l.err(); err != nil {
			return nil, err
		}
		return nil, l.bad("missing header")
	}
	header := strings.Split(l.scan.Text(), "\t")
	if header[0] != "barcode" || len(header) < 2 || len(header) > 35 {
		return nil, l.bad("expected barcode then up to 32 annotations and/or map_x, map_y")
	}
	out := &CellAnnotations{Coverage: CellCoverage{Columns: []string{}}, ByBarcode: map[string]int{}, Rows: []CellAnnotation{}}
	x, y := -1, -1
	var columns []int
	seen := map[string]bool{}
	for i, name := range header {
		if !validLabel(name) || seen[name] {
			return nil, l.bad("empty, duplicate or invalid column name")
		}
		seen[name] = true
		switch name {
		case "barcode":
		case "map_x":
			x = i
		case "map_y":
			y = i
		default:
			columns = append(columns, i)
			out.Coverage.Columns = append(out.Coverage.Columns, name)
		}
	}
	if (x < 0) != (y < 0) || len(columns) > 32 {
		return nil, l.bad("map_x and map_y must occur together; at most 32 annotation columns")
	}
	barcodes := make(map[string]bool, len(m.Barcodes))
	for i, b := range m.Barcodes {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		barcodes[b] = true
	}
	bytes := len(l.scan.Text())
	for l.next() {
		bytes += len(l.scan.Text())
		if bytes > maxLabelBytes || len(out.Rows) >= MaxCells || (len(out.Rows)+1)*len(columns) > 4000000 {
			return nil, l.bad("cell annotations exceed 64 MiB, 2000000 rows or 4000000 annotation values")
		}
		fields := strings.Split(l.scan.Text(), "\t")
		if len(fields) != len(header) {
			return nil, l.bad("wrong number of tab-separated fields")
		}
		if !validLabel(fields[0]) {
			return nil, l.bad("invalid barcode")
		}
		if _, exists := out.ByBarcode[fields[0]]; exists {
			return nil, l.bad("duplicate barcode")
		}
		row := CellAnnotation{Barcode: fields[0], Values: []string{}, Expression: barcodes[fields[0]]}
		for _, i := range columns {
			if fields[i] != "" && !validLabel(fields[i]) {
				return nil, l.bad("invalid annotation; empty means missing")
			}
			row.Values = append(row.Values, fields[i])
		}
		if x >= 0 && (fields[x] != "" || fields[y] != "") {
			var p [2]float64
			for k, i := range []int{x, y} {
				v, e := strconv.ParseFloat(fields[i], 64)
				if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e12 {
					return nil, l.bad("coordinates must be a complete finite pair within ±1e12")
				}
				p[k] = v
			}
			row.Position = &p
		}
		if row.Expression {
			out.Coverage.Matched++
			if row.Position != nil {
				out.Coverage.Mapped++
			}
		} else {
			out.Coverage.AnnotationOnly++
		}
		out.ByBarcode[row.Barcode] = len(out.Rows)
		out.Rows = append(out.Rows, row)
	}
	if err := l.err(); err != nil {
		return nil, err
	}
	out.Coverage.Rows = len(out.Rows)
	out.Coverage.WithoutAnnotation = len(m.Barcodes) - out.Coverage.Matched
	out.Coverage.WithoutCoordinates = len(m.Barcodes) - out.Coverage.Mapped
	return out, nil
}

// OpenWithCells opts in to one annotation source. Empty path preserves the original MEX ID,
// even if a file named cells.tsv has subsequently appeared beside the matrix.
func OpenWithCells(ctx context.Context, dir, cellsPath string) (*Input, error) {
	return OpenSelected(ctx, dir, cellsPath, nil)
}

func OpenSelected(ctx context.Context, dir, cellsPath string, selection *AnnDataSelection) (*Input, error) {
	var in *Input
	var err error
	if selection == nil {
		in, err = Open(ctx, dir)
	} else {
		var abs string
		abs, err = filepath.Abs(dir)
		if err == nil {
			in, err = openSelectedHDF5(ctx, abs, selection)
		}
	}
	if err != nil || cellsPath == "" {
		return in, err
	}
	if in.AnnData != nil {
		return nil, fmt.Errorf("AnnData supplies observations; omit the external cell TSV")
	}
	abs, err := filepath.Abs(cellsPath)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 1<<30 {
		return nil, fmt.Errorf("cell annotations must be a regular file no larger than 1 GiB")
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 1<<30 {
		return nil, fmt.Errorf("invalid cell annotation file")
	}
	h := sha256.New()
	limit := &io.LimitedReader{R: f, N: stat.Size() + 1}
	in.Cells, err = ReadCells(ctx, io.TeeReader(limit, h), in.Matrix)
	if err != nil {
		return nil, err
	}
	if limit.N != 1 {
		return nil, fmt.Errorf("cell annotations changed size while reading; reopen them")
	}
	source := Source{Role: "cells", Path: abs, Size: stat.Size(), SHA256: hex.EncodeToString(h.Sum(nil))}
	in.Sources = append(in.Sources, source)
	identity := sha256.Sum256([]byte(fmt.Sprintf("clonoloom-expression-cells/1\n%s\n%d\n%s", in.ID, source.Size, source.SHA256)))
	in.ID = "x-" + hex.EncodeToString(identity[:])
	return in, nil
}

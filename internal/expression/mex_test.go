package expression

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

const header = "%%MatrixMarket matrix coordinate integer general\n"
const features = "g1\tSame\tGene Expression\ng2\tSame\tGene Expression\nab1\tAntibody\tAntibody Capture\n"
const barcodes = "AA-1\nBB-1\nCC-1\nDD-1\n"
const matrix = header + "% counts\n3 4 6\n2 3 7\n1 1 2\n3 1 11\n2 1 3\n1 2 0\n2 2 4\n"

func readTest(m, f, b string) (*Matrix, error) {
	return ReadMEX(context.Background(), strings.NewReader(m), strings.NewReader(f), strings.NewReader(b), false, nil)
}
func gz(data string) []byte {
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	w.Write([]byte(data))
	w.Close()
	return out.Bytes()
}

func TestMEXValuesAndCompression(t *testing.T) {
	m, err := readTest(matrix, features, barcodes)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.GeneUMIs, []uint64{5, 4, 7, 0}) || !reflect.DeepEqual(m.Genes, []int{2, 1, 1, 0}) {
		t.Fatal(m)
	}
	if !reflect.DeepEqual(m.Indptr, []int{0, 3, 5, 6, 6}) || !reflect.DeepEqual(m.Indices, []uint32{0, 1, 2, 0, 1, 1}) || !reflect.DeepEqual(m.Counts, []uint64{2, 3, 11, 0, 4, 7}) {
		t.Fatal(m)
	}
	compressed, err := ReadMEX(context.Background(), bytes.NewReader(gz(matrix)), bytes.NewReader(gz(features)), bytes.NewReader(gz(barcodes)), false, nil)
	if err != nil || !reflect.DeepEqual(m, compressed) {
		t.Fatal(compressed, err)
	}
	crlf, err := readTest(strings.ReplaceAll(matrix, "\n", "\r\n"), strings.ReplaceAll(features, "\n", "\r\n"), strings.ReplaceAll(barcodes, "\n", "\r\n"))
	if err != nil || !reflect.DeepEqual(m, crlf) {
		t.Fatal(err)
	}
	legacy, err := ReadMEX(context.Background(), strings.NewReader(header+"1 1 1\n1 1 2\n"), strings.NewReader("g1\tGene\n"), strings.NewReader("AA-1\n"), true, nil)
	if err != nil || legacy.GeneUMIs[0] != 2 || legacy.Features[0].Type != "Gene Expression" {
		t.Fatal(legacy, err)
	}
}
func TestMEXRejectsMalformedEvidence(t *testing.T) {
	cases := []struct{ name, m, f, b, want string }{
		{"header", strings.Replace(matrix, "integer", "real", 1), features, barcodes, "raw integer"},
		{"missing header", "", features, barcodes, "header"},
		{"missing dimensions", header, features, barcodes, "declared"},
		{"dimensions", header + "3 5 0\n", features, barcodes, "dimensions"},
		{"huge declaration", header + "3 4 10000001\n", features, barcodes, "10,000,000"},
		{"short", header + "3 4 1\n", features, barcodes, "read 0"},
		{"extra", header + "3 4 0\n1 1 1\n", features, barcodes, "more coordinates"},
		{"zero index", header + "3 4 1\n0 1 1\n", features, barcodes, "outside"},
		{"negative", header + "3 4 1\n1 1 -1\n", features, barcodes, "integer"},
		{"fraction", header + "3 4 1\n1 1 1.5\n", features, barcodes, "integer"},
		{"infinity", header + "3 4 1\n1 1 Inf\n", features, barcodes, "integer"},
		{"overflow", header + "3 4 1\n1 1 18446744073709551616\n", features, barcodes, "integer"},
		{"unsafe", header + "3 4 1\n1 1 9007199254740992\n", features, barcodes, "exact integer"},
		{"unsafe total", header + "3 4 2\n1 1 9007199254740991\n2 1 1\n", features, barcodes, "total"},
		{"duplicate", header + "3 4 2\n1 1 0\n1 1 1\n", features, barcodes, "duplicate"},
		{"feature id", matrix, strings.Replace(features, "g2", "g1", 1), barcodes, "duplicate feature"},
		{"missing feature type", matrix, "g1\tGene\n", barcodes, "3 tab"},
		{"barcode", matrix, features, strings.Replace(barcodes, "BB-1", "AA-1", 1), "duplicate barcode"},
		{"blank barcode", matrix, features, barcodes + "\n", "nonempty"},
		{"barcode tab", matrix, features, "AA\tBB\n", "barcode"},
		{"invalid utf8", matrix, features, "\xff\n", "barcode"},
		{"line limit", matrix, strings.Repeat("a", maxLineBytes+1), barcodes, "token too long"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := readTest(c.m, c.f, c.b)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
	for _, which := range []int{0, 1, 2} {
		t.Run("gzip", func(t *testing.T) {
			data := [][]byte{gz(matrix), gz(features), gz(barcodes)}
			data[which] = data[which][:len(data[which])-4]
			_, err := ReadMEX(context.Background(), bytes.NewReader(data[0]), bytes.NewReader(data[1]), bytes.NewReader(data[2]), false, nil)
			if err == nil {
				t.Fatal("truncated gzip accepted")
			}
		})
	}
}
func TestMEXEmptyAndCancelled(t *testing.T) {
	m, err := readTest(header+"0 0 0\n", "", "")
	if err != nil || len(m.Indptr) != 1 {
		t.Fatal(m, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ReadMEX(ctx, strings.NewReader(matrix), strings.NewReader(features), strings.NewReader(barcodes), false, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestMEXLinkKeepsSampleScopeAndRawEvidence(t *testing.T) {
	m, err := readTest(matrix, features, barcodes)
	if err != nil {
		t.Fatal(err)
	}
	dict := rep.NewStrings(true)
	aa, _ := dict.Add("AA-1")
	bb, _ := dict.Add("BB-1")
	ee, _ := dict.Add("EE-1")
	cc, _ := dict.Add("CC-1")
	table := &rep.Table{Dict: dict, Samples: []rep.Sample{{Name: "one"}, {Name: "two"}}, Sample: []uint32{0, 0, 0, 0, 0, 1, 1}, Cell: []uint32{aa, aa, bb, ee, 0, aa, cc}}
	links, err := Link(context.Background(), m, table, 0)
	if err != nil || links.Matched != 2 || links.ReceptorCells != 3 || links.ExpressionOnly != 2 || links.ReceptorOnly != 1 || links.MissingBarcodeRows != 1 || !reflect.DeepEqual(links.ChainRows, []int{2, 1, 0, 0}) {
		t.Fatal(links, err)
	}
	other, err := Link(context.Background(), m, table, 1)
	if err != nil || other.Matched != 2 || !reflect.DeepEqual(other.ChainRows, []int{1, 0, 1, 0}) {
		t.Fatal(other, err)
	}
	if _, err = Link(context.Background(), m, table, 2); err == nil {
		t.Fatal("sample accepted")
	}
	table.Cell = nil
	if _, err = Link(context.Background(), m, table, 0); err == nil {
		t.Fatal("bulk accepted")
	}
}
func TestMEXOpenIdentityAndCompanions(t *testing.T) {
	copyTo := func(dir string) {
		for name, data := range map[string]string{"matrix.mtx": matrix, "features.tsv": features, "barcodes.tsv": barcodes} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	one, two := t.TempDir(), t.TempDir()
	copyTo(one)
	copyTo(two)
	a, err := Open(context.Background(), one)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(context.Background(), two)
	if err != nil || a.ID != b.ID {
		t.Fatal(b, err)
	}
	if err = os.WriteFile(filepath.Join(two, "matrix.mtx"), []byte(strings.Replace(matrix, "1 1 2", "1 1 9", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	b, err = Open(context.Background(), two)
	if err != nil || a.ID == b.ID {
		t.Fatal(b, err)
	}
	if err = os.WriteFile(filepath.Join(two, "matrix.mtx.gz"), gz(matrix), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(context.Background(), two); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal(err)
	}
	if _, err = Open(context.Background(), t.TempDir()); err == nil {
		t.Fatal("missing files accepted")
	}
}
func FuzzMEX(f *testing.F) {
	f.Add(matrix, features, barcodes, false)
	f.Add(header+"1 1 1\n1 1 3\n", "g\tGene\n", "cell\n", true)
	f.Add(string(gz(matrix)), string(gz(features)), string(gz(barcodes)), false)
	f.Fuzz(func(t *testing.T, m, features, barcodes string, legacy bool) {
		result, err := ReadMEX(context.Background(), strings.NewReader(m), strings.NewReader(features), strings.NewReader(barcodes), legacy, nil)
		if err != nil {
			return
		}
		if len(result.Indptr) != len(result.Barcodes)+1 || result.Indptr[len(result.Barcodes)] != len(result.Counts) {
			t.Fatal("invalid sparse shape")
		}
		for _, i := range result.Indices {
			if int(i) >= len(result.Features) {
				t.Fatal("invalid feature")
			}
		}
	})
}

func TestMEXLimitsAndCancellationDuringReading(t *testing.T) {
	var f, m strings.Builder
	for i := 0; i < 70000; i++ {
		fmt.Fprintf(&f, "g%d\tGene\tGene Expression\n", i)
	}
	m.WriteString(header + "70000 1 70000\n")
	for i := 1; i <= 70000; i++ {
		fmt.Fprintf(&m, "%d 1 1\n", i)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := ReadMEX(ctx, strings.NewReader(m.String()), strings.NewReader(f.String()), strings.NewReader("one\n"), false, func(n int) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	f.Reset()
	for i := 0; i <= MaxFeatures; i++ {
		fmt.Fprintf(&f, "g%d\tGene\tGene Expression\n", i)
	}
	if _, err := readTest(header, f.String(), ""); err == nil || !strings.Contains(err.Error(), "200,000") {
		t.Fatal(err)
	}
	// Gzip checksum corruption must fail even after every declared coordinate was consumed.
	data := gz(matrix)
	data[len(data)-8] ^= 1
	if _, err := ReadMEX(context.Background(), bytes.NewReader(data), strings.NewReader(features), strings.NewReader(barcodes), false, nil); err == nil {
		t.Fatal("bad checksum accepted")
	}
}

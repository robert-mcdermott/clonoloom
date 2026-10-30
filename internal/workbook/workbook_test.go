package workbook

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkbookExactEvidence(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/workbook/precision.json")
	if err != nil {
		t.Fatal(err)
	}
	budget := MaxNodes
	s, stats, err := Evidence(context.Background(), raw, "Precision", &budget)
	if err != nil {
		t.Fatal(err)
	}
	if stats.NumericText != 7 {
		t.Fatalf("numeric text count %d", stats.NumericText)
	}
	cells := map[string][]Cell{}
	for _, row := range s.Rows[1:] {
		cells[row[0].Text] = row
	}
	for _, path := range []string{"/subjects/0/count", "/subjects/0/rate", "/subjects/2/rate", "/subjects/2/estimate", "/small", "/large", "/unsafe_integer"} {
		if cells[path][2].Type != "text" {
			t.Fatal("precision lost", path)
		}
	}
	if cells["/subjects/1/count"][2] != (Cell{"number", "0"}) || cells["/subjects/0/available"][2] != (Cell{"boolean", "false"}) || cells["/subjects/0/estimate"][1].Text != "null" || cells["/empty_string"][2] != (Cell{"text", ""}) || cells["/empty_array"][4].Text != "0" || cells["/path~1~0/"][2].Text != "+SUM(A1:A2)" {
		t.Fatal("missingness/types/paths")
	}
	data, err := Write(context.Background(), []Sheet{s})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		in, _ := f.Open()
		raw, _ := io.ReadAll(in)
		in.Close()
		dec := xml.NewDecoder(bytes.NewReader(raw))
		for {
			token, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(f.Name, err)
			}
			if start, ok := token.(xml.StartElement); ok && (start.Name.Local == "f" || start.Name.Local == "externalLink" || start.Name.Local == "hyperlink") {
				t.Fatal("executable content")
			}
		}
	}
	if dir := os.Getenv("CLONOLOOM_WORKBOOK_OUT"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "precision.xlsx"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestWorkbookRefusals(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	budget := MaxNodes
	if _, _, err := Evidence(ctx, []byte(`{}`), "cancel", &budget); err == nil {
		t.Fatal("cancel")
	}
	budget = 1
	if _, _, err := Evidence(context.Background(), []byte(`[1]`), "limit", &budget); err == nil {
		t.Fatal("node budget")
	}
	for _, raw := range []string{`{"a":1,"a":2}`, `[NaN]`, `"\ud800"`} {
		budget = MaxNodes
		if _, _, err := Evidence(context.Background(), []byte(raw), "bad", &budget); err == nil {
			t.Fatal("invalid exact JSON", raw)
		}
	}
	for _, value := range []string{strings.Repeat("x", 32768), strings.Repeat("😀", 16384), strings.Repeat("\n", 254)} {
		if _, err := Write(context.Background(), []Sheet{{Name: "Limit", Rows: [][]Cell{{Text(value)}}}}); err == nil {
			t.Fatal("text overflow")
		}
	}
	for _, name := range []string{"History", "bad/name", "'bad", "", "12345678901234567890123456789012"} {
		if _, err := Write(context.Background(), []Sheet{{Name: name, Rows: [][]Cell{{Text("x")}}}}); err == nil {
			t.Fatal("invalid sheet", name)
		}
	}
	if _, err := Write(ctx, []Sheet{{Name: "Cancel", Rows: [][]Cell{{Text("x")}}}}); err == nil {
		t.Fatal("ignored write cancellation")
	}
	for _, s := range []string{"", "NaN", "Inf", "1e-9999", "1.000000000000001", "-0.0", "1e301"} {
		if Numeric(s) {
			t.Fatal("unsafe numeric", s)
		}
	}
	for _, s := range []string{"0", "1.0", "0.125", "1000000000000000", "1e-300", "1e300"} {
		if !Numeric(s) {
			t.Fatal("safe number", s)
		}
	}
}

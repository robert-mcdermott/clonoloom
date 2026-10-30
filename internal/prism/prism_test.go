package prism

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestPrismWriterControls(t *testing.T) {
	m := Mapping{Levels: []string{"before <&>", "after"}, Tables: []Table{{ID: "Table0", Title: "Artificial format controls", Rows: []Row{
		{Subject: "=subject<&01", Cells: [2]Cell{{State: "observed", Value: json.RawMessage("0")}, {State: "missing", Value: json.RawMessage("null"), Missing: "Not measured"}}},
		{Subject: "00123", Cells: [2]Cell{{State: "observed", Value: json.RawMessage("0.1234567890123456789")}, {State: "observed", Value: json.RawMessage("1.2300e+2")}}},
	}}}}
	raw, e := Write(context.Background(), m, "Artificial format controls", "No scientific study", "0.8.0")
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(raw, []byte("<d>0.1234567890123456789</d>")) || !bytes.Contains(raw, []byte("<d>1.2300e+2</d>")) || !bytes.Contains(raw, []byte("=subject&lt;&amp;01")) {
		t.Fatal("lexical precision or text escaped incorrectly")
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if start, ok := token.(xml.StartElement); ok && (start.Name.Local == "Script" || start.Name.Local == "Template") {
			t.Fatal("executable content")
		}
	}
	for _, value := range []string{"1e100000000", "1e-100000000", "0e100000000", "-0.0", "true", "[1]", "\"12\"", "9007199254740992", "-9007199254740993", "1e-301"} {
		if Number(value) == nil {
			t.Fatal("unsafe numeric", value)
		}
	}
	for _, value := range []string{"0", "1.0", "1e-300", "9007199254740991", "0.1234567890123456789"} {
		if e := Number(value); e != nil {
			t.Fatal(value, e)
		}
	}
	if _, e := Write(context.Background(), m, "bad\x00title", "", "0.8.0"); e == nil {
		t.Fatal("invalid XML control")
	}
	if _, e := Write(context.Background(), m, strings.Repeat("x", 65537), "", "0.8.0"); e == nil {
		t.Fatal("text bound")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Write(ctx, m, "cancel", "", "0.8.0"); e == nil {
		t.Fatal("cancel")
	}
	m.Tables[0].Rows = make([]Row, MaxCells/2+1)
	if _, e := Write(context.Background(), m, "oversized", "", "0.8.0"); e == nil {
		t.Fatal("cell bound")
	}
}

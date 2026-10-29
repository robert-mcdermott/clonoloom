package airr

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLExactScalarsAndCollections(t *testing.T) {
	source := `# source comment
---
Info:
  version: '2.0'
Repertoire:
- repertoire_id: R001
  subject:
    subject_id: '001'
  sample:
  - sample_id: S01
    single_cell: false
    collection_time_point_relative: 0
  extra:
    integer: 9007199254740993
    decimal: 1.234567890123456789
    exponent: 1e400
    negative_zero: -0
    absent_value:
    null_value: null
    tilde: ~
    yes: yes
    date: '2026-10-28'
    false_text: 'false'
    quote: 'researcher''s # comment stays'
    double: "line\nbreak"
    flow: {"a": [1, false, null, "#"]}
    literal: |-
      first # content
      second
...
`
	doc, err := ReadRepertoireYAML(context.Background(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	r := doc.Repertoires[0]
	if doc.SchemaVersion != "2.0" || r.SubjectID != "001" || r.TimePoint != "0" || r.SingleCell == nil || *r.SingleCell {
		t.Fatalf("%+v %+v", doc, r)
	}
	for _, token := range []string{`9007199254740993`, `1.234567890123456789`, `1e400`, `"negative_zero":-0`, `"absent_value":null`, `"yes":"yes"`, `"literal":"first # content\nsecond"`} {
		if !strings.Contains(string(r.Raw), token) {
			t.Fatalf("lost %s in %s", token, r.Raw)
		}
	}
	if output := os.Getenv("CLONOLOOM_YAML_EVIDENCE"); output != "" {
		os.WriteFile(output, []byte(source), 0600)
		os.WriteFile(output+".json", r.Raw, 0600)
	}
	for _, source := range []string{"repertoire_id: R\nnote: |\n  text", "repertoire_id: R\nnote: |\n  text\n", "repertoire_id: R\nnote: |+\n\n", "repertoire_id: R\nnote: |\n\n"} {
		d, err := ReadRepertoireYAML(context.Background(), []byte(source))
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		json.Unmarshal(d.Repertoires[0].Raw, &got)
		expected := ""
		if strings.Contains(source, "text") {
			expected = "text"
			if strings.HasSuffix(source, "\n") {
				expected += "\n"
			}
		} else if strings.Contains(source, "|+") {
			expected = "\n"
		}
		if got["note"] != expected {
			t.Fatalf("%q: %q != %q", source, got["note"], expected)
		}
	}
}

func TestYAMLRefusesUnsupportedOrAmbiguousInput(t *testing.T) {
	for _, source := range []string{
		"repertoire_id: R\nrepertoire_id: S", "repertoire_id: R\n'repertoire_id': S",
		"repertoire_id: R\nextra: &a 1", "repertoire_id: R\nextra: *a", "repertoire_id: R\nextra: !!str 1",
		"repertoire_id: R\nextra: 01", "repertoire_id: R\nextra: .inf", "repertoire_id: R\nextra: +1", "repertoire_id: R\nextra: 0x10", "repertoire_id: R\nextra: 1:20",
		"repertoire_id: R\nextra: >\n  folded", "repertoire_id: R\nextra: |2\n  literal", "repertoire_id: R\nextra: \"\\ud83d\\ude00\"",
		"repertoire_id: R\nextra: [yes, no]", "repertoire_id: R\nextra: {a: 1}", "repertoire_id: R\nextra: {\"a\": 1, \"a\": 2}",
		"repertoire_id: R\nextra: words\n  continued", "repertoire_id: R\nextra: 'words\n  continued'",
		"repertoire_id: R\n---\nrepertoire_id: S", "%YAML 1.1\n---\nrepertoire_id: R", "repertoire_id: R\n<<: {}",
		"repertoire_id: R\nextra: {\"<<\": {}}", "repertoire_id: R\nextra:\n\tkey: 0", "repertoire_id: R\nextra: |\n  text\n   \n",
		"repertoire_id: R\nextra: word 'quote # ignored?", "repertoire_id: 0", "repertoire_id: R\ntrue: 1",
	} {
		if _, err := ReadRepertoireYAML(context.Background(), []byte(source)); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadRepertoireYAML(ctx, []byte("repertoire_id: R")); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err := ReadRepertoireYAML(context.Background(), []byte("repertoire_id: R\nx: "+strings.Repeat("a", maxYAMLLineBytes))); err == nil {
		t.Fatal("line budget ignored")
	}
}

func TestYAMLReferenceCases(t *testing.T) {
	files, err := filepath.Glob("../../testdata/metadata/yaml/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatal("no YAML fixtures")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, _ := os.ReadFile(path)
			got, err := ReadRepertoireYAML(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := os.ReadFile(strings.TrimSuffix(path, ".yaml") + ".json")
			expected, err := ReadRepertoireDocument(context.Background(), want)
			if err != nil {
				t.Fatal(err)
			}
			if got.SchemaVersion != expected.SchemaVersion || len(got.Repertoires) != len(expected.Repertoires) {
				t.Fatal("document differs")
			}
			for i, r := range got.Repertoires {
				var v any
				d := json.NewDecoder(strings.NewReader(string(expected.Repertoires[i].Raw)))
				d.UseNumber()
				d.Decode(&v)
				canonical, _ := json.Marshal(v)
				if string(canonical) != string(r.Raw) {
					t.Fatalf("exact reference differs\n%s\n%s", canonical, r.Raw)
				}
			}
		})
	}
}
func FuzzRepertoireYAML(f *testing.F) {
	for _, s := range []string{"repertoire_id: R\nx: 9007199254740993", "Repertoire:\n- repertoire_id: R\n  sample:\n  - single_cell: false", "repertoire_id: R\nx: |-\n  text\n", "repertoire_id: R\nx: [1,null]"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := ReadRepertoireYAML(context.Background(), data)
		if err != nil {
			return
		}
		for _, r := range doc.Repertoires {
			if !json.Valid(r.Raw) {
				t.Fatal("invalid projection")
			}
			if _, err := ReadRepertoires(r.Raw); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestYAMLStructuralBudgetsAndCRLF(t *testing.T) {
	var nested strings.Builder
	nested.WriteString("repertoire_id: R\n")
	for i := 0; i < 66; i++ {
		nested.WriteString(strings.Repeat(" ", i*2) + "x:\n")
	}
	nested.WriteString(strings.Repeat(" ", 132) + "0\n")
	for _, source := range []string{nested.String(), strings.Repeat("#\n", maxYAMLLines), "repertoire_id: R\nx:\n" + strings.Repeat("- [0,0,0,0,0,0,0,0,0,0]\n", 100_001)} {
		if _, err := ReadRepertoireYAML(context.Background(), []byte(source)); err == nil {
			t.Fatal("structural budget ignored")
		}
	}
	lf := "repertoire_id: R\nx: |+\n  first\n\n  second\n\n"
	a, err := ReadRepertoireYAML(context.Background(), []byte(lf))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReadRepertoireYAML(context.Background(), []byte(strings.ReplaceAll(lf, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	if string(a.Repertoires[0].Raw) != string(b.Repertoires[0].Raw) {
		t.Fatal("CRLF changed decoded values")
	}
}

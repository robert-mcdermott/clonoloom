package reads

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

// VDJtools' format: count, frequency, CDR3 nucleotides and amino acids, V, D, J.
const vdjtools = "count\tfreq\tcdr3nt\tcdr3aa\tv\td\tj\n" +
	"120\t0.6\tTGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT\tCASSLAGGPEAFF\tTRBV5-1\tTRBD2\tTRBJ2-7\n" +
	"60\t0.3\tTGTGCCAGCAGTGAACGG\tCASSER\tTRBV6-2\t.\tTRBJ1-1\n" +
	"20\t0.1\tTGTGCCAGCTAGTTT\tCAS*F\tTRBV20-1\t.\tTRBJ2-1\n"

// MiXCR's clones: allele scores in its calls.
const mixcr = "cloneId\tcloneCount\tcloneFraction\tnSeqCDR3\taaSeqCDR3\tallVHitsWithScore\tallJHitsWithScore\n" +
	"0\t5\t0.5\tTGTGCC\tCASSLAGGPEAFF\tTRBV5-1*00(1250)\tTRBJ2-7*00(300)\n"

func TestInspectGuessesTheColumns(t *testing.T) {
	in, err := Inspect(strings.NewReader(vdjtools), 12)
	if err != nil {
		t.Fatal(err)
	}
	g := in.Guess.Columns
	if in.Separator != "tab" || len(in.Rows) != 3 || g["junction_aa"] != "cdr3aa" || g["junction"] != "cdr3nt" || g["v_call"] != "v" || g["d_call"] != "d" || g["j_call"] != "j" || g["count"] != "count" || in.Guess.Unit != "" {
		t.Errorf("%+v", in)
	}
	m, _ := Inspect(strings.NewReader(mixcr), 12)
	if g := m.Guess.Columns; g["junction_aa"] != "aaSeqCDR3" || g["v_call"] != "allVHitsWithScore" || g["count"] != "cloneCount" || g["clone_id"] != "cloneId" {
		t.Errorf("%+v", m.Guess)
	}
	// A bare "CDR3" column holds amino acids or nucleotides, as its values say; a semicolon sheet.
	sheet := "\uFEFFSample;CDR3;Reads\nA;CASSLF;3\nB;CASSPF;4\n"
	s, _ := Inspect(strings.NewReader(sheet), 12)
	if s.Separator != "semicolon" || s.Header[0] != "Sample" || s.Guess.Columns["junction_aa"] != "CDR3" || s.Guess.Columns["sample"] != "Sample" || s.Guess.Unit != rep.Reads {
		t.Errorf("%+v", s)
	}
}

func TestCheckAndReadThroughAMapping(t *testing.T) {
	m := Mapping{Columns: map[string]string{"junction_aa": "cdr3aa", "junction": "cdr3nt", "v_call": "v", "d_call": "d", "j_call": "j", "count": "count"}, Unit: rep.Reads}
	check, err := Check(context.Background(), strings.NewReader(vdjtools), "S1.tsv", m)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ColumnCheck{}
	for _, c := range check.Columns {
		by[c.Field] = c
	}
	// D: "." is no call; the stop codon's junction is still letters.
	if check.Rows != 3 || by["d_call"].Values != 1 || by["junction_aa"].Valid != 3 || !strings.Contains(by["junction_aa"].Note, "67% begin with C") || !strings.Contains(by["productive"].Note, "no column") || check.Samples[0] != "S1" {
		t.Errorf("%+v", check)
	}
	// Read: a row with a stop in its junction is not productive; the counts in reads.
	path := filepath.Join(t.TempDir(), "S1.tsv")
	os.WriteFile(path, []byte(vdjtools), 0o644)
	res, err := ReadInputs(context.Background(), []Input{{Path: path, Mapping: &m}})
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if tab.Len() != 3 || res.Unit != rep.Reads || res.Sources[0].Format != FormatTable || res.Sources[0].Mapping == nil || tab.LocusName(0) != "TRB" {
		t.Fatalf("%d rows, %v, %+v", tab.Len(), res.Unit, res.Sources[0])
	}
	if tab.Productive[0] != rep.True || tab.Productive[2] != rep.False || tab.Count[1] != 60 || tab.Dict.Get(tab.D[1]) != "" {
		t.Errorf("productive %v, count %v", tab.Productive, tab.Count)
	}
	// Without a count, every row counts once, in sequences.
	once := Mapping{Columns: map[string]string{"junction_aa": "cdr3aa"}}
	res, _ = ReadInputs(context.Background(), []Input{{Path: path, Mapping: &once}})
	if res.Unit != rep.Sequences || res.Table.Count[0] != 1 {
		t.Errorf("%v %v", res.Unit, res.Table.Count)
	}
	// Mappings that cannot be: no junction, a column not there, a count without its unit.
	for _, bad := range []Mapping{
		{Columns: map[string]string{"v_call": "v"}},
		{Columns: map[string]string{"junction_aa": "cdr3"}},
		{Columns: map[string]string{"junction_aa": "cdr3aa", "count": "count"}},
		{Columns: map[string]string{"junction_aa": "cdr3aa", "weight": "freq"}},
	} {
		if _, err := ReadInputs(context.Background(), []Input{{Path: path, Mapping: &bad}}); err == nil {
			t.Errorf("%+v read", bad)
		}
	}
}

func FuzzMapped(f *testing.F) {
	f.Add([]byte(vdjtools))
	f.Add([]byte(mixcr))
	f.Fuzz(func(t *testing.T, data []byte) {
		Inspect(strings.NewReader(string(data)), 12)
		m := Mapping{Columns: map[string]string{"junction_aa": "cdr3aa", "count": "count", "v_call": "v"}, Unit: rep.Reads}
		Check(context.Background(), strings.NewReader(string(data)), "x.tsv", m)
		var b *rep.Builder
		readMapped(context.Background(), strings.NewReader(string(data)), "x.tsv", 0, &b, newProblems(), m)
	})
}

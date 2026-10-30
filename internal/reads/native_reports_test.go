package reads

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

const nativeHeader = "count\tfrequency\tCDR3nt\tCDR3aa\tV\tD\tJ\n"
const nativeRow = "4\t0.25\tTGTTCTTAA\tCS*\tTRBV7-9\t.\tTRBJ2-5\n"

func TestNativeReportsConventionsAndEvidence(t *testing.T) {
	read := func(aa string, trust bool) *Result {
		t.Helper()
		header, row := strings.TrimSuffix(nativeHeader, "\n"), strings.TrimSuffix(nativeRow, "\n")
		row = strings.Replace(row, "CS*", aa, 1)
		if trust {
			header += "\tC\tcid"
			row += "\tTRBC2\tsim-1"
		}
		r, err := ReadReader(context.Background(), strings.NewReader(header+"\n"+row+"\n"), "sim.tsv")
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	trust, vdj := read("CS_", true), read("CS_", false)
	if trust.Table.JunctionsAA.Get(trust.Table.JunctionAA[0]) != "CS*" || trust.Table.StopCodon[0] != rep.True || vdj.Table.JunctionAA[0] != 0 || vdj.Table.VJInFrame[0] != rep.False {
		t.Fatal("marker conventions conflated")
	}
	partial := read("partial", true)
	if partial.Table.Junction[0] != 0 || partial.Table.JunctionAA[0] != 0 || partial.Table.Productive[0] != rep.Missing || partial.Problems.Count("report-partial") != 1 {
		t.Fatal("partial junction became a clonotype")
	}
	if trust.Unit != "" || trust.Table.Cell != nil || trust.Table.FullLength != nil {
		t.Fatal("invented unit or cell evidence")
	}
	ambiguous := read("CS?", true)
	if ambiguous.Table.StopCodon[0] != rep.Missing || ambiguous.Table.Productive[0] != rep.Missing {
		t.Fatal("an ambiguous codon was certified stop-free")
	}
	for _, aa := range []string{"", ".", "NA"} {
		r := read(aa, false)
		if r.Table.Productive[0] != rep.Missing || r.Table.JunctionAA[0] != 0 {
			t.Fatal("translated missing peptide")
		}
	}
	for _, s := range []string{"0", "0.0", "9007199254740991"} {
		if _, e := nativeCount(s, false); e != nil {
			t.Fatal(s, e)
		}
	}
	if n, _ := nativeCount(".", true); !math.IsNaN(n) {
		t.Fatal("missing count became zero")
	}
	for _, s := range []string{"-1", "NaN", "Inf", "1e3", "0x12", "9007199254740992", "9007199254740991.1", strings.Repeat("1", 129)} {
		if _, e := nativeCount(s, true); e == nil {
			t.Fatal("accepted count", s)
		}
	}
	if _, e := nativeCount("1.25", false); e == nil {
		t.Fatal("fractional VDJtools count")
	}
	for _, s := range []string{"1.000000000000000001", "-1e-308", "1e-100000000", "1/2", "NaN"} {
		if nativeFrequency(s) {
			t.Fatal("accepted frequency", s)
		}
	}
}

func TestNativeReportRefusalsAndCancellation(t *testing.T) {
	valid := nativeHeader + nativeRow
	for _, bad := range []string{
		strings.Replace(valid, "frequency", "frequency\tfreq", 1),
		strings.Replace(valid, "\tD\t", "\tV\t", 1),
		strings.Replace(valid, "\tD\t", "\tunknown\t", 1),
		strings.Replace(valid, "0.25", "1.000000000000000001", 1),
		strings.Replace(valid, "CS*", "Cſ*", 1),
		valid + "1\t0.1\n",
		strings.Replace(valid, "\tJ\n", "\tJ\tC\n", 1),
	} {
		if _, err := ReadReader(context.Background(), strings.NewReader(bad), "bad.tsv"); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadReader(ctx, strings.NewReader(valid), "cancel.tsv"); err == nil {
		t.Fatal("ignored cancellation")
	}
	// Evidence is bounded without truncation; row width remains valid.
	row := strings.TrimSuffix(nativeRow, "\n") + "\t" + strings.Repeat("x", 14<<20) + "\n"
	header := strings.TrimSuffix(nativeHeader, "\n") + "\tnote\n"
	if _, err := ReadReader(context.Background(), strings.NewReader(header+strings.Repeat(row, 5)), "large.tsv"); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatal("evidence bound", err)
	}
}

func TestNativeReportsGzipUploadsAndMappings(t *testing.T) {
	text := nativeHeader + nativeRow
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	z.Write([]byte(text))
	z.Close()
	path := filepath.Join(t.TempDir(), "content-addressed")
	os.WriteFile(path, compressed.Bytes(), 0600)
	r, err := OpenInputs(context.Background(), []Input{{Path: path, Name: "simulated.vdjtools.tsv.gz"}})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Sources[0].Gzipped || r.Sources[0].SHA256 != fmt.Sprintf("%x", sha256.Sum256(compressed.Bytes())) || r.Sources[0].Native.Rows[0].Text != strings.TrimSuffix(nativeRow, "\n") {
		t.Fatal("gzip evidence")
	}
	m := Mapping{Columns: map[string]string{"count": "count", "junction_aa": "CDR3aa"}, Unit: rep.Reads}
	mapped, err := ReadInputs(context.Background(), []Input{{Path: path, Name: "sim.tsv.gz", Mapping: &m}})
	if err != nil || mapped.Sources[0].Format != FormatTable || mapped.Sources[0].Native != nil || mapped.Unit != rep.Reads {
		t.Fatal("explicit mapping changed", err)
	}
}

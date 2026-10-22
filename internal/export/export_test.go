package export

import (
	"bytes"
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/airr"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

func table(t *testing.T) (*rep.Table, *clonotype.Table) {
	t.Helper()
	b := rep.NewBuilder(rep.UMIs)
	add := func(sample, id, v, j, aa string, count float64, productive rep.Flag) {
		r := rep.NewRow()
		r.Sample, r.SequenceID, r.Locus, r.V, r.J, r.JunctionAA, r.Junction, r.Count, r.Productive = sample, id, "TRB", v, j, aa, "TGT", count, productive
		if err := b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	add("S1", "a", "TRBV5-1*01", "TRBJ2-7*01", "CASSLAGGPEAFF", 5, rep.True)
	add("S1", "b", "TRBV6-2*01,TRBV6-3*01", "TRBJ1-1*01", "CASSPGQGAYEQYF", 2, rep.True)
	add("S2", "c", "TRBV5-1*02", "TRBJ2-7*01", "CASSLAGGPEAFF", 7, rep.True)
	add("S2", "d", "TRBV5-1*01", "TRBJ2-7*01", "CASSX", 4, rep.False)
	rt := b.Table()
	def, _ := clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}.Normalize()
	ct, err := clonotype.Compute(rt, nil, def)
	if err != nil {
		t.Fatal(err)
	}
	return rt, ct
}

// The rearrangements' TSV has every column the AIRR schema requires, each row's clonotype, and
// reads back, through ClonoLoom's own AIRR reader, into the same clonotypes.
func TestRearrangementsReadBack(t *testing.T) {
	rt, ct := table(t)
	var buf bytes.Buffer
	if err := Rearrangements(&buf, rt, nil, ct); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	header := strings.Split(lines[0], "\t")
	for _, f := range airr.Required {
		if !strings.Contains("\t"+lines[0]+"\t", "\t"+f+"\t") {
			t.Errorf("no %s column", f)
		}
	}
	if len(lines) != 5 {
		t.Fatalf("%d lines", len(lines))
	}
	field := func(line int, name string) string {
		for i, h := range header {
			if h == name {
				return strings.Split(lines[line], "\t")[i]
			}
		}
		return "?"
	}
	if field(1, "clone_id") == "" || field(1, "clone_id") != field(3, "clone_id") || field(4, "clone_id") != "" || field(1, "umi_count") != "5" || field(4, "productive") != "F" || field(2, "v_call") != "TRBV6-2*01,TRBV6-3*01" || field(3, "repertoire_id") != "S2" {
		t.Errorf("%s", buf.String())
	}
	// Read back: the same clonotypes, abundances and samples.
	path := filepath.Join(t.TempDir(), "export.tsv")
	os.WriteFile(path, buf.Bytes(), 0o644)
	res, err := reads.ReadFiles(context.Background(), []string{path})
	if err != nil {
		t.Fatal(err)
	}
	def, _ := clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}.Normalize()
	back, err := clonotype.Compute(res.Table, nil, def)
	if err != nil {
		t.Fatal(err)
	}
	if res.Table.Unit != rep.UMIs || len(back.Clonotypes) != len(ct.Clonotypes) || res.Table.Len() != 4 {
		t.Fatalf("%v, %d clonotypes, %d rows", res.Table.Unit, len(back.Clonotypes), res.Table.Len())
	}
	totals := func(c *clonotype.Table) map[string]float64 {
		out := map[string]float64{}
		for _, x := range c.Clonotypes {
			out[x.Chains[0].Junction+x.Chains[0].V] = x.Total
		}
		return out
	}
	if a, b := totals(ct), totals(back); len(a) != len(b) || a["CASSLAGGPEAFFTRBV5-1"] != b["CASSLAGGPEAFFTRBV5-1"] || b["CASSLAGGPEAFFTRBV5-1"] != 12 {
		t.Errorf("%v, read back %v", a, b)
	}
	// What a filter leaves out is not written.
	keep := []bool{true, true, true, false}
	buf.Reset()
	Rearrangements(&buf, rt, keep, ct)
	if n := strings.Count(buf.String(), "\n"); n != 4 {
		t.Errorf("%d lines kept", n)
	}
}

func TestClonotypesCSV(t *testing.T) {
	_, ct := table(t)
	var buf bytes.Buffer
	if err := Clonotypes(&buf, ct, []string{"S1", "S2"}); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rows[0], ",") != "clonotype_id,chain1_locus,chain1_junction,chain1_v,chain1_j,total,samples,S1,S2" || len(rows) != 3 {
		t.Fatalf("%v", rows)
	}
	for _, r := range rows[1:] {
		if r[2] == "CASSLAGGPEAFF" && (r[5] != "12" || r[6] != "2" || r[7] != "5" || r[8] != "7") {
			t.Errorf("%v", r)
		}
		if r[2] == "CASSPGQGAYEQYF" && (r[3] != "TRBV6-2|TRBV6-3" || r[8] != "") {
			t.Errorf("%v", r)
		}
	}
}

func TestNotice(t *testing.T) {
	n := Notice{What: "Clonotypes", Dataset: "Study", DatasetID: "d-1", Files: []File{{Name: "a.tsv", SHA256: "ab12"}}, Samples: 2, Unit: "UMIs", Filter: "productive only",
		Definition: "amino-acid junction, V gene and J gene", DefinitionID: "def-1", Version: "0.2.0", Made: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)}
	if n.Restricted() || !strings.Contains(n.Text(), "the data are yours") || !strings.Contains(n.Text(), "File: a.tsv, SHA-256 ab12") {
		t.Errorf("%s", n.Text())
	}
	n.Credits = []Credit{{Title: "Emerson et al. 2017", Source: "immuneACCESS", License: "immuneACCESS Terms of Use", LicenseClass: "terms", Citation: "Emerson RO, et al. Nat Genet. 2017"}}
	if !n.Restricted() || !strings.Contains(n.Text(), "THESE DATA MAY NOT BE SHARED") || !strings.Contains(n.Text(), "Cite: Emerson RO") {
		t.Errorf("%s", n.Text())
	}
}

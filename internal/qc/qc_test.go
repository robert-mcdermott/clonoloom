package qc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

func bulk(t *testing.T) *rep.Table {
	t.Helper()
	b := rep.NewBuilder(rep.Templates)
	add := func(sample, v, j, junction, aa string, count float64, productive rep.Flag, inFrame, stop rep.Flag) {
		r := rep.NewRow()
		r.Sample, r.Locus, r.V, r.J, r.Junction, r.JunctionAA, r.Count, r.Productive, r.VJInFrame, r.StopCodon = sample, "TRB", v, j, junction, aa, count, productive, inFrame, stop
		if err := b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	// S1: three clonotypes (one met twice, by two V alleles of one gene), a singleton and a
	// doubleton; an out-of-frame and a stop-codon rearrangement; one row's V call ties two genes.
	add("S1", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", 5, rep.True, rep.True, rep.False)
	add("S1", "TRBV5-1*02", "TRBJ2-7*01", "", "CASSLAGGPEAFF", 3, rep.True, rep.True, rep.False)
	add("S1", "TRBV6-2*01,TRBV6-3*01", "TRBJ1-1*01", "", "CASSPGQGAYEQYF", 1, rep.True, rep.True, rep.False)
	add("S1", "TRBV7-2*01", "TRBJ1-2*01", "", "CASSQDF", 2, rep.True, rep.True, rep.False)
	add("S1", "TRBV7-2*01", "TRBJ1-2*01", "TGTGCCAG", "", 6, rep.False, rep.False, rep.Missing)
	add("S1", "TRBV7-2*01", "TRBJ1-2*01", "", "CAS*F", 3, rep.False, rep.True, rep.True)
	add("S2", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", 1, rep.True, rep.True, rep.False)
	return b.Table()
}

func TestBulkQC(t *testing.T) {
	r := Compute(bulk(t), Filter{})
	s := r.Samples[0]
	if s.Rows != 6 || s.Kept != 6 || s.Abundance != 20 {
		t.Fatalf("%+v", s)
	}
	if s.Productive != 11.0/20 || s.OutOfFrame != 6.0/20 || s.Stop != 3.0/20 || s.Unknown != 0 {
		t.Fatalf("status %v %v %v %v", s.Productive, s.OutOfFrame, s.Stop, s.Unknown)
	}
	// Clonotypes: CASSLAGGPEAFF (8), CASSPGQGAYEQYF (1), CASSQDF (2).
	if s.Clonotypes != 3 || s.Singletons != 1 || s.Doubletons != 1 || s.TopShare != 8.0/11 {
		t.Fatalf("%d %d %d %v", s.Clonotypes, s.Singletons, s.Doubletons, s.TopShare)
	}
	if want := Coverage(11, 1, 1); float64(s.Coverage) != want {
		t.Fatalf("coverage %v, want %v", s.Coverage, want)
	}
	// V resolved to one gene: all but the tie (1 of 11); to one family: all.
	if s.VGene != 10.0/11 || s.VFamily != 1 || s.JGene != 1 {
		t.Fatalf("resolution %v %v %v", s.VGene, s.VFamily, s.JGene)
	}
	if len(s.Lengths) != 3 || s.Lengths[0].Length != 7 || s.Lengths[2].Length != 14 || s.Lengths[1].Abundance != 8 || s.Lengths[1].Clonotypes != 1 {
		t.Fatalf("lengths %+v", s.Lengths)
	}
	if !r.IntegerCounts || float64(r.Samples[1].Coverage) != Coverage(1, 1, 0) {
		t.Fatal("S2")
	}
}

func TestFiltersSayWhatTheyLeftOut(t *testing.T) {
	tab := bulk(t)
	r := Compute(tab, Filter{ProductiveOnly: true, MinCount: 2, ExcludeSamples: []string{"S2"}})
	s := r.Samples[0]
	if s.Kept != 3 || s.Removed[ReasonNotProductive] != 2 || s.Removed[ReasonBelowCount] != 1 || r.Samples[1].Removed[ReasonSample] != 1 {
		t.Fatalf("%+v %+v", s, r.Samples[1].Removed)
	}
	// The productive share is measured before the productive rule, on what the rest keeps.
	if s.Productive != 10.0/19 || s.Abundance != 10 {
		t.Fatalf("productive %v abundance %v", s.Productive, s.Abundance)
	}
	// A minimum count above one leaves out the singletons coverage is estimated from: it is not
	// estimated, and the report says why.
	if !math.IsNaN(float64(s.Coverage)) || !strings.Contains(r.NoCoverage, "fewer than 2 times") {
		t.Fatalf("coverage %v under a minimum count: %q", s.Coverage, r.NoCoverage)
	}
	if plain := Compute(tab, Filter{}); plain.NoCoverage != "" || math.IsNaN(float64(plain.Samples[0].Coverage)) {
		t.Fatalf("coverage without a filter: %q", plain.NoCoverage)
	}
	if other := Compute(tab, Filter{Loci: []string{"TRA"}}); other.Samples[0].Kept != 0 || other.Samples[0].Removed[ReasonLocus] != 6 {
		t.Fatalf("%+v", other.Samples[0])
	}
}

func TestCellsByTheirChains(t *testing.T) {
	b := rep.NewBuilder(rep.Cells)
	add := func(cell, locus string, productive rep.Flag, umis float64) {
		r := rep.NewRow()
		r.Sample, r.Cell, r.Locus, r.JunctionAA, r.Productive, r.UMIs, r.Count = "S", cell, locus, "CASSF", productive, umis, umis
		b.Append(&r)
	}
	add("a", "TRA", rep.True, 3)
	add("a", "TRB", rep.True, 9)
	add("b", "TRB", rep.True, 5)
	add("c", "TRA", rep.True, 2)
	add("c", "TRA", rep.True, 2)
	add("c", "TRA", rep.True, 1)
	add("c", "TRB", rep.True, 4)
	add("d", "TRB", rep.False, 1)
	add("e", "IGH", rep.True, 20)
	add("e", "IGK", rep.True, 10)
	add("e", "IGL", rep.True, 6)
	r := Compute(b.Table(), Filter{})
	ch := r.Samples[0].Chains
	if ch.Cells != 5 || ch.Paired != 3 || ch.Single != 1 || ch.None != 1 || ch.Multi != 1 || ch.ByLocus["TRA"] != 2 || ch.ByLocus["TRB"] != 3 {
		t.Fatalf("%+v", ch)
	}
	if ch.Configurations["TRA 1, TRB 1"] != 1 || ch.Configurations["TRA 0, TRB 1"] != 1 || ch.Configurations["TRA 3+, TRB 1"] != 1 || ch.Configurations["IGH 1, light 2"] != 1 {
		t.Fatalf("%v", ch.Configurations)
	}
	if r.Samples[0].Abundance != 4 || !math.IsNaN(float64(r.Samples[0].Coverage)) {
		t.Fatalf("cells %v coverage %v", r.Samples[0].Abundance, r.Samples[0].Coverage)
	}
	// Quartiles of UMIs per productive contig (R's type 7).
	if q := ch.UMIs; q.Median != 4.5 || q.Q1 != 2.25 || q.Q3 != 8.25 { // numpy.quantile([1,2,2,3,4,5,6,9,10,20], [.25,.5,.75])
		t.Fatalf("%+v", q)
	}
	// A cell with more than two chains of a locus left out as a possible doublet.
	f := Compute(b.Table(), Filter{MaxChains: 2})
	if f.Samples[0].Chains.Cells != 4 || f.Samples[0].Removed[ReasonManyChains] != 4 {
		t.Fatalf("%+v", f.Samples[0])
	}
}

func TestVendorMetricsBothLayouts(t *testing.T) {
	dir := t.TempDir()
	wide := dir + "/wide.csv"
	long := dir + "/long.csv"
	writeFile(t, wide, "Estimated Number of Cells,Number of Cells With Productive V-J Spanning Pair,Cells With Productive TRA Contig,Cells With Productive V-J Spanning Pair\n\"1,000\",800,90.0%,80.0%\n")
	writeFile(t, long, "Category,Library Type,Grouped By,Group Name,Metric Name,Metric Value\nLibrary,VDJ T,Physical library ID,x,VDJ Cells,999\nCells,VDJ T,,,VDJ cells,\"1,000\"\nCells,VDJ T,,,Number of cells with productive V-J spanning pair,801\n")
	ch := &Chains{Cells: 1000, Paired: 800, ByLocus: map[string]int{"TRA": 900}}
	for path, want := range map[string][]bool{wide: {true, true, true}, long: {false}} {
		m, err := CellRangerMetrics(path)
		if err != nil {
			t.Fatal(err)
		}
		var got []bool
		for _, c := range CompareCellRanger(m, ch, []string{"TRA"}) {
			if c.Agrees != nil {
				got = append(got, *c.Agrees)
			}
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %v", path, got)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: %v, want %v", path, got, want)
			}
		}
	}
}

// A QC report is JSON even where a number is not a number (coverage of single-cell data).
func TestReportsAreJSON(t *testing.T) {
	b := rep.NewBuilder(rep.Cells)
	r := rep.NewRow()
	r.Sample, r.Cell, r.Locus, r.JunctionAA, r.Productive = "S", "a", "TRB", "CASSF", rep.True
	b.Append(&r)
	data, err := json.Marshal(Compute(b.Table(), Filter{}))
	if err != nil || !strings.Contains(string(data), `"coverage":null`) {
		t.Fatalf("%v %s", err, data)
	}
}

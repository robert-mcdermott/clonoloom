package specificity

import (
	"context"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/rep"
	"clonoloom/internal/tcrdist"
)

const header = "complex.id\tgene\tcdr3\tv.segm\tj.segm\tspecies\tmhc.a\tmhc.b\tmhc.class\tantigen.epitope\tantigen.gene\tantigen.species\treference.id\tmethod\tvdjdb.score\n"
const row = "1\tTRB\tCASSLGQETQYF\tTRBV5-1*01\tTRBJ2-5*01\tHomoSapiens\tA\tB\tI\tPEPTIDE\tprotein\tvirus\tref\t{\"test\":true}\t3\n"

func table() *clonotype.Table {
	d, _ := (clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}).Normalize()
	return &clonotype.Table{Definition: d, Unit: rep.Templates, Clonotypes: []clonotype.Clonotype{{ID: "one", Chains: []clonotype.Chain{{Locus: "TRB", Junction: "CASSLGQETQYF", V: "TRBV5-1", J: "TRBJ2-5"}, {Locus: "TRB", Junction: "CASSLGQETQYF", V: "TRBV5-1", J: "TRBJ2-5"}}}}, Start: []int32{0, 2}, SampleOf: []uint32{0, 1}, Abundance: []float64{3, 5}, SampleTotals: []float64{10, 10, 0}}
}
func TestEvidenceAndShares(t *testing.T) {
	records, err := Parse(strings.NewReader(header + row + row))
	if err != nil {
		t.Fatal(err)
	}
	records = append(records, records[0])
	records[2].Species = "MusMusculus"
	for _, metric := range []string{"exact", "tcrdist"} {
		r, err := Find(context.Background(), table(), records, Options{Metric: metric, Genes: true, CDR3Only: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Matches) != 4 || len(r.Records) != 2 || len(r.Epitopes) != 1 || r.Epitopes[0].Counts[0] != 3 || r.Epitopes[0].Shares[1] != .5 || len(r.Epitopes[0].Clonotypes) != 1 {
			t.Fatalf("%+v", r)
		}
	}
}
func TestDefinitionsAndFilters(t *testing.T) {
	records, _ := Parse(strings.NewReader(header + row))
	cases := []struct {
		name   string
		change func(*clonotype.Table, []Record, *Options)
		n      int
		bad    bool
	}{
		{"missing gene", func(tb *clonotype.Table, _ []Record, _ *Options) {
			tb.Clonotypes[0].Chains[0].V = ""
			tb.Clonotypes[0].Chains = tb.Clonotypes[0].Chains[:1]
		}, 0, false},
		{"wrong locus", func(tb *clonotype.Table, _ []Record, _ *Options) {
			tb.Clonotypes[0].Chains[0].Locus = "TRA"
			tb.Clonotypes[0].Chains = tb.Clonotypes[0].Chains[:1]
		}, 0, false},
		{"gene ignored", func(tb *clonotype.Table, _ []Record, o *Options) { tb.Clonotypes[0].Chains[0].V = ""; o.Genes = false }, 2, false},
		{"family", func(tb *clonotype.Table, _ []Record, _ *Options) { tb.Definition.V = "family" }, 0, true},
		{"nt", func(tb *clonotype.Table, _ []Record, _ *Options) { tb.Definition.Junction = "nt" }, 0, true},
		{"species", func(_ *clonotype.Table, _ []Record, o *Options) { o.Species = "MusMusculus" }, 0, false},
		{"score", func(_ *clonotype.Table, r []Record, o *Options) { r[0].Score = 1; o.MinScore = 2 }, 0, false},
		{"loops required", func(_ *clonotype.Table, _ []Record, o *Options) { o.Metric = "tcrdist" }, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := table()
			rr := append([]Record(nil), records...)
			o := Options{Genes: true}
			c.change(tb, rr, &o)
			got, err := Find(context.Background(), tb, rr, o, nil)
			if (err != nil) != c.bad || len(got.Matches) != c.n {
				t.Fatalf("matches %d, error %v", len(got.Matches), err)
			}
		})
	}
}
func TestNearBoundaryAndCancellation(t *testing.T) {
	records, _ := Parse(strings.NewReader(header + row))
	tb := table()
	tb.Clonotypes[0].Chains = tb.Clonotypes[0].Chains[:1]
	tb.Clonotypes[0].Chains[0].Junction = "CASSLAQETQYF"
	distance := tcrdist.ChainDistance(tcrdist.NewChain(records[0].CDR3, nil), tcrdist.NewChain(tb.Clonotypes[0].Chains[0].Junction, nil))
	for _, radius := range []int{distance - 1, distance} {
		r, e := Find(context.Background(), tb, records, Options{Metric: "tcrdist", CDR3Only: true, Radius: radius}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if (len(r.Matches) == 1) != (radius == distance) {
			t.Fatal(radius, r)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Find(ctx, tb, records, Options{}, nil); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestReaderHeaderAndErrors(t *testing.T) {
	good := strings.Split(strings.TrimSuffix(header, "\n"), "\t")
	values := strings.Split(strings.TrimSuffix(row, "\n"), "\t")
	for i, j := 0, len(good)-1; i < j; i, j = i+1, j-1 {
		good[i], good[j] = good[j], good[i]
		values[i], values[j] = values[j], values[i]
	}
	if r, e := Parse(strings.NewReader(strings.Join(good, "\t") + "\n" + strings.Join(values, "\t") + "\n")); e != nil || r[0].Score != 3 || r[0].Method != "{\"test\":true}" {
		t.Fatal(r, e)
	}
	for _, bad := range []string{"", "cdr3\n", header + "bad\n", header + strings.Replace(row, "\t3\n", "\tbad\n", 1), strings.Replace(header, "complex.id", "gene", 1) + row} {
		if _, e := Parse(strings.NewReader(bad)); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}

func TestTCRdistLoopsAndMissingEvidence(t *testing.T) {
	loops, err := tcrdist.ParseLoops(strings.NewReader("id\torganism\tregion\tcdrs\nTRBV5-1*01\thuman\tV\tAAA;AAA;AAA\nTRBV6-1*01\thuman\tV\tAGA;AAA;AAA\n"))
	if err != nil {
		t.Fatal(err)
	}
	records, _ := Parse(strings.NewReader(header + row + row))
	records[0].V = "TRBV6-1*01"
	records[1].V = "TRBV999*01"
	tb := table()
	tb.Clonotypes[0].Chains = tb.Clonotypes[0].Chains[:1]
	// A/G has BLOSUM62 score 0: min(4,4-0)=4 in CDR1, weight one.
	for _, radius := range []int{3, 4} {
		got, err := Find(context.Background(), tb, records, Options{Metric: "tcrdist", Radius: radius}, loops)
		if err != nil {
			t.Fatal(err)
		}
		if (len(got.Matches) == 1) != (radius == 4) || got.Excluded["database: missing V-gene loops"] != 1 {
			t.Fatal(radius, got)
		}
	}
	tb.Clonotypes[0].Chains[0].V = ""
	got, err := Find(context.Background(), tb, records, Options{Metric: "tcrdist", Radius: 100}, loops)
	if err != nil || len(got.Matches) != 0 || got.Excluded["query: missing V-gene loops"] != 1 {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ParseContext(ctx, strings.NewReader(header+row)); err != context.Canceled {
		t.Fatal(err)
	}
}

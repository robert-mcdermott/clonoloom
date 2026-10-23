package bcell

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/rep"
	"clonoloom/internal/seq"
)

func shmFixture(t *testing.T) (GermlineRow, [rep.EvidenceCount]string) {
	t.Helper()
	raw := germlineEvidence(map[string]string{"sequence": "GCTAAATGCAAATGGTTATTTAAA", "rev_comp": "F", "v_cigar": "9M15S", "j_cigar": "12S12M", "fwr1_start": "1", "fwr1_end": "6", "fwr3_start": "7", "fwr3_end": "9", "cdr3_start": "10", "cdr3_end": "12", "fwr4_start": "13", "fwr4_end": "24"})
	var g GermlineRow
	if why := projectGermline(raw, "GCCGAATGC", "TGGCTGGGGTAA", &g); why != "" {
		t.Fatal(why)
	}
	return g, raw
}
func TestSHMAnalyticCodons(t *testing.T) {
	g, raw := shmFixture(t)
	r := measureSHM(g, raw, "TGCAAATGG", rep.True)
	c := r.Regions[0]
	// GCT/GCC: S; AAA/GAA: R; TTA/CTG: two-hit Leu/Leu;
	// TTT/GGG: three-hit Phe/Gly; AAA/TAA: reference stop, unclassified.
	if r.Excluded != "" || r.CodonExclusion != "" || r.FrameAnchor != 10 || c.EvaluableBases != 21 || c.Mutations != 8 || c.Masked != 3 || c.EvaluableCodons != 6 || c.Replacement != 1 || c.Silent != 1 || c.Unclassified != 6 || c.MultiHitCodons != 2 || c.ReplacementCodons != 2 || c.SilentCodons != 2 || c.StopCodons != 1 || *c.Frequency != 8.0/21 {
		t.Fatalf("%+v", r)
	}
	if r.Regions[8].EvaluableBases != 0 || r.Regions[8].Frequency != nil || r.Regions[8].Masked != 3 || r.Regions[4].Unavailable == "" {
		t.Fatal(r.Regions)
	}
	reverse := raw
	reverse[fields["sequence"]] = seq.ReverseComplement(raw[fields["sequence"]])
	reverse[fields["rev_comp"]] = "T"
	if got := measureSHM(g, reverse, "TGCAAATGG", rep.True); !reflect.DeepEqual(got, r) {
		t.Fatal(got, r)
	}
	// Clipping removes the first base, not the query frame. Only the second V codon mutates in an evaluable codon.
	g.Query = g.Query[1:]
	g.Germline = g.Germline[1:]
	g.Regions = g.Regions[1:]
	g.QueryPositions = g.QueryPositions[1:]
	g.ReferencePositions = g.ReferencePositions[1:]
	c = measureSHM(g, raw, "TGCAAATGG", rep.True).Regions[0]
	if c.EvaluableBases != 20 || c.EvaluableCodons != 5 || c.Mutations != 8 || c.Silent != 0 || c.Unclassified != 7 {
		t.Fatal(c)
	}
}
func TestSHMMissingAndInvalidEvidence(t *testing.T) {
	for _, tc := range []struct{ field, value, why string }{
		{"cdr3_start", "", "CDR3"}, {"cdr3_end", "13", "CDR3"}, {"cdr3_start", "9999999999999999999999999", "CDR3"}, {"fwr3_start", "6", "CDR3"},
	} {
		t.Run(tc.field+tc.value, func(t *testing.T) {
			g, raw := shmFixture(t)
			raw[fields[tc.field]] = tc.value
			r := measureSHM(g, raw, "TGCAAATGG", rep.True)
			if !strings.Contains(r.CodonExclusion, tc.why) || r.Regions[0].Mutations != 8 || r.Regions[0].Unclassified != 8 || r.Regions[0].EvaluableCodons != 0 {
				t.Fatal(r)
			}
		})
	}
	g, raw := shmFixture(t)
	for _, flag := range []rep.Flag{rep.False, rep.Missing} {
		r := measureSHM(g, raw, "TGCAAATGG", flag)
		if r.CodonExclusion == "" || r.Regions[0].Unclassified != 8 {
			t.Fatal(r)
		}
	}
	if r := measureSHM(g, raw, "TGTAAATGG", rep.True); !strings.Contains(r.CodonExclusion, "disagree") {
		t.Fatal(r)
	}
	raw[fields["fwr1_end"]] = "5" // Partial regional codon: nucleotide difference stays unclassified locally.
	r := measureSHM(g, raw, "TGCAAATGG", rep.True)
	if r.Regions[3].Mutations != 2 || r.Regions[3].Silent != 1 || r.Regions[3].Replacement != 0 || r.Regions[3].Unclassified != 1 {
		t.Fatal(r)
	}
	// One ambiguous base masks its entire codon for R/S, but only itself for bases.
	g, raw = shmFixture(t)
	g.Germline = "N" + g.Germline[1:]
	r = measureSHM(g, raw, "TGCAAATGG", rep.True)
	if r.Regions[0].EvaluableBases != 20 || r.Regions[0].Ambiguous != 1 || r.Regions[0].EvaluableCodons != 5 || r.Regions[0].Silent != 0 {
		t.Fatal(r)
	}
	// A deletion has no query-region assignment and suppresses codon inference globally.
	g, raw = shmFixture(t)
	g.Query = "-" + g.Query[1:]
	g.QueryPositions[0] = 0
	r = measureSHM(g, raw, "TGCAAATGG", rep.True)
	if r.Regions[0].Gaps != 1 || !strings.Contains(r.CodonExclusion, "indel") || r.Regions[0].EvaluableCodons != 0 {
		t.Fatal(r)
	}
	g, raw = shmFixture(t)
	g.QueryPositions[0] = -1
	if r = measureSHM(g, raw, "TGCAAATGG", rep.True); r.Excluded == "" || r.Regions[0].Unavailable == "" {
		t.Fatal(r)
	}
	g.QueryPositions = nil
	if r = measureSHM(g, raw, "TGCAAATGG", rep.True); r.Excluded == "" {
		t.Fatal(r)
	}
}
func TestSHMIdentityPoolingAndCancellation(t *testing.T) {
	_, raw := shmFixture(t)
	a := familyRow("a", "a", "", "IGH", "TGCAAATGG")
	a.Evidence = raw
	a.VJInFrame = rep.True
	b := a
	b.SequenceID = "b"
	b.Count = 1000000
	tab := familyTable(t, []rep.Row{a, b})
	src := []FamilySource{{Name: "test.tsv", SHA256: "sha"}}
	f, err := Families(context.Background(), tab, nil, src, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Germlines(context.Background(), tab, src, f, GermlineReference{"IGHV1-1*01": "GCCGAATGC", "IGHJ1*01": "TGGCTGGGGTAA"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := SHM(context.Background(), tab, src, g)
	if err != nil || r.Evaluated != 2 || len(r.Rows) != 2 {
		t.Fatal(r, err)
	}
	for _, s := range r.Summaries {
		if s.Region == "all" && (s.Rows != 2 || s.EvaluableBases != 42 || s.Mutations != 16 || s.CodonRows != 2) {
			t.Fatal(s)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = SHM(ctx, tab, src, g); err != context.Canceled {
		t.Fatal(err)
	}
	g.Rows[0].Sample = "other"
	if _, err = SHM(context.Background(), tab, src, g); err == nil {
		t.Fatal("accepted altered row")
	}
	g.Rows[0].Sample = "a"
	g.Rows = append(g.Rows, g.Rows[0])
	if _, err = SHM(context.Background(), tab, src, g); err == nil {
		t.Fatal("accepted duplicate row")
	}
}
func FuzzSHMEvidence(f *testing.F) {
	f.Add("10", "12", "GCTAAATGCAAATGGTTATTTAAA")
	f.Fuzz(func(t *testing.T, start, end, query string) {
		if len(query) > 10000 {
			return
		}
		g, raw := shmFixture(t)
		raw[fields["cdr3_start"]] = start
		raw[fields["cdr3_end"]] = end
		raw[fields["sequence"]] = query
		r := measureSHM(g, raw, "TGCAAATGG", rep.True)
		for _, s := range r.Regions {
			if s.Mutations > s.EvaluableBases || s.Unclassified < 0 || s.Replacement+s.Silent > s.Mutations {
				t.Fatal(r)
			}
		}
	})
}

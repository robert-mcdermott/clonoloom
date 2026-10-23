package invariant

import (
	"clonoloom/internal/clonotype"
	"testing"
)

func alpha(v, j, aa string) clonotype.Chain {
	return clonotype.Chain{Locus: "TRA", V: v, J: j, Junction: aa}
}
func TestPublishedRulesAndMissingEvidence(t *testing.T) {
	d, _ := (clonotype.Definition{}).Normalize()
	// Published GEM consensus: Van Rhijn 2013 Fig 3; iNKT canonical sequence:
	// Musvosvi et al. Nat Med 2023, doi:10.1038/s41591-022-02110-9, Extended Data Fig 10.
	// MAIT TCRs: Sulfated bile acid is a host-derived ligand for MAIT cells (2024),
	// https://pmc.ncbi.nlm.nih.gov/articles/PMC11147531/, Methods, MAIT TCR transduction.
	// Mouse iNKT: https://pmc.ncbi.nlm.nih.gov/articles/PMC7479954/.
	// Boundary variants below are constructed and not claimed to be published receptors.
	cases := []struct {
		name, species string
		chains        []clonotype.Chain
		want          Flags
	}{
		{"published human MAIT", "human", []clonotype.Chain{alpha("TRAV1-2", "TRAJ33", "CAVKDSNYQLIW")}, Flags{Match, NoMatch, NoMatch}},
		{"published mouse iNKT", "mouse", []clonotype.Chain{alpha("TRAV11", "TRAJ18", "CVVGDRGSALGRLHF")}, Flags{NoMatch, Match, Unknown}},
		{"human MAIT", "human", []clonotype.Chain{alpha("TRAV1-2*01", "TRAJ33*01", "CAVRDSNYQLIW")}, Flags{Match, NoMatch, NoMatch}},
		{"GEM consensus", "human", []clonotype.Chain{alpha("TRAV1-2", "TRAJ9", "CAVRNTGGFKTIF")}, Flags{NoMatch, NoMatch, Match}},
		{"GEM L variant", "human", []clonotype.Chain{alpha("TRAV1-2", "TRAJ9", "CAVLNTGGFKTIF")}, Flags{NoMatch, NoMatch, Match}},
		{"GEM wrong motif", "human", []clonotype.Chain{alpha("TRAV1-2", "TRAJ9", "CAVANTGGFKTIF")}, Flags{NoMatch, NoMatch, NoMatch}},
		{"iNKT paired", "human", []clonotype.Chain{alpha("TRAV10", "TRAJ18", "CVVSDRGSTLGRLYF"), {Locus: "TRB", V: "TRBV25-1*01"}}, Flags{NoMatch, Match, NoMatch}},
		{"iNKT unpaired", "human", []clonotype.Chain{alpha("TRAV10", "TRAJ18", "CVVSDRGSTLGRLYF")}, Flags{NoMatch, Unknown, NoMatch}},
		{"iNKT other beta", "human", []clonotype.Chain{alpha("TRAV10", "TRAJ18", "CVVSDRGSTLGRLYF"), {Locus: "TRB", V: "TRBV6-2"}}, Flags{NoMatch, NoMatch, NoMatch}},
		{"mouse MAIT", "mouse", []clonotype.Chain{alpha("TRAV1", "TRAJ33", "CAVRDSNYQLIW")}, Flags{Match, NoMatch, Unknown}},
		{"species matters", "mouse", []clonotype.Chain{alpha("TRAV1-2", "TRAJ33", "CAVRDSNYQLIW")}, Flags{NoMatch, NoMatch, Unknown}},
		{"ambiguous", "human", []clonotype.Chain{alpha("TRAV1-2|TRAV2", "TRAJ33", "CAVRDSNYQLIW")}, Flags{Unknown, Unknown, Unknown}},
		{"same gene alleles", "human", []clonotype.Chain{alpha("TRAV1-2*01|TRAV1-2*02", "TRAJ33", "CAVRDSNYQLIW")}, Flags{Match, NoMatch, NoMatch}},
		{"unknown residue", "human", []clonotype.Chain{alpha("TRAV1-2", "TRAJ9", "CAVRXTGGFKTIF")}, Flags{Unknown, Unknown, Unknown}},
		{"no alpha", "human", []clonotype.Chain{{Locus: "TRB", V: "TRBV25-1"}}, Flags{Unknown, Unknown, Unknown}},
		{"unspecified species", "", []clonotype.Chain{alpha("TRAV1-2", "TRAJ33", "CAVRDSNYQLIW")}, Flags{Unknown, Unknown, Unknown}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(clonotype.Clonotype{Chains: tc.chains}, d, tc.species); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	for _, j := range []string{"TRAJ12", "TRAJ20", "TRAJ33"} {
		for n := 11; n <= 13; n++ {
			c := clonotype.Clonotype{Chains: []clonotype.Chain{alpha("TRAV1-2", j, "CAVRDSNYQLIWF"[:n])}}
			if (Classify(c, d, "human")[0] == Match) != (n == 12) {
				t.Fatal(j, n)
			}
		}
	}
	for _, v := range []string{"TRAV11", "TRAV11D"} {
		for n := 14; n <= 16; n++ {
			c := clonotype.Clonotype{Chains: []clonotype.Chain{alpha(v, "TRAJ18", "CVVGDRGSALGRLHFY"[:n])}}
			if (Classify(c, d, "mouse")[1] == Match) != (n == 15) {
				t.Fatal(v, n)
			}
		}
	}
	for _, change := range []func(*clonotype.Definition){func(d *clonotype.Definition) { d.Junction = "nt" }, func(d *clonotype.Definition) { d.V = "family" }, func(d *clonotype.Definition) { d.J = "none" }, func(d *clonotype.Definition) { d.Vendor = true }} {
		dd := d
		change(&dd)
		if got := Classify(clonotype.Clonotype{Chains: cases[0].chains}, dd, "human"); got != (Flags{Unknown, Unknown, Unknown}) {
			t.Fatal(got)
		}
	}
}
func TestSampleSharesCountClonotypesOnce(t *testing.T) {
	d, _ := (clonotype.Definition{}).Normalize()
	a := alpha("TRAV1-2", "TRAJ33", "CAVRDSNYQLIW")
	ct := &clonotype.Table{Definition: d, Clonotypes: []clonotype.Clonotype{{Chains: []clonotype.Chain{a, a}}, {Chains: []clonotype.Chain{{Locus: "TRB"}}}}, SampleTotals: []float64{10, 0}, Start: []int32{0, 1, 2}, SampleOf: []uint32{0, 0}, Abundance: []float64{2, 8}}
	r, err := Compute(ct, "human")
	if err != nil {
		t.Fatal(err)
	}
	s := r.Samples[0]
	if s.Matched[0] != 2 || s.Evaluable[0] != 2 || *s.Share[0] != .2 || r.Samples[1].Share[0] != nil {
		t.Fatal(r)
	}
	if _, err := Compute(ct, "rhesus"); err == nil {
		t.Fatal("unsupported species accepted")
	}
}

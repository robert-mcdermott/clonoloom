package neighborhood

import (
	"context"
	"math"
	"strings"
	"testing"

	"clonoloom/internal/pgen"
)

func receptors(cdr3s ...string) *Repertoire {
	r := &Repertoire{Locus: "TRB"}
	for i, s := range cdr3s {
		r.Receptors = append(r.Receptors, Receptor{CDR3: s, V: "TRBV5-1", J: "TRBJ1-1", Clonotype: i, Count: 1, Samples: 1})
	}
	return r
}

// The rates, odds ratio and χ² are Mayer-Blackwell et al.'s, the χ² with Yates's correction as
// scipy's chi2_contingency applies it to a 2 × 2 table (the values pinned here are scipy's).
func TestTheEnrichmentStatistics(t *testing.T) {
	if got := rate(10, 990); math.Abs(got-11.0/1002) > 1e-15 {
		t.Errorf("rate: %g", got)
	}
	if got := oddsRatio(10, 990, 2.5, 99997.5); math.Abs(got-(11.0/991)/(3.5/99998.5)) > 1e-9 {
		t.Errorf("odds ratio: %g", got)
	}
	for _, c := range []struct {
		hits, misses, bHits, bMisses, want float64
	}{
		{10, 990, 2.5, 99997.5, 753.168426523823},
		{1, 999, 0, 100000, 73.361490224758},
		{5, 95, 5, 95, 0},
		{100, 100, 100, 100, 0},
	} {
		got := chiSquare(c.hits, c.misses, c.bHits, c.bMisses)
		if math.Abs(got-c.want) > 1e-9*math.Max(1, c.want) {
			t.Errorf("χ²(%g, %g, %g, %g) = %.12f, scipy %.12f", c.hits, c.misses, c.bHits, c.bMisses, got, c.want)
		}
	}
}

// A centroid's radius is the largest at which the weighted background within it is at most the
// control share of the background's receptors, and 0 where even 0 is too many.
func TestTheRadiusHoldsTheControlRate(t *testing.T) {
	// A background of 1,000: one receptor identical to the first centroid, the rest far from both.
	sample := receptors("CASSLGGNTEAFF", "CASSPRQGAYEQYF")
	bg := &Repertoire{Locus: "TRB"}
	bg.Receptors = append(bg.Receptors, Receptor{CDR3: "CASSLGGNTEAFF", V: "TRBV5-1", J: "TRBJ1-1", Clonotype: -1, Count: 1})
	for range 999 {
		bg.Receptors = append(bg.Receptors, Receptor{CDR3: "CAWWWWWWWWWWWWWWF", V: "TRBV5-1", J: "TRBJ1-1", Clonotype: -1, Count: 1})
	}
	o := Options{Control: 1e-3, MaxRadius: 48, Step: 2, MinNeighbors: 1}
	res, err := Find(context.Background(), sample, bg, nil, "", o, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Centers) != 2 || res.Background != 1000 || res.Organism != "" {
		t.Fatalf("%+v", res)
	}
	by := map[string]Center{}
	for _, c := range res.Centers {
		by[sample.Receptors[c.Receptor].CDR3] = c
	}
	// The first has one background receptor at distance 0: 1/1000 is its control rate exactly, so
	// every radius holds it.
	if c := by["CASSLGGNTEAFF"]; c.Radius != 48 || c.Background != 1 || c.Weighted != 1 {
		t.Errorf("with a background receptor on it: %+v", c)
	}
	// The second has none within 48: its radius is the largest too.
	if c := by["CASSPRQGAYEQYF"]; c.Radius != 48 || c.Background != 0 {
		t.Errorf("with none: %+v", c)
	}
	// A control rate below one receptor in a thousand leaves the first no radius at all.
	o.Control = 1e-4
	res, err = Find(context.Background(), sample, bg, nil, "", o, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Centers {
		if sample.Receptors[c.Receptor].CDR3 == "CASSLGGNTEAFF" && c.Radius != 0 {
			t.Errorf("a background receptor on it, within a rate it breaks: %+v", c)
		}
	}
	// Workers do not change the answer.
	one, err := Find(context.Background(), sample, bg, nil, "", DefaultOptions(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	many, err := Find(context.Background(), sample, bg, nil, "", DefaultOptions(), 8, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Centers) != len(many.Centers) {
		t.Fatalf("%d centers on one worker, %d on eight", len(one.Centers), len(many.Centers))
	}
	for i := range one.Centers {
		if one.Centers[i].Radius != many.Centers[i].Radius || one.Centers[i].ChiSquare != many.Centers[i].ChiSquare {
			t.Errorf("%+v against %+v", one.Centers[i], many.Centers[i])
		}
	}
}

// A neighborhood holds its centroid and every receptor within its radius, nearest first.
func TestANeighborhoodHoldsTheReceptorsWithinIt(t *testing.T) {
	sample := receptors("CASSLGGNTEAFF", "CASSLGGNTEAFF", "CASSLGQNTEAFF", "CAWTTTTTTTTTTF")
	sample.Receptors[1].CDR3 = "CASSLGGSTEAFF"
	bg := receptors("CAWWWWWWWWWWWWWWF")
	bg.Receptors[0].Clonotype = -1
	res, err := Find(context.Background(), sample, bg, nil, "", Options{Control: 1, MaxRadius: 24, Step: 2, MinNeighbors: 1}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Centers {
		if c.Members[0] != c.Receptor {
			t.Errorf("%+v does not hold its centroid first", c)
		}
		if c.Neighbors != len(c.Members) {
			t.Errorf("%d neighbors, %d members", c.Neighbors, len(c.Members))
		}
		if sample.Receptors[c.Receptor].CDR3 == "CAWTTTTTTTTTTF" && c.Neighbors != 1 {
			t.Errorf("a receptor unlike the others has %d neighbors", c.Neighbors)
		}
		if sample.Receptors[c.Receptor].CDR3 == "CASSLGGNTEAFF" && c.Neighbors != 3 {
			t.Errorf("one of three alike has %d neighbors", c.Neighbors)
		}
	}
}

// The motif names the residues the neighbors have at each position the distance compares, and
// marks a position some neighbor has none at.
func TestTheMotifNamesTheNeighborsResidues(t *testing.T) {
	sample := receptors("CASSLGGNTEAFF", "CASSLGQNTEAFF", "CASSLGWNTEAFF")
	bg := receptors("CAWWWWWWWWWWWWWWF")
	bg.Receptors[0].Clonotype = -1
	res, err := Find(context.Background(), sample, bg, nil, "", Options{Control: 1, MaxRadius: 48, Step: 2, MinNeighbors: 1}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	for _, c := range res.Centers {
		if sample.Receptors[c.Receptor].CDR3 == "CASSLGGNTEAFF" {
			got = c.Motif
		}
	}
	// The CDR3 less its first three and last two residues, with the three that differ in brackets.
	if got != "(SLG[GQW]NTEA)" {
		t.Errorf("the motif is %q", got)
	}
	// A shorter neighbor leaves a position it has no residue at.
	shorter := receptors("CASSLGGNTEAFF", "CASSLGNTEAFF")
	res, err = Find(context.Background(), shorter, bg, nil, "", Options{Control: 1, MaxRadius: 48, Step: 2, MinNeighbors: 2}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Centers) == 0 {
		t.Fatal("no centroid")
	}
	// On the longer centroid, the shorter neighbor leaves a position with no residue.
	marked := false
	for _, c := range res.Centers {
		if shorter.Receptors[c.Receptor].CDR3 != "CASSLGGNTEAFF" {
			continue
		}
		marked = true
		if !strings.Contains(c.Motif, "?") {
			t.Errorf("a shorter neighbor leaves no optional position: %q", c.Motif)
		}
	}
	if !marked {
		t.Error("the longer centroid has no neighborhood")
	}
}

// A shuffled background keeps the dataset's genes and its CDR3s, no longer together.
func TestTheShuffledBackground(t *testing.T) {
	sample := receptors("CASSLGGNTEAFF", "CASSPRQGAYEQYF", "CAWTTTTTTTTTTF", "CASSQETQYF")
	for i := range sample.Receptors {
		sample.Receptors[i].V = []string{"TRBV5-1", "TRBV7-9", "TRBV5-1", "TRBV20-1"}[i]
	}
	bg, src := ShuffledBackground(sample, 7)
	if src.Kind != "shuffled" || src.Receptors != 4 || bg.Len() != 4 {
		t.Fatalf("%+v", src)
	}
	was, now := map[string]int{}, map[string]int{}
	genes, nowGenes := map[string]int{}, map[string]int{}
	moved := 0
	for i, r := range bg.Receptors {
		was[sample.Receptors[i].CDR3]++
		now[r.CDR3]++
		genes[sample.Receptors[i].V]++
		nowGenes[r.V]++
		if r.V != sample.Receptors[i].V {
			t.Errorf("receptor %d changed its V gene", i)
		}
		if r.CDR3 != sample.Receptors[i].CDR3 {
			moved++
		}
		if r.Clonotype != -1 {
			t.Errorf("a background receptor names a clonotype")
		}
	}
	for k, n := range was {
		if now[k] != n {
			t.Errorf("%s: %d CDR3s, %d after the shuffle", k, n, now[k])
		}
	}
	if moved == 0 {
		t.Error("no CDR3 moved")
	}
}

// A model background is drawn with the dataset's V and J genes, and weighs each receptor by what
// recombination makes over what the dataset holds.
func TestTheModelBackground(t *testing.T) {
	m := pgen.Simulator()
	sample := receptors("CASSLGGNTEAFF", "CASSPRQGAYEQYF", "CASSQETQYF")
	sample.Receptors[0].V, sample.Receptors[0].J = "TRBV5-1", "TRBJ1-1"
	sample.Receptors[1].V, sample.Receptors[1].J = "TRBV5-1", "TRBJ1-1"
	sample.Receptors[2].V, sample.Receptors[2].J = "TRBV20-1", "TRBJ2-7"
	bg, src, err := ModelBackground(context.Background(), sample, m, "clonoloom_human_T_beta", "1", 600, 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if src.Kind != "model" || !src.Weighted || src.Pairs != 2 || src.Missing != 0 || bg.Len() < 500 {
		t.Fatalf("%+v, %d receptors", src, bg.Len())
	}
	counts := map[string]int{}
	weights := map[string]float64{}
	for i, r := range bg.Receptors {
		if r.CDR3 == "" || r.Clonotype != -1 {
			t.Fatalf("%+v", r)
		}
		if strings.ContainsAny(r.CDR3, "*X") {
			t.Errorf("%s is not productive", r.CDR3)
		}
		key := gene(r.V) + "|" + gene(r.J)
		counts[key]++
		weights[key] = bg.weight(i)
	}
	// Two thirds of the draws have the pair two of the three receptors use, and weigh half as
	// much each: the dataset holds that pair twice as often as recombination would.
	if counts["TRBV5-1|TRBJ1-1"] < 2*counts["TRBV20-1|TRBJ2-7"]-20 {
		t.Errorf("drawn by the dataset's usage: %v", counts)
	}
	if !(weights["TRBV5-1|TRBJ1-1"] < weights["TRBV20-1|TRBJ2-7"]) {
		t.Errorf("the commoner pair does not weigh less: %v", weights)
	}
	// The model's usage is a distribution over the pairs it can make.
	total := 0.0
	for _, p := range m.Usage() {
		total += p
	}
	if math.Abs(total-1) > 1e-9 {
		t.Errorf("the model's V–J usage sums to %g", total)
	}
	// Genes the model lacks are counted, not drawn.
	sample.Receptors[2].V = "TRAV1-2"
	_, src, err = ModelBackground(context.Background(), sample, m, "clonoloom_human_T_beta", "1", 300, 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if src.Missing != 1 || src.Pairs != 1 {
		t.Errorf("%+v", src)
	}
}

func TestTheOptionsAreChecked(t *testing.T) {
	for name, o := range map[string]Options{
		"no control rate":   {Control: 0, MaxRadius: 48, Step: 2, MinNeighbors: 1},
		"a rate above one":  {Control: 2, MaxRadius: 48, Step: 2, MinNeighbors: 1},
		"a negative radius": {Control: 1e-6, MaxRadius: -2, Step: 2, MinNeighbors: 1},
		"no step":           {Control: 1e-6, MaxRadius: 48, Step: 0, MinNeighbors: 1},
		"a step too large":  {Control: 1e-6, MaxRadius: 48, Step: 50, MinNeighbors: 1},
		"no neighbors":      {Control: 1e-6, MaxRadius: 48, Step: 2, MinNeighbors: 0},
	} {
		if err := o.Check(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := DefaultOptions().Check(); err != nil {
		t.Errorf("the defaults: %v", err)
	}
	if got := (Options{MaxRadius: 6, Step: 2}).thresholds(); len(got) != 4 || got[3] != 6 {
		t.Errorf("thresholds: %v", got)
	}
}

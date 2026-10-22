package pgen

import (
	"context"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	"clonoloom/internal/random"
)

// Invariants of the probability under ClonoLoom's own model (built in, so these need no data;
// validation/pgen_test.go checks it against OLGA and against the simulator).

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-12*math.Max(math.Abs(a), math.Abs(b))
}

// draws are n productive receptors drawn from the model.
func draws(m *Model, n int, seed uint64) []Draw {
	g := NewGenerator(m)
	r := random.New(seed, 0)
	var out []Draw
	for len(out) < n {
		if d := g.Draw(r); d.Productive {
			out = append(out, d)
		}
	}
	return out
}

func TestReadsCDR3s(t *testing.T) {
	if got := Translate("TGTGCCAGCAGTTTTTAA"); got != "CASSF*" {
		t.Errorf("Translate: %s", got)
	}
	if got := Translate("TGTGCNAGC"); got != "CXS" {
		t.Errorf("Translate with N: %s", got)
	}
	if _, err := AminoAcids("CASS1F"); err == nil {
		t.Error("an amino-acid CDR3 with 1 read")
	}
	if a, err := AminoAcids("cass"); err != nil || len(a) != 4 {
		t.Errorf("lower case: %v %v", a, err)
	}
	if _, err := Nucleotides("TGTG"); err == nil {
		t.Error("a nucleotide CDR3 of 4 read")
	}
	if _, err := Nucleotides("TGTNCC"); err == nil {
		t.Error("a nucleotide CDR3 with N read")
	}
}

// An ambiguous residue's probability is the sum of its amino acids'; X's, of the 20 (no stop).
func TestAmbiguousResiduesSumTheirAminoAcids(t *testing.T) {
	m := Simulator()
	for _, d := range draws(m, 5, 1) {
		at := len(d.AA) / 2
		with := func(c byte) float64 {
			p, err := m.AA(d.AA[:at]+string(c)+d.AA[at+1:], nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			return p
		}
		sum := 0.0
		for _, a := range "ACDEFGHIKLMNPQRSTVWY" {
			sum += with(byte(a))
		}
		if x := with('X'); !near(x, sum) || x <= 0 {
			t.Errorf("%s with X at %d: %g, the amino acids' sum %g", d.AA, at, x, sum)
		}
		if b := with('B'); !near(b, with('D')+with('N')) {
			t.Errorf("%s with B: %g, D and N %g", d.AA, b, with('D')+with('N'))
		}
	}
}

// The genes' masks partition the probability: each V's (and each J's) sum to the CDR3's.
func TestMasksPartitionTheProbability(t *testing.T) {
	m := Simulator()
	for _, d := range draws(m, 5, 2) {
		all, err := m.AA(d.AA, nil, nil)
		if err != nil || all <= 0 {
			t.Fatalf("%s: %g %v", d.AA, all, err)
		}
		for _, segment := range []byte{'V', 'J'} {
			genes := m.V
			if segment == 'J' {
				genes = m.J
			}
			sum := 0.0
			for i := range genes {
				mask := make([]bool, len(genes))
				mask[i] = true
				var p float64
				if segment == 'V' {
					p, _ = m.AA(d.AA, mask, nil)
				} else {
					p, _ = m.AA(d.AA, nil, mask)
				}
				sum += p
			}
			if !near(sum, all) {
				t.Errorf("%s: the %c genes' probabilities sum to %g, the CDR3's %g", d.AA, segment, sum, all)
			}
		}
		// The genes it was drawn with make it.
		vm, err := m.Mask('V', d.V)
		if err != nil {
			t.Fatal(err)
		}
		jm, err := m.Mask('J', d.J)
		if err != nil {
			t.Fatal(err)
		}
		if p, _ := m.AA(d.AA, vm, jm); p <= 0 || p > all {
			t.Errorf("%s with %s and %s: %g (alone %g)", d.AA, d.V, d.J, p, all)
		}
	}
}

// A receptor drawn has a probability as nucleotides, at most its amino acids'.
func TestDrawsHaveAProbability(t *testing.T) {
	m := Simulator()
	for _, d := range draws(m, 200, 3) {
		nt, err := m.NT(d.NT, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		aa, _ := m.AA(d.AA, nil, nil)
		if !(nt > 0) || nt > aa*(1+1e-12) || Translate(d.NT) != d.AA {
			t.Fatalf("%s (%s): nucleotides %g, amino acids %g", d.NT, d.AA, nt, aa)
		}
	}
}

// All answers what each query alone does, the same on any number of workers; a CDR3 that cannot be
// read is NaN.
func TestAllIsEachAlone(t *testing.T) {
	m := Simulator()
	var queries []Query
	for _, d := range draws(m, 600, 4) {
		queries = append(queries, Query{CDR3: d.AA}, Query{CDR3: d.NT, NT: true})
	}
	queries = append(queries, Query{CDR3: "CASS1F"}, Query{CDR3: "TGTG", NT: true})
	one, err := m.All(context.Background(), queries, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	many, err := m.All(context.Background(), queries, 7, func(done, of int) { calls.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range queries {
		if math.Float64bits(one[i]) != math.Float64bits(many[i]) {
			t.Fatalf("%s: %g on one worker, %g on seven", q.CDR3, one[i], many[i])
		}
		if i >= len(queries)-2 {
			if !math.IsNaN(one[i]) {
				t.Errorf("%s: %g, not NaN", q.CDR3, one[i])
			}
			continue
		}
		alone, _ := m.AA(q.CDR3, nil, nil)
		if q.NT {
			alone, _ = m.NT(q.CDR3, nil, nil)
		}
		if one[i] != alone {
			t.Errorf("%s: %g, alone %g", q.CDR3, one[i], alone)
		}
	}
	if calls.Load() == 0 {
		t.Error("no progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.All(ctx, queries, 2, nil); err == nil {
		t.Error("a cancelled computation finished")
	}
}

func TestMaskNamesGenes(t *testing.T) {
	m := Simulator()
	mask, err := m.Mask('V', "TRBV5-1")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i, on := range mask {
		if on {
			n++
			if name := m.V[i].Name; name != "TRBV5-1" && !strings.HasPrefix(name, "TRBV5-1*") {
				t.Errorf("TRBV5-1 names %s", m.V[i].Name)
			}
		}
	}
	if n == 0 {
		t.Error("TRBV5-1 names no gene")
	}
	if tie, err := m.Mask('J', "TRBJ1-1, TRBJ2-7"); err != nil || count(tie) < 2 {
		t.Errorf("a tie: %v %v", tie, err)
	}
	if _, err := m.Mask('V', "TRAV1-1"); err == nil {
		t.Error("a gene the model lacks named")
	}
}

func count(mask []bool) int {
	n := 0
	for _, on := range mask {
		if on {
			n++
		}
	}
	return n
}

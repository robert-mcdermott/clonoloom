package motifs

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"math"
	"math/bits"
	"math/rand/v2"
	"strings"
	"testing"
)

func records(seqs ...string) []Record {
	out := []Record{}
	for _, s := range seqs {
		out = append(out, Record{ID: ID("TRBV5-1", s), V: "TRBV5-1", Junction: s, Abundance: 10})
	}
	return out
}

// Exhaust all 70 allocations of four labels among eight fixed pooled units.
// This checks conditional sequence-label calibration, not biological FDR.
func TestMotifConditionalNull(t *testing.T) {
	pool := records("CASSQGHTANEQYF", "CASSQGHTGNEQYF", "CASSQGHTSNEQYF", "CASSQGHTTNEQYF",
		"CASSGATGANEQYF", "CASSNATGANEQYF", "CASSRATGANEQYF", "CASSYATGANEQYF")
	var ids string
	tails := []float64{}
	for mask := 0; mask < 256; mask++ {
		if bits.OnesCount(uint(mask)) != 4 {
			continue
		}
		q, b := []Record{}, []Record{}
		for i, row := range pool {
			if mask&(1<<i) != 0 {
				q = append(q, row)
			} else {
				b = append(b, row)
			}
		}
		r, err := Compute(context.Background(), q, b, Options{MinK: 4, MaxK: 4}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var keys strings.Builder
		for _, g := range r.Groups {
			keys.WriteString(g.ID)
			if g.Kind == "local" && g.Motif == "QGHT" && g.Start == 4 {
				tails = append(tails, float64(g.P))
			}
		}
		if ids == "" {
			ids = keys.String()
		} else if ids != keys.String() {
			t.Fatal("arm labels changed pooled family")
		}
	}
	if len(tails) != 70 {
		t.Fatal(len(tails))
	}
	for _, alpha := range []float64{1.0 / 70, .05, .25, .5, 1} {
		n := 0
		for _, p := range tails {
			if p <= alpha+1e-12 {
				n++
			}
		}
		if float64(n)/70 > alpha+1e-12 {
			t.Fatalf("conditional null: %d/70 exceeds %g", n, alpha)
		}
	}
}

func TestMotifBoundsAndCancellation(t *testing.T) {
	q := records("CASSQGHTANEQYF")
	if _, err := Compute(context.Background(), make([]Record, 4000), q, Options{}, nil); err == nil {
		t.Fatal("unit bound")
	}
	rng := rand.New(rand.NewPCG(64, 128))
	large := []Record{}
	for i := 0; i < 400; i++ {
		var s strings.Builder
		s.WriteByte('C')
		for j := 0; j < 38; j++ {
			s.WriteByte(Alphabet[rng.IntN(len(Alphabet))])
		}
		s.WriteByte('F')
		large = append(large, records(s.String())[0])
	}
	if _, err := Compute(context.Background(), large, q, Options{}, nil); err == nil || !strings.Contains(err.Error(), "100000") {
		t.Fatal("family bound", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := Compute(ctx, q, records("CASSGATGANEQYF"), Options{}, func(float64) { cancel() }); err != context.Canceled {
		t.Fatal("mid-analysis cancellation", err)
	}
}

func TestMotifExtremeTailRetainsLogScores(t *testing.T) {
	letters := "ACDEFGIKLMNPQRSTVWY" // H is absent from every background junction.
	q, b := []Record{}, []Record{}
	for i := 0; i < 1000; i++ {
		n := i
		var word strings.Builder
		for j := 0; j < 3; j++ {
			word.WriteByte(letters[n%len(letters)])
			n /= len(letters)
		}
		q = append(q, records("CASSHH" + word.String() + "GGNEQYF")[0])
		b = append(b, records("CASSAA" + word.String() + "GGNEQYF")[0])
	}
	r, err := Compute(context.Background(), q, b, Options{MinK: 2, MaxK: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range r.Groups {
		if g.Kind == "local" && g.Motif == "HH" && g.Start == 4 {
			if len(g.Query) != 1000 || len(g.Background) != 0 || float64(g.P) != 0 || float64(g.LogP) > -1000 || math.IsInf(float64(g.LogQ), 0) || math.IsNaN(float64(g.LogQ)) || float64(g.Score) < 300 || !math.IsNaN(float64(g.Fold)) || !g.Reported {
				t.Fatal("extreme tail lost", g)
			}
			return
		}
	}
	t.Fatal("extreme signature absent")
}
func TestMotifPoliciesAndFamily(t *testing.T) {
	q := records("CASSGITGQANEQYF", "CASSGLTGQANEQYF", "CASSGMTGQANEQYF", "CASSGVTGQANEQYF")
	b := records("CASSGATGQANEQYF", "CASSGSTGQANEQYF", "CASSQGHTGNEQYF")
	r, e := Compute(context.Background(), q, b, Options{}, nil)
	if e != nil {
		t.Fatal(e)
	}
	loose, e := Compute(context.Background(), q, b, Options{Q: 1, MinFold: 1, MinMembers: 2}, nil)
	if e != nil || loose.Family != r.Family {
		t.Fatal(e)
	}
	found := false
	for i, g := range r.Groups {
		if g.LogP != loose.Groups[i].LogP && !(math.IsNaN(float64(g.LogP)) && math.IsNaN(float64(loose.Groups[i].LogP))) {
			t.Fatal("reporting changed test")
		}
		if g.Kind == "global" && g.Allowed == "ILMV" {
			found = true
			if len(g.Query) != 4 || len(g.Background) != 0 {
				t.Fatal(g)
			}
		}
	}
	if !found {
		t.Fatal("conservative global group absent")
	}
	for i := range q {
		q[i].Abundance *= 1e6
	}
	weighted, e := Compute(context.Background(), q, b, Options{}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(r.Groups)
	bb, _ := json.Marshal(weighted.Groups)
	if !bytes.Equal(a, bb) {
		t.Fatal("abundance changed scientific scores")
	}
	var buf bytes.Buffer
	if e = CSV(&buf, r); e != nil {
		t.Fatal(e)
	}
	rows, e := csv.NewReader(&buf).ReadAll()
	if e != nil || len(rows) != r.Family+1 {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Compute(ctx, q, b, Options{}, nil); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e = Compute(context.Background(), append(q, q[0]), b, Options{}, nil); e == nil {
		t.Fatal("duplicate unit accepted")
	}
	if _, e = Compute(context.Background(), q, nil, Options{}, nil); e == nil {
		t.Fatal("no background")
	}
	if _, e = Compute(context.Background(), q, b, Options{MinK: 1}, nil); e == nil {
		t.Fatal("invalid k")
	}
	for _, j := range []string{"CASXXXTGQANEQYF", "ASSGITGQANEQYF", "CASSGITGQANEQYA", "CASSF"} {
		if Valid("TRBV5-1", j) {
			t.Fatal(j)
		}
	}
}
func TestLocalWindowsNeverChainAcrossPositions(t *testing.T) {
	q := records("CASSQGHTGANEQYF", "CASSGQGHTANEQYF", "CASSGGAQGHTEQYF")
	b := records("CASSASGTNREQYF")
	r, e := Compute(context.Background(), q, b, Options{}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, g := range r.Groups {
		if g.Kind == "local" {
			if g.End-g.Start > 3 {
				t.Fatal(g)
			}
			for _, i := range g.Query {
				at := []int{}
				s := r.Query[i].Junction
				for p := 3; p+len(g.Motif) <= len(s)-3; p++ {
					if strings.HasPrefix(s[p:], g.Motif) {
						at = append(at, p+1)
					}
				}
				valid := false
				for _, p := range at {
					valid = valid || p >= g.Start && p <= g.End
				}
				if !valid {
					t.Fatal("position mismatch", g)
				}
			}
		}
	}
	if got := cliques("ANS"); strings.Join(got, ",") != "AS,NS" {
		t.Fatal("transitive substitutions merged", got)
	}
}

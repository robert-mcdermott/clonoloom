package convergence

import (
	"clonoloom/internal/motifs"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"testing"
)

func fixture() Input {
	in := Input{Selection: Selection{Species: "human", Groups: []string{"g"}, Alleles: []string{"A*02:01"}}, Groups: []Signature{{ID: "g", Label: "Declared group"}}}
	for i := 0; i < 8; i++ {
		sample := fmt.Sprintf("sample-%d", i)
		id := fmt.Sprintf("subject-%d", i)
		in.Selection.Samples = append(in.Selection.Samples, sample)
		calls := []string{"A*01:01", "A*03:01"}
		if i < 4 {
			calls = []string{"A*02:01", "A*03:01"}
		}
		in.Selection.Subjects = append(in.Selection.Subjects, Subject{ID: id, Samples: []string{sample}, Typing: map[string][]string{"A": calls}})
		junction := "CASS" + string("AGNPSTVY"[i]) + "QGHTNEQYF"
		rec := motifs.Record{ID: motifs.ID("TRBV5-1", junction), V: "TRBV5-1", Junction: junction, Samples: []string{sample}, Abundance: float64(i + 1)}
		in.Records = append(in.Records, rec)
		if i < 4 {
			in.Groups[0].Members = append(in.Groups[0].Members, rec.ID)
		}
	}
	return in
}
func TestConvergenceTypingAndPolicies(t *testing.T) {
	for _, v := range []string{"A*02:01", "HLA-A*02:01:01:01"} {
		a, l, e := Allele(v)
		if e != nil || a != "HLA-A*02:01" || l != "A" {
			t.Fatal(a, l, e)
		}
	}
	for _, v := range []string{"A*02", "A*0201", "A2", "A*02:01G", "A*02:01:01G", "A*02:01N", "A*02:01P", "A*02:01/A*03:01", "DRB3*01:01", "HLA-A*02:01:01:01:01"} {
		if _, _, e := Allele(v); e == nil {
			t.Fatal("unsupported allele accepted", v)
		}
	}
	in := fixture()
	base, e := Compute(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	x := base.Associations[0]
	if x.N != 8 || x.Shared != 4 || math.Abs(float64(x.P)-1.0/70) > 1e-14 {
		t.Fatal(x)
	}
	// Repeated sample and extra receptor observations do not add subject replication.
	in.Selection.Samples = append(in.Selection.Samples, "repeat")
	in.Selection.Subjects[0].Samples = append(in.Selection.Subjects[0].Samples, "repeat")
	in.Records[0].Samples = append(in.Records[0].Samples, "repeat", "repeat")
	in.Records[0].Abundance = 1e9
	repeated, e := Compute(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(base.Associations)
	b, _ := json.Marshal(repeated.Associations)
	if string(a) != string(b) {
		t.Fatal("replication inflated")
	}
	in.Selection.Subjects[0].Typing["A"] = []string{"A*02:01", "A*02:01"}
	homo, e := Compute(context.Background(), in)
	if e != nil || homo.Associations[0].Carriers != 4 {
		t.Fatal(e, "homozygote counted twice")
	}
	in.Selection.Subjects[0].Typing["A"] = []string{"A*02:01"}
	partial, e := Compute(context.Background(), in)
	if e != nil || partial.Associations[0].N != 7 || partial.Associations[0].Carriers != 3 || partial.Associations[0].Excluded != 1 {
		t.Fatal(e, "partial became negative")
	}
	in.Selection.Alleles = []string{}
	only, e := Compute(context.Background(), in)
	if e != nil || only.Family != 0 || only.ObservedSubjects != 8 || len(only.Recurrence[0].Subjects) != 4 {
		t.Fatal(e, "recurrence only")
	}
	in.Records = nil
	empty, e := Compute(context.Background(), Input{Selection: in.Selection, Groups: []Signature{{ID: "g", Label: "empty"}}})
	if e != nil || empty.ObservedSubjects != 0 || !math.IsNaN(float64(empty.Recurrence[0].Frequency)) {
		t.Fatal(e, "missing receptor evidence became absence")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Compute(ctx, fixture()); e != context.Canceled {
		t.Fatal(e)
	}
	for _, mutate := range []func(*Input){func(in *Input) { in.Selection.Species = "mouse" }, func(in *Input) { in.Selection.Alleles = []string{"A*02:01", "HLA-A*02:01:01"} }, func(in *Input) { in.Selection.Subjects[0].Samples = []string{"unknown"} }, func(in *Input) { in.Selection.Subjects[1].ID = in.Selection.Subjects[0].ID }, func(in *Input) { in.Groups[0].Members = append(in.Groups[0].Members, "unknown") }, func(in *Input) { in.Selection.Groups = make([]string, 129) }} {
		v := fixture()
		mutate(&v)
		if _, e = Compute(context.Background(), v); e == nil {
			t.Fatal("invalid evidence accepted")
		}
	}
}
func TestConvergenceConditionalNull(t *testing.T) {
	values := []float64{}
	for mask := 0; mask < 256; mask++ {
		if bits.OnesCount(uint(mask)) != 4 {
			continue
		}
		in := fixture()
		for i := range in.Selection.Subjects {
			a := "A*01:01"
			if mask&(1<<i) != 0 {
				a = "A*02:01"
			}
			in.Selection.Subjects[i].Typing["A"] = []string{a, "A*03:01"}
		}
		result, e := Compute(context.Background(), in)
		if e != nil {
			t.Fatal(e)
		}
		values = append(values, float64(result.Associations[0].P))
		if result.Family != 1 {
			t.Fatal("family")
		}
	}
	for _, alpha := range []float64{1.0 / 70, .05, .25, .5, 1} {
		n := 0
		for _, v := range values {
			if v <= alpha+1e-12 {
				n++
			}
		}
		if float64(n)/70 > alpha+1e-12 {
			t.Fatal(alpha, n)
		}
	}
}

func TestConvergenceEmptyTypingIdentity(t *testing.T) {
	left, right := fixture().Selection, fixture().Selection
	left.Subjects[0].Typing = map[string][]string{"A": {}}
	right.Subjects[0].Typing = nil
	a, err := Normalize(left)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Normalize(right)
	if err != nil {
		t.Fatal(err)
	}
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(ab) != string(bb) {
		t.Fatal("equivalent unknown typing changes artifact identity")
	}
	if _, retained := left.Subjects[0].Typing["A"]; !retained {
		t.Fatal("normalization mutated caller")
	}
}

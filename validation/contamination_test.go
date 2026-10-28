package validation

import (
	"clonoloom/internal/contamination"
	"clonoloom/internal/seq"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestContaminationIndependentReference(t *testing.T) {
	raw, e := os.ReadFile("reference/contamination.json")
	if e != nil {
		t.Fatal(e)
	}
	var ref struct {
		Input            contamination.Input
		Pairs            []contamination.Pair
		Matches          []contamination.Match
		SameSubjectPairs int
	}
	if e = json.Unmarshal(raw, &ref); e != nil {
		t.Fatal(e)
	}
	for i := range ref.Input.Receptors {
		ref.Input.Receptors[i].AA = seq.Translate(ref.Input.Receptors[i].NT)
	}
	r, e := contamination.Compute(context.Background(), ref.Input)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Pairs) != len(ref.Pairs) || len(r.Matches) != len(ref.Matches) || r.SameSubjectPairs != ref.SameSubjectPairs {
		t.Fatal("counts")
	}
	close := func(a, b float64) {
		t.Helper()
		if math.IsNaN(a) && math.IsNaN(b) {
			return
		}
		if math.IsNaN(a) || math.IsNaN(b) || math.Abs(a-b) > 1e-12*math.Max(1, math.Abs(b)) {
			t.Fatalf("%g != %g", a, b)
		}
	}
	for i, x := range r.Pairs {
		w := ref.Pairs[i]
		if x.A != w.A || x.B != w.B || x.Shared != w.Shared || x.Review != w.Review || x.HighPgen != w.HighPgen || x.UnknownPgen != w.UnknownPgen {
			t.Fatal(x, w)
		}
		for _, v := range [][2]float64{{x.SharedA, w.SharedA}, {x.SharedB, w.SharedB}, {float64(x.FractionA), float64(w.FractionA)}, {float64(x.FractionB), float64(w.FractionB)}} {
			close(v[0], v[1])
		}
	}
	for i, x := range r.Matches {
		w := ref.Matches[i]
		if x.Receptor != w.Receptor || x.A != w.A || x.B != w.B || x.Review != w.Review || x.Pattern != w.Pattern || x.PgenClass != w.PgenClass {
			t.Fatal(x, w)
		}
		for _, v := range [][2]float64{{x.CountA, w.CountA}, {x.CountB, w.CountB}, {x.FractionA, w.FractionA}, {x.FractionB, w.FractionB}, {x.Ratio, w.Ratio}} {
			close(v[0], v[1])
		}
	}
	raw, e = json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if digest != "7fa8a3cf06cc2df9064452f95af6fc27bfbbe1eb447b215e8ac6240a59beadaa" {
		t.Fatalf("contamination result digest %s", digest)
	}
}

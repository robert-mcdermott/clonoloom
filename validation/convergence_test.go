package validation

import (
	"clonoloom/internal/convergence"
	"clonoloom/internal/motifs"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestConvergenceIndependentReference(t *testing.T) {
	raw, err := os.ReadFile("reference/convergence.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Subjects []convergence.Subject
		Records  []struct {
			V, Junction string
			Samples     []string
		}
		Groups []struct {
			ID, Label string
			Members   []int
		}
		Alleles    []string
		Recurrence []struct {
			ID               string
			Subjects         []string
			ObservedSubjects int
			Frequency        float64
		}
		Associations []convergence.Association
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	in := convergence.Input{Selection: convergence.Selection{Species: "human", Subjects: ref.Subjects, Alleles: ref.Alleles}}
	for _, s := range ref.Subjects {
		in.Selection.Samples = append(in.Selection.Samples, s.Samples...)
	}
	for _, r := range ref.Records {
		in.Records = append(in.Records, motifs.Record{ID: motifs.ID(r.V, r.Junction), V: r.V, Junction: r.Junction, Samples: r.Samples})
	}
	for _, g := range ref.Groups {
		in.Selection.Groups = append(in.Selection.Groups, g.ID)
		group := convergence.Signature{ID: g.ID, Label: g.Label, Members: []string{}}
		for _, i := range g.Members {
			group.Members = append(group.Members, in.Records[i].ID)
		}
		in.Groups = append(in.Groups, group)
	}
	got, err := convergence.Compute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	close := func(a, b float64) {
		t.Helper()
		if math.IsNaN(a) && math.IsNaN(b) {
			return
		}
		if math.IsNaN(a) || math.IsNaN(b) || math.Abs(a-b) > 2e-13 {
			t.Fatalf("%g != %g", a, b)
		}
	}
	if len(got.Associations) != len(ref.Associations) {
		t.Fatal("family")
	}
	for i, x := range got.Associations {
		w := ref.Associations[i]
		if x.Group != w.Group || x.Allele != w.Allele || x.N != w.N || x.Carriers != w.Carriers || x.GroupSubjects != w.GroupSubjects || x.Shared != w.Shared || x.Excluded != w.Excluded {
			t.Fatal("subject table", x, w)
		}
		for _, pair := range [][2]float64{{float64(x.P), float64(w.P)}, {float64(x.Q), float64(w.Q)}, {float64(x.Difference), float64(w.Difference)}} {
			close(pair[0], pair[1])
		}
		for j, f := range []convergence.Frequency{x.Carrier, x.Noncarrier} {
			expected := []convergence.Frequency{w.Carrier, w.Noncarrier}[j]
			if f.Matches != expected.Matches || f.Total != expected.Total {
				t.Fatal(f, expected)
			}
			close(float64(f.Estimate), float64(expected.Estimate))
			close(float64(f.Lower), float64(expected.Lower))
			close(float64(f.Upper), float64(expected.Upper))
		}
	}
	for i, g := range got.Recurrence {
		w := ref.Recurrence[i]
		if g.ID != w.ID || fmt.Sprint(g.Subjects) != fmt.Sprint(w.Subjects) || g.ObservedSubjects != w.ObservedSubjects {
			t.Fatal(g, w)
		}
		close(float64(g.Frequency), w.Frequency)
	}
	data, _ := json.Marshal(got)
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest != "18298429e34838f6882abb19a5a6ef7b76a9f1e4d964c4c19685a7ee5d596945" {
		t.Fatalf("result digest %s", digest)
	}
	t.Logf("convergence result digest %s", digest)
}

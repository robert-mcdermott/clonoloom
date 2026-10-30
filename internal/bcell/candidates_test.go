package bcell

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func candidateFixture() (FamilyResult, SHMResult, IsotypeResult) {
	threshold := 0.1
	f := FamilyResult{Options: FamilyOptions{Mode: "paired", Scope: "subject", Threshold: &threshold}, SingleCell: true, Unit: "cells", Families: []Family{{ID: "a", Scope: "subject:S"}, {ID: "b", Scope: "subject:S"}, {ID: "c", Scope: "subject:S"}, {ID: "d", Scope: "subject:S"}, {ID: "other", Scope: "subject:T"}}}
	s := SHMResult{}
	iso := IsotypeResult{}
	for i, x := range []struct {
		family, sample, class string
		mutations, bases      int
	}{{"a", "after", "IgG", 1, 100}, {"a", "after", "Unresolved", 2, 100}, {"b", "after", "IgM", 0, 100}, {"b", "after", "IgM", 0, 0}, {"c", "after", "IgG", 0, 0}, {"a", "before", "IgM", 0, 100}, {"b", "before", "IgM", 0, 100}, {"b", "before", "IgM", 0, 100}, {"d", "before", "IgM", 0, 100}, {"other", "other", "IgG", 5, 100}} {
		id := string(rune('a' + i))
		scope, subject := "subject:S", "S"
		if x.family == "other" {
			scope, subject = "subject:T", "T"
		}
		row := FamilyRow{ID: "row-" + id, Source: FamilySource{Name: "artificial.tsv", SHA256: "artificial"}, Line: uint32(i + 2), SequenceID: id, Locus: "IGH"}
		f.Observations = append(f.Observations, FamilyObservation{ID: id, Family: x.family, Sample: x.sample, Subject: subject, Scope: scope, Cell: id, Rows: []FamilyRow{row}})
		s.Rows = append(s.Rows, SHMRow{FamilyRow: row, Family: x.family, Observation: id, Sample: x.sample, Regions: []SHMRegion{{Region: "all", SHMCounts: SHMCounts{Mutations: x.mutations, EvaluableBases: x.bases}}}})
		iso.Observations = append(iso.Observations, IsotypeObservation{ID: id, Family: x.family, Sample: x.sample, Subject: subject, Scope: scope, Cell: id, Class: x.class})
	}
	f.Observations = append(f.Observations, FamilyObservation{ID: "excluded", Sample: "after", Excluded: "ambiguous chains"})
	return f, s, iso
}
func TestCandidateReviewRanksAndDenominators(t *testing.T) {
	f, s, iso := candidateFixture()
	for _, test := range []struct {
		metric, direction, order string
		ranks                    []int
	}{{"size", "descending", "abcd", []int{1, 1, 3, 4}}, {"mutation", "descending", "abcd", []int{1, 2, 2, 2}}, {"isotype", "ascending", "bcad", []int{1, 2, 3, 3}}, {"change", "descending", "cabd", []int{1, 2, 3, 4}}} {
		t.Run(test.metric, func(t *testing.T) {
			options := CandidateOptions{Sample: "after", Baseline: "before", PreferredClass: "IgG", Criteria: []CandidateCriterion{{test.metric, test.direction}}}
			r, e := CandidateReview(context.Background(), f, &s, &iso, options)
			if e != nil {
				t.Fatal(e)
			}
			if r.CurrentTotal != 5 || r.BaselineTotal != 4 || r.CurrentExcluded != 1 || len(r.Rows) != 4 || r.Unit != "sample-scoped cells" {
				t.Fatal(r)
			}
			for i, row := range r.Rows {
				if row.Family != string(test.order[i]) || row.Rank != test.ranks[i] {
					t.Fatal(r.Rows)
				}
				if row.Family == "a" && (row.Metrics["mutation"].Numerator != 3 || row.Metrics["mutation"].Denominator != 200 || row.Metrics["isotype"].Value != nil) {
					t.Fatal(row)
				}
			}
			digestBytes, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{
				"size":     "b9f626d31927c7e5e754c80a7f81efd5e567ca0b4f241527f3258016a2aa19c7",
				"mutation": "857f7707128a4a846cf8bbc466496612a5ec65060962cb496bd8404146a6d7e1",
				"isotype":  "2bd081d7aa546d404a7c85bc3416a4bcb54037c808163969a541db4f8058d2e5",
				"change":   "497398fc0d899afe6c47020acc23a735b53eeb03fb7dfaa6dc784996b4f000dc",
			}[test.metric]
			if got := fmt.Sprintf("%x", sha256.Sum256(digestBytes)); got != want {
				t.Fatalf("%s native result digest %s, want %s", test.metric, got, want)
			}
			if dir := os.Getenv("CLONOLOOM_CANDIDATES_OUT"); dir != "" {
				os.MkdirAll(dir, 0700)
				raw, e := json.MarshalIndent(map[string]any{"families": f, "shm": s, "isotypes": iso, "result": r}, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(dir, "core-"+test.metric+".json"), raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
	// A tied primary count defers to the next criterion, including missingness.
	ordered, err := CandidateReview(context.Background(), f, &s, &iso, CandidateOptions{Sample: "after", PreferredClass: "IgG", Criteria: []CandidateCriterion{{"size", "descending"}, {"isotype", "ascending"}}})
	if err != nil {
		t.Fatal(err)
	}
	for i, family := range []string{"b", "a", "c", "d"} {
		if ordered.Rows[i].Family != family || ordered.Rows[i].Rank != i+1 || ordered.Rows[i].Tied != 1 {
			t.Fatal(ordered.Rows)
		}
	}
	// The same rational values must tie even with different denominators.
	s.Rows[0].Regions[0].Mutations = 1
	s.Rows[0].Regions[0].EvaluableBases = 3
	s.Rows[1].Regions[0].Mutations = 1
	s.Rows[1].Regions[0].EvaluableBases = 3
	s.Rows[2].Regions[0].Mutations = 2
	s.Rows[2].Regions[0].EvaluableBases = 6
	s.Rows[3].Regions[0].Mutations = 0
	s.Rows[3].Regions[0].EvaluableBases = 0
	// Remove the unavailable B observation from this control only.
	f.Observations = append(f.Observations[:3], f.Observations[4:]...)
	r, e := CandidateReview(context.Background(), f, &s, &iso, CandidateOptions{Sample: "after", Criteria: []CandidateCriterion{{"mutation", "descending"}}})
	if e != nil || r.Rows[0].Rank != 1 || r.Rows[1].Rank != 1 || r.Rows[0].Tied != 2 {
		t.Fatal(r, e)
	}
}
func TestCandidateRefusals(t *testing.T) {
	f, s, iso := candidateFixture()
	base := CandidateOptions{Sample: "after", Criteria: []CandidateCriterion{{"size", "descending"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := CandidateReview(ctx, f, &s, &iso, base); e != context.Canceled {
		t.Fatal(e)
	}
	for _, o := range []CandidateOptions{{}, {Sample: "missing", Criteria: base.Criteria}, {Sample: "after", Criteria: []CandidateCriterion{{"size", ""}}}, {Sample: "after", Criteria: []CandidateCriterion{{"binding", "descending"}}}, {Sample: "after", Criteria: []CandidateCriterion{{"size", "descending"}, {"size", "ascending"}}}, {Sample: "after", Baseline: "other", Criteria: base.Criteria}, {Sample: "after", Baseline: "after", Criteria: base.Criteria}, {Sample: "after", Criteria: []CandidateCriterion{{"change", "descending"}}}, {Sample: "after", Criteria: []CandidateCriterion{{"isotype", "descending"}}}} {
		if _, e := CandidateReview(context.Background(), f, &s, &iso, o); e == nil {
			t.Fatal("accepted", o)
		}
	}
	for _, m := range []string{"mutation", "isotype"} {
		o := base
		o.Criteria = []CandidateCriterion{{m, "descending"}}
		if _, e := CandidateReview(context.Background(), f, nil, nil, o); e == nil {
			t.Fatal("missing parent", m)
		}
	}
	bad := f
	bad.Observations = append(append([]FamilyObservation{}, f.Observations...), f.Observations[0])
	if _, e := CandidateReview(context.Background(), bad, &s, &iso, base); e == nil {
		t.Fatal("duplicate observation")
	}
	s.Rows[0].Sample = "other"
	if _, e := CandidateReview(context.Background(), f, &s, &iso, base); e == nil {
		t.Fatal("changed source identity")
	}
}

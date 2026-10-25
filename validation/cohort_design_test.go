package validation

import (
	"clonoloom/internal/study"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// A separate SQLite oracle enumerates membership, not ClonoLoom's implementation.
func TestCohortDesignAgainstExhaustiveSQL(t *testing.T) {
	data, err := os.ReadFile("reference/cohort_design.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Cases [][]json.RawMessage `json:"cases"`
	}
	if err = json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	for _, row := range ref.Cases {
		var assignment string
		json.Unmarshal(row[0], &assignment)
		ints := make([]int, 9)
		for j := range ints {
			json.Unmarshal(row[j+1], &ints[j])
		}
		for l, layout := range []string{"independent", "paired"} {
			var want bool
			json.Unmarshal(row[10+l], &want)
			input := study.CohortInput{Dataset: "sql-fixture", Spec: study.CohortSpec{Unit: "subject", Layout: layout, GroupField: "condition", Levels: []string{"A", "B"}, Exclude: map[string]string{}}}
			for n, level := range assignment {
				s := study.CohortSample{Name: fmt.Sprintf("sample-%d", n), Subject: fmt.Sprintf("S%d", n/2), Batch: "run"}
				if level == '0' {
					input.Spec.Exclude[s.Name] = "not selected in this oracle case"
				} else if level == '1' {
					s.Condition = "A"
				} else {
					s.Condition = "B"
				}
				input.Samples = append(input.Samples, s)
			}
			d, err := study.CheckCohort(input)
			if err != nil {
				t.Fatal(err)
			}
			got := []int{d.Groups[0].Samples, d.Groups[1].Samples, d.Groups[0].Subjects, d.Groups[1].Subjects, d.Subjects, d.CompletePairs}
			for j, v := range got {
				if v != ints[j] {
					t.Fatalf("%s %s column %d: %d != %d", assignment, layout, j, v, ints[j])
				}
			}
			if d.OK != want {
				t.Fatalf("%s %s: ok %v want %v; %+v", assignment, layout, d.OK, want, d.Findings)
			}
			for n, sample := range d.Rows {
				if sample.Included != (assignment[n] != '0') {
					t.Fatalf("%s %s: wrong disposition for %s", assignment, layout, sample.Name)
				}
			}
			if d.Included != ints[0]+ints[1] || d.Excluded != 8-d.Included {
				t.Fatalf("%s %s: incorrect population totals", assignment, layout)
			}
			codes := map[string]int{"cohort-repeated-samples": ints[6]}
			if layout == "independent" {
				codes["cohort-shared-subject"] = ints[7]
			} else {
				codes["cohort-incomplete-pairs"] = ints[8]
			}
			for code, count := range codes {
				found := 0
				for _, f := range d.Findings {
					if f.Code == code {
						found += f.Count
					}
				}
				if found != count {
					t.Fatalf("%s %s: %s count %d, want %d", assignment, layout, code, found, count)
				}
			}
		}
	}
	if len(ref.Cases) != 6561 {
		t.Fatalf("only %d cases", len(ref.Cases))
	}
}

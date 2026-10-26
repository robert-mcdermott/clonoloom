package simulate

import (
	"clonoloom/internal/seq"
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestContaminationStudyTruth(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "study")
	s := Presets["contamination-study"]
	sum, e := Run(context.Background(), s, dir, "validation", nil)
	if e != nil {
		t.Fatal(e)
	}
	if sum.Subjects != 8 || sum.Samples != 9 || sum.Rows != 706 {
		t.Fatal(sum)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "truth/observations.tsv"))
	if e != nil {
		t.Fatal(e)
	}
	rd := csv.NewReader(strings.NewReader(string(raw)))
	rd.Comma = '\t'
	rows, e := rd.ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	classes := map[string]int{}
	sequences := map[string]map[string]bool{}
	var synonymous string
	for _, row := range rows[1:] {
		n, e := strconv.Atoi(row[6])
		if e != nil || n < 1 {
			t.Fatal(row)
		}
		classes[row[2]]++
		if sequences[row[0]] == nil {
			sequences[row[0]] = map[string]bool{}
		}
		sequences[row[0]][row[5]] = true
		if row[2] == "synonymous control" {
			synonymous = row[5]
		}
	}
	if classes["planted carryover"] != 3 || classes["independent shared control"] != 27 || classes["carryover source"] != 6 {
		t.Fatal(classes)
	}
	if sequences["D01-1"][synonymous] {
		t.Fatal("synonymous identity was not changed")
	}
	sameAA := false
	for nt := range sequences["D01-1"] {
		if seq.Translate(nt) == seq.Translate(synonymous) {
			sameAA = true
		}
	}
	if !sameAA {
		t.Fatal("synonymous AA changed")
	}
	if got := digest(t, dir); got != "12ade8e6577af384176759f70a5b45c8ede70ea98caf74de7fcbb55984f8a02f" {
		t.Fatalf("contamination study digest %s", got)
	}
	if _, e = Run(context.Background(), s, dir, "validation", nil); e == nil {
		t.Fatal("overwrite")
	}
}

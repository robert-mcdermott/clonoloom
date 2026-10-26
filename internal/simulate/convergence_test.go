package simulate

import (
	"clonoloom/internal/convergence"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestConvergenceStudyTruth(t *testing.T) {
	s := Presets["convergence-study"]
	dir := filepath.Join(t.TempDir(), "study")
	sum, err := Run(context.Background(), s, dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Subjects != 32 || sum.Samples != 36 || sum.Rows != 1080 {
		t.Fatal(sum)
	}
	var subs []convergence.Subject
	raw, err := os.ReadFile(filepath.Join(dir, "truth/subjects.json"))
	if err != nil || json.Unmarshal(raw, &subs) != nil {
		t.Fatal(err)
	}
	if len(subs) != 32 || len(subs[0].Samples) != 2 || len(subs[4].Samples) != 1 || len(subs[30].Typing["A"]) != 0 || len(subs[29].Typing["A"]) != 1 {
		t.Fatal("subject truth")
	}
	raw, err = os.ReadFile(filepath.Join(dir, "truth/receptors.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rd := csv.NewReader(strings.NewReader(string(raw)))
	rd.Comma = '\t'
	rows, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	positive, global, samples := map[string]bool{}, map[string]bool{}, map[string]int{}
	for _, row := range rows[1:] {
		n, e := strconv.Atoi(row[5])
		if e != nil || n < 1 {
			t.Fatal(row)
		}
		samples[row[0]]++
		if row[6] == "local QGHT" {
			positive[row[1]] = true
			if len(row[4]) != 17 || row[4][4:8] != "QGHT" {
				t.Fatal(row)
			}
		}
		if row[6] == "global ILMV" {
			global[row[1]] = true
		}
	}
	if len(positive) != 18 || len(global) != 11 || len(samples) != 36 {
		t.Fatal(len(positive), len(global), len(samples))
	}
	for _, n := range samples {
		if n != 30 {
			t.Fatal(n)
		}
	}
	got := digest(t, dir)
	if got != "46922cb9605d5aae1e59f70a9725c85f7001204b6fa60249eb56ec9ff2ce1110" {
		t.Fatalf("convergence study digest %s", got)
	}
	if _, err = Run(context.Background(), s, dir, "validation", nil); err == nil {
		t.Fatal("overwrote existing study")
	}
	s.Motifs = &MotifStudy{}
	if s.Check() == nil {
		t.Fatal("mixed models")
	}
}

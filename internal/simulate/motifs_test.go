package simulate

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMotifStudyTruth(t *testing.T) {
	s := Presets["motif-study"]
	dir := filepath.Join(t.TempDir(), "study")
	sum, err := Run(context.Background(), s, dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Subjects != 12 || sum.Samples != 12 || sum.Rows != 1120 {
		t.Fatal(sum)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "truth/receptors.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(string(raw)))
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	unique := map[string]bool{}
	local, global := 0, 0
	lengths := map[int]bool{}
	for _, row := range rows[1:] {
		count, err := strconv.Atoi(row[5])
		if err != nil || count < 1 {
			t.Fatal(row)
		}
		aa := row[4]
		lengths[len(aa)] = true
		unique[row[2]+"/"+row[3]+"/"+aa] = true
		switch row[6] {
		case "local QGHT":
			local++
			at := strings.Index(aa, "QGHT")
			if row[2] != "query" || row[3] != "TRBV5-1*01" || at < 4 || at > 6 {
				t.Fatal(row)
			}
		case "global conservative ILMV":
			global++
			if row[2] != "query" || row[3] != "TRBV7-9*01" || !strings.ContainsRune("ILMV", rune(aa[5])) || aa[:5]+aa[6:] != "CASSGTGQANEQYF" {
				t.Fatal(row)
			}
		case "none":
		default:
			t.Fatal(row)
		}
	}
	if local != 36 || global != 16 || len(unique) != 1108 || len(lengths) != 3 || !lengths[15] || !lengths[16] || !lengths[17] {
		t.Fatal(local, global, len(unique), lengths)
	}
	got := digest(t, dir)
	t.Logf("motif study digest %s", got)
	if got != "ed97231951ebebe6c4fa1b5e4879118e941ef6c32df8baeb9f70714913b484b8" {
		t.Fatal("study digest", got)
	}
	if _, err = Run(context.Background(), s, dir, "validation", nil); err == nil {
		t.Fatal("overwrote existing study")
	}
	s.Counts = &CountStudy{}
	if err = s.Check(); err == nil {
		t.Fatal("mixed simulation models")
	}
}

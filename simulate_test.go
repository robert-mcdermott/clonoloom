package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/simulate"
)

func TestTheSimulateCommand(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "study")
	var out, errs bytes.Buffer
	if code := runSimulate([]string{"--preset", "small", "--out", dir, "--quiet"}, &out, &errs); code != 0 || !strings.Contains(out.String(), "2 subjects, 2 samples") {
		t.Fatalf("%d %s %s", code, out.String(), errs.String())
	}
	if code := runSimulate([]string{"--preset", "small", "--out", dir}, &out, &errs); code != 1 {
		t.Errorf("into a folder that is not empty: %d", code)
	}
	if code := runSimulate([]string{"--preset", "nothing"}, &out, &errs); code != 2 {
		t.Errorf("an unknown preset: %d", code)
	}
	out.Reset()
	if code := runSimulate([]string{"--preset", "small", "--seed", "99", "--print-params"}, &out, &errs); code != 0 {
		t.Fatal(code)
	}
	var params struct {
		Name string `json:"name"`
		Seed uint64 `json:"seed"`
	}
	if err := json.Unmarshal(out.Bytes(), &params); err != nil || params.Name != "small" || params.Seed != 99 {
		t.Errorf("%v %+v", err, params)
	}
	// Opened as a folder, a study is its samples and their metadata: its truth is not data, nor is
	// a specificity database beside it (the vaccine study's).
	os.MkdirAll(filepath.Join(dir, "database"), 0o755)
	os.WriteFile(filepath.Join(dir, "database", "vdjdb.txt"), []byte("complex.id\tgene\tcdr3\n"), 0o644)
	items, _ := walkFolder(dir)
	var names []string
	for _, item := range items {
		names = append(names, item.Name)
	}
	if strings.Join(names, " ") != "repertoires.airr.json S01-1.tsv S02-1.tsv" {
		t.Errorf("the folder holds %v", names)
	}
	if _, err := os.Stat(filepath.Join(dir, "truth", "truth.json")); err != nil {
		t.Error(err)
	}
}

func TestBCellSimulateCommandAndFolder(t *testing.T) {
	var out, errs bytes.Buffer
	if code := runSimulate([]string{"--preset", "bcell-immunization", "--seed", "99", "--print-params"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var study simulate.Study
	if err := json.Unmarshal(out.Bytes(), &study); err != nil || study.BCells == nil || study.Seed != 99 {
		t.Fatal("parameters", err)
	}
	study.Name = "renamed"
	study.BCells.Subjects, study.BCells.Families, study.BCells.CellsPerSample = 1, 2, 8
	params, _ := json.Marshal(study)
	path := filepath.Join(t.TempDir(), "params.json")
	if err := os.WriteFile(path, params, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "study")
	out.Reset()
	if code := runSimulate([]string{"--params", path, "--out", dir, "--quiet"}, &out, &errs); code != 0 || !strings.Contains(out.String(), "1 subjects, 3 samples, 48 rearrangements") {
		t.Fatal(code, out.String(), errs.String())
	}
	items, err := walkFolder(dir)
	if err != "" || len(items) != 4 {
		t.Fatalf("folder: %v %v", items, err)
	}
	for _, item := range items {
		if strings.Contains(item.Name, "truth") || strings.Contains(item.Name, "observations") || strings.Contains(item.Name, "germlines") {
			t.Fatal("truth or reference treated as data")
		}
	}
}

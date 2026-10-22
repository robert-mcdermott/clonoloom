package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	// Opened as a folder, a study is its samples and their metadata: its truth is not data.
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

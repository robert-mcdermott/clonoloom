package simulate

import (
	"clonoloom/internal/batchqc"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBatchQCStudyTruth(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "study")
	s := Presets["batch-qc-study"]
	sum, e := Run(context.Background(), s, dir, "validation", nil)
	if e != nil {
		t.Fatal(e)
	}
	if sum.Subjects != 25 || sum.Samples != 26 || sum.Rows != 2427 {
		t.Fatal(sum)
	}
	var assignments []batchqc.Assignment
	raw, _ := os.ReadFile(filepath.Join(dir, "truth/samples.json"))
	if e = json.Unmarshal(raw, &assignments); e != nil {
		t.Fatal(e)
	}
	batches := map[string]int{}
	excluded := 0
	for _, a := range assignments {
		if a.Exclude != "" {
			excluded++
		} else {
			batches[a.Batch]++
		}
	}
	if excluded != 2 || batches["run-A"] != 8 || batches["run-B"] != 8 || batches["run-C"] != 3 || batches["run-D"] != 5 {
		t.Fatal(batches, excluded)
	}
	if got := digest(t, dir); got != "7523127cc2d43b8168ffb85b7064a57727511953eeea17fb7393dbe165d72782" {
		t.Fatalf("batch QC simulator digest %s; rows %d", got, sum.Rows)
	}
	if _, e = Run(context.Background(), s, dir, "validation", nil); e == nil {
		t.Fatal("overwrite")
	}
}

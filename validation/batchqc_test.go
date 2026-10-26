package validation

import (
	"clonoloom/internal/batchqc"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestBatchQCIndependentReference(t *testing.T) {
	raw, e := os.ReadFile("reference/batch_qc.json")
	if e != nil {
		t.Fatal(e)
	}
	var reference struct {
		Input  batchqc.Input   `json:"input"`
		Groups []batchqc.Group `json:"groups"`
	}
	if e = json.Unmarshal(raw, &reference); e != nil {
		t.Fatal(e)
	}
	got, e := batchqc.Compute(context.Background(), reference.Input)
	if e != nil {
		t.Fatal(e)
	}
	if got.Included != 14 || got.Excluded != 1 || got.Subjects != 6 || len(got.Groups) != len(reference.Groups) {
		t.Fatal("sample accounting", got)
	}
	equal := func(a, b float64) bool {
		return math.IsNaN(a) && math.IsNaN(b) || math.Abs(a-b) < 2e-12*math.Max(1, math.Abs(b))
	}
	for i, g := range got.Groups {
		want := reference.Groups[i]
		if g.Batch != want.Batch || g.Metric != want.Metric || g.Valid != want.Valid || g.Scored != want.Scored || g.Flagged != want.Flagged || g.Reason != want.Reason || !equal(float64(g.Median), float64(want.Median)) || !equal(float64(g.MAD), float64(want.MAD)) {
			t.Fatalf("group %d got %+v want %+v", i, g, want)
		}
		for j, p := range g.Observations {
			q := want.Observations[j]
			if p.Sample != q.Sample || p.Flag != q.Flag || p.Reason != q.Reason || !equal(float64(p.Value), float64(q.Value)) || !equal(float64(p.Transformed), float64(q.Transformed)) || !equal(float64(p.Z), float64(q.Z)) {
				t.Fatalf("point %d/%d: %+v / %+v", i, j, p, q)
			}
		}
	}
	b, _ := json.Marshal(got)
	digest := fmt.Sprintf("%x", sha256.Sum256(b))
	const want = "1eadd32521a74e5261d5deaddb49fa9ff672bcf30cec4750f281cbf7e8f0f753"
	if digest != want {
		t.Fatalf("batch QC result digest %s", digest)
	}
}

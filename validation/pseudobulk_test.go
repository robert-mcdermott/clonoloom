package validation

import (
	"clonoloom/internal/pseudobulk"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestPseudobulkIndependentReference(t *testing.T) {
	b, e := os.ReadFile("reference/pseudobulk.json")
	if e != nil {
		t.Fatal(e)
	}
	var ref struct {
		Input  pseudobulk.Input
		Result pseudobulk.Result
	}
	if e = json.Unmarshal(b, &ref); e != nil {
		t.Fatal(e)
	}
	got, e := pseudobulk.Compute(context.Background(), ref.Input)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(got)
	want, _ := json.Marshal(ref.Result)
	if string(raw) != string(want) {
		t.Fatalf("independent aggregation differs\ngot %s\nwant %s", raw, want)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if digest != "f7b51cf23e2e5b404d7331d4f5092c1e4fd4de91df756629f82045acee33a3db" {
		t.Fatal("pseudobulk digest", digest)
	}
}

package cache

import (
	"clonoloom/internal/reads"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIMGTCacheAndCompanionChange(t *testing.T) {
	dir := t.TempDir()
	entries, e := os.ReadDir("../../testdata/imgt-inputs")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		data, _ := os.ReadFile(filepath.Join("../../testdata/imgt-inputs", entry.Name()))
		os.WriteFile(filepath.Join(dir, entry.Name()), data, 0600)
	}
	c, e := Open(t.TempDir(), 0)
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	c.read = func(ctx context.Context, in []reads.Input) (*reads.Result, error) {
		calls++
		return reads.ReadInputs(ctx, in)
	}
	open := func() *reads.Result {
		t.Helper()
		r, e := reads.OpenInputsWith(context.Background(), []reads.Input{{Path: dir}}, c.Tables)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	first := open()
	second := open()
	if calls != 1 || first.Table.Len() != second.Table.Len() {
		t.Fatal("cache miss")
	}
	if first.Table.EvidenceRow(2) != second.Table.EvidenceRow(2) || first.Sources[4].AnnotationParameters != second.Sources[4].AnnotationParameters {
		t.Fatal("cache lost provenance")
	}
	p := filepath.Join(dir, "2_IMGT-gapped-nt-sequences.txt")
	data, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(data), "aaa...tgtgcttggccc", "aaa...tgtgcttggcca", 1)), 0600)
	third := open()
	if calls != 2 || third.Sources[1].SHA256 == first.Sources[1].SHA256 {
		t.Fatal("stale companion")
	}
	p = filepath.Join(dir, "11_Parameters.txt")
	os.WriteFile(p, []byte("changed parameters"), 0600)
	open()
	if calls != 3 {
		t.Fatal("parameters ignored by cache")
	}
}

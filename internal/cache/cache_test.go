package cache

import (
	"compress/gzip"
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

const airrTSV = "sequence_id\tv_call\tj_call\tjunction\tjunction_aa\tproductive\tumi_count\trepertoire_id\n" +
	"a\tTRBV5-1*01\tTRBJ2-7*01\tTGTGCCAGCAGCTTGGCAGGGGGCCCTGAGGCTTTCTTT\tCASSLAGGPEAFF\tT\t3\tR1\n" +
	"b\tTRBV6-2*01,TRBV6-3*01\tTRBJ1-1*01\t\tCASSPGQGAYEQYF\tF\t\tR1\n" +
	"c\tTRBV5-1*01\tTRBJ2-7*01\tTGTGCCAGCAGCTTGGCAGGGGGCCCTGAGGCTTTCTTT\tCASSLAGGPEAFF\tT\t5\tR2\n"

// counting reads tables as the readers do, counting how often.
func counting(c *Cache) *int {
	n := new(int)
	c.read = func(ctx context.Context, inputs []reads.Input) (*reads.Result, error) {
		*n++
		return reads.ReadInputs(ctx, inputs)
	}
	return n
}

func write(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if filepath.Ext(path) == ".gz" {
		f, _ := os.Create(path)
		z := gzip.NewWriter(f)
		z.Write([]byte(content))
		z.Close()
		f.Close()
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// same compares two reads of the same files: every column, every string, the sources (but where
// and when they lie), the problems and the unit.
func same(t *testing.T, a, b *reads.Result) {
	t.Helper()
	for i, s := range slots(a.Table) {
		other := slots(b.Table)[i]
		var x, y any
		switch {
		case s.u8 != nil:
			x, y = *s.u8, *other.u8
		case s.i8 != nil:
			x, y = *s.i8, *other.i8
		case s.u16 != nil:
			x, y = *s.u16, *other.u16
		case s.u32 != nil:
			x, y = *s.u32, *other.u32
		case s.f64 != nil:
			fx, fy := *s.f64, *other.f64
			if (fx == nil) != (fy == nil) || len(fx) != len(fy) {
				t.Fatalf("column %s: %d and %d values", s.name, len(fx), len(fy))
			}
			for k := range fx {
				if fx[k] != fy[k] && !(math.IsNaN(fx[k]) && math.IsNaN(fy[k])) {
					t.Fatalf("column %s row %d: %v and %v", s.name, k, fx[k], fy[k])
				}
			}
			continue
		}
		if !reflect.DeepEqual(x, y) {
			t.Fatalf("column %s differs", s.name)
		}
	}
	for i, st := range stores(a.Table) {
		sa, sb := *st.s, *stores(b.Table)[i].s
		if sa == nil || sb == nil {
			if sa != sb {
				t.Fatalf("%s: missing store differs", st.name)
			}
			continue
		}
		if sa.Len() != sb.Len() {
			t.Fatalf("%s: %d and %d strings", st.name, sa.Len(), sb.Len())
		}
		for code := uint32(1); code <= uint32(sa.Len()); code++ {
			if sa.Get(code) != sb.Get(code) {
				t.Fatalf("%s %d: %q and %q", st.name, code, sa.Get(code), sb.Get(code))
			}
		}
	}
	if !reflect.DeepEqual(a.Table.Samples, b.Table.Samples) || a.Table.Unit != b.Table.Unit || a.Unit != b.Unit {
		t.Fatalf("samples or unit differ: %+v %+v", a.Table.Samples, b.Table.Samples)
	}
	if !reflect.DeepEqual(a.Problems.Summaries(), b.Problems.Summaries()) {
		t.Fatalf("problems differ: %+v %+v", a.Problems.Summaries(), b.Problems.Summaries())
	}
	if len(a.Sources) != len(b.Sources) {
		t.Fatal("sources differ")
	}
	for i := range a.Sources {
		x, y := a.Sources[i], b.Sources[i]
		x.Modified, y.Modified, x.Path, y.Path = "", "", "", ""
		if !reflect.DeepEqual(x, y) {
			t.Fatalf("source %d: %+v and %+v", i, x, y)
		}
	}
}

func TestATableIsReadOnceAndAnsweredAgain(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "data", "study.tsv"), filepath.Join(dir, "data", "more.tsv.gz")
	write(t, a, airrTSV)
	write(t, b, airrTSV)
	c, err := Open(filepath.Join(dir, "cache"), 0)
	if err != nil {
		t.Fatal(err)
	}
	reads := counting(c)
	inputs := []readsInput{{Path: a}, {Path: b}}
	first, err := c.Tables(context.Background(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Tables(context.Background(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if *reads != 1 {
		t.Fatalf("read %d times", *reads)
	}
	same(t, first, second)
	if second.Table.Call(second.Table.V[1]).Key(2) == "" || second.Table.Junctions.Get(second.Table.Junction[0]) == "" {
		t.Error("the cached table's strings are not readable")
	}
	// Moved, same name: answered from the cache, at its new place.
	moved := filepath.Join(dir, "elsewhere", "study.tsv")
	write(t, moved, airrTSV)
	again, err := c.Tables(context.Background(), []readsInput{{Path: moved}, {Path: b}})
	if err != nil || *reads != 1 || again.Sources[0].Path != moved {
		t.Fatalf("moved: %v, %d reads, %s", err, *reads, again.Sources[0].Path)
	}
	// Another name names other samples: read again. Changed bytes: read again.
	renamed := filepath.Join(dir, "elsewhere", "other.tsv")
	write(t, renamed, airrTSV)
	if _, err := c.Tables(context.Background(), []readsInput{{Path: renamed}, {Path: b}}); err != nil || *reads != 2 {
		t.Fatalf("renamed: %v, %d reads", err, *reads)
	}
	write(t, a, airrTSV+"d\tTRBV5-1*01\tTRBJ2-7*01\t\tCASSF\tT\t1\tR2\n")
	changed, err := c.Tables(context.Background(), inputs)
	if err != nil || *reads != 3 || changed.Table.Len() != 7 {
		t.Fatalf("changed: %v, %d reads, %d rows", err, *reads, changed.Table.Len())
	}
	u, _ := c.Usage()
	if len(u.Items) != 3 || u.Size == 0 {
		t.Errorf("usage %+v", u)
	}
}

type readsInput = reads.Input

// A damaged entry is removed and the files read again; the cache is kept within its limit, least
// recently used first.
func TestDamagedEntriesAndTheLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	write(t, path, airrTSV)
	c, _ := Open(filepath.Join(dir, "cache"), 0)
	reads := counting(c)
	inputs := []readsInput{{Path: path}}
	c.Tables(context.Background(), inputs)
	u, _ := c.Usage()
	entry := u.Items[0].path
	data, _ := os.ReadFile(entry)
	data[len(data)-1] ^= 0xff
	os.WriteFile(entry, data, 0o644)
	res, err := c.Tables(context.Background(), inputs)
	if err != nil || *reads != 2 || res.Table.Len() != 3 {
		t.Fatalf("damaged: %v, %d reads", err, *reads)
	}
	if _, err := load(entry); err != nil {
		t.Errorf("the entry was not written again: %v", err)
	}
	// A limit of one entry's size keeps the most recent one.
	c.limit = u.Items[0].Size
	other := filepath.Join(dir, "other.tsv")
	write(t, other, airrTSV)
	c.Tables(context.Background(), []readsInput{{Path: other}})
	u, _ = c.Usage()
	if len(u.Items) != 1 || u.Size > c.limit {
		t.Errorf("after eviction: %+v", u)
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if u, _ := c.Usage(); len(u.Items) != 0 {
		t.Error("not cleared")
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"10GB": 10e9, "500mb": 500e6, "0": 0, "1.5G": 1.5e9, "2048": 2048} {
		if got, err := ParseSize(in); err != nil || got != want {
			t.Errorf("%s: %d %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "ten", "-1GB"} {
		if _, err := ParseSize(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// The development files, where fetched: each cached table is the one read (every format).
func TestDevelopmentFilesRoundTrip(t *testing.T) {
	root := os.Getenv("CLONOLOOM_DATA")
	if root == "" {
		root = "../../validation/cache"
	}
	var files [][]readsInput
	for _, pattern := range []string{"10x-pbmc-1k/*_t_filtered_contig_annotations.csv", "10x-pbmc-10k-v2/*_vdj_b_airr_rearrangement.tsv", "airr-standards-tests/lang/R/inst/extdata/rearrangement-example.tsv.gz", "emerson-2017-immunoseq/P00207.tsv"} {
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		if len(matches) > 0 {
			files = append(files, []readsInput{{Path: matches[0]}})
		}
	}
	if len(files) == 0 {
		t.Skip("no development data: node validation/fetch.mjs --ci")
	}
	c, _ := Open(t.TempDir(), 0)
	for _, inputs := range files {
		read, err := reads.ReadInputs(context.Background(), inputs)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Tables(context.Background(), inputs); err != nil {
			t.Fatal(err)
		}
		cached, err := c.Tables(context.Background(), inputs)
		if err != nil {
			t.Fatal(err)
		}
		same(t, read, cached)
		if cached.Table.Unit != rep.Unit(read.Unit) && cached.Unit != read.Unit {
			t.Errorf("%s: unit", inputs[0].Path)
		}
	}
}

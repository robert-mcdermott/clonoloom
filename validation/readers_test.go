// ClonoLoom's readers on the development files, against independent readers: AIRR files as the
// AIRR Community's own library (airr) reads them, Cell Ranger contig annotations and immunoSEQ
// exports as pandas reads them, every column ClonoLoom reads compared by its digest
// (validation/reference/readers.json, made by read_repertoires.py).
//
// The files are fetched by node validation/fetch.mjs into validation/cache/ (or $CLONOLOOM_DATA);
// a missing one is skipped, unless CLONOLOOM_REQUIRE_DATA names its data set (a comma-separated
// list, or 1 for all): CI requires the openly licensed data sets.
package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

type readerReference struct {
	GeneratedBy string `json:"generated_by"`
	Files       map[string]struct {
		Format    string                    `json:"format"`
		Rows      int                       `json:"rows"`
		Unit      string                    `json:"unit"`
		Columns   map[string]string         `json:"columns"`
		GeneNames map[string]map[string]int `json:"gene_names"`
	} `json:"files"`
}

func dataRoot() string {
	if root := os.Getenv("CLONOLOOM_DATA"); root != "" {
		return root
	}
	return "cache"
}

// required reports whether a data set must be present (CLONOLOOM_REQUIRE_DATA).
func required(dataset string) bool {
	want := os.Getenv("CLONOLOOM_REQUIRE_DATA")
	if want == "1" {
		return true
	}
	for _, name := range strings.Split(want, ",") {
		if strings.TrimSpace(name) == dataset {
			return true
		}
	}
	return false
}

const missing = "\x00"

func digest(values []string) string {
	h := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(h[:])
}

func numberText(v float64) string {
	if math.IsNaN(v) || v < 0 {
		return missing
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func flagText(f rep.Flag) string {
	switch f {
	case rep.True:
		return "T"
	case rep.False:
		return "F"
	}
	return missing
}

func orMissing(s string) string {
	if s == "" {
		return missing
	}
	return s
}

// columns are the table's values as read_repertoires.py writes them.
func columns(t *rep.Table) map[string][]string {
	n := t.Len()
	col := map[string][]string{}
	add := func(name string, f func(int) string) {
		values := make([]string, n)
		for i := range values {
			values[i] = f(i)
		}
		col[name] = values
	}
	dict := func(c []uint32) func(int) string {
		return func(i int) string {
			if c == nil {
				return missing
			}
			return orMissing(t.Dict.Get(c[i]))
		}
	}
	add("sequence_id", func(i int) string { return orMissing(t.IDs.Get(t.SequenceID[i])) })
	add("locus", func(i int) string { return orMissing(t.LocusName(i)) })
	add("v_call", dict(t.V))
	add("d_call", dict(t.D))
	add("j_call", dict(t.J))
	add("c_call", dict(t.C))
	add("junction", func(i int) string { return orMissing(t.Junctions.Get(t.Junction[i])) })
	add("junction_aa", func(i int) string { return orMissing(t.JunctionsAA.Get(t.JunctionAA[i])) })
	add("productive", func(i int) string { return flagText(t.Productive[i]) })
	add("cell", dict(t.Cell))
	add("clone", dict(t.Clone))
	add("count", func(i int) string { return numberText(t.Count[i]) })
	add("reads", func(i int) string {
		if t.Reads == nil {
			return missing
		}
		return numberText(t.Reads[i])
	})
	flags := func(f []rep.Flag) func(int) string {
		return func(i int) string {
			if f == nil {
				return missing
			}
			return flagText(f[i])
		}
	}
	add("is_cell", flags(t.IsCell))
	add("high_confidence", flags(t.HighConfidence))
	add("full_length", flags(t.FullLength))
	add("sample", func(i int) string { return t.Samples[t.Sample[i]].Name })
	return col
}

// recorded are the sizes and SHA-256 sources.json records for each file, by data set and path.
func recorded(t *testing.T) map[string]struct {
	Size   int64
	SHA256 string
} {
	raw, err := os.ReadFile("sources.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Datasets map[string]struct {
			Files []struct {
				Path   string `json:"path"`
				Size   int64  `json:"size"`
				SHA256 string `json:"sha256"`
			} `json:"files"`
		} `json:"datasets"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	out := map[string]struct {
		Size   int64
		SHA256 string
	}{}
	for name, set := range manifest.Datasets {
		for _, f := range set.Files {
			out[name+"/"+f.Path] = struct {
				Size   int64
				SHA256 string
			}{f.Size, f.SHA256}
		}
	}
	return out
}

func TestReadersAgreeWithIndependentReaders(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "readers.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref readerReference
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	fingerprints := recorded(t)
	checked := map[string]int{}
	for rel, want := range ref.Files {
		dataset := strings.Split(filepath.ToSlash(rel), "/")[0]
		path := filepath.Join(dataRoot(), filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			if required(dataset) {
				t.Errorf("%s is required and missing (node validation/fetch.mjs %s)", rel, dataset)
			}
			continue
		}
		t.Run(rel, func(t *testing.T) {
			res, err := reads.ReadFiles(context.Background(), []string{path})
			if err != nil {
				t.Fatal(err)
			}
			// The fingerprint taken while reading is the file's, as sources.json records it: the
			// reader read every byte, and the file is as it was fetched (nothing wrote to it).
			if f, ok := fingerprints[filepath.ToSlash(rel)]; ok && (res.Sources[0].SHA256 != f.SHA256 || res.Sources[0].Size != f.Size) {
				t.Errorf("fingerprint %s (%d bytes), sources.json records %s (%d bytes)", res.Sources[0].SHA256, res.Sources[0].Size, f.SHA256, f.Size)
			}
			if string(res.Sources[0].Format) != want.Format {
				t.Fatalf("read as %s, the reference as %s", res.Sources[0].Format, want.Format)
			}
			table := res.Table
			if table.Len() != want.Rows {
				t.Fatalf("%d rows, the independent reader %d", table.Len(), want.Rows)
			}
			if want.Unit != "" && string(res.Unit) != want.Unit {
				t.Errorf("unit %q, the reference %q", res.Unit, want.Unit)
			}
			got := columns(table)
			for name, sha := range want.Columns {
				values, ok := got[name]
				if !ok {
					t.Errorf("no column %s to compare", name)
					continue
				}
				if digest(values) != sha {
					t.Errorf("column %s differs from the independent reader's", name)
				}
			}
			// immunoSEQ gene names: each Adaptive name the reference counts maps, through the
			// dataset's record of names, to as many rows of the table.
			for segment, names := range want.GeneNames {
				column := map[string][]uint32{"v": table.V, "d": table.D, "j": table.J}[segment]
				have := map[string]int{}
				for _, code := range column {
					if code != 0 {
						have[table.Dict.Get(code)]++
					}
				}
				expect := map[string]int{}
				for name, n := range names {
					mapped, ok := res.Sources[0].GeneNames[name]
					if !ok {
						t.Errorf("%s gene %s is not in the dataset's record of names", segment, name)
						continue
					}
					expect[mapped] += n
				}
				for name, n := range expect {
					if have[name] != n {
						t.Errorf("%s gene %s: %d rows, the reference %d", segment, name, have[name], n)
					}
				}
			}
			checked[dataset]++
		})
	}
	t.Logf("compared with %s: %v", ref.GeneratedBy, checked)
}

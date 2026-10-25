package simulate

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Read literal TSV fields independently of the repertoire readers/count-model code.
func TestReplicatedCountTruth(t *testing.T) {
	for _, name := range []string{"counts-independent", "counts-paired", "counts-null"} {
		t.Run(name, func(t *testing.T) {
			s := Presets[name]
			dir := filepath.Join(t.TempDir(), "study")
			sum, err := Run(context.Background(), s, dir, "validation", nil)
			if err != nil {
				t.Fatal(err)
			}
			read := func(name string) [][]string {
				t.Helper()
				raw, e := os.ReadFile(filepath.Join(dir, name))
				if e != nil {
					t.Fatal(e)
				}
				r := csv.NewReader(strings.NewReader(string(raw)))
				r.Comma = '\t'
				rows, e := r.ReadAll()
				if e != nil {
					t.Fatal(e)
				}
				return rows
			}
			number := func(s string) float64 {
				t.Helper()
				v, e := strconv.ParseFloat(s, 64)
				if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatal(s, e)
				}
				return v
			}
			observed := map[string]map[string]float64{}
			for _, file := range sum.Files {
				if !strings.HasSuffix(file, ".tsv") || strings.Contains(file, "/") {
					continue
				}
				rows := read(file)
				col := map[string]int{}
				for i, v := range rows[0] {
					col[v] = i
				}
				if _, ok := col["umi_count"]; !ok {
					t.Fatal("UMI unit missing")
				}
				sample := strings.TrimSuffix(file, ".tsv")
				observed[sample] = map[string]float64{}
				for _, r := range rows[1:] {
					key := r[col["junction"]]
					if _, ok := observed[sample][key]; ok {
						t.Fatal("junction collision")
					}
					observed[sample][key] = number(r[col["umi_count"]])
				}
			}
			truths := read("truth/observations.tsv")
			if len(truths) != 1+sum.Samples*648 || len(observed) != sum.Samples {
				t.Fatal("missing truth/sample", len(truths), sum)
			}
			shared := map[string]bool{}
			private := map[string]string{}
			exposures := map[float64]bool{}
			positive := 0
			negative := 0
			for _, r := range truths[1:] {
				sample, subject, clone, junction := r[0], r[1], r[4], r[5]
				exposure, mean, alpha, effect, count := number(r[6]), number(r[7]), number(r[8]), number(r[9]), number(r[10])
				if exposure <= 0 || mean <= 0 || alpha <= 0 || count < 0 || count != math.Trunc(count) {
					t.Fatal(r)
				}
				exposures[exposure] = true
				if effect > 0 {
					positive++
				}
				if effect < 0 {
					negative++
				}
				if s.Counts.Null && effect != 0 {
					t.Fatal("nonnull generator")
				}
				if strings.HasPrefix(clone, "0:") {
					shared[junction] = true
				} else {
					if old, ok := private[junction]; ok && old != subject {
						t.Fatal("private receptor shared")
					}
					private[junction] = subject
				}
				got, ok := observed[sample][junction]
				if count != got || count > 0 && !ok {
					t.Fatal("count differs from independent truth", r, got)
				}
				delete(observed[sample], junction)
			}
			if len(shared) != 48 || len(private) != 600*s.Counts.Subjects || len(exposures) != sum.Samples {
				t.Fatal("lost heterogeneity/identities", len(shared), len(private), len(exposures))
			}
			if !s.Counts.Null && (positive != 4*sum.Samples || negative != 4*sum.Samples) {
				t.Fatal("effect panel changed", positive, negative)
			}
			for sample, rows := range observed {
				if len(rows) != 0 {
					t.Fatal("unrecorded source counts", sample)
				}
			}
			var manifest struct{ Files []bcellFile }
			raw, e := os.ReadFile(filepath.Join(dir, "truth/truth.json"))
			if e != nil || json.Unmarshal(raw, &manifest) != nil {
				t.Fatal(e)
			}
			hash := sha256.New()
			sort.Strings(sum.Files)
			for _, f := range sum.Files {
				b, e := os.ReadFile(filepath.Join(dir, f))
				if e != nil {
					t.Fatal(e)
				}
				fmt.Fprintln(hash, f)
				hash.Write(b)
			}
			for _, f := range manifest.Files {
				b, e := os.ReadFile(filepath.Join(dir, f.Path))
				if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != f.SHA256 {
					t.Fatal(f, e)
				}
			}
			want := map[string]string{"counts-independent": "9f32c8b288a383f7ba77d695a2a267f80e9998f38bc4ec9628a722ed599d4777", "counts-paired": "aa62df0fb1d16d419da36e7735611601d13b59cc77bd31de122f55096378eb4e", "counts-null": "16e8754335e1918b272bf9a42a9fe14d46107e242e57fb5471283e01d21c3afb"}
			if got := fmt.Sprintf("%x", hash.Sum(nil)); got != want[name] {
				t.Errorf("digest %s", got)
			}
			if _, e = Run(context.Background(), s, dir, "validation", nil); e == nil {
				t.Fatal("overwrote source study")
			}
		})
	}
}

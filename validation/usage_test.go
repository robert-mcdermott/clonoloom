// ClonoLoom's gene usage, junction lengths and amino-acid composition (internal/usage) on the
// development files' bulk samples in AIRR format and the simulated standard study, against an
// independent implementation in R (validation/reference/usage.R): every share, by clonotypes and by
// abundance, and every name, both ways.
package validation

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"clonoloom/internal/usage"
)

type usageReference struct {
	Clonotypes  int                              `json:"clonotypes"`
	Abundance   float64                          `json:"abundance"`
	Genes       map[string]map[string][2]float64 `json:"genes"`
	Lengths     map[string][2]float64            `json:"lengths"`
	MeanLength  [2]float64                       `json:"meanLength"`
	Composition map[string][2]float64            `json:"composition"`
}

func TestUsageAgreesWithR(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "usage.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string                               `json:"generated_by"`
		Files       map[string]map[string]usageReference `json:"files"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	var files []string
	for rel := range ref.Files {
		files = append(files, rel)
	}
	sort.Strings(files)
	compared, shares := 0, 0
	for _, rel := range files {
		path, ok := referencePath(t, rel)
		if !ok {
			if required(strings.Split(rel, "/")[0]) {
				t.Errorf("%s is required and missing", rel)
			}
			continue
		}
		res, err := reads.ReadFiles(t.Context(), []string{path})
		if err != nil {
			t.Fatal(err)
		}
		def, _ := clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}.Normalize()
		table, err := clonotype.Compute(res.Table, nil, def)
		if err != nil {
			t.Fatal(err)
		}
		got := usage.Measure(res.Table, table, nil, diversity.Options{})
		byName := map[string]usage.Sample{}
		for _, s := range got.Samples {
			byName[s.Name] = s
		}
		for name, want := range ref.Files[rel] {
			where := rel + " " + name
			s, ok := byName[name]
			if !ok || len(s.Loci) != 1 {
				t.Errorf("%s: %+v", where, s)
				continue
			}
			l := s.Loci[0]
			if s.Clonotypes != want.Clonotypes || l.Clonotypes != want.Clonotypes || math.Abs(l.Abundance-want.Abundance) > 1e-9 {
				t.Errorf("%s: %d clonotypes, %v; R %d, %v", where, l.Clonotypes, l.Abundance, want.Clonotypes, want.Abundance)
				continue
			}
			compare := func(what string, got []usage.Share, want map[string][2]float64) {
				seen := map[string]bool{}
				for _, sh := range got {
					name := sh.Name
					if name == "" {
						name = "(none)"
					}
					seen[name] = true
					w, ok := want[name]
					if !ok {
						t.Errorf("%s %s: %q is not R's", where, what, name)
						continue
					}
					for k, v := range [2]float64{float64(sh.Clonotypes), float64(sh.Abundance)} {
						if math.Abs(v-w[k]) > 1e-12*math.Max(1, math.Abs(w[k])) {
							t.Errorf("%s %s %q: %.15g, R %.15g", where, what, name, v, w[k])
						}
					}
					shares++
				}
				for name := range want {
					if !seen[name] {
						t.Errorf("%s %s: R's %q missing", where, what, name)
					}
				}
			}
			for _, g := range l.Genes {
				key := g.Segment + " " + g.Level
				if g.Segment == "VJ" && g.Level == "family" {
					continue // R gives the pairs at the gene level
				}
				compare(key, g.Shares, want.Genes[key])
			}
			compare("lengths", l.Lengths, want.Lengths)
			compare("composition", l.Composition, want.Composition)
			compare("mean length", []usage.Share{l.MeanLength}, map[string][2]float64{"mean": want.MeanLength})
			compared++
		}
	}
	if compared == 0 {
		t.Skip("no development data: node validation/fetch.mjs --ci")
	}
	t.Logf("%d samples, %d shares compared (%s)", compared, shares, ref.GeneratedBy)
}

// Gene usage, lengths and composition on the simulated standard study against its truth: each
// sample's expected shares, from its subject's population of clones (each with its genes, junction
// and cells) and the sample's draws of templates. By abundance, a gene's expected share is its
// clones' share of the productive cells; by clonotypes, of the productive clones each weighted by
// its chance to be seen, 1 − (1 − q)^T (q its share of all cells, T the templates drawn). The share
// of 95% intervals (a bootstrap of each sample's clonotypes) holding the expected share is reported
// and checked.
func TestUsageAgainstTheSimulatedTruth(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	dir := filepath.Join(t.TempDir(), "standard")
	if _, err := simulate.Run(t.Context(), simulate.Presets["standard"], dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	var truth struct {
		Samples []struct {
			ID        string `json:"id"`
			Subject   string `json:"subject"`
			Templates int    `json:"templates"`
		} `json:"samples"`
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatal(err)
	}
	opt := diversity.DefaultOptions()
	if raceDetector {
		opt.Replicates = 50
	}
	type tally struct{ held, n int }
	held := map[string]*tally{}
	var worst float64
	for k, sample := range truth.Samples {
		clones := populationClones(t, filepath.Join(dir, "truth", sample.Subject+".tsv.gz"))
		var cells float64
		for _, c := range clones {
			cells += c.cells
		}
		// The expected shares: by feature, the sums of cells (abundance) and of chances to be seen
		// (clonotypes), over the productive clones, and their denominators.
		expected := map[string][2]float64{}
		addExp := func(key string, cl, ab float64) {
			e := expected[key]
			expected[key] = [2]float64{e[0] + cl, e[1] + ab}
		}
		for _, c := range clones {
			if !c.productive {
				continue
			}
			seen := -math.Expm1(float64(sample.Templates) * math.Log1p(-c.cells/cells))
			n := float64(len(c.junctionAA))
			addExp("n", seen, c.cells)
			addExp("r", seen*n, c.cells*n)
			for seg, gene := range map[string]string{"V": c.v, "D": c.d, "J": c.j} {
				addExp("gene "+seg+" "+gene, seen, c.cells)
			}
			addExp("length "+strconv.Itoa(len(c.junctionAA)), seen, c.cells)
			for i := range c.junctionAA {
				addExp("aa "+c.junctionAA[i:i+1], seen, c.cells)
			}
		}
		res, err := reads.ReadFiles(t.Context(), []string{filepath.Join(dir, sample.ID+".tsv")})
		if err != nil {
			t.Fatal(err)
		}
		def, _ := clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}.Normalize()
		table, err := clonotype.Compute(res.Table, nil, def)
		if err != nil {
			t.Fatal(err)
		}
		o := opt
		o.Stream = uint64(k) << 8
		l := usage.Measure(res.Table, table, nil, o).Samples[0].Loci[0]
		check := func(kind, key string, s usage.Share, den string) {
			e, d := expected[key], expected[den]
			for w, v := range []struct {
				est usage.Number
				iv  *usage.Pair
			}{{s.Clonotypes, s.ClonotypesInterval}, {s.Abundance, s.AbundanceInterval}} {
				want := e[w] / d[w]
				name := kind + []string{" by clonotypes", " by abundance"}[w]
				if held[name] == nil {
					held[name] = &tally{}
				}
				held[name].n++
				if v.iv != nil && float64(v.iv[0]) <= want && want <= float64(v.iv[1]) {
					held[name].held++
				}
				worst = max(worst, math.Abs(float64(v.est)-want))
			}
		}
		for _, g := range l.Genes {
			if g.Level != "gene" || g.Segment == "VJ" || g.Segment == "C" {
				continue
			}
			for _, s := range g.Shares {
				check(g.Segment+" genes", "gene "+g.Segment+" "+s.Name, s, "n")
			}
		}
		for _, s := range l.Lengths {
			check("lengths", "length "+s.Name, s, "n")
		}
		for _, s := range l.Composition {
			check("amino acids", "aa "+s.Name, s, "r")
		}
	}
	var report strings.Builder
	var names []string
	for name := range held {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h := held[name]
		share := float64(h.held) / float64(h.n)
		fmt.Fprintf(&report, "  %-28s %5.1f%% of %d intervals held the truth\n", name, 100*share, h.n)
		if share < 0.85 {
			t.Errorf("%s: %.1f%% of intervals held the truth", name, 100*share)
		}
	}
	t.Logf("12 samples; the largest difference from an expected share %.4f\n%s", worst, report.String())
}

type populationClone struct {
	productive          bool
	v, d, j, junctionAA string
	cells               float64
}

// populationClones reads a subject's population of clones (internal/simulate's truth).
func populationClones(t *testing.T, path string) []populationClone {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(z)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var col map[string]int
	var out []populationClone
	for sc.Scan() {
		fields := strings.Split(sc.Text(), "\t")
		if col == nil {
			col = map[string]int{}
			for i, name := range fields {
				col[name] = i
			}
			continue
		}
		cells, _ := strconv.ParseFloat(fields[col["cells"]], 64)
		out = append(out, populationClone{productive: fields[col["productive"]] == "T", v: fields[col["v_call"]], d: fields[col["d_call"]], j: fields[col["j_call"]],
			junctionAA: fields[col["junction_aa"]], cells: cells})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// ClonoLoom's overlap between samples (internal/overlap) on two cohorts of the development data,
// against independent implementations (validation/reference/overlap.R): immunarch's own functions
// (Jaccard, Sørensen–Dice, the overlap coefficient, Morisita–Horn), vegan's vegdist (Morisita–Horn
// and Morisita's index) and R itself (shared clonotypes, the rarefied measures, the shares of
// abundance shared, the clonotypes by how many subjects hold them).
package validation

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/overlap"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
)

type overlapReference struct {
	Samples   []string         `json:"samples"`
	Depth     float64          `json:"depth"`
	Pairs     []map[string]any `json:"pairs"`
	Incidence []int            `json:"incidence"`
}

// cohortOf opens a cohort's files as one dataset and groups its clonotypes by the default bulk
// definition, answering the table and the samples' names.
func cohortOf(t *testing.T, dataset string) (*clonotype.Table, []string, bool) {
	t.Helper()
	var paths []string
	for _, pattern := range []string{"*.tsv"} {
		found, _ := filepath.Glob(filepath.Join(dataRoot(), dataset, pattern))
		paths = append(paths, found...)
	}
	if dataset == "simulated-standard" && len(paths) == 0 {
		if p, ok := referencePath(t, "simulated-standard/S01-1.tsv"); ok {
			paths, _ = filepath.Glob(filepath.Join(filepath.Dir(p), "*.tsv"))
		}
	}
	if len(paths) == 0 {
		return nil, nil, false
	}
	sort.Strings(paths)
	res, err := reads.ReadFiles(t.Context(), paths)
	if err != nil {
		t.Fatal(err)
	}
	def, _ := clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}.Normalize()
	table, err := clonotype.Compute(res.Table, nil, def)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range res.Table.Samples {
		names = append(names, s.Name)
	}
	return table, names, true
}

func TestOverlapAgreesWithImmunarchAndVegan(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "overlap.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string                      `json:"generated_by"`
		Cohorts     map[string]overlapReference `json:"cohorts"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	simulatedSubject := regexp.MustCompile(`-[0-9]+$`)
	compared := 0
	for name, want := range ref.Cohorts {
		table, names, ok := cohortOf(t, name)
		if !ok {
			if required(name) {
				t.Errorf("%s is required and missing", name)
			}
			continue
		}
		subjects := map[string]string{}
		for _, n := range names {
			if name == "simulated-standard" {
				subjects[n] = simulatedSubject.ReplaceAllString(n, "")
			} else {
				subjects[n] = n
			}
		}
		got := overlap.Measure(table, names, subjects, nil, diversity.Options{})
		if got.Depth != want.Depth || strings.Join(names, ",") != strings.Join(want.Samples, ",") {
			t.Errorf("%s: depth %v, samples %v; R %v, %v", name, got.Depth, names, want.Depth, want.Samples)
			continue
		}
		index := map[string]int{}
		for i, n := range names {
			index[n] = i
		}
		byPair := map[[2]int]overlap.Pair{}
		for _, p := range got.Pairs {
			byPair[[2]int{p.A, p.B}] = p
		}
		for _, w := range want.Pairs {
			a, b := index[w["a"].(string)], index[w["b"].(string)]
			p := byPair[[2]int{a, b}]
			where := name + " " + w["a"].(string) + "–" + w["b"].(string)
			for id, refKey := range map[string]string{
				overlap.Shared: "shared", overlap.Jaccard: "jaccard", overlap.Sorensen: "sorensen", overlap.Coefficient: "overlap",
				overlap.Shared + overlap.Rarefied: "sharedRarefied", overlap.Jaccard + overlap.Rarefied: "jaccardRarefied",
				overlap.Sorensen + overlap.Rarefied: "sorensenRarefied", overlap.Coefficient + overlap.Rarefied: "overlapRarefied",
				overlap.MorisitaHorn: "morisitaHorn", "morisitaHornVegan": "morisitaHornVegan", overlap.Morisita: "morisitaVegan",
				overlap.AbundanceOfA: "abundanceOfA", overlap.AbundanceOfB: "abundanceOfB",
			} {
				measure := id
				if id == "morisitaHornVegan" {
					measure = overlap.MorisitaHorn
				}
				v, wv := float64(p.Measures[measure].Estimate), w[refKey].(float64)
				// vegan sets Morisita's dissimilarity to zero where the index exceeds one, as it can
				// by chance for two samples of one repertoire; ClonoLoom gives the index as it is.
				if refKey == "morisitaVegan" {
					v = min(v, 1)
				}
				// The rarefied measures to 10⁻⁸: R's lchoose of numbers near 10⁵ carries about 10⁻¹¹.
				tol := 1e-9
				if strings.HasSuffix(id, overlap.Rarefied) {
					tol = 1e-8
				}
				if math.Abs(v-wv) > tol*math.Max(1, math.Abs(wv)) {
					t.Errorf("%s %s: %.12g, the reference %.12g", where, refKey, v, wv)
				}
			}
			compared++
		}
		for k, n := range want.Incidence {
			if k >= len(got.Public.Incidence) || got.Public.Incidence[k].Clonotypes != n {
				t.Errorf("%s: clonotypes in %d subjects %+v, R %d", name, k+1, got.Public.Incidence, n)
			}
		}
	}
	if compared == 0 {
		t.Skip("no development data: node validation/fetch.mjs --ci")
	}
	t.Logf("%d pairs compared (%s)", compared, ref.GeneratedBy)
}

// Overlap on simulated studies against their truth. For each pair of samples, from their
// subjects' populations of clones: the clonotypes the two samples are expected to share,
// Σ P₁(seen)·P₂(seen) over the clonotypes both populations hold (P(seen) = 1 − (1 − q)^T, q the
// clonotype's share of its population's cells, T the templates drawn that gave reads); the same at
// the common depth m (each clonotype's share of the productive cells); and the populations' own
// Morisita–Horn index, 2Σpr/(Σp² + Σr²), which is 1 for two samples of one subject: the overlap
// both of Morisita's indices estimate. Reported: how often each 95% interval held the truth, and the
// mean error. One study of the standard preset (66 pairs) with the tests; CLONOLOOM_COVERAGE=1 runs
// five, each its own seed (330 pairs), whose results clonoloom-spec/research.md §5.6 reports.
func TestOverlapAgainstTheSimulatedTruth(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	studies := 1
	if os.Getenv("CLONOLOOM_COVERAGE") == "1" {
		studies = 5
	}
	type tally struct {
		held, n    int
		err        float64
		same, diff [2]int // held, of n: pairs of one subject, of two
	}
	tallies := map[string]*tally{}
	pairs := 0
	for k := range studies {
		study := simulate.Presets["standard"]
		study.Seed += uint64(k)
		dir := filepath.Join(t.TempDir(), "standard")
		if _, err := simulate.Run(t.Context(), study, dir, "validation", nil); err != nil {
			t.Fatal(err)
		}
		got, truths, same := simulatedOverlap(t, dir)
		for i, p := range got.Pairs {
			pairs++
			for id, want := range truths[i] {
				v := p.Measures[id]
				tl := tallies[id]
				if tl == nil {
					tl = &tally{}
					tallies[id] = tl
				}
				held := 0
				if float64(v.Lower) <= want && want <= float64(v.Upper) {
					held = 1
				}
				tl.held += held
				tl.n++
				tl.err += (float64(v.Estimate) - want) / math.Max(want, 1e-9)
				if same[i] {
					tl.same[0] += held
					tl.same[1]++
				} else {
					tl.diff[0] += held
					tl.diff[1]++
				}
			}
		}
	}
	var report strings.Builder
	for _, id := range []string{overlap.Shared, overlap.Shared + overlap.Rarefied, overlap.MorisitaHorn, overlap.Morisita} {
		tl := tallies[id]
		fmt.Fprintf(&report, "  %-15s held in %5.1f%% of %d pairs (one subject %d of %d, two subjects %d of %d), mean error %+.4f\n",
			id, 100*float64(tl.held)/float64(tl.n), tl.n, tl.same[0], tl.same[1], tl.diff[0], tl.diff[1], tl.err/float64(tl.n))
	}
	t.Logf("%d simulated studies, %d pairs\n%s", studies, pairs, report.String())
	// Morisita–Horn's plug-in Σx²/X² is biased high in a sample, so its index is biased low: reported
	// only.
	for _, id := range []string{overlap.Shared, overlap.Shared + overlap.Rarefied, overlap.Morisita} {
		if share := float64(tallies[id].held) / float64(tallies[id].n); share < 0.85 {
			t.Errorf("%s: %.1f%% of intervals held the truth", id, 100*share)
		}
	}
}

// simulatedOverlap measures a simulated study's overlap and answers, for each pair, the truth of
// each measure checked and whether its two samples are one subject's.
func simulatedOverlap(t *testing.T, dir string) (overlap.Result, []map[string]float64, []bool) {
	t.Helper()
	var truth struct {
		Samples []struct {
			ID            string `json:"id"`
			Subject       string `json:"subject"`
			TemplatesSeen int    `json:"templatesSeen"`
		} `json:"samples"`
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatal(err)
	}
	// Each subject's clonotypes: their cells, as shares of all cells and of the productive cells.
	type population struct{ all, productive map[string]float64 }
	populations := map[string]population{}
	subjects := map[string]string{}
	templates := map[string]float64{}
	var paths []string
	for _, s := range truth.Samples {
		subjects[s.ID] = s.Subject
		templates[s.ID] = float64(s.TemplatesSeen)
		paths = append(paths, filepath.Join(dir, s.ID+".tsv"))
		if _, ok := populations[s.Subject]; ok {
			continue
		}
		clones := populationClones(t, filepath.Join(dir, "truth", s.Subject+".tsv.gz"))
		p := population{all: map[string]float64{}, productive: map[string]float64{}}
		var all, productive float64
		for _, c := range clones {
			all += c.cells
			if c.productive {
				productive += c.cells
			}
		}
		for _, c := range clones {
			if c.productive {
				key := c.junctionAA + "\x1f" + c.v + "\x1f" + c.j
				p.all[key] += c.cells / all
				p.productive[key] += c.cells / productive
			}
		}
		populations[s.Subject] = p
	}
	res, err := reads.ReadFiles(t.Context(), paths)
	if err != nil {
		t.Fatal(err)
	}
	def, _ := clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}.Normalize()
	table, err := clonotype.Compute(res.Table, nil, def)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range res.Table.Samples {
		names = append(names, s.Name)
	}
	opt := diversity.DefaultOptions()
	if raceDetector {
		opt.Replicates = 50
	}
	got := overlap.Measure(table, names, subjects, nil, opt)
	seen := func(q, n float64) float64 { return -math.Expm1(n * math.Log1p(-q)) }
	var truths []map[string]float64
	var same []bool
	for _, p := range got.Pairs {
		a, b := names[p.A], names[p.B]
		pa, pb := populations[subjects[a]], populations[subjects[b]]
		var shared, atDepth, pr, pp, rr float64
		for key, qa := range pa.all {
			if qb, ok := pb.all[key]; ok {
				shared += seen(qa, templates[a]) * seen(qb, templates[b])
				atDepth += seen(pa.productive[key], got.Depth) * seen(pb.productive[key], got.Depth)
				pr += pa.productive[key] * pb.productive[key]
			}
		}
		for _, v := range pa.productive {
			pp += v * v
		}
		for _, v := range pb.productive {
			rr += v * v
		}
		mh := 2 * pr / (pp + rr)
		if subjects[a] == subjects[b] {
			mh = 1 // one repertoire: exactly, whatever the order of the sums' rounding
		}
		truths = append(truths, map[string]float64{overlap.Shared: shared, overlap.Shared + overlap.Rarefied: atDepth, overlap.MorisitaHorn: mh, overlap.Morisita: mh})
		same = append(same, subjects[a] == subjects[b])
	}
	return got, truths, same
}

// Overlap between four single-cell T-cell runs (two human, two mouse), against scirpy's
// repertoire_overlap (validation/reference/scirpy_overlap.py): with clonotypes by amino-acid
// junction identity of both chains, and of the beta chain alone, every pair's shared clonotypes and
// Jaccard index, and each sample's clonotypes.
func TestOverlapAgreesWithScirpy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "overlap_sc.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string `json:"generated_by"`
		Definitions map[string][]struct {
			A, B       string
			Jaccard    float64 `json:"jaccard"`
			Shared     int     `json:"shared"`
			Clonotypes [2]int  `json:"clonotypes"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range ref.Definitions["paired"] {
		for _, rel := range []string{p.A, p.B} {
			if !slices.Contains(paths, rel) {
				paths = append(paths, rel)
			}
		}
	}
	sort.Strings(paths)
	var full []string
	for _, rel := range paths {
		path := filepath.Join(dataRoot(), filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			if required(strings.Split(rel, "/")[0]) {
				t.Errorf("%s is required and missing", rel)
			}
			t.Skip("no development data: node validation/fetch.mjs --ci")
		}
		full = append(full, path)
	}
	res, err := reads.ReadFiles(t.Context(), full)
	if err != nil {
		t.Fatal(err)
	}
	// Each sample by the file it came from.
	index := map[string]int{}
	var names []string
	for i, s := range res.Table.Samples {
		index[paths[s.Source]] = i
		names = append(names, s.Name)
	}
	compared := 0
	for name, chains := range map[string][]string{"paired": {"TRA", "TRB"}, "beta": {"TRB"}} {
		table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Chains: chains, Junction: "aa", V: "none", J: "none", Cells: "primary", ProductiveOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		got := overlap.Measure(table, names, nil, nil, diversity.Options{})
		for _, w := range ref.Definitions[name] {
			a, b := index[w.A], index[w.B]
			if a > b {
				a, b = b, a
			}
			var p overlap.Pair
			for _, x := range got.Pairs {
				if x.A == a && x.B == b {
					p = x
				}
			}
			where := name + " " + strings.Split(w.A, "/")[0] + "–" + strings.Split(w.B, "/")[0]
			if s := float64(p.Measures[overlap.Shared].Estimate); s != float64(w.Shared) || math.Abs(float64(p.Measures[overlap.Jaccard].Estimate)-w.Jaccard) > 1e-12 {
				t.Errorf("%s: %v shared, Jaccard %v; scirpy %d, %v", where, s, float64(p.Measures[overlap.Jaccard].Estimate), w.Shared, w.Jaccard)
			}
			if got.Samples[index[w.A]].Clonotypes != w.Clonotypes[0] || got.Samples[index[w.B]].Clonotypes != w.Clonotypes[1] {
				t.Errorf("%s: clonotypes %d and %d; scirpy %v", where, got.Samples[index[w.A]].Clonotypes, got.Samples[index[w.B]].Clonotypes, w.Clonotypes)
			}
			compared++
		}
	}
	t.Logf("%d pairs compared with %s", compared, ref.GeneratedBy)
}

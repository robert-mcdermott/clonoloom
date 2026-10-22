// Generation probabilities (internal/pgen) against OLGA and against the truth of ClonoLoom's own
// model:
//   - under OLGA's seven default models, every CDR3 of validation/reference/pgen.tsv.gz (real ones,
//     ones OLGA drew from each model, nucleotide CDR3s, CDR3s with V and J masks and with ambiguous
//     residues) within a relative 10⁻⁹ of OLGA 1.3.0's probability (made by olga_pgen.py); where
//     OLGA leaves out the longest insertions at one codon position (mouse TRA), against OLGA on a
//     model that lets it count them, and above OLGA's value on the model as shipped;
//   - ClonoLoom's own model (the simulator's) against the simulator itself: each of 200 junctions,
//     chosen from a pilot run, as frequent among 20 million rearrangements as its probability says;
//   - ClonoLoom's generator against the probabilities: under its own model, and under OLGA's human
//     TRB model (frequencies relative to the junctions compared, its unused genes left out);
//   - the budget: a million CDR3s' probabilities within 2 minutes on 10 cores.
package validation

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/pgen"
	"clonoloom/internal/random"
	"clonoloom/internal/simulate"
)

// olgaModel reads one of OLGA's default models as fetch.mjs fetches them (the reference data
// olga-models).
func olgaModel(t *testing.T, name string) *pgen.Model {
	t.Helper()
	dir := filepath.Join(dataRoot(), "olga-models", name)
	var files []*os.File
	for _, f := range []string{"model_params.txt", "model_marginals.txt", "V_gene_CDR3_anchors.csv", "J_gene_CDR3_anchors.csv"} {
		h, err := os.Open(filepath.Join(dir, f))
		if err != nil {
			if required("olga-models") {
				t.Fatalf("olga-models is required and missing (node validation/fetch.mjs olga-models)")
			}
			t.Skip("olga-models not fetched")
		}
		defer h.Close()
		files = append(files, h)
	}
	m, err := pgen.ReadIGoR(name, files[0], files[1], files[2], files[3])
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPgenEqualsOLGA(t *testing.T) {
	var ref struct {
		Made   string            `json:"made"`
		SHA256 string            `json:"sha256"`
		Rows   int               `json:"rows"`
		Models map[string]string `json:"models"`
	}
	raw, err := os.ReadFile(filepath.Join("reference", "pgen.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	packed, err := os.ReadFile(filepath.Join("reference", "pgen.tsv.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(packed); hex.EncodeToString(sum[:]) != ref.SHA256 {
		t.Fatal("pgen.tsv.gz is not the file pgen.json describes")
	}
	// The models fetched are the ones OLGA used.
	for file, sum := range ref.Models {
		data, err := os.ReadFile(filepath.Join(dataRoot(), "olga-models", filepath.FromSlash(file)))
		if err != nil {
			continue // olgaModel skips or fails below
		}
		if got := sha256.Sum256(data); hex.EncodeToString(got[:]) != sum {
			t.Fatalf("%s differs from the file OLGA %s used", file, ref.Made)
		}
	}
	z, err := gzip.NewReader(strings.NewReader(string(packed)))
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		model, kind, cdr3, v, j string
		// want: OLGA's probability (on the padded model where olga_pgen.py pads it); shipped: on
		// the model as shipped.
		want, shipped float64
	}
	var rows []row
	sc := bufio.NewScanner(z)
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		if len(f) != 7 {
			t.Fatalf("a row of %d fields", len(f))
		}
		p, err := strconv.ParseFloat(f[5], 64)
		if err != nil {
			t.Fatal(err)
		}
		shipped := p
		if f[6] != "" {
			if shipped, err = strconv.ParseFloat(f[6], 64); err != nil {
				t.Fatal(err)
			}
		}
		rows = append(rows, row{f[0], f[1], f[2], f[3], f[4], p, shipped})
	}
	if len(rows) != ref.Rows {
		t.Fatalf("%d rows, pgen.json %d", len(rows), ref.Rows)
	}
	byModel := map[string][]row{}
	for _, r := range rows {
		byModel[r.model] = append(byModel[r.model], r)
	}
	names := make([]string, 0, len(byModel))
	for name := range byModel {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			m := olgaModel(t, name)
			list := byModel[name]
			queries := make([]pgen.Query, len(list))
			for i, r := range list {
				q := pgen.Query{CDR3: r.cdr3, NT: r.kind == "nt"}
				if r.v != "" {
					if q.V, err = m.Mask('V', r.v); err != nil {
						t.Fatal(err)
					}
				}
				if r.j != "" {
					if q.J, err = m.Mask('J', r.j); err != nil {
						t.Fatal(err)
					}
				}
				queries[i] = q
			}
			start := time.Now()
			got, err := m.All(context.Background(), queries, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			worst, worstAt, bad := 0.0, 0, 0
			// OLGA on the model as shipped: only ever less, by the events it leaves out.
			dropped, most, below := 0, 0.0, 0
			for i, r := range list {
				if r.shipped != r.want {
					dropped++
					most = math.Max(most, (got[i]-r.shipped)/got[i])
					if got[i] < r.shipped*(1-1e-9) {
						below++
					}
				}
				var rel float64
				switch {
				case r.want == 0:
					rel = got[i]
				default:
					rel = math.Abs(got[i]-r.want) / r.want
				}
				if math.IsNaN(got[i]) || rel > 1e-9 {
					bad++
				}
				if rel > worst || math.IsNaN(rel) {
					worst, worstAt = rel, i
				}
			}
			if bad > 0 {
				r := list[worstAt]
				t.Errorf("%d of %d differ from OLGA's by more than a relative 10⁻⁹; the most %s %s (V %q, J %q): %.15e, OLGA %.15e", bad, len(list), r.kind, r.cdr3, r.v, r.j, got[worstAt], r.want)
			}
			if below > 0 {
				t.Errorf("%d of %d below OLGA's on the model as shipped", below, len(list))
			}
			if dropped > 0 {
				t.Logf("%s: OLGA on the model as shipped leaves out the longest insertions at one codon position: %d CDR3s, up to %.0f%% of a probability", name, dropped, 100*most)
			}
			t.Logf("%s: %d CDR3s within a relative %.1e of %s, in %v", name, len(list), worst, ref.Made, time.Since(start).Round(time.Millisecond))
		})
	}
}

// zScores compares counts with expectations: each (count − expected)/√expected.
func zScores(counts []int, expected []float64) []float64 {
	out := make([]float64, len(counts))
	for i := range counts {
		out[i] = (float64(counts[i]) - expected[i]) / math.Sqrt(expected[i])
	}
	return out
}

// checkZ: z-scores as a standard normal's: none beyond 4.5, their mean within 0.25 of 0, their
// mean square between 0.7 and 1.35 (3.5 standard errors, for 200).
func checkZ(t *testing.T, what string, z []float64) {
	t.Helper()
	worst, sum, squares := 0.0, 0.0, 0.0
	for _, x := range z {
		worst = math.Max(worst, math.Abs(x))
		sum += x
		squares += float64(x * x)
	}
	mean, ms := sum/float64(len(z)), squares/float64(len(z))
	t.Logf("%s: %d junctions, largest |z| %.2f, mean z %.3f, mean z² %.3f", what, len(z), worst, mean, ms)
	if worst > 4.5 || math.Abs(mean) > 0.25 || ms < 0.7 || ms > 1.35 {
		t.Errorf("%s: the frequencies are not those the probabilities give", what)
	}
}

// targets are the n junctions most frequent among draws of a pilot run, with their counts.
func targets(counts map[string]int, n int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if counts[keys[a]] != counts[keys[b]] {
			return counts[keys[a]] > counts[keys[b]]
		}
		return keys[a] < keys[b]
	})
	return keys[:min(n, len(keys))]
}

func TestPgenOfTheSimulatorsModel(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("measured without -short and without the race detector")
	}
	m := pgen.Simulator()
	locus := simulate.HumanTRB()
	g := simulate.NewGenerator(&locus, simulate.DefaultRecombination())
	draw := func(seed uint64, n int, keep map[string]bool) map[string]int {
		r := random.New(seed, 0)
		counts := map[string]int{}
		for range n {
			x := g.Rearrange(r)
			if x.InFrame && (keep == nil || keep[x.JunctionAA]) {
				counts[x.JunctionAA]++
			}
		}
		return counts
	}
	chosen := targets(draw(1, 1_000_000, nil), 200)
	keep := map[string]bool{}
	for _, s := range chosen {
		keep[s] = true
	}
	const n = 20_000_000
	counts := draw(2, n, keep)
	observed, expected := make([]int, len(chosen)), make([]float64, len(chosen))
	for i, s := range chosen {
		p, err := m.AA(s, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		observed[i], expected[i] = counts[s], float64(p*n)
	}
	checkZ(t, "the simulator's 20 million rearrangements against ClonoLoom's model", zScores(observed, expected))

	// ClonoLoom's generator under the same model: the same frequencies.
	gen := pgen.NewGenerator(m)
	r := random.New(3, 0)
	const ng = 5_000_000
	got := map[string]int{}
	for range ng {
		if d := gen.Draw(r); d.AA != "" && keep[d.AA] {
			got[d.AA]++
		}
	}
	for i, s := range chosen {
		observed[i], expected[i] = got[s], float64(expected[i]*ng/n)
	}
	checkZ(t, "ClonoLoom's generator's 5 million receptors against its model", zScores(observed, expected))
}

// Under OLGA's human TRB model, whose unused genes the generator leaves out: each junction's share
// of the draws of the junctions compared, against its share of their probabilities.
func TestGeneratorAgreesWithPgen(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("measured without -short and without the race detector")
	}
	m := olgaModel(t, "human_T_beta")
	gen := pgen.NewGenerator(m)
	pilot := map[string]int{}
	r := random.New(4, 0)
	for range 300_000 {
		if d := gen.Draw(r); d.Productive {
			pilot[d.AA]++
		}
	}
	chosen := targets(pilot, 200)
	keep := map[string]bool{}
	for _, s := range chosen {
		keep[s] = true
	}
	counts := map[string]int{}
	total := 0
	r = random.New(5, 0)
	for range 5_000_000 {
		if d := gen.Draw(r); keep[d.AA] {
			counts[d.AA]++
			total++
		}
	}
	probs := make([]float64, len(chosen))
	sum := 0.0
	for i, s := range chosen {
		p, err := m.AA(s, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		probs[i] = p
		sum += p
	}
	observed, expected := make([]int, len(chosen)), make([]float64, len(chosen))
	for i, s := range chosen {
		observed[i], expected[i] = counts[s], float64(float64(total)*probs[i])/sum
	}
	checkZ(t, "ClonoLoom's generator under OLGA's human TRB model", zScores(observed, expected))
}

// Budget (clonoloom-spec/design.md, "Budgets"): a million CDR3s' generation probabilities within 2
// minutes on 10 cores (scaled to the processors here), each drawn from OLGA's human TRB model.
func TestPgenWithinBudget(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("budgets are measured without -short and without the race detector")
	}
	m := olgaModel(t, "human_T_beta")
	gen := pgen.NewGenerator(m)
	r := random.New(6, 0)
	const n = 1_000_000
	queries := make([]pgen.Query, 0, n)
	for len(queries) < n {
		if d := gen.Draw(r); d.Productive {
			queries = append(queries, pgen.Query{CDR3: d.AA})
		}
	}
	start := time.Now()
	got, err := m.All(context.Background(), queries, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	zero := 0
	for _, p := range got {
		if !(p > 0) {
			zero++
		}
	}
	procs := runtime.GOMAXPROCS(0)
	scaled := time.Duration(float64(elapsed) * float64(procs) / 10)
	t.Logf("%d CDR3s' generation probabilities in %v on %d processors (%v on 10); %d not above 0", n, elapsed.Round(time.Millisecond), procs, scaled.Round(time.Millisecond), zero)
	if zero > 0 {
		t.Errorf("%d CDR3s the model made have no probability", zero)
	}
	if scaled > 2*time.Minute {
		t.Errorf("took %v on %d processors, %v on 10: budget 2 minutes", elapsed, procs, scaled)
	}
}

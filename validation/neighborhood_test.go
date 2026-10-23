// Neighborhoods (internal/neighborhood) against tcrdist3's: on Dash et al. 2017's human
// epitope-specific β chains against a background of the OTS donors' β chains with weights,
// every centroid's radius, its neighbors in the target and the background, and the rates, odds
// ratio and χ² computed from them, equal to tcrdist3 0.3's bkgd_cntl_nn2 (made by
// reference/tcrdist3_neighborhoods.py). The backgrounds ClonoLoom builds are checked against the
// simulated vaccine study's truth instead (vaccine_test.go).
package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/neighborhood"
	"clonoloom/internal/pgen"
	"clonoloom/internal/reads"
)

type neighborhoodReference struct {
	Made       string    `json:"made"`
	Control    float64   `json:"control"`
	Thresholds []int     `json:"thresholds"`
	Weights    []float64 `json:"weights"`
	Loops      struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
	} `json:"loops"`
	Target []struct {
		CDR3 string `json:"cdr3_b_aa"`
		V    string `json:"v_b_gene"`
		J    string `json:"j_b_gene"`
	} `json:"target"`
	Background []struct {
		CDR3 string `json:"cdr3_b_aa"`
		V    string `json:"v_b_gene"`
		J    string `json:"j_b_gene"`
	} `json:"background"`
	Centers []struct {
		CDR3               string  `json:"cdr3"`
		V                  string  `json:"v"`
		J                  string  `json:"j"`
		Radius             int     `json:"radius"`
		TargetHits         int     `json:"targetHits"`
		BackgroundHits     int     `json:"backgroundHits"`
		BackgroundWeighted float64 `json:"backgroundWeighted"`
		Rate               float64 `json:"rate"`
		BackgroundRate     float64 `json:"backgroundRate"`
		RelativeRate       float64 `json:"relativeRate"`
		OddsRatio          float64 `json:"oddsRatio"`
		ChiSquare          float64 `json:"chiSquare"`
	} `json:"centers"`
}

func TestNeighborhoodsEqualTCRdist3(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "tcrdist3_neighborhoods.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref neighborhoodReference
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	loops := loopsFile(t)
	db, _ := os.ReadFile(filepath.Join(dataRoot(), "tcrdist3-loops", "tcrdist", "db", ref.Loops.File))
	if sum := sha256.Sum256(db); hex.EncodeToString(sum[:]) != ref.Loops.SHA256 {
		t.Fatalf("the loops' database is not the one tcrdist3 used (%s)", ref.Loops.SHA256)
	}
	sample := &neighborhood.Repertoire{Locus: "TRB"}
	for i, r := range ref.Target {
		sample.Receptors = append(sample.Receptors, neighborhood.Receptor{CDR3: r.CDR3, V: r.V, J: r.J, Clonotype: i, Count: 1, Samples: 1})
	}
	background := &neighborhood.Repertoire{Locus: "TRB", Weight: ref.Weights}
	for _, r := range ref.Background {
		background.Receptors = append(background.Receptors, neighborhood.Receptor{CDR3: r.CDR3, V: r.V, J: r.J, Clonotype: -1, Count: 1})
	}
	if len(ref.Centers) != len(sample.Receptors) || len(background.Receptors) != len(ref.Weights) {
		t.Fatalf("%d centroids of %d receptors, %d weights of %d", len(ref.Centers), len(sample.Receptors), len(ref.Weights), len(background.Receptors))
	}
	step := 2
	if len(ref.Thresholds) > 1 {
		step = ref.Thresholds[1] - ref.Thresholds[0]
	}
	o := neighborhood.Options{Control: ref.Control, MaxRadius: ref.Thresholds[len(ref.Thresholds)-1], Step: step, MinNeighbors: 1}
	start := time.Now()
	got, err := neighborhood.Find(context.Background(), sample, background, loops, "human", o, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Centers) != len(ref.Centers) {
		t.Fatalf("%d centroids, tcrdist3's %d", len(got.Centers), len(ref.Centers))
	}
	by := map[int]neighborhood.Center{}
	for _, c := range got.Centers {
		by[c.Receptor] = c
	}
	bad := 0
	near := func(a, b float64) bool { return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b)) }
	for i, want := range ref.Centers {
		c, ok := by[i]
		if !ok {
			t.Fatalf("no centroid %d", i)
		}
		if sample.Receptors[i].CDR3 != want.CDR3 || sample.Receptors[i].V != want.V {
			t.Fatalf("centroid %d is %s %s, tcrdist3's %s %s", i, sample.Receptors[i].CDR3, sample.Receptors[i].V, want.CDR3, want.V)
		}
		switch {
		case c.Radius != want.Radius:
			bad++
			if bad < 5 {
				t.Errorf("%s: radius %d, tcrdist3's %d", want.CDR3, c.Radius, want.Radius)
			}
		case c.Neighbors != want.TargetHits || c.Background != want.BackgroundHits || !near(c.Weighted, want.BackgroundWeighted):
			bad++
			if bad < 5 {
				t.Errorf("%s at %d: %d neighbors, %d background (%.6f weighted); tcrdist3's %d, %d (%.6f)",
					want.CDR3, c.Radius, c.Neighbors, c.Background, c.Weighted, want.TargetHits, want.BackgroundHits, want.BackgroundWeighted)
			}
		case !near(c.Rate, want.Rate) || !near(c.BackgroundRate, want.BackgroundRate) || !near(c.RelativeRate, want.RelativeRate) ||
			!near(c.OddsRatio, want.OddsRatio) || !near(c.ChiSquare, want.ChiSquare):
			bad++
			if bad < 5 {
				t.Errorf("%s: rate %.12g/%.12g, relative %.12g, odds %.12g, χ² %.12g; tcrdist3's %.12g/%.12g, %.12g, %.12g, %.12g",
					want.CDR3, c.Rate, c.BackgroundRate, c.RelativeRate, c.OddsRatio, c.ChiSquare,
					want.Rate, want.BackgroundRate, want.RelativeRate, want.OddsRatio, want.ChiSquare)
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d centroids differ from tcrdist3's", bad, len(ref.Centers))
	}
	t.Logf("%d centroids against %d background receptors equal to %s, in %v", len(ref.Centers), len(background.Receptors), ref.Made, time.Since(start).Round(time.Millisecond))
}

// Budget (clonoloom-spec/design.md, "Budgets"): 10,000 receptors' neighborhoods against a
// background of 100,000 within 2 minutes on 10 cores. The receptors are the OTS donors' β chains,
// repeated to 10,000 as a cohort's would be, and the background is drawn from ClonoLoom's model
// with their V and J usage.
func TestNeighborhoodsWithinBudget(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("budgets are measured without -short and without the race detector")
	}
	loops := loopsFile(t)
	var paths []string
	for _, unit := range []string{"Sureshchandra_2023/csv_paired/SRR18365371_1_Paired_All.csv.gz", "Bai_2023/csv_paired/SRR14045264_1_Paired_All.csv.gz", "Poon_2023/csv_paired/SRR19760479_1_Paired_All.csv.gz"} {
		path := filepath.Join(dataRoot(), "ots-healthy-pbmc", filepath.FromSlash(unit))
		if _, err := os.Stat(path); err != nil {
			if required("ots-healthy-pbmc") {
				t.Fatalf("ots-healthy-pbmc is required and missing")
			}
			t.Skip("ots-healthy-pbmc not fetched")
		}
		paths = append(paths, path)
	}
	res, err := reads.ReadFiles(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Chains: []string{"TRB"}, Junction: "aa", V: "gene", J: "gene", ProductiveOnly: true, RequireAll: true})
	if err != nil {
		t.Fatal(err)
	}
	const n, background = 10_000, 100_000
	sample := &neighborhood.Repertoire{Locus: "TRB"}
	for i := 0; len(sample.Receptors) < n; i++ {
		c := table.Clonotypes[i%len(table.Clonotypes)]
		sample.Receptors = append(sample.Receptors, neighborhood.Receptor{
			CDR3: c.Chains[0].Junction, V: c.Chains[0].V, J: c.Chains[0].J,
			Clonotype: len(sample.Receptors), Count: c.Total, Samples: c.Samples,
		})
	}
	start := time.Now()
	bg, src, err := neighborhood.ModelBackground(context.Background(), sample, pgen.Simulator(), "clonoloom_human_T_beta", "1", background, 20261027, nil)
	if err != nil {
		t.Fatal(err)
	}
	made := time.Since(start)
	start = time.Now()
	found, err := neighborhood.Find(context.Background(), sample, bg, loops, "human", neighborhood.DefaultOptions(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	procs := runtime.GOMAXPROCS(0)
	scaled := time.Duration(float64(elapsed) * float64(procs) / 10)
	t.Logf("%d receptors against %d drawn in %v: %d centroids in %v on %d processors (%v on 10)",
		sample.Len(), src.Receptors, made.Round(time.Millisecond), len(found.Centers), elapsed.Round(time.Millisecond), procs, scaled.Round(time.Millisecond))
	if scaled > 2*time.Minute {
		t.Errorf("took %v on %d processors, %v on 10: budget 2 minutes", elapsed, procs, scaled)
	}
}

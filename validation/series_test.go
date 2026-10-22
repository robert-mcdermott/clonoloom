// ClonoLoom's simulated studies over time (internal/simulate, series.go) against their truth, and
// read back as a study's design (internal/study): each preset's samples read with their metadata
// and no finding, its subjects' samples placed at their days with their reference sample and
// replicates; each blood sample's share in the reference population's clonotypes as the truth
// expects it; each trajectory's clonotypes rising and falling as their folds say; the null series'
// expectations the same at every time; and replicate samples differing by the biological variance
// the study states, beyond sampling.
package validation

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"clonoloom/internal/study"
	"clonoloom/internal/validate"
)

type seriesTruth struct {
	Study   simulate.Study `json:"study"`
	Samples []struct {
		ID                  string   `json:"id"`
		Subject             string   `json:"subject"`
		ProductiveTemplates int      `json:"productiveTemplates"`
		Day                 *float64 `json:"day"`
		Reference           bool     `json:"reference"`
		ReferenceShare      *float64 `json:"referenceShare"`
		Attribution         *float64 `json:"attribution"`
	} `json:"samples"`
	Series []struct {
		Subject string `json:"subject"`
		File    string `json:"file"`
	} `json:"series"`
}

// clonotypeRow is a clonotype of a subject's series file: its trajectory, whether the reference
// holds it, and its expected frequency at each time point.
type clonotypeRow struct {
	trajectory string
	reference  bool
	frequency  []float64
}

func readSeriesFile(t *testing.T, path string) map[string]clonotypeRow {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]clonotypeRow{}
	s := bufio.NewScanner(z)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	s.Scan()
	for s.Scan() {
		c := strings.Split(s.Text(), "\t")
		row := clonotypeRow{trajectory: c[3], reference: c[4] == "T"}
		for _, v := range c[6:] {
			x, _ := strconv.ParseFloat(v, 64)
			row.frequency = append(row.frequency, x)
		}
		out[c[0]+"|"+c[1]+"|"+c[2]] = row
	}
	return out
}

// sampleClonotypes are a sample table's productive clonotypes and their templates.
func sampleClonotypes(t *testing.T, path string) (map[string]float64, float64) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	s.Scan()
	col := map[string]int{}
	for i, name := range strings.Split(s.Text(), "\t") {
		col[name] = i
	}
	out := map[string]float64{}
	total := 0.0
	for s.Scan() {
		c := strings.Split(s.Text(), "\t")
		if c[col["productive"]] != "T" {
			continue
		}
		n, _ := strconv.ParseFloat(c[col["umi_count"]], 64)
		out[c[col["v_call"]]+"|"+c[col["j_call"]]+"|"+c[col["junction_aa"]]] += n
		total += n
	}
	return out, total
}

func TestSimulatedSeriesAgainstTheirTruth(t *testing.T) {
	for _, preset := range []string{"car-t", "transplant", "null-series"} {
		t.Run(preset, func(t *testing.T) { checkSeries(t, preset) })
	}
}

func checkSeries(t *testing.T, preset string) {
	dir := filepath.Join(t.TempDir(), preset)
	if _, err := simulate.Run(context.Background(), simulate.Presets[preset], dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	var truth seriesTruth
	raw, _ := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatal(err)
	}
	ser := truth.Study.Series
	// Read as ClonoLoom reads it, and placed as the Tracking view places it.
	var paths []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tsv") || strings.HasSuffix(e.Name(), ".json") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	res, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if report := validate.Check(res); len(report.Findings) != 0 || len(res.Repertoires) != len(truth.Samples) {
		t.Fatalf("%d findings, %d repertoires of %d samples", len(report.Findings), len(res.Repertoires), len(truth.Samples))
	}
	var samples []study.Sample
	for _, r := range res.Repertoires {
		samples = append(samples, study.Sample{Name: r.RepertoireID, Subject: r.SubjectID, Time: r.TimePoint, Tissue: r.Tissue})
	}
	design := study.Describe(samples)
	replicated := 0
	for _, tp := range ser.Times {
		if tp.Replicates > 1 {
			replicated++
		}
	}
	if len(design.Subjects) != len(truth.Series) || len(design.Unassigned) != 0 {
		t.Fatalf("%d subjects, %v without one", len(design.Subjects), design.Unassigned)
	}
	for _, s := range design.Subjects {
		wantRefs := 0
		if ser.Reference != nil {
			wantRefs = 1
		}
		if len(s.References) != wantRefs || len(s.Replicates) != replicated || len(s.Untimed) != 0 {
			t.Errorf("%s: references %v, replicates %v, untimed %v", s.ID, s.References, s.Replicates, s.Untimed)
		}
		for _, p := range s.Samples {
			if p.Time != nil && (p.Time.Days == nil || p.Time.Event != ser.Event) {
				t.Errorf("%s: %+v", p.Name, p.Time)
			}
		}
	}
	for _, f := range design.Findings {
		if f.Severity == "warning" {
			t.Errorf("design: %+v", f)
		}
	}
	// Against the truth: each subject's series and samples.
	times := map[float64]int{}
	for i, tp := range ser.Times {
		times[tp.Day] = i
	}
	var attributionErrors []float64
	byClass := map[string][][2]float64{} // trajectory → at each time point, observed and expected shares pooled
	var replicateLogRatios []float64
	var replicateSampling []float64
	for _, st := range truth.Series {
		clonotypes := readSeriesFile(t, filepath.Join(dir, st.File))
		// Expectations without change in a series that has none.
		if len(ser.Trajectories) == 0 && ser.Reference == nil {
			for key, c := range clonotypes {
				for _, f := range c.frequency[1:] {
					if math.Abs(f-c.frequency[0]) > 1e-12*c.frequency[0] {
						t.Fatalf("%s %s: expected frequencies %v change", st.Subject, key, c.frequency)
					}
				}
			}
		}
		observedOf := map[string]map[string]float64{}
		totalOf := map[string]float64{}
		for _, s := range truth.Samples {
			if s.Subject != st.Subject || s.Day == nil {
				continue
			}
			obs, total := sampleClonotypes(t, filepath.Join(dir, s.ID+".tsv"))
			observedOf[s.ID], totalOf[s.ID] = obs, total
			at := times[*s.Day]
			// Attribution: the share of the sample's productive templates in the reference's
			// clonotypes.
			if s.Attribution != nil {
				in := 0.0
				for key, n := range obs {
					if clonotypes[key].reference {
						in += n
					}
				}
				attributionErrors = append(attributionErrors, in/total-*s.Attribution)
			}
			// Each trajectory's clonotypes, pooled: observed and expected share.
			for key, c := range clonotypes {
				if c.trajectory == "" {
					continue
				}
				pair := byClass[c.trajectory]
				for len(pair) < len(ser.Times) {
					pair = append(pair, [2]float64{})
				}
				pair[at][0] += obs[key] / total
				pair[at][1] += c.frequency[at]
				byClass[c.trajectory] = pair
			}
		}
		// Replicates at one time: each clonotype's log ratio between them, where both expect many
		// templates; sampling alone would give the variance of 1/n_a + 1/n_b.
		for _, tp := range ser.Times {
			if tp.Replicates < 2 {
				continue
			}
			var ids []string
			for _, s := range truth.Samples {
				if s.Subject == st.Subject && s.Day != nil && *s.Day == tp.Day {
					ids = append(ids, s.ID)
				}
			}
			sort.Strings(ids)
			a, b := ids[0], ids[1]
			for key, c := range clonotypes {
				f := c.frequency[times[tp.Day]]
				ea, eb := f*totalOf[a], f*totalOf[b]
				if ea < 30 || eb < 30 || observedOf[a][key] == 0 || observedOf[b][key] == 0 {
					continue
				}
				replicateLogRatios = append(replicateLogRatios, math.Log(observedOf[a][key]/ea)-math.Log(observedOf[b][key]/eb))
				replicateSampling = append(replicateSampling, 1/ea+1/eb)
			}
		}
	}
	if len(attributionErrors) > 0 {
		worst, sum := 0.0, 0.0
		for _, e := range attributionErrors {
			worst = math.Max(worst, math.Abs(e))
			sum += math.Abs(e)
		}
		t.Logf("attribution: %d samples, mean difference from the truth %.4f, largest %.4f", len(attributionErrors), sum/float64(len(attributionErrors)), worst)
		if worst > 0.05 {
			t.Errorf("attribution differs from the truth by up to %.4f", worst)
		}
	}
	classes := make([]string, 0, len(byClass))
	for name := range byClass {
		classes = append(classes, name)
	}
	sort.Strings(classes)
	for _, name := range classes {
		for at, p := range byClass[name] {
			if p[1] < 0.01 {
				continue // too few cells to measure their share
			}
			ratio := p[0] / p[1]
			t.Logf("%s at %s: observed share %.4f, expected %.4f", name, strconv.FormatFloat(ser.Times[at].Day, 'f', -1, 64), p[0], p[1])
			if ratio < 0.75 || ratio > 1.33 {
				t.Errorf("%s at time point %d: observed share %.4f, expected %.4f", name, at, p[0], p[1])
			}
		}
	}
	if len(replicateLogRatios) > 0 && ser.Variance > 0 {
		var ss, sampling float64
		for i, x := range replicateLogRatios {
			ss += x * x
			sampling += replicateSampling[i]
		}
		n := float64(len(replicateLogRatios))
		got := ss/n - sampling/n
		// Each sample's clone is its expectation times a gamma of shape 1/variance: the variance of
		// its logarithm is the trigamma of the shape, twice for the two samples. A variance
		// estimated from n values is itself uncertain by about √(2/n) of it: within three of that.
		want := 2 * trigamma(1/ser.Variance)
		tolerance := 3 * want * math.Sqrt(2/n)
		t.Logf("replicates: %d clonotypes, variance of the log ratio beyond sampling %.4f, the model's %.4f (± %.4f)", len(replicateLogRatios), got, want, tolerance)
		if math.Abs(got-want) > tolerance {
			t.Errorf("replicates differ by %.4f beyond sampling, the model %.4f ± %.4f", got, want, tolerance)
		}
	}
}

// trigamma is ψ′(x), x > 0: by recurrence to x ≥ 6, then its asymptotic series.
func trigamma(x float64) float64 {
	out := 0.0
	for x < 6 {
		out += 1 / (x * x)
		x++
	}
	inv := 1 / x
	inv2 := inv * inv
	return out + inv + inv2/2 + inv*inv2*(1.0/6-inv2*(1.0/30-inv2/42))
}

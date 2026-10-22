// ClonoLoom's rarefaction and extrapolation (internal/diversity) on the development files' bulk
// samples and the simulated standard study, against iNEXT 3.0.2 as validation/reference/
// rarefaction.R computes it: the same sizes along each curve; there, the same Hill numbers of
// order 0, 1 and 2 and sample coverage; the same Hill numbers at each coverage; and two cohorts
// compared at the same size and coverage. The intervals are each a bootstrap's, so their widths
// are compared within a band.
package validation

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"clonoloom/internal/diversity"
)

type rarefactionOrder struct {
	QD    []float64  `json:"qD"`
	Lower []*float64 `json:"qD.LCL"`
	Upper []*float64 `json:"qD.UCL"`
}

type rarefactionReference struct {
	N         float64 `json:"n"`
	Bootstrap bool    `json:"bootstrap"`
	Size      struct {
		M       []float64                   `json:"m"`
		SC      []float64                   `json:"SC"`
		SCLower []*float64                  `json:"SC.LCL"`
		SCUpper []*float64                  `json:"SC.UCL"`
		Orders  map[string]rarefactionOrder `json:"orders"`
	} `json:"size"`
	Coverage struct {
		SC     []float64                   `json:"SC"`
		M      []float64                   `json:"m"`
		Orders map[string]rarefactionOrder `json:"orders"`
	} `json:"coverage"`
}

type cohortLevel struct {
	Assemblage []string  `json:"Assemblage"`
	M          []float64 `json:"m"`
	Method     []string  `json:"Method"`
	Order      []float64 `json:"Order.q"`
	SC         []float64 `json:"SC"`
	QD         []float64 `json:"qD"`
	Lower      []float64 `json:"qD.LCL"`
	Upper      []float64 `json:"qD.UCL"`
}

// z95 is the standard normal's 97.5% quantile, as iNEXT's intervals use it.
const z95 = 1.959963984540054

// seOf recovers iNEXT's standard error from its interval, from whichever bound it did not clamp.
func seOf(estimate float64, lower, upper *float64, lo, hi float64) float64 {
	switch {
	case upper != nil && *upper < hi:
		return (*upper - estimate) / z95
	case lower != nil && *lower > lo:
		return (estimate - *lower) / z95
	}
	return math.NaN()
}

// agree says whether ClonoLoom's value at a size or coverage is iNEXT's, to what each can be
// expected to agree: 10⁻⁹ (relative); the Shannon diversity extrapolated, 5·10⁻⁷, since its β
// divides by the small difference between the values at n and n − 1, which magnifies iNEXT's
// rounding of the value at n (R sums a term for every clonotype: S01-1's Shannon diversity is
// 2768.6720701115 in iNEXT, 2768.6720701097 in ClonoLoom and 2768.67207010965 exactly, and the
// value at 2n differs by 3·10⁻⁹); at a coverage reached by rarefaction, by what iNEXT's
// minimization leaves of the size there (R's optimize stops within √ε·m + 4·10⁻⁵ of it, doubled
// here), since a Hill number rises by at most about one a cell.
func agree(got, want float64, q int, method string, atCoverage bool, size float64) bool {
	tol := 1e-9 * math.Max(1, math.Abs(want))
	if q == 1 && method == "extrapolated" {
		tol = 5e-7 * math.Abs(want)
	}
	if atCoverage && method == "interpolated" {
		tol = 2*(1.49e-8*size+4.1e-5) + 1e-9*math.Abs(want)
	}
	return math.Abs(got-want) <= tol
}

func TestRarefactionAgreesWithINEXT(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "rarefaction.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string                                     `json:"generated_by"`
		Files       map[string]map[string]rarefactionReference `json:"files"`
		Cohorts     map[string]struct {
			Size     cohortLevel `json:"size"`
			Coverage cohortLevel `json:"coverage"`
		} `json:"cohorts"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	opt := diversity.DefaultOptions()
	if raceDetector {
		opt.Replicates = 0 // the values only: the bootstrap is compared without the race detector
	}
	var files []string
	for rel := range ref.Files {
		files = append(files, rel)
	}
	sort.Strings(files)
	samples := map[string][]float64{} // by name, for the cohorts
	var ratios []float64
	ratio := func(where string, ours, theirs float64) {
		if opt.Replicates == 0 || !(theirs > 1e-9) || math.IsNaN(ours) {
			return
		}
		ratios = append(ratios, ours/theirs)
		if r := ours / theirs; r < 0.7 || r > 1.43 {
			t.Errorf("%s: bootstrap SE %.4g, iNEXT's %.4g", where, ours, theirs)
		}
	}
	compared, flat := 0, 0
	for _, rel := range files {
		path, ok := referencePath(t, rel)
		if !ok {
			if required(strings.Split(rel, "/")[0]) {
				t.Errorf("%s is required and missing", rel)
			}
			continue
		}
		abundances := sampleAbundances(t, path)
		for name, want := range ref.Files[rel] {
			x, ok := abundances[name]
			if !ok {
				t.Errorf("%s: no sample %s", rel, name)
				continue
			}
			samples[name] = x
			where := rel + " " + name
			c := diversity.Rarefy(x, diversity.Levels{}, opt)
			if c.Abundance != want.N {
				t.Errorf("%s: n %v, iNEXT's %v", where, c.Abundance, want.N)
				continue
			}
			// iNEXT's sizes, each among the curve's (which adds sizes spaced on a logarithmic scale).
			bySize := map[float64]diversity.Point{}
			for _, p := range c.BySize {
				bySize[float64(p.Size)] = p
			}
			for i, m := range want.Size.M {
				p, ok := bySize[m]
				if !ok {
					t.Errorf("%s: no size %v on the curve", where, m)
					continue
				}
				if got := float64(p.Coverage.Estimate); math.Abs(got-want.Size.SC[i]) > 1e-9 {
					t.Errorf("%s at %v: coverage %.12g, iNEXT %.12g", where, m, got, want.Size.SC[i])
				}
				if want.Bootstrap {
					ratio(where+" coverage at "+fmtFloat(m), float64(p.Coverage.SE), seOf(want.Size.SC[i], want.Size.SCLower[i], want.Size.SCUpper[i], 0, 1))
				}
				for q := range 3 {
					o := want.Size.Orders[fmtFloat(float64(q))]
					got := float64(p.Hill[q].Estimate)
					if !agree(got, o.QD[i], q, p.Method, false, m) {
						t.Errorf("%s at %v, q %d: %.12g, iNEXT %.12g", where, m, q, got, o.QD[i])
					}
					if want.Bootstrap {
						ratio(where+" q"+fmtFloat(float64(q))+" at "+fmtFloat(m), float64(p.Hill[q].SE), seOf(o.QD[i], o.Lower[i], o.Upper[i], 0, math.Inf(1)))
					}
				}
			}
			// By coverage: iNEXT's coverages are those its sizes reach, so each is among ClonoLoom's.
			// Where the coverage hardly changes with size, the size at a coverage is not determined,
			// and iNEXT's minimization, and its bootstrap, answer any size along the flat stretch:
			// those coverages are not compared. Flat: by less than 10⁻⁹ from one of iNEXT's sizes to
			// the next, or by less than 0.05 along the whole curve (the AIRR Community's test files,
			// of three clonotypes each, whose coverage is 0.998 at one cell).
			if span := want.Size.SC[len(want.Size.SC)-1] - want.Size.SC[0]; span < 0.05 {
				flat += len(want.Coverage.SC)
				compared++
				continue
			}
			for i, target := range want.Coverage.SC {
				j := 0
				for k := range want.Size.SC {
					if math.Abs(want.Size.SC[k]-target) < math.Abs(want.Size.SC[j]-target) {
						j = k
					}
				}
				if (j > 0 && want.Size.SC[j]-want.Size.SC[j-1] < 1e-9) || (j+1 < len(want.Size.SC) && want.Size.SC[j+1]-want.Size.SC[j] < 1e-9) {
					flat++
					continue
				}
				var p diversity.Point
				for k, x := range c.ByCoverage {
					if k == 0 || math.Abs(float64(x.Coverage.Estimate)-target) < math.Abs(float64(p.Coverage.Estimate)-target) {
						p = x
					}
				}
				tol := 1e-9
				if p.Method == "interpolated" {
					tol = 2e-5
				}
				if got := float64(p.Coverage.Estimate); math.Abs(got-target) > tol {
					t.Errorf("%s: no coverage %.12g (the nearest %.12g)", where, target, got)
					continue
				}
				if math.Abs(float64(p.Size)-want.Coverage.M[i]) > 0.5+1e-9 {
					t.Errorf("%s at coverage %.6f: size %v, iNEXT %v", where, target, float64(p.Size), want.Coverage.M[i])
				}
				for q := range 3 {
					o := want.Coverage.Orders[fmtFloat(float64(q))]
					got := float64(p.Hill[q].Estimate)
					if !agree(got, o.QD[i], q, p.Method, true, float64(p.Size)) {
						t.Errorf("%s at coverage %.6f, q %d: %.12g, iNEXT %.12g", where, target, q, got, o.QD[i])
					}
					if want.Bootstrap {
						ratio(where+" q"+fmtFloat(float64(q))+" at coverage "+fmtFloat(target), float64(p.Hill[q].SE), seOf(o.QD[i], o.Lower[i], o.Upper[i], 0, math.Inf(1)))
					}
				}
			}
			compared++
		}
	}
	if compared == 0 {
		t.Skip("no development data: node validation/fetch.mjs --ci")
	}
	// The cohorts: the same levels suggested, the same values there.
	for name, levels := range ref.Cohorts {
		var names []string
		for _, s := range levels.Size.Assemblage {
			if len(names) == 0 || names[len(names)-1] != s {
				names = append(names, s)
			}
		}
		var xs [][]float64
		for _, s := range names {
			if samples[s] == nil {
				t.Errorf("cohort %s: no sample %s", name, s)
			}
			xs = append(xs, samples[s])
		}
		// iNEXT's defaults are the levels ClonoLoom suggests with extrapolation.
		// The coverage each extrapolated sample reaches is the level itself (a rarefied one's carries
		// iNEXT's minimization).
		suggested, ok := diversity.Suggest(xs)
		if !ok || suggested.ExtrapolatedSize != levels.Size.M[0] {
			t.Errorf("cohort %s: suggested %+v; iNEXT size %v", name, suggested, levels.Size.M[0])
			continue
		}
		for i, method := range levels.Coverage.Method {
			if method != "Rarefaction" && math.Abs(suggested.ExtrapolatedCoverage-levels.Coverage.SC[i]) > 1e-12 {
				t.Errorf("cohort %s: suggested coverage %.15g; iNEXT %.15g", name, suggested.ExtrapolatedCoverage, levels.Coverage.SC[i])
			}
		}
		for _, level := range []struct {
			what string
			rows cohortLevel
		}{{"size", levels.Size}, {"coverage", levels.Coverage}} {
			for i, s := range level.rows.Assemblage {
				lv := diversity.Levels{Knots: 4}
				if level.what == "size" {
					lv.Sizes = []float64{suggested.ExtrapolatedSize}
				} else {
					lv.Coverages = []float64{suggested.ExtrapolatedCoverage}
				}
				c := diversity.Rarefy(samples[s], lv, opt)
				p := append(c.AtSize, c.AtCoverage...)[0]
				q := int(level.rows.Order[i])
				where := "cohort " + name + " " + s + " at the common " + level.what + ", q " + fmtFloat(float64(q))
				if math.Abs(float64(p.Size)-level.rows.M[i]) > 1e-3 {
					t.Errorf("%s: size %v, iNEXT %v", where, float64(p.Size), level.rows.M[i])
				}
				if got := float64(p.Hill[q].Estimate); !agree(got, level.rows.QD[i], q, p.Method, level.what == "coverage", float64(p.Size)) {
					t.Errorf("%s: %.12g, iNEXT %.12g", where, got, level.rows.QD[i])
				}
				lower := level.rows.Lower[i]
				ratio(where, float64(p.Hill[q].SE), seOf(level.rows.QD[i], &lower, &level.rows.Upper[i], 0, math.Inf(1)))
			}
		}
	}
	sort.Float64s(ratios)
	if len(ratios) > 0 {
		t.Logf("%d samples compared (%s), %d coverages where the size is not determined left out; %d bootstrap SEs over iNEXT's: median %.3f, 2.5%%–97.5%% %.3f–%.3f, range %.3f–%.3f", compared, ref.GeneratedBy, flat, len(ratios),
			ratios[len(ratios)/2], ratios[len(ratios)*25/1000], ratios[len(ratios)*975/1000], ratios[0], ratios[len(ratios)-1])
	}
}

func fmtFloat(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

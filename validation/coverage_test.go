// How often the diversity intervals hold the truth (clonoloom-spec/requirements.md, R1): samples
// drawn from repertoires whose diversity is known (clonotypes of power-law sizes), each measured
// with its bootstrap intervals (internal/diversity), the share of 95% intervals holding the
// truth counted. The asymptotic estimates' truth is the repertoire's own Hill numbers; the
// observed measures', their expected value at the sample's size (exactly, for the clonotypes
// seen; the mean over the samples, for the others).
//
// A quick version runs with the tests; CLONOLOOM_COVERAGE=1 runs the full one (200 samples of
// each repertoire, 200 replicates each), whose results clonoloom-spec/research.md §5.3 reports.
package validation

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"clonoloom/internal/diversity"
	"clonoloom/internal/random"
)

type repertoire struct {
	name     string
	clones   int
	alpha    float64
	depth    int
	freq     []float64
	hill     [3]float64
	expected float64 // E[clonotypes seen] at depth
}

func makeRepertoire(name string, clones int, alpha float64, depth int, seed uint64) repertoire {
	r := random.New(seed, 0)
	rep := repertoire{name: name, clones: clones, alpha: alpha, depth: depth, freq: make([]float64, clones)}
	total := 0.0
	for i := range rep.freq {
		rep.freq[i] = math.Floor(math.Pow(1-r.Float64(), -1/alpha))
		total += rep.freq[i]
	}
	var h, simpson float64
	for i := range rep.freq {
		rep.freq[i] /= total
		p := rep.freq[i]
		h -= p * math.Log(p)
		simpson += p * p
		rep.expected += 1 - math.Pow(1-p, float64(depth))
	}
	rep.hill = [3]float64{float64(clones), math.Exp(h), 1 / simpson}
	return rep
}

func TestIntervalCoverageOnSimulations(t *testing.T) {
	if raceDetector {
		t.Skip("measured without the race detector")
	}
	samples, replicates := 40, 100
	full := os.Getenv("CLONOLOOM_COVERAGE") == "1"
	if full {
		samples, replicates = 200, 200
	} else if testing.Short() {
		t.Skip("-short")
	}
	repertoires := []repertoire{
		makeRepertoire("well sampled (5,000 clonotypes, 50,000 templates)", 5000, 1.3, 50000, 11),
		makeRepertoire("undersampled (50,000 clonotypes, 20,000 templates)", 50000, 1.3, 20000, 12),
	}
	var report strings.Builder
	for ri, rep := range repertoires {
		type stat struct{ held, n, width float64 }
		measures := []string{"chao1", "hill1Estimated", "hill2Estimated", "richness", "hill1", "hill2"}
		stats := map[string]*stat{}
		for _, m := range measures {
			stats[m] = &stat{}
		}
		results := make([]diversity.Result, samples)
		var counts []int
		var observedMean [3]float64
		for s := range samples {
			r := random.New(uint64(1000+ri), uint64(s))
			counts = r.Multinomial(rep.depth, rep.freq, counts)
			x := make([]float64, 0, len(counts))
			for _, c := range counts {
				if c > 0 {
					x = append(x, float64(c))
				}
			}
			opt := diversity.DefaultOptions()
			opt.Replicates, opt.Stream = replicates, uint64(ri<<20|s)
			results[s] = diversity.Measure(x, opt)
			for k, id := range []string{"richness", "hill1", "hill2"} {
				observedMean[k] += float64(results[s].Measures[id].Estimate) / float64(samples)
			}
		}
		// The observed Hill numbers' expectation at this depth: exact for the clonotypes seen; the
		// mean over the samples for order 1 and 2.
		truth := map[string]float64{"chao1": rep.hill[0], "hill1Estimated": rep.hill[1], "hill2Estimated": rep.hill[2],
			"richness": rep.expected, "hill1": observedMean[1], "hill2": observedMean[2]}
		var coverage float64
		for _, res := range results {
			coverage += float64(res.Measures["coverage"].Estimate) / float64(samples)
			for _, m := range measures {
				v := res.Measures[m]
				st := stats[m]
				st.n++
				st.width += float64(v.Upper-v.Lower) / truth[m] / float64(samples)
				if float64(v.Lower) <= truth[m] && truth[m] <= float64(v.Upper) {
					st.held++
				}
			}
		}
		fmt.Fprintf(&report, "%s: Hill numbers %.0f, %.1f, %.1f; mean sample coverage %.3f\n", rep.name, rep.hill[0], rep.hill[1], rep.hill[2], coverage)
		for _, m := range measures {
			st := stats[m]
			fmt.Fprintf(&report, "  %-15s held the truth in %5.1f%% of %d samples (mean width %.1f%% of the truth)\n", m, 100*st.held/st.n, int(st.n), 100*st.width)
		}
		// The estimates the bootstrap is meant for hold the truth as often as they say, where the
		// repertoire is well sampled; the observed ones, at any depth.
		minimum := 0.85
		if !full {
			minimum = 0.75
		}
		for _, m := range []string{"hill1Estimated", "hill2Estimated", "richness", "hill1", "hill2"} {
			if ri == 1 && (m == "hill1Estimated") {
				continue // undersampled: the asymptotic entropy is biased low, its coverage reported only
			}
			if share := stats[m].held / stats[m].n; share < minimum {
				t.Errorf("%s: %s's intervals held the truth in %.1f%% of samples", rep.name, m, 100*share)
			}
		}
	}
	t.Log("\n" + report.String())
}

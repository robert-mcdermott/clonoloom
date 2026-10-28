// Package cohort compares saved subject observations, never cells or sequences.
// Welch (1947), doi:10.1093/biomet/34.1-2.28; Student (1908), doi:10.2307/2331554;
// Mann & Whitney (1947), doi:10.1214/aoms/1177730491; Wilcoxon (1945), doi:10.2307/3001968.
// BH: Benjamini & Hochberg (1995), doi:10.1111/j.2517-6161.1995.tb02031.x.
// Probability intervals use Hoeffding (1963), doi:10.1080/01621459.1963.10500830,
// and McDiarmid (1989), doi:10.1017/CBO9781107359949.008. Their conservative
// use here, missingness refusal, fixed two-test family and permutation limits
// are ClonoLoom policies; see research.md §5.32 for assumptions and derivations.
package cohort

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"clonoloom/internal/diversity"
	"clonoloom/internal/dmath"
	"clonoloom/internal/expansion"
	"clonoloom/internal/stats"
	"clonoloom/internal/study"
)

const Version = 1
const ExactLimit = 50000
const WorkLimit = 50000000
const Citation = "Welch 1947, doi:10.1093/biomet/34.1-2.28; Student 1908, doi:10.2307/2331554; Mann & Whitney 1947, doi:10.1214/aoms/1177730491; Wilcoxon 1945, doi:10.2307/3001968; Benjamini & Hochberg 1995, doi:10.1111/j.2517-6161.1995.tb02031.x; Hoeffding 1963, doi:10.1080/01621459.1963.10500830; McDiarmid 1989, doi:10.1017/CBO9781107359949.008; Phipson & Smyth 2010, doi:10.2202/1544-6115.1585."

type Options struct {
	Features     []string `json:"features"`
	Confidence   float64  `json:"confidence"`
	Permutations int      `json:"permutations"`
	Seed         uint64   `json:"seed"`
}

func (o Options) Normalize() (Options, error) {
	if len(o.Features) < 1 || len(o.Features) > 13 {
		return o, errors.New("choose 1–13 features before testing")
	}
	o.Features = append([]string(nil), o.Features...)
	sort.Strings(o.Features)
	for i, id := range o.Features {
		if id == "" || i > 0 && id == o.Features[i-1] {
			return o, errors.New("feature IDs must be distinct and nonempty")
		}
	}
	if o.Confidence == 0 {
		o.Confidence = .95
	}
	if !(o.Confidence >= .8 && o.Confidence <= .99) {
		return o, errors.New("confidence must be between 0.8 and 0.99")
	}
	if o.Permutations == 0 {
		o.Permutations = 9999
	}
	if o.Permutations < 999 || o.Permutations > 99999 {
		return o, errors.New("permutations must be 999–99999")
	}
	if o.Seed == 0 {
		o.Seed = 20261025
	}
	if o.Seed > 1<<53-1 {
		return o, errors.New("seed must be at most 9007199254740991")
	}
	return o, nil
}

type Effect struct {
	Name     string           `json:"name"`
	Unit     string           `json:"unit"`
	Estimate diversity.Number `json:"estimate"`
	Lower    diversity.Number `json:"lower"`
	Upper    diversity.Number `json:"upper"`
	Interval string           `json:"interval"`
	Reason   string           `json:"reason"`
}
type Test struct {
	Name       string           `json:"name"`
	Statistic  diversity.Number `json:"statistic"`
	DF         diversity.Number `json:"df"`
	P          diversity.Number `json:"p"`
	Adjusted   diversity.Number `json:"adjusted"`
	Reason     string           `json:"reason"`
	NullMethod string           `json:"nullMethod"`
	Draws      int              `json:"draws"`
	Tail       int              `json:"tail"`
	Nonzero    int              `json:"nonzero"`
}
type Point struct {
	Subject string           `json:"subject"`
	Sample  string           `json:"sample"`
	Group   string           `json:"group"`
	Value   diversity.Number `json:"value"`
	Missing string           `json:"missing"`
}
type Comparison struct {
	Feature        study.Feature    `json:"feature"`
	NReference     int              `json:"nReference"`
	NComparison    int              `json:"nComparison"`
	Pairs          int              `json:"pairs"`
	Missing        int              `json:"missing"`
	MeanReference  diversity.Number `json:"meanReference"`
	MeanComparison diversity.Number `json:"meanComparison"`
	Mean           Effect           `json:"mean"`
	Probability    Effect           `json:"probability"`
	Parametric     Test             `json:"parametric"`
	Rank           Test             `json:"rank"`
	Points         []Point          `json:"points"`
}
type Result struct {
	Options     Options      `json:"options"`
	Layout      string       `json:"layout"`
	Levels      []string     `json:"levels"`
	Family      int          `json:"family"`
	Tested      int          `json:"tested"`
	Description string       `json:"description"`
	Comparisons []Comparison `json:"comparisons"`
}

func missing() diversity.Number { return diversity.Number(math.NaN()) }
func finite(x float64) bool     { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func emptyTest(name string) Test {
	return Test{Name: name, Statistic: missing(), DF: missing(), P: missing(), Adjusted: missing()}
}
func emptyEffect(name, unit, interval string) Effect {
	return Effect{Name: name, Unit: unit, Estimate: missing(), Lower: missing(), Upper: missing(), Interval: interval}
}

// Compute rejects an incomplete/mislabeled input table and leaves a feature untested
// if any selected subject value is missing. No complete-case deletion is performed.
func Compute(ctx context.Context, input study.CohortInput, table study.FeatureTable, options Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	design, err := study.CheckCohort(input)
	if err != nil {
		return nil, err
	}
	if !design.OK {
		return nil, errors.New("design does not pass subject-level structural checks")
	}
	if len(table.Rows) != design.Included {
		return nil, errors.New("feature rows differ from the selected design")
	}
	expected := map[string]study.CohortRow{}
	for _, row := range design.Rows {
		if row.Included {
			expected[row.Name] = row
		}
	}
	rows := append([]study.FeatureRow(nil), table.Rows...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Subject != rows[j].Subject {
			return rows[i].Subject < rows[j].Subject
		}
		return rows[i].Name < rows[j].Name
	})
	for _, row := range rows {
		r, ok := expected[row.Name]
		if !ok || r.Subject != row.Subject || r.Group != row.Group || len(row.Values) != len(table.Catalog) {
			return nil, errors.New("feature row membership or width differs from the design")
		}
		delete(expected, row.Name)
	}
	catalog := map[string]int{}
	for i, f := range table.Catalog {
		if _, ok := catalog[f.ID]; ok {
			return nil, errors.New("duplicate feature ID")
		}
		catalog[f.ID] = i
	}
	result := &Result{Options: o, Layout: design.Input.Spec.Layout, Levels: design.Input.Spec.Levels, Family: 2 * len(o.Features), Comparisons: []Comparison{}}
	budget := WorkLimit
	for _, id := range o.Features {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		col, ok := catalog[id]
		if !ok {
			return nil, fmt.Errorf("unknown feature %q", id)
		}
		f := table.Catalog[col]
		c := Comparison{Feature: f, MeanReference: missing(), MeanComparison: missing(), Mean: emptyEffect("Mean difference (comparison minus reference)", f.Unit, "Two-sided t interval; marginal, not multiplicity-adjusted"), Probability: emptyEffect("Probability of superiority, half credit for ties", "fraction", "Conservative bounded-difference interval; marginal, not multiplicity-adjusted"), Parametric: emptyTest("Welch t"), Rank: emptyTest("Mann–Whitney U (comparison group)"), Points: []Point{}}
		paired := result.Layout == "paired"
		if paired {
			c.Parametric.Name = "Paired t"
			c.Rank.Name = "Wilcoxon signed-rank W+"
			c.Probability.Name = "Within-subject probability of increase, half credit for zero difference"
		}
		var a, b []float64
		for _, row := range rows {
			v := row.Values[col]
			x := float64(v.Value)
			reason := v.Missing
			if !finite(x) && reason == "" {
				reason = "Value unavailable"
			}
			c.Points = append(c.Points, Point{Subject: row.Subject, Sample: row.Name, Group: row.Group, Value: v.Value, Missing: reason})
			if reason != "" {
				c.Missing++
			}
			if row.Group == result.Levels[0] {
				a = append(a, x)
			} else {
				b = append(b, x)
			}
		}
		c.NReference, c.NComparison = len(a), len(b)
		if paired {
			c.Pairs = len(a)
		}
		if c.Missing > 0 {
			reason := fmt.Sprintf("%d selected observations have missing values; no subjects removed", c.Missing)
			c.Parametric.Reason = reason
			c.Rank.Reason = reason
			c.Mean.Reason = reason
			c.Probability.Reason = reason
		} else {
			parametric(&c, a, b, paired, o.Confidence)
			rank, probability, err := rankTest(ctx, a, b, paired, o, id, &budget)
			if err != nil {
				return nil, err
			}
			c.Rank = rank
			c.Probability = probability
		}
		result.Comparisons = append(result.Comparisons, c)
	}
	// Untestable slots retain p=1 in the denominator but are displayed as missing.
	ps := make([]float64, result.Family)
	for i, c := range result.Comparisons {
		ps[2*i], ps[2*i+1] = 1, 1
		if finite(float64(c.Parametric.P)) {
			ps[2*i] = float64(c.Parametric.P)
			result.Tested++
		}
		if finite(float64(c.Rank.P)) {
			ps[2*i+1] = float64(c.Rank.P)
			result.Tested++
		}
	}
	qs := expansion.AdjustBH(ps)
	for i := range result.Comparisons {
		c := &result.Comparisons[i]
		if finite(float64(c.Parametric.P)) {
			c.Parametric.Adjusted = diversity.Number(qs[2*i])
		}
		if finite(float64(c.Rank.P)) {
			c.Rank.Adjusted = diversity.Number(qs[2*i+1])
		}
	}
	result.Description = fmt.Sprintf("Two-group %s subject comparison: %q minus %q; %d subjects and %d selected observations. Each selected feature receives a two-sided %s and %s. BH family: both tests for all %d selected features (%d slots; %d testable); untestable slots contribute p=1 but remain missing. No correction across other results, contrasts or exploratory feature selection. BH control assumes independence or suitable positive dependence; arbitrary dependence is not guaranteed. %.1f%% intervals are marginal, not multiplicity-adjusted. Mean intervals use the t model; probability intervals use conservative bounded-difference bounds, not rank-test inversion or median-shift intervals. Any missing selected observation leaves that feature untested. No automatic outlier removal, transformations, pooling, covariate adjustment or depth standardization. Subjects must be independent between biological units. Welch assumes independent samples and normal means (small samples especially require plausible normality); paired t assumes normal subject differences. Mann–Whitney tests identical distributions under label exchangeability, not merely equal medians; signed-rank assumes differences symmetric about zero with exchangeable signs. Literal saved floating-point values define ties and exact zero differences. Rank tails include ties and use absolute centered rank scores, exact up to %d assignments, otherwise %d Monte Carlo draws with (tail+1)/(draws+1), seed %d. Plots show observed subject values, paired links and separately labeled effects; absence of significance is not equivalence or evidence of no effect.", result.Layout, result.Levels[1], result.Levels[0], design.Subjects, design.Included, result.Comparisons[0].Parametric.Name, result.Comparisons[0].Rank.Name, len(o.Features), result.Family, result.Tested, 100*o.Confidence, ExactLimit, o.Permutations, o.Seed)
	return result, nil
}

func moments(x []float64) (mean, variance float64) {
	// Shifted two-pass moments avoid cancellation for tightly clustered values.
	base := x[0]
	sum := 0.0
	for _, v := range x {
		sum += v - base
	}
	mean = base + sum/float64(len(x))
	for _, v := range x {
		d := v - mean
		variance += float64(d * d)
	}
	variance /= float64(len(x) - 1)
	return
}
func parametric(c *Comparison, a, b []float64, paired bool, level float64) {
	ma, va := moments(a)
	mb, vb := moments(b)
	c.MeanReference = diversity.Number(ma)
	c.MeanComparison = diversity.Number(mb)
	effect := mb - ma
	v1, v2 := va/float64(len(a)), vb/float64(len(b))
	variance := v1 + v2
	df := float64(variance*variance) / (float64(v1*v1)/float64(len(a)-1) + float64(v2*v2)/float64(len(b)-1))
	if paired {
		diff := make([]float64, len(a))
		for i := range a {
			diff[i] = b[i] - a[i]
		}
		var v float64
		effect, v = moments(diff)
		variance = v / float64(len(diff))
		df = float64(len(diff) - 1)
	}
	c.Mean.Estimate = diversity.Number(effect)
	if !(variance > 0) || !finite(variance) || !finite(effect) || !finite(df) {
		c.Mean.Reason = "Zero or non-finite standard error; t inference unavailable"
		c.Parametric.Reason = c.Mean.Reason
		return
	}
	se := math.Sqrt(variance)
	t := effect / se
	p := stats.StudentTTwoSided(t, df)
	q := stats.StudentTQuantile(1-float64((1-level)/2), df)
	half := float64(q * se)
	if !finite(t) || !finite(p) || !finite(half) {
		c.Mean.Reason = "Non-finite t inference"
		c.Parametric.Reason = c.Mean.Reason
		return
	}
	c.Mean.Lower = diversity.Number(effect - half)
	c.Mean.Upper = diversity.Number(effect + half)
	c.Parametric.Statistic = diversity.Number(t)
	c.Parametric.DF = diversity.Number(df)
	c.Parametric.P = diversity.Number(p)
}

func probabilityInterval(estimate, scale, level float64, name string) Effect {
	radius := math.Sqrt(float64(dmath.Log(2/(1-level))*scale) / 2)
	return Effect{Name: name, Unit: "fraction", Estimate: diversity.Number(estimate), Lower: diversity.Number(math.Max(0, estimate-radius)), Upper: diversity.Number(math.Min(1, estimate+radius)), Interval: "Conservative bounded-difference interval; marginal, not multiplicity-adjusted"}
}

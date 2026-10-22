package track

// Two-sample tests between a subject's time points (clonoloom-spec/requirements.md, T2 and T3;
// roadmap.md, wave 3, slices 6 and 7): each subject's samples at one time point (pooled,
// replicates and all) against its samples at another, each clonotype tested by internal/expansion,
// the false discovery rate controlled within each subject. The beta-binomial test's variance is
// fitted once for the study: on each subject's replicate samples (every pair taken at one time),
// or on two time points the user names as stable (each subject's samples at them, pooled).

import (
	"errors"
	"math"
	"sort"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/expansion"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

// Comparison is two time points compared, by their keys (study.Placed: "day 0", "baseline",
// "reference"), and how; Stable, where given, the two time points the beta-binomial test's
// variance is fitted on instead of the replicate samples.
type Comparison struct {
	From string `json:"from"`
	To   string `json:"to"`
	expansion.Options
	Stable *Stable `json:"stable,omitempty"`
}

// Stable is two time points between which the clonotypes are taken not to change.
type Stable struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Normalize completes a comparison's options and checks its time points.
func (c Comparison) Normalize() (Comparison, error) {
	if c.From == "" || c.To == "" {
		return c, errors.New("compare: give the two time points, from and to")
	}
	if c.From == c.To {
		return c, errors.New("compare: the two time points are the same")
	}
	o, err := c.Options.Normalize()
	if err != nil {
		return c, err
	}
	c.Options = o
	if c.Test == expansion.Binomial || c.Test == expansion.Fisher {
		c.Stable = nil // nothing is fitted
	}
	if c.Stable != nil && (c.Stable.From == "" || c.Stable.To == "" || c.Stable.From == c.Stable.To) {
		return c, errors.New("stable: give two different time points")
	}
	return c, nil
}

// Tested is a subject's comparison: the two time points, and each clonotype tested between them.
type Tested struct {
	Subject string `json:"subject"`
	From    Point  `json:"from"`
	To      Point  `json:"to"`
	expansion.Result
}

// Fit is the beta-binomial test's variance model and what it was fitted on: replicate samples, or
// two time points named as stable. With it, the share of the pairs' clonotypes a two-sided test at
// 0.05 calls changed: by the binomial test, and by the beta-binomial test with the model (about
// 0.05 where the model holds); and up to 3,000 of the clonotypes, each by its total in the pair and
// its count in the first sample as a z-score of sampling, (k − nθ)/√(nθ(1 − θ)), for a plot.
type Fit struct {
	On    string    `json:"on"`
	Pairs []FitPair `json:"pairs"`
	expansion.Fitted
	CalledBySampling float64    `json:"calledBySampling"`
	CalledByModel    float64    `json:"calledByModel"`
	Points           []FitPoint `json:"points"`
}

// FitPair is a pair of samples (or of groups of samples) the model was fitted on: the subject, the
// two, the share θ of the clonotypes' total the first holds, and the clonotypes with at least
// minTotal in the two.
type FitPair struct {
	Subject    string   `json:"subject"`
	A          []string `json:"a"`
	B          []string `json:"b"`
	Theta      float64  `json:"theta"`
	Clonotypes int      `json:"clonotypes"`
}

// FitPoint is a clonotype of a pair the model was fitted on.
type FitPoint struct {
	N int64   `json:"n"`
	Z float64 `json:"z"`
}

// Tests is a comparison made in every subject that has both its time points (Without: those
// that have not): the test used (the beta-binomial test where its variance could be fitted, when
// asked for automatically), the fit, and what to keep in mind reading it.
type Tests struct {
	Comparison Comparison `json:"comparison"`
	Test       string     `json:"test"`
	Fit        *Fit       `json:"fit,omitempty"`
	Subjects   []Tested   `json:"subjects"`
	Without    []string   `json:"without,omitempty"`
	Caveat     string     `json:"caveat"`
}

// ErrReads: the tests refuse counts of reads.
var ErrReads = errors.New("the counts are of reads: each cell or molecule is counted as many times as it was amplified and sequenced, so a test of them would call nearly every difference significant. The tests need templates, UMIs or cells")

// The caveats, by the test used.
var caveats = map[string]string{
	expansion.BetaBinomial: "The beta-binomial test allows for the variance between samples beyond sampling, as the samples it was fitted on show it (below); it takes that variance to be the same at every time point and in every subject.",
	expansion.Binomial:     "The binomial test accounts for sampling alone: two blood samples taken at one time differ by more, so between two time points it calls some clonotypes changed that only varied.",
	expansion.Fisher:       "Fisher's exact test, as the binomial test, accounts for sampling alone: two blood samples taken at one time differ by more, so between two time points it calls some clonotypes changed that only varied.",
}

// Test makes the comparison in each subject of the design, in the clonotype table t whose samples
// are named by names.
func Test(t *clonotype.Table, names []string, design study.Design, c Comparison) (Tests, error) {
	c, err := c.Normalize()
	if err != nil {
		return Tests{}, err
	}
	if t.Unit == rep.Reads {
		return Tests{}, ErrReads
	}
	index := indexOf(names)
	model, fit, note, err := prepare(t, index, design, c)
	if err != nil {
		return Tests{}, err
	}
	out := Tests{Comparison: c, Fit: fit, Subjects: []Tested{}, Test: c.Test}
	for _, s := range design.Subjects {
		points, _, _ := Timeline(t, index, s)
		tested, ok, err := compare(t, index, s, points, c, model)
		if err != nil {
			return Tests{}, err
		}
		if !ok {
			out.Without = append(out.Without, s.ID)
			continue
		}
		out.Test = tested.Test
		out.Subjects = append(out.Subjects, tested)
	}
	if out.Test == expansion.Auto {
		out.Test = expansion.Binomial
		if model != nil {
			out.Test = expansion.BetaBinomial
		}
	}
	out.Caveat = caveats[out.Test]
	if c.Stable != nil && out.Test == expansion.BetaBinomial {
		out.Caveat += " Fitted on two time points named as stable, it takes nothing to have changed between them."
	}
	if note != "" {
		out.Caveat = note + " " + out.Caveat
	}
	if t.Unit == rep.UnitUnknown {
		out.Caveat += " The files do not say whether the counts are of reads or of cells or molecules: the tests assume cells or molecules."
	}
	return out, nil
}

func indexOf(names []string) map[string]uint32 {
	index := map[string]uint32{}
	for i, n := range names {
		index[n] = uint32(i)
	}
	return index
}

// prepare fits the beta-binomial test's variance where the comparison asks for it: answers the
// model and its fit, or, asked for automatically and without samples to fit on, a note saying the
// binomial test is used instead.
func prepare(t *clonotype.Table, index map[string]uint32, design study.Design, c Comparison) (*expansion.Model, *Fit, string, error) {
	if c.Test != expansion.Auto && c.Test != expansion.BetaBinomial {
		return nil, nil, "", nil
	}
	fit, err := fitVariance(t, index, design, c)
	if errors.Is(err, expansion.ErrNoObservations) && c.Test == expansion.Auto {
		if c.Stable != nil {
			return nil, nil, "No subject has samples at both time points named as stable, with clonotypes to fit the variance on: the binomial test is used.", nil
		}
		return nil, nil, "No replicate samples (two of one subject at one time) to fit the variance on: the binomial test is used. Give replicate samples, or name two time points as stable, for the beta-binomial test.", nil
	}
	if err != nil {
		return nil, nil, "", err
	}
	return &fit.Model, fit, "", nil
}

// fitVariance fits the variance model on the pairs of samples that should not differ.
func fitVariance(t *clonotype.Table, index map[string]uint32, design study.Design, c Comparison) (*Fit, error) {
	fit := &Fit{On: "replicates", Pairs: []FitPair{}}
	type group struct {
		subject string
		a, b    []string
	}
	var groups []group
	for _, s := range design.Subjects {
		if c.Stable != nil {
			points, _, _ := Timeline(t, index, s)
			from, to := pointOf(points, c.Stable.From), pointOf(points, c.Stable.To)
			if from != nil && to != nil {
				groups = append(groups, group{s.ID, from.Samples, to.Samples})
			}
			continue
		}
		for _, r := range s.Replicates {
			for i := 0; i < len(r); i++ {
				for j := i + 1; j < len(r); j++ {
					groups = append(groups, group{s.ID, []string{r[i]}, []string{r[j]}})
				}
			}
		}
	}
	if c.Stable != nil {
		fit.On = "stable"
	}
	// Each pair's clonotypes with at least minTotal in the two: their counts, by (k, n), and the
	// share θ the first holds of them all.
	type key struct{ k, n int64 }
	var obs []expansion.Observation
	type counted struct {
		k, n int64
	}
	var all []struct {
		theta float64
		items []counted
	}
	for _, g := range groups {
		inA, inB := members(index, g.a), members(index, g.b)
		if len(inA) == 0 || len(inB) == 0 {
			continue
		}
		var items []counted
		var sumA, sumN float64
		for i := range t.Clonotypes {
			samples, values := t.Abundances(i)
			var ka, kb float64
			for k, s := range samples {
				if inA[s] {
					ka += values[k]
				}
				if inB[s] {
					kb += values[k]
				}
			}
			if ka+kb < float64(c.MinTotal) {
				continue
			}
			if ka != math.Trunc(ka) || kb != math.Trunc(kb) {
				return nil, expansion.ErrNotCounts
			}
			items = append(items, counted{int64(ka), int64(ka + kb)})
			sumA += ka
			sumN += ka + kb
		}
		if len(items) == 0 {
			continue
		}
		theta := sumA / sumN
		fit.Pairs = append(fit.Pairs, FitPair{Subject: g.subject, A: g.a, B: g.b, Theta: theta, Clonotypes: len(items)})
		all = append(all, struct {
			theta float64
			items []counted
		}{theta, items})
		histogram := map[key]int{}
		var keys []key
		for _, it := range items {
			kk := key{it.k, it.n}
			if histogram[kk] == 0 {
				keys = append(keys, kk)
			}
			histogram[kk]++
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].n != keys[j].n {
				return keys[i].n < keys[j].n
			}
			return keys[i].k < keys[j].k
		})
		for _, kk := range keys {
			obs = append(obs, expansion.Observation{K: kk.k, N: kk.n, Theta: theta, Count: histogram[kk]})
		}
	}
	fitted, err := expansion.Fit(obs)
	if err != nil {
		return nil, err
	}
	fit.Fitted = fitted
	// What a test at 0.05 calls between them; and the points of the plot.
	var bySampling, byModel int
	var points []FitPoint
	for _, o := range obs {
		if expansion.BinomialP(o.K, o.N, o.Theta, expansion.TwoSided) < 0.05 {
			bySampling += o.Count
		}
		if expansion.BetaBinomialP(o.K, o.N, o.Theta, fitted.Model.V(float64(o.N)), expansion.TwoSided) < 0.05 {
			byModel += o.Count
		}
	}
	fit.CalledBySampling = float64(bySampling) / float64(fitted.Clonotypes)
	fit.CalledByModel = float64(byModel) / float64(fitted.Clonotypes)
	for _, a := range all {
		for _, it := range a.items {
			sd := math.Sqrt(float64(float64(it.n)*a.theta) * (1 - a.theta))
			points = append(points, FitPoint{N: it.n, Z: (float64(it.k) - float64(float64(it.n)*a.theta)) / sd})
		}
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].N < points[j].N })
	step := (len(points) + 2999) / 3000
	for i := 0; i < len(points); i += max(step, 1) {
		fit.Points = append(fit.Points, points[i])
	}
	return fit, nil
}

// members is the table's indices of samples, by name.
func members(index map[string]uint32, names []string) map[uint32]bool {
	out := map[uint32]bool{}
	for _, n := range names {
		if i, ok := index[n]; ok {
			out[i] = true
		}
	}
	return out
}

// compare makes the comparison in one subject, whose points Timeline placed: the samples at each
// of its two time points pooled. Answers false when the subject has not both.
func compare(t *clonotype.Table, index map[string]uint32, s study.Subject, points []Point, c Comparison, model *expansion.Model) (Tested, bool, error) {
	from, to := pointOf(points, c.From), pointOf(points, c.To)
	if from == nil || to == nil {
		return Tested{}, false, nil
	}
	group := func(p *Point) []uint32 {
		var out []uint32
		for _, name := range p.Samples {
			out = append(out, index[name])
		}
		return out
	}
	res, err := expansion.Compare(t, group(from), group(to), c.Options, model)
	if errors.Is(err, expansion.ErrNotCounts) && t.Unit == rep.Reads {
		err = ErrReads
	}
	if err != nil {
		return Tested{}, false, err
	}
	return Tested{Subject: s.ID, From: *from, To: *to, Result: res}, true, nil
}

// pointOf is the subject's time point of a key, those of one key (two reference samples) pooled.
func pointOf(points []Point, key string) *Point {
	var out *Point
	for _, p := range points {
		if p.Key != key {
			continue
		}
		if out == nil {
			q := p
			q.Samples = append([]string(nil), p.Samples...)
			out = &q
			continue
		}
		out.Samples = append(out.Samples, p.Samples...)
		out.Total += p.Total
	}
	return out
}

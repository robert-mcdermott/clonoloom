package track

// Classes of clonotypes across several time points (clonoloom-spec/requirements.md, T4; roadmap.md,
// wave 3, slice 8): in each subject, each clonotype tested between a base time point and every
// later one (internal/expansion, the beta-binomial test's variance fitted once for the study, the
// false discovery rate controlled within each comparison), and classed by those calls:
//   - emerging: called increased at the last time point;
//   - contracting: called decreased at the last time point;
//   - transient: called changed at a time point between, not at the last;
//   - persistent: never called, and found at the base and at every later time point.
// The others (never called, missing at one time point or more) are not classed.

import (
	"errors"
	"math"
	"sort"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/expansion"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

// The classes, in the order the views list them.
const (
	Emerging    = "emerging"
	Transient   = "transient"
	Contracting = "contracting"
	Persistent  = "persistent"
)

// Classes lists the classes in order.
var Classes = []string{Emerging, Transient, Contracting, Persistent}

// MaxPersistent is the most persistent clonotypes a subject lists (the largest); all are counted.
const MaxPersistent = 200

// Classing is how clonotypes are classed: the base time point (a key, as Comparison's), the tests'
// options, and, where given, the two time points the beta-binomial test's variance is fitted on.
type Classing struct {
	Base string `json:"base"`
	expansion.Options
	Stable *Stable `json:"stable,omitempty"`
}

// Normalize completes a classing's options and checks it.
func (c Classing) Normalize() (Classing, error) {
	if c.Base == "" {
		return c, errors.New("class: give the base time point")
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

// ClassedClonotype is a clonotype classed: its class, its frequency at the base and at each later
// time point (the samples at one time pooled), its call at each later time point against the base
// ("" where none or not tested), and the least adjusted p-value of its calls (1 for a persistent
// clonotype).
type ClassedClonotype struct {
	ID          string            `json:"id"`
	Chains      []clonotype.Chain `json:"chains"`
	Class       string            `json:"class"`
	Frequencies []float64         `json:"frequencies"`
	Calls       []string          `json:"calls"`
	Q           float64           `json:"q"`
}

// ClassCounts are a subject's clonotypes by class; Unclassed those never called and missing at a
// time point or more.
type ClassCounts struct {
	Emerging    int `json:"emerging"`
	Transient   int `json:"transient"`
	Contracting int `json:"contracting"`
	Persistent  int `json:"persistent"`
	Unclassed   int `json:"unclassed"`
}

// Classed is a subject's clonotypes classed: the base and later time points, the counts, and the
// clonotypes of each class (those called, the most significant first; the largest persistent ones,
// at most MaxPersistent).
type Classed struct {
	Subject    string             `json:"subject"`
	Base       Point              `json:"base"`
	Later      []Point            `json:"later"`
	Counts     ClassCounts        `json:"counts"`
	Clonotypes []ClassedClonotype `json:"clonotypes"`
}

// ClassResult is the clonotypes of every subject with the base time point and a later one
// (Without: those without), classed; the test used, the variance fitted, and what to keep in mind.
type ClassResult struct {
	Classing Classing  `json:"classing"`
	Test     string    `json:"test"`
	Fit      *Fit      `json:"fit,omitempty"`
	Subjects []Classed `json:"subjects"`
	Without  []string  `json:"without,omitempty"`
	Caveat   string    `json:"caveat"`
}

// Classify classes the clonotypes of each subject of the design, in the clonotype table t whose
// samples are named by names.
func Classify(t *clonotype.Table, names []string, design study.Design, c Classing) (ClassResult, error) {
	c, err := c.Normalize()
	if err != nil {
		return ClassResult{}, err
	}
	if t.Unit == rep.Reads {
		return ClassResult{}, ErrReads
	}
	index := indexOf(names)
	cmp := Comparison{From: c.Base, Options: c.Options, Stable: c.Stable}
	model, fit, note, err := prepare(t, index, design, cmp)
	if err != nil {
		return ClassResult{}, err
	}
	out := ClassResult{Classing: c, Fit: fit, Subjects: []Classed{}, Test: c.Test}
	for _, s := range design.Subjects {
		classed, used, ok, err := classify(t, index, s, c, model)
		if err != nil {
			return ClassResult{}, err
		}
		if !ok {
			out.Without = append(out.Without, s.ID)
			continue
		}
		out.Test = used
		out.Subjects = append(out.Subjects, classed)
	}
	if out.Test == expansion.Auto {
		out.Test = expansion.Binomial
		if model != nil {
			out.Test = expansion.BetaBinomial
		}
	}
	out.Caveat = caveats[out.Test] + " Each later time point is compared with the base, the false discovery rate controlled within each comparison: a clonotype called at one of several comparisons is more likely a false discovery than one called at a single comparison."
	if note != "" {
		out.Caveat = note + " " + out.Caveat
	}
	return out, nil
}

// classify classes one subject's clonotypes; answers false when it has not the base and a later
// time point.
func classify(t *clonotype.Table, index map[string]uint32, s study.Subject, c Classing, model *expansion.Model) (Classed, string, bool, error) {
	points, _, _ := Timeline(t, index, s)
	base := pointOf(points, c.Base)
	if base == nil {
		return Classed{}, "", false, nil
	}
	// The later time points: those after the base, reference samples aside, each key once.
	var later []Point
	seen := map[string]bool{c.Base: true}
	after := false
	for _, p := range points {
		if p.Key == c.Base {
			after = true
			continue
		}
		if !after || p.Reference != "" || seen[p.Key] {
			continue
		}
		seen[p.Key] = true
		later = append(later, *pointOf(points, p.Key))
	}
	if len(later) == 0 {
		return Classed{}, "", false, nil
	}
	group := func(p Point) []uint32 {
		var out []uint32
		for _, name := range p.Samples {
			out = append(out, index[name])
		}
		return out
	}
	// Each clonotype's calls and adjusted p-values at each later time point.
	type state struct {
		calls []string
		q     []float64
	}
	byID := map[string]*state{}
	used := c.Test
	for k, p := range later {
		res, err := expansion.Compare(t, group(*base), group(p), c.Options, model)
		if err != nil {
			return Classed{}, "", false, err
		}
		used = res.Test
		for _, x := range res.Clonotypes {
			st := byID[x.ID]
			if st == nil {
				st = &state{calls: make([]string, len(later)), q: make([]float64, len(later))}
				for i := range st.q {
					st.q[i] = math.NaN()
				}
				byID[x.ID] = st
			}
			st.calls[k], st.q[k] = x.Call, x.Q
		}
	}
	// Every clonotype at the subject's base and later time points, classed.
	all := append([]Point{*base}, later...)
	members := make([]map[uint32]bool, len(all))
	for k, p := range all {
		members[k] = map[uint32]bool{}
		for _, i := range group(p) {
			members[k][i] = true
		}
	}
	out := Classed{Subject: s.ID, Base: *base, Later: later, Clonotypes: []ClassedClonotype{}}
	var persistent []ClassedClonotype
	var totals []float64
	for i, ct := range t.Clonotypes {
		samples, values := t.Abundances(i)
		counts := make([]float64, len(all))
		found := false
		for j, s := range samples {
			for k := range all {
				if members[k][s] {
					counts[k] += values[j]
					found = true
				}
			}
		}
		if !found {
			continue
		}
		x := ClassedClonotype{ID: ct.ID, Chains: ct.Chains, Calls: make([]string, len(later)), Frequencies: make([]float64, len(all)), Q: 1}
		for k := range all {
			x.Frequencies[k] = counts[k] / all[k].Total
		}
		called := false
		if st := byID[ct.ID]; st != nil {
			copy(x.Calls, st.calls)
			for k, call := range st.calls {
				if call != "" {
					called = true
					x.Q = math.Min(x.Q, st.q[k])
				}
			}
		}
		last := x.Calls[len(later)-1]
		switch {
		case last == expansion.Increase:
			x.Class = Emerging
			out.Counts.Emerging++
		case last == expansion.Decrease:
			x.Class = Contracting
			out.Counts.Contracting++
		case called:
			x.Class = Transient
			out.Counts.Transient++
		default:
			everywhere := true
			for _, n := range counts {
				everywhere = everywhere && n > 0
			}
			if !everywhere {
				out.Counts.Unclassed++
				continue
			}
			x.Class = Persistent
			out.Counts.Persistent++
			total := 0.0
			for _, n := range counts {
				total += n
			}
			persistent = append(persistent, x)
			totals = append(totals, total)
			continue
		}
		out.Clonotypes = append(out.Clonotypes, x)
	}
	rank := map[string]int{Emerging: 0, Transient: 1, Contracting: 2}
	sort.SliceStable(out.Clonotypes, func(i, j int) bool {
		a, b := out.Clonotypes[i], out.Clonotypes[j]
		if a.Class != b.Class {
			return rank[a.Class] < rank[b.Class]
		}
		return a.Q < b.Q
	})
	order := make([]int, len(persistent))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return totals[order[i]] > totals[order[j]] })
	for _, i := range order[:min(len(order), MaxPersistent)] {
		out.Clonotypes = append(out.Clonotypes, persistent[i])
	}
	return out, used, true, nil
}

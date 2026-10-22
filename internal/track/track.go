// Package track follows chosen clonotypes through each subject's samples over time
// (clonoloom-spec/requirements.md, T1 and T6; roadmap.md, wave 3, slice 5): a reference sample's
// largest clonotypes, each sample's largest, a list, or those a test called between two time
// points (test.go; T2, slice 6), each one's count and frequency in every
// sample of its subject and at every time point (the samples taken at one time pooled), with the
// frequency's interval (Wilson's score interval) where the counts are of cells or molecules. A
// clonotype absent from a sample has a count of 0 and an interval from 0 to what the sample's
// depth could have missed, never a frequency of zero taken as known.
package track

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

// The ways of choosing the clonotypes followed.
const (
	// ByReference: each of the subject's reference samples' n largest clonotypes (each sample's
	// largest, for a subject without a reference sample).
	ByReference = "reference"
	// ByLargest: each of the subject's samples' n largest clonotypes.
	ByLargest = "largest"
	// ByList: the clonotypes a list names, by ID or junction.
	ByList = "list"
	// ByTest: the n clonotypes a test called changed between two time points, the most significant
	// first.
	ByTest = "test"
)

// MaxFollowed is the most clonotypes followed in one subject; MaxN the most of each sample's
// largest; MaxList the most entries of a list.
const (
	MaxFollowed = 200
	MaxN        = 100
	MaxList     = 1000
)

// Choice is which clonotypes are followed: by a reference sample, each sample's largest, a list
// (clonotype IDs, or junctions as the definition writes them), or a test's calls.
type Choice struct {
	By   string      `json:"by"`
	N    int         `json:"n,omitempty"`
	List []string    `json:"list,omitempty"`
	Test *Comparison `json:"test,omitempty"`
}

// Normalize completes a choice: the default way (a reference sample's), its default number (10 of
// a reference sample, 5 of each sample, 20 of a test's calls), a list trimmed of blanks and
// repeats, a comparison's defaults.
func (c Choice) Normalize() (Choice, error) {
	switch c.By {
	case "":
		c.By = ByReference
	case ByReference, ByLargest, ByList, ByTest:
	default:
		return c, fmt.Errorf("track by %q: by is reference, largest, list or test", c.By)
	}
	if c.By == ByTest {
		if c.Test == nil {
			return c, errors.New("track by test: give the comparison (from, to, and the test's options)")
		}
		cmp, err := c.Test.Normalize()
		if err != nil {
			return c, err
		}
		c.Test, c.List = &cmp, nil
	} else {
		c.Test = nil
	}
	if c.By == ByList {
		c.N = 0
		seen := map[string]bool{}
		var list []string
		for _, e := range c.List {
			e = strings.TrimSpace(e)
			if e != "" && !seen[e] {
				seen[e] = true
				list = append(list, e)
			}
		}
		if len(list) == 0 {
			return c, errors.New("track by list: name at least one clonotype, by its ID or junction")
		}
		if len(list) > MaxList {
			return c, fmt.Errorf("track by list: at most %d clonotypes", MaxList)
		}
		c.List = list
		return c, nil
	}
	c.List = nil
	if c.N == 0 {
		c.N = map[string]int{ByReference: 10, ByLargest: 5, ByTest: 20}[c.By]
	}
	if c.N < 1 || c.N > MaxN {
		return c, fmt.Errorf("n: from 1 to %d", MaxN)
	}
	return c, nil
}

// Value is a count out of a total: its frequency, and the frequency's interval where the counts
// are of cells or molecules (missing otherwise).
type Value struct {
	Count     float64          `json:"count"`
	Total     float64          `json:"total"`
	Frequency diversity.Number `json:"frequency"`
	Lower     diversity.Number `json:"lower"`
	Upper     diversity.Number `json:"upper"`
}

// Point is one of a subject's time points: its samples (those taken at one time together), or a
// reference sample on its own. Kind is reference (a reference sample without a time point),
// before, day, visit or after; Key names its time across subjects (study.Placed); Followed the
// share of its abundance in the clonotypes followed.
type Point struct {
	Key       string           `json:"key"`
	Label     string           `json:"label"`
	Kind      string           `json:"kind"`
	Days      *float64         `json:"days,omitempty"`
	Event     string           `json:"event,omitempty"`
	Reference string           `json:"reference,omitempty"`
	Samples   []string         `json:"samples"`
	Total     float64          `json:"total"`
	Followed  diversity.Number `json:"followed"`
}

// Sample is one of a subject's samples followed: its time point (an index of the subject's
// points) and its abundance among the clonotypes.
type Sample struct {
	Name  string  `json:"name"`
	Point int     `json:"point"`
	Total float64 `json:"total"`
}

// Reason is why a clonotype is followed: its rank among a sample's largest, the list's entry that
// names it, or the test's call (increase or decrease) and its rank among the calls.
type Reason struct {
	Sample string `json:"sample,omitempty"`
	Rank   int    `json:"rank,omitempty"`
	Listed string `json:"listed,omitempty"`
	Called string `json:"called,omitempty"`
}

// Followed is a clonotype followed through a subject's samples: its chains, why, its value in
// each sample (in the subject's order) and at each time point (its samples pooled), and the time
// point where its frequency is highest.
type Followed struct {
	ID      string            `json:"id"`
	Chains  []clonotype.Chain `json:"chains"`
	Why     []Reason          `json:"why"`
	Samples []Value           `json:"samples"`
	Points  []Value           `json:"points"`
	Peak    int               `json:"peak"`
}

// Subject is a subject's samples in time order, its time points, and its clonotypes followed,
// those highest at their peak first. By is how they were chosen (a subject without a reference
// sample has each sample's n largest); More the clonotypes chosen beyond MaxFollowed, left out.
type Subject struct {
	ID         string     `json:"id"`
	By         string     `json:"by"`
	Samples    []Sample   `json:"samples"`
	Points     []Point    `json:"points"`
	Clonotypes []Followed `json:"clonotypes"`
	More       int        `json:"more,omitempty"`
}

// Result is the clonotypes followed in every subject with two samples or more, or one and a
// reference sample. Intervals says whether frequencies have intervals (Caveat says why not, or
// what to keep in mind); Unmatched the list's entries that name no clonotype; Without the subjects
// left out, with too few samples to follow anything through (or, following a test's calls, without
// both its time points).
type Result struct {
	Choice     Choice    `json:"choice"`
	Confidence float64   `json:"confidence"`
	Subjects   []Subject `json:"subjects"`
	Intervals  bool      `json:"intervals"`
	Caveat     string    `json:"caveat,omitempty"`
	Unmatched  []string  `json:"unmatched,omitempty"`
	Without    []string  `json:"without,omitempty"`
}

// Follow follows the clonotypes chosen through each subject of the design (study.Describe), in the
// clonotype table t whose samples are named by names (in the table's order), with intervals at
// the confidence given (0.95 when not between 0 and 1).
func Follow(t *clonotype.Table, names []string, design study.Design, choice Choice, confidence float64) (Result, error) {
	choice, err := choice.Normalize()
	if err != nil {
		return Result{}, err
	}
	if confidence <= 0 || confidence >= 1 {
		confidence = 0.95
	}
	res := Result{Choice: choice, Confidence: confidence, Subjects: []Subject{}, Intervals: true}
	z := diversity.NormalQuantile(0.5 + float64(confidence/2))
	switch t.Unit {
	case rep.Reads:
		res.Intervals = false
		res.Caveat = "The counts are of reads: each cell or molecule is counted as many times as it was amplified and sequenced, so a frequency's interval from them would be far too narrow, and none is given. Use templates, UMIs or cells where the data have them."
	case rep.UnitUnknown:
		res.Caveat = "The files do not say whether the counts are of reads or of cells or molecules: the intervals assume cells or molecules, and are far too narrow if they are reads."
	}
	index := map[string]uint32{}
	for i, n := range names {
		index[n] = uint32(i)
	}
	var listed map[int][]string
	if choice.By == ByList {
		listed, res.Unmatched = match(t, choice.List)
	}

	// Each subject's samples, and the samples of every subject followed, to read the table once.
	type plan struct {
		subject study.Subject
		samples []uint32
		sub     Subject
		tested  *Tested
	}
	var plans []*plan
	wanted := map[uint32]bool{}
	for _, s := range design.Subjects {
		p := &plan{subject: s, sub: Subject{ID: s.ID, Clonotypes: []Followed{}}}
		p.sub.Points, p.sub.Samples, p.samples = Timeline(t, index, s)
		if len(p.sub.Points) < 2 {
			res.Without = append(res.Without, s.ID)
			continue
		}
		if choice.By == ByTest {
			tested, ok, err := compare(t, index, s, p.sub.Points, *choice.Test)
			if err != nil {
				return Result{}, err
			}
			if !ok {
				res.Without = append(res.Without, s.ID)
				continue
			}
			p.tested = &tested
		}
		p.sub.By = choice.By
		if choice.By == ByReference && len(s.References) == 0 {
			p.sub.By = ByLargest
		}
		for _, i := range p.samples {
			wanted[i] = true
		}
		plans = append(plans, p)
	}

	// Each wanted sample's clonotypes, for its largest.
	present := map[uint32][]entry{}
	if choice.By == ByReference || choice.By == ByLargest {
		for i := range t.Clonotypes {
			samples, values := t.Abundances(i)
			for k, s := range samples {
				if wanted[s] && values[k] > 0 {
					present[s] = append(present[s], entry{i, values[k]})
				}
			}
		}
	}

	for _, p := range plans {
		sub := &p.sub
		chosen := map[int][]Reason{}
		var order []int
		add := func(i int, r Reason) {
			if _, ok := chosen[i]; !ok {
				order = append(order, i)
			}
			chosen[i] = append(chosen[i], r)
		}
		switch sub.By {
		case ByTest:
			byID := map[string]int{}
			for i, c := range t.Clonotypes {
				byID[c.ID] = i
			}
			rank := 0
			for _, c := range p.tested.Clonotypes {
				if c.Call == "" || rank == choice.N {
					break
				}
				rank++
				add(byID[c.ID], Reason{Called: c.Call, Rank: rank})
			}
		case ByList:
			ids := make([]int, 0, len(listed))
			for i := range listed {
				ids = append(ids, i)
			}
			sort.Ints(ids)
			for _, i := range ids {
				for _, e := range listed[i] {
					add(i, Reason{Listed: e})
				}
			}
		default:
			for _, placed := range p.subject.Samples {
				i, ok := index[placed.Name]
				if !ok || (sub.By == ByReference && placed.Reference == "") {
					continue
				}
				for rank, e := range largest(present[i], choice.N) {
					add(e.clonotype, Reason{Sample: placed.Name, Rank: rank + 1})
				}
			}
		}
		for _, i := range order {
			sub.Clonotypes = append(sub.Clonotypes, follow(t, i, chosen[i], p.samples, sub, res.Intervals, z))
		}
		// Those highest at their peak first; then by the table's order.
		sort.SliceStable(sub.Clonotypes, func(a, b int) bool {
			fa, fb := peakOf(sub.Clonotypes[a]), peakOf(sub.Clonotypes[b])
			return fa > fb
		})
		if len(sub.Clonotypes) > MaxFollowed {
			sub.More = len(sub.Clonotypes) - MaxFollowed
			sub.Clonotypes = sub.Clonotypes[:MaxFollowed]
		}
		// Each point's share in the clonotypes followed.
		for k := range sub.Points {
			pt := &sub.Points[k]
			in := 0.0
			for _, c := range sub.Clonotypes {
				in += c.Points[k].Count
			}
			pt.Followed = diversity.Number(math.NaN())
			if pt.Total > 0 {
				pt.Followed = diversity.Number(in / pt.Total)
			}
		}
		res.Subjects = append(res.Subjects, *sub)
	}
	return res, nil
}

// Timeline places a subject's samples of the table (by name, index) at its time points, in time
// order: the samples taken at one time together, a reference sample on its own. Answers the points
// (each with its total), the samples with their points, and the samples' indices in the table.
func Timeline(t *clonotype.Table, index map[string]uint32, s study.Subject) ([]Point, []Sample, []uint32) {
	points, samples, indices := []Point{}, []Sample{}, []uint32{}
	// The time of the last point, when it is not a reference sample's: samples at that time join it.
	var last *study.Time
	for _, placed := range s.Samples {
		i, ok := index[placed.Name]
		if !ok {
			continue
		}
		indices = append(indices, i)
		if placed.Reference == "" && last != nil && study.Same(*placed.Time, *last) {
			n := len(points) - 1
			points[n].Samples = append(points[n].Samples, placed.Name)
			points[n].Total += t.SampleTotals[i]
			samples = append(samples, Sample{Name: placed.Name, Point: n, Total: t.SampleTotals[i]})
			continue
		}
		pt := Point{Key: placed.Key, Kind: "reference", Label: placed.Reference, Reference: placed.Reference, Samples: []string{placed.Name}, Total: t.SampleTotals[i]}
		if placed.Time != nil {
			pt.Kind, pt.Label, pt.Days, pt.Event = placed.Time.Kind, placed.Time.Label, placed.Time.Days, placed.Time.Event
		}
		last = nil
		if placed.Reference == "" {
			last = placed.Time
		}
		points = append(points, pt)
		samples = append(samples, Sample{Name: placed.Name, Point: len(points) - 1, Total: t.SampleTotals[i]})
	}
	return points, samples, indices
}

// entry is a clonotype's abundance in a sample.
type entry struct {
	clonotype int
	abundance float64
}

// largest answers a sample's n largest clonotypes, ties in the table's order.
func largest(list []entry, n int) []entry {
	out := append([]entry(nil), list...)
	sort.Slice(out, func(a, b int) bool {
		if out[a].abundance != out[b].abundance {
			return out[a].abundance > out[b].abundance
		}
		return out[a].clonotype < out[b].clonotype
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// match finds the clonotypes a list names: by ID, or by a junction (one chain's, or every chain's
// as the table writes them, "CASS…; CAVS…"), amino acids in either case. Answers the entries that
// name each clonotype, and those that name none.
func match(t *clonotype.Table, list []string) (map[int][]string, []string) {
	byKey := map[string][]int{}
	for i, c := range t.Clonotypes {
		byKey[c.ID] = append(byKey[c.ID], i)
		keys := []string{strings.ToUpper(c.Junctions())}
		if len(c.Chains) > 1 {
			for _, ch := range c.Chains {
				keys = append(keys, strings.ToUpper(ch.Junction))
			}
		}
		for _, k := range keys {
			if k != "" {
				byKey[k] = append(byKey[k], i)
			}
		}
	}
	out := map[int][]string{}
	var unmatched []string
	for _, e := range list {
		found := byKey[e]
		if len(found) == 0 {
			parts := strings.Split(strings.ToUpper(e), ";")
			for k := range parts {
				parts[k] = strings.TrimSpace(parts[k])
			}
			found = byKey[strings.Join(parts, "; ")]
		}
		if len(found) == 0 {
			unmatched = append(unmatched, e)
			continue
		}
		for _, i := range found {
			if !contains(out[i], e) {
				out[i] = append(out[i], e)
			}
		}
	}
	return out, unmatched
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// follow is clonotype i's value in each of the subject's samples and at each of its points.
func follow(t *clonotype.Table, i int, why []Reason, samples []uint32, sub *Subject, intervals bool, z float64) Followed {
	c := t.Clonotypes[i]
	f := Followed{ID: c.ID, Chains: c.Chains, Why: why, Samples: make([]Value, len(samples)), Points: make([]Value, len(sub.Points))}
	for k, s := range samples {
		n := t.In(i, s)
		f.Samples[k] = Value{Count: n, Total: t.SampleTotals[s]}
		f.Points[sub.Samples[k].Point].Count += n
		f.Points[sub.Samples[k].Point].Total += t.SampleTotals[s]
	}
	for k := range f.Samples {
		f.Samples[k] = valueOf(f.Samples[k].Count, f.Samples[k].Total, intervals, z)
	}
	for k := range f.Points {
		f.Points[k] = valueOf(f.Points[k].Count, f.Points[k].Total, intervals, z)
		if float64(f.Points[k].Frequency) > float64(f.Points[f.Peak].Frequency) {
			f.Peak = k
		}
	}
	return f
}

func peakOf(f Followed) float64 {
	v := float64(f.Points[f.Peak].Frequency)
	if math.IsNaN(v) {
		return -1
	}
	return v
}

// valueOf is k of n with its frequency, and its interval where asked and both are whole numbers.
func valueOf(k, n float64, intervals bool, z float64) Value {
	v := Value{Count: k, Total: n, Frequency: diversity.Number(math.NaN()), Lower: diversity.Number(math.NaN()), Upper: diversity.Number(math.NaN())}
	if n <= 0 {
		return v
	}
	v.Frequency = diversity.Number(k / n)
	if intervals && k == math.Trunc(k) && n == math.Trunc(n) {
		lo, hi := Wilson(k, n, z)
		v.Lower, v.Upper = diversity.Number(lo), diversity.Number(hi)
	}
	return v
}

// Wilson is Wilson's score interval for a proportion, k of n, at the standard normal quantile z
// (Wilson 1927): (p + z²/2n ± z √(p(1−p)/n + z²/4n²)) / (1 + z²/n). It holds 0 (k = 0) and 1
// (k = n) at its ends, never goes beyond them, and at k = 0 its upper end is about z²/n, what a
// sample of n could miss.
func Wilson(k, n, z float64) (float64, float64) {
	if n <= 0 {
		return math.NaN(), math.NaN()
	}
	p := k / n
	z2 := float64(z * z)
	denom := 1 + z2/n
	center := (p + z2/float64(2*n)) / denom
	half := float64(z / denom * math.Sqrt(float64(p*(1-p))/n+z2/float64(4*float64(n*n))))
	lo, hi := center-half, center+half
	if k <= 0 || lo < 0 {
		lo = 0
	}
	if k >= n || hi > 1 {
		hi = 1
	}
	return lo, hi
}

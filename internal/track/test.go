package track

// Two-sample tests between a subject's time points (clonoloom-spec/requirements.md, T2; roadmap.md,
// wave 3, slice 6): each subject's samples at one time point (pooled, replicates and all) against
// its samples at another, each clonotype tested by internal/expansion, the false discovery rate
// controlled within each subject.

import (
	"errors"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/expansion"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

// Comparison is two time points compared, by their keys (study.Placed: "day 0", "baseline",
// "reference"), and how.
type Comparison struct {
	From string `json:"from"`
	To   string `json:"to"`
	expansion.Options
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
	c.Options = o
	return c, err
}

// Tested is a subject's comparison: the two time points, and each clonotype tested between them.
type Tested struct {
	Subject string `json:"subject"`
	From    Point  `json:"from"`
	To      Point  `json:"to"`
	expansion.Result
}

// Tests is a comparison made in every subject that has both its time points (Without: those
// that have not), and what to keep in mind reading it.
type Tests struct {
	Comparison Comparison `json:"comparison"`
	Subjects   []Tested   `json:"subjects"`
	Without    []string   `json:"without,omitempty"`
	Caveat     string     `json:"caveat"`
}

// ErrReads: the tests refuse counts of reads.
var ErrReads = errors.New("the counts are of reads: each cell or molecule is counted as many times as it was amplified and sequenced, so a test of them would call nearly every difference significant. The tests need templates, UMIs or cells")

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
	out := Tests{Comparison: c, Subjects: []Tested{}, Caveat: "These tests account for sampling alone: two blood samples taken at one time differ by more, so between two time points they call some clonotypes changed that only varied. Replicate samples, taken at one time, measure that variance."}
	if c.Test == expansion.Fisher {
		out.Caveat = "Fisher's exact test, as the binomial test, accounts for sampling alone: two blood samples taken at one time differ by more, so between two time points it calls some clonotypes changed that only varied. Replicate samples, taken at one time, measure that variance."
	}
	if t.Unit == rep.UnitUnknown {
		out.Caveat += " The files do not say whether the counts are of reads or of cells or molecules: the tests assume cells or molecules."
	}
	index := map[string]uint32{}
	for i, n := range names {
		index[n] = uint32(i)
	}
	for _, s := range design.Subjects {
		points, _, _ := Timeline(t, index, s)
		tested, ok, err := compare(t, index, s, points, c)
		if err != nil {
			return Tests{}, err
		}
		if !ok {
			out.Without = append(out.Without, s.ID)
			continue
		}
		out.Subjects = append(out.Subjects, tested)
	}
	return out, nil
}

// compare makes the comparison in one subject, whose points Timeline placed: the samples at each
// of its two time points pooled. Answers false when the subject has not both.
func compare(t *clonotype.Table, index map[string]uint32, s study.Subject, points []Point, c Comparison) (Tested, bool, error) {
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
	res, err := expansion.Compare(t, group(from), group(to), c.Options)
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

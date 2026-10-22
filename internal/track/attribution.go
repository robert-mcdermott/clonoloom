package track

// Attribution (clonoloom-spec/requirements.md, T5; roadmap.md, wave 3, slice 8), two measures of
// each of a subject's samples, and of each time point (its samples pooled), against its reference
// sample (an infusion product, a donor), under the definition, each with Wilson's interval for
// sampling the sample's cells (the limit of a bootstrap of them):
//   - the share in the reference's clonotypes: the sample's cells in clonotypes the reference
//     sample holds. Clonotypes of the reference its sample missed are not counted, and the
//     subject's own cells count where the reference holds their clonotypes;
//   - the share derived from the reference, where the subject has samples before the reference
//     event and the reference shares few clonotypes with them (a donor, not an infusion product
//     made from the subject's own blood): among the sample's cells in clonotypes found in the
//     reference or in the samples before, but not in both, the share in the reference's. Clonotypes
//     either side missed, or both hold, say nothing of where a cell came from, and are left out.

import (
	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

// MaxOverlap is the most of the reference's cells in clonotypes found before the reference event
// for the share derived from it to be told: beyond it, the reference and the subject's own
// repertoire cannot be told apart by clonotype.
const MaxOverlap = 0.25

// Attributed is a subject's attribution: its reference samples and their clonotypes, each later
// sample's and time point's measures, the share in the reference's clonotypes before the
// reference event (the samples before day 0, or at baseline, pooled), and the share of the
// reference's cells in clonotypes found before it (Overlap), where the subject has samples then.
// Derived says whether the share derived from the reference is told, and Why, if not.
type Attributed struct {
	Subject    string            `json:"subject"`
	Reference  []string          `json:"reference"`
	Label      string            `json:"label"`
	Clonotypes int               `json:"clonotypes"`
	Points     []AttributedValue `json:"points"`
	Samples    []AttributedValue `json:"samples"`
	Before     *Value            `json:"before,omitempty"`
	Overlap    *float64          `json:"overlap,omitempty"`
	Derived    bool              `json:"derived"`
	Why        string            `json:"why,omitempty"`
}

// AttributedValue is a time point's (or a sample's) measures: its share in the reference's
// clonotypes (Value), and the share derived from the reference (From: of its cells in clonotypes
// of one side only, those in the reference's), each with its interval.
type AttributedValue struct {
	Name  string   `json:"name,omitempty"`
	Point Point    `json:"point"`
	Days  *float64 `json:"days,omitempty"`
	Value Value    `json:"value"`
	From  *Value   `json:"from,omitempty"`
}

// Attribution is every subject with a reference sample attributed (Without: those without one),
// whether the shares have intervals, and what to keep in mind.
type Attribution struct {
	Subjects   []Attributed `json:"subjects"`
	Without    []string     `json:"without,omitempty"`
	Confidence float64      `json:"confidence"`
	Intervals  bool         `json:"intervals"`
	Caveat     string       `json:"caveat"`
}

// Attribute attributes each subject's samples of the design to its reference samples, in the
// clonotype table t whose samples are named by names.
func Attribute(t *clonotype.Table, names []string, design study.Design, confidence float64) (Attribution, error) {
	if confidence <= 0 || confidence >= 1 {
		confidence = 0.95
	}
	z := diversity.NormalQuantile(0.5 + float64(confidence/2))
	out := Attribution{Subjects: []Attributed{}, Confidence: confidence, Intervals: t.Unit != rep.Reads,
		Caveat: "The share in the reference's clonotypes counts each sample's cells in the clonotypes the reference sample holds: those of the reference its sample missed are not counted, and the subject's own cells count where the reference holds their clonotypes. An infusion product made from the subject's own blood holds the blood's largest clonotypes, so the blood is in them before the infusion too, and neither measure tells the infused cells apart from the subject's own. Where the reference shares few clonotypes with the samples before the event (a donor), the share derived from it counts only the clonotypes found on one side."}
	if t.Unit == rep.Reads {
		out.Caveat += " The counts are of reads: no interval is given."
	}
	index := indexOf(names)
	for _, s := range design.Subjects {
		points, samples, indices := Timeline(t, index, s)
		isRef, isBefore := map[uint32]bool{}, map[uint32]bool{}
		var refNames []string
		label := ""
		for k, sm := range samples {
			p := points[sm.Point]
			switch {
			case p.Reference != "":
				isRef[indices[k]] = true
				refNames = append(refNames, sm.Name)
				if label == "" {
					label = p.Reference
				}
			case before(p):
				isBefore[indices[k]] = true
			}
		}
		if len(refNames) == 0 {
			out.Without = append(out.Without, s.ID)
			continue
		}
		// Each sample's cells in the reference's clonotypes, and in clonotypes of one side only.
		in, fromRef, fromOwn := map[uint32]float64{}, map[uint32]float64{}, map[uint32]float64{}
		clonotypes := 0
		var refCells, refShared float64
		for i := range t.Clonotypes {
			idx, values := t.Abundances(i)
			held, seenBefore := false, false
			var cells float64
			for j, x := range idx {
				if isRef[x] && values[j] > 0 {
					held = true
					cells += values[j]
				}
				if isBefore[x] && values[j] > 0 {
					seenBefore = true
				}
			}
			if held {
				clonotypes++
				refCells += cells
				if seenBefore {
					refShared += cells
				}
			}
			for j, x := range idx {
				if held {
					in[x] += values[j]
				}
				if held && !seenBefore {
					fromRef[x] += values[j]
				}
				if seenBefore && !held {
					fromOwn[x] += values[j]
				}
			}
		}
		a := Attributed{Subject: s.ID, Reference: refNames, Label: label, Clonotypes: clonotypes, Points: []AttributedValue{}, Samples: []AttributedValue{}}
		if len(isBefore) > 0 && refCells > 0 {
			overlap := refShared / refCells
			a.Overlap = &overlap
			a.Derived = overlap <= MaxOverlap
			if !a.Derived {
				a.Why = "The reference holds clonotypes the samples before the event hold too, for most of its cells: its cells and the subject's own cannot be told apart by clonotype."
			}
		} else {
			a.Why = "No samples before the reference event, to tell the subject's own clonotypes from the reference's."
		}
		pointIn, pointFrom, pointOwn := make([]float64, len(points)), make([]float64, len(points)), make([]float64, len(points))
		var beforeIn, beforeTotal float64
		for k, sm := range samples {
			p := points[sm.Point]
			if p.Reference != "" {
				continue
			}
			i := indices[k]
			pointIn[sm.Point] += in[i]
			pointFrom[sm.Point] += fromRef[i]
			pointOwn[sm.Point] += fromOwn[i]
			v := AttributedValue{Name: sm.Name, Point: p, Days: p.Days, Value: valueOf(in[i], t.SampleTotals[i], out.Intervals, z)}
			if a.Derived && !before(p) {
				f := valueOf(fromRef[i], fromRef[i]+fromOwn[i], out.Intervals, z)
				v.From = &f
			}
			a.Samples = append(a.Samples, v)
			if before(p) {
				beforeIn += in[i]
				beforeTotal += t.SampleTotals[i]
			}
		}
		for k, p := range points {
			if p.Reference != "" {
				continue
			}
			v := AttributedValue{Point: p, Days: p.Days, Value: valueOf(pointIn[k], p.Total, out.Intervals, z)}
			if a.Derived && !before(p) {
				f := valueOf(pointFrom[k], pointFrom[k]+pointOwn[k], out.Intervals, z)
				v.From = &f
			}
			a.Points = append(a.Points, v)
		}
		if beforeTotal > 0 {
			v := valueOf(beforeIn, beforeTotal, out.Intervals, z)
			a.Before = &v
		}
		out.Subjects = append(out.Subjects, a)
	}
	return out, nil
}

// before says whether a time point comes before the reference event: baseline, or a negative day.
func before(p Point) bool {
	return p.Kind == study.Before || (p.Days != nil && *p.Days < 0)
}

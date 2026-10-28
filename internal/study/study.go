// Package study is a study's design as tracking needs it (clonoloom-spec/roadmap.md, wave 3,
// slice 3; requirements D10 and G3): each sample's subject; its time point, read as days from the
// subject's reference event (time.go); whether it is a reference sample of its subject (an
// infusion product, a donor, a baseline), from its annotation or else its tissue; each subject's
// samples in time order, with those taken at one time (replicates) together; and what is checked
// before anything is tracked, as findings with stable codes, as the validator gives them.
package study

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Sample is one sample's annotation, as the project holds it (its own, else its metadata's).
type Sample struct {
	Name      string `json:"name"`
	Subject   string `json:"subject,omitempty"`
	Time      string `json:"timepoint,omitempty"`
	Tissue    string `json:"tissue,omitempty"`
	Reference string `json:"reference,omitempty"`
}

// Placed is a subject's sample in its place: its time point, what it is if it is a reference
// sample (from the annotation's reference field, or its tissue), and its key, which names its time
// point across subjects ("reference" for a reference sample; Time.Key for the others).
type Placed struct {
	Name          string `json:"name"`
	Time          *Time  `json:"time,omitempty"`
	Reference     string `json:"reference,omitempty"`
	ReferenceFrom string `json:"referenceFrom,omitempty"` // annotation or tissue
	Key           string `json:"key"`
}

// Subject is a subject's samples: those with a time point, or reference samples, in time order
// (references without one first); groups of samples at one time (not references); and those left
// out of tracking for want of a time point.
type Subject struct {
	ID         string     `json:"id"`
	Samples    []Placed   `json:"samples"`
	References []string   `json:"references,omitempty"`
	Replicates [][]string `json:"replicates,omitempty"`
	Untimed    []string   `json:"untimed,omitempty"`
}

// Finding is something the checks found: a stable code, a severity (warning: left out of
// tracking; note: informs), how many samples or subjects it concerns of how many, examples, and
// what to do.
type Finding struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail"`
	Count    int      `json:"count"`
	Of       int      `json:"of,omitempty"`
	Examples []string `json:"examples,omitempty"`
	Remedy   string   `json:"remedy,omitempty"`
}

// Design is a study's subjects, the samples without a subject, the time span of the samples with
// days (for a timeline's axis), and the checks' findings, warnings first.
type Design struct {
	Subjects   []Subject `json:"subjects"`
	Unassigned []string  `json:"unassigned,omitempty"`
	FirstDay   *float64  `json:"firstDay,omitempty"`
	LastDay    *float64  `json:"lastDay,omitempty"`
	Findings   []Finding `json:"findings"`
}

// A tissue that makes a sample its subject's reference when the annotation does not say.
var referenceTissue = regexp.MustCompile(`(?i)^(infusion product|cell product|car-?t product|product|infusion|apheresis|leukapheresis|graft|donor|donor blood|donor pbmc)$`)

// A reference field that says no.
var notReference = regexp.MustCompile(`(?i)^(no|false|0|none|n)$`)

// Describe reads the samples' annotation into the study's design and checks it.
func Describe(samples []Sample) Design {
	var d Design
	bySubject := map[string]*Subject{}
	var ids []string
	unread := []string{}
	var bare, untimed []string
	timed := 0
	for _, s := range samples {
		id := strings.TrimSpace(s.Subject)
		if id == "" {
			d.Unassigned = append(d.Unassigned, s.Name)
			continue
		}
		sub := bySubject[id]
		if sub == nil {
			sub = &Subject{ID: id, Samples: []Placed{}}
			bySubject[id] = sub
			ids = append(ids, id)
		}
		p := Placed{Name: s.Name}
		if r := strings.TrimSpace(s.Reference); r != "" && !notReference.MatchString(r) {
			p.Reference, p.ReferenceFrom = r, "annotation"
		} else if r == "" && referenceTissue.MatchString(strings.TrimSpace(s.Tissue)) {
			p.Reference, p.ReferenceFrom = strings.TrimSpace(s.Tissue), "tissue"
		}
		given := strings.TrimSpace(s.Time) != ""
		if given {
			if t, ok := ParseTime(s.Time); ok {
				p.Time = &t
				timed++
				if t.Bare {
					bare = append(bare, s.Name+" ("+t.Text+")")
				}
			} else {
				unread = append(unread, s.Name+" ("+strings.TrimSpace(s.Time)+")")
			}
		}
		if p.Reference != "" {
			sub.References = append(sub.References, s.Name)
			p.Key = "reference"
		} else if p.Time != nil {
			p.Key = p.Time.Key()
		}
		if p.Time == nil && p.Reference == "" {
			sub.Untimed = append(sub.Untimed, s.Name)
			if !given {
				untimed = append(untimed, id+": "+s.Name) // one not understood is said so apart
			}
			continue
		}
		sub.Samples = append(sub.Samples, p)
	}
	sort.SliceStable(ids, func(i, j int) bool { return naturalLess(ids[i], ids[j]) })
	var single, noReference, mixed, replicated []string
	replicateGroups := 0
	for _, id := range ids {
		sub := bySubject[id]
		sort.SliceStable(sub.Samples, func(i, j int) bool {
			a, b := sub.Samples[i], sub.Samples[j]
			if (a.Time == nil) != (b.Time == nil) {
				return a.Time == nil // a reference without a time point first
			}
			if a.Time != nil && !Same(*a.Time, *b.Time) {
				return earlier(*a.Time, *b.Time)
			}
			return naturalLess(a.Name, b.Name)
		})
		// Replicates: samples at one time, not references.
		kinds := map[string]bool{}
		for i := 0; i < len(sub.Samples); {
			j := i + 1
			if sub.Samples[i].Time != nil {
				kinds[sub.Samples[i].Time.Kind] = true
				for j < len(sub.Samples) && sub.Samples[j].Time != nil && Same(*sub.Samples[i].Time, *sub.Samples[j].Time) {
					j++
				}
			}
			var group []string
			for _, p := range sub.Samples[i:j] {
				if p.Reference == "" {
					group = append(group, p.Name)
				}
			}
			if len(group) > 1 {
				sub.Replicates = append(sub.Replicates, group)
				replicateGroups++
				replicated = append(replicated, id+": "+strings.Join(group, ", "))
			}
			i = j
		}
		// The distinct times of its samples (not references), and the days' span.
		times := 0
		var last *Time
		for _, p := range sub.Samples {
			if p.Reference == "" {
				if last == nil || !Same(*last, *p.Time) {
					times++
				}
				last = p.Time
				if p.Time.Days != nil {
					v := *p.Time.Days
					if d.FirstDay == nil || v < *d.FirstDay {
						d.FirstDay = &v
					}
					if d.LastDay == nil || v > *d.LastDay {
						d.LastDay = &v
					}
				}
			}
		}
		if times < 2 {
			single = append(single, id)
		}
		if len(sub.References) == 0 {
			noReference = append(noReference, id)
		}
		if (kinds[Day] && kinds[Visit]) || (kinds[After] && (kinds[Day] || kinds[Visit])) {
			mixed = append(mixed, id)
		}
		d.Subjects = append(d.Subjects, *sub)
	}
	n, subjects := len(samples), len(ids)
	add := func(f Finding) {
		if f.Count > 0 {
			if len(f.Examples) > 5 {
				f.Examples = f.Examples[:5]
			}
			d.Findings = append(d.Findings, f)
		}
	}
	add(Finding{Code: "study-no-subject", Severity: "warning", Title: "Samples without a subject", Count: len(d.Unassigned), Of: n, Examples: d.Unassigned,
		Detail: "A sample is tracked, tested and attributed within its subject: these are left out.", Remedy: "Give each sample its subject in the Data view's samples table, or fill it from the files' metadata or names."})
	add(Finding{Code: "study-time-unread", Severity: "warning", Title: "Time points not understood", Count: len(unread), Of: n, Examples: unread,
		Detail: "These samples are left out of tracking. ClonoLoom reads days, weeks, months, years and hours (day 28, d28, D+28, 4 weeks, wk4, month 3, 6h), a number of days, baseline or pre, post, and visits (T1, visit 2).", Remedy: "Write each time point in one of these forms."})
	add(Finding{Code: "study-no-time", Severity: "warning", Title: "Samples without a time point", Count: len(untimed), Of: n, Examples: untimed,
		Detail: "These samples are left out of tracking. A reference sample (an infusion product, a donor) needs none.", Remedy: "Give each sample its time point, as days from the subject's reference event."})
	add(Finding{Code: "study-bare-number", Severity: "note", Title: "Time points read as days", Count: len(bare), Of: timed, Examples: bare,
		Detail: "A number without a unit is read as days.", Remedy: "Write the unit (week 4, month 3) where these are not days."})
	add(Finding{Code: "study-replicates", Severity: "note", Title: "Samples taken at one time", Count: replicateGroups, Examples: replicated,
		Detail: "Two or more samples share a subject and time. Their technical or biological relationship must be established from the study design; they do not add independent subjects. Use only justified stable replicates to fit expansion variance.", Remedy: ""})
	add(Finding{Code: "study-one-sample", Severity: "note", Title: "Subjects with one time point", Count: len(single), Of: subjects, Examples: single,
		Detail: "A subject needs samples at two times or more for its clonotypes to be followed or tested between them.", Remedy: ""})
	add(Finding{Code: "study-no-reference", Severity: "note", Title: "Subjects without a reference sample", Count: len(noReference), Of: subjects, Examples: noReference,
		Detail: "A reference sample (an infusion product, a donor, a baseline) is what attribution measures each sample against, and whose clonotypes tracking can follow; without one, each sample's largest clonotypes are followed.", Remedy: "Mark a subject's reference sample in the Data view's Reference column (its tissue, such as infusion product or donor, marks it too)."})
	add(Finding{Code: "study-mixed-times", Severity: "note", Title: "Time points that cannot be ordered against each other", Count: len(mixed), Of: subjects, Examples: mixed,
		Detail: "These subjects have days with visits, or a time after the event without its day: visits are placed after the days, and after last.", Remedy: "Give every sample its day where it is known."})
	sort.SliceStable(d.Findings, func(i, j int) bool {
		return d.Findings[i].Severity == "warning" && d.Findings[j].Severity != "warning"
	})
	if d.Subjects == nil {
		d.Subjects = []Subject{}
	}
	if d.Findings == nil {
		d.Findings = []Finding{}
	}
	return d
}

// naturalLess orders "P2" before "P10": digit runs compared as numbers.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na, _ := strconv.Atoi(a[si:i])
			nb, _ := strconv.Atoi(b[sj:j])
			if na != nb {
				return na < nb
			}
			continue
		}
		if ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	return len(a)-i < len(b)-j
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

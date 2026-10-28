// Package convergence measures observed cross-subject sequence-group recurrence
// and conditional HLA association. Inspiration: Glanville et al. Nature (2017),
// doi:10.1038/nature22976. Fisher (1922), doi:10.2307/2340521; Benjamini and
// Yekutieli (2001), doi:10.1214/aos/1013699998; Wilson (1927), JASA 22:209–212,
// doi:10.1080/01621459.1927.10502953. ClonoLoom's choices: research §5.36.
package convergence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"clonoloom/internal/conga"
	"clonoloom/internal/diversity"
	"clonoloom/internal/dmath"
	"clonoloom/internal/motifs"
	"clonoloom/internal/track"
)

const Version = 1
const Citation = "Glanville J et al. Identifying specificity groups in the T cell receptor repertoire. Nature. 2017;547:94–98. doi:10.1038/nature22976. Fisher RA. J R Stat Soc. 1922;85:87–94. doi:10.2307/2340521. Benjamini Y, Yekutieli D. Ann Stat. 2001;29:1165–1188. doi:10.1214/aos/1013699998. Wilson EB. J Am Stat Assoc. 1927;22:209–212. doi:10.1080/01621459.1927.10502953. IPD-IMGT/HLA nomenclature: https://hla.alleles.org/pages/nomenclature/naming_alleles/"
const Description = "Observed cross-subject recurrence and HLA association of explicitly selected saved sequence signatures. Each named human subject contributes one binary observation, unioned across declared samples; sequences, cells, repeated samples and abundance do not inflate replication. Recurrence is descriptive, not evidence of convergent recombination or shared specificity. HLA tests use a separate complete two-field diploid typed-subject universe per locus, including subjects without the group. Missing, incomplete, ambiguous and expression-suffix typing is unknown, not non-carriage. One-sided hypergeometric enrichment with BY over every declared group × allele slot; untestable slots remain. Carrier and non-carrier frequencies have marginal 95% Wilson intervals, not BY-adjusted or simultaneous intervals. Independent exchangeable subjects and comparable ascertainment are assumptions; population structure, relatedness, linkage disequilibrium, uneven repertoire depth, group selection and HLA-dependent sampling can confound the result. No adjustment for ancestry, haplotype phase, class-II heterodimers, restriction or antigen specificity is inferred. Select groups, alleles and subjects before inspecting association outcomes; validate hypotheses independently."

// Two-field normalization accepts syntactically resolved 2–4 field names, without
// asserting registration in an HLA release. Never turn G/P groups or expression
// suffixes into ordinary protein alleles by stripping their suffix.
var allelePattern = regexp.MustCompile(`^(?:HLA-)?(A|B|C|DRB1|DQA1|DQB1|DPA1|DPB1)\*([0-9]{2,3}):([0-9]{2,3})(?::[0-9]{2,3}){0,2}$`)

func Allele(raw string) (string, string, error) {
	m := allelePattern.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", "", fmt.Errorf("%q: use an unambiguous 2–4 field HLA allele without group/expression suffix", raw)
	}
	return "HLA-" + m[1] + "*" + m[2] + ":" + m[3], m[1], nil
}

type Subject struct {
	ID      string              `json:"id"`
	Samples []string            `json:"samples"`
	Typing  map[string][]string `json:"typing"`
}
type Selection struct {
	Species  string    `json:"species"`
	Samples  []string  `json:"samples"`
	Subjects []Subject `json:"subjects"`
	Groups   []string  `json:"groups"`
	Alleles  []string  `json:"alleles"`
}
type Input struct {
	Selection Selection       `json:"selection"`
	Records   []motifs.Record `json:"records"`
	Groups    []Signature     `json:"groups"`
}
type Signature struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Members []string `json:"members"`
}
type Typing struct {
	Locus   string   `json:"locus"`
	Raw     []string `json:"raw"`
	Alleles []string `json:"alleles"`
	Reason  string   `json:"reason"`
}
type Observation struct {
	Subject   string   `json:"subject"`
	Samples   []string `json:"samples"`
	Receptors int      `json:"receptors"`
	Typing    []Typing `json:"typing"`
	Groups    []string `json:"groups"`
	Reason    string   `json:"reason"`
}
type Recurrence struct {
	ID               string           `json:"id"`
	Label            string           `json:"label"`
	Subjects         []string         `json:"subjects"`
	ObservedSubjects int              `json:"observedSubjects"`
	Receptors        []Receptor       `json:"receptors"`
	Frequency        diversity.Number `json:"frequency"`
}
type Receptor struct {
	ID       string   `json:"id"`
	V        string   `json:"v"`
	Junction string   `json:"junction"`
	Subjects []string `json:"subjects"`
}
type Frequency struct {
	Matches  int              `json:"matches"`
	Total    int              `json:"total"`
	Estimate diversity.Number `json:"estimate"`
	Lower    diversity.Number `json:"lower"`
	Upper    diversity.Number `json:"upper"`
}
type Association struct {
	Group         string           `json:"group"`
	Allele        string           `json:"allele"`
	Locus         string           `json:"locus"`
	N             int              `json:"n"`
	GroupSubjects int              `json:"groupSubjects"`
	Carriers      int              `json:"carriers"`
	Shared        int              `json:"shared"`
	Excluded      int              `json:"excluded"`
	Carrier       Frequency        `json:"carrier"`
	Noncarrier    Frequency        `json:"noncarrier"`
	Difference    diversity.Number `json:"difference"`
	P             diversity.Number `json:"p"`
	LogP          diversity.Number `json:"logP"`
	Q             diversity.Number `json:"q"`
	LogQ          diversity.Number `json:"logQ"`
	Reason        string           `json:"reason"`
}
type Result struct {
	Description      string        `json:"description"`
	Selection        Selection     `json:"selection"`
	Observations     []Observation `json:"observations"`
	Recurrence       []Recurrence  `json:"recurrence"`
	Associations     []Association `json:"associations"`
	Family           int           `json:"family"`
	Tested           int           `json:"tested"`
	ObservedSubjects int           `json:"observedSubjects"`
}

func missing() diversity.Number { return diversity.Number(math.NaN()) }
func frequency(k, n int) Frequency {
	f := Frequency{Matches: k, Total: n, Estimate: missing(), Lower: missing(), Upper: missing()}
	if n > 0 {
		f.Estimate = diversity.Number(float64(k) / float64(n))
		lo, hi := track.Wilson(float64(k), float64(n), 1.959963984540054)
		f.Lower, f.Upper = diversity.Number(lo), diversity.Number(hi)
	}
	return f
}
func sorted(set map[string]bool) []string {
	out := []string{}
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
func has(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
func Normalize(s Selection) (Selection, error) {
	if s.Species != "human" {
		return s, errors.New("explicit human species is required for HLA analysis")
	}
	if len(s.Subjects) < 1 || len(s.Subjects) > 200 || len(s.Samples) < 1 || len(s.Samples) > 256 || len(s.Groups) < 1 || len(s.Groups) > 128 || len(s.Alleles) > 64 {
		return s, errors.New("choose 1–200 subjects, 1–256 samples, 1–128 groups and at most 64 alleles")
	}
	// No alleles is a descriptive recurrence-only analysis, not a failed HLA test.
	unique := func(ss []string) bool {
		seen := map[string]bool{}
		for _, v := range ss {
			if strings.TrimSpace(v) == "" || len(v) > 256 || seen[v] {
				return false
			}
			seen[v] = true
		}
		return true
	}
	if !unique(s.Samples) || !unique(s.Groups) {
		return s, errors.New("sample and group selections must be distinct")
	}
	sampleSet, assigned, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, v := range s.Samples {
		sampleSet[v] = true
	}
	for _, sub := range s.Subjects {
		if strings.TrimSpace(sub.ID) != sub.ID || sub.ID == "" || len(sub.ID) > 256 || ids[sub.ID] || len(sub.Samples) == 0 || !unique(sub.Samples) || len(sub.Typing) > 8 {
			return s, errors.New("subjects need unique nonempty IDs and assigned samples")
		}
		ids[sub.ID] = true
		for _, v := range sub.Samples {
			if !sampleSet[v] || assigned[v] {
				return s, errors.New("every selected sample needs exactly one named subject")
			}
			assigned[v] = true
		}
		for locus, calls := range sub.Typing {
			if !has([]string{"A", "B", "C", "DRB1", "DQA1", "DQB1", "DPA1", "DPB1"}, locus) || len(calls) > 8 {
				return s, errors.New("unsupported typing locus or too many calls")
			}
			for _, call := range calls {
				if len(call) > 100 {
					return s, errors.New("typing call is too long")
				}
			}
		}
	}
	if len(assigned) != len(sampleSet) {
		return s, errors.New("every selected sample needs exactly one named subject")
	}
	s.Alleles = append([]string{}, s.Alleles...)
	for i, a := range s.Alleles {
		normalized, _, err := Allele(a)
		if err != nil {
			return s, err
		}
		s.Alleles[i] = normalized
	}
	if !unique(s.Alleles) {
		return s, errors.New("alleles must be distinct after two-field normalization")
	}
	// Preserve declared group/allele order. Canonicalize sample and subject ordering.
	s.Samples = append([]string{}, s.Samples...)
	sort.Strings(s.Samples)
	s.Subjects = append([]Subject{}, s.Subjects...)
	sort.Slice(s.Subjects, func(i, j int) bool { return s.Subjects[i].ID < s.Subjects[j].ID })
	for i := range s.Subjects {
		s.Subjects[i].Samples = append([]string{}, s.Subjects[i].Samples...)
		sort.Strings(s.Subjects[i].Samples)
		// Absent and explicitly empty loci are the same unknown typing. Preserve
		// every nonempty raw call, but canonicalize this representation so the
		// visible text controls and structured clients identify the same result.
		typing := map[string][]string{}
		for locus, calls := range s.Subjects[i].Typing {
			if len(calls) > 0 {
				typing[locus] = append([]string{}, calls...)
			}
		}
		s.Subjects[i].Typing = typing
	}
	return s, nil
}
func Compute(ctx context.Context, in Input) (*Result, error) {
	s, err := Normalize(in.Selection)
	if err != nil {
		return nil, err
	}
	if len(in.Records) > 4000 || len(in.Groups) != len(s.Groups) {
		return nil, errors.New("invalid captured convergence evidence")
	}
	r := &Result{Description: Description, Selection: s, Observations: []Observation{}, Recurrence: []Recurrence{}, Associations: []Association{}, Family: len(s.Groups) * len(s.Alleles)}
	sampleSubject := map[string]string{}
	for _, sub := range s.Subjects {
		for _, sample := range sub.Samples {
			sampleSubject[sample] = sub.ID
		}
	}
	records := map[string]motifs.Record{}
	members := map[string]map[string]bool{}
	observed := map[string]map[string]bool{}
	for _, rec := range in.Records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if rec.ID != motifs.ID(rec.V, rec.Junction) || !motifs.Valid(rec.V, rec.Junction) || records[rec.ID].ID != "" {
			return nil, errors.New("invalid or duplicate captured receptor")
		}
		records[rec.ID] = rec
		members[rec.ID] = map[string]bool{}
		for _, sample := range rec.Samples {
			if sub := sampleSubject[sample]; sub != "" {
				members[rec.ID][sub] = true
				if observed[sub] == nil {
					observed[sub] = map[string]bool{}
				}
				observed[sub][rec.ID] = true
			}
		}
	}
	groupSubjects := map[string]map[string]bool{}
	for i, g := range in.Groups {
		if g.ID != s.Groups[i] || g.Label == "" {
			return nil, errors.New("captured group selection differs")
		}
		rr := Recurrence{ID: g.ID, Label: g.Label, Subjects: []string{}, Receptors: []Receptor{}, Frequency: missing()}
		gs := map[string]bool{}
		seen := map[string]bool{}
		for _, id := range g.Members {
			rec, ok := records[id]
			if !ok || seen[id] {
				return nil, errors.New("invalid captured group membership")
			}
			seen[id] = true
			subjects := sorted(members[id])
			if len(subjects) == 0 {
				continue
			}
			for _, sub := range subjects {
				gs[sub] = true
			}
			rr.Receptors = append(rr.Receptors, Receptor{ID: id, V: rec.V, Junction: rec.Junction, Subjects: subjects})
		}
		sort.Slice(rr.Receptors, func(i, j int) bool { return rr.Receptors[i].ID < rr.Receptors[j].ID })
		rr.Subjects = sorted(gs)
		groupSubjects[g.ID] = gs
		r.Recurrence = append(r.Recurrence, rr)
	}
	loci := map[string]bool{}
	for _, a := range s.Alleles {
		_, locus, _ := Allele(a)
		loci[locus] = true
	}
	for _, sub := range s.Subjects {
		ob := Observation{Subject: sub.ID, Samples: sub.Samples, Receptors: len(observed[sub.ID]), Typing: []Typing{}, Groups: []string{}}
		if ob.Receptors == 0 {
			ob.Reason = "No eligible receptor evidence in selected samples; group absence is not established"
		} else {
			r.ObservedSubjects++
		}
		for _, g := range s.Groups {
			if groupSubjects[g][sub.ID] {
				ob.Groups = append(ob.Groups, g)
			}
		}
		for _, locus := range sorted(loci) {
			raw := sub.Typing[locus]
			t := Typing{Locus: locus, Raw: append([]string{}, raw...), Alleles: []string{}}
			if len(raw) != 2 {
				t.Reason = "Exactly two explicit allele copies are required; repeat a homozygous call"
			} else {
				for _, call := range raw {
					a, l, e := Allele(call)
					if e != nil || l != locus {
						t.Reason = "Unresolved, ambiguous, expression-suffix or wrong-locus typing"
						break
					}
					t.Alleles = append(t.Alleles, a)
				}
			}
			if t.Reason != "" {
				t.Alleles = []string{}
			} else {
				sort.Strings(t.Alleles)
			}
			ob.Typing = append(ob.Typing, t)
		}
		r.Observations = append(r.Observations, ob)
	}
	for i := range r.Recurrence {
		rr := &r.Recurrence[i]
		rr.ObservedSubjects = r.ObservedSubjects
		if r.ObservedSubjects > 0 {
			rr.Frequency = diversity.Number(float64(len(rr.Subjects)) / float64(r.ObservedSubjects))
		}
	}
	for _, g := range s.Groups {
		for _, a := range s.Alleles {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			_, locus, _ := Allele(a)
			x := Association{Group: g, Allele: a, Locus: locus, Difference: missing(), P: missing(), LogP: missing(), Q: missing(), LogQ: missing()}
			for _, ob := range r.Observations {
				if ob.Reason != "" {
					continue
				}
				var typing *Typing
				for i := range ob.Typing {
					if ob.Typing[i].Locus == locus {
						typing = &ob.Typing[i]
						break
					}
				}
				if typing == nil || typing.Reason != "" {
					continue
				}
				x.N++
				positive := groupSubjects[g][ob.Subject]
				carrier := has(typing.Alleles, a)
				if positive {
					x.GroupSubjects++
				}
				if carrier {
					x.Carriers++
				}
				if carrier && positive {
					x.Shared++
				}
			}
			x.Excluded = len(s.Subjects) - x.N
			x.Carrier = frequency(x.Shared, x.Carriers)
			x.Noncarrier = frequency(x.GroupSubjects-x.Shared, x.N-x.Carriers)
			if x.Carriers > 0 && x.Carriers < x.N {
				x.Difference = diversity.Number(float64(x.Carrier.Estimate) - float64(x.Noncarrier.Estimate))
			}
			if x.N < 4 {
				x.Reason = "Fewer than four evaluable typed subjects"
			} else if x.Carriers == 0 || x.Carriers == x.N {
				x.Reason = "Allele carriage is constant in the typed universe"
			} else if x.GroupSubjects == 0 || x.GroupSubjects == x.N {
				x.Reason = "Group membership is constant in the typed universe"
			} else {
				lp := conga.LogTail(x.N, x.Carriers, x.GroupSubjects, x.Shared)
				x.LogP = diversity.Number(lp)
				x.P = diversity.Number(dmath.Exp(lp))
				r.Tested++
			}
			r.Associations = append(r.Associations, x)
		}
	}
	// Log-space BY; all declared slots stay, including unavailable p=1 slots.
	order := make([]int, r.Family)
	for i := range order {
		order[i] = i
	}
	lp := func(i int) float64 {
		v := float64(r.Associations[i].LogP)
		if math.IsNaN(v) {
			return 0
		}
		return v
	}
	sort.SliceStable(order, func(i, j int) bool { return lp(order[i]) < lp(order[j]) })
	h := 0.0
	for i := 1; i <= r.Family; i++ {
		h += 1 / float64(i)
	}
	next := 0.0
	for rank := len(order) - 1; rank >= 0; rank-- {
		i := order[rank]
		x := &r.Associations[i]
		next = math.Min(next, lp(i)+dmath.Log(float64(r.Family))+dmath.Log(h)-dmath.Log(float64(rank+1)))
		if x.Reason == "" {
			x.LogQ = diversity.Number(next)
			x.Q = diversity.Number(dmath.Exp(next))
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

package bcell

// Observed constant-region usage and unordered family co-occurrence. AIRR 2.0 c_call;
// IMGT nomenclature (Lefranc 2014, doi:10.3389/fimmu.2014.00022).
// Horns et al. 2016, doi:10.7554/eLife.16578, and Hoehn et al. 2022,
// doi:10.1371/journal.pcbi.1009885 provide switching context, not the counting policy.
// No transition, ancestral trait or switching-rate inference is performed here.
import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

const IsotypeVersion = 1
const IsotypeDescription = "Observed heavy-chain constant-region usage within a saved, thresholded family result. Each assigned observation contributes once: a sample-scoped cell in single-cell data, a heavy-chain source row in bulk data. All retained heavy C calls must be supported and agree at the reported resolution; allele ties are collapsed, same-class subclass ties retain class only, and missing, unsupported or conflicting calls remain unresolved. Light chains do not define isotype. Observation shares include unresolved assigned observations in the denominator; parent-excluded observations stay outside it. Abundance belongs to the observation, never to each source chain; a missing count makes its bucket and the group's abundance denominator unavailable. Family pairs count unordered co-occurrence of resolved constant genes once per family. They do not count switching events or establish direction, timing, ancestral isotypes, rates or significance. IgM/IgD co-occurrence alone is not switching evidence. Sample annotations are pinned; no timepoints, tissues or subjects are pooled as biological replicates."
const unresolvedIsotype = "Unresolved"

var isotypeClasses = map[string]string{
	"IGHM": "IgM", "IGHD": "IgD", "IGHE": "IgE", "IGHG": "IgG", "IGHG1": "IgG", "IGHG2": "IgG", "IGHG3": "IgG", "IGHG4": "IgG", "IGHG2A": "IgG", "IGHG2B": "IgG", "IGHG2C": "IgG", "IGHA": "IgA", "IGHA1": "IgA", "IGHA2": "IgA",
}

type IsotypeSource struct {
	FamilyRow
	Call string `json:"call"`
}
type IsotypeObservation struct {
	ID        string          `json:"id"`
	Family    string          `json:"family"`
	Sample    string          `json:"sample"`
	Subject   string          `json:"subject"`
	Scope     string          `json:"scope"`
	Cell      string          `json:"cell,omitempty"`
	Timepoint string          `json:"timepoint"`
	Tissue    string          `json:"tissue"`
	Class     string          `json:"class"`
	Gene      string          `json:"gene"`
	Issue     string          `json:"issue,omitempty"`
	Excluded  string          `json:"excluded,omitempty"`
	Abundance *float64        `json:"abundance"`
	Rows      []IsotypeSource `json:"rows"`
}
type IsotypeUsage struct {
	Family                string   `json:"family"` // Empty: sample-wide, across assigned families.
	Sample                string   `json:"sample"`
	Timepoint             string   `json:"timepoint"`
	Tissue                string   `json:"tissue"`
	Level                 string   `json:"level"` // class or gene (subclass where supplied).
	State                 string   `json:"state"`
	Observations          int      `json:"observations"`
	Total                 int      `json:"total"`
	Share                 float64  `json:"share"`
	Abundance             *float64 `json:"abundance"`
	TotalAbundance        *float64 `json:"totalAbundance"`
	AbundanceShare        *float64 `json:"abundanceShare"`
	MissingAbundance      int      `json:"missingAbundance"`
	TotalMissingAbundance int      `json:"totalMissingAbundance"`
}
type IsotypeFamily struct {
	ID            string   `json:"id"`
	Scope         string   `json:"scope"`
	Observations  int      `json:"observations"`
	ResolvedClass int      `json:"resolvedClass"`
	ResolvedGene  int      `json:"resolvedGene"`
	Samples       []string `json:"samples"`
	Classes       []string `json:"classes"`
	Genes         []string `json:"genes"`
	Evidence      string   `json:"evidence"`
}
type IsotypePair struct {
	Left              string `json:"left"`
	Right             string `json:"right"`
	Families          int    `json:"families"`
	TotalFamilies     int    `json:"totalFamilies"`
	EvaluableFamilies int    `json:"evaluableFamilies"` // At least one resolved gene, not proof of full coverage.
}
type IsotypeResult struct {
	Observations  []IsotypeObservation `json:"observations"`
	Usage         []IsotypeUsage       `json:"usage"`
	Families      []IsotypeFamily      `json:"families"`
	Pairs         []IsotypePair        `json:"pairs"`
	Assigned      int                  `json:"assigned"`
	ResolvedClass int                  `json:"resolvedClass"`
	ResolvedGene  int                  `json:"resolvedGene"`
	Excluded      map[string]int       `json:"excluded"`
	Issues        map[string]int       `json:"issues"`
	Unit          string               `json:"unit"`
	AbundanceUnit string               `json:"abundanceUnit"`
	SingleCell    bool                 `json:"singleCell"`
	Description   string               `json:"description"`
}

// The shared gene parser is intentionally permissive about allele suffixes. Here
// a malformed suffix must not conceal another candidate (IGHM*01/IGHD*01).
// Accept numeric alleles and an optional finite annotation-tool score only.
func supportedIsotypeSuffix(raw string) bool {
	_, suffix, hasAllele := strings.Cut(raw, "*")
	if !hasAllele {
		return true
	}
	allele, score, hasScore := strings.Cut(suffix, "(")
	if allele == "" {
		return false
	}
	for _, c := range allele {
		if c < '0' || c > '9' {
			return false
		}
	}
	if hasScore {
		if !strings.HasSuffix(score, ")") {
			return false
		}
		value, err := strconv.ParseFloat(strings.TrimSuffix(score, ")"), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

// Union all heavy-row candidate calls. Never choose the first tie or borrow a
// resolved call to hide a missing/unsupported call on another retained heavy row.
func observedIsotype(calls []string) (class, gene, issue string) {
	class, gene = unresolvedIsotype, unresolvedIsotype
	classes, names := map[string]bool{}, map[string]bool{}
	missing, unsupported := len(calls) == 0, false
	for _, raw := range calls {
		if len(raw) > 4096 {
			unsupported = true
			continue
		}
		c := genes.ParseCall(raw)
		if len(c.Names) == 0 && c.Unreadable == 0 {
			missing = true
		}
		if c.Unreadable != 0 {
			unsupported = true
		}
		for _, n := range c.Names {
			label, ok := isotypeClasses[n.Gene]
			if !ok || n.Locus != "IGH" || n.Segment != genes.C || !supportedIsotypeSuffix(n.Raw) {
				unsupported = true
				continue
			}
			classes[label] = true
			names[n.Gene] = true
		}
	}
	if unsupported {
		return class, gene, "unsupported heavy constant call"
	}
	if missing {
		return class, gene, "missing heavy constant call"
	}
	if len(classes) != 1 {
		return class, gene, "conflicting or ambiguous classes"
	}
	for c := range classes {
		class = c
	}
	if len(names) != 1 {
		return class, gene, "conflicting or ambiguous subclasses"
	}
	for g := range names {
		gene = g
	}
	if gene == "IGHG" || gene == "IGHA" {
		return class, unresolvedIsotype, "class-only constant call"
	}
	return class, gene, ""
}
func sortedIsotypeKeys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type isotypeAmount struct {
	n, missing int
	sum        float64
}

func (a *isotypeAmount) add(v *float64) error {
	a.n++
	if v == nil {
		a.missing++
		return nil
	}
	if *v < 0 || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return errors.New("invalid saved observation abundance")
	}
	a.sum += *v
	if math.IsInf(a.sum, 0) {
		return errors.New("isotype abundance sum exceeds finite range")
	}
	return nil
}
func (a isotypeAmount) value() *float64 {
	if a.missing != 0 {
		return nil
	}
	v := a.sum
	return &v
}

func Isotypes(ctx context.Context, t *rep.Table, sources []FamilySource, parent FamilyResult, annotations []study.Sample) (IsotypeResult, error) {
	out := IsotypeResult{Observations: []IsotypeObservation{}, Usage: []IsotypeUsage{}, Families: []IsotypeFamily{}, Pairs: []IsotypePair{}, Excluded: map[string]int{}, Issues: map[string]int{}, Unit: "heavy-chain rows", AbundanceUnit: parent.Unit, SingleCell: parent.SingleCell, Description: IsotypeDescription}
	if parent.SingleCell {
		out.Unit = "cells"
	}
	if parent.Options.Threshold == nil {
		return out, errors.New("isotypes need a thresholded family result")
	}
	indices := map[string]int{}
	for i := 0; i < t.Len(); i++ {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return out, err
			}
		}
		if int(t.Source[i]) >= len(sources) {
			return out, errors.New("source index outside dataset")
		}
		id := familyID("bcr-row-", []any{sources[t.Source[i]], t.Line[i], t.IDs.Get(t.SequenceID[i])})
		if _, ok := indices[id]; ok {
			return out, errors.New("duplicate source row identity")
		}
		indices[id] = i
	}
	ann := map[string]study.Sample{}
	for _, a := range annotations {
		if _, ok := ann[a.Name]; ok {
			return out, errors.New("duplicate sample annotation")
		}
		ann[a.Name] = a
	}
	familyScopes := map[string]string{}
	for _, f := range parent.Families {
		if _, ok := familyScopes[f.ID]; ok || f.ID == "" {
			return out, errors.New("invalid saved family identity")
		}
		familyScopes[f.ID] = f.Scope
	}
	seenRows, seenObs := map[string]bool{}, map[string]bool{}
	observations := append([]FamilyObservation{}, parent.Observations...)
	sort.Slice(observations, func(i, j int) bool { return observations[i].ID < observations[j].ID })
	for _, o := range observations {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if seenObs[o.ID] || o.ID == "" || len(o.Rows) == 0 {
			return out, errors.New("invalid saved observation identity")
		}
		seenObs[o.ID] = true
		a := ann[o.Sample]
		row := IsotypeObservation{ID: o.ID, Family: o.Family, Sample: o.Sample, Subject: o.Subject, Scope: o.Scope, Cell: o.Cell, Timepoint: a.Time, Tissue: a.Tissue, Class: unresolvedIsotype, Gene: unresolvedIsotype, Abundance: o.Abundance, Excluded: o.Excluded, Rows: []IsotypeSource{}}
		var calls []string
		for _, r := range o.Rows {
			i, ok := indices[r.ID]
			if !ok || seenRows[r.ID] || r.Source != sources[t.Source[i]] || r.Line != t.Line[i] || r.SequenceID != t.IDs.Get(t.SequenceID[i]) || r.Locus != t.LocusName(i) || o.Sample != t.Samples[t.Sample[i]].Name {
				return out, errors.New("saved family rows do not match the open dataset")
			}
			if parent.SingleCell && (len(t.Cell) == 0 || o.Cell != t.Dict.Get(t.Cell[i])) {
				return out, errors.New("saved cell identity differs from dataset")
			}
			seenRows[r.ID] = true
			raw := t.Dict.Get(t.C[i])
			row.Rows = append(row.Rows, IsotypeSource{r, raw})
			if r.Locus == "IGH" {
				calls = append(calls, raw)
			}
		}
		sort.Slice(row.Rows, func(i, j int) bool { return row.Rows[i].ID < row.Rows[j].ID })
		if row.Excluded == "" {
			scope, ok := familyScopes[o.Family]
			if !ok || scope != o.Scope {
				return out, errors.New("saved observation has no matching family")
			}
			row.Class, row.Gene, row.Issue = observedIsotype(calls)
			out.Assigned++
			if row.Class != unresolvedIsotype {
				out.ResolvedClass++
			}
			if row.Gene != unresolvedIsotype {
				out.ResolvedGene++
			}
			if row.Issue != "" {
				out.Issues[row.Issue]++
			}
		} else {
			out.Excluded[row.Excluded]++
		}
		out.Observations = append(out.Observations, row)
	}
	type groupKey struct{ family, sample string }
	type bucketKey struct {
		groupKey
		level, state string
	}
	totals := map[groupKey]*isotypeAmount{}
	buckets := map[bucketKey]*isotypeAmount{}
	families := map[string]*IsotypeFamily{}
	fsamples, fclasses, fgenes := map[string]map[string]bool{}, map[string]map[string]bool{}, map[string]map[string]bool{}
	for _, f := range parent.Families {
		families[f.ID] = &IsotypeFamily{ID: f.ID, Scope: f.Scope}
		fsamples[f.ID] = map[string]bool{}
		fclasses[f.ID] = map[string]bool{}
		fgenes[f.ID] = map[string]bool{}
	}
	for _, o := range out.Observations {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if o.Excluded != "" {
			continue
		}
		f := families[o.Family]
		f.Observations++
		fsamples[o.Family][o.Sample] = true
		if o.Class != unresolvedIsotype {
			f.ResolvedClass++
			fclasses[o.Family][o.Class] = true
		}
		if o.Gene != unresolvedIsotype {
			f.ResolvedGene++
			fgenes[o.Family][o.Gene] = true
		}
		for _, family := range []string{"", o.Family} {
			g := groupKey{family, o.Sample}
			if totals[g] == nil {
				totals[g] = &isotypeAmount{}
			}
			if err := totals[g].add(o.Abundance); err != nil {
				return out, err
			}
			for _, p := range [][2]string{{"class", o.Class}, {"gene", o.Gene}} {
				b := bucketKey{g, p[0], p[1]}
				if buckets[b] == nil {
					buckets[b] = &isotypeAmount{}
				}
				if err := buckets[b].add(o.Abundance); err != nil {
					return out, err
				}
			}
		}
	}
	keys := make([]bucketKey, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.family != b.family {
			return a.family < b.family
		}
		if a.sample != b.sample {
			return a.sample < b.sample
		}
		if a.level != b.level {
			return a.level < b.level
		}
		return a.state < b.state
	})
	for _, k := range keys {
		a, total := buckets[k], totals[k.groupKey]
		metadata := ann[k.sample]
		u := IsotypeUsage{Family: k.family, Sample: k.sample, Timepoint: metadata.Time, Tissue: metadata.Tissue, Level: k.level, State: k.state, Observations: a.n, Total: total.n, Share: float64(a.n) / float64(total.n), Abundance: a.value(), TotalAbundance: total.value(), MissingAbundance: a.missing, TotalMissingAbundance: total.missing}
		if u.Abundance != nil && u.TotalAbundance != nil && *u.TotalAbundance > 0 {
			v := *u.Abundance / *u.TotalAbundance
			u.AbundanceShare = &v
		}
		out.Usage = append(out.Usage, u)
	}
	ids := make([]string, 0, len(families))
	for id := range families {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	pairs := map[[2]string]int{}
	evaluable := 0
	for _, id := range ids {
		f := families[id]
		f.Samples = sortedIsotypeKeys(fsamples[id])
		f.Classes = sortedIsotypeKeys(fclasses[id])
		f.Genes = sortedIsotypeKeys(fgenes[id])
		f.Evidence = "no resolved constant gene"
		if len(f.Genes) > 0 {
			evaluable++
			f.Evidence = "one resolved constant gene"
		}
		if len(f.Genes) > 1 {
			f.Evidence = "multiple constant genes observed"
		}
		if len(f.Genes) == 2 && f.Genes[0] == "IGHD" && f.Genes[1] == "IGHM" {
			f.Evidence = "IgM/IgD only; not switching evidence"
		}
		for i, a := range f.Genes {
			for _, b := range f.Genes[i+1:] {
				pairs[[2]string{a, b}]++
			}
		}
		out.Families = append(out.Families, *f)
	}
	pairKeys := make([][2]string, 0, len(pairs))
	for k := range pairs {
		pairKeys = append(pairKeys, k)
	}
	sort.Slice(pairKeys, func(i, j int) bool {
		a, b := pairKeys[i], pairKeys[j]
		return a[0] < b[0] || a[0] == b[0] && a[1] < b[1]
	})
	for _, k := range pairKeys {
		out.Pairs = append(out.Pairs, IsotypePair{k[0], k[1], pairs[k], len(families), evaluable})
	}
	return out, nil
}

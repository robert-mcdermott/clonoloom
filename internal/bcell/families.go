package bcell

// Length-normalized nucleotide Hamming distance and single-linkage B-cell families:
// Gupta et al. 2017, J Immunol 198:2489–2499, doi:10.4049/jimmunol.1601850.
// V/J/length partitioning follows the published Change-O method (Gupta et al. 2015,
// Bioinformatics 31:3356–3358, doi:10.1093/bioinformatics/btv359), independently implemented.
// ClonoLoom's conservative choices: resolved gene calls, ACGT-only productive junctions,
// explicit subject boundaries and threshold, and in paired mode the maximum of heavy/light
// normalized distances, within both chains' V/J/length partitions. No fitted threshold,
// context-dependent SHM distance, or claim of biological ancestry. See research.md §5.16.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
)

const FamilyVersion = 1
const maxFamilyObservations = 250000
const maxFamilyBaseComparisons int64 = 200000000

type FamilyOptions struct {
	Mode      string   `json:"mode"`      // auto, heavy, paired; auto is resolved before storage
	Scope     string   `json:"scope"`     // subject, or explicitly sample
	Threshold *float64 `json:"threshold"` // nil: nearest distances only; zero is exact identity
}

func (o FamilyOptions) Normalize(singleCell bool) (FamilyOptions, error) {
	if o.Mode == "" || o.Mode == "auto" {
		o.Mode = "heavy"
		if singleCell {
			o.Mode = "paired"
		}
	}
	if o.Mode != "heavy" && o.Mode != "paired" {
		return o, errors.New("family mode is heavy or paired")
	}
	if o.Mode == "paired" && !singleCell {
		return o, errors.New("paired families need cell IDs; use heavy mode for bulk data")
	}
	if o.Scope == "" {
		o.Scope = "subject"
	}
	if o.Scope != "subject" && o.Scope != "sample" {
		return o, errors.New("family scope is subject or sample")
	}
	if o.Threshold != nil && (math.IsNaN(*o.Threshold) || math.IsInf(*o.Threshold, 0) || *o.Threshold < 0 || *o.Threshold > 1) {
		return o, errors.New("choose a family threshold from 0 to 1, or null to inspect nearest distances first")
	}
	return o, nil
}

// FamilySource identifies the immutable source, not its current absolute path.
type FamilySource struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}
type FamilyRow struct {
	ID         string       `json:"id"`
	Source     FamilySource `json:"source"`
	SequenceID string       `json:"sequenceId"`
	Line       uint32       `json:"line"`
	Locus      string       `json:"locus"`
}
type FamilyChain struct {
	Locus    string `json:"locus"`
	V        string `json:"v"`
	J        string `json:"j"`
	Junction string `json:"junction"`
}
type FamilyObservation struct {
	ID             string       `json:"id"`
	Sample         string       `json:"sample"`
	Subject        string       `json:"subject"`
	Scope          string       `json:"scope"`
	Cell           string       `json:"cell,omitempty"`
	Rows           []FamilyRow  `json:"rows"`
	Heavy          FamilyChain  `json:"heavy"`
	Light          *FamilyChain `json:"light,omitempty"`
	Abundance      *float64     `json:"abundance"`
	Representative string       `json:"representative,omitempty"`
	Family         string       `json:"family,omitempty"`
	Excluded       string       `json:"excluded,omitempty"`
}
type FamilyRepresentative struct {
	ID           string       `json:"id"`
	Scope        string       `json:"scope"`
	Heavy        FamilyChain  `json:"heavy"`
	Light        *FamilyChain `json:"light,omitempty"`
	Observations int          `json:"observations"`
	Nearest      *float64     `json:"nearest"` // nearest distinct receptor; null means no comparable receptor
	Neighbor     string       `json:"neighbor,omitempty"`
	Family       string       `json:"family,omitempty"`
}
type FamilyAmount struct {
	Sample       string   `json:"sample"`
	Observations int      `json:"observations"`
	Abundance    *float64 `json:"abundance"`
}
type Family struct {
	ID              string         `json:"id"`
	Scope           string         `json:"scope"`
	Representatives int            `json:"representatives"`
	Observations    int            `json:"observations"`
	Samples         []FamilyAmount `json:"samples"`
}
type FamilyBin struct {
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
	Count int     `json:"count"`
}
type FamilyResult struct {
	Options         FamilyOptions          `json:"familiesOptions"`
	Unit            string                 `json:"unit"`
	SingleCell      bool                   `json:"singleCell"`
	Observations    []FamilyObservation    `json:"observations"`
	Representatives []FamilyRepresentative `json:"representatives"`
	Families        []Family               `json:"families"`
	Histogram       []FamilyBin            `json:"histogram"`
	WithoutNeighbor int                    `json:"withoutNeighbor"`
	Excluded        map[string]int         `json:"excluded"` // observations, not sequence rows
	Eligible        int                    `json:"eligible"`
	FilteredRows    int                    `json:"filteredRows"`
	OtherRows       int                    `json:"otherRows"`
	Comparisons     int64                  `json:"comparisons"`
	Description     string                 `json:"description"`
}

func familyID(prefix string, value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return prefix + hex.EncodeToString(sum[:16])
}

func familyChain(t *rep.Table, row int) (FamilyChain, string) {
	c := FamilyChain{Locus: t.LocusName(row), Junction: strings.ToUpper(t.Junctions.Get(t.Junction[row]))}
	if t.Productive[row] != rep.True {
		return c, "not explicitly productive"
	}
	if c.Junction == "" || len(c.Junction)%3 != 0 || len(c.Junction) > 1000 || strings.Trim(c.Junction, "ACGT") != "" {
		return c, "missing, ambiguous or out-of-frame nucleotide junction"
	}
	for _, x := range []struct {
		code    uint32
		segment genes.Segment
		dst     *string
	}{{t.V[row], genes.V, &c.V}, {t.J[row], genes.J, &c.J}} {
		call := t.Call(x.code)
		if call.Unreadable != 0 || !call.Resolved(genes.LevelGene) || call.Locus() != c.Locus {
			return c, "missing, ambiguous or inconsistent V/J gene"
		}
		for _, n := range call.Names {
			if n.Segment != x.segment {
				return c, "missing, ambiguous or inconsistent V/J gene"
			}
		}
		*x.dst = call.Key(genes.LevelGene)
	}
	return c, ""
}

func chainPartition(c FamilyChain) string {
	return fmt.Sprintf("%s|%s|%s|%d", c.Locus, c.V, c.J, len(c.Junction))
}
func receptorKey(scope string, heavy FamilyChain, light *FamilyChain) string {
	b, _ := json.Marshal([]any{scope, heavy, light})
	return string(b)
}
func partitionKey(scope string, heavy FamilyChain, light *FamilyChain) string {
	parts := []string{scope, chainPartition(heavy)}
	if light != nil {
		parts = append(parts, chainPartition(*light))
	}
	b, _ := json.Marshal(parts)
	return string(b)
}
func normalizedHamming(a, b string) float64 {
	d := 0
	for i := range a {
		if a[i] != b[i] {
			d++
		}
	}
	return float64(d) / float64(len(a))
}
func familyDistance(a, b FamilyRepresentative) float64 {
	d := normalizedHamming(a.Heavy.Junction, b.Heavy.Junction)
	if a.Light != nil {
		d = math.Max(d, normalizedHamming(a.Light.Junction, b.Light.Junction))
	}
	return d
}

// Families compares raw retained observations, not global junction clonotypes: otherwise the
// same receptor in different subjects could bridge families. Cell IDs are scoped by sample.
// Duplicate identical chains in a cell are collapsed; multiple distinct chains are excluded.
// A missing bulk count stays null. Cell abundance is exactly one, independent of chain counts.
func Families(ctx context.Context, t *rep.Table, keep []bool, sources []FamilySource, subjects map[string]string, o FamilyOptions, progress func(int, int)) (FamilyResult, error) {
	result := FamilyResult{Observations: []FamilyObservation{}, Representatives: []FamilyRepresentative{}, Families: []Family{}, Excluded: map[string]int{}, SingleCell: len(t.Cell) > 0, Unit: string(t.Unit)}
	o, err := o.Normalize(result.SingleCell)
	if err != nil {
		return result, err
	}
	result.Options = o
	if len(keep) != 0 && len(keep) != t.Len() {
		return result, errors.New("family filter length differs from the table")
	}
	if result.SingleCell {
		result.Unit = string(rep.Cells)
	}
	if progress == nil {
		progress = func(int, int) {}
	}
	groups := map[string][]int{}
	for row := 0; row < t.Len(); row++ {
		if row%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return result, err
			}
		}
		if len(keep) > 0 && !keep[row] {
			result.FilteredRows++
			continue
		}
		locus := t.LocusName(row)
		if locus != "IGH" && (o.Mode != "paired" || locus != "IGK" && locus != "IGL") {
			result.OtherRows++
			continue
		}
		if int(t.Source[row]) >= len(sources) {
			return result, errors.New("family source identity is missing")
		}
		sample := t.Samples[t.Sample[row]].Name
		key := familyID("row-", []any{sources[t.Source[row]], t.Line[row], t.IDs.Get(t.SequenceID[row])})
		if result.SingleCell && t.Cell[row] != 0 {
			key = familyID("cell-", []string{sample, t.Dict.Get(t.Cell[row])})
		}
		groups[key] = append(groups[key], row)
		if len(groups) > maxFamilyObservations {
			return result, fmt.Errorf("families support at most %d observations; restrict the QC filter or split the dataset", maxFamilyObservations)
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		rows := groups[key]
		first := rows[0]
		sample := t.Samples[t.Sample[first]].Name
		obs := FamilyObservation{ID: key, Sample: sample, Subject: strings.TrimSpace(subjects[sample]), Rows: []FamilyRow{}}
		obs.Scope = "subject:" + obs.Subject
		if o.Scope == "sample" {
			obs.Scope = "sample:" + sample
		} else if obs.Subject == "" {
			obs.Excluded = "sample has no subject"
		}
		if result.SingleCell {
			obs.Cell = t.Dict.Get(t.Cell[first])
			one := 1.0
			obs.Abundance = &one
			if obs.Cell == "" {
				obs.Excluded = "missing cell ID in single-cell data"
			}
		} else if count := t.Count[first]; !math.IsNaN(count) && !math.IsInf(count, 0) && count >= 0 {
			obs.Abundance = &count
		}
		heavy, light := map[FamilyChain]bool{}, map[FamilyChain]bool{}
		invalid := ""
		for _, row := range rows {
			src := sources[t.Source[row]]
			ref := FamilyRow{Source: src, SequenceID: t.IDs.Get(t.SequenceID[row]), Line: t.Line[row], Locus: t.LocusName(row)}
			ref.ID = familyID("bcr-row-", []any{src, ref.Line, ref.SequenceID})
			obs.Rows = append(obs.Rows, ref)
			chain, why := familyChain(t, row)
			if why != "" {
				if invalid == "" || why < invalid {
					invalid = why
				}
				continue
			}
			if chain.Locus == "IGH" {
				heavy[chain] = true
			} else {
				light[chain] = true
			}
		}
		if obs.Excluded == "" {
			obs.Excluded = invalid
		}
		sort.Slice(obs.Rows, func(i, j int) bool { return obs.Rows[i].ID < obs.Rows[j].ID })
		if obs.Excluded == "" && len(heavy) != 1 {
			obs.Excluded = "requires one distinct productive heavy chain"
		}
		if obs.Excluded == "" && o.Mode == "paired" && len(light) != 1 {
			obs.Excluded = "requires one distinct productive light chain"
		}
		if obs.Excluded == "" {
			for c := range heavy {
				obs.Heavy = c
			}
			if o.Mode == "paired" {
				for c := range light {
					c := c
					obs.Light = &c
				}
			}
			result.Eligible++
		} else {
			result.Excluded[obs.Excluded]++
		}
		result.Observations = append(result.Observations, obs)
	}
	// Sort and collapse by the complete compared receptor. Each distinct receptor contributes
	// once to the nearest-distance histogram, irrespective of abundance or repeated samples.
	represented := map[string][]int{}
	for i, obs := range result.Observations {
		if obs.Excluded == "" {
			k := receptorKey(obs.Scope, obs.Heavy, obs.Light)
			represented[k] = append(represented[k], i)
		}
	}
	keys = keys[:0]
	for key := range represented {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	partitions := map[string][]int{}
	members := make([][]int, len(keys))
	for i, key := range keys {
		members[i] = represented[key]
		obs := result.Observations[members[i][0]]
		r := FamilyRepresentative{ID: familyID("bcr-", key), Scope: obs.Scope, Heavy: obs.Heavy, Light: obs.Light, Observations: len(members[i])}
		result.Representatives = append(result.Representatives, r)
		for _, m := range members[i] {
			result.Observations[m].Representative = r.ID
		}
		p := partitionKey(r.Scope, r.Heavy, r.Light)
		partitions[p] = append(partitions[p], i)
	}
	partitionKeys := make([]string, 0, len(partitions))
	cost := int64(0)
	for key, indices := range partitions {
		partitionKeys = append(partitionKeys, key)
		n := int64(len(indices))
		bases := len(result.Representatives[indices[0]].Heavy.Junction)
		if light := result.Representatives[indices[0]].Light; light != nil {
			bases += len(light.Junction)
		}
		cost += n * (n - 1) / 2 * int64(bases)
		if cost > maxFamilyBaseComparisons {
			return result, fmt.Errorf("family comparison exceeds the %d-base work limit; restrict the QC filter or use explicit sample scope (no subsampling is performed)", maxFamilyBaseComparisons)
		}
	}
	sort.Strings(partitionKeys)
	parent := make([]int, len(keys))
	for i := range parent {
		parent[i] = i
	}
	root := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for pi, key := range partitionKeys {
		indices := partitions[key]
		for at, i := range indices {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			for _, j := range indices[at+1:] {
				if result.Comparisons%4096 == 0 {
					if err := ctx.Err(); err != nil {
						return result, err
					}
				}
				d := familyDistance(result.Representatives[i], result.Representatives[j])
				result.Comparisons++
				for _, pair := range [][2]int{{i, j}, {j, i}} {
					a, b := &result.Representatives[pair[0]], result.Representatives[pair[1]]
					if a.Nearest == nil || d < *a.Nearest || d == *a.Nearest && b.ID < a.Neighbor {
						distance := d
						a.Nearest = &distance
						a.Neighbor = b.ID
					}
				}
				if o.Threshold != nil && d <= *o.Threshold {
					a, b := root(i), root(j)
					if a > b {
						a, b = b, a
					}
					parent[b] = a
				}
			}
		}
		progress(pi+1, len(partitionKeys))
	}
	result.Histogram = make([]FamilyBin, 20)
	for i := range result.Histogram {
		result.Histogram[i] = FamilyBin{Lower: float64(i) / 20, Upper: float64(i+1) / 20}
	}
	for _, r := range result.Representatives {
		if r.Nearest == nil {
			result.WithoutNeighbor++
		} else {
			bin := min(19, int(*r.Nearest*20))
			result.Histogram[bin].Count++
		}
	}
	if o.Threshold != nil {
		components := map[int][]int{}
		for i := range parent {
			p := root(i)
			components[p] = append(components[p], i)
		}
		for i := range parent {
			indices, ok := components[i]
			if !ok {
				continue
			}
			ids := make([]string, len(indices))
			for k, n := range indices {
				ids[k] = result.Representatives[n].ID
			}
			sort.Strings(ids)
			family := Family{ID: familyID("bcf-", []any{FamilyVersion, o, ids}), Scope: result.Representatives[i].Scope, Representatives: len(indices), Samples: []FamilyAmount{}}
			amounts := map[string]*FamilyAmount{}
			missing := map[string]bool{}
			for _, n := range indices {
				result.Representatives[n].Family = family.ID
				for _, m := range members[n] {
					obs := &result.Observations[m]
					obs.Family = family.ID
					family.Observations++
					x := amounts[obs.Sample]
					if x == nil {
						zero := 0.0
						x = &FamilyAmount{Sample: obs.Sample, Abundance: &zero}
						amounts[obs.Sample] = x
					}
					x.Observations++
					if obs.Abundance == nil {
						missing[obs.Sample] = true
					} else {
						*x.Abundance += *obs.Abundance
					}
				}
			}
			names := make([]string, 0, len(amounts))
			for name := range amounts {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				x := amounts[name]
				if missing[name] || math.IsInf(*x.Abundance, 0) {
					x.Abundance = nil
				}
				family.Samples = append(family.Samples, *x)
			}
			result.Families = append(result.Families, family)
		}
		sort.Slice(result.Families, func(i, j int) bool { return result.Families[i].ID < result.Families[j].ID })
	}
	mode := "heavy-chain junctions"
	if o.Mode == "paired" {
		mode = "paired heavy/light junctions, using the larger of the two normalized distances"
	}
	result.Description = fmt.Sprintf("Length-normalized nucleotide Hamming distance on %s; identical V and J genes and junction length on each compared chain, within %s. Productive ACGT junctions and resolved genes only; ambiguous/missing evidence is excluded. Each distinct receptor contributes once to nearest distances; no comparable receptor is missing, not zero.", mode, o.Scope)
	if o.Threshold == nil {
		result.Description += " Distances only; no family threshold has been selected."
	} else {
		result.Description += fmt.Sprintf(" Single-linkage families at distance ≤ %g; chains of links can join receptors farther apart than the threshold. Threshold-dependent families are not proven ancestry or the existing junction clonotypes.", *o.Threshold)
	}
	return result, nil
}

// Package invariant flags receptors consistent with published invariant-population rules.
// MAIT and iNKT: Schattgen et al., Nat Biotechnol 2022, doi:10.1038/s41587-021-00989-2,
// Methods, TCR sequence features. GEM: Van Rhijn et al., Nat Immunol 2013,
// doi:10.1038/ni.2630, and J Immunol 2014, doi:10.4049/jimmunol.1401380.
// These are sequence rules, not diagnoses of cell phenotype or antigen specificity.
package invariant

import (
	"clonoloom/internal/clonotype"
	"clonoloom/internal/genes"
	"fmt"
	"strings"
)

const Version = 1
const Match = "match"
const NoMatch = "no match"
const Unknown = "not evaluable"

var Populations = []string{"MAIT", "iNKT", "GEM"}

type Flags [3]string

type Sample struct {
	Total     float64     `json:"total"`
	Matched   [3]float64  `json:"matched"`
	Evaluable [3]float64  `json:"evaluable"`
	Share     [3]*float64 `json:"share"`
}
type Result struct {
	Species string   `json:"species"`
	Version int      `json:"version"`
	Flags   []Flags  `json:"flags"`
	Samples []Sample `json:"samples"`
}

func ValidSpecies(species string) bool {
	return species == "" || species == "human" || species == "mouse"
}

// Classify uses only the retained chains, never a discarded secondary chain. Unknown covers
// missing/ambiguous evidence and definitions that removed the required resolution. A match
// needs one fully resolved alpha chain; human iNKT also needs a beta chain in the same clonotype.
func Classify(c clonotype.Clonotype, d clonotype.Definition, species string) Flags {
	out := Flags{Unknown, Unknown, Unknown}
	if species != "human" && species != "mouse" {
		return out
	}
	if d.Vendor || d.Junction != "aa" || (d.V != "gene" && d.V != "allele") || (d.J != "gene" && d.J != "allele") {
		return out
	}
	for p := range out {
		if species == "mouse" && p == 2 {
			continue
		} // No mouse GEM rule is asserted.
		seen, uncertain, matched := false, false, false
		for _, ch := range c.Chains {
			if ch.Locus != "TRA" {
				continue
			}
			seen = true
			v, j := resolved(ch.V, "TRA", genes.V), resolved(ch.J, "TRA", genes.J)
			if v == "" || j == "" || !aminoAcids(ch.Junction) {
				uncertain = true
				continue
			}
			n := len(ch.Junction)
			switch p {
			case 0:
				if species == "human" {
					matched = matched || (v == "TRAV1-2" && (j == "TRAJ33" || j == "TRAJ20" || j == "TRAJ12") && n == 12)
				} else {
					matched = matched || (v == "TRAV1" && j == "TRAJ33" && n == 12)
				}
			case 1:
				if species == "mouse" {
					matched = matched || ((v == "TRAV11" || v == "TRAV11D") && j == "TRAJ18" && n == 15)
					continue
				}
				if v != "TRAV10" || j != "TRAJ18" || n < 14 || n > 16 {
					continue
				}
				beta, missing := false, false
				for _, b := range c.Chains {
					if b.Locus != "TRB" {
						continue
					}
					beta = true
					bv := resolved(b.V, "TRB", genes.V)
					missing = missing || bv == ""
					matched = matched || bv == "TRBV25-1" || bv == "TRBV25"
				}
				uncertain = uncertain || !beta || missing
			case 2:
				// X in the published motif means one resolved amino acid, not an unknown residue.
				matched = matched || (v == "TRAV1-2" && j == "TRAJ9" && n == 13 && strings.HasPrefix(ch.Junction, "CAV") && strings.ContainsRune("RL", rune(ch.Junction[3])) && ch.Junction[5:] == "TGGFKTIF")
			}
		}
		if matched {
			out[p] = Match
		} else if seen && !uncertain {
			out[p] = NoMatch
		}
	}
	return out
}
func resolved(s, locus string, segment genes.Segment) string {
	c := genes.ParseCall(s)
	if c.Unreadable != 0 || !c.Resolved(genes.LevelGene) {
		return ""
	}
	for _, n := range c.Names {
		if n.Locus != locus || n.Segment != segment {
			return ""
		}
	}
	return c.Key(genes.LevelGene)
}
func aminoAcids(s string) bool {
	if s == "" {
		return false
	}
	for _, aa := range s {
		if !strings.ContainsRune("ACDEFGHIKLMNPQRSTVWY", aa) {
			return false
		}
	}
	return true
}

// Compute counts each clonotype once per rule and sample, regardless of matching chain count.
// Shares use ALL retained abundance, including unevaluable receptors. Evaluable abundance is
// reported separately; a zero denominator or an entirely unevaluable rule has a missing share.
func Compute(t *clonotype.Table, species string) (*Result, error) {
	if !ValidSpecies(species) {
		return nil, fmt.Errorf("invariant species: human, mouse, or empty (not evaluated)")
	}
	r := &Result{Species: species, Version: Version, Flags: make([]Flags, len(t.Clonotypes)), Samples: make([]Sample, len(t.SampleTotals))}
	for s, total := range t.SampleTotals {
		r.Samples[s].Total = total
	}
	for i, c := range t.Clonotypes {
		f := Classify(c, t.Definition, species)
		r.Flags[i] = f
		ss, vv := t.Abundances(i)
		for k, s := range ss {
			for p, state := range f {
				if state != Unknown {
					r.Samples[s].Evaluable[p] += vv[k]
				}
				if state == Match {
					r.Samples[s].Matched[p] += vv[k]
				}
			}
		}
	}
	for s := range r.Samples {
		x := &r.Samples[s]
		for p := range x.Share {
			if x.Total > 0 && x.Evaluable[p] > 0 {
				v := x.Matched[p] / x.Total
				x.Share[p] = &v
			}
		}
	}
	return r, nil
}

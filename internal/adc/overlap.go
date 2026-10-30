package adc

// Descriptive presence overlap. Jaccard = intersection/union (Jaccard 1901);
// see research §5.65 and docs/ADC-OVERLAP.md. No abundance or prevalence estimator.
import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

const MatchDefinition = "productive-single-chain-locus-exact-junction-aa-v1"

type Sequence struct {
	Locus, Junction string
	Productive      bool
}
type Eligibility struct {
	Rows     int `json:"rows"`
	Eligible int `json:"eligible"`
	Unique   int `json:"unique"`
	Excluded int `json:"excluded"`
}
type PublicPair struct {
	Capture        string      `json:"capture"`
	ManifestSHA256 string      `json:"manifestSHA256"`
	Repository     string      `json:"repository"`
	Repertoire     string      `json:"repertoire"`
	Processing     string      `json:"processing"`
	Study          string      `json:"study"`
	Subject        string      `json:"subject"`
	Stop           string      `json:"stop"`
	License        string      `json:"permissionEvidence"`
	Citation       string      `json:"citation"`
	Eligibility    Eligibility `json:"eligibility"`
	Shared         int         `json:"shared"`
	Union          int         `json:"union"`
	Jaccard        *float64    `json:"jaccard"`
}
type KeyPresence struct {
	Key      string `json:"key"`
	Captures int    `json:"captures"`
	Subjects int    `json:"subjects"`
}

type PublicOverlap struct {
	SharedKeys         []KeyPresence `json:"sharedKeys"`
	Definition         string        `json:"definition"`
	Locus              string        `json:"locus"`
	Query              Eligibility   `json:"query"`
	Pairs              []PublicPair  `json:"pairs"`
	AssignedSubjects   int           `json:"assignedSubjects"`
	EvaluableSubjects  int           `json:"evaluableSubjects"`
	MissingSubjects    int           `json:"missingSubjects"`
	UnassignedCaptures int           `json:"unassignedCaptures"`
	QueryKeysShared    int           `json:"queryKeysShared"`
	Notice             string        `json:"notice"`
}

func SequenceKey(s Sequence, locus string) string {
	if !s.Productive || s.Locus != locus || len(s.Junction) < 3 || len(s.Junction) > 256 || s.Junction[0] != 'C' || !strings.ContainsRune("FW", rune(s.Junction[len(s.Junction)-1])) {
		return ""
	}
	for _, c := range s.Junction {
		if !strings.ContainsRune("ACDEFGHIKLMNPQRSTVWY", c) {
			return ""
		}
	}
	return s.Locus + ":" + s.Junction
}
func sequenceSet(ctx context.Context, rows []Sequence, locus string) (map[string]bool, Eligibility, error) {
	set := map[string]bool{}
	e := Eligibility{Rows: len(rows)}
	for i, row := range rows {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, e, err
			}
		}
		key := SequenceKey(row, locus)
		if key == "" {
			e.Excluded++
			continue
		}
		e.Eligible++
		set[key] = true
	}
	e.Unique = len(set)
	return set, e, nil
}
func CompareCaptures(ctx context.Context, query []Sequence, locus string, captures map[string]Capture) (PublicOverlap, error) {
	out := PublicOverlap{Definition: MatchDefinition, Locus: locus, Pairs: []PublicPair{}, SharedKeys: []KeyPresence{}, Notice: "Descriptive presence among selected captured records only. All captures have unestablished completeness. No abundance units, pairing, population prevalence, antigen specificity, assay equivalence or exact remote replay are inferred. A subject comparison is evaluable only when both eligible sets are nonempty. Missing/unevaluable subjects are excluded from the evaluable denominator; scope is repository + study + subject. The project's active clonotype definition and QC filter are not applied by this separate explicit matching contract."}
	valid := false
	for _, l := range []string{"TRA", "TRB", "TRG", "TRD", "IGH", "IGK", "IGL"} {
		if locus == l {
			valid = true
		}
	}
	if !valid || len(query) > 1000000 || len(captures) < 1 || len(captures) > 20 {
		return out, errors.New("choose one locus and 1–20 captures; query limit is one million rows")
	}
	q, e, err := sequenceSet(ctx, query, locus)
	if err != nil {
		return out, err
	}
	out.Query = e
	names := []string{}
	for name := range captures {
		names = append(names, name)
	}
	sort.Strings(names)
	subjects := map[string]bool{}
	scopes := map[string]bool{}
	sharedKeys := map[string]KeyPresence{}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		c := captures[name]
		m := c.Manifest
		scope, _ := json.Marshal([]string{m.Spec.Repository, m.Spec.Repertoire, m.Spec.Processing})
		if scopes[string(scope)] {
			return out, errors.New("duplicate capture scope; select one observation per repertoire/processing pair")
		}
		scopes[string(scope)] = true
		if c.Subject != "" && c.Study != "" {
			key, _ := json.Marshal([]string{m.Spec.Repository, c.Study, c.Subject})
			if subjects[string(key)] {
				return out, errors.New("multiple captures for one scoped subject; select one explicitly instead of pooling visits or processing")
			}
			subjects[string(key)] = true
			out.AssignedSubjects++
		} else {
			out.UnassignedCaptures++
		}
		rows := []Sequence{}
		for _, row := range c.Rows {
			l, _ := text(row, "locus")
			j, _ := text(row, "junction_aa")
			rows = append(rows, Sequence{l, j, string(row["productive"]) == "true"})
		}
		set, elig, err := sequenceSet(ctx, rows, locus)
		if err != nil {
			return out, err
		}
		p := PublicPair{Capture: name, ManifestSHA256: c.SHA256, Repository: m.Spec.Repository, Repertoire: m.Spec.Repertoire, Processing: m.Spec.Processing, Study: c.Study, Subject: c.Subject, Stop: m.Stop, License: m.Spec.License, Citation: m.Spec.Citation, Eligibility: elig}
		for key := range q {
			if set[key] {
				p.Shared++
				presence := sharedKeys[key]
				presence.Key = key
				presence.Captures++
				if c.Subject != "" && c.Study != "" {
					presence.Subjects++
				}
				sharedKeys[key] = presence
			}
		}
		p.Union = len(q) + len(set) - p.Shared
		if len(q) > 0 && len(set) > 0 {
			v := float64(p.Shared) / float64(p.Union)
			p.Jaccard = &v
		}
		if c.Subject != "" && c.Study != "" {
			if len(q) > 0 && len(set) > 0 {
				out.EvaluableSubjects++
			} else {
				out.MissingSubjects++
			}
		}
		out.Pairs = append(out.Pairs, p)
	}
	for _, presence := range sharedKeys {
		out.SharedKeys = append(out.SharedKeys, presence)
	}
	sort.Slice(out.SharedKeys, func(i, j int) bool { return out.SharedKeys[i].Key < out.SharedKeys[j].Key })
	out.QueryKeysShared = len(sharedKeys)
	return out, nil
}

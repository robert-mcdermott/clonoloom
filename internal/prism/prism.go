// Package prism maps saved subject observations to data-only Prism Column tables.
// GraphPad's public XML schema and Column/missing-value guides; research §5.67.
// Presentation follows Apache-2.0 CytoWeave; mapping and exact tokens are native Go.
package prism

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/airr"
	"clonoloom/internal/study"
)

const Limits = "Data-only Prism 7+ interchange, not scientific replay or an analysis template. Each row is one declared subject; paired columns share that subject. Independent subjects occupy only their own group column: opposite blanks are structural, not missing measurements. Missing measurements stay blank and keep their reasons in mapping.json. No pooling, imputation, complete-case deletion, technical-replicate expansion or new statistics. Prism can round imported numbers and may exclude incomplete pairs when analyzing; ClonoLoom cohort comparisons instead leave a feature untested when any selected measurement is missing. Review the design, missingness, statistical methods and correction family before a new Prism analysis. Keep original JSON and certificates; import/re-saving is not an exact replay. Provider rights and identifiers remain."
const MaxCells = 100000
const MaxBytes = 32 << 20

type Cell struct {
	State   string          `json:"state"`
	Sample  string          `json:"sample,omitempty"`
	Pointer string          `json:"pointer,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
	Missing string          `json:"missing,omitempty"`
}
type Row struct {
	Subject string  `json:"subject"`
	Cells   [2]Cell `json:"cells"`
}
type Table struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Feature study.Feature `json:"feature"`
	Rows    []Row         `json:"rows"`
}
type Mapping struct {
	Format     string            `json:"format"`
	Version    int               `json:"version"`
	DesignID   string            `json:"designID"`
	Unit       string            `json:"unit"`
	Layout     string            `json:"layout"`
	GroupField string            `json:"groupField"`
	Levels     []string          `json:"levels"`
	Excluded   []study.CohortRow `json:"excluded"`
	Tables     []Table           `json:"tables"`
	Limits     string            `json:"limits"`
}
type value struct {
	Value   json.RawMessage `json:"value"`
	Missing string          `json:"missing"`
}
type featureRow struct {
	study.CohortSample
	Group  string  `json:"group"`
	Values []value `json:"values"`
}
type featureDoc struct {
	Format    string             `json:"format"`
	Version   int                `json:"version"`
	Design    study.CohortDesign `json:"design"`
	Selection struct {
		ID    string            `json:"id"`
		Input study.CohortInput `json:"input"`
	} `json:"selection"`
	Parent struct {
		Unit string `json:"unit"`
	} `json:"parent"`
	Table struct {
		Catalog []study.Feature `json:"catalog"`
		Rows    []featureRow    `json:"rows"`
	} `json:"table"`
}
type point struct {
	Subject string `json:"subject"`
	Sample  string `json:"sample"`
	Group   string `json:"group"`
	value
}
type comparison struct {
	Feature     study.Feature `json:"feature"`
	Points      []point       `json:"points"`
	NReference  int           `json:"nReference"`
	NComparison int           `json:"nComparison"`
	Pairs       int           `json:"pairs"`
	Missing     int           `json:"missing"`
}

// Number checks interchange bounds without rewriting the original JSON token.
// Decimal fractions can still round inside Prism; exact exported bytes remain separate.
func Number(token string) error {
	if len(token) > 256 || !json.Valid([]byte(token)) {
		return errors.New("invalid or oversized numeric token")
	}
	if i := strings.IndexAny(token, "eE"); i >= 0 {
		exp, e := strconv.Atoi(token[i+1:])
		if e != nil || exp < -400 || exp > 400 {
			return errors.New("numeric exponent outside bounded Prism interchange")
		}
	}
	r, ok := new(big.Rat).SetString(token)
	if !ok || strings.Contains(token, "/") {
		return errors.New("expected a JSON number")
	}
	if r.Sign() == 0 && strings.HasPrefix(token, "-") {
		return errors.New("negative zero has no portable Prism representation")
	}
	a := new(big.Rat).Abs(r)
	lo, _ := new(big.Rat).SetString("1e-300")
	hi, _ := new(big.Rat).SetString("1e300")
	if a.Sign() != 0 && (a.Cmp(lo) < 0 || a.Cmp(hi) > 0) {
		return errors.New("numeric value outside the conservative Prism range 1e-300 through 1e300")
	}
	if r.IsInt() && new(big.Int).Abs(r.Num()).Cmp(big.NewInt(9007199254740991)) > 0 {
		return errors.New("integer exceeds exact binary64 integer range; use the native evidence")
	}
	return nil
}
func checked(v value) (Cell, error) {
	token := string(v.Value)
	c := Cell{Value: v.Value, Missing: v.Missing, State: "observed"}
	if token == "null" {
		if strings.TrimSpace(v.Missing) == "" {
			return c, errors.New("missing measurement requires a reason")
		}
		c.State = "missing"
		return c, nil
	}
	if v.Missing != "" {
		return c, errors.New("numeric measurement conflicts with its missingness reason")
	}
	return c, Number(token)
}
func equalValue(a, b value) bool { return a.Missing == b.Missing && bytes.Equal(a.Value, b.Value) }

// Map checks recorded structure and identity only; no scientific worker is run.
func Map(ctx context.Context, raw []byte) (Mapping, error) {
	out := Mapping{Format: "clonoloom-prism-mapping", Version: 1, Unit: "subject", Limits: Limits, Excluded: []study.CohortRow{}, Tables: []Table{}}
	fail := func(s string) (Mapping, error) { return out, errors.New(s) }
	if len(raw) > MaxBytes {
		return fail("Prism source exceeds 32 MiB")
	}
	if e := airr.CheckExactJSON(ctx, raw); e != nil {
		return out, e
	}
	var head struct {
		Format   string          `json:"format"`
		Version  int             `json:"version"`
		Features json.RawMessage `json:"features"`
		Analysis struct {
			Layout  string   `json:"layout"`
			Levels  []string `json:"levels"`
			Options struct {
				Features []string `json:"features"`
			} `json:"options"`
			Comparisons []comparison `json:"comparisons"`
		} `json:"analysis"`
	}
	if e := json.Unmarshal(raw, &head); e != nil {
		return out, e
	}
	if head.Version != 1 || (head.Format != "clonoloom-cohort-features" && head.Format != "clonoloom-cohort-compare") {
		return fail("Prism maps only version 1 subject features and cohort comparisons")
	}
	featureRaw, prefix := raw, ""
	isComparison := head.Format == "clonoloom-cohort-compare"
	if isComparison {
		featureRaw, prefix = head.Features, "/features"
	}
	var f featureDoc
	if e := json.Unmarshal(featureRaw, &f); e != nil {
		return out, e
	}
	if f.Format != "clonoloom-cohort-features" || f.Version != 1 {
		return fail("unsupported embedded feature table")
	}
	design, e := study.CheckCohort(f.Selection.Input)
	if e != nil {
		return out, e
	}
	if !design.OK || design.ID != f.Selection.ID || !reflect.DeepEqual(design, f.Design) {
		return fail("saved subject design is inconsistent")
	}
	switch f.Parent.Unit {
	case "cells", "reads", "umis", "templates", "sequences":
	default:
		return fail("unsupported abundance unit")
	}
	if !reflect.DeepEqual(f.Table.Catalog, study.FeatureCatalog(f.Parent.Unit)) {
		return fail("unsupported or changed feature catalog")
	}
	out.DesignID, out.Layout, out.GroupField, out.Levels = design.ID, design.Input.Spec.Layout, design.Input.Spec.GroupField, design.Input.Spec.Levels
	expected := map[string]study.CohortRow{}
	subjects := map[string]bool{}
	for _, r := range design.Rows {
		if r.Included {
			expected[r.Name] = r
			subjects[r.Subject] = true
		} else {
			out.Excluded = append(out.Excluded, r)
		}
	}
	if len(f.Table.Rows) != len(expected) {
		return fail("feature rows differ from selected experimental units")
	}
	rows := map[string]featureRow{}
	positions := map[string]int{}
	for i, r := range f.Table.Rows {
		want, ok := expected[r.Name]
		_, duplicate := rows[r.Name]
		if !ok || duplicate || r.CohortSample != want.CohortSample || r.Group != want.Group || len(r.Values) != len(f.Table.Catalog) {
			return fail("ambiguous subject/sample/group or feature mapping")
		}
		for _, v := range r.Values {
			if _, e := checked(v); e != nil {
				return out, e
			}
		}
		rows[r.Name], positions[r.Name] = r, i
	}
	names := []string{}
	for s := range subjects {
		names = append(names, s)
	}
	sort.Strings(names)
	features := f.Table.Catalog
	if isComparison {
		if head.Analysis.Layout != out.Layout || !reflect.DeepEqual(head.Analysis.Levels, out.Levels) || len(head.Analysis.Comparisons) < 1 || len(head.Analysis.Comparisons) > 13 {
			return fail("comparison design differs")
		}
		features = []study.Feature{}
		ids := []string{}
		seen := map[string]bool{}
		for _, c := range head.Analysis.Comparisons {
			if seen[c.Feature.ID] {
				return fail("repeated comparison feature")
			}
			seen[c.Feature.ID] = true
			features = append(features, c.Feature)
			ids = append(ids, c.Feature.ID)
		}
		sort.Strings(ids)
		if !reflect.DeepEqual(ids, head.Analysis.Options.Features) {
			return fail("comparison feature selection differs")
		}
	}
	if len(features)*len(names)*2 > MaxCells {
		return fail("Prism export exceeds 100,000 data cells")
	}
	for ti, feature := range features {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		col := -1
		for j, c := range f.Table.Catalog {
			if c == feature {
				col = j
			}
		}
		if col < 0 {
			return fail("comparison feature differs from its catalog")
		}
		table := Table{ID: fmt.Sprintf("Table%d", ti), Title: fmt.Sprintf("%s [%s; %s; subject]", feature.Name, feature.Unit, out.Layout), Feature: feature, Rows: []Row{}}
		cells := map[string]Cell{}
		if isComparison {
			c := head.Analysis.Comparisons[ti]
			counts := [2]int{}
			missing := 0
			for pi, p := range c.Points {
				r, ok := rows[p.Sample]
				_, duplicate := cells[p.Sample]
				if !ok || duplicate || p.Subject != r.Subject || p.Group != r.Group || !equalValue(p.value, r.Values[col]) {
					return fail("comparison points differ from captured subject measurements")
				}
				cell, e := checked(p.value)
				if e != nil {
					return out, e
				}
				cell.Sample = p.Sample
				cell.Pointer = fmt.Sprintf("/analysis/comparisons/%d/points/%d/value", ti, pi)
				cells[p.Sample] = cell
				g := 0
				if p.Group == out.Levels[1] {
					g = 1
				}
				counts[g]++
				if cell.State == "missing" {
					missing++
				}
			}
			pairs := 0
			if out.Layout == "paired" {
				pairs = len(names)
			}
			if len(cells) != len(expected) || c.NReference != counts[0] || c.NComparison != counts[1] || c.Pairs != pairs || c.Missing != missing {
				return fail("comparison denominators or missingness differ")
			}
		} else {
			for name, r := range rows {
				cell, _ := checked(r.Values[col])
				cell.Sample = name
				cell.Pointer = fmt.Sprintf("%s/table/rows/%d/values/%d/value", prefix, positions[name], col)
				cells[name] = cell
			}
		}
		bySubject := map[string]*Row{}
		for _, s := range names {
			bySubject[s] = &Row{Subject: s, Cells: [2]Cell{{State: "structural-empty"}, {State: "structural-empty"}}}
		}
		for sample, cell := range cells {
			r := rows[sample]
			g := 0
			if r.Group == out.Levels[1] {
				g = 1
			}
			target := bySubject[r.Subject]
			if target.Cells[g].State != "structural-empty" {
				return fail("duplicate subject/group observation")
			}
			target.Cells[g] = cell
		}
		for _, s := range names {
			r := *bySubject[s]
			if out.Layout == "paired" && (r.Cells[0].State == "structural-empty" || r.Cells[1].State == "structural-empty") {
				return fail("incomplete structural pair")
			}
			table.Rows = append(table.Rows, r)
		}
		out.Tables = append(out.Tables, table)
	}
	return out, nil
}

func text(s string) (string, error) {
	if !utf8.ValidString(s) || len(s) > 65536 {
		return "", errors.New("Prism text is invalid UTF-8 or exceeds 64 KiB")
	}
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' || r == 0xfffe || r == 0xffff {
			return "", errors.New("text contains a character unsupported by XML 1.0")
		}
	}
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String(), nil
}

// Write emits data and info only: no scripts, analyses, templates or graphs.
func Write(ctx context.Context, m Mapping, title, notes, software string) ([]byte, error) {
	if len(m.Tables) < 1 || len(m.Tables) > 13 || len(m.Levels) != 2 {
		return nil, errors.New("invalid Prism table count or levels")
	}
	cells := 0
	for _, table := range m.Tables {
		cells += 2 * len(table.Rows)
		if len(table.Rows) == 0 || cells > MaxCells {
			return nil, errors.New("Prism export requires nonempty tables within 100,000 data cells")
		}
	}
	var b strings.Builder
	var err error
	esc := func(s string) string {
		x, e := text(s)
		if e != nil {
			err = e
		}
		return x
	}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><GraphPadPrismFile xmlns="http://graphpad.com/prism/Prism.htm" PrismXMLVersion="5.00" LowestVersionThatCanOpen="7.00"><Created><OriginalVersion CreatedByProgram="ClonoLoom" CreatedByVersion="` + esc(software) + `"/></Created><InfoSequence><Ref ID="Info0" Selected="1"/></InfoSequence><Info ID="Info0"><Title>` + esc(title) + `</Title><Notes>`)
	for _, line := range strings.Split(Limits+"\n"+notes, "\n") {
		b.WriteString(`<Font Face="Helvetica" Color="#000000">` + esc(line) + `<BR/></Font>`)
	}
	b.WriteString(`</Notes></Info><TableSequence Selected="1">`)
	for _, t := range m.Tables {
		b.WriteString(`<Ref ID="` + esc(t.ID) + `"/>`)
	}
	b.WriteString(`</TableSequence>`)
	for _, t := range m.Tables {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		b.WriteString(`<Table ID="` + esc(t.ID) + `" XFormat="none" YFormat="replicates" Replicates="1" TableType="OneWay" EVFormat="AsteriskAfterNumber"><Title>` + esc(t.Title) + `</Title><RowTitlesColumn Width="180"><Subcolumn>`)
		for _, r := range t.Rows {
			b.WriteString(`<d>` + esc(r.Subject) + `</d>`)
		}
		b.WriteString(`</Subcolumn></RowTitlesColumn>`)
		for g, level := range m.Levels {
			b.WriteString(`<YColumn Width="140" Subcolumns="1"><Title>` + esc(level) + `</Title><Subcolumn>`)
			for _, r := range t.Rows {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				c := r.Cells[g]
				if c.State == "observed" {
					if e := Number(string(c.Value)); e != nil {
						return nil, e
					}
					b.WriteString(`<d>` + string(c.Value) + `</d>`)
				} else if c.State == "missing" || c.State == "structural-empty" {
					b.WriteString(`<d/>`)
				} else {
					return nil, errors.New("invalid cell state")
				}
			}
			b.WriteString(`</Subcolumn></YColumn>`)
		}
		b.WriteString(`</Table>`)
		if b.Len() > MaxBytes {
			return nil, errors.New("Prism XML exceeds 32 MiB")
		}
	}
	b.WriteString(`</GraphPadPrismFile>`)
	if b.Len() > MaxBytes {
		return nil, errors.New("Prism XML exceeds 32 MiB")
	}
	if err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

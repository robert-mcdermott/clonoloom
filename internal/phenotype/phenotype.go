// Package phenotype links supplied expression annotations to sample-scoped clonotypes.
// Sizes and contingency counts are descriptive, not tests of biological replication.
// See docs/PHENOTYPES.md for populations, missingness and method references.
package phenotype

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
)

const Version = 1
const MaxCells = 100000
const MaxMapPoints = 20000

type Options struct {
	Population  string `json:"population"`
	Breakpoints []int  `json:"breakpoints"`
	Annotation  string `json:"annotation"`
}

func (o Options) Normalize() (Options, error) {
	if o.Population == "" {
		o.Population = "receptor"
	}
	if o.Population != "receptor" && o.Population != "mapped" {
		return o, errors.New("size population must be receptor or mapped")
	}
	if len(o.Breakpoints) == 0 {
		o.Breakpoints = []int{1, 2, 5}
	}
	if len(o.Breakpoints) > 5 {
		return o, errors.New("choose at most five size breakpoints")
	}
	previous := 0
	for _, b := range o.Breakpoints {
		if b <= previous || b > 10000000 {
			return o, errors.New("size breakpoints must be increasing positive integers up to 10000000")
		}
		previous = b
	}
	return o, nil
}
func (o Options) Bin(n int) string {
	lo := 1
	for _, hi := range o.Breakpoints {
		if n <= hi {
			if lo == hi {
				return fmt.Sprint(hi)
			}
			return fmt.Sprintf("%d–%d", lo, hi)
		}
		lo = hi + 1
	}
	return fmt.Sprintf(">%d", lo-1)
}

type Cell struct {
	Barcode    string      `json:"barcode"`
	Values     []string    `json:"values"`
	Position   *[2]float64 `json:"position"`
	Status     string      `json:"status"`
	Chains     string      `json:"chains"`
	Clone      string      `json:"clone"`
	Candidates []string    `json:"candidates"`
	Size       *int        `json:"size"`
	Bin        string      `json:"bin"`
}
type Clone struct {
	ID         string `json:"id"`
	Receptor   int    `json:"receptor"`
	Expression int    `json:"expression"`
	Mapped     int    `json:"mapped"`
	Size       int    `json:"size"`
}
type Group struct {
	Value         string   `json:"value"`
	Cells         int      `json:"cells"`
	Mapped        int      `json:"mapped"`
	Assigned      int      `json:"assigned"`
	Sized         int      `json:"sized"`
	Expanded      int      `json:"expanded"`
	Clonotypes    int      `json:"clonotypes"`
	ExpandedShare *float64 `json:"expandedShare"`
}
type Association struct {
	Clone           string  `json:"clone"`
	Value           string  `json:"value"`
	Cells           int     `json:"cells"`
	Mapped          int     `json:"mapped"`
	CloneExpression int     `json:"cloneExpression"`
	GroupExpression int     `json:"groupExpression"`
	CloneShare      float64 `json:"cloneShare"`
	GroupShare      float64 `json:"groupShare"`
}
type Coverage struct {
	Receptor           int            `json:"receptor"`
	Eligible           int            `json:"eligible"` // unique clonotype assignment, after QC
	Matched            int            `json:"matched"`  // raw exact barcode matches, before QC
	ExpressionOnly     int            `json:"expressionOnly"`
	ReceptorOnly       int            `json:"receptorOnly"`
	MissingBarcodeRows int            `json:"missingBarcodeRows"`
	Mapped             int            `json:"mapped"`
	Assigned           int            `json:"assigned"` // in expression
	Sized              int            `json:"sized"`
	Status             map[string]int `json:"status"` // expression cells only
}
type Result struct {
	FilterID     string               `json:"filterId"`
	Options      Options              `json:"options"`
	DefinitionID string               `json:"definitionId"`
	Definition   clonotype.Definition `json:"definition"`
	Filter       qc.Filter            `json:"filter"`
	Sample       string               `json:"sample"`
	Columns      []string             `json:"columns"`
	Cells        []Cell               `json:"cells"`
	Clones       []Clone              `json:"clones"`
	Groups       []Group              `json:"groups"`
	Associations []Association        `json:"associations"`
	Coverage     Coverage             `json:"coverage"`
}

type evidence struct {
	kept       int
	productive int
	loci       map[string]int
	vendors    map[string]bool
	candidates map[string]bool
	clone      string
	status     string
}

// Chain status uses QC's productive contigs and receptor arms. Mixed families and unknown
// productive loci take precedence over multi (>2/locus), paired and single. This is evidence,
// not a cell-type call; the clonotype's primary/all-chain rule is applied separately.
func chainStatus(e *evidence) string {
	if e == nil {
		return "No receptor evidence"
	}
	if e.kept == 0 {
		return "QC excluded"
	}
	if e.productive == 0 {
		return "No productive chains"
	}
	families := 0
	paired, multi := false, false
	for _, arms := range [][2]int{{e.loci["TRA"], e.loci["TRB"]}, {e.loci["TRG"], e.loci["TRD"]}, {e.loci["IGH"], e.loci["IGK"] + e.loci["IGL"]}} {
		if arms[0]+arms[1] > 0 {
			families++
		}
		paired = paired || (arms[0] > 0 && arms[1] > 0)
	}
	for _, n := range e.loci {
		multi = multi || n > 2
	}
	if e.loci[""] > 0 || families == 0 {
		return "Unresolved locus"
	}
	if families > 1 {
		return "Mixed receptors"
	}
	if multi {
		return "Multi-chain (>2 per locus)"
	}
	if paired {
		return "Paired"
	}
	return "Single arm"
}

// Compute uses membership from the existing clonotype engine. A cell with multiple distinct
// memberships (each locus separately), or conflicting retained vendor IDs, is ambiguous.
// Such cells do not enter unique-clone sizes or phenotype association denominators.
func Compute(ctx context.Context, t *rep.Table, ct *clonotype.Table, filter qc.Filter, sample int, cells []Cell, columns []string, options Options) (*Result, error) {
	o, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	if t.Cell == nil || !ct.SingleCell {
		return nil, errors.New("clonotype overlays require single-cell receptor data")
	}
	if sample < 0 || sample >= len(t.Samples) {
		return nil, errors.New("choose a receptor sample")
	}
	if len(cells) > MaxCells {
		return nil, fmt.Errorf("overlays accept at most %d expression cells", MaxCells)
	}
	column := -1
	for i, name := range columns {
		if name == o.Annotation && o.Annotation != "" {
			column = i
		}
	}
	if o.Annotation != "" && column < 0 {
		return nil, errors.New("choose a supplied phenotype annotation")
	}
	keep, _ := qc.Apply(t, filter)
	byCell := map[string]*evidence{}
	r := &Result{Options: o, DefinitionID: ct.DefinitionID, Definition: ct.Definition, Filter: filter, Sample: t.Samples[sample].Name, Columns: columns, Cells: make([]Cell, 0, len(cells)), Clones: []Clone{}, Groups: []Group{}, Associations: []Association{}, Coverage: Coverage{Status: map[string]int{}}}
	for row, s := range t.Sample {
		if row%65536 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if int(s) != sample {
			continue
		}
		barcode := t.Dict.Get(t.Cell[row])
		if barcode == "" {
			r.Coverage.MissingBarcodeRows++
			continue
		}
		e := byCell[barcode]
		if e == nil {
			e = &evidence{loci: map[string]int{}, vendors: map[string]bool{}, candidates: map[string]bool{}}
			byCell[barcode] = e
		}
		if !keep[row] {
			continue
		}
		e.kept++
		if t.Productive[row] == rep.True {
			e.productive++
			e.loci[t.LocusName(row)]++
		}
		if t.Clone != nil && t.Clone[row] != 0 {
			e.vendors[t.Dict.Get(t.Clone[row])] = true
		}
	}
	for i, c := range ct.Clonotypes {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		for _, row := range ct.MembersOf(i) {
			if int(t.Sample[row]) == sample {
				if e := byCell[t.Dict.Get(t.Cell[row])]; e != nil {
					e.candidates[c.ID] = true
				}
			}
		}
	}
	clones := map[string]*Clone{}
	for _, e := range byCell {
		switch {
		case e.kept == 0:
			e.status = "QC excluded"
		case len(e.candidates) > 1 || (ct.Definition.Vendor && len(e.vendors) > 1):
			e.status = "Ambiguous clonotype"
		case len(e.candidates) == 0:
			e.status = "Unassigned clonotype"
		default:
			e.status = "Assigned"
			for id := range e.candidates {
				e.clone = id
			}
			r.Coverage.Eligible++
			c := clones[e.clone]
			if c == nil {
				c = &Clone{ID: e.clone}
				clones[e.clone] = c
			}
			c.Receptor++
		}
	}
	filterBytes, _ := json.Marshal(filter)
	filterSum := sha256.Sum256(filterBytes)
	r.FilterID = fmt.Sprintf("qc-%x", filterSum[:12])
	r.Coverage.Receptor = len(byCell)
	seen := map[string]bool{}
	groups := map[string]*Group{}
	groupClones := map[string]map[string]bool{}
	associations := map[[2]string]*Association{}
	for i, c := range cells {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if c.Barcode == "" || seen[c.Barcode] {
			return nil, errors.New("expression barcodes must be nonempty and unique")
		}
		seen[c.Barcode] = true
		if len(c.Values) != len(columns) {
			return nil, errors.New("expression annotation width differs from its columns")
		}
		c.Status = "No receptor evidence"
		c.Clone = ""
		c.Size = nil
		c.Bin = ""
		c.Candidates = []string{}
		e := byCell[c.Barcode]
		c.Chains = chainStatus(e)
		if e != nil {
			r.Coverage.Matched++
			c.Status = e.status
			c.Clone = e.clone
			for id := range e.candidates {
				c.Candidates = append(c.Candidates, id)
			}
			sort.Strings(c.Candidates)
		}
		if c.Position != nil {
			r.Coverage.Mapped++
		}
		if c.Clone != "" {
			cl := clones[c.Clone]
			cl.Expression++
			if c.Position != nil {
				cl.Mapped++
			}
			r.Coverage.Assigned++
		}
		r.Cells = append(r.Cells, c)
	}
	if r.Coverage.Mapped > MaxMapPoints {
		return nil, fmt.Errorf("overlays accept at most %d mapped cells; no cells were sampled", MaxMapPoints)
	}
	r.Coverage.ExpressionOnly = len(cells) - r.Coverage.Matched
	r.Coverage.ReceptorOnly = r.Coverage.Receptor - r.Coverage.Matched
	for _, cl := range clones {
		cl.Size = cl.Receptor
		if o.Population == "mapped" {
			cl.Size = cl.Mapped
		}
		r.Clones = append(r.Clones, *cl)
	}
	sort.Slice(r.Clones, func(i, j int) bool { return r.Clones[i].ID < r.Clones[j].ID })
	for i := range r.Cells {
		c := &r.Cells[i]
		value := ""
		if column >= 0 {
			value = c.Values[column]
		}
		g := groups[value]
		if g == nil {
			g = &Group{Value: value}
			groups[value] = g
			groupClones[value] = map[string]bool{}
		}
		g.Cells++
		if c.Position != nil {
			g.Mapped++
		}
		if c.Clone != "" {
			cl := clones[c.Clone]
			g.Assigned++
			groupClones[value][c.Clone] = true
			if cl.Size > 0 {
				n := cl.Size
				c.Size = &n
				c.Bin = o.Bin(n)
				g.Sized++
				r.Coverage.Sized++
				if n > 1 {
					g.Expanded++
				}
			} else {
				c.Status = "Outside size population"
			}
			key := [2]string{c.Clone, value}
			a := associations[key]
			if a == nil {
				a = &Association{Clone: c.Clone, Value: value}
				associations[key] = a
			}
			a.Cells++
			if c.Position != nil {
				a.Mapped++
			}
		}
		r.Coverage.Status[c.Status]++
	}
	for value, g := range groups {
		g.Clonotypes = len(groupClones[value])
		if g.Sized > 0 {
			f := float64(g.Expanded) / float64(g.Sized)
			g.ExpandedShare = &f
		}
		r.Groups = append(r.Groups, *g)
	}
	sort.Slice(r.Groups, func(i, j int) bool { return r.Groups[i].Value < r.Groups[j].Value })
	for _, a := range associations {
		a.CloneExpression = clones[a.Clone].Expression
		a.GroupExpression = groups[a.Value].Cells
		a.CloneShare = float64(a.Cells) / float64(a.CloneExpression)
		a.GroupShare = float64(a.Cells) / float64(a.GroupExpression)
		r.Associations = append(r.Associations, *a)
	}
	sort.Slice(r.Associations, func(i, j int) bool {
		a, b := r.Associations[i], r.Associations[j]
		if a.Clone != b.Clone {
			return a.Clone < b.Clone
		}
		return a.Value < b.Value
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

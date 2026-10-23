package main

// Clonotypes (clonoloom-spec/requirements.md, C1–C3, C6): a dataset's clonotypes under a
// definition and the dataset's filter (internal/clonotype), served to the page as sorted, searched
// pages of the table, and one clonotype in full. A table is computed once per dataset, definition
// and filter, and kept while it is used; its sorted orders are kept with it.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/invariant"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
)

// clonotypeTables keeps the tables lately computed, the most recently used last.
type clonotypeTables struct {
	mu      sync.Mutex
	entries []*clonotypeEntry
}

type clonotypeEntry struct {
	key        string
	table      *clonotype.Table
	invariants map[string]*invariant.Result
	orders     map[string][]int32
	mu         sync.Mutex
}

const keptTables = 6

func (c *clonotypeTables) get(key string, make func() (*clonotype.Table, error)) (*clonotypeEntry, error) {
	c.mu.Lock()
	for i, e := range c.entries {
		if e.key == key {
			c.entries = append(append(c.entries[:i:i], c.entries[i+1:]...), e)
			c.mu.Unlock()
			return e, nil
		}
	}
	c.mu.Unlock()
	t, err := make()
	if err != nil {
		return nil, err
	}
	e := &clonotypeEntry{key: key, table: t, orders: map[string][]int32{}}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, e)
	if len(c.entries) > keptTables {
		c.entries = c.entries[len(c.entries)-keptTables:]
	}
	return e, nil
}

// forget drops a dataset's tables (when it is closed).
func (c *clonotypeTables) forget(dataset string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.entries[:0]
	for _, e := range c.entries {
		if !strings.HasPrefix(e.key, dataset+"\n") {
			kept = append(kept, e)
		}
	}
	c.entries = kept
}

// order is the table's clonotypes sorted by a key: total (the default), samples, junction, length,
// v, j, id, or sample:<index>; ties broken by the clonotype's place in the table (its key's order).
func (e *clonotypeEntry) order(by string, desc bool) ([]int32, error) {
	name := fmt.Sprintf("%s|%v", by, desc)
	e.mu.Lock()
	defer e.mu.Unlock()
	if o, ok := e.orders[name]; ok {
		return o, nil
	}
	t := e.table
	n := len(t.Clonotypes)
	o := make([]int32, n)
	for i := range o {
		o[i] = int32(i)
	}
	first := func(i int32) clonotype.Chain {
		if c := t.Clonotypes[i].Chains; len(c) > 0 {
			return c[0]
		}
		return clonotype.Chain{}
	}
	var less func(a, b int32) bool
	switch {
	case by == "" || by == "total":
		less = func(a, b int32) bool { return t.Clonotypes[a].Total < t.Clonotypes[b].Total }
	case by == "samples":
		less = func(a, b int32) bool { return t.Clonotypes[a].Samples < t.Clonotypes[b].Samples }
	case by == "junction":
		less = func(a, b int32) bool { return t.Clonotypes[a].Junctions() < t.Clonotypes[b].Junctions() }
	case by == "length":
		less = func(a, b int32) bool { return len(first(a).Junction) < len(first(b).Junction) }
	case by == "v":
		less = func(a, b int32) bool { return first(a).V < first(b).V }
	case by == "j":
		less = func(a, b int32) bool { return first(a).J < first(b).J }
	case by == "id":
		less = func(a, b int32) bool { return t.Clonotypes[a].ID < t.Clonotypes[b].ID }
	case strings.HasPrefix(by, "sample:"):
		s, err := strconv.Atoi(strings.TrimPrefix(by, "sample:"))
		if err != nil || s < 0 || s >= len(t.SampleTotals) {
			return nil, fmt.Errorf("no sample %q to sort by", by)
		}
		values := make([]float64, n)
		for i := range values {
			values[i] = t.In(i, uint32(s))
		}
		less = func(a, b int32) bool { return values[a] < values[b] }
	default:
		return nil, fmt.Errorf("clonotypes are not sorted by %q", by)
	}
	sort.SliceStable(o, func(x, y int) bool {
		a, b := o[x], o[y]
		if desc {
			if less(b, a) {
				return true
			}
			if less(a, b) {
				return false
			}
		} else {
			if less(a, b) {
				return true
			}
			if less(b, a) {
				return false
			}
		}
		return a < b
	})
	e.orders[name] = o
	return o, nil
}

// clonotypeQuery narrows the table: text in a junction, gene, ID or the source's clonotype ID
// (without regard to case), a locus, a minimum total, presence in a sample.
type clonotypeQuery struct {
	Text     string  `json:"text"`
	Locus    string  `json:"locus"`
	MinTotal float64 `json:"minTotal"`
	Sample   *int    `json:"sample"`
}

func (q clonotypeQuery) empty() bool {
	return strings.TrimSpace(q.Text) == "" && q.Locus == "" && q.MinTotal <= 0 && q.Sample == nil
}

func (q clonotypeQuery) matches(t *clonotype.Table, i int) bool {
	c := t.Clonotypes[i]
	if q.MinTotal > 0 && c.Total < q.MinTotal {
		return false
	}
	if q.Locus != "" {
		ok := false
		for _, ch := range c.Chains {
			ok = ok || ch.Locus == q.Locus
		}
		if !ok {
			return false
		}
	}
	if q.Sample != nil && t.In(i, uint32(*q.Sample)) == 0 {
		return false
	}
	if text := strings.ToUpper(strings.TrimSpace(q.Text)); text != "" {
		hit := strings.Contains(strings.ToUpper(c.ID), text) || strings.Contains(strings.ToUpper(c.Vendor), text)
		for _, ch := range c.Chains {
			hit = hit || strings.Contains(ch.Junction, text) || strings.Contains(strings.ToUpper(ch.V), text) || strings.Contains(strings.ToUpper(ch.J), text)
		}
		if !hit {
			return false
		}
	}
	return true
}

type clonotypeRequest struct {
	Species    string               `json:"species"`
	Population string               `json:"population"`
	Dataset    string               `json:"dataset"`
	Definition clonotype.Definition `json:"definition"`
	Filter     qc.Filter            `json:"filter"`
	Sort       string               `json:"sort"`
	Desc       *bool                `json:"desc"`
	Query      clonotypeQuery       `json:"query"`
	Offset     int                  `json:"offset"`
	Limit      int                  `json:"limit"`
	ID         string               `json:"id"`
}

type clonotypeRow struct {
	Invariant invariant.Flags    `json:"invariant"`
	Index     int                `json:"index"`
	Rank      int                `json:"rank"`
	ID        string             `json:"id"`
	Chains    []clonotype.Chain  `json:"chains"`
	Vendor    string             `json:"vendor,omitempty"`
	Total     float64            `json:"total"`
	Frequency float64            `json:"frequency"`
	Samples   int                `json:"samples"`
	In        map[string]float64 `json:"in"`
}

type sampleTotal struct {
	Name  string  `json:"name"`
	Total float64 `json:"total"`
}

type clonotypePage struct {
	Invariant    *invariant.Result    `json:"invariant,omitempty"`
	DefinitionID string               `json:"definitionId"`
	Definition   clonotype.Definition `json:"definition"`
	Describe     string               `json:"describe"`
	Unit         rep.Unit             `json:"unit"`
	SingleCell   bool                 `json:"singleCell"`
	Samples      []sampleTotal        `json:"samples"`
	Count        int                  `json:"count"`
	Matched      int                  `json:"matched"`
	Cells        int                  `json:"cells,omitempty"`
	Excluded     map[string]int       `json:"excluded"`
	Rows         []clonotypeRow       `json:"rows"`
}

func (a *app) registerClonotypes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/clonotypes", a.handleClonotypes)
	mux.HandleFunc("POST /api/clonotypes/one", a.handleClonotype)
	mux.HandleFunc("GET /api/clonotypes/presets", func(w http.ResponseWriter, r *http.Request) {
		out := make([]map[string]any, len(clonotype.Presets))
		for i, p := range clonotype.Presets {
			n, _ := p.Normalize()
			out[i] = map[string]any{"definition": n, "id": n.ID(), "describe": n.Describe()}
		}
		writeJSON(w, map[string]any{"presets": out})
	})
}

// clonotypeTable answers the request's table, computing it the first time.
func (a *app) clonotypeTable(req clonotypeRequest) (*dataset, *clonotypeEntry, int, error) {
	ds, ok := a.datasets.get(req.Dataset)
	if !ok {
		return nil, nil, http.StatusNotFound, fmt.Errorf("the dataset is not open: open it again from its files")
	}
	if !invariant.ValidSpecies(req.Species) {
		return nil, nil, http.StatusBadRequest, fmt.Errorf("invariant species: human, mouse, or empty")
	}
	if req.Population != "" && req.Species == "" {
		return nil, nil, http.StatusBadRequest, fmt.Errorf("choose a species before filtering invariant populations")
	}
	if req.Population != "" && req.Population != "MAIT" && req.Population != "iNKT" && req.Population != "GEM" {
		return nil, nil, http.StatusBadRequest, fmt.Errorf("population: MAIT, iNKT, GEM, or empty")
	}
	def, err := req.Definition.Normalize()
	if err != nil {
		return nil, nil, http.StatusBadRequest, err
	}
	filter, _ := json.Marshal(req.Filter)
	key := ds.ID + "\n" + def.Canonical() + "\n" + string(filter)
	entry, err := a.clonotypes.get(key, func() (*clonotype.Table, error) {
		keep, _ := qc.Apply(ds.Result.Table, req.Filter)
		return clonotype.Compute(ds.Result.Table, keep, def)
	})
	if err != nil {
		return nil, nil, http.StatusUnprocessableEntity, err
	}
	return ds, entry, 0, nil
}

func (a *app) handleClonotypes(w http.ResponseWriter, r *http.Request) {
	var req clonotypeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Send {\"dataset\", \"definition\", \"filter\", \"sort\", \"query\", \"offset\", \"limit\"}.")
		return
	}
	ds, entry, status, err := a.clonotypeTable(req)
	if err != nil {
		code := ""
		if status == http.StatusNotFound {
			code = "not-open"
		}
		writeErrorCode(w, status, code, err.Error())
		return
	}
	desc := req.Desc == nil || *req.Desc
	order, err := entry.order(req.Sort, desc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	t := entry.table
	var inv *invariant.Result
	if req.Species != "" {
		inv = entry.invariant(req.Species)
	}
	matched := order
	if !req.Query.empty() || req.Population != "" {
		matched = make([]int32, 0, 1024)
		for _, i := range order {
			populationMatch := req.Population == ""
			if inv != nil {
				for p, name := range invariant.Populations {
					populationMatch = populationMatch || (req.Population == name && inv.Flags[i][p] == invariant.Match)
				}
			}
			if populationMatch && req.Query.matches(t, int(i)) {
				matched = append(matched, i)
			}
		}
	}
	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	offset := max(0, req.Offset)
	page := clonotypePage{DefinitionID: t.DefinitionID, Definition: t.Definition, Describe: t.Definition.Describe(), Unit: t.Unit, SingleCell: t.SingleCell,
		Count: len(t.Clonotypes), Matched: len(matched), Cells: t.Cells, Excluded: t.Excluded, Rows: []clonotypeRow{}}
	var grand float64
	for i, s := range ds.Result.Table.Samples {
		page.Samples = append(page.Samples, sampleTotal{Name: s.Name, Total: t.SampleTotals[i]})
		grand += t.SampleTotals[i]
	}
	for k := offset; k < len(matched) && k < offset+limit; k++ {
		i := int(matched[k])
		c := t.Clonotypes[i]
		row := clonotypeRow{Invariant: invariant.Flags{invariant.Unknown, invariant.Unknown, invariant.Unknown}, Index: i, Rank: k + 1, ID: c.ID, Chains: c.Chains, Vendor: c.Vendor, Total: c.Total, Samples: c.Samples, In: map[string]float64{}}
		if inv != nil {
			row.Invariant = inv.Flags[i]
		}
		if grand > 0 {
			row.Frequency = c.Total / grand
		}
		samples, values := t.Abundances(i)
		for j, s := range samples {
			row.In[strconv.Itoa(int(s))] = values[j]
		}
		page.Rows = append(page.Rows, row)
	}
	if req.Species != "" {
		summary := *inv
		summary.Flags = nil
		page.Invariant = &summary
	}
	writeJSON(w, page)
}

type memberRow struct {
	Row        int     `json:"row"`
	Source     int     `json:"source"`
	Line       uint32  `json:"line"`
	Sample     string  `json:"sample"`
	SequenceID string  `json:"sequenceId,omitempty"`
	Cell       string  `json:"cell,omitempty"`
	Locus      string  `json:"locus,omitempty"`
	V          string  `json:"v,omitempty"`
	D          string  `json:"d,omitempty"`
	J          string  `json:"j,omitempty"`
	C          string  `json:"c,omitempty"`
	Junction   string  `json:"junction,omitempty"`
	JunctionAA string  `json:"junctionAa,omitempty"`
	Count      float64 `json:"count"`
	Reads      float64 `json:"reads,omitempty"`
	UMIs       float64 `json:"umis,omitempty"`
}

// handleClonotype answers one clonotype ({"id"} with the table's request): its chains, its
// abundance and frequency in each sample, and its members (the first 500).
func (a *app) handleClonotype(w http.ResponseWriter, r *http.Request) {
	var req clonotypeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "Send the table's request with the clonotype's \"id\".")
		return
	}
	ds, entry, status, err := a.clonotypeTable(req)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	t, rt := entry.table, ds.Result.Table
	index := -1
	for i, c := range t.Clonotypes {
		if c.ID == req.ID {
			index = i
			break
		}
	}
	if index < 0 {
		writeError(w, http.StatusNotFound, "No such clonotype under this definition.")
		return
	}
	c := t.Clonotypes[index]
	type inSample struct {
		Sample    string  `json:"sample"`
		Abundance float64 `json:"abundance"`
		Frequency float64 `json:"frequency"`
	}
	var in []inSample
	samples, values := t.Abundances(index)
	for k, s := range samples {
		f := 0.0
		if t.SampleTotals[s] > 0 {
			f = values[k] / t.SampleTotals[s]
		}
		in = append(in, inSample{Sample: rt.Samples[s].Name, Abundance: values[k], Frequency: f})
	}
	members := t.MembersOf(index)
	rows := make([]memberRow, 0, min(len(members), 500))
	for _, m := range members[:min(len(members), 500)] {
		i := int(m)
		mr := memberRow{Row: i, Source: int(rt.Source[i]), Line: rt.Line[i], Sample: rt.Samples[rt.Sample[i]].Name, SequenceID: rt.IDs.Get(rt.SequenceID[i]), Locus: rt.LocusName(i),
			V: rt.Dict.Get(rt.V[i]), D: rt.Dict.Get(rt.D[i]), J: rt.Dict.Get(rt.J[i]), C: rt.Dict.Get(rt.C[i]), Junction: rt.Junctions.Get(rt.Junction[i]), JunctionAA: rt.JunctionsAA.Get(rt.JunctionAA[i]), Count: finiteOr0(rt.Count[i])}
		if rt.Cell != nil {
			mr.Cell = rt.Dict.Get(rt.Cell[i])
		}
		if rt.Reads != nil {
			mr.Reads = finiteOr0(rt.Reads[i])
		}
		if rt.UMIs != nil {
			mr.UMIs = finiteOr0(rt.UMIs[i])
		}
		rows = append(rows, mr)
	}
	writeJSON(w, map[string]any{"invariant": invariant.Classify(c, t.Definition, req.Species), "species": req.Species, "clonotype": c, "describe": t.Definition.Describe(), "unit": t.Unit, "in": in, "members": rows, "memberCount": len(members)})
}

func finiteOr0(v float64) float64 {
	if v != v {
		return 0
	}
	return v
}

// The annotation shares the table's immutable lifetime and is computed once per species.
func (e *clonotypeEntry) invariant(species string) *invariant.Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.invariants == nil {
		e.invariants = map[string]*invariant.Result{}
	}
	if r := e.invariants[species]; r != nil {
		return r
	}
	r, _ := invariant.Compute(e.table, species)
	e.invariants[species] = r
	return r
}

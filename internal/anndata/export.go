// Package anndata exports the documented AnnData encodings, not an inferred biological model.
// https://anndata.readthedocs.io/en/stable/fileformat-prose.html
// Exact joins, raw-count semantics and namespaced annotations are ClonoLoom policies.
package anndata

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"clonoloom/internal/expression"
	"clonoloom/internal/h5write"
	"clonoloom/internal/phenotype"
)

const Version = 1
const MaxEntries = 2000000

// Export keeps all expression observations and modalities, including cells excluded from PCA.
// Metadata contains JSON documents (parents, method, receptor definitions and source notices).
// It is deliberately not a scirpy obsm['airr'] encoding: assignments are obs columns and their
// chain definitions and ambiguity candidates are explicit, versioned JSON in uns/clonoloom.
func Export(ctx context.Context, in *expression.Input, p *phenotype.Result, analysis *expression.AnalysisResult, metadata map[string]string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if in == nil || in.Matrix == nil || p == nil {
		return nil, fmt.Errorf("AnnData needs expression counts and saved phenotypes")
	}
	m := in.Matrix
	n := len(m.Barcodes)
	if n == 0 || n > phenotype.MaxCells || len(m.Features) == 0 || len(m.Features) > expression.MaxFeatures || len(m.Counts) > MaxEntries {
		return nil, fmt.Errorf("AnnData export accepts 1–100000 cells, 1–200000 features and at most 2000000 sparse entries")
	}
	if len(p.Columns) > 32 || n*(len(p.Columns)+8) > 4000000 || analysis != nil && (analysis.Components > 1000 || analysis.Components < 1 || n*(analysis.Components+4) > 16000000) {
		return nil, fmt.Errorf("AnnData export exceeds 32 supplied observation columns, 4000000 observation values or 16000000 embedding values")
	}
	if len(m.Indptr) != n+1 || len(m.Indices) != len(m.Counts) || m.Indptr[0] != 0 || m.Indptr[n] != len(m.Counts) || len(p.Cells) != n {
		return nil, fmt.Errorf("AnnData matrix or phenotype dimensions differ")
	}
	if in.AnnData != nil && (len(in.AnnData.VariableColumns) > 126 || len(m.Features)*(len(in.AnnData.VariableColumns)+2) > 4000000) {
		return nil, fmt.Errorf("AnnData export exceeds 126 supplied variable columns or 4000000 variable annotation values")
	}
	cells := map[string]phenotype.Cell{}
	for _, c := range p.Cells {
		if _, ok := cells[c.Barcode]; ok || c.Barcode == "" || len(c.Values) != len(p.Columns) {
			return nil, fmt.Errorf("duplicate barcode or annotation dimensions in phenotypes")
		}
		cells[c.Barcode] = c
	}
	seen := map[string]bool{}
	ptr := make([]int64, n+1)
	indices := make([]int32, len(m.Indices))
	counts := make([]int64, len(m.Counts))
	for i, b := range m.Barcodes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if b == "" || seen[b] {
			return nil, fmt.Errorf("invalid expression barcode")
		}
		seen[b] = true
		if _, ok := cells[b]; !ok {
			return nil, fmt.Errorf("phenotype barcode does not match expression input")
		}
		a, z := m.Indptr[i], m.Indptr[i+1]
		if a < 0 || z < a || z > len(counts) {
			return nil, fmt.Errorf("invalid sparse row pointers")
		}
		ptr[i+1] = int64(z)
		for j := a; j < z; j++ {
			if int(m.Indices[j]) >= len(m.Features) || j > a && m.Indices[j] <= m.Indices[j-1] || m.Counts[j] > expression.MaxCount {
				return nil, fmt.Errorf("invalid sparse index or count")
			}
			indices[j] = int32(m.Indices[j])
			counts[j] = int64(m.Counts[j])
		}
	}
	f := h5write.NewFile()
	w := &builder{ctx: ctx}
	root := f.Root
	encode(root, "anndata", "0.1.0")
	x := w.group(root, "X", "csr_matrix", "0.1.0")
	x.AttrInt64s("shape", []int64{int64(n), int64(len(m.Features))})
	w.check(x.Int64s("indptr", ptr))
	w.check(x.Int32s("indices", indices))
	w.check(x.Int64s("data", counts))
	obs := w.frame(root, "obs", m.Barcodes)
	var order []string
	text := func(name string, vals []string) { order = append(order, name); w.category(obs, name, vals) }
	column := func(fn func(phenotype.Cell) string) []string {
		v := make([]string, n)
		for i, b := range m.Barcodes {
			v[i] = fn(cells[b])
		}
		return v
	}
	text("clonoloom_clone_id", column(func(c phenotype.Cell) string { return c.Clone }))
	text("clonoloom_assignment", column(func(c phenotype.Cell) string { return c.Status }))
	text("clonoloom_chain_status", column(func(c phenotype.Cell) string { return c.Chains }))
	text("clonoloom_size_category", column(func(c phenotype.Cell) string { return c.Bin }))
	text("clonoloom_sample", column(func(c phenotype.Cell) string { return p.Sample }))
	sizes := make([]float64, n)
	for i, b := range m.Barcodes {
		sizes[i] = math.NaN()
		if s := cells[b].Size; s != nil {
			if *s < 0 || uint64(*s) > expression.MaxCount {
				return nil, fmt.Errorf("invalid clone size")
			}
			sizes[i] = float64(*s)
		}
	}
	order = append(order, "clonoloom_clone_size")
	w.floats(obs, "clonoloom_clone_size", []uint64{uint64(n)}, sizes)
	annotationNames := map[string]string{}
	for j, name := range p.Columns {
		key := fmt.Sprintf("annotation_%d", j)
		annotationNames[key] = name
		text(key, column(func(c phenotype.Cell) string { return c.Values[j] }))
	}
	vars := make([]string, len(m.Features))
	names := make([]string, len(vars))
	types := make([]string, len(vars))
	seen = map[string]bool{}
	for i, feature := range m.Features {
		if feature.ID == "" || seen[feature.ID] {
			return nil, fmt.Errorf("invalid feature ID")
		}
		seen[feature.ID] = true
		vars[i] = feature.ID
		names[i] = feature.Name
		types[i] = feature.Type
	}
	vf := w.frame(root, "var", vars)
	w.strings(vf, "feature_name", names)
	w.category(vf, "feature_type", types)
	vorder := []string{"feature_name", "feature_type"}
	variableNames := map[string]string{}
	if in.AnnData != nil {
		for j, name := range in.AnnData.VariableColumns {
			key := fmt.Sprintf("annotation_%d", j)
			variableNames[key] = name
			values := make([]string, len(vars))
			if len(in.AnnData.VariableValues) != len(vars) {
				return nil, fmt.Errorf("variable annotation dimensions differ")
			}
			for i, row := range in.AnnData.VariableValues {
				if len(row) != len(in.AnnData.VariableColumns) {
					return nil, fmt.Errorf("variable annotation dimensions differ")
				}
				values[i] = row[j]
			}
			w.category(vf, key, values)
			vorder = append(vorder, key)
		}
	}
	vf.AttrStrings("column-order", vorder)
	obsm := w.group(root, "obsm", "dict", "0.1.0")
	// Source coordinates and computed coordinates remain distinct. All missing rows are NaN.
	supplied := missing(n, 2)
	hasSupplied := false
	if in.Cells != nil {
		for i, b := range m.Barcodes {
			if at, ok := in.Cells.ByBarcode[b]; ok {
				if at < 0 || at >= len(in.Cells.Rows) || in.Cells.Rows[at].Barcode != b {
					return nil, fmt.Errorf("invalid annotation index")
				}
				if pos := in.Cells.Rows[at].Position; pos != nil {
					if !finite(pos[:]) {
						return nil, fmt.Errorf("nonfinite supplied position")
					}
					copy(supplied[i*2:], pos[:])
					hasSupplied = true
				}
			}
		}
	}
	if analysis == nil {
		for i, b := range m.Barcodes {
			pos := cells[b].Position
			has := !math.IsNaN(supplied[i*2])
			if has != (pos != nil) || has && (*pos != [2]float64{supplied[i*2], supplied[i*2+1]}) {
				return nil, fmt.Errorf("saved supplied coordinates differ from the expression source")
			}
		}
	}
	if hasSupplied {
		w.floats(obsm, "X_supplied", []uint64{uint64(n), 2}, supplied)
	}
	if analysis != nil {
		if analysis.Components < 1 || analysis.Components > 1000 || analysis.InputCells != n {
			return nil, fmt.Errorf("invalid expression map dimensions")
		}
		barcodeSet := map[string]bool{}
		for _, b := range m.Barcodes {
			barcodeSet[b] = true
		}
		byBarcode := map[string]expression.AnalysisCell{}
		for _, c := range analysis.Cells {
			if _, ok := byBarcode[c.Barcode]; ok || !barcodeSet[c.Barcode] {
				return nil, fmt.Errorf("invalid map barcode")
			}
			byBarcode[c.Barcode] = c
		}
		pcs := missing(n, analysis.Components)
		umap := missing(n, 2)
		retained := make([]bool, n)
		reasons := make([]string, n)
		excluded := map[string]string{}
		for _, c := range analysis.Excluded {
			if _, ok := byBarcode[c.Barcode]; ok || excluded[c.Barcode] != "" || c.Reason == "" {
				return nil, fmt.Errorf("invalid map exclusion")
			}
			excluded[c.Barcode] = c.Reason
		}
		if len(byBarcode)+len(excluded) != n {
			return nil, fmt.Errorf("map population differs from input")
		}
		for i, b := range m.Barcodes {
			c, ok := byBarcode[b]
			if !ok {
				reasons[i] = excluded[b]
				if reasons[i] == "" {
					return nil, fmt.Errorf("map exclusion missing")
				}
				if cells[b].Position != nil {
					return nil, fmt.Errorf("excluded cell has computed coordinates")
				}
				continue
			}
			if len(c.Scores) != analysis.Components || !finite(c.Scores) || c.Position == nil || !finite(c.Position[:]) || cells[b].Position == nil || *cells[b].Position != *c.Position {
				return nil, fmt.Errorf("computed map or phenotype coordinates differ")
			}
			retained[i] = true
			copy(pcs[i*analysis.Components:], c.Scores)
			copy(umap[i*2:], c.Position[:])
		}
		w.floats(obsm, "X_clonoloom_pca", []uint64{uint64(n), uint64(analysis.Components)}, pcs)
		w.floats(obsm, "X_clonoloom_umap", []uint64{uint64(n), 2}, umap)
		order = append(order, "clonoloom_map_retained")
		w.check(obs.Bools("clonoloom_map_retained", retained))
		w.check(obs.DatasetEncoding("clonoloom_map_retained", "array", "0.2.0"))
		text("clonoloom_map_exclusion", reasons)
	}
	obs.AttrStrings("column-order", order)
	for _, key := range []string{"layers", "obsp", "varm", "varp"} {
		w.group(root, key, "dict", "0.1.0")
	}
	uns := w.group(w.group(root, "uns", "dict", "0.1.0"), "clonoloom", "dict", "0.1.0")
	meta := map[string]string{}
	for k, v := range metadata {
		meta[k] = v
	}
	ann, _ := json.Marshal(annotationNames)
	vann, _ := json.Marshal(variableNames)
	meta["observation_columns_json"] = string(ann)
	meta["variable_columns_json"] = string(vann)
	meta["schema"] = "clonoloom-anndata-1"
	meta["matrix_semantics"] = "X contains exact raw integer counts for every input cell and feature modality, without normalization or imputation. Missing embeddings are NaN. No measured-zero row is invented for absent barcodes."
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		w.check(uns.ScalarString(k, meta[k]))
		w.check(uns.DatasetEncoding(k, "string", "0.2.0"))
	}
	if w.err != nil {
		return nil, w.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw := f.Bytes()
	if len(raw) > 512<<20 {
		return nil, fmt.Errorf("AnnData export exceeds 512 MiB")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}
func finite(v []float64) bool {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}
func missing(n, d int) []float64 {
	v := make([]float64, n*d)
	for i := range v {
		v[i] = math.NaN()
	}
	return v
}
func encode(g *h5write.Group, kind, version string) {
	g.AttrString("encoding-type", kind)
	g.AttrString("encoding-version", version)
}

type builder struct {
	ctx context.Context
	err error
}

func (w *builder) check(err error) {
	if w.err == nil {
		w.err = err
	}
	if w.err == nil {
		w.err = w.ctx.Err()
	}
}
func (w *builder) group(g *h5write.Group, name, kind, version string) *h5write.Group {
	x, e := g.Group(name)
	w.check(e)
	if x == nil {
		x = h5write.NewFile().Root
	}
	encode(x, kind, version)
	return x
}
func (w *builder) strings(g *h5write.Group, name string, v []string) {
	w.check(g.Strings(name, v))
	w.check(g.DatasetEncoding(name, "string-array", "0.2.0"))
}
func (w *builder) floats(g *h5write.Group, name string, dims []uint64, v []float64) {
	w.check(g.Float64s(name, dims, v))
	w.check(g.DatasetEncoding(name, "array", "0.2.0"))
}
func (w *builder) frame(g *h5write.Group, name string, index []string) *h5write.Group {
	f := w.group(g, name, "dataframe", "0.2.0")
	f.AttrString("_index", "_index")
	w.strings(f, "_index", index)
	return f
}
func (w *builder) category(g *h5write.Group, name string, v []string) {
	c := w.group(g, name, "categorical", "0.2.0")
	c.AttrBool("ordered", false)
	categories := []string{}
	codes := make([]int32, len(v))
	index := map[string]int32{}
	for i, s := range v {
		if s == "" {
			codes[i] = -1
			continue
		}
		code, ok := index[s]
		if !ok {
			code = int32(len(categories))
			index[s] = code
			categories = append(categories, s)
		}
		codes[i] = code
	}
	w.strings(c, "categories", categories)
	w.check(c.Int32s("codes", codes))
	w.check(c.DatasetEncoding("codes", "array", "0.2.0"))
}

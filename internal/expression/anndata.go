package expression

// AnnData encodings follow https://anndata.readthedocs.io/en/stable/fileformat-prose.html.
// MuData embeds AnnData under /mod: https://mudata.readthedocs.io/en/latest/io/output.html.
// Choosing count semantics, retaining exact barcodes and rejecting duplicate coordinates are
// ClonoLoom policies. X is never assumed to be counts and no normalized layer is chosen for you.
import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/hdf5"
)

type AnnDataSelection struct {
	Modality     string `json:"modality,omitempty"`
	Layer        string `json:"layer"`
	Embedding    string `json:"embedding,omitempty"`
	AIRRModality string `json:"airrModality,omitempty"`
	AIRRKey      string `json:"airrKey,omitempty"`
	FeatureName  string `json:"featureName,omitempty"`
	FeatureType  string `json:"featureType,omitempty"`
}
type EmbeddingSource struct {
	Path     string    `json:"path"`
	Axes     [2]string `json:"axes"`
	Supplied bool      `json:"supplied"`
}
type AnnDataEvidence struct {
	Selection        AnnDataSelection `json:"selection"`
	Format           string           `json:"format"`
	ObservationIndex string           `json:"observationIndex"`
	VariableIndex    string           `json:"variableIndex"`
	VariableColumns  []string         `json:"variableColumns"`
	VariableValues   [][]string       `json:"-"`
	Embedding        *EmbeddingSource `json:"embedding,omitempty"`
	AIRR             *AIRREvidence    `json:"airr,omitempty"`
}
type frame struct {
	indexName      string
	index, columns []string
	values         [][]string
}

func member(s string) bool {
	return validLabel(s) && s != "." && s != ".." && !strings.Contains(s, "/")
}
func attrText(f *hdf5.Reader, path, key string) (string, error) {
	v, e := f.Attribute(path, key)
	if e != nil {
		return "", e
	}
	if v.Kind != "string" || len(v.Shape) != 0 || len(v.Text) != 1 {
		return "", fmt.Errorf("%s: %s must be a scalar string", path, key)
	}
	return v.Text[0], nil
}
func encoding(f *hdf5.Reader, path string) (string, error) {
	typ, e := attrText(f, path, "encoding-type")
	if e != nil {
		return "", e
	}
	version, e := attrText(f, path, "encoding-version")
	if e != nil {
		return "", e
	}
	versions := map[string]string{"anndata": "0.1.0", "MuData": "0.1.0", "dataframe": "0.2.0", "array": "0.2.0", "string-array": "0.2.0", "csr_matrix": "0.1.0", "csc_matrix": "0.1.0", "categorical": "0.2.0", "nullable-integer": "0.1.0", "nullable-boolean": "0.1.0", "nullable-string-array": "0.1.0", "awkward-array": "0.1.0", "dict": "0.1.0"}
	if versions[typ] != version || version == "" {
		return "", fmt.Errorf("%s: unsupported AnnData encoding %q version %q", path, typ, version)
	}
	return typ, nil
}
func vectorValue(f *hdf5.Reader, path string, limit uint64) (hdf5.Value, error) {
	v, e := f.Value(path, limit)
	if e == nil && len(v.Shape) != 1 {
		e = fmt.Errorf("%s: expected a vector", path)
	}
	return v, e
}
func column(f *hdf5.Reader, path string, limit uint64) ([]string, error) {
	kind, e := encoding(f, path)
	if e != nil {
		return nil, e
	}
	switch kind {
	case "categorical":
		ordered, e := f.Attribute(path, "ordered")
		if e != nil {
			return nil, e
		}
		if ordered.Kind != "boolean" || len(ordered.Shape) != 0 {
			return nil, fmt.Errorf("categorical ordered must be a scalar boolean")
		}
		categories, e := column(f, path+"/categories", limit)
		if e != nil {
			return nil, e
		}
		codes, e := vectorValue(f, path+"/codes", limit)
		if e != nil {
			return nil, e
		}
		if codes.Kind != "integer" {
			return nil, fmt.Errorf("categorical codes must be integers")
		}
		out := make([]string, len(codes.Text))
		for i, x := range codes.Numbers {
			if x == -1 {
				continue
			}
			if x < 0 || x >= float64(len(categories)) {
				return nil, fmt.Errorf("categorical code outside categories")
			}
			out[i] = categories[int(x)]
		}
		return out, nil
	case "nullable-integer", "nullable-boolean", "nullable-string-array":
		raw, e := vectorValue(f, path+"/values", limit)
		if e != nil {
			return nil, e
		}
		expected := map[string]string{"nullable-integer": "integer", "nullable-boolean": "boolean", "nullable-string-array": "string"}[kind]
		if raw.Kind != expected {
			return nil, fmt.Errorf("nullable values datatype does not match encoding")
		}
		values, e := column(f, path+"/values", limit)
		if e != nil {
			return nil, e
		}
		mask, e := vectorValue(f, path+"/mask", limit)
		if e != nil {
			return nil, e
		}
		if mask.Kind != "boolean" || len(mask.Text) != len(values) {
			return nil, fmt.Errorf("nullable mask must be boolean and match values")
		}
		for i, x := range mask.Numbers {
			if x == 1 {
				values[i] = ""
			}
		}
		return values, nil
	case "array", "string-array":
		v, e := vectorValue(f, path, limit)
		if e != nil {
			return nil, e
		}
		if (kind == "string-array") != (v.Kind == "string") {
			return nil, fmt.Errorf("%s: annotation encoding/datatype mismatch", path)
		}
		if v.Kind == "float" {
			for i, x := range v.Numbers {
				if math.IsNaN(x) {
					v.Text[i] = ""
				} else if math.IsInf(x, 0) {
					return nil, fmt.Errorf("%s: infinite annotation", path)
				}
			}
		}
		return v.Text, nil
	default:
		return nil, fmt.Errorf("%s: unsupported annotation encoding %q", path, kind)
	}
}
func readFrame(f *hdf5.Reader, path string, limit, maxColumns int) (*frame, error) {
	kind, e := encoding(f, path)
	if e != nil {
		return nil, e
	}
	if kind != "dataframe" {
		return nil, fmt.Errorf("%s: expected dataframe", path)
	}
	index, e := attrText(f, path, "_index")
	if e != nil {
		return nil, e
	}
	if !member(index) {
		return nil, fmt.Errorf("invalid dataframe index name")
	}
	names, e := f.Attribute(path, "column-order")
	if e != nil {
		return nil, e
	}
	if (names.Kind != "string" && len(names.Text) != 0) || len(names.Shape) != 1 || len(names.Text) > maxColumns {
		return nil, fmt.Errorf("%s: column-order must have at most %d columns", path, maxColumns)
	}
	ids, e := column(f, path+"/"+index, uint64(limit))
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	total := 0
	for _, s := range ids {
		if !validLabel(s) || seen[s] {
			return nil, fmt.Errorf("%s: invalid or duplicate index", path)
		}
		seen[s] = true
		total += len(s)
	}
	if len(ids)*len(names.Text) > 4000000 {
		return nil, fmt.Errorf("dataframe exceeds 4000000 annotation values")
	}
	out := &frame{indexName: index, index: ids, columns: names.Text, values: make([][]string, len(ids))}
	for i := range out.values {
		out.values[i] = make([]string, len(names.Text))
	}
	seen = map[string]bool{index: true}
	for j, name := range names.Text {
		if !member(name) || seen[name] {
			return nil, fmt.Errorf("invalid/duplicate dataframe column")
		}
		seen[name] = true
		vals, e := column(f, path+"/"+name, uint64(limit))
		if e != nil {
			return nil, e
		}
		if len(vals) != len(ids) {
			return nil, fmt.Errorf("%s: column length differs from index", path)
		}
		for i, s := range vals {
			if s != "" && !validLabel(s) {
				return nil, fmt.Errorf("%s: invalid annotation", path)
			}
			total += len(s)
			if total > maxLabelBytes {
				return nil, fmt.Errorf("dataframe exceeds 64 MiB labels")
			}
			out.values[i][j] = s
		}
	}
	return out, nil
}
func ReadAnnData(ctx context.Context, r io.ReaderAt, size int64, selection AnnDataSelection) (*Input, error) {
	f, e := hdf5.Open(ctx, r, size)
	if e != nil {
		return nil, e
	}
	kind, e := encoding(f, "/")
	if e != nil {
		return nil, e
	}
	base := ""
	switch kind {
	case "anndata":
		if selection.Modality != "" || selection.AIRRModality != "" {
			return nil, fmt.Errorf("AnnData has no modalities; leave modality fields empty")
		}
	case "MuData":
		if !member(selection.Modality) {
			return nil, fmt.Errorf("MuData requires an explicit expression modality")
		}
		base = "/mod/" + selection.Modality
		k, e := encoding(f, base)
		if e != nil {
			return nil, e
		}
		if k != "anndata" {
			return nil, fmt.Errorf("selected modality is not AnnData")
		}
	default:
		return nil, fmt.Errorf("expected AnnData or MuData root")
	}
	if selection.Layer != "X" && !(strings.HasPrefix(selection.Layer, "layers/") && member(strings.TrimPrefix(selection.Layer, "layers/"))) {
		return nil, fmt.Errorf("select a count matrix explicitly: X or layers/<name>; values must be nonnegative integer counts")
	}
	obs, e := readFrame(f, base+"/obs", MaxCells, 64)
	if e != nil {
		return nil, e
	}
	vars, e := readFrame(f, base+"/var", MaxFeatures, 128)
	if e != nil {
		return nil, e
	}
	m := &Matrix{Barcodes: obs.index, Features: make([]Feature, len(vars.index))}
	findColumn := func(name string) (int, error) {
		if name == "" {
			return -1, nil
		}
		for j, s := range vars.columns {
			if s == name {
				return j, nil
			}
		}
		return -1, fmt.Errorf("selected variable column %q is missing", name)
	}
	ni, e := findColumn(selection.FeatureName)
	if e != nil {
		return nil, e
	}
	ti, e := findColumn(selection.FeatureType)
	if e != nil {
		return nil, e
	}
	for i, id := range vars.index {
		name, typ := id, "Gene Expression"
		if ni >= 0 {
			name = vars.values[i][ni]
		}
		if ti >= 0 {
			typ = vars.values[i][ti]
		}
		if !validLabel(name) || !validLabel(typ) {
			return nil, fmt.Errorf("missing feature name/type in selected variable columns")
		}
		m.Features[i] = Feature{id, name, typ}
	}
	if e = readAnnMatrix(ctx, f, base+"/"+selection.Layer, m); e != nil {
		return nil, e
	}
	ev := &AnnDataEvidence{Selection: selection, Format: kind, ObservationIndex: base + "/obs/" + obs.indexName, VariableIndex: base + "/var/" + vars.indexName, VariableColumns: vars.columns, VariableValues: vars.values}
	cells := &CellAnnotations{Coverage: CellCoverage{Columns: obs.columns, Rows: len(obs.index), Matched: len(obs.index), WithoutCoordinates: len(obs.index)}, ByBarcode: map[string]int{}, Rows: make([]CellAnnotation, len(obs.index))}
	for i, b := range obs.index {
		cells.Rows[i] = CellAnnotation{Barcode: b, Values: obs.values[i], Expression: true}
		cells.ByBarcode[b] = i
	}
	if selection.Embedding != "" {
		if !member(selection.Embedding) {
			return nil, fmt.Errorf("embedding must name one obsm member")
		}
		p := base + "/obsm/" + selection.Embedding
		k, e := encoding(f, p)
		if e != nil {
			return nil, e
		}
		if k != "array" {
			return nil, fmt.Errorf("embedding must be a dense numeric array")
		}
		v, e := f.Value(p, uint64(MaxCells)*2)
		if e != nil {
			return nil, e
		}
		if v.Numbers == nil || len(v.Shape) != 2 || v.Shape[0] != uint64(len(obs.index)) || v.Shape[1] != 2 {
			return nil, fmt.Errorf("embedding must have observations × 2 coordinates")
		}
		axes := [2]string{selection.Embedding + " 1", selection.Embedding + " 2"}
		switch selection.Embedding {
		case "X_umap":
			axes = [2]string{"UMAP 1", "UMAP 2"}
		case "X_tsne":
			axes = [2]string{"t-SNE 1", "t-SNE 2"}
		}
		ev.Embedding = &EmbeddingSource{Path: p, Axes: axes, Supplied: true}
		for i := range cells.Rows {
			x, y := v.Numbers[2*i], v.Numbers[2*i+1]
			if math.IsNaN(x) && math.IsNaN(y) {
				continue
			}
			if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || math.Abs(x) > 1e12 || math.Abs(y) > 1e12 {
				return nil, fmt.Errorf("embedding coordinates must be complete finite pairs within ±1e12, or both NaN for missing")
			}
			cells.Rows[i].Position = &[2]float64{x, y}
			cells.Coverage.Mapped++
		}
		cells.Coverage.WithoutCoordinates -= cells.Coverage.Mapped
	}
	if selection.AIRRModality != "" && selection.AIRRKey == "" {
		return nil, fmt.Errorf("AIRR modality requires an explicit obsm AIRR key")
	}
	if selection.AIRRKey != "" {
		if !member(selection.AIRRKey) {
			return nil, fmt.Errorf("invalid AIRR key")
		}
		airrBase := base
		airrObs := obs
		if kind == "MuData" {
			if !member(selection.AIRRModality) {
				return nil, fmt.Errorf("select the AIRR modality explicitly")
			}
			airrBase = "/mod/" + selection.AIRRModality
			k, e := encoding(f, airrBase)
			if e != nil {
				return nil, e
			}
			if k != "anndata" {
				return nil, fmt.Errorf("AIRR modality is not AnnData")
			}
			if airrBase != base {
				airrObs, e = readFrame(f, airrBase+"/obs", MaxCells, 32)
				if e != nil {
					return nil, e
				}
			}
		}
		ev.AIRR, e = readAIRREvidence(ctx, f, airrBase+"/obsm/"+selection.AIRRKey, airrObs.index, m.Barcodes)
		if e != nil {
			return nil, e
		}
	}
	return &Input{Matrix: m, Cells: cells, AnnData: ev}, ctx.Err()
}
func readAnnMatrix(ctx context.Context, f *hdf5.Reader, path string, m *Matrix) error {
	k, e := encoding(f, path)
	if e != nil {
		return e
	}
	nc, nf := len(m.Barcodes), len(m.Features)
	counts := func(v hdf5.Value) ([]uint64, error) {
		if v.Kind != "integer" && v.Kind != "float" {
			return nil, fmt.Errorf("counts require a numeric array")
		}
		out := make([]uint64, len(v.Numbers))
		for i, x := range v.Numbers {
			if i%65536 == 0 {
				if e := ctx.Err(); e != nil {
					return nil, e
				}
			}
			if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > float64(MaxCount) || x != math.Trunc(x) {
				return nil, fmt.Errorf("selected count matrix contains non-integer, negative, non-finite or unsafe counts")
			}
			out[i] = uint64(x)
		}
		return out, nil
	}
	m.Indptr = make([]int, nc+1)
	switch k {
	case "array":
		v, e := f.Value(path, MaxEntries)
		if e != nil {
			return e
		}
		if len(v.Shape) != 2 || v.Shape[0] != uint64(nc) || v.Shape[1] != uint64(nf) {
			return fmt.Errorf("dense count shape differs from obs × var")
		}
		vs, e := counts(v)
		if e != nil {
			return e
		}
		m.Entries = len(vs)
		for cell := 0; cell < nc; cell++ {
			for feat := 0; feat < nf; feat++ {
				x := vs[cell*nf+feat]
				if x > 0 {
					m.Indices = append(m.Indices, uint32(feat))
					m.Counts = append(m.Counts, x)
				}
			}
			m.Indptr[cell+1] = len(m.Counts)
		}
	case "csr_matrix", "csc_matrix":
		shape, e := f.Attribute(path, "shape")
		if e != nil {
			return e
		}
		if shape.Kind != "integer" || len(shape.Shape) != 1 || len(shape.Numbers) != 2 || shape.Numbers[0] != float64(nc) || shape.Numbers[1] != float64(nf) {
			return fmt.Errorf("sparse count shape differs from obs × var")
		}
		major, minor := nc, nf
		if k == "csc_matrix" {
			major, minor = nf, nc
		}
		ptr, e := f.Integers(path+"/indptr", uint64(major+1))
		if e != nil {
			return e
		}
		if len(ptr) != major+1 || ptr[0] != 0 || ptr[major] > MaxEntries {
			return fmt.Errorf("invalid sparse indptr length/start/entry limit")
		}
		n := ptr[major]
		for i, p := range ptr {
			if p > n || (i > 0 && p < ptr[i-1]) {
				return fmt.Errorf("sparse indptr must be monotone")
			}
		}
		idx, e := f.Integers(path+"/indices", n)
		if e != nil {
			return e
		}
		v, e := vectorValue(f, path+"/data", n)
		if e != nil {
			return e
		}
		vs, e := counts(v)
		if e != nil {
			return e
		}
		if len(idx) != int(n) || len(vs) != int(n) {
			return fmt.Errorf("sparse array lengths disagree")
		}
		m.Entries = int(n)
		for _, i := range idx {
			if i >= uint64(minor) {
				return fmt.Errorf("sparse index outside shape")
			}
		}
		m.Indices = make([]uint32, int(n))
		m.Counts = make([]uint64, int(n))
		if k == "csr_matrix" {
			for i, p := range ptr {
				m.Indptr[i] = int(p)
			}
			for i, x := range idx {
				m.Indices[i] = uint32(x)
			}
			copy(m.Counts, vs)
		} else {
			for _, cell := range idx {
				m.Indptr[int(cell)+1]++
			}
			for i := 1; i < len(m.Indptr); i++ {
				m.Indptr[i] += m.Indptr[i-1]
			}
			next := append([]int(nil), m.Indptr...)
			for feat := 0; feat < nf; feat++ {
				for j := ptr[feat]; j < ptr[feat+1]; j++ {
					cell := int(idx[j])
					dest := next[cell]
					m.Indices[dest] = uint32(feat)
					m.Counts[dest] = vs[j]
					next[cell]++
				}
			}
		}
	default:
		return fmt.Errorf("selected count matrix must be dense, CSR or CSC")
	}
	m.GeneUMIs = make([]uint64, nc)
	m.Genes = make([]int, nc)
	for cell := 0; cell < nc; cell++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		lo, hi := m.Indptr[cell], m.Indptr[cell+1]
		if hi-lo > nf {
			return fmt.Errorf("duplicate count coordinates")
		}
		sort.Sort(cscBlock{m.Indices[lo:hi], m.Counts[lo:hi]})
		for j := lo; j < hi; j++ {
			if j > lo && m.Indices[j] == m.Indices[j-1] {
				return fmt.Errorf("duplicate count coordinate")
			}
			if m.Features[m.Indices[j]].Type == "Gene Expression" {
				x := m.Counts[j]
				if x > MaxCount-m.GeneUMIs[cell] {
					return fmt.Errorf("gene-expression count total exceeds 2^53-1")
				}
				m.GeneUMIs[cell] += x
				if x > 0 {
					m.Genes[cell]++
				}
			}
		}
	}
	return nil
}

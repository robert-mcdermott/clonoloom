package expression

// Scirpy stores unfiltered AIRR chains in obsm as an Awkward array, aligned to obs:
// https://scirpy.scverse.org/en/latest/data-structure.html
// The form/length/buffer representation follows the documented ak.to_buffers contract:
// https://awkward-array.org/doc/main/reference/generated/ak.to_buffers.html
// Only explicit built-in forms are decoded. No Python objects, extension behaviors or code run.
import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/hdf5"
)

type AIRRRow struct {
	Barcode string    `json:"barcode"`
	Chain   int       `json:"chain"`
	Values  []*string `json:"values"`
}
type AIRRCell struct {
	Barcode    string `json:"barcode"`
	Chains     *int   `json:"chains"`
	Expression bool   `json:"expression"`
}
type AIRREvidence struct {
	MissingCells    int        `json:"missingCells"`
	CellRows        []AIRRCell `json:"-"`
	Path            string     `json:"path"`
	Fields          []string   `json:"fields"`
	Cells           int        `json:"cells"`
	CellsWithChains int        `json:"cellsWithChains"`
	Matched         int        `json:"matched"`
	AIRROnly        int        `json:"airrOnly"`
	ExpressionOnly  int        `json:"expressionOnly"`
	Chains          int        `json:"chains"`
	Rows            []AIRRRow  `json:"-"`
}
type awkwardForm struct {
	Class      string                     `json:"class"`
	FormKey    string                     `json:"form_key"`
	Primitive  string                     `json:"primitive"`
	Offsets    string                     `json:"offsets"`
	Index      string                     `json:"index"`
	Fields     []string                   `json:"fields"`
	Content    *awkwardForm               `json:"content"`
	Contents   []*awkwardForm             `json:"contents"`
	Parameters map[string]json.RawMessage `json:"parameters"`
	InnerShape []int                      `json:"inner_shape"`
}
type awkwardNode struct {
	form       *awkwardForm
	content    *awkwardNode
	contents   []*awkwardNode
	values     hdf5.Value
	index      []int
	length     int
	stringType bool
}

func readAwkward(f *hdf5.Reader, path string, form *awkwardForm, depth int, nodes *int) (*awkwardNode, error) {
	if form == nil || depth > 16 || *nodes >= 256 {
		return nil, fmt.Errorf("Awkward form depth/node limit")
	}
	*nodes++
	n := &awkwardNode{form: form}
	var e error
	buffer := func(suffix string) (hdf5.Value, error) {
		if !member(form.FormKey) {
			return hdf5.Value{}, fmt.Errorf("invalid Awkward buffer key")
		}
		return vectorValue(f, path+"/"+form.FormKey+"-"+suffix, 4000001)
	}
	for k, v := range form.Parameters {
		if k != "__array__" {
			continue // Descriptive parameters never register Python behavior.
		}
		var label string
		if json.Unmarshal(v, &label) != nil || (label != "string" && label != "char") {
			return nil, fmt.Errorf("unsupported Awkward array parameter")
		}
		n.stringType = label == "string"
		if label == "char" && (form.Class != "NumpyArray" || form.Primitive != "uint8") {
			return nil, fmt.Errorf("invalid Awkward character form")
		}
	}
	switch form.Class {
	case "NumpyArray":
		if len(form.InnerShape) != 0 {
			return nil, fmt.Errorf("nested Awkward numeric shape unsupported")
		}
		n.values, e = buffer("data")
		if e != nil {
			return nil, e
		}
		expected := map[string]string{"bool": "boolean", "int8": "integer", "int16": "integer", "int32": "integer", "int64": "integer", "uint8": "integer", "uint16": "integer", "uint32": "integer", "uint64": "integer", "float32": "float", "float64": "float"}[form.Primitive]
		if expected == "" || n.values.Kind != expected || n.values.DType != form.Primitive {
			return nil, fmt.Errorf("Awkward primitive/buffer datatype mismatch")
		}
		n.length = len(n.values.Text)
	case "EmptyArray":
		n.length = 0
	case "RecordArray":
		if len(form.Fields) == 0 || len(form.Fields) > 128 || len(form.Fields) != len(form.Contents) {
			return nil, fmt.Errorf("invalid Awkward record fields")
		}
		n.length = 4000000
		seen := map[string]bool{}
		for i, child := range form.Contents {
			field := form.Fields[i]
			if !member(field) || seen[field] {
				return nil, fmt.Errorf("invalid/duplicate AIRR field")
			}
			seen[field] = true
			node, e := readAwkward(f, path, child, depth+1, nodes)
			if e != nil {
				return nil, e
			}
			n.contents = append(n.contents, node)
			n.length = min(n.length, node.length)
		}
	case "ListOffsetArray", "IndexedOptionArray", "IndexedArray", "UnmaskedArray":
		n.content, e = readAwkward(f, path, form.Content, depth+1, nodes)
		if e != nil {
			return nil, e
		}
		if form.Class == "UnmaskedArray" {
			n.length = n.content.length
			break
		}
		suffix, code := "index", form.Index
		if form.Class == "ListOffsetArray" {
			suffix, code = "offsets", form.Offsets
		}
		if code != "i32" && code != "u32" && code != "i64" {
			return nil, fmt.Errorf("unsupported Awkward index type")
		}
		v, e := buffer(suffix)
		if e != nil {
			return nil, e
		}
		if v.Kind != "integer" || v.DType != map[string]string{"i32": "int32", "i64": "int64", "u32": "uint32"}[code] {
			return nil, fmt.Errorf("Awkward indices must be integers")
		}
		n.index = make([]int, len(v.Text))
		for i, x := range v.Numbers {
			if x < 0 {
				if form.Class != "IndexedOptionArray" || x != -1 {
					return nil, fmt.Errorf("invalid negative Awkward index")
				}
			} else if x > float64(n.content.length) || (form.Class != "ListOffsetArray" && x == float64(n.content.length)) {
				return nil, fmt.Errorf("Awkward index outside content")
			}
			n.index[i] = int(x)
			if form.Class == "ListOffsetArray" && i > 0 && n.index[i] < n.index[i-1] {
				return nil, fmt.Errorf("Awkward offsets must be monotone")
			}
		}
		n.length = len(n.index)
		if form.Class == "ListOffsetArray" {
			if n.length == 0 {
				return nil, fmt.Errorf("Awkward list needs offsets")
			}
			n.length--
		}
	default:
		return nil, fmt.Errorf("unsupported Awkward form %q; use packed ListOffset/Record/IndexedOption arrays", form.Class)
	}
	if n.stringType && (form.Class != "ListOffsetArray" || form.Content == nil || form.Content.Class != "NumpyArray" || form.Content.Primitive != "uint8") {
		return nil, fmt.Errorf("invalid Awkward string form")
	}
	return n, nil
}
func (n *awkwardNode) unwrap(i int) (*awkwardNode, int, error) {
	if i < 0 || i >= n.length {
		return nil, 0, fmt.Errorf("Awkward row outside content")
	}
	switch n.form.Class {
	case "UnmaskedArray":
		return n.content.unwrap(i)
	case "IndexedArray", "IndexedOptionArray":
		if n.index[i] < 0 {
			return nil, 0, nil
		}
		return n.content.unwrap(n.index[i])
	}
	return n, i, nil
}
func (n *awkwardNode) scalar(i int) (*string, error) {
	n, i, e := n.unwrap(i)
	if e != nil || n == nil {
		return nil, e
	}
	if n.stringType {
		lo, hi := n.index[i], n.index[i+1]
		b := make([]byte, hi-lo)
		for j := range b {
			x := n.content.values.Numbers[lo+j]
			if x < 0 || x > 255 {
				return nil, fmt.Errorf("invalid Awkward character byte")
			}
			b[j] = byte(x)
		}
		if !utf8.Valid(b) {
			return nil, fmt.Errorf("invalid AIRR UTF-8")
		}
		s := string(b)
		return &s, nil
	}
	if n.form.Class != "NumpyArray" {
		return nil, fmt.Errorf("AIRR fields must be scalar strings, numbers or booleans")
	}
	if n.values.Kind == "float" {
		x := n.values.Numbers[i]
		if math.IsNaN(x) {
			return nil, nil
		}
		if math.IsInf(x, 0) {
			return nil, fmt.Errorf("infinite AIRR value")
		}
	}
	s := n.values.Text[i]
	return &s, nil
}
func readAIRREvidence(ctx context.Context, f *hdf5.Reader, path string, barcodes, expression []string) (*AIRREvidence, error) {
	k, e := encoding(f, path)
	if e != nil {
		return nil, e
	}
	if k != "awkward-array" {
		return nil, fmt.Errorf("selected AIRR key is not an Awkward array")
	}
	raw, e := attrText(f, path, "form")
	if e != nil {
		return nil, e
	}
	if len(raw) > 65536 {
		return nil, fmt.Errorf("Awkward form exceeds 64 KiB")
	}
	var form awkwardForm
	if e = json.Unmarshal([]byte(raw), &form); e != nil {
		return nil, e
	}
	length, e := f.Attribute(path, "length")
	if e != nil {
		return nil, e
	}
	if length.Kind != "integer" || len(length.Shape) != 0 || len(length.Numbers) != 1 || length.Numbers[0] != float64(len(barcodes)) {
		return nil, fmt.Errorf("AIRR length differs from selected modality obs")
	}
	nodes := 0
	root, e := readAwkward(f, path, &form, 0, &nodes)
	if e != nil {
		return nil, e
	}
	if root.length < len(barcodes) {
		return nil, fmt.Errorf("AIRR buffers shorter than observations")
	}
	// Establish fields even for all-empty cell lists, without inferring a receptor model.
	list := root
	for list.form.Class == "IndexedOptionArray" || list.form.Class == "IndexedArray" || list.form.Class == "UnmaskedArray" {
		list = list.content
	}
	if list.form.Class != "ListOffsetArray" || list.content.form.Class != "RecordArray" {
		return nil, fmt.Errorf("AIRR must contain one list of chain records per cell")
	}
	record := list.content
	out := &AIRREvidence{Path: path, Fields: record.form.Fields, Cells: len(barcodes), Rows: []AIRRRow{}}
	expressionSet := map[string]bool{}
	for _, b := range expression {
		expressionSet[b] = true
	}
	total := 0
	for i, b := range barcodes {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if expressionSet[b] {
			out.Matched++
		} else {
			out.AIRROnly++
		}
		node, j, e := root.unwrap(i)
		if e != nil {
			return nil, e
		}
		if node == nil {
			out.MissingCells++
			out.CellRows = append(out.CellRows, AIRRCell{Barcode: b, Expression: expressionSet[b]})
			continue
		}
		lo, hi := node.index[j], node.index[j+1]
		chainCount := hi - lo
		out.CellRows = append(out.CellRows, AIRRCell{Barcode: b, Chains: &chainCount, Expression: expressionSet[b]})
		if hi > lo {
			out.CellsWithChains++
		}
		if len(out.Rows)+(hi-lo) > 1000000 || (len(out.Rows)+(hi-lo))*len(out.Fields) > 4000000 {
			return nil, fmt.Errorf("AIRR exceeds 1000000 chains or 4000000 field values")
		}
		for chain := lo; chain < hi; chain++ {
			row := AIRRRow{Barcode: b, Chain: chain - lo + 1, Values: make([]*string, len(out.Fields))}
			for k, field := range record.contents {
				v, e := field.scalar(chain)
				if e != nil {
					return nil, e
				}
				if v != nil {
					if strings.ContainsAny(*v, "\x00\t\r\n") {
						return nil, fmt.Errorf("invalid AIRR field text")
					}
					total += len(*v)
					if total > 64<<20 {
						return nil, fmt.Errorf("AIRR values exceed 64 MiB")
					}
				}
				row.Values[k] = v
			}
			out.Rows = append(out.Rows, row)
		}
	}
	out.Chains = len(out.Rows)
	out.ExpressionOnly = len(expression) - out.Matched
	return out, nil
}

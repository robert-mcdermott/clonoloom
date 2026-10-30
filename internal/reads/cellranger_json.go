package reads

// Original implementation of the documented nested contig format, not the
// assembly/annotation algorithm. 10x Genomics Cell Ranger V(D)J annotations:
// https://www.10xgenomics.com/support/software/cell-ranger/7.2/analysis/cr-5p-outputs-annotations-vdj
// Unknown flags and mixed chains remain unresolved; see docs/CELLRANGER-JSON.md.
import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"clonoloom/internal/airr"
	"clonoloom/internal/rep"
)

const CellRangerJSONLayout = "cellranger-nested-v1"
const maxContigJSON = 8 << 20
const maxJSONContigs = 100_000

// Only the array layout is recognized. A metadata object and OAS metadata lines
// continue through their existing readers. Peeking never consumes a table header.
func jsonStart(r *bufio.Reader) bool {
	data, _ := r.Peek(4096)
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	return len(data) > 0 && data[0] == '['
}

func cellRangerJSONName(name string) bool {
	name = strings.TrimSuffix(strings.ToLower(name), ".gz")
	return strings.HasSuffix(name, "contig_annotations.json")
}

// A renamed upload is recognized from its first object; a known filename still
// goes through full layout checks. The probe is bounded and opens no other file.
func sniffCellRangerJSON(ctx context.Context, path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	r, _, err := decompress(f)
	if err != nil {
		return false, err
	}
	br := bufio.NewReader(r)
	if bom, _ := br.Peek(3); bytes.Equal(bom, []byte{0xef, 0xbb, 0xbf}) {
		br.Discard(3)
	}
	d := json.NewDecoder(io.LimitReader(metadataReader{ctx, br}, maxContigJSON+1))
	if token, err := d.Token(); err != nil || token != json.Delim('[') {
		return false, ctx.Err()
	}
	if !d.More() {
		return false, nil
	}
	var raw map[string]json.RawMessage
	if err := d.Decode(&raw); err != nil {
		// A large AIRR Repertoire object may exceed the native-contig probe
		// budget. Its existing metadata reader remains authoritative.
		return false, ctx.Err()
	}
	_, named := raw["contig_name"]
	_, annotated := raw["annotations"]
	_, barcode := raw["barcode"]
	// Explicit AIRR identity remains authoritative even if a repertoire has
	// similarly named extension fields.
	if _, repertoire := raw["repertoire_id"]; repertoire {
		return false, nil
	}
	return named && annotated && barcode, nil
}

func readCellRangerJSON(ctx context.Context, input io.Reader, src Source, index int, b **rep.Builder) (Source, rep.Unit, error) {
	src.Format = FormatCellRangerJSON
	src.Native = &NativeTable{Layout: CellRangerJSONLayout, Rows: []NativeRow{}}
	if *b == nil {
		*b = rep.NewBuilder(rep.Cells)
	}
	br := bufio.NewReader(input)
	if bom, _ := br.Peek(3); bytes.Equal(bom, []byte{0xef, 0xbb, 0xbf}) {
		br.Discard(3)
	}
	limited := &io.LimitedReader{R: metadataReader{ctx, br}, N: maxNativeBytes + 1}
	d := json.NewDecoder(limited)
	start, err := d.Token()
	if err != nil || start != json.Delim('[') {
		return src, "", fmt.Errorf("Cell Ranger JSON requires a top-level array of detailed contig objects")
	}
	name := src.Path
	for _, suffix := range []string{".gz", ".json"} {
		if ext := filepath.Ext(name); strings.EqualFold(ext, suffix) {
			name = strings.TrimSuffix(name, ext)
		}
	}
	sample := sampleName(name)
	seen := map[string]bool{}
	for d.More() {
		if err := ctx.Err(); err != nil {
			return src, "", err
		}
		if len(src.Native.Rows) >= maxJSONContigs {
			return src, "", fmt.Errorf("Cell Ranger JSON exceeds 100000 contigs; split the input explicitly")
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return src, "", fmt.Errorf("contig %d: invalid or oversized JSON: %w", len(src.Native.Rows)+1, err)
		}
		if len(raw) > maxContigJSON {
			return src, "", fmt.Errorf("contig exceeds 8 MiB; no records were imported")
		}
		if err := airr.CheckExactJSON(ctx, raw); err != nil {
			return src, "", fmt.Errorf("contig %d: %w", len(src.Native.Rows)+1, err)
		}
		r, err := decodeJSONContig(raw, sample)
		if err != nil {
			return src, "", fmt.Errorf("contig %d: %w", len(src.Native.Rows)+1, err)
		}
		key := r.Sample + "\x00" + r.SequenceID
		if seen[key] {
			return src, "", fmt.Errorf("duplicate contig_name %q in sample %q", r.SequenceID, r.Sample)
		}
		seen[key] = true
		r.Source, r.Line = index, uint32(len(src.Native.Rows)+1) // JSON record ordinal, not a text line.
		if err := (*b).Append(&r); err != nil {
			return src, "", err
		}
		src.Native.Rows = append(src.Native.Rows, NativeRow{ID: r.SequenceID, Line: r.Line, Text: string(raw)})
	}
	if end, err := d.Token(); err != nil || end != json.Delim(']') {
		return src, "", fmt.Errorf("Cell Ranger JSON array is incomplete")
	}
	var extra json.RawMessage
	if err := d.Decode(&extra); err != io.EOF {
		return src, "", fmt.Errorf("Cell Ranger JSON must contain exactly one array")
	}
	if limited.N <= 0 {
		return src, "", fmt.Errorf("Cell Ranger JSON exceeds 64 MiB decompressed input")
	}
	src.Rows = len(src.Native.Rows)
	return src, rep.Cells, nil
}

func decodeJSONContig(raw []byte, sample string) (rep.Row, error) {
	r := rep.NewRow()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return r, fmt.Errorf("expected a detailed contig object")
	}
	for _, key := range []string{"contig_name", "barcode", "annotations", "sequence"} {
		if _, ok := m[key]; !ok {
			return r, fmt.Errorf("unsupported contig layout: missing %s", key)
		}
	}
	// These are CSV-style aliases, not part of the supported nested layout.
	for _, key := range []string{"contig_id", "chain", "v_gene", "d_gene", "j_gene", "c_gene", "cdr3_nt", "reads", "umis", "raw_clonotype_id"} {
		if _, ok := m[key]; ok {
			return r, fmt.Errorf("unsupported or conflicting flat field %s; use the CSV reader", key)
		}
	}
	var err error
	if r.SequenceID, err = jsonContigString(m, "contig_name", true); err != nil {
		return r, err
	}
	if r.Cell, err = jsonContigString(m, "barcode", true); err != nil {
		return r, err
	}
	if r.Sample, err = jsonContigString(m, "sample", false); err != nil {
		return r, err
	}
	if r.Sample == "" {
		r.Sample = sample
	}
	for key, dst := range map[string]*rep.Flag{"productive": &r.Productive, "is_cell": &r.IsCell, "high_confidence": &r.HighConfidence, "full_length": &r.FullLength} {
		v, ok := m[key]
		if !ok || bytes.Equal(v, []byte("null")) {
			continue
		}
		switch string(v) {
		case "true":
			*dst = rep.True
		case "false":
			*dst = rep.False
		default:
			return r, fmt.Errorf("%s must be boolean or null", key)
		}
	}
	for key, dst := range map[string]*float64{"read_count": &r.Reads, "umi_count": &r.UMIs} {
		v, ok := m[key]
		if !ok || bytes.Equal(v, []byte("null")) {
			continue
		}
		// JSON counts must be integer tokens. No strings, exponent notation or rounding.
		if len(v) == 0 || strings.Trim(string(v), "0123456789") != "" {
			return r, fmt.Errorf("%s must be an integer JSON token or null", key)
		}
		*dst, err = nativeCount(string(v), false)
		if err != nil {
			return r, fmt.Errorf("%s: %w", key, err)
		}
	}
	r.Count = r.UMIs
	if r.Junction, err = jsonContigString(m, "cdr3_seq", false); err != nil {
		return r, err
	}
	if r.JunctionAA, err = jsonContigString(m, "cdr3", false); err != nil {
		return r, err
	}
	if r.Evidence[0], err = jsonContigString(m, "sequence", false); err != nil {
		return r, err
	}
	if strings.Trim(r.Junction+r.Evidence[0], "ACGTNacgtn") != "" || strings.Trim(r.JunctionAA, "ACDEFGHIKLMNPQRSTVWYXacdefghiklmnpqrstvwyx*") != "" {
		return r, fmt.Errorf("unsupported nucleotide or amino-acid alphabet")
	}
	r.Junction, r.JunctionAA = strings.ToUpper(r.Junction), strings.ToUpper(r.JunctionAA)
	if v, ok := m["clonotype"]; ok && !bytes.Equal(v, []byte("null")) {
		return r, fmt.Errorf("non-null legacy clonotype field is unsupported; use an explicit CSV/AIRR export")
	}
	if v, ok := m["info"]; ok && !bytes.Equal(v, []byte("null")) {
		var info map[string]json.RawMessage
		if json.Unmarshal(v, &info) != nil || info == nil {
			return r, fmt.Errorf("info must be an object or null")
		}
		if r.Clone, err = jsonContigString(info, "raw_clonotype_id", false); err != nil {
			return r, err
		}
	}
	// Decode through maps: encoding/json's struct-field matching is otherwise
	// case-insensitive and could let an unknown "Feature" override "feature".
	var annotations []map[string]json.RawMessage
	if len(m["annotations"]) == 0 || m["annotations"][0] != '[' || json.Unmarshal(m["annotations"], &annotations) != nil {
		return r, fmt.Errorf("annotations must be an array of feature objects")
	}
	chains := map[string]bool{}
	for _, a := range annotations {
		var feature map[string]json.RawMessage
		if json.Unmarshal(a["feature"], &feature) != nil || feature == nil {
			return r, fmt.Errorf("each annotation must contain a feature object")
		}
		chain, e := jsonContigString(feature, "chain", true)
		if e != nil {
			return r, e
		}
		if rep.LocusCode(chain) == 0 {
			return r, fmt.Errorf("unsupported annotation chain %q", chain)
		}
		chains[chain] = true
		region, e := jsonContigString(feature, "region_type", true)
		if e != nil {
			return r, e
		}
		gene, e := jsonContigString(feature, "gene_name", true)
		if e != nil {
			return r, e
		}
		var dst *string
		switch region {
		case "L-REGION+V-REGION":
			dst = &r.V
		case "D-REGION":
			dst = &r.D
		case "J-REGION":
			dst = &r.J
		case "C-REGION":
			dst = &r.C
		case "5'UTR":
			continue
		default:
			return r, fmt.Errorf("unsupported annotation region_type %q", region)
		}
		if *dst != "" && *dst != gene {
			return r, fmt.Errorf("competing %s gene calls require an explicit CSV/AIRR export", region)
		}
		*dst = gene
	}
	// A nonempty sentinel prevents the common table builder from guessing a chain
	// from one J/V gene when the source has mixed or no chain annotations.
	r.Locus = "unresolved"
	if len(chains) == 1 {
		for chain := range chains {
			r.Locus = chain
		}
	}
	return r, nil
}

func jsonContigString(m map[string]json.RawMessage, key string, required bool) (string, error) {
	v, ok := m[key]
	if !ok || bytes.Equal(v, []byte("null")) {
		if required {
			return "", fmt.Errorf("%s must be a nonempty string", key)
		}
		return "", nil
	}
	var s string
	if json.Unmarshal(v, &s) != nil || (required && s == "") || strings.TrimSpace(s) != s || strings.ContainsFunc(s, unicode.IsControl) {
		return "", fmt.Errorf("%s must be a string without padding/control characters", key)
	}
	if (key == "barcode" || key == "contig_name" || key == "sample" || key == "raw_clonotype_id") && len(s) > 4096 {
		return "", fmt.Errorf("%s exceeds 4096 bytes", key)
	}
	return s, nil
}

// JSON mixed with other representations cannot silently duplicate a contig or
// assign contradictory source cell/clone calls within the same sample/barcode.
func checkJSONContigIdentities(ctx context.Context, res *Result) error {
	hasJSON := false
	for _, s := range res.Sources {
		hasJSON = hasJSON || s.Format == FormatCellRangerJSON
	}
	if !hasJSON {
		return nil
	}
	t := res.Table
	type contigKey struct {
		sample uint32
		id     string
	}
	ids := map[contigKey]bool{}
	type call struct {
		flag  rep.Flag
		clone uint32
	}
	cells := map[[2]uint32]call{}
	for i := 0; i < t.Len(); i++ {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		id := contigKey{t.Sample[i], t.IDs.Get(t.SequenceID[i])}
		if id.id != "" {
			if ids[id] {
				return fmt.Errorf("duplicate contig identity %q; open alternate representations separately", id.id)
			}
			ids[id] = true
		}
		if len(t.Cell) == 0 || t.Cell[i] == 0 {
			continue
		}
		key := [2]uint32{t.Sample[i], t.Cell[i]}
		current, found := cells[key]
		if !found {
			current.flag = rep.Missing
		}
		if len(t.IsCell) != 0 && t.IsCell[i] != rep.Missing {
			if current.flag != rep.Missing && current.flag != t.IsCell[i] {
				return fmt.Errorf("conflicting is_cell calls for barcode %q", t.Dict.Get(t.Cell[i]))
			}
			current.flag = t.IsCell[i]
		}
		if len(t.Clone) != 0 && t.Clone[i] != 0 {
			if current.clone != 0 && current.clone != t.Clone[i] {
				return fmt.Errorf("conflicting source clonotypes for barcode %q", t.Dict.Get(t.Cell[i]))
			}
			current.clone = t.Clone[i]
		}
		cells[key] = current
	}
	return nil
}

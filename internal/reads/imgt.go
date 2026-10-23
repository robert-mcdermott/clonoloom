package reads

// IMGT/V-QUEST and HighV-QUEST tabular output, documented at
// https://www.imgt.org/IMGT_vquest/user_guide (tables 1, 2, 3, 6 and 11).
// Brochet, Lefranc & Giudicelli 2008, NAR 36:W503–508, doi:10.1093/nar/gkn316.
// This implementation is based on the format documentation, not another reader's code.
// ClonoLoom's choices: require the four annotation tables, join by both supplied identifiers,
// retain native annotations, and withhold query coordinates after reported indel correction.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
)

var imgtNames = []string{"1_Summary", "2_IMGT-gapped-nt-sequences", "3_Nt-sequences", "6_Junction", "11_Parameters"}

// imgtName recognizes documented names, optionally prefixed with a sample name.
func imgtName(name string) (role int, prefix string) {
	base := strings.TrimSuffix(filepath.Base(name), ".gz")
	ext := filepath.Ext(base)
	if ext != ".txt" && ext != ".tsv" {
		return -1, ""
	}
	base = strings.TrimSuffix(base, ext)
	for i, suffix := range imgtNames {
		if strings.HasSuffix(base, suffix) {
			return i, strings.TrimSuffix(base, suffix)
		}
	}
	return -1, ""
}

// IMGTFiles finds one extracted run. Other IMGT reports remain in the source folder;
// they are not silently treated as additional rearrangements. No recursion or archive extraction.
func IMGTFiles(dir string) ([]Input, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
	}
	var out []Input
	found := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if role, _ := imgtName(e.Name()); role >= 0 {
			found = true
			out = append(out, Input{Path: filepath.Join(dir, e.Name())})
		}
	}
	return out, found, nil
}

// PrepareIMGTInputs groups explicitly named tables before cache fingerprinting. Group names
// survive library storage and relocation. One run's tables are in documented order, independent
// of selection order. Two runs with the same sample name must be opened separately or renamed.
func PrepareIMGTInputs(inputs []Input) ([]Input, error) {
	type group struct {
		name   string
		tables map[int]Input
	}
	groups := map[string]*group{}
	owners := map[string]string{}
	keys := make([]string, len(inputs))
	for i, in := range inputs {
		if in.Mapping != nil {
			if in.IMGTGroup != "" {
				return nil, fmt.Errorf("%s: cannot combine an IMGT group with a column mapping", in.Path)
			}
			continue
		}
		name := in.Name
		if name == "" {
			name = in.Path
		}
		role, prefix := imgtName(name)
		if role < 0 {
			if in.IMGTGroup != "" {
				return nil, fmt.Errorf("%s: unrecognized IMGT table name", name)
			}
			continue
		}
		key := filepath.Dir(in.Path) + "/" + prefix
		sample := strings.TrimRight(prefix, "_-.")
		if sample == "" {
			sample = filepath.Base(filepath.Dir(in.Path))
		}
		if in.Name != "" && filepath.Dir(in.Name) != "." {
			key = filepath.Dir(in.Name) + "/" + prefix
			if prefix == "" {
				sample = filepath.Base(filepath.Dir(in.Name))
			}
		}
		if in.Name != "" && filepath.Dir(in.Name) == "." {
			// Library files may live in different hash-shard directories. Their supplied
			// names, not their storage paths, identify the run.
			key = "named:" + prefix
			if prefix == "" {
				sample = "IMGT"
			}
		}
		if in.IMGTGroup != "" {
			key, sample = "saved:"+in.IMGTGroup, in.IMGTGroup
		}
		if owner, ok := owners[sample]; ok && owner != key {
			return nil, fmt.Errorf("IMGT sample %q occurs in two runs: open them separately or give their table names distinct prefixes", sample)
		}
		owners[sample] = key
		g := groups[key]
		if g == nil {
			g = &group{sample, map[int]Input{}}
			groups[key] = g
		}
		if _, dup := g.tables[role]; dup {
			return nil, fmt.Errorf("%s: duplicate %s table for IMGT sample %q", name, imgtNames[role], sample)
		}
		in.IMGTGroup = sample
		g.tables[role] = in
		keys[i] = key
	}
	var out []Input
	emitted := map[string]bool{}
	for i, in := range inputs {
		key := keys[i]
		if key == "" {
			out = append(out, in)
			continue
		}
		if emitted[key] {
			continue
		}
		emitted[key] = true
		g := groups[key]
		for role := 0; role < len(imgtNames); role++ {
			table, ok := g.tables[role]
			if !ok && role < 4 {
				return nil, fmt.Errorf("IMGT sample %q: missing %s.txt; open the extracted output folder or select all four annotation tables together", g.name, imgtNames[role])
			}
			if ok {
				out = append(out, table)
			}
		}
	}
	return out, nil
}

type imgtKey struct{ number, id string }
type imgtPosition struct {
	offset int64
	size   int
	line   uint32
}
type imgtIndex struct {
	file   *os.File
	rows   map[imgtKey]imgtPosition
	order  []imgtKey
	cols   columns
	source Source
}

func (x *imgtIndex) close() {
	if x != nil && x.file != nil {
		name := x.file.Name()
		x.file.Close()
		os.Remove(name)
	}
}
func (x *imgtIndex) get(key imgtKey) ([]string, uint32, error) {
	pos, ok := x.rows[key]
	if !ok {
		return nil, 0, fmt.Errorf("%s: missing Sequence number %q, Sequence ID %q", x.source.Name, key.number, key.id)
	}
	data := make([]byte, pos.size)
	if _, err := x.file.ReadAt(data, pos.offset); err != nil {
		return nil, 0, err
	}
	return splitTab(data, nil), pos.line, nil
}

// Index only identities and byte offsets in memory; spool the bounded lines to a private
// temporary file. This supports arbitrarily reordered tables and gzip without retaining four
// entire annotation files in memory. The spool is removed on success, failure and cancellation.
func indexIMGT(ctx context.Context, r io.Reader, name string, role int) (x *imgtIndex, err error) {
	x = &imgtIndex{rows: map[imgtKey]imgtPosition{}, source: Source{Name: filepath.Base(name), Format: FormatIMGT}}
	defer func() {
		if err != nil {
			x.close()
		}
	}()
	plain, gz, err := decompress(r)
	if err != nil {
		return nil, err
	}
	x.source.Gzipped = gz
	ls := newLines(plain)
	ls.ctx = ctx
	if !ls.next() {
		if ls.err != nil {
			return nil, ls.err
		}
		return nil, fmt.Errorf("empty IMGT table")
	}
	x.source.Header = splitTab(ls.line, nil)
	seen := map[string]bool{}
	for i, h := range x.source.Header {
		h = strings.TrimSpace(h)
		x.source.Header[i] = h
		lower := strings.ToLower(h)
		if h == "" || seen[lower] {
			return nil, fmt.Errorf("line 1: empty or duplicate column %q", h)
		}
		seen[lower] = true
	}
	x.cols = newColumns(x.source.Header)
	required := []string{"Sequence number", "Sequence ID", "V-GENE and allele"}
	switch role {
	case 0:
		required = append(required, "Sequence")
	case 1:
		required = append(required, "V-REGION")
	case 2:
		required = append(required, "V-REGION start", "V-REGION end")
	case 3:
		required = append(required, "JUNCTION")
	}
	for _, h := range required {
		if x.cols.index(h) < 0 {
			return nil, fmt.Errorf("line 1: missing %s column in %s", h, imgtNames[role])
		}
	}
	if x.cols.index("V-DOMAIN Functionality", "Functionality") < 0 {
		return nil, fmt.Errorf("line 1: missing V-DOMAIN Functionality or Functionality column")
	}
	x.file, err = os.CreateTemp("", "clonoloom-imgt-*")
	if err != nil {
		return nil, err
	}
	var offset int64
	numbers := map[string]bool{}
	for ls.next() {
		if err := ctx.Err(); err != nil {
			return x, err
		}
		if len(ls.line) == 0 {
			continue
		}
		fields := splitTab(ls.line, nil)
		if len(fields) != len(x.source.Header) {
			return x, fmt.Errorf("line %d: %d fields, expected %d", ls.n, len(fields), len(x.source.Header))
		}
		key := imgtKey{field(fields, x.cols.index("Sequence number")), field(fields, x.cols.index("Sequence ID"))}
		if n, e := strconv.ParseUint(key.number, 10, 32); e != nil || n == 0 || key.id == "" {
			return x, fmt.Errorf("line %d: require a positive Sequence number and nonempty Sequence ID", ls.n)
		}
		if numbers[key.number] {
			return x, fmt.Errorf("line %d: duplicate Sequence number %s", ls.n, key.number)
		}
		numbers[key.number] = true
		if _, dup := x.rows[key]; dup {
			return x, fmt.Errorf("line %d: duplicate sequence identity (%s, %s)", ls.n, key.number, key.id)
		}
		if _, err := x.file.Write(ls.line); err != nil {
			return x, err
		}
		x.rows[key] = imgtPosition{offset, len(ls.line), ls.n}
		offset += int64(len(ls.line))
		x.order = append(x.order, key)
	}
	if ls.err != nil {
		return x, ls.err
	}
	x.source.Rows = len(x.order)
	return x, nil
}

func readIMGT(ctx context.Context, inputs []Input, src int, b *rep.Builder, p *Problems) ([]Source, error) {
	if len(inputs) < 4 || len(inputs) > 5 {
		return nil, fmt.Errorf("an IMGT run requires four annotation tables and optional Parameters")
	}
	var indexes [4]*imgtIndex
	defer func() {
		for _, x := range indexes {
			x.close()
		}
	}()
	sources := make([]Source, len(inputs))
	for i, in := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := os.Open(in.Path)
		if err != nil {
			return nil, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, err
		}
		name := in.Name
		if name == "" {
			name = in.Path
		}
		h := &hashingReader{r: f, h: sha256.New()}
		if i < 4 {
			indexes[i], err = indexIMGT(ctx, h, name, i)
			if err == nil {
				sources[i] = indexes[i].source
			}
		} else {
			var plain io.Reader
			var gz bool
			plain, gz, err = decompress(h)
			if err == nil {
				var data []byte
				data, err = io.ReadAll(io.LimitReader(plain, (64<<10)+1))
				if len(data) > 64<<10 {
					err = fmt.Errorf("parameters exceed 64 KiB")
				}
				sources[i] = Source{Format: FormatIMGT, Gzipped: gz, AnnotationParameters: string(data)}
			}
		}
		if err == nil {
			_, err = io.Copy(io.Discard, h)
		}
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(name), err)
		}
		sources[i].Path, sources[i].Name, sources[i].IMGTGroup = in.Path, filepath.Base(name), in.IMGTGroup
		sources[i].Size, sources[i].SHA256, sources[i].Modified = h.n, hex.EncodeToString(h.h.Sum(nil)), info.ModTime().UTC().Format(time.RFC3339)
	}
	// Membership must be identical; an incomplete or mixed run is not a partial successful join.
	for i := 1; i < 4; i++ {
		if len(indexes[i].rows) != len(indexes[0].rows) {
			return nil, fmt.Errorf("%s: %d identities, Summary has %d; use tables from the same complete run", sources[i].Name, len(indexes[i].rows), len(indexes[0].rows))
		}
	}
	sources[0].GeneNames = map[string]string{}
	for _, key := range indexes[0].order {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var values [4][]string
		var line [4]uint32
		for i, x := range indexes {
			var err error
			values[i], line[i], err = x.get(key)
			if err != nil {
				return nil, err
			}
		}
		get := func(i int, names ...string) string { return field(values[i], indexes[i].cols.index(names...)) }
		// Repeated annotations must agree when supplied. Missing calls are allowed in failed rows.
		for _, h := range []string{"V-GENE and allele", "D-GENE and allele", "J-GENE and allele"} {
			baseline := imgtCall(get(0, h))
			for i := 1; i < 4; i++ {
				other := imgtCall(get(i, h))
				if baseline != "" && other != "" && baseline != other {
					return nil, fmt.Errorf("%s line %d: %s conflicts with Summary for (%s, %s)", sources[i].Name, line[i], h, key.number, key.id)
				}
			}
		}
		r := rep.NewRow()
		r.Source, r.Line, r.Sample, r.SequenceID = src, line[0], inputs[0].IMGTGroup, key.id
		r.Count = 1 // explicitly one annotated sequence, never one read or molecule
		calls := []*string{&r.V, &r.D, &r.J, &r.C}
		for i, h := range []string{"V-GENE and allele", "D-GENE and allele", "J-GENE and allele", "C-GENE and allele"} {
			raw := get(0, h)
			*calls[i] = imgtCall(raw)
			if raw != "" {
				sources[0].GeneNames[raw] = *calls[i]
				if *calls[i] == "" {
					p.addf("imgt-annotation", src, line[0], h, raw, "no recognizable gene call; native annotation retained")
				}
			}
		}
		functionality := get(0, "V-DOMAIN Functionality", "Functionality")
		switch strings.TrimSpace(strings.TrimSuffix(functionality, "(see comment)")) {
		case "productive":
			r.Productive = rep.True
		case "unproductive":
			r.Productive = rep.False
		}
		frame := get(3, "JUNCTION frame")
		if frame == "" {
			frame = get(0, "JUNCTION frame")
		}
		switch frame {
		case "in-frame":
			r.VJInFrame = rep.True
		case "out-of-frame":
			r.VJInFrame = rep.False
		}
		r.Junction = strings.ToUpper(get(3, "JUNCTION"))
		r.JunctionAA = strings.ToUpper(get(3, "JUNCTION (AA)"))
		if r.JunctionAA == "" {
			aa := get(0, "AA JUNCTION")
			if at := strings.Index(aa, " (see"); at >= 0 {
				aa = aa[:at]
			}
			if !strings.Contains(aa, "#") && frame != "out-of-frame" {
				r.JunctionAA = strings.ToUpper(aa)
			}
		}
		set := func(name, value string) {
			for i, h := range rep.EvidenceFields {
				if h == name {
					r.Evidence[i] = value
					return
				}
			}
		}
		set("sequence", get(0, "Sequence"))
		// Summary is already oriented. Preserve the submitted orientation in native annotations;
		// AIRR rev_comp=F describes our exported sequence, which needs no reversal.
		if get(0, "Sequence") != "" {
			set("rev_comp", "F")
		}
		aligned := get(1, "V-D-J-REGION")
		if aligned == "" {
			aligned = get(1, "V-J-REGION")
		}
		if aligned == "" {
			aligned = get(1, "V-REGION")
		}
		set("sequence_alignment", aligned)
		native := map[string]any{"Sequence number": key.number, "table_lines": map[string]uint32{sources[0].Name: line[0], sources[1].Name: line[1], sources[2].Name: line[2], sources[3].Name: line[3]}}
		for _, h := range []string{"V-DOMAIN Functionality", "Functionality", "V-DOMAIN Functionality comment", "Functionality comment", "Orientation", "JUNCTION frame", "AA JUNCTION", "V-REGION potential ins/del", "V-REGION insertions", "V-REGION deletions", "V-GENE and allele", "D-GENE and allele", "J-GENE and allele", "C-GENE and allele"} {
			if v := get(0, h); v != "" {
				native[h] = v
			}
		}
		indel := get(0, "V-REGION insertions") != "" || get(0, "V-REGION deletions") != ""
		coordinates := map[string]string{}
		withheld := false
		for _, region := range []struct{ imgt, airr string }{{"V-REGION", "v_sequence"}, {"D-REGION", "d_sequence"}, {"J-REGION", "j_sequence"}, {"FR1-IMGT", "fwr1"}, {"CDR1-IMGT", "cdr1"}, {"FR2-IMGT", "fwr2"}, {"CDR2-IMGT", "cdr2"}, {"FR3-IMGT", "fwr3"}, {"CDR3-IMGT", "cdr3"}, {"FR4-IMGT", "fwr4"}} {
			start, end := get(2, region.imgt+" start"), get(2, region.imgt+" end")
			if start != "" {
				coordinates[region.imgt+" start"] = start
			}
			if end != "" {
				coordinates[region.imgt+" end"] = end
			}
			// A region must match the oriented Summary substring before its coordinate space is
			// asserted to be AIRR query coordinates. Native positions survive every rejected mapping.
			a, e1 := strconv.ParseInt(start, 10, 64)
			z, e2 := strconv.ParseInt(end, 10, 64)
			sequence, regionSeq := get(0, "Sequence"), get(2, region.imgt)
			if !indel && e1 == nil && e2 == nil && a > 0 && z >= a && z <= int64(len(sequence)) && regionSeq != "" && strings.EqualFold(sequence[a-1:z], regionSeq) {
				set(region.airr+"_start", start)
				set(region.airr+"_end", end)
			} else if start != "" || end != "" {
				withheld = true
				p.addf("imgt-coordinate", src+2, line[2], region.imgt, start+".."+end, "query coordinates withheld: require an uncorrected region matching the oriented Summary sequence")
			}
		}
		if withheld {
			native["query_coordinates_withheld"] = true
		}
		junctionNative := map[string]string{}
		for _, h := range []string{"JUNCTION frame", "JUNCTION", "JUNCTION (AA)", "JUNCTION with restored frameshift", "JUNCTION (AA) (with frameshift)"} {
			if v := get(3, h); v != "" {
				junctionNative[h] = v
			}
		}
		if len(junctionNative) > 0 {
			native["Junction"] = junctionNative
		}
		if len(coordinates) > 0 {
			native["nt_coordinates"] = coordinates
		}
		raw, _ := json.Marshal(native)
		set("imgt_annotations", string(raw))
		if err := b.Append(&r); err != nil {
			return nil, err
		}
	}
	return sources, nil
}

// Remove only species/functionality decorations. Keep tied gene/allele calls as a sorted set;
// never select a best allele from a tie, or infer an isotype from a V gene.
func imgtCall(raw string) string {
	var names []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ' ' || r == '\t' || r == ',' || r == '|' || r == ';' }) {
		n := genes.Parse(part)
		if n.Locus != "" && !seen[part] {
			names = append(names, part)
			seen[part] = true
		}
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

package main

// De-identification is an explicit, allowlisted copy, not an anonymity claim.
// NIST SP 800-188 (2023), doi:10.6028/NIST.SP.800-188; AIRR Rearrangement
// schema 2.0, Vander Heiden et al. (2018), doi:10.3389/fimmu.2018.02206.
// ClonoLoom choice: fresh secret-key pseudonyms, separate private correspondence,
// unchanged native scientific table fields, and refusal of unmapped nested evidence.
import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/export"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
)

const deidMaxBytes = 64 << 20
const deidMapBytes = 32 << 20
const deidMaxBindings = 250000
const deidLifetime = 15 * time.Minute
const deidLimits = "Pseudonymized dataset tables, not anonymous data or a scientific replay certificate. Receptor sequences, gene calls, stable clonotype IDs, counts and optional annotations may permit linkage. No privacy-risk estimate or legal certification is provided. Original data and saved scientific evidence remain unchanged. Provider restrictions still apply. Keep the identity map separately and private."

var deidFields = []string{"subject", "timepoint", "tissue", "reference", "cellSubset", "condition", "batch", "hla"}

type deidAnnotation struct {
	Sample string            `json:"sample"`
	Fields map[string]string `json:"fields"`
}
type deidRequest struct {
	Dataset     string               `json:"dataset"`
	Definition  clonotype.Definition `json:"definition"`
	Filter      qc.Filter            `json:"filter"`
	What        string               `json:"what"` // tables (AIRR + clonotypes), or clonotypes only
	DropText    *bool                `json:"dropText"`
	DropHLA     *bool                `json:"dropHLA"`
	Annotations []deidAnnotation     `json:"annotations"`
	Accept      bool                 `json:"accept"`
}
type deidBinding struct {
	Kind      string `json:"kind"`
	Scope     string `json:"scope"`
	Original  string `json:"original"`
	Pseudonym string `json:"pseudonym"`
}
type deidRecord struct {
	Format          string               `json:"format"`
	Version         int                  `json:"version"`
	Scope           string               `json:"scope"`
	ExportID        string               `json:"exportId"`
	Software        artifacts.Software   `json:"software"`
	What            string               `json:"what"`
	DropText        bool                 `json:"dropText"`
	DropHLA         bool                 `json:"dropHLA"`
	Removed         []string             `json:"removed"`
	Unit            string               `json:"unit"`
	Definition      clonotype.Definition `json:"definition"`
	DefinitionID    string               `json:"definitionId"`
	Filter          qc.Filter            `json:"filter"`
	Samples         int                  `json:"samples"`
	Subjects        int                  `json:"subjects"`
	MissingSubjects int                  `json:"missingSubjects"`
	Rows            int                  `json:"retainedRows"`
	Clonotypes      int                  `json:"clonotypes"`
	Bindings        map[string]int       `json:"bindings"`
	Credits         []export.Credit      `json:"credits"`
	Restricted      bool                 `json:"restricted"`
	Files           []runOutput          `json:"files"`
	Limits          string               `json:"limits"`
}
type deidPreview struct {
	Token       string        `json:"token"`
	Fingerprint string        `json:"fingerprint"`
	Expires     string        `json:"expires"`
	Record      deidRecord    `json:"record"`
	Subjects    []deidBinding `json:"subjects"` // local review only; never in the ZIP
}
type deidPackage struct {
	Preview             deidPreview
	Bundle, IdentityMap []byte
	Expires             time.Time
}
type deidShelf struct {
	mu        sync.Mutex
	preparing bool
	current   *deidPackage
}
type deidBuffer struct {
	bytes.Buffer
	ctx   context.Context
	limit int
}

func (b *deidBuffer) Write(p []byte) (int, error) {
	if e := b.ctx.Err(); e != nil {
		return 0, e
	}
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("de-identification output exceeds byte limit")
	}
	return b.Buffer.Write(p)
}
func (b *deidBuffer) WriteString(s string) (int, error) { return b.Write([]byte(s)) }

type deidAliases struct {
	key     []byte
	byKey   map[string]string
	byAlias map[string]bool
	entries []deidBinding
	size    int
}

func newDeidAliases() (*deidAliases, error) {
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		return nil, e
	}
	return &deidAliases{key: key, byKey: map[string]string{}, byAlias: map[string]bool{}, entries: []deidBinding{}}, nil
}
func (a *deidAliases) alias(kind, scope, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !utf8.ValidString(value) || len(value) > 8192 {
		return "", errors.New("identifier is not bounded UTF-8")
	}
	key := string(mustJSON([]string{kind, scope, value}))
	if v := a.byKey[key]; v != "" {
		return v, nil
	}
	if len(a.entries) >= deidMaxBindings {
		return "", errors.New("export exceeds 250000 identity bindings")
	}
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(key))
	id := kind + "-" + hex.EncodeToString(mac.Sum(nil)[:12])
	if id == value || a.byAlias[id] {
		return "", errors.New("pseudonym collision; prepare again")
	}
	b := deidBinding{kind, scope, value, id}
	a.size += len(mustJSON(b))
	if a.size > deidMapBytes/2 {
		return "", errors.New("identity correspondence exceeds bound")
	}
	a.entries = append(a.entries, b)
	a.byKey[key] = id
	a.byAlias[id] = true
	return id, nil
}
func deidCSV(ctx context.Context, rows [][]string) ([]byte, error) {
	b := &deidBuffer{ctx: ctx, limit: deidMaxBytes}
	w := csv.NewWriter(b)
	for _, r := range rows {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if e := w.Write(r); e != nil {
			return nil, e
		}
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func (a *app) prepareDeidentified(ctx context.Context, request deidRequest) (*deidPackage, error) {
	if request.DropText == nil || request.DropHLA == nil {
		return nil, errors.New("choose both free-text and HLA removal explicitly")
	}
	if request.What != "tables" && request.What != "clonotypes" {
		return nil, errors.New("choose tables or clonotypes; other formats have no de-identification contract")
	}
	if math.IsNaN(request.Filter.MinCount) || math.IsInf(request.Filter.MinCount, 0) || request.Filter.MinCount < 0 || request.Filter.MaxChains < 0 {
		return nil, errors.New("invalid QC thresholds")
	}
	for _, locus := range request.Filter.Loci {
		if rep.LocusCode(locus) == 0 {
			return nil, errors.New("QC locus must be a recognized receptor locus")
		}
	}
	if len(request.Annotations) > 1000 {
		return nil, errors.New("at most 1000 sample annotations")
	}
	ds, entry, _, e := a.clonotypeTable(clonotypeRequest{Dataset: request.Dataset, Definition: request.Definition, Filter: request.Filter})
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	rt, ct := ds.Result.Table, entry.table
	if len(rt.Samples) == 0 || len(rt.Samples) > 1000 || len(request.Annotations) != len(rt.Samples) {
		return nil, errors.New("supply exactly one annotation record for each sample, at most 1000")
	}
	def, e := request.Definition.Normalize()
	if e != nil {
		return nil, e
	}
	def.Name = ""
	notice := a.notice(ds, string(ct.Unit), request.Filter, def.Describe(), ct.DefinitionID)
	if notice.Restricted() && !request.Accept {
		return nil, errors.New("provider-restricted data require explicit private-use acknowledgment; pseudonyms do not change permissions")
	}
	annotations := map[string]map[string]string{}
	allowed := map[string]bool{}
	for _, f := range deidFields {
		allowed[f] = true
	}
	for _, row := range request.Annotations {
		if row.Sample == "" || annotations[row.Sample] != nil || row.Fields == nil {
			return nil, errors.New("duplicate or invalid sample annotation")
		}
		for k, v := range row.Fields {
			if !allowed[k] || len(v) > 8192 || !utf8.ValidString(v) {
				return nil, errors.New("unsupported or oversized sample annotation")
			}
		}
		annotations[row.Sample] = row.Fields
	}
	aliases, e := newDeidAliases()
	if e != nil {
		return nil, e
	}
	names := make([]string, len(rt.Samples))
	subjects := make([]string, len(rt.Samples))
	subjectSet := map[string]bool{}
	missing := 0
	originals := []string{}
	seenSamples := map[string]bool{}
	for i, s := range rt.Samples {
		fields, ok := annotations[s.Name]
		if !ok || seenSamples[s.Name] {
			return nil, errors.New("sample annotations do not match the open dataset")
		}
		seenSamples[s.Name] = true
		names[i], e = aliases.alias("sample", "", s.Name)
		if e != nil {
			return nil, e
		}
		subjects[i], e = aliases.alias("subject", "", fields["subject"])
		if e != nil {
			return nil, e
		}
		if subjects[i] == "" {
			missing++
		} else {
			subjectSet[subjects[i]] = true
			originals = append(originals, fields["subject"])
		}
		originals = append(originals, s.Name)
	}
	// Kept free text is not a hidden channel for a known original identifier. A
	// conservative collision is refused, not replaced inside scientific metadata.
	checkAnnotation := func(value string) error {
		for _, original := range originals {
			if original != "" && strings.Contains(value, original) {
				return errors.New("retained annotation contains an original subject/sample identifier; remove it or request field removal")
			}
		}
		return nil
	}
	sampleRows := [][]string{{"sample_id", "subject_id"}}
	for _, f := range deidFields {
		if f != "subject" && ((f == "hla" && !*request.DropHLA) || (f != "hla" && !*request.DropText)) {
			sampleRows[0] = append(sampleRows[0], f)
		}
	}
	for i, s := range rt.Samples {
		row := []string{names[i], subjects[i]}
		for _, f := range sampleRows[0][2:] {
			v := annotations[s.Name][f]
			if e := checkAnnotation(v); e != nil {
				return nil, e
			}
			if *request.DropHLA && strings.Contains(strings.ToUpper(v), "HLA") {
				return nil, errors.New("retained free text contains HLA; remove free text before export")
			}
			row = append(row, v)
		}
		sampleRows = append(sampleRows, row)
	}
	files := map[string][]byte{}
	files["samples.csv"], e = deidCSV(ctx, sampleRows)
	if e != nil {
		return nil, e
	}
	cloned := &deidBuffer{ctx: ctx, limit: deidMaxBytes}
	if e = export.Clonotypes(cloned, ct, names); e != nil {
		return nil, e
	}
	files["clonotypes.csv"] = cloned.Bytes()
	keep, _ := qc.Apply(rt, request.Filter)
	kept := 0
	for _, yes := range keep {
		if yes {
			kept++
		}
	}
	if request.What == "tables" {
		original := &deidBuffer{ctx: ctx, limit: deidMaxBytes}
		if e = export.Rearrangements(original, rt, keep, ct); e != nil {
			return nil, e
		}
		out := &deidBuffer{ctx: ctx, limit: deidMaxBytes}
		scan := bufio.NewScanner(bytes.NewReader(original.Bytes()))
		scan.Buffer(make([]byte, 64<<10), 8<<20)
		if !scan.Scan() {
			return nil, errors.New("missing rearrangement header")
		}
		header := scan.Text()
		columns := strings.Split(header, "\t")
		at := map[string]int{}
		for i, k := range columns {
			at[k] = i
		}
		if _, e = out.WriteString(header + "\n"); e != nil {
			return nil, e
		}
		sourceRow := 0
		for scan.Scan() {
			if e = ctx.Err(); e != nil {
				return nil, e
			}
			for sourceRow < len(keep) && !keep[sourceRow] {
				sourceRow++
			}
			if sourceRow >= len(keep) {
				return nil, errors.New("rearrangement row contract differs")
			}
			fields := strings.Split(scan.Text(), "\t")
			if len(fields) != len(columns) {
				return nil, errors.New("rearrangement column contract differs")
			}
			for _, key := range []string{"imgt_annotations", "germline_database"} {
				if i, ok := at[key]; ok && fields[i] != "" {
					return nil, errors.New("native IMGT/germline database text has no de-identification contract; choose clonotypes only or retain the original evidence privately")
				}
			}
			sample := int(rt.Sample[sourceRow])
			rowIndex := sourceRow
			sourceRow++
			for _, f := range []struct{ field, kind string }{{"sequence_id", "sequence"}, {"cell_id", "cell"}} {
				i, ok := at[f.field]
				if !ok {
					return nil, errors.New("missing identity column")
				}
				originalID := ""
				if f.kind == "sequence" && rt.SequenceID != nil {
					originalID = rt.IDs.Get(rt.SequenceID[rowIndex])
				}
				if f.kind == "cell" && rt.Cell != nil {
					originalID = rt.Dict.Get(rt.Cell[rowIndex])
				}
				fields[i], e = aliases.alias(f.kind, rt.Samples[sample].Name, originalID)
				if e != nil {
					return nil, e
				}
			}
			repertoireAt, ok := at["repertoire_id"]
			if !ok {
				return nil, errors.New("missing repertoire identity column")
			}
			fields[repertoireAt] = names[sample]
			if _, e = out.WriteString(strings.Join(fields, "\t") + "\n"); e != nil {
				return nil, e
			}
		}
		if e = scan.Err(); e != nil {
			return nil, e
		}
		for sourceRow < len(keep) && !keep[sourceRow] {
			sourceRow++
		}
		if sourceRow != len(keep) {
			return nil, errors.New("incomplete rearrangement rows")
		}
		files["rearrangements.tsv"] = out.Bytes()
	}
	// Original repertoire identifiers live only in the separate audit map.
	for i, s := range rt.Samples {
		if s.RepertoireID != "" {
			binding := deidBinding{"repertoire", s.Name, s.RepertoireID, names[i]}
			aliases.size += len(mustJSON(binding))
			if len(aliases.entries) >= deidMaxBindings || aliases.size > deidMapBytes/2 {
				return nil, errors.New("identity correspondence exceeds bound")
			}
			aliases.entries = append(aliases.entries, binding)
		}
	}
	filter := request.Filter
	filter.ExcludeSamples = append([]string(nil), filter.ExcludeSamples...)
	for i, name := range filter.ExcludeSamples {
		found := false
		for j, s := range rt.Samples {
			if s.Name == name {
				filter.ExcludeSamples[i] = names[j]
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("QC exclusion names a sample outside this dataset")
		}
	}
	record := deidRecord{Format: "clonoloom-deidentified-tables", Version: 1, Scope: "native-table-copy-with-pseudonymized-identifiers", Software: artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}, What: request.What, DropText: *request.DropText, DropHLA: *request.DropHLA, Unit: string(ct.Unit), Definition: def, DefinitionID: ct.DefinitionID, Filter: filter, Samples: len(names), Subjects: len(subjectSet), MissingSubjects: missing, Rows: kept, Clonotypes: len(ct.Clonotypes), Credits: notice.Credits, Restricted: notice.Restricted(), Bindings: map[string]int{}, Limits: deidLimits, Removed: []string{"original subject/sample/sequence/cell/repertoire identifiers", "project, dataset and definition labels", "paths, source fingerprints and original filenames", "raw metadata, history, recipes, artifacts, checkpoints, certificates and figures"}}
	if record.DropText {
		record.Removed = append(record.Removed, "timepoint, tissue, reference, cellSubset, condition, batch annotations")
	}
	if record.DropHLA {
		record.Removed = append(record.Removed, "hla annotation")
	}
	randomID := make([]byte, 16)
	if _, e = rand.Read(randomID); e != nil {
		return nil, e
	}
	record.ExportID = hex.EncodeToString(randomID)
	for _, b := range aliases.entries {
		record.Bindings[b.Kind]++
	}
	// Notices contain only controlled scientific provenance and the known provider credits.
	notice.What = "Pseudonymized native dataset tables. " + deidLimits
	notice.Dataset = "Pseudonymized dataset"
	notice.DatasetID = record.ExportID
	notice.Files = nil
	notice.Filter = filter.Describe()
	notice.Definition = def.Describe()
	notice.Method = "Identifier transformation v1; scientific table values copied from the native export. NIST SP 800-188, doi:10.6028/NIST.SP.800-188. AIRR schema 2.0, doi:10.3389/fimmu.2018.02206."
	notice.Made = time.Now()
	files["NOTICE.txt"] = []byte(notice.Text())
	// Provider credits cannot be silently rewritten. Refuse known identifier collisions
	// there, rather than dropping attribution to make a privacy claim.
	for _, original := range originals {
		if len(original) >= 4 && strings.Contains(string(mustJSON(record.Credits)), original) {
			return nil, errors.New("provider credits contain a selected identifier; this attribution cannot be removed automatically")
		}
	}
	paths := []string{}
	total := 0
	for name, b := range files {
		total += len(b)
		paths = append(paths, name)
		record.Files = append(record.Files, runOutput{name, int64(len(b)), replayDigest(b)})
	}
	if total > deidMaxBytes {
		return nil, errors.New("export exceeds 64 MiB of table/notice bytes")
	}
	sort.Strings(paths)
	sort.Slice(record.Files, func(i, j int) bool { return record.Files[i].Path < record.Files[j].Path })
	rawRecord := mustJSON(record)
	fingerprint := replayDigest(rawRecord)
	files["transformation.json"] = rawRecord
	paths = append(paths, "transformation.json")
	zipped := &deidBuffer{ctx: ctx, limit: deidMaxBytes + (4 << 20)}
	z := zip.NewWriter(zipped)
	for _, name := range paths {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(w, runContextReader{ctx, bytes.NewReader(files[name])}); err != nil {
			return nil, err
		}
	}
	if e = z.Close(); e != nil {
		return nil, e
	}
	identity := mustJSON(struct {
		Format      string           `json:"format"`
		Version     int              `json:"version"`
		Fingerprint string           `json:"bundleFingerprint"`
		Dataset     string           `json:"originalDataset"`
		Sources     any              `json:"sources"`
		Metadata    any              `json:"metadata"`
		Annotations []deidAnnotation `json:"annotations"`
		Bindings    []deidBinding    `json:"bindings"`
		Limits      string           `json:"limits"`
	}{"clonoloom-private-identity-map", 1, fingerprint, ds.ID, ds.Result.Sources, ds.Result.Metadata, request.Annotations, aliases.entries, "PRIVATE correspondence: never distribute with the export. This file identifies original subjects and sources; it is not de-identified."})
	if len(identity) > deidMapBytes {
		return nil, errors.New("identity map exceeds 32 MiB")
	}
	token := make([]byte, 32)
	if _, e = rand.Read(token); e != nil {
		return nil, e
	}
	expires := time.Now().Add(deidLifetime)
	preview := deidPreview{Token: hex.EncodeToString(token), Fingerprint: fingerprint, Expires: expires.UTC().Format(time.RFC3339), Record: record, Subjects: []deidBinding{}}
	for _, b := range aliases.entries {
		if b.Kind == "subject" {
			preview.Subjects = append(preview.Subjects, b)
		}
	}
	return &deidPackage{preview, zipped.Bytes(), identity, expires}, ctx.Err()
}

package reads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"clonoloom/internal/airr"
	"clonoloom/internal/rep"
)

// Format is a source's format, as detected from its header.
type Format string

const (
	FormatAIRR        Format = "airr"
	FormatCellRanger  Format = "cellranger-contigs"
	FormatImmunoSEQv3 Format = "immunoseq-v3"
	FormatImmunoSEQv2 Format = "immunoseq-v2"
)

// FormatNames are formats' names as the interface and methods text say them.
var FormatNames = map[Format]string{
	FormatAIRR:        "AIRR Rearrangement TSV",
	FormatCellRanger:  "Cell Ranger V(D)J contig annotations",
	FormatImmunoSEQv3: "immunoSEQ export (v3)",
	FormatImmunoSEQv2: "immunoSEQ export (v2)",
}

// ReaderVersion changes when a reader reads any file differently; cached tables and records name
// it.
const ReaderVersion = 1

// Source is one file of a dataset as read.
type Source struct {
	// Size, Modified and SHA256 fingerprint the file as read (the bytes on disk, compressed or
	// not), so that a project knows when its data changed or moved.
	Size     int64    `json:"size"`
	Modified string   `json:"modified,omitempty"`
	SHA256   string   `json:"sha256,omitempty"`
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Format   Format   `json:"format"`
	Gzipped  bool     `json:"gzipped,omitempty"`
	Rows     int      `json:"rows"`
	Header   []string `json:"header,omitempty"`
	// GeneNames are a vendor\'s gene names and the IMGT names ClonoLoom read them as.
	GeneNames map[string]string `json:"geneNames,omitempty"`
}

// Result is a dataset's sources read into one table.
type Result struct {
	Table    *rep.Table
	Sources  []Source
	Problems *Problems
	// Unit: the abundance unit the formats give, or unknown when they leave it open or disagree.
	Unit rep.Unit
	// Companions: files read beside the rearrangements (Cell Ranger's clonotypes and metrics).
	Companions []Companion
	// Repertoires: AIRR Repertoire metadata named with the tables.
	Repertoires []airr.Repertoire
}

// ErrUnknownFormat: the header is not one ClonoLoom reads.
var ErrUnknownFormat = errors.New("not a format ClonoLoom reads")

// Input is a file to read: its path, and the name to call it by when that is not the path's
// (a file the page uploaded to the library is stored under its SHA-256).
type Input struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// ReadFiles reads files into one table, each a source of the dataset, in the order given.
func ReadFiles(ctx context.Context, paths []string) (*Result, error) {
	inputs := make([]Input, len(paths))
	for i, p := range paths {
		inputs[i] = Input{Path: p}
	}
	return ReadInputs(ctx, inputs)
}

// ReadInputs reads files into one table, each a source of the dataset, fingerprinting each file
// as it is read.
func ReadInputs(ctx context.Context, inputs []Input) (*Result, error) {
	res := &Result{Problems: newProblems()}
	var b *rep.Builder
	units := make([]rep.Unit, len(inputs))
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
		hashed := &hashingReader{r: f, h: sha256.New()}
		src, unit, err := readOne(ctx, hashed, name, i, &b, res.Problems)
		if err == nil {
			// The rest of the file (a reader may stop before its end) completes the fingerprint.
			_, err = io.Copy(io.Discard, hashed)
		}
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(name), err)
		}
		src.Path, src.Name = in.Path, filepath.Base(name)
		src.Size, src.Modified, src.SHA256 = hashed.n, info.ModTime().UTC().Format(time.RFC3339), hex.EncodeToString(hashed.h.Sum(nil))
		units[i] = unit
		res.Sources = append(res.Sources, src)
	}
	if b == nil {
		b = rep.NewBuilder(rep.UnitUnknown)
	}
	res.Unit = agreedUnit(units, b, res.Sources, res.Problems)
	res.Table = b.Table()
	res.Table.Unit = res.Unit
	return res, nil
}

// agreedUnit is the dataset's unit: its sources' when they agree. Sources that disagree are
// counted alike where they can be: immunoSEQ files some of which lack templates are all counted in
// reads. Otherwise the unit is unknown, and the validator says why.
func agreedUnit(units []rep.Unit, b *rep.Builder, sources []Source, p *Problems) rep.Unit {
	if len(units) == 0 {
		return rep.UnitUnknown
	}
	first, same := units[0], true
	for _, u := range units[1:] {
		same = same && u == first
	}
	if same {
		return first
	}
	immunoSEQ := true
	for _, s := range sources {
		immunoSEQ = immunoSEQ && (s.Format == FormatImmunoSEQv3 || s.Format == FormatImmunoSEQv2)
	}
	if immunoSEQ {
		templates := 0
		for i, u := range units {
			if u == rep.Templates {
				templates++
				b.CountReads(i)
			}
		}
		p.addf("mixed-units", 0, 0, "templates", "", "%d of %d files have templates for every row and the others do not: every file is counted in reads, so that samples compare", templates, len(units))
		return rep.Reads
	}
	p.addf("mixed-units", 0, 0, "", "", "the files count different things (%s): compare their samples only within a unit", unitList(units))
	return rep.UnitUnknown
}

func unitList(units []rep.Unit) string {
	seen := map[rep.Unit]bool{}
	var out []string
	for _, u := range units {
		if !seen[u] {
			seen[u] = true
			name := string(u)
			if name == "" {
				name = "unknown"
			}
			out = append(out, name)
		}
	}
	return strings.Join(out, ", ")
}

// hashingReader hashes what is read through it.
type hashingReader struct {
	r io.Reader
	h hash.Hash
	n int64
}

func (h *hashingReader) Read(p []byte) (int, error) {
	n, err := h.r.Read(p)
	h.h.Write(p[:n])
	h.n += int64(n)
	return n, err
}

// ReadReader reads one source from r (tests, and files inside archives).
func ReadReader(ctx context.Context, r io.Reader, name string) (*Result, error) {
	res := &Result{Problems: newProblems()}
	var b *rep.Builder
	src, unit, err := readOne(ctx, r, name, 0, &b, res.Problems)
	if err != nil {
		return nil, err
	}
	res.Sources = []Source{src}
	res.Unit = unit
	res.Table = b.Table()
	res.Table.Unit = unit
	return res, nil
}

func readOne(ctx context.Context, r io.Reader, path string, index int, b **rep.Builder, p *Problems) (Source, rep.Unit, error) {
	src := Source{Path: path, Name: filepath.Base(path)}
	plain, gzipped, err := decompress(r)
	if err != nil {
		return src, "", err
	}
	src.Gzipped = gzipped
	ls := newLines(plain)
	ls.ctx = ctx
	if !ls.next() {
		if ls.err != nil {
			return src, "", ls.err
		}
		return src, "", errors.New("the file is empty")
	}
	sample := sampleName(path)
	if bytes.IndexByte(ls.line, '\t') >= 0 {
		header := splitTab(ls.line, nil)
		for i := range header {
			header[i] = strings.TrimSpace(header[i])
		}
		src.Header = header
		if kind := companionKind(header); kind != "" {
			return src, "", ErrCompanion{Kind: kind}
		}
		if version := isImmunoSEQHeader(header); version > 0 {
			src.Format = FormatImmunoSEQv3
			if version == 2 {
				src.Format = FormatImmunoSEQv2
			}
			if *b == nil {
				*b = rep.NewBuilder(rep.UnitUnknown)
			}
			before := (*b).Len()
			src.GeneNames = map[string]string{}
			layout := newImmunoSEQLayout(header, version)
			with, without, err := readImmunoSEQ(ls, layout, len(header), *b, index, sample, src.GeneNames, p)
			if err != nil {
				return src, "", err
			}
			src.Rows = (*b).Len() - before
			switch {
			case without == 0:
				return src, rep.Templates, nil
			case with > 0:
				p.addf("templates-missing", index, 0, "templates", "", "%d of %d rows have no template count: the file is counted in reads", without, with+without)
			}
			(*b).CountReads(index)
			if layout.count >= 0 && layout.countUnit != rep.Reads {
				// A v2 export's "count (templates/reads)" without estimatedNumberGenomes does not
				// say which it counts.
				return src, rep.UnitUnknown, nil
			}
			return src, rep.Reads, nil
		}
		if isAIRRHeader(header) {
			src.Format = FormatAIRR
			layout := newAIRRLayout(header)
			if *b == nil {
				*b = rep.NewBuilder(rep.UnitUnknown)
			}
			before := (*b).Len()
			cells, counted, err := readAIRR(ls, layout, *b, index, sample, p)
			if err != nil {
				return src, "", err
			}
			src.Rows = (*b).Len() - before
			return src, layout.unit(cells > 0, counted, index, p), nil
		}
	}
	header, ok := splitCSV(ls.line, nil)
	if kind := companionKind(header); ok && kind != "" {
		return src, "", ErrCompanion{Kind: kind}
	}
	if ok && isCellRangerContigHeader(header) {
		for i := range header {
			header[i] = strings.TrimSpace(header[i])
		}
		src.Header, src.Format = header, FormatCellRanger
		if *b == nil {
			*b = rep.NewBuilder(rep.Cells)
		}
		before := (*b).Len()
		if err := readCellRanger(ls, newCellRangerLayout(header), len(header), *b, index, sample, p); err != nil {
			return src, "", err
		}
		src.Rows = (*b).Len() - before
		return src, rep.Cells, nil
	}
	return src, "", ErrUnknownFormat
}

// sampleName is a sample's name from its file's name: without folders and without the extensions
// of tables and compression ("Subject07_day28.tsv.gz" is "Subject07_day28"). Cell Ranger's files
// have standard names, so their sample is named by the prefix 10x's downloads give them
// ("sc5p_v2_hs_PBMC_10k_t_filtered_contig_annotations.csv") or by the run's folder
// (".../PBMC_10k/outs/filtered_contig_annotations.csv", ".../per_sample_outs/S1/vdj_t/...").
func sampleName(path string) string {
	name := filepath.Base(path)
	for {
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".gz", ".tsv", ".csv", ".txt", ".tab", ".airr":
			name = name[:len(name)-len(ext)]
			continue
		}
		break
	}
	for _, standard := range cellRangerNames {
		if name == standard {
			return runName(filepath.Dir(path), name)
		}
		if prefix, ok := strings.CutSuffix(name, "_"+standard); ok && prefix != "" {
			return prefix
		}
	}
	return name
}

// Cell Ranger's standard file names (without extensions).
var cellRangerNames = []string{"filtered_contig_annotations", "all_contig_annotations", "airr_rearrangement", "clonotypes", "consensus_annotations", "metrics_summary"}

// runName is a Cell Ranger run's or sample's name from the folder its files are in: the first
// folder up that is not one of Cell Ranger's own (outs, vdj_t, vdj_b, vdj_t_gd, per_sample_outs,
// multi).
func runName(dir, fallback string) string {
	for i := 0; i < 6 && dir != "" && dir != "." && dir != string(filepath.Separator); i++ {
		base := filepath.Base(dir)
		switch base {
		case "outs", "vdj_t", "vdj_b", "vdj_t_gd", "per_sample_outs", "multi", "vdj":
			dir = filepath.Dir(dir)
			continue
		}
		return base
	}
	return fallback
}

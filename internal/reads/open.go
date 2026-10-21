package reads

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"clonoloom/internal/airr"
)

// Companion is a file read beside a dataset's rearrangements rather than as them: a Cell Ranger
// clonotype summary or metrics, which QC compares with what ClonoLoom computes.
type Companion struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Sample string `json:"sample"`
}

// Companion kinds.
const (
	CompanionClonotypes = "cellranger-clonotypes"
	CompanionMetrics    = "cellranger-metrics"
	CompanionConsensus  = "cellranger-consensus"
)

// ErrCompanion: a file that is not rearrangements but goes with them.
type ErrCompanion struct{ Kind string }

func (e ErrCompanion) Error() string {
	switch e.Kind {
	case CompanionClonotypes:
		return "a Cell Ranger clonotype summary: open its run's folder or its contig annotations (filtered_contig_annotations.csv)"
	case CompanionMetrics:
		return "a Cell Ranger metrics summary: open its run's folder or its contig annotations"
	case CompanionConsensus:
		return "Cell Ranger's consensus annotations: open its run's folder or its contig annotations"
	}
	return "a file that goes with rearrangements: open them with it"
}

// companionKind recognizes Cell Ranger's companion tables by their headers.
func companionKind(header []string) string {
	c := newColumns(header)
	switch {
	case c.index("clonotype_id") >= 0 && c.index("cdr3s_aa") >= 0:
		return CompanionClonotypes
	case c.index("consensus_id") >= 0 && c.index("clonotype_id") >= 0:
		return CompanionConsensus
	case c.index("Estimated Number of Cells") >= 0 || (c.index("Category") >= 0 && c.index("Metric Name") >= 0) || (c.index("Library or Sample") >= 0 && c.index("Metric Name") >= 0):
		return CompanionMetrics
	}
	return ""
}

// Open reads what a user named as one dataset: rearrangement tables, Cell Ranger output folders
// (each expanded to its contig annotations), AIRR Repertoire metadata (matched to the samples by
// repertoire_id) and companion files, in the order given.
func Open(ctx context.Context, paths []string) (*Result, error) {
	var tables, metadata []string
	var companions []Companion
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			contigs, more, err := CellRangerFiles(path)
			if err != nil {
				return nil, err
			}
			tables = append(tables, contigs...)
			companions = append(companions, more...)
			continue
		}
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".json.gz") {
			metadata = append(metadata, path)
			continue
		}
		if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
			return nil, fmt.Errorf("%s: AIRR metadata in YAML are not read yet (wave 9): save them as JSON to open them with the tables", filepath.Base(path))
		}
		if kind, err := sniffCompanion(path); err == nil && kind != "" {
			companions = append(companions, Companion{Path: path, Kind: kind, Sample: sampleName(path)})
			continue
		}
		tables = append(tables, path)
		companions = append(companions, besideContigs(path)...)
	}
	if len(tables) == 0 {
		if len(companions) > 0 {
			return nil, ErrCompanion{Kind: companions[0].Kind}
		}
		return nil, errors.New("no rearrangement tables among the files named")
	}
	res, err := ReadFiles(ctx, tables)
	if err != nil {
		return nil, err
	}
	res.Companions = companions
	for _, path := range metadata {
		data, err := readMaybeGzip(path)
		if err != nil {
			return nil, err
		}
		reps, err := airr.ReadRepertoires(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		res.Repertoires = append(res.Repertoires, reps...)
	}
	return res, nil
}

// besideContigs finds the companions of a Cell Ranger contig file named on its own: in its folder,
// under its own prefix ("<run>_clonotypes.csv" beside "<run>_filtered_contig_annotations.csv", as
// 10x's downloads name them) or under Cell Ranger's own names.
func besideContigs(path string) []Companion {
	base := filepath.Base(path)
	prefix := ""
	switch {
	case strings.HasSuffix(base, "filtered_contig_annotations.csv"):
		prefix = strings.TrimSuffix(base, "filtered_contig_annotations.csv")
	case strings.HasSuffix(base, "all_contig_annotations.csv"):
		prefix = strings.TrimSuffix(base, "all_contig_annotations.csv")
	default:
		return nil
	}
	dir, sample := filepath.Dir(path), sampleName(path)
	var out []Companion
	for _, c := range []struct{ name, kind string }{{"clonotypes.csv", CompanionClonotypes}, {"consensus_annotations.csv", CompanionConsensus}, {"metrics_summary.csv", CompanionMetrics}} {
		if p := filepath.Join(dir, prefix+c.name); isFile(p) {
			out = append(out, Companion{Path: p, Kind: c.kind, Sample: sample})
		}
	}
	return out
}

// sniffCompanion reads a table's header to tell a companion file from rearrangements.
func sniffCompanion(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	plain, _, err := decompress(f)
	if err != nil {
		return "", err
	}
	ls := newLines(plain)
	if !ls.next() {
		return "", ls.err
	}
	var header []string
	if bytes.IndexByte(ls.line, '\t') >= 0 {
		header = splitTab(ls.line, nil)
	} else {
		header, _ = splitCSV(ls.line, nil)
	}
	return companionKind(header), nil
}

func readMaybeGzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	head := make([]byte, 2)
	if n, _ := io.ReadFull(f, head); n == 2 && head[0] == 0x1f && head[1] == 0x8b {
		f.Seek(0, io.SeekStart)
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		r = gz
	} else {
		f.Seek(0, io.SeekStart)
	}
	data, err := io.ReadAll(io.LimitReader(r, maxMetadataBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxMetadataBytes {
		return nil, errors.New("the metadata file is larger than 64 MB")
	}
	return data, nil
}

const maxMetadataBytes = 64 << 20

// CellRangerFiles finds a Cell Ranger output folder's contig annotations and their companions:
// the folder itself or its outs/ (a vdj run), and a multi run's per_sample_outs/<sample>/vdj_t,
// vdj_b and vdj_t_gd folders. Filtered annotations are preferred to all contigs.
func CellRangerFiles(dir string) (contigs []string, companions []Companion, err error) {
	var folders []string
	for _, base := range []string{dir, filepath.Join(dir, "outs")} {
		if isFile(filepath.Join(base, "filtered_contig_annotations.csv")) || isFile(filepath.Join(base, "all_contig_annotations.csv")) {
			folders = append(folders, base)
		}
		for _, pattern := range []string{"per_sample_outs/*/vdj_t", "per_sample_outs/*/vdj_b", "per_sample_outs/*/vdj_t_gd"} {
			matches, _ := filepath.Glob(filepath.Join(base, pattern))
			sort.Strings(matches)
			folders = append(folders, matches...)
		}
	}
	for _, folder := range folders {
		contig := filepath.Join(folder, "filtered_contig_annotations.csv")
		if !isFile(contig) {
			contig = filepath.Join(folder, "all_contig_annotations.csv")
		}
		if !isFile(contig) {
			continue
		}
		contigs = append(contigs, contig)
		sample := sampleName(contig)
		for _, c := range []struct{ name, kind string }{{"clonotypes.csv", CompanionClonotypes}, {"consensus_annotations.csv", CompanionConsensus}, {"metrics_summary.csv", CompanionMetrics}} {
			if p := filepath.Join(folder, c.name); isFile(p) {
				companions = append(companions, Companion{Path: p, Kind: c.kind, Sample: sample})
			}
		}
		// A multi run's metrics are the sample's, beside its vdj folders.
		if p := filepath.Join(filepath.Dir(folder), "metrics_summary.csv"); strings.HasPrefix(filepath.Base(folder), "vdj_") && isFile(p) {
			companions = append(companions, Companion{Path: p, Kind: CompanionMetrics, Sample: sample})
		}
	}
	if len(contigs) == 0 {
		return nil, nil, fmt.Errorf("%s: no Cell Ranger contig annotations (filtered_contig_annotations.csv) in the folder, its outs/ or per_sample_outs/", filepath.Base(dir))
	}
	return contigs, companions, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

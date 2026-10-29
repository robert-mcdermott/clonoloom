package main

// Datasets (clonoloom-spec/requirements.md, D1–D11): what a user opens, read where it is by the
// readers (internal/reads), fingerprinted, validated (internal/validate) and kept in memory while
// the program runs. A dataset's ID is made from its files' SHA-256 and the readers' version, so the
// same files open as the same dataset. The page records each dataset in its project by its paths
// and fingerprints, and opens it again from them; a file that moved is found again by its
// fingerprint.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"clonoloom/internal/airr"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
	"clonoloom/internal/validate"
)

// dataset is an opened dataset.
type dataset struct {
	ID     string
	Name   string
	Inputs []reads.Input
	Result *reads.Result
	Report validate.Report
	Opened time.Time
	// Credits are the examples its files are, with their sources, licenses and citations.
	Credits []credit
}

type datasets struct {
	mu   sync.Mutex
	byID map[string]*dataset
	// opening serializes opening the same files twice at once.
	opening sync.Mutex
}

func newDatasets() *datasets {
	return &datasets{byID: map[string]*dataset{}}
}

func (d *datasets) get(id string) (*dataset, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	ds, ok := d.byID[id]
	return ds, ok
}

func (d *datasets) put(ds *dataset) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.byID[ds.ID] = ds
}

func (d *datasets) remove(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.byID[id]
	delete(d.byID, id)
	return ok
}

// datasetID is made from the readers' identity version and each source's SHA-256, in order.
func datasetID(sources []reads.Source, metadata []reads.MetadataFile) string {
	h := sha256.New()
	fmt.Fprintf(h, "clonoloom-dataset/1 readers %d\n", reads.IdentityVersion)
	for _, s := range sources {
		fmt.Fprintf(h, "%s %d\n", s.SHA256, s.Size)
		if s.IMGTGroup != "" {
			fmt.Fprintf(h, "imgt-group %q\n", s.IMGTGroup)
		}
		// A table read through a mapping of its columns is another dataset for each mapping.
		if s.Mapping != nil {
			raw, _ := json.Marshal(s.Mapping)
			fmt.Fprintf(h, "mapping %s\n", raw)
		}
	}
	// Metadata read with the tables make another dataset of them (one whose samples are
	// described); the tables alone keep the ID they always had.
	for _, m := range metadata {
		fmt.Fprintf(h, "metadata %s %d\n", m.SHA256, m.Size)
	}
	return "d-" + hex.EncodeToString(h.Sum(nil))[:16]
}

// openDataset reads the inputs as one dataset, validates it and registers it.
func (a *app) openDataset(ctx context.Context, paths []string, inputs []reads.Input, name string) (*dataset, error) {
	a.datasets.opening.Lock()
	defer a.datasets.opening.Unlock()
	if len(inputs) == 0 {
		for _, p := range paths {
			inputs = append(inputs, reads.Input{Path: p})
		}
	}
	// The tables from the cache when it holds them (internal/cache), else read.
	readTables := reads.ReadInputs
	if a.cache != nil {
		readTables = a.cache.Tables
	}
	res, err := reads.OpenInputsWith(ctx, inputs, readTables)
	if err != nil {
		return nil, err
	}
	ds := &dataset{ID: datasetID(res.Sources, res.Metadata), Result: res, Report: validate.Check(res), Opened: time.Now().UTC()}
	var sums []string
	for _, s := range res.Sources {
		ds.Inputs = append(ds.Inputs, reads.Input{Path: s.Path, Name: s.Name, Mapping: s.Mapping, IMGTGroup: s.IMGTGroup})
		sums = append(sums, s.SHA256)
	}
	for _, m := range res.Metadata {
		ds.Inputs = append(ds.Inputs, reads.Input{Path: m.Path, Name: m.Name})
	}
	ds.Credits = a.credits(sums)
	ds.Name = name
	if ds.Name == "" {
		ds.Name = defaultDatasetName(paths, res)
	}
	if old, ok := a.datasets.get(ds.ID); ok {
		// The same files opened again: the dataset already read (its name as now asked).
		if name != "" {
			old.Name = name
		}
		return old, nil
	}
	a.datasets.put(ds)
	return ds, nil
}

// defaultDatasetName: a Cell Ranger output's or a folder's name, a single file's sample, or the
// folder the files share.
func defaultDatasetName(paths []string, res *reads.Result) string {
	if len(paths) == 1 {
		if info, err := os.Stat(paths[0]); err == nil && info.IsDir() {
			return filepath.Base(paths[0])
		}
	}
	if len(res.Table.Samples) == 1 {
		return res.Table.Samples[0].Name
	}
	if len(res.Sources) > 0 {
		dir := filepath.Dir(res.Sources[0].Path)
		same := true
		for _, s := range res.Sources[1:] {
			same = same && filepath.Dir(s.Path) == dir
		}
		if same {
			return filepath.Base(dir)
		}
	}
	return fmt.Sprintf("%d files", len(res.Sources))
}

// sampleSummary is what the Data view shows of a sample before QC.
type sampleSummary struct {
	Name         string            `json:"name"`
	RepertoireID string            `json:"repertoireId,omitempty"`
	Source       int               `json:"source"`
	Rows         int               `json:"rows"`
	Productive   int               `json:"productive"`
	Abundance    float64           `json:"abundance"`
	Cells        int               `json:"cells,omitempty"`
	Loci         map[string]int    `json:"loci"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Repertoire   *airr.Repertoire  `json:"repertoire,omitempty"`
}

type sourceSummary struct {
	IMGTGroup            string            `json:"imgtGroup,omitempty"`
	AnnotationParameters string            `json:"annotationParameters,omitempty"`
	Path                 string            `json:"path"`
	Name                 string            `json:"name"`
	Format               reads.Format      `json:"format"`
	FormatName           string            `json:"formatName"`
	Gzipped              bool              `json:"gzipped,omitempty"`
	Rows                 int               `json:"rows"`
	Size                 int64             `json:"size"`
	Modified             string            `json:"modified,omitempty"`
	SHA256               string            `json:"sha256"`
	GeneNames            map[string]string `json:"geneNames,omitempty"`
	Mapping              *reads.Mapping    `json:"mapping,omitempty"`
}

type datasetSummary struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Unit       rep.Unit          `json:"unit"`
	Formats    []string          `json:"formats"`
	Rows       int               `json:"rows"`
	SingleCell bool              `json:"singleCell"`
	Sources    []sourceSummary   `json:"sources"`
	Samples    []sampleSummary   `json:"samples"`
	Companions []reads.Companion `json:"companions"`
	// Metadata: the AIRR Repertoire metadata files read with the tables.
	Metadata      []reads.MetadataFile `json:"metadata"`
	Report        validate.Report      `json:"report"`
	ReaderVersion int                  `json:"readerVersion"`
	Bytes         int64                `json:"bytes"`
	Opened        string               `json:"opened"`
	Credits       []credit             `json:"credits,omitempty"`
}

func (ds *dataset) summary() datasetSummary {
	res, t := ds.Result, ds.Result.Table
	s := datasetSummary{ID: ds.ID, Name: ds.Name, Unit: res.Unit, Rows: t.Len(), SingleCell: t.Cell != nil, Report: ds.Report,
		ReaderVersion: reads.ReaderVersion, Bytes: t.Size(), Opened: ds.Opened.Format(time.RFC3339), Companions: res.Companions, Credits: ds.Credits}
	if s.Companions == nil {
		s.Companions = []reads.Companion{}
	}
	s.Metadata = res.Metadata
	if s.Metadata == nil {
		s.Metadata = []reads.MetadataFile{}
	}
	formats := map[string]bool{}
	for _, src := range res.Sources {
		name := reads.FormatNames[src.Format]
		if !formats[name] {
			formats[name] = true
			s.Formats = append(s.Formats, name)
		}
		s.Sources = append(s.Sources, sourceSummary{Path: src.Path, Name: src.Name, Format: src.Format, FormatName: name, Gzipped: src.Gzipped,
			Rows: src.Rows, Size: src.Size, Modified: src.Modified, SHA256: src.SHA256, GeneNames: src.GeneNames, Mapping: src.Mapping, IMGTGroup: src.IMGTGroup, AnnotationParameters: src.AnnotationParameters})
	}
	repertoires := map[string]*airr.Repertoire{}
	for i := range res.Repertoires {
		repertoires[res.Repertoires[i].RepertoireID] = &res.Repertoires[i]
	}
	s.Samples = make([]sampleSummary, len(t.Samples))
	cells := make([]map[uint32]bool, len(t.Samples))
	for i, sample := range t.Samples {
		s.Samples[i] = sampleSummary{Name: sample.Name, RepertoireID: sample.RepertoireID, Source: sample.Source, Loci: map[string]int{}, Metadata: sample.Metadata}
		s.Samples[i].Repertoire, _ = repertoireMatch(sample, repertoires)
		if t.Cell != nil {
			cells[i] = map[uint32]bool{}
		}
	}
	for row := 0; row < t.Len(); row++ {
		ss := &s.Samples[t.Sample[row]]
		ss.Rows++
		if t.Productive[row] == rep.True {
			ss.Productive++
		}
		if l := t.LocusName(row); l != "" {
			ss.Loci[l]++
		}
		if t.Cell != nil {
			if c := t.Cell[row]; c != 0 {
				cells[t.Sample[row]][c] = true
			}
		} else if v := t.Count[row]; !math.IsNaN(v) {
			ss.Abundance += v
		}
	}
	for i := range s.Samples {
		if cells[i] != nil {
			s.Samples[i].Cells = len(cells[i])
			s.Samples[i].Abundance = float64(len(cells[i]))
		}
	}
	return s
}

func (a *app) registerDatasets(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/datasets/open", a.handleOpenDataset)
	mux.HandleFunc("GET /api/datasets", a.handleListDatasets)
	mux.HandleFunc("GET /api/datasets/{id}", a.handleGetDataset)
	mux.HandleFunc("GET /api/datasets/{id}/metadata", a.handleMetadata)
	mux.HandleFunc("GET /api/datasets/{id}/airr-objects", a.handleAIRRObjects)
	mux.HandleFunc("DELETE /api/datasets/{id}", a.handleCloseDataset)
	mux.HandleFunc("POST /api/datasets/locate", a.handleLocate)
	mux.HandleFunc("POST /api/datasets/inspect", a.handleInspect)
	mux.HandleFunc("POST /api/datasets/check", a.handleCheckMapping)
}

// handleInspect answers a table's separator, header, first rows and a guessed mapping of its
// columns, and the fields a mapping can give: {"path"} or {"sha256"} (a file in the library).
func (a *app) handleInspect(w http.ResponseWriter, r *http.Request) {
	path, ok := a.mappedPath(w, r, nil)
	if !ok {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer f.Close()
	in, err := reads.Inspect(f, 12)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, map[string]any{"inspection": in, "fields": reads.MappableFields})
}

// handleCheckMapping reads a whole table through a mapping and answers each mapped column checked:
// {"path" or "sha256", "mapping"}.
func (a *app) handleCheckMapping(w http.ResponseWriter, r *http.Request) {
	var mapping reads.Mapping
	path, ok := a.mappedPath(w, r, &mapping)
	if !ok {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer f.Close()
	check, err := reads.Check(r.Context(), f, path, mapping)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, check)
}

// mappedPath reads a request naming one file on this computer ("path") or in the library
// ("sha256"), and its mapping when asked for.
func (a *app) mappedPath(w http.ResponseWriter, r *http.Request, mapping *reads.Mapping) (string, bool) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "Only this computer may read files.")
		return "", false
	}
	var body struct {
		Path    string         `json:"path"`
		SHA256  string         `json:"sha256"`
		Mapping *reads.Mapping `json:"mapping"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Send {\"path\"} or {\"sha256\"}.")
		return "", false
	}
	path := body.Path
	if body.SHA256 != "" {
		p, ok := a.store.filePath(body.SHA256)
		if !ok || !isFile(p) {
			writeError(w, http.StatusNotFound, "The file is not in the library.")
			return "", false
		}
		path = p
	} else {
		abs, err := absPath(path)
		if err != nil || strings.TrimSpace(path) == "" || !isFile(abs) {
			writeError(w, http.StatusBadRequest, "Invalid path: "+path)
			return "", false
		}
		path = abs
	}
	if mapping != nil {
		if body.Mapping == nil {
			writeError(w, http.StatusBadRequest, "Send the mapping.")
			return "", false
		}
		*mapping = *body.Mapping
	}
	return path, true
}

// handleOpenDataset opens {"paths": [...]} (files and folders on this computer, as one dataset),
// or {"library": [{"sha256", "name"}]} (files the page added to the library), with an optional
// name, and answers the dataset's summary. Only this computer may name paths.
func (a *app) handleOpenDataset(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "Only this computer may open files.")
		return
	}
	var body struct {
		Paths   []string      `json:"paths"`
		Inputs  []reads.Input `json:"inputs"`
		Library []struct {
			SHA256  string         `json:"sha256"`
			Name    string         `json:"name"`
			Mapping *reads.Mapping `json:"mapping"`
		} `json:"library"`
		Name string `json:"name"`
		// Mappings: by path, how to read a table ClonoLoom does not recognize.
		Mappings map[string]reads.Mapping `json:"mappings"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Send {\"paths\": [...]} or {\"library\": [...]}.")
		return
	}
	var inputs []reads.Input
	if len(body.Inputs) > 0 {
		if len(body.Paths) > 0 || len(body.Library) > 0 {
			writeError(w, 400, "Use inputs, paths or library separately.")
			return
		}
		for _, in := range body.Inputs {
			abs, err := absPath(in.Path)
			if err != nil || strings.TrimSpace(in.Path) == "" {
				writeError(w, 400, "Invalid input path.")
				return
			}
			in.Path = abs
			inputs = append(inputs, in)
		}
	}
	for _, f := range body.Library {
		path, ok := a.store.filePath(f.SHA256)
		if !ok {
			writeError(w, http.StatusBadRequest, "Invalid file hash.")
			return
		}
		if !isFile(path) {
			writeError(w, http.StatusNotFound, f.Name+" is not in the library.")
			return
		}
		inputs = append(inputs, reads.Input{Path: path, Name: f.Name, Mapping: f.Mapping})
	}
	paths := make([]string, 0, len(body.Paths))
	for _, p := range body.Paths {
		abs, err := absPath(p)
		if err != nil || strings.TrimSpace(p) == "" {
			writeError(w, http.StatusBadRequest, "Invalid path: "+p)
			return
		}
		paths = append(paths, abs)
	}
	if len(body.Mappings) > 0 {
		// Files named with mappings: each read through its own when it has one (the paths still name
		// the dataset).
		for i, p := range body.Paths {
			in := reads.Input{Path: paths[i]}
			if m, ok := body.Mappings[p]; ok {
				in.Mapping = &m
			}
			inputs = append(inputs, in)
		}
	}
	if len(paths) == 0 && len(inputs) == 0 {
		writeError(w, http.StatusBadRequest, "Name the files to open.")
		return
	}
	if len(paths) == 1 {
		// A folder that is not a Cell Ranger output: the tables it holds, as one dataset.
		if info, err := os.Stat(paths[0]); err == nil && info.IsDir() && dirKind(paths[0]) == "" {
			items, problem := walkFolder(paths[0])
			paths = paths[:0]
			for _, item := range items {
				if item.Kind == "table" || item.Kind == "metadata" || item.Kind == "cellranger" || item.Kind == "imgt" {
					paths = append(paths, item.path)
				}
			}
			if len(paths) == 0 {
				msg := "No tables ClonoLoom reads in the folder."
				if problem != "" {
					msg += " " + problem
				}
				writeError(w, http.StatusUnprocessableEntity, msg)
				return
			}
		}
	}
	ds, err := a.openDataset(r.Context(), paths, inputs, body.Name)
	if err != nil {
		var companion reads.ErrCompanion
		switch {
		case errors.As(err, &companion), errors.Is(err, reads.ErrUnknownFormat):
			writeErrorCode(w, http.StatusUnprocessableEntity, "format", err.Error())
		case errors.Is(err, fs.ErrNotExist):
			writeErrorCode(w, http.StatusNotFound, "missing", err.Error())
		case errors.Is(err, context.Canceled):
			writeError(w, http.StatusServiceUnavailable, "Opening was cancelled.")
		default:
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		}
		return
	}
	writeJSON(w, ds.summary())
}

func (a *app) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	a.datasets.mu.Lock()
	list := make([]map[string]any, 0, len(a.datasets.byID))
	for _, ds := range a.datasets.byID {
		list = append(list, map[string]any{"id": ds.ID, "name": ds.Name, "rows": ds.Result.Table.Len(), "bytes": ds.Result.Table.Size()})
	}
	a.datasets.mu.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i]["id"].(string) < list[j]["id"].(string) })
	writeJSON(w, map[string]any{"datasets": list})
}

func (a *app) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	ds, ok := a.datasets.get(r.PathValue("id"))
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not-open", "The dataset is not open: open it again from its files.")
		return
	}
	writeJSON(w, ds.summary())
}

func (a *app) handleCloseDataset(w http.ResponseWriter, r *http.Request) {
	if !a.datasets.remove(r.PathValue("id")) {
		writeError(w, http.StatusNotFound, "The dataset is not open.")
		return
	}
	a.clonotypes.forget(r.PathValue("id"))
	writeJSON(w, map[string]any{"ok": true})
}

// Limits of a search for a file that moved: folders deep and entries looked at.
const (
	locateDepth   = 4
	locateEntries = 50000
)

// handleLocate finds a file that moved, by its fingerprint: {"name", "size", "sha256", "near":
// [folders]}. The folders named (and those under them, a few levels down) are searched for a file
// of that size, its name first, and each candidate's SHA-256 checked. Only this computer may ask.
func (a *app) handleLocate(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "Only this computer may look for files.")
		return
	}
	var body struct {
		Name   string   `json:"name"`
		Size   int64    `json:"size"`
		SHA256 string   `json:"sha256"`
		Near   []string `json:"near"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || body.SHA256 == "" || body.Size <= 0 {
		writeError(w, http.StatusBadRequest, "Send {\"name\", \"size\", \"sha256\", \"near\": [...]}.")
		return
	}
	path, err := locate(r.Context(), body.Name, body.Size, body.SHA256, body.Near)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": path})
}

func locate(ctx context.Context, name string, size int64, sha string, near []string) (string, error) {
	var named, sized []string
	seen := 0
	for _, root := range near {
		root, err := absPath(root)
		if err != nil {
			continue
		}
		base := strings.Count(root, string(filepath.Separator))
		filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || ctx.Err() != nil {
				return nil
			}
			seen++
			if seen > locateEntries {
				return filepath.SkipAll
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") && path != root || strings.Count(path, string(filepath.Separator))-base >= locateDepth {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil || info.Size() != size {
				return nil
			}
			if entry.Name() == filepath.Base(name) {
				named = append(named, path)
			} else {
				sized = append(sized, path)
			}
			return nil
		})
	}
	for _, path := range append(named, sized...) {
		if fileSHA256(path) == sha {
			return path, nil
		}
	}
	return "", fmt.Errorf("no file with %s's contents in the folders searched", filepath.Base(name))
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

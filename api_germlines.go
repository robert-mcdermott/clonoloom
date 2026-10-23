package main

// Germline artifacts derive from immutable family results and checked reference bytes.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
	"clonoloom/internal/examples"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
)

const humanGermlines = "ogrdb-human-2026-05-27"
const simulatedGermlines = "simulated-bcell-immunization"

var germlineAnalysis = analysis{kind: "germlines", prefix: "grm-", version: bcell.GermlineVersion, title: "D-masked B-cell germlines", over: "germlines", work: (*app).germlineWork, prepare: (*app).germlinePrepare}

type germlineOptions struct {
	Families     string      `json:"families"`
	FamilySHA256 string      `json:"familySha256"`
	Reference    string      `json:"reference"`
	Files        []loopsFile `json:"files"`
}
type germlineDocument struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Dataset     string          `json:"dataset"`
	DatasetName string          `json:"datasetName"`
	Samples     []string        `json:"samples"`
	Options     germlineOptions `json:"germlinesOptions"`
	bcell.GermlineResult
}

func (a *app) germlineReference(name string) (bcell.GermlineReference, []loopsFile, error) {
	library := a.references
	if name == simulatedGermlines {
		library = a.examples
	} else if name != humanGermlines {
		return nil, nil, errors.New("choose the catalogued human OGRDB reference or artificial immunization reference")
	}
	var state examples.State
	if library != nil {
		for _, s := range library.List() {
			if s.Name == name {
				state = s
				break
			}
		}
	}
	if state.State != "ready" {
		return nil, nil, errors.New("germline reference is not ready: download the human set in Reference data, or generate the artificial immunization example")
	}
	files := state.Files
	if name == simulatedGermlines {
		files = []examples.File{{Path: "reference/germlines.fasta"}}
	}
	ref := bcell.GermlineReference{}
	records := []loopsFile{}
	for _, f := range files {
		handle, err := os.Open(filepath.Join(state.Path, filepath.FromSlash(f.Path)))
		if err != nil {
			return nil, nil, errors.New("germline reference file is missing; regenerate a v2 artificial example or download the reference again")
		}
		raw, err := io.ReadAll(io.LimitReader(handle, (10<<20)+1))
		handle.Close()
		if err != nil {
			return nil, nil, err
		}
		if len(raw) > 10<<20 {
			return nil, nil, errors.New("germline reference exceeds 10 MiB")
		}
		sum := sha256.Sum256(raw)
		hash := hex.EncodeToString(sum[:])
		if name != simulatedGermlines && hash != f.SHA256 {
			return nil, nil, errors.New("germline reference checksum changed; download it again")
		}
		part, err := bcell.ReadGermlineFASTA(bytes.NewReader(raw))
		if err != nil {
			return nil, nil, err
		}
		for allele, seq := range part {
			if _, ok := ref[allele]; ok {
				return nil, nil, errors.New("duplicate allele across germline reference files")
			}
			ref[allele] = seq
		}
		records = append(records, loopsFile{Reference: name, File: f.Path, SHA256: hash, Source: state.Source.Source, License: state.License, Citation: state.Citation})
	}
	if len(ref) == 0 {
		return nil, nil, errors.New("empty germline reference set")
	}
	return ref, records, nil
}

func (a *app) germlineParent(id, dataset string) (familyDocument, string, error) {
	var doc familyDocument
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != "families" || m.State != artifacts.Complete || m.Dataset != dataset {
		return doc, "", errors.New("choose a completed families result from this dataset")
	}
	path, err := a.artifacts.Path(id, "families.json")
	if err != nil {
		return doc, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return doc, "", err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	expected := ""
	for _, f := range m.Files {
		if f.Path == "families.json" {
			expected = f.SHA256
		}
	}
	if hash != expected {
		return doc, "", errors.New("saved family result checksum changed")
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		return doc, "", err
	}
	if doc.Format != "clonoloom-families" || doc.Version != bcell.FamilyVersion || doc.Dataset != dataset || doc.Options.Threshold == nil {
		return doc, "", errors.New("germlines need a supported, thresholded families result")
	}
	return doc, hash, nil
}
func (a *app) germlinePrepare(s *analysisSpec) error {
	o := s.Germlines
	if o.Reference == "" {
		o.Reference = humanGermlines
	}
	_, hash, err := a.germlineParent(o.Families, s.Dataset)
	if err != nil {
		return err
	}
	_, files, err := a.germlineReference(o.Reference)
	if err != nil {
		return err
	}
	o.FamilySHA256, o.Files = hash, files
	// The parent determines the filtered membership; current UI choices are irrelevant.
	s.Filter = qc.Filter{}
	return nil
}
func (a *app) germlineWork(k analysis, s analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, ok := a.datasets.get(s.Dataset)
		if !ok {
			return errors.New("open the dataset again from its files")
		}
		o := *s.Germlines
		parent, hash, err := a.germlineParent(o.Families, s.Dataset)
		if err != nil {
			return err
		}
		ref, files, err := a.germlineReference(o.Reference)
		if err != nil {
			return err
		}
		if hash != o.FamilySHA256 || !bytes.Equal(mustJSON(files), mustJSON(o.Files)) {
			return errors.New("germline inputs changed; request a new analysis")
		}
		sources := make([]bcell.FamilySource, len(ds.Result.Sources))
		for i, f := range ds.Result.Sources {
			sources[i] = bcell.FamilySource{Name: f.Name, SHA256: f.SHA256}
		}
		run.Progress(.2, "Reconstructing family V/J germlines and masking D/N/P junctions")
		result, err := bcell.Germlines(ctx, ds.Result.Table, sources, parent.FamilyResult, ref)
		if err != nil {
			return err
		}
		doc := germlineDocument{Format: "clonoloom-germlines", Version: k.version, Dataset: ds.ID, DatasetName: ds.Name, Samples: parent.Samples, Options: o, GermlineResult: result}
		return a.keepAnalysis(k, s, ds, run, doc, len(doc.Samples), artifacts.Method{Name: "Family allele intersection; ungapped-reference AIRR CIGAR projection; D/N/P masking", Citation: "Gupta et al. 2015, doi:10.1093/bioinformatics/btv359; AIRR Rearrangement schema 2.0, https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html; Vander Heiden et al. 2018, doi:10.3389/fimmu.2018.02206. Allele intersection is a conservative ClonoLoom policy. Reference citations are recorded with each file."}, map[string]any{"reconstructed": result.Reconstructed, "rows": len(result.Rows), "familyLoci": len(result.Families), "reference": o.Reference})
	}
}

func (a *app) handleGermlinePage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != "germlines" || m.State != artifacts.Complete {
		writeError(w, 404, "No such germline result.")
		return
	}
	offset, limit := 0, 50
	for _, p := range []struct {
		name string
		dst  *int
	}{{"offset", &offset}, {"limit", &limit}} {
		if v := r.URL.Query().Get(p.name); v != "" {
			n, e := strconv.Atoi(v)
			if e != nil {
				writeError(w, 400, "Use an integer offset and limit.")
				return
			}
			*p.dst = n
		}
	}
	if offset < 0 || limit < 1 || limit > 200 {
		writeError(w, 400, "Use a nonnegative offset and a limit of 1–200.")
		return
	}
	path, err := a.artifacts.Path(id, "germlines.json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	var doc germlineDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	rows, families := doc.Rows, doc.Families
	doc.Rows, doc.Families = nil, nil
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "families"
	}
	var selected any
	count := 0
	switch kind {
	case "families":
		count = len(families)
		start := min(offset, count)
		selected = families[start:min(count, start+limit)]
	case "rows":
		page := []bcell.GermlineRow{}
		family := r.URL.Query().Get("family")
		locus := r.URL.Query().Get("locus")
		for _, row := range rows {
			if (family != "" && row.Family != family) || (locus != "" && row.Locus != locus) {
				continue
			}
			if count >= offset && len(page) < limit {
				page = append(page, row)
			}
			count++
		}
		selected = page
	default:
		writeError(w, 400, "Choose families or rows.")
		return
	}
	writeJSON(w, map[string]any{"artifact": m, "result": doc, "rows": selected, "count": count, "offset": offset, "limit": limit, "rowCount": len(rows), "familyCount": len(families), "kind": kind})
}

package main

// SHM artifacts pin an immutable germline parent, including its reference provenance.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
)

var shmAnalysis = analysis{kind: "shm", prefix: "shm-", version: bcell.SHMVersion, title: "Somatic hypermutation", over: "shm", work: (*app).shmWork, prepare: (*app).shmPrepare}

type shmOptions struct {
	Germlines      string `json:"germlines"`
	GermlineSHA256 string `json:"germlineSha256"`
}
type shmDocument struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Dataset     string          `json:"dataset"`
	DatasetName string          `json:"datasetName"`
	Samples     []string        `json:"samples"`
	Options     shmOptions      `json:"shmOptions"`
	Germlines   germlineOptions `json:"germlineProvenance"`
	bcell.SHMResult
}

func (a *app) checkedGermlineParent(id, dataset string) (germlineDocument, string, error) {
	var doc germlineDocument
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != "germlines" || m.State != artifacts.Complete || m.Dataset != dataset {
		return doc, "", errors.New("choose a completed germlines result from this dataset")
	}
	path, err := a.artifacts.Path(id, "germlines.json")
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
		if f.Path == "germlines.json" {
			expected = f.SHA256
		}
	}
	if hash != expected {
		return doc, "", errors.New("saved germline result checksum changed")
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		return doc, "", err
	}
	if doc.Format != "clonoloom-germlines" || doc.Version != bcell.GermlineVersion || doc.Dataset != dataset {
		return doc, "", errors.New("a supported germlines result is required")
	}
	return doc, hash, nil
}
func (a *app) shmPrepare(s *analysisSpec) error {
	_, hash, err := a.checkedGermlineParent(s.SHM.Germlines, s.Dataset)
	if err != nil {
		return err
	}
	s.SHM.GermlineSHA256 = hash
	s.Filter = qc.Filter{}
	return nil
}
func (a *app) shmWork(k analysis, s analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, ok := a.datasets.get(s.Dataset)
		if !ok {
			return errors.New("open the dataset again from its files")
		}
		parent, hash, err := a.checkedGermlineParent(s.SHM.Germlines, s.Dataset)
		if err != nil {
			return err
		}
		if hash != s.SHM.GermlineSHA256 {
			return errors.New("germline input changed; request a new analysis")
		}
		sources := make([]bcell.FamilySource, len(ds.Result.Sources))
		for i, f := range ds.Result.Sources {
			sources[i] = bcell.FamilySource{Name: f.Name, SHA256: f.SHA256}
		}
		run.Progress(.2, "Counting evaluable bases and codons against saved germlines")
		result, err := bcell.SHM(ctx, ds.Result.Table, sources, parent.GermlineResult)
		if err != nil {
			return err
		}
		doc := shmDocument{Format: "clonoloom-shm", Version: k.version, Dataset: ds.ID, DatasetName: ds.Name, Samples: parent.Samples, Options: *s.SHM, Germlines: parent.Options, SHMResult: result}
		return a.keepAnalysis(k, s, ds, run, doc, len(doc.Samples), artifacts.Method{Name: "Observed somatic divergence; conservative single-hit R/S attribution", Citation: "Gupta et al. 2015, doi:10.1093/bioinformatics/btv359; AIRR Rearrangement schema 2.0, https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html; NCBI genetic code table 1, https://www.ncbi.nlm.nih.gov/Taxonomy/Utils/wprintgc.cgi#SG1. Evaluability and attribution policies are ClonoLoom's own."}, map[string]any{"evaluated": result.Evaluated, "rows": len(result.Rows)})
	}
}
func (a *app) handleSHMPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != "shm" || m.State != artifacts.Complete {
		writeError(w, 404, "No such SHM result.")
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
	path, err := a.artifacts.Path(id, "shm.json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	var doc shmDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	rows, summaries := doc.Rows, doc.Summaries
	doc.Rows, doc.Summaries = nil, nil
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "summaries"
	}
	var selected any
	count := 0
	switch kind {
	case "summaries":
		count = len(summaries)
		start := min(offset, count)
		selected = summaries[start:min(count, start+limit)]
	case "rows":
		page := []bcell.SHMRow{}
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
		writeError(w, 400, "Choose summaries or rows.")
		return
	}
	writeJSON(w, map[string]any{"artifact": m, "result": doc, "rows": selected, "count": count, "offset": offset, "limit": limit, "rowCount": len(rows), "summaryCount": len(summaries), "kind": kind})
}

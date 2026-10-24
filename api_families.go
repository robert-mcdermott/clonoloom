package main

// Clonal families use retained rearrangements, never pre-collapsed clonotypes.
// The complete immutable result is exported; the interactive view reads bounded pages.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
)

var familyAnalysis = analysis{kind: "families", prefix: "fam-", version: bcell.FamilyVersion, title: "B-cell clonal families", subjects: true, over: "families", work: (*app).familyWork, prepare: (*app).familyPrepare}

func (a *app) familyPrepare(spec *analysisSpec) error {
	ds, ok := a.datasets.get(spec.Dataset)
	if !ok {
		return errors.New("open the dataset again from its files")
	}
	o, err := spec.Families.Normalize(len(ds.Result.Table.Cell) > 0)
	if err != nil {
		return err
	}
	spec.Families = &o
	subjects := map[string]string{}
	for _, s := range ds.Result.Table.Samples {
		subjects[s.Name] = ""
		if spec.Subjects != nil {
			subjects[s.Name] = strings.TrimSpace(spec.Subjects[s.Name])
			continue
		}
		for _, r := range ds.Result.Repertoires {
			if r.RepertoireID == s.RepertoireID || r.RepertoireID == s.Name {
				subjects[s.Name] = strings.TrimSpace(r.SubjectID)
				break
			}
		}
	}
	spec.Subjects = subjects
	return nil
}

type familyDocument struct {
	Format            string            `json:"format"`
	Version           int               `json:"version"`
	Dataset           string            `json:"dataset"`
	DatasetName       string            `json:"datasetName"`
	FilterDescription string            `json:"filterDescription"`
	Filter            qc.Filter         `json:"filter"`
	Subjects          map[string]string `json:"subjects"`
	Samples           []string          `json:"samples"`
	bcell.FamilyResult
}

func (a *app) familyWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, ok := a.datasets.get(spec.Dataset)
		if !ok {
			return errors.New("open the dataset again from its files")
		}
		run.Progress(.05, "Applying the rearrangement filter")
		keep, _ := qc.Apply(ds.Result.Table, spec.Filter)
		sources := make([]bcell.FamilySource, len(ds.Result.Sources))
		for i, s := range ds.Result.Sources {
			sources[i] = bcell.FamilySource{Name: s.Name, SHA256: s.SHA256}
		}
		result, err := bcell.Families(ctx, ds.Result.Table, keep, sources, spec.Subjects, *spec.Families, func(done, total int) {
			run.Progress(.1+.8*float64(done)/float64(max(1, total)), fmt.Sprintf("Comparing partitions: %d of %d", done, total))
		})
		if err != nil {
			return err
		}
		doc := familyDocument{Format: "clonoloom-families", Version: k.version, Dataset: ds.ID, DatasetName: ds.Name, Filter: spec.Filter, FilterDescription: spec.Filter.Describe(), Subjects: spec.Subjects, Samples: sampleNames(ds), FamilyResult: result}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples), artifacts.Method{Name: "Normalized nucleotide Hamming distance; single-linkage B-cell families", Citation: "Gupta et al. 2015, doi:10.1093/bioinformatics/btv359; Gupta et al. 2017, doi:10.4049/jimmunol.1601850. Paired maximum distance and conservative eligibility are ClonoLoom policies."}, map[string]any{"families": len(result.Families), "representatives": len(result.Representatives), "eligible": result.Eligible, "observations": len(result.Observations)})
	}
}

// Summaries and the selected family's observations are paged independently. The
// immutable JSON remains the authority; no dataset needs reopening to inspect it.
func (a *app) handleFamilyPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != "families" || m.State != artifacts.Complete {
		writeError(w, 404, "No such families result.")
		return
	}
	offset, limit := 0, 50
	for _, x := range []struct {
		name string
		dst  *int
	}{{"offset", &offset}, {"limit", &limit}} {
		if v := r.URL.Query().Get(x.name); v != "" {
			n, e := strconv.Atoi(v)
			if e != nil {
				writeError(w, 400, "Use an integer offset and limit.")
				return
			}
			*x.dst = n
		}
	}
	if offset < 0 || limit < 1 || limit > 200 {
		writeError(w, 400, "Use a nonnegative offset and a limit of 1–200.")
		return
	}
	path, err := a.artifacts.Path(id, "families.json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	var doc familyDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	families, representatives, observations := doc.Families, doc.Representatives, doc.Observations
	doc.Families = nil
	doc.Representatives = nil
	doc.Observations = nil
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "families"
	}
	family := r.URL.Query().Get("family")
	count := 0
	var rows any
	switch kind {
	case "families":
		count = len(families)
		start := min(offset, count)
		rows = families[start:min(count, start+limit)]
	case "nearest":
		count = len(representatives)
		start := min(offset, count)
		rows = representatives[start:min(count, start+limit)]
	case "members":
		selected := []bcell.FamilyObservation{}
		for _, o := range observations {
			if family != "" && o.Family != family {
				continue
			}
			if count >= offset && len(selected) < limit {
				selected = append(selected, o)
			}
			count++
		}
		rows = selected
	default:
		writeError(w, 400, "Choose families, nearest or members.")
		return
	}
	writeJSON(w, map[string]any{"artifact": m, "result": doc, "rows": rows, "count": count, "offset": offset, "limit": limit, "kind": kind, "familyCount": len(families), "representativeCount": len(representatives), "observationCount": len(observations)})
}

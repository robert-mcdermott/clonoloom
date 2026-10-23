package main

// Isotype artifacts pin the family parent and sample annotations.
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
	"clonoloom/internal/study"
)

var isotypeAnalysis = analysis{kind: "isotypes", prefix: "iso-", version: bcell.IsotypeVersion, title: "B-cell isotypes", over: "isotypes", work: (*app).isotypeWork, prepare: (*app).isotypePrepare}

type isotypeOptions struct {
	Families     string `json:"families"`
	FamilySHA256 string `json:"familySha256"`
}
type isotypeDocument struct {
	Format        string              `json:"format"`
	Version       int                 `json:"version"`
	Dataset       string              `json:"dataset"`
	DatasetName   string              `json:"datasetName"`
	Samples       []string            `json:"samples"`
	Options       isotypeOptions      `json:"isotypeOptions"`
	FamilyOptions bcell.FamilyOptions `json:"familiesOptions"`
	FamilyFilter  string              `json:"familyFilter"`
	Annotations   []study.Sample      `json:"annotations"`
	bcell.IsotypeResult
}

func (a *app) isotypePrepare(s *analysisSpec) error {
	_, hash, err := a.checkedFamilyParent(s.Isotypes.Families, s.Dataset)
	if err != nil {
		return err
	}
	s.Isotypes.FamilySHA256 = hash
	s.Filter = qc.Filter{}
	return a.prepareObservationAnnotations(s)
}
func (a *app) isotypeWork(k analysis, s analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, ok := a.datasets.get(s.Dataset)
		if !ok {
			return errors.New("open the dataset again from its files")
		}
		parent, hash, err := a.checkedFamilyParent(s.Isotypes.Families, s.Dataset)
		if err != nil {
			return err
		}
		if hash != s.Isotypes.FamilySHA256 {
			return errors.New("family input changed; request a new analysis")
		}
		sources := make([]bcell.FamilySource, len(ds.Result.Sources))
		for i, f := range ds.Result.Sources {
			sources[i] = bcell.FamilySource{Name: f.Name, SHA256: f.SHA256}
		}
		run.Progress(.2, "Counting observed heavy-chain isotypes and family co-occurrence")
		result, err := bcell.Isotypes(ctx, ds.Result.Table, sources, parent.FamilyResult, s.Study)
		if err != nil {
			return err
		}
		doc := isotypeDocument{Format: "clonoloom-isotypes", Version: k.version, Dataset: ds.ID, DatasetName: ds.Name, Samples: sampleNames(ds), Options: *s.Isotypes, FamilyOptions: parent.Options, FamilyFilter: parent.FilterDescription, Annotations: s.Study, IsotypeResult: result}
		return a.keepAnalysis(k, s, ds, run, doc, len(doc.Samples), artifacts.Method{Name: "Observed constant-region usage and unordered family co-occurrence", Citation: "AIRR Rearrangement 2.0 c_call; Lefranc 2014, doi:10.3389/fimmu.2014.00022; Horns et al. 2016, doi:10.7554/eLife.16578; Hoehn et al. 2022, doi:10.1371/journal.pcbi.1009885. Counting and ambiguity policies are ClonoLoom choices; no switching inference."}, map[string]any{"assigned": result.Assigned, "resolvedClass": result.ResolvedClass, "resolvedGene": result.ResolvedGene, "families": len(result.Families)})
	}
}
func (a *app) handleIsotypePage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != "isotypes" || m.State != artifacts.Complete {
		writeError(w, 404, "No such isotype result.")
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
	path, err := a.artifacts.Path(id, "isotypes.json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	var doc isotypeDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	observations, usage, families, pairs := doc.Observations, doc.Usage, doc.Families, doc.Pairs
	doc.Observations, doc.Usage, doc.Families, doc.Pairs = nil, nil, nil, nil
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "usage"
	}
	selected := []any{}
	count := 0
	add := func(v any) {
		if count >= offset && len(selected) < limit {
			selected = append(selected, v)
		}
		count++
	}
	family, sample, level := r.URL.Query().Get("family"), r.URL.Query().Get("sample"), r.URL.Query().Get("level")
	if level != "" && level != "class" && level != "gene" {
		writeError(w, 400, "Choose class or gene resolution.")
		return
	}
	switch kind {
	case "usage":
		for _, u := range usage {
			if u.Family == family && (sample == "" || u.Sample == sample) && (level == "" || u.Level == level) {
				add(u)
			}
		}
	case "families":
		left, right := r.URL.Query().Get("left"), r.URL.Query().Get("right")
		for _, f := range families {
			if family != "" && f.ID != family {
				continue
			}
			contains := func(v string) bool {
				if v == "" {
					return true
				}
				for _, g := range f.Genes {
					if g == v {
						return true
					}
				}
				return false
			}
			if contains(left) && contains(right) {
				add(f)
			}
		}
	case "pairs":
		for _, p := range pairs {
			add(p)
		}
	case "observations":
		for _, o := range observations {
			if (family == "" || o.Family == family) && (sample == "" || o.Sample == sample) {
				add(o)
			}
		}
	default:
		writeError(w, 400, "Choose usage, families, pairs or observations.")
		return
	}
	writeJSON(w, map[string]any{"artifact": m, "result": doc, "rows": selected, "count": count, "offset": offset, "limit": limit, "observationCount": len(observations), "familyCount": len(families), "kind": kind})
}

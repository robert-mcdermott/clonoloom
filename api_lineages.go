package main

// Lineage artifacts pin an immutable germline parent, including its reference provenance.
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
	"clonoloom/internal/study"
)

var lineageAnalysis = analysis{kind: "lineages", prefix: "lin-", version: bcell.LineageVersion, title: "B-cell lineage trees", over: "lineages", work: (*app).lineageWork, prepare: (*app).lineagePrepare}

type lineageOptions struct {
	Germlines      string `json:"germlines"`
	GermlineSHA256 string `json:"germlineSha256"`
	ExactTaxa      int    `json:"exactTaxa"`
	MaxTaxa        int    `json:"maxTaxa"`
	SearchBudget   int    `json:"searchBudget"`
	WorkBudget     int    `json:"workBudget"`
}
type lineageDocument struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Dataset     string          `json:"dataset"`
	DatasetName string          `json:"datasetName"`
	Samples     []string        `json:"samples"`
	Options     lineageOptions  `json:"lineageOptions"`
	Germlines   germlineOptions `json:"germlineProvenance"`
	Annotations []study.Sample  `json:"annotations"`
	bcell.LineageResult
}

func (a *app) lineagePrepare(s *analysisSpec) error {
	_, hash, err := a.checkedGermlineParent(s.Lineages.Germlines, s.Dataset)
	if err != nil {
		return err
	}
	s.Lineages.GermlineSHA256 = hash
	s.Lineages.ExactTaxa, s.Lineages.MaxTaxa, s.Lineages.SearchBudget, s.Lineages.WorkBudget = bcell.LineageExactTaxa, bcell.LineageMaxTaxa, bcell.LineageSearchBudget, bcell.LineageWorkBudget
	s.Filter = qc.Filter{}
	ds, ok := a.datasets.get(s.Dataset)
	if !ok {
		return errors.New("open the dataset again from its files")
	}
	supplied := s.Study != nil
	by := map[string]study.Sample{}
	for _, a := range s.Study {
		if _, ok := by[a.Name]; ok {
			return errors.New("duplicate sample annotation")
		}
		by[a.Name] = a
	}
	normalized := []study.Sample{}
	for _, sample := range ds.Result.Table.Samples {
		a := study.Sample{Name: sample.Name}
		if supplied {
			a.Time, a.Tissue = by[sample.Name].Time, by[sample.Name].Tissue
			delete(by, sample.Name)
		} else {
			for _, r := range ds.Result.Repertoires {
				if r.RepertoireID == sample.RepertoireID || r.RepertoireID == sample.Name {
					a.Time, a.Tissue = r.TimePoint, r.Tissue
					break
				}
			}
		}
		a.Time, a.Tissue = strings.TrimSpace(a.Time), strings.TrimSpace(a.Tissue)
		normalized = append(normalized, a)
	}
	if len(by) > 0 {
		return errors.New("sample annotation names an unknown sample")
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Name < normalized[j].Name })
	s.Study = normalized
	return nil
}
func (a *app) lineageWork(k analysis, s analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, ok := a.datasets.get(s.Dataset)
		if !ok {
			return errors.New("open the dataset again from its files")
		}
		parent, hash, err := a.checkedGermlineParent(s.Lineages.Germlines, s.Dataset)
		if err != nil {
			return err
		}
		if hash != s.Lineages.GermlineSHA256 {
			return errors.New("germline input changed; request a new analysis")
		}
		sources := make([]bcell.FamilySource, len(ds.Result.Sources))
		for i, f := range ds.Result.Sources {
			sources[i] = bcell.FamilySource{Name: f.Name, SHA256: f.SHA256}
		}
		run.Progress(.2, "Searching family/locus parsimony trees on shared V/J panels")
		result, err := bcell.Lineages(ctx, ds.Result.Table, sources, parent.GermlineResult, s.Study)
		if err != nil {
			return err
		}
		doc := lineageDocument{Format: "clonoloom-lineages", Version: k.version, Dataset: ds.ID, DatasetName: ds.Name, Samples: parent.Samples, Options: *s.Lineages, Germlines: parent.Options, LineageResult: result, Annotations: s.Study}
		return a.keepAnalysis(k, s, ds, run, doc, len(doc.Samples), artifacts.Method{Name: "Equal-cost nucleotide parsimony; exhaustive or deterministic addition/NNI; conditional ancestral states", Citation: "Fitch 1971, doi:10.1093/sysbio/20.4.406; Sankoff 1975, doi:10.1137/0128004; Hoehn et al. 2022, doi:10.1371/journal.pcbi.1009885. Panel, search limits and tie policies are ClonoLoom choices; no migration or switching statistics are inferred."}, map[string]any{"built": result.Built, "trees": len(result.Trees), "members": len(result.Members)})
	}
}
func (a *app) handleLineagePage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != "lineages" || m.State != artifacts.Complete {
		writeError(w, 404, "No such lineage result.")
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
	path, err := a.artifacts.Path(id, "lineages.json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	var doc lineageDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	rows, trees := doc.Members, doc.Trees
	doc.Members, doc.Trees = nil, nil
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "trees"
	}
	var selected any
	count := 0
	switch kind {
	case "trees":
		count = len(trees)
		start := min(offset, count)
		page := []bcell.LineageTree{}
		for _, tree := range trees[start:min(count, start+limit)] {
			page = append(page, tree.Summary())
		}
		selected = page
	case "detail":
		for _, tree := range trees {
			if tree.ID == r.URL.Query().Get("tree") {
				writeJSON(w, map[string]any{"artifact": m, "result": doc, "tree": tree, "rowCount": len(rows), "treeCount": len(trees)})
				return
			}
		}
		writeError(w, 404, "No such tree in this result.")
		return
	case "rows":
		page := []bcell.LineageMember{}
		family := r.URL.Query().Get("family")
		locus := r.URL.Query().Get("locus")
		for _, row := range rows {
			if (r.URL.Query().Get("node") != "" && row.Node != r.URL.Query().Get("node")) || (r.URL.Query().Get("tree") != "" && row.Tree != r.URL.Query().Get("tree")) || (family != "" && row.Family != family) || (locus != "" && row.Locus != locus) {
				continue
			}
			if count >= offset && len(page) < limit {
				page = append(page, row)
			}
			count++
		}
		selected = page
	default:
		writeError(w, 400, "Choose trees, detail or rows.")
		return
	}
	writeJSON(w, map[string]any{"artifact": m, "result": doc, "rows": selected, "count": count, "offset": offset, "limit": limit, "rowCount": len(rows), "treeCount": len(trees), "kind": kind})
}

package main

// A submitted comparison pins expression/phenotype parents and a receptor snapshot.
// Jobs reuse these inputs; the source dataset must be open to prepare the snapshot.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/conga"
	"clonoloom/internal/jobs"
	"clonoloom/internal/tcrdist"
)

const congaKind = "conga"
const congaCitation = "Schattgen et al. 2022, Nature Biotechnology 40:54–63, doi:10.1038/s41587-021-00989-2 (CoNGA); Dash et al. 2017, Nature 547:89–93, doi:10.1038/nature22383; Mayer-Blackwell et al. 2021, doi:10.7554/eLife.68605 (TCRdist3 conventions). Dunn 1961, doi:10.1080/01621459.1961.10482090 (Bonferroni). Mean-PC profiles, shared-CDR3 exclusions, inclusive ties and all-scale Bonferroni are declared ClonoLoom policies; see docs/CONGA.md."

type congaRequest struct {
	Phenotypes string        `json:"phenotypes"`
	Options    conga.Options `json:"options"`
}
type congaSpec struct {
	congaRequest
	PhenotypesSHA256 string                `json:"phenotypesSha256"`
	Map              string                `json:"map"`
	MapSHA256        string                `json:"mapSha256"`
	Method           int                   `json:"method"`
	Receptors        []clonotype.Clonotype `json:"receptors"`
	LoopsSHA256      string                `json:"loopsSha256,omitempty"`
}

func (s congaSpec) artifactID() string {
	sum := sha256.Sum256(mustJSON(s))
	return "conga-" + hex.EncodeToString(sum[:])[:24]
}

type congaDocument struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	Description string `json:"description"`
	// Full parent assignment/provenance survives standalone export, including cells absent from
	// the graph. The graph itself remains in the separately fingerprinted expression-map parent.
	Phenotypes         phenotypeDocument `json:"phenotypes"`
	Map                string            `json:"map"`
	ExpressionExcluded int               `json:"expressionExcluded"`
	Loops              *loopsFile        `json:"loops,omitempty"`
	Result             *conga.Result     `json:"analysis"`
}

func (a *app) registerConga(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/conga", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			congaRequest
			Start *bool `json:"start"`
		}
		d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		d.DisallowUnknownFields()
		if err := d.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		s := congaSpec{congaRequest: body.congaRequest, Method: conga.Version}
		var err error
		if s.Options, err = s.Options.Normalize(); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if s.Phenotypes == "" {
			writeError(w, 400, "Choose a saved phenotype result with a computed expression map.")
			return
		}
		p, m, err := a.modularityParents(modularitySpec{modularityRequest: modularityRequest{Phenotypes: s.Phenotypes}, PhenotypesSHA256: s.PhenotypesSHA256, Map: s.Map, MapSHA256: s.MapSHA256})
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		// Pin the bytes verified in this request; the job verifies them again before computing.

		_, s.PhenotypesSHA256, err = a.readCheckedResult(s.Phenotypes, phenotypeKind, &p)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		s.Map = p.Map
		_, s.MapSHA256, err = a.readCheckedResult(s.Map, expressionMapKind, &m)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		if err = conga.CheckDefinition(p.Result.Definition); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		ds, entry, code, err := a.clonotypeTable(clonotypeRequest{Dataset: p.Dataset, Definition: p.Result.Definition, Filter: p.Result.Filter})
		if err != nil {
			writeError(w, code, err.Error())
			return
		}
		if string(mustJSON(ds.Result.Sources)) != string(mustJSON(p.ReceptorSources)) {
			writeError(w, 409, "Receptor snapshot differs from phenotype sources; reopen the original dataset.")
			return
		}
		// Capture only the assigned retained population, with no other sample's abundances.
		wanted := map[string]bool{}
		retained := map[string]bool{}
		for _, c := range m.Result.Cells {
			retained[c.Barcode] = true
		}
		for _, c := range p.Result.Cells {
			if retained[c.Barcode] && (c.Status == "Assigned" || c.Status == "Outside size population") {
				wanted[c.Clone] = true
			}
		}
		if len(wanted) > conga.MaxClones {
			writeError(w, 400, "At most 2000 assigned clonotypes can be compared.")
			return
		}
		for _, c := range entry.table.Clonotypes {
			if wanted[c.ID] {
				s.Receptors = append(s.Receptors, clonotype.Clonotype{ID: c.ID, Chains: c.Chains})
			}
		}
		if !s.Options.CDR3Only {
			_, file, e := a.tcrdistLoops()
			if e != nil {
				writeErrorCode(w, 409, prepareCode(e), e.Error())
				return
			}
			s.LoopsSHA256 = file.SHA256
		}

		id := s.artifactID()
		if saved, err := a.artifacts.Get(id); err == nil && saved.Kind == congaKind && saved.State == artifacts.Complete {
			writeJSON(w, map[string]any{"artifact": id, "ready": true})
			return
		}
		if body.Start != nil && !*body.Start {
			out := map[string]any{"artifact": id, "ready": false}
			for _, j := range a.jobs.List(0) {
				if j.Key == id && !j.State.Finished() {
					out["job"] = j.ID
				}
			}
			writeJSON(w, out)
			return
		}
		j, _, err := a.jobs.Submit(congaKind, "Receptor / expression neighborhoods · "+p.Result.Sample, id, s, a.congaWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": j.ID})
	})
	mux.HandleFunc("GET /api/conga/{id}", func(w http.ResponseWriter, r *http.Request) {
		if m, err := a.artifacts.Get(r.PathValue("id")); err != nil || m.Kind != congaKind || m.State != artifacts.Complete {
			writeError(w, 404, "No complete conga result with that ID.")
			return
		}
		var doc congaDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), congaKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
}
func (a *app) congaWork(s congaSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Method != conga.Version {
			return errors.New("conga method version changed; start a new result")
		}
		run.Progress(0, "Verifying phenotype and expression-graph parents")
		p, m, err := a.modularityParents(modularitySpec{modularityRequest: modularityRequest{Phenotypes: s.Phenotypes}, PhenotypesSHA256: s.PhenotypesSHA256, Map: s.Map, MapSHA256: s.MapSHA256})
		if err != nil {
			return err
		}
		var loops *tcrdist.Loops
		var loopRecord *loopsFile
		if !s.Options.CDR3Only {
			l, f, e := a.tcrdistLoops()
			if e != nil {
				return e
			}
			if f.SHA256 != s.LoopsSHA256 {
				return errors.New("V-loop reference changed; start a new comparison")
			}
			loops = l
			loopRecord = &f
		}
		result, err := conga.Compute(ctx, p.Result, m.Result, s.Receptors, loops, s.Options, func(v float64) { run.Progress(.1+float64(.8*v), "Comparing clonotype neighborhoods") })
		if err != nil {
			return err
		}
		description := fmt.Sprintf("CoNGA-style comparison in sample %s: %d retained paired cells, %d clonotype mean-PC profiles, %d tested clonotype-by-k hypotheses. Exact Euclidean distances on all saved PCs; paired TCRdist (CDR3 only: %t; organism: %s). Self and shared alpha/beta CDR3s excluded from both candidate pools. Directed neighborhoods include all exact cutoff ties. Conditional uniform-independent-neighbor null; inclusive hypergeometric upper tails and Bonferroni across every tested clone and scale. Null exchangeability is an assumption, not established by the data. No conditioning on cell type, batch or receptor-chain groups. Saved PCs may include receptor genes; their expression can directly couple the two modalities. No UMAP distances, pooled donors, biological replicates, antigen-specificity or causal inference. Repeated analysis choices are not corrected. This is a declared adaptation, not the original CoNGA score or full pipeline.", p.Result.Sample, result.Eligible, len(result.Nodes), result.Tested, result.Options.CDR3Only, result.Options.Organism)
		doc := congaDocument{Format: "clonoloom-conga", Version: conga.Version, Description: description, Phenotypes: p, Map: s.Map, ExpressionExcluded: len(p.Result.Cells) - len(m.Result.Cells), Loops: loopRecord, Result: result}

		raw, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		id := s.artifactID()
		dir, err := a.artifacts.Stage(id)
		if err != nil {
			return err
		}
		defer a.artifacts.Discard(id)
		file := congaKind + ".json"
		if err = os.WriteFile(filepath.Join(dir, file), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		manifest := artifacts.Manifest{ID: id, Kind: congaKind, Name: "Receptor / expression neighborhoods · " + p.Result.Sample, Dataset: p.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: phenotypeKind, ID: s.Phenotypes, SHA256: s.PhenotypesSHA256}, {Kind: expressionMapKind, ID: s.Map, SHA256: s.MapSHA256}}, Job: &artifacts.Job{ID: run.ID(), Kind: congaKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.congaRequest), Method: artifacts.Method{Name: "CoNGA-style clonotype neighborhood overlap", Version: fmt.Sprint(conga.Version), Citation: congaCitation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: file, Role: congaKind}}, Summary: mustJSON(map[string]any{"eligible": result.Eligible, "tested": result.Tested, "clonotypes": len(result.Nodes)})}
		saved, err := a.artifacts.Promote(manifest, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			raw, err := os.ReadFile(filepath.Join(dir, file))
			var back congaDocument
			if err == nil {
				err = json.Unmarshal(raw, &back)
			}
			if err != nil || back.Result == nil || back.Result.Tested != result.Tested || back.Map != s.Map {
				return []artifacts.Finding{{Severity: "fatal", Code: "conga.unreadable", Message: "The conga result could not be read back."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("conga result failed validation")
		}
		run.Log("info", "Saved %d tested clone-by-k hypotheses in %s.", result.Tested, id)
		return nil
	}
}
func (a *app) handleCongaJobs() {
	a.jobs.Handle(congaKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s congaSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.Phenotypes == "" || s.PhenotypesSHA256 == "" || s.Map == "" || s.MapSHA256 == "" || (s.Method != conga.Version || len(s.Receptors) == 0 || !s.Options.CDR3Only && s.LoopsSHA256 == "") {
			return nil, errors.New("comparison parent or method is unavailable")
		}
		if _, err := s.Options.Normalize(); err != nil {
			return nil, err
		}
		return a.congaWork(s), nil
	}, nil)
}

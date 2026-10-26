package main

// Source-pinned, descriptive cross-subject nucleotide sharing; research §5.37.
import (
	"archive/zip"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/contamination"
	"clonoloom/internal/diversity"
	"clonoloom/internal/export"
	"clonoloom/internal/genes"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
	"clonoloom/internal/seq"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const contaminationKind = "contamination"

func contaminationDefinition() clonotype.Definition {
	d, _ := (clonotype.Definition{Chains: []string{"TRB"}, Junction: "nt", V: "gene", J: "gene", ProductiveOnly: true}).Normalize()
	return d
}

type contaminationRequest struct {
	Dataset   string                 `json:"dataset"`
	Species   string                 `json:"species"`
	Unrelated bool                   `json:"unrelated"`
	Samples   []contamination.Sample `json:"samples"`
	Filter    qc.Filter              `json:"filter"`
	Options   contamination.Options  `json:"options"`
	Pgen      string                 `json:"pgen"`
}
type contaminationSpec struct {
	contaminationRequest
	Input           contamination.Input  `json:"input"`
	Definition      clonotype.Definition `json:"definition"`
	Notice          export.Notice        `json:"notice"`
	PgenModel       *modelSet            `json:"pgenModel,omitempty"`
	PgenManifest    *artifacts.Manifest  `json:"pgenManifest,omitempty"`
	PgenSHA256      string               `json:"pgenSha256,omitempty"`
	PgenDescription string               `json:"pgenDescription"`
	Method          int                  `json:"method"`
}

func (s contaminationSpec) artifactID() string {
	s.Notice.Made = time.Time{}
	return "cnt-" + fmt.Sprintf("%x", sha256.Sum256(mustJSON(s)))[:24]
}

type contaminationDocument struct {
	Format      string                `json:"format"`
	Version     int                   `json:"version"`
	Description string                `json:"description"`
	Evidence    contaminationSpec     `json:"evidence"`
	Analysis    *contamination.Result `json:"analysis"`
}

func (a *app) captureContamination(req contaminationRequest) (contaminationSpec, error) {
	s := contaminationSpec{contaminationRequest: req, Definition: contaminationDefinition(), Method: contamination.Version, PgenDescription: "No Pgen parent supplied; publicness explanation is unknown."}
	if req.Species != "human" || !req.Unrelated {
		return s, errors.New("declare human, unrelated subjects; distinguish repeated samples by the same subject ID")
	}
	if req.Filter.MinCount < 0 || math.IsNaN(req.Filter.MinCount) || math.IsInf(req.Filter.MinCount, 0) || req.Filter.MaxChains < 0 {
		return s, errors.New("invalid QC thresholds")
	}
	ds, ok := a.datasets.get(req.Dataset)
	if !ok {
		return s, errors.New("open the source dataset first")
	}
	t := ds.Result.Table
	if t.Cell != nil || t.Unit == rep.Cells || t.Unit == rep.UnitUnknown || t.Unit == rep.Sequences {
		return s, errors.New("this screen requires bulk TRB with explicit reads, UMIs or template counts")
	}
	var err error
	s.Options, err = s.Options.Normalize()
	if err != nil {
		return s, err
	}
	s.Samples = append([]contamination.Sample{}, s.Samples...)
	sort.Slice(s.Samples, func(i, j int) bool { return s.Samples[i].Name < s.Samples[j].Name })
	s.Input = contamination.Input{Options: s.Options, Samples: []contamination.Depth{}, Receptors: []contamination.Receptor{}}
	indexes := map[string]int{}
	present := map[string]bool{}
	for _, sample := range t.Samples {
		present[sample.Name] = true
	}
	for i, sub := range s.Samples {
		sub.Subject = strings.TrimSpace(sub.Subject)
		sub.Batch = strings.TrimSpace(sub.Batch)
		s.Samples[i] = sub
		if _, exists := indexes[sub.Name]; exists || !present[sub.Name] {
			return s, errors.New("select distinct existing samples")
		}
		indexes[sub.Name] = i
		s.Input.Samples = append(s.Input.Samples, contamination.Depth{Sample: sub, Excluded: map[string]int{}})
	}
	if len(s.Samples) < 2 || len(s.Samples) > 64 {
		return s, errors.New("select 2–64 samples")
	}
	keep, removed := qc.Apply(t, s.Filter)
	for i, sample := range t.Samples {
		if slot, ok := indexes[sample.Name]; ok {
			for reason, n := range removed[i] {
				s.Input.Samples[slot].Excluded["QC: "+reason] = n
			}
		}
	}
	records := map[string]*contamination.Receptor{}
	for row := 0; row < t.Len(); row++ {
		slot, chosen := indexes[t.Samples[t.Sample[row]].Name]
		if !chosen || !keep[row] {
			continue
		}
		depth := &s.Input.Samples[slot]
		if t.LocusName(row) != "TRB" || t.Productive[row] != rep.True {
			depth.Excluded["not explicitly productive TRB"]++
			continue
		}
		count := t.Count[row]
		if math.IsNaN(count) || math.IsInf(count, 0) || count < 0 || math.Trunc(count) != count {
			return s, fmt.Errorf("sample %s has missing, nonfinite or noninteger retained counts", depth.Sample.Name)
		}
		depth.Total += count
		depth.Rows++
		v, j := t.Call(t.V[row]), t.Call(t.J[row])
		nt := t.Junctions.Get(t.Junction[row])
		vg, jg := v.Key(genes.LevelGene), j.Key(genes.LevelGene)
		if !v.Resolved(genes.LevelGene) || !j.Resolved(genes.LevelGene) || !contamination.Valid(vg, jg, nt) {
			depth.Excluded["unresolved V/J or missing/nonstandard nucleotide junction"]++
			continue
		}
		if count == 0 {
			continue
		}
		id := contamination.ID(vg, jg, nt)
		rec := records[id]
		if rec == nil {
			if len(records) >= 5000 {
				return s, errors.New("at most 5000 eligible nucleotide identities")
			}
			rec = &contamination.Receptor{ID: id, V: vg, J: jg, NT: nt, AA: seq.Translate(nt), Counts: make([]float64, len(s.Samples)), Pgen: diversity.Number(math.NaN())}
			records[id] = rec
		}
		rec.Counts[slot] += count
	}
	for _, rec := range records {
		s.Input.Receptors = append(s.Input.Receptors, *rec)
	}
	contamination.Sort(s.Input.Receptors)
	for _, rec := range s.Input.Receptors {
		for i, c := range rec.Counts {
			s.Input.Samples[i].Eligible += c
		}
	}
	if s.Pgen != "" {
		var p pgenDocument
		m, hash, e := a.readCheckedResult(s.Pgen, "pgen", &p)
		if e != nil {
			return s, e
		}
		if p.Format != "clonoloom-pgen" || p.Version != 1 || m.Dataset != s.Dataset || p.Dataset != s.Dataset || p.Definition.ID() != s.Definition.ID() || p.Organism != "human" || string(mustJSON(p.Filter)) != string(mustJSON(s.Filter)) || p.Set.Checksum == "" {
			return s, errors.New("Pgen parent must match this dataset, productive NT TRB V/J-gene definition, QC and human organism")
		}
		probabilities := map[string]float64{}
		for _, row := range p.Rows {
			if len(row.Chains) != 1 || row.Chains[0].Locus != "TRB" {
				continue
			}
			c := row.Chains[0]
			id := contamination.ID(c.V, c.J, c.Junction)
			if _, exists := probabilities[id]; exists {
				return s, errors.New("duplicate Pgen identity")
			}
			if row.Pgen >= 0 {
				probabilities[id] = row.Pgen
			}
		}
		for i := range s.Input.Receptors {
			rec := &s.Input.Receptors[i]
			if value, ok := probabilities[rec.ID]; ok {
				rec.Pgen = diversity.Number(value)
			}
		}
		s.PgenModel = &p.Set
		s.PgenManifest = &m
		s.PgenSHA256 = hash
		s.PgenDescription = fmt.Sprintf("Saved nucleotide Pgen %s, SHA-256 %s; %s; checksum %s; joint V/J probability: %t. %s", s.Pgen, hash, p.Set.Name, p.Set.Checksum, p.Genes, p.Set.Citation+" Source: "+p.Set.Source+"; license: "+p.Set.License)
	}
	s.Notice = a.notice(ds, string(t.Unit), s.Filter, s.Definition.Describe(), s.Definition.ID())
	s.Notice.What = contamination.Description
	s.Notice.Method += " " + contamination.Citation
	s.Notice.Derived = true
	for _, source := range ds.Result.Metadata {
		s.Notice.Files = append(s.Notice.Files, export.File{Name: source.Name, SHA256: source.SHA256})
	}
	if _, err = contamination.Compute(context.Background(), s.Input); err != nil {
		return s, err
	}
	return s, nil
}
func (a *app) registerContamination(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/contamination", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			contaminationRequest
			Start *bool `json:"start"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s, err := a.captureContamination(body.contaminationRequest)
		if err != nil {
			writeError(w, 422, err.Error())
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.Kind == contaminationKind && m.State == artifacts.Complete {
			var doc contaminationDocument
			if _, _, e = a.readCheckedResult(id, contaminationKind, &doc); e != nil {
				writeError(w, 409, e.Error())
				return
			}
			writeJSON(w, map[string]any{"artifact": id, "ready": true})
			return
		}
		if body.Start != nil && !*body.Start {
			writeJSON(w, map[string]any{"artifact": id, "ready": false})
			return
		}
		j, _, err := a.jobs.Submit(contaminationKind, "Contamination evidence · "+s.Notice.Dataset, id, s, a.contaminationWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "job": j.ID, "ready": false})
	})
	mux.HandleFunc("GET /api/contamination/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc contaminationDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), contaminationKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/contamination/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc contaminationDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), contaminationKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		n := doc.Evidence.Notice
		n.Made = m.Created
		if n.Restricted() && r.URL.Query().Get("accept") != "1" {
			restricted(w, n)
			return
		}
		a.writeZip(w, m.ID, func(z *zip.Writer) error {
			for _, f := range []struct {
				name  string
				value any
			}{{"contamination.json", doc}, {"manifest.json", m}, {"samples.json", doc.Analysis.Evidence.Samples}} {
				out, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if _, e = out.Write(mustJSON(f.value)); e != nil {
					return e
				}
			}
			out, e := z.Create("matches.csv")
			if e != nil {
				return e
			}
			if e = contamination.MatchesCSV(out, doc.Analysis); e != nil {
				return e
			}
			out, e = z.Create("pairs.csv")
			if e != nil {
				return e
			}
			if e = contamination.PairsCSV(out, doc.Analysis); e != nil {
				return e
			}
			return writeNotice(z, n)
		})
	})
}
func (s contaminationSpec) check() error {
	if s.Method != contamination.Version || s.Dataset == "" || s.Species != "human" || !s.Unrelated || s.Definition.ID() != contaminationDefinition().ID() || string(mustJSON(s.Options)) != string(mustJSON(s.Input.Options)) {
		return errors.New("inconsistent captured contamination provenance")
	}
	if len(s.Samples) != len(s.Input.Samples) {
		return errors.New("captured samples differ")
	}
	for i, x := range s.Samples {
		if x != s.Input.Samples[i].Sample {
			return errors.New("captured assignments differ")
		}
	}
	if s.Pgen == "" {
		for _, rec := range s.Input.Receptors {
			if !math.IsNaN(float64(rec.Pgen)) {
				return errors.New("Pgen values require a captured model parent")
			}
		}
	}
	if s.Pgen != "" {
		if s.PgenModel == nil || s.PgenModel.Checksum == "" || s.PgenManifest == nil || s.PgenManifest.ID != s.Pgen || s.PgenManifest.Kind != "pgen" {
			return errors.New("Pgen provenance differs")
		}
		found := false
		for _, f := range s.PgenManifest.Files {
			if f.Path == "pgen.json" && f.SHA256 == s.PgenSHA256 {
				found = true
			}
		}
		if !found {
			return errors.New("Pgen checksum differs")
		}
	}
	return nil
}
func (a *app) contaminationWork(s contaminationSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		result, err := contamination.Compute(ctx, s.Input)
		if err != nil {
			return err
		}
		run.Progress(.8, "Saving exact nucleotide sharing evidence")
		desc := fmt.Sprintf("%s Dataset %s (%s). Definition %s; QC %s; abundance unit %s. %d declared samples; %d cross-subject pairs; %d shared pair/identity records. Thresholds: min count %g in both samples, higher fraction >= %g, asymmetry ratio >= %g, high NT Pgen >= %g. %s", contamination.Description, s.Notice.Dataset, s.Dataset, s.Definition.Describe(), s.Notice.Filter, s.Notice.Unit, len(s.Samples), len(result.Pairs), len(result.Matches), s.Options.MinCount, s.Options.HighFraction, s.Options.Ratio, s.Options.HighPgen, s.PgenDescription)
		doc := contaminationDocument{Format: "clonoloom-contamination", Version: contamination.Version, Description: desc, Evidence: s, Analysis: result}
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
		if err = os.WriteFile(filepath.Join(dir, "contamination.json"), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: contaminationKind, Name: "Contamination evidence · " + s.Notice.Dataset, Dataset: s.Dataset, Lineage: artifacts.Lineage{Parents: contaminationParents(s), Job: &artifacts.Job{ID: run.ID(), Kind: contaminationKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.contaminationRequest), Method: artifacts.Method{Name: "Descriptive contamination evidence", Version: "1", Citation: contamination.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: "contamination.json", Role: contaminationKind}}, Summary: mustJSON(map[string]any{"samples": len(s.Samples), "pairs": len(result.Pairs), "matches": len(result.Matches)})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, "contamination.json"))
			var back contaminationDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Analysis == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "contamination.unreadable", Message: "Contamination result failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("contamination promotion failed")
		}
		return nil
	}
}
func (a *app) handleContaminationJobs() {
	a.jobs.Handle(contaminationKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s contaminationSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if err := s.check(); err != nil {
			return nil, err
		}
		return a.contaminationWork(s), nil
	}, nil)
}

func contaminationParents(s contaminationSpec) []artifacts.Parent {
	if s.PgenManifest == nil {
		return nil
	}
	return []artifacts.Parent{{Kind: "pgen", ID: s.Pgen, Name: s.PgenManifest.Name}}
}

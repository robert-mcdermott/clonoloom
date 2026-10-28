package main

import (
	"archive/zip"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/export"
	"clonoloom/internal/genes"
	"clonoloom/internal/jobs"
	"clonoloom/internal/motifs"
	"clonoloom/internal/qc"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const motifsKind = "motifs"

type motifsRequest struct {
	Dataset    string               `json:"dataset"`
	Definition clonotype.Definition `json:"definition"`
	Filter     qc.Filter            `json:"filter"`
	Query      []string             `json:"query"`
	Background []string             `json:"background"`
	Subjects   map[string]string    `json:"subjects"`
	Options    motifs.Options       `json:"options"`
}
type motifsSpec struct {
	motifsRequest
	QueryRecords      []motifs.Record `json:"queryRecords"`
	BackgroundRecords []motifs.Record `json:"backgroundRecords"`
	Excluded          map[string]int  `json:"excluded"`
	Samples           []string        `json:"samples"`
	Notice            export.Notice   `json:"notice"`
	Unit              string          `json:"unit"`
	Method            int             `json:"method"`
}

func (s motifsSpec) artifactID() string {
	s.Notice.Made = time.Time{}
	return "mot-" + fmt.Sprintf("%x", sha256.Sum256(mustJSON(s)))[:24]
}

type motifsDocument struct {
	Format      string         `json:"format"`
	Version     int            `json:"version"`
	Description string         `json:"description"`
	Evidence    motifsSpec     `json:"evidence"`
	Result      *motifs.Result `json:"analysis"`
}

// Replay uses captured evidence, but its arm/sample/subject provenance must still
// agree with the declared request. Source files are deliberately not reopened.
func (s motifsSpec) check() error {
	def, err := s.Definition.Normalize()
	if err != nil || def.Vendor || def.Junction != "aa" || (def.V != "gene" && def.V != "allele") || string(mustJSON(def)) != string(mustJSON(s.Definition)) {
		return errors.New("invalid captured motif definition")
	}
	o, err := s.Options.Normalize()
	if err != nil || o != s.Options || s.Method != motifs.Version || s.Dataset == "" || len(s.Samples) == 0 || len(s.Samples) > 256 {
		return errors.New("invalid captured motif method or options")
	}
	present := map[string]bool{}
	for _, sample := range s.Samples {
		if sample == "" || present[sample] {
			return errors.New("invalid captured source samples")
		}
		present[sample] = true
	}
	for sample, subject := range s.Subjects {
		if !present[sample] || len(subject) > 256 {
			return errors.New("invalid captured subject annotation")
		}
	}
	selected := map[string]bool{}
	for arm, names := range [][]string{s.Query, s.Background} {
		if len(names) == 0 {
			return errors.New("empty captured sample arm")
		}
		allowed := map[string]bool{}
		for _, name := range names {
			if !present[name] || selected[name] {
				return errors.New("invalid captured sample selections")
			}
			selected[name], allowed[name] = true, true
		}
		rows := s.QueryRecords
		if arm == 1 {
			rows = s.BackgroundRecords
		}
		for _, row := range rows {
			if len(row.Samples) == 0 || len(row.Clonotypes) == 0 {
				return errors.New("captured receptor lacks source membership")
			}
			seen, subjects := map[string]bool{}, map[string]bool{}
			for _, name := range row.Samples {
				if !allowed[name] || seen[name] {
					return errors.New("captured receptor is outside its arm")
				}
				seen[name] = true
				if subject := strings.TrimSpace(s.Subjects[name]); subject != "" {
					subjects[subject] = true
				}
			}
			if len(subjects) != len(row.Subjects) {
				return errors.New("captured subjects differ from annotations")
			}
			for _, subject := range row.Subjects {
				if !subjects[subject] {
					return errors.New("captured subjects differ from annotations")
				}
				delete(subjects, subject)
			}
		}
	}
	return nil
}

func (a *app) captureMotifs(req motifsRequest) (motifsSpec, error) {
	s := motifsSpec{motifsRequest: req, Method: motifs.Version, Excluded: map[string]int{}}
	def, err := req.Definition.Normalize()
	if err != nil {
		return s, err
	}
	if def.Vendor || def.Junction != "aa" || def.V == "none" || def.V == "family" {
		return s, errors.New("motifs need amino-acid junctions with V genes or alleles")
	}
	s.Definition = def
	s.Options, err = s.Options.Normalize()
	if err != nil {
		return s, err
	}
	ds, entry, _, err := a.clonotypeTable(clonotypeRequest{Dataset: s.Dataset, Definition: def, Filter: s.Filter})
	if err != nil {
		return s, err
	}
	ct := entry.table
	s.Unit = string(ct.Unit)
	s.Samples = sampleNames(ds)
	if len(s.Samples) > 256 {
		return s, errors.New("motif screen supports at most 256 source samples")
	}
	if ct.Excluded[clonotype.ReasonNoCount] > 0 {
		return s, errors.New("retained source rows lack abundance; confirm a count unit before selecting positive observed receptors")
	}
	present := map[string]bool{}
	for _, n := range s.Samples {
		present[n] = true
	}
	selected := map[string]int{}
	for arm, names := range [][]string{s.Query, s.Background} {
		if len(names) == 0 {
			return s, errors.New("select query and explicit background samples")
		}
		for _, name := range names {
			if !present[name] || selected[name] != 0 {
				return s, errors.New("sample selections must exist, be distinct and disjoint")
			}
			selected[name] = arm + 1
		}
		sort.Strings(names)
	}
	for name, subject := range s.Subjects {
		if !present[name] || len(subject) > 256 {
			return s, errors.New("invalid subject annotation")
		}
	}
	records := []map[string]*motifs.Record{{}, {}}
	sampleSets := []map[string]map[string]bool{{}, {}}
	subjectSets := []map[string]map[string]bool{{}, {}}
	cloneSets := []map[string]map[string]bool{{}, {}}
	for why, n := range ct.Excluded {
		s.Excluded["clonotype: "+why] = n
	}
	for i, c := range ct.Clonotypes {
		chains := []clonotype.Chain{}
		for _, ch := range c.Chains {
			if ch.Locus == "TRB" {
				chains = append(chains, ch)
			}
		}
		if len(chains) != 1 {
			s.Excluded["clonotypes without exactly one retained TRB chain"]++
			continue
		}
		ch := chains[0]
		call := genes.ParseCall(ch.V)
		v := call.Key(genes.LevelGene)
		if !call.Resolved(genes.LevelGene) || !motifs.Valid(v, ch.Junction) {
			s.Excluded["clonotypes with unresolved V or nonstandard/short/long junction"]++
			continue
		}
		id := motifs.ID(v, ch.Junction)
		samples, amounts := ct.Abundances(i)
		for j, sample := range samples {
			arm := selected[s.Samples[sample]] - 1
			if arm < 0 {
				continue
			}
			amount := amounts[j]
			if !(amount > 0) || math.IsInf(amount, 0) {
				continue
			}
			rec := records[arm][id]
			if rec == nil {
				if len(records[0])+len(records[1]) >= 4000 {
					return s, errors.New("at most 4000 total distinct TRBV/junction units")
				}
				rec = &motifs.Record{ID: id, V: v, Junction: ch.Junction}
				records[arm][id] = rec
				sampleSets[arm][id] = map[string]bool{}
				subjectSets[arm][id] = map[string]bool{}
				cloneSets[arm][id] = map[string]bool{}
			}
			rec.Abundance += amount
			sampleSets[arm][id][s.Samples[sample]] = true
			if subject := strings.TrimSpace(s.Subjects[s.Samples[sample]]); subject != "" {
				subjectSets[arm][id][subject] = true
			}
			cloneSets[arm][id][c.ID] = true
		}
	}
	sorted := func(m map[string]bool) []string {
		out := []string{}
		for v := range m {
			out = append(out, v)
		}
		sort.Strings(out)
		return out
	}
	for arm := range records {
		keys := []string{}
		for id := range records[arm] {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			r := records[arm][id]
			r.Samples = sorted(sampleSets[arm][id])
			r.Subjects = sorted(subjectSets[arm][id])
			r.Clonotypes = sorted(cloneSets[arm][id])
			if arm == 0 {
				s.QueryRecords = append(s.QueryRecords, *r)
			} else {
				s.BackgroundRecords = append(s.BackgroundRecords, *r)
			}
		}
	}
	if len(s.QueryRecords) == 0 || len(s.BackgroundRecords) == 0 {
		return s, errors.New("both arms need eligible positive TRB receptors")
	}
	s.Notice = a.notice(ds, s.Unit, s.Filter, s.Definition.Describe(), s.Definition.ID())
	s.Notice.What = motifs.Description
	s.Notice.Method += " " + motifs.Citation
	s.Notice.Derived = true
	for _, source := range ds.Result.Metadata {
		s.Notice.Files = append(s.Notice.Files, export.File{Name: source.Name, SHA256: source.SHA256})
	}
	return s, nil
}
func (a *app) registerMotifs(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/motifs", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			motifsRequest
			Start *bool `json:"start"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s, err := a.captureMotifs(body.motifsRequest)
		if err != nil {
			writeError(w, 422, err.Error())
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.State == artifacts.Complete && m.Kind == motifsKind {
			var doc motifsDocument
			if _, _, e = a.readCheckedResult(id, motifsKind, &doc); e != nil {
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
		j, _, err := a.jobs.Submit(motifsKind, "Motifs · "+s.Notice.Dataset, id, s, a.motifsWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "job": j.ID, "ready": false})
	})
	mux.HandleFunc("GET /api/motifs/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc motifsDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), motifsKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/motifs/{id}/group/{group}", func(w http.ResponseWriter, r *http.Request) {
		var doc motifsDocument
		_, _, err := a.readCheckedResult(r.PathValue("id"), motifsKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		for _, g := range doc.Result.Groups {
			if g.ID == r.PathValue("group") {
				writeJSON(w, map[string]any{"group": g, "profile": motifs.Residues(doc.Result, g)})
				return
			}
		}
		writeError(w, 404, "Signature not found.")
	})
	mux.HandleFunc("GET /api/motifs/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc motifsDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), motifsKind, &doc)
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
			}{{"motifs.json", doc}, {"manifest.json", m}} {
				out, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if _, e = out.Write(mustJSON(f.value)); e != nil {
					return e
				}
			}
			out, e := z.Create("signatures.csv")
			if e != nil {
				return e
			}
			if e = motifs.CSV(out, doc.Result); e != nil {
				return e
			}
			out, e = z.Create("receptors.csv")
			if e != nil {
				return e
			}
			c := csv.NewWriter(out)
			if e = c.Write([]string{"arm", "id", "v", "junction_aa", "samples", "subjects", "clonotypes", "abundance", "unit"}); e != nil {
				return e
			}
			for arm, rows := range [][]motifs.Record{doc.Result.Query, doc.Result.Background} {
				for _, row := range rows {
					if e = c.Write([]string{[]string{"query", "background"}[arm], row.ID, row.V, row.Junction, strings.Join(row.Samples, ";"), strings.Join(row.Subjects, ";"), strings.Join(row.Clonotypes, ";"), strconv.FormatFloat(row.Abundance, 'g', -1, 64), doc.Evidence.Unit}); e != nil {
						return e
					}
				}
			}
			c.Flush()
			if e = c.Error(); e != nil {
				return e
			}
			return writeNotice(z, n)
		})
	})
}
func (a *app) motifsWork(s motifsSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		result, err := motifs.Compute(ctx, s.QueryRecords, s.BackgroundRecords, s.Options, func(p float64) { run.Progress(.15+float64(.7*p), "Testing pooled sequence signatures") })
		if err != nil {
			return err
		}
		desc := fmt.Sprintf("%s Dataset %s; definition %s; QC %s; abundance %s (descriptive only). Query samples %v; background samples %v. %d query and %d background units, %d shared identities across arms. Family %d; tested %d; reported %d at BY <= %g, query members >= %d and fold >= %g (zero background matches shown explicitly).", motifs.Description, s.Notice.Dataset, s.Definition.Describe(), s.Filter.Describe(), s.Unit, s.Query, s.Background, len(result.Query), len(result.Background), result.Shared, result.Family, result.Tested, result.Reported, s.Options.Q, s.Options.MinMembers, s.Options.MinFold)
		doc := motifsDocument{Format: "clonoloom-motifs", Version: motifs.Version, Description: desc, Evidence: s, Result: result}
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
		if err = os.WriteFile(filepath.Join(dir, "motifs.json"), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: motifsKind, Name: "Motifs · " + s.Notice.Dataset, Dataset: s.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: "dataset", ID: s.Dataset, Name: s.Notice.Dataset}}, Job: &artifacts.Job{ID: run.ID(), Kind: motifsKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.motifsRequest), Method: artifacts.Method{Name: "GLIPH2-style sequence signatures (ClonoLoom adaptation)", Version: "1", Citation: motifs.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: "motifs.json", Role: motifsKind}}, Summary: mustJSON(map[string]any{"family": result.Family, "tested": result.Tested, "reported": result.Reported})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, "motifs.json"))
			var back motifsDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Result == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "motifs.unreadable", Message: "Motif result failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("motif promotion failed")
		}
		return nil
	}
}
func (a *app) handleMotifsJobs() {
	a.jobs.Handle(motifsKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s motifsSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if err := s.check(); err != nil {
			return nil, err
		}
		return a.motifsWork(s), nil
	}, nil)
}

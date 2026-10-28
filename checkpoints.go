package main

// Named analysis snapshots and undoable recovery follow Apache-2.0 CytoWeave
// 0.8.1's checkpoint workflow. Go preserves saved evidence bytes and computes
// descriptive differences here; recovery is not scientific recomputation.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
)

const checkpointProjectLimit = 16 << 20
const checkpointLimits = "Local checkpoint recovery restores recorded analysis settings and attached completed result files, not original receptor/expression sources, separate unattached parents, reference installations, running jobs or a whole library. It performs no scientific recomputation. Exact certificate replay remains limited to the published per-kind support matrix. Recorded differences are descriptive, not effects, significance or biological validation. Evidence can contain identifiers, sequences and paths; it is not de-identified or a redistribution license."

var checkpointEnvelope = map[string]bool{"format": true, "version": true, "id": true, "name": true, "created": true, "modified": true, "history": true, "checkpoints": true, "portableArchive": true}

type checkpointRecord struct {
	Format   string             `json:"format"`
	Version  int                `json:"version"`
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Created  string             `json:"created"`
	Project  string             `json:"project"`
	Software artifacts.Software `json:"software"`
	Files    []runOutput        `json:"files"`
	Limits   string             `json:"limits"`
}
type checkpointData struct {
	Record   checkpointRecord
	Files    map[string][]byte
	Analysis json.RawMessage
	Results  map[string]json.RawMessage
}
type checkpointChange struct {
	Category string `json:"category"`
	Path     string `json:"path"`
	Summary  string `json:"summary"`
	Before   string `json:"beforeJSON"`
	After    string `json:"afterJSON"`
}
type checkpointDifference struct {
	Format  string             `json:"format"`
	Version int                `json:"version"`
	Before  string             `json:"before"`
	After   string             `json:"after"`
	Changes []checkpointChange `json:"changes"`
	Limits  string             `json:"limits"`
}

func checkpointProject(raw []byte) (string, json.RawMessage, []string, error) {
	if len(raw) > checkpointProjectLimit {
		return "", nil, nil, errors.New("checkpoint project exceeds 16 MiB")
	}
	var doc map[string]json.RawMessage
	if err := strictRunJSON(raw, &doc); err != nil {
		return "", nil, nil, err
	}
	var format, id string
	var v int
	json.Unmarshal(doc["format"], &format)
	json.Unmarshal(doc["version"], &v)
	json.Unmarshal(doc["id"], &id)
	if format != "clonoloom-project" || v != 1 || !projectIDPattern.MatchString(id) {
		return "", nil, nil, errors.New("checkpoint needs a supported version-1 project and valid identity")
	}
	for _, key := range []string{"datasets", "artifacts", "decisions", "views", "samples", "filters", "definitions", "expression", "cohortDesigns", "history", "checkpoints"} {
		if value, ok := doc[key]; ok && (!bytes.HasPrefix(bytes.TrimSpace(value), []byte("["))) {
			return "", nil, nil, fmt.Errorf("project %s must be a list", key)
		}
	}
	var refs []struct {
		ID string `json:"id"`
	}
	if value, ok := doc["artifacts"]; ok {
		if err := json.Unmarshal(value, &refs); err != nil {
			return "", nil, nil, err
		}
	}
	if len(refs) > 128 {
		return "", nil, nil, errors.New("checkpoint supports at most 128 attached results")
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, ref := range refs {
		if seen[ref.ID] || ref.ID == "" {
			return "", nil, nil, errors.New("duplicate or empty result identity")
		}
		seen[ref.ID] = true
		ids = append(ids, ref.ID)
	}
	analysis := map[string]json.RawMessage{}
	for k, v := range doc {
		if !checkpointEnvelope[k] {
			analysis[k] = v
		}
	}
	// Match parseProject's version-1 defaults for older minimal envelopes while
	// retaining the original project.json bytes in the inventory.
	for _, key := range []string{"datasets", "artifacts", "decisions", "views"} {
		if _, ok := analysis[key]; !ok {
			analysis[key] = json.RawMessage(`[]`)
		}
	}
	return id, mustJSON(analysis), ids, nil
}
func checkpointRead(ctx context.Context, root *os.Root, path string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := root.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("checkpoint file %s is not a bounded regular file", path)
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(runContextReader{ctx, io.LimitReader(f, limit+1)})
	if int64(len(raw)) > limit {
		return nil, errors.New("checkpoint file exceeds bound")
	}
	return raw, err
}
func (a *app) collectCheckpoint(ctx context.Context, raw []byte, base string) (checkpointData, error) {
	d := checkpointData{Files: map[string][]byte{"project.json": raw}, Results: map[string]json.RawMessage{}}
	id, analysis, ids, err := checkpointProject(raw)
	if err != nil {
		return d, err
	}
	d.Analysis = analysis
	d.Record.Project = id
	var total = int64(len(raw))
	for _, id := range ids {
		if _, err := a.artifacts.Path(id, "result.json"); err != nil {
			return d, err
		}
		folder := filepath.Join(base, id)
		if info, err := os.Lstat(folder); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return d, fmt.Errorf("result %s needs an existing local directory, not a link", id)
		}
		root, err := os.OpenRoot(folder)
		if err != nil {
			return d, fmt.Errorf("result %s unavailable: %w", id, err)
		}
		err = func() error {
			defer root.Close()
			manifest, err := checkpointRead(ctx, root, "manifest.json", certificate.MaxManifestBytes)
			if err != nil {
				return err
			}
			m, err := certificate.Manifest(manifest)
			if err != nil {
				return fmt.Errorf("result %s: %w", id, err)
			}
			if m.ID != id {
				return errors.New("result identity differs from its directory")
			}
			// Keep the original manifest, including numeric tokens and provenance.
			d.Files["artifacts/"+id+"/manifest.json"] = manifest
			total += int64(len(manifest))
			for _, file := range m.Files {
				if file.Path == "manifest.json" {
					return errors.New("result file shadows its manifest")
				}
				if total+file.Size > certificate.MaxBytes || len(d.Files) >= 256 {
					return errors.New("checkpoint exceeds 256 files or 512 MiB")
				}
				contents, err := checkpointRead(ctx, root, file.Path, certificate.MaxFileBytes)
				if err != nil {
					return err
				}
				if int64(len(contents)) != file.Size || replayDigest(contents) != file.SHA256 {
					return fmt.Errorf("result %s file %s differs from its recorded checksum", id, file.Path)
				}
				d.Files["artifacts/"+id+"/"+file.Path] = contents
				total += int64(len(contents))
			}
			d.Results[id] = mustJSON(struct {
				Kind           string             `json:"kind"`
				Parameters     json.RawMessage    `json:"parameters"`
				Method         artifacts.Method   `json:"method"`
				Parents        []artifacts.Parent `json:"parents"`
				Files          []artifacts.File   `json:"files"`
				ManifestSHA256 string             `json:"manifestSHA256"`
			}{m.Kind, m.Lineage.Parameters, m.Lineage.Method, m.Lineage.Parents, m.Files, replayDigest(manifest)})
			return nil
		}()
		if err != nil {
			return d, err
		}
	}
	if total > certificate.MaxBytes || len(d.Files) > 256 {
		return d, errors.New("checkpoint exceeds 256 files or 512 MiB")
	}
	names := make([]string, 0, len(d.Files))
	for path := range d.Files {
		names = append(names, path)
	}
	sort.Strings(names)
	for _, path := range names {
		raw := d.Files[path]
		d.Record.Files = append(d.Record.Files, runOutput{path, int64(len(raw)), replayDigest(raw)})
	}
	return d, nil
}
func checkpointID(r checkpointRecord) string { r.ID = ""; return "cp-" + replayDigest(mustJSON(r)) }
func (a *app) checkpointPath(id string) (string, error) {
	if !strings.HasPrefix(id, "cp-") || !runDigest(strings.TrimPrefix(id, "cp-")) {
		return "", errors.New("invalid checkpoint identity")
	}
	return filepath.Join(a.store.dir, "checkpoints", id), nil
}
func checkpointWrite(ctx context.Context, dir string, files map[string][]byte) error {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		// Match the library's save durability: sync each staged file before promotion.
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = f.Write(files[name])
		if err == nil {
			err = f.Sync()
		}
		closed := f.Close()
		if err == nil {
			err = closed
		}
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}
func (a *app) createCheckpoint(ctx context.Context, raw []byte, name string) (checkpointRecord, error) {
	var empty checkpointRecord
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 {
		return empty, errors.New("checkpoint name needs 1–200 bytes")
	}
	d, err := a.collectCheckpoint(ctx, raw, a.artifacts.Dir())
	if err != nil {
		return empty, err
	}
	d.Record.Format = "clonoloom-checkpoint"
	d.Record.Version = 1
	d.Record.Name = name
	d.Record.Created = time.Now().UTC().Format(time.RFC3339Nano)
	d.Record.Software = artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}
	d.Record.Limits = checkpointLimits
	d.Record.ID = checkpointID(d.Record)
	recordBytes := mustJSON(d.Record)
	if len(recordBytes) > 1<<20 {
		return empty, errors.New("checkpoint record exceeds 1 MiB")
	}
	parent := filepath.Join(a.store.dir, "checkpoints")
	if err = os.MkdirAll(parent, 0700); err != nil {
		return empty, err
	}
	stage, err := os.MkdirTemp(parent, ".creating-")
	if err != nil {
		return empty, err
	}
	defer os.RemoveAll(stage)
	d.Files["checkpoint.json"] = recordBytes
	if err = checkpointWrite(ctx, stage, d.Files); err != nil {
		return empty, err
	}
	path, _ := a.checkpointPath(d.Record.ID)
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return empty, errors.New("checkpoint destination already exists or is unavailable")
	}
	if err = os.Rename(stage, path); err != nil {
		return empty, err
	}
	return d.Record, nil
}
func (a *app) loadCheckpoint(ctx context.Context, id string) (checkpointData, error) {
	var d checkpointData
	path, err := a.checkpointPath(id)
	if err != nil {
		return d, err
	}
	if info, err := os.Lstat(path); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return d, errors.New("checkpoint needs an existing local directory, not a link")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return d, err
	}
	defer root.Close()
	raw, err := checkpointRead(ctx, root, "checkpoint.json", 1<<20)
	if err != nil {
		return d, err
	}
	var record checkpointRecord
	if err = strictRunJSON(raw, &record); err != nil {
		return d, err
	}
	if record.Format != "clonoloom-checkpoint" || record.Version != 1 || record.ID != id || checkpointID(record) != id {
		return d, errors.New("checkpoint version, identity or inventory differs")
	}
	project, err := checkpointRead(ctx, root, "project.json", checkpointProjectLimit)
	if err != nil {
		return d, err
	}
	d, err = a.collectCheckpoint(ctx, project, filepath.Join(path, "artifacts"))
	if err != nil {
		return d, err
	}
	if d.Record.Project != record.Project || !bytes.Equal(mustJSON(d.Record.Files), mustJSON(record.Files)) {
		return d, errors.New("checkpoint bytes differ from the immutable inventory")
	}
	d.Record = record
	return d, nil
}

// Preflight every destination before installing any missing result. Never replace a
// conflicting result. Interrupted recovery can leave verified, unreferenced results;
// retry is idempotent and the browser commits analysis only after all files succeed.
func (a *app) recoverCheckpoint(ctx context.Context, d checkpointData) error {
	ids := make([]string, 0, len(d.Results))
	for id := range d.Results {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	missing := []string{}
	for _, id := range ids {
		target := filepath.Join(a.artifacts.Dir(), id)
		if info, err := os.Lstat(target); os.IsNotExist(err) {
			missing = append(missing, id)
			continue
		} else if err != nil {
			return err
		} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("existing recovery destination is not a local directory")
		}
		root, err := os.OpenRoot(target)
		if err != nil {
			return err
		}
		err = func() error {
			defer root.Close()
			prefix := "artifacts/" + id + "/"
			for name, want := range d.Files {
				if !strings.HasPrefix(name, prefix) {
					continue
				}
				got, err := checkpointRead(ctx, root, strings.TrimPrefix(name, prefix), int64(len(want)))
				if err != nil || !bytes.Equal(got, want) {
					return fmt.Errorf("existing result %s conflicts with checkpoint; it was not overwritten", id)
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
	for _, id := range missing {
		if err := ctx.Err(); err != nil {
			return err
		}
		stage, err := os.MkdirTemp(a.artifacts.Dir(), ".recovering-")
		if err != nil {
			return err
		}
		err = func() error {
			defer os.RemoveAll(stage)
			files := map[string][]byte{}
			prefix := "artifacts/" + id + "/"
			for name, raw := range d.Files {
				if strings.HasPrefix(name, prefix) {
					files[strings.TrimPrefix(name, prefix)] = raw
				}
			}
			if err := checkpointWrite(ctx, stage, files); err != nil {
				return err
			}
			target := filepath.Join(a.artifacts.Dir(), id)
			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				return errors.New("recovery destination appeared; retry after checking it")
			}
			return os.Rename(stage, target)
		}()
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}
func checkpointCategory(path string) string {
	if strings.Contains(path, "/definition") {
		return "Clonotype definitions"
	}
	if strings.Contains(path, "/filter") {
		return "QC filters"
	}
	for _, p := range []struct{ Prefix, Name string }{{"/datasets", "Sources and dataset settings"}, {"/expression", "Expression sources"}, {"/definitions", "Clonotype definitions"}, {"/filters", "QC filters"}, {"/samples", "Sample populations and annotations"}, {"/cohortDesigns", "Cohort populations and design"}, {"/artifacts", "Attached result identities"}, {"/results", "Saved result evidence and parameters"}, {"/views", "Saved views and selections"}, {"/decisions", "Recorded decisions"}} {
		if strings.HasPrefix(path, p.Prefix) {
			return p.Name
		}
	}
	return "Other analysis settings"
}

// Compare JSON structures with raw scalar tokens: no float conversion, approximate
// equality or inferred effect sizes. Missing and null remain distinct; array order matters.
func checkpointDiff(before, after json.RawMessage, path string, out *[]checkpointChange) error {
	before, after = bytes.TrimSpace(before), bytes.TrimSpace(after)
	var compactA, compactB bytes.Buffer
	json.Compact(&compactA, before)
	json.Compact(&compactB, after)
	if bytes.Equal(compactA.Bytes(), compactB.Bytes()) {
		return nil
	}
	if len(before) > 0 && len(after) > 0 && before[0] == '{' && after[0] == '{' {
		var a, b map[string]json.RawMessage
		json.Unmarshal(before, &a)
		json.Unmarshal(after, &b)
		keys := map[string]bool{}
		for k := range a {
			keys[k] = true
		}
		for k := range b {
			keys[k] = true
		}
		names := []string{}
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			pointer := strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
			if err := checkpointDiff(a[k], b[k], path+"/"+pointer, out); err != nil {
				return err
			}
		}
		return nil
	}
	if len(before) > 0 && len(after) > 0 && before[0] == '[' && after[0] == '[' {
		var a, b []json.RawMessage
		json.Unmarshal(before, &a)
		json.Unmarshal(after, &b)
		n := max(len(a), len(b))
		for i := 0; i < n; i++ {
			var x, y json.RawMessage
			if i < len(a) {
				x = a[i]
			}
			if i < len(b) {
				y = b[i]
			}
			if err := checkpointDiff(x, y, fmt.Sprintf("%s/%d", path, i), out); err != nil {
				return err
			}
		}
		return nil
	}
	old, new := string(before), string(after)
	if len(before) == 0 {
		old = "(absent)"
	}
	if len(after) == 0 {
		new = "(absent)"
	}
	action := "changed"
	if len(before) == 0 {
		action = "added"
	} else if len(after) == 0 {
		action = "removed"
	}
	category := checkpointCategory(path)
	if len(*out) >= 4096 {
		return errors.New("more than 4096 differences; narrow the analysis before comparing")
	}
	summary := category + ": " + path + " " + action + "."
	if len(old) <= 80 && len(new) <= 80 {
		summary = category + ": " + path + " " + action + " from " + old + " to " + new + "."
	}
	*out = append(*out, checkpointChange{category, path, summary, old, new})
	return nil
}
func compareCheckpoints(a, b checkpointData) (checkpointDifference, error) {
	r := checkpointDifference{Format: "clonoloom-checkpoint-diff", Version: 1, Before: a.Record.ID, After: b.Record.ID, Changes: []checkpointChange{}, Limits: checkpointLimits}
	if r.After == "" {
		r.After = "current analysis"
	}
	if a.Record.Project != b.Record.Project {
		return r, errors.New("checkpoints belong to different projects")
	}
	if err := checkpointDiff(a.Analysis, b.Analysis, "", &r.Changes); err != nil {
		return r, err
	}
	err := checkpointDiff(mustJSON(a.Results), mustJSON(b.Results), "/results", &r.Changes)
	return r, err
}
func (a *app) registerCheckpoints(mux *http.ServeMux) {
	type body struct {
		Project json.RawMessage `json:"project"`
		Name    string          `json:"name,omitempty"`
		Against string          `json:"against,omitempty"`
	}
	read := func(r *http.Request) (body, error) {
		var b body
		raw, err := io.ReadAll(io.LimitReader(r.Body, checkpointProjectLimit+1))
		if err != nil {
			return b, err
		}
		if len(raw) > checkpointProjectLimit {
			return b, errors.New("checkpoint request exceeds 16 MiB")
		}
		err = strictRunJSON(raw, &b)
		return b, err
	}
	mux.HandleFunc("POST /api/checkpoints", func(w http.ResponseWriter, r *http.Request) {
		b, err := read(r)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		record, err := a.createCheckpoint(r.Context(), b.Project, b.Name)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, record)
	})
	mux.HandleFunc("POST /api/checkpoints/{id}/{operation}", func(w http.ResponseWriter, r *http.Request) {
		operation := r.PathValue("operation")
		if operation != "compare" && operation != "recover" {
			writeError(w, 404, "Unknown checkpoint operation.")
			return
		}
		b, err := read(r)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		project, _, _, err := checkpointProject(b.Project)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		d, err := a.loadCheckpoint(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		if project != d.Record.Project {
			writeError(w, 409, "Checkpoint belongs to another project.")
			return
		}
		if operation == "recover" {
			if b.Against != "" {
				writeError(w, 400, "Recovery takes one checkpoint.")
				return
			}
			if err = a.portableCheckpoint(r.Context(), b.Project, &d); err != nil {
				writeError(w, 409, err.Error())
				return
			}
			if err = a.recoverCheckpoint(r.Context(), d); err != nil {
				writeError(w, 409, err.Error())
				return
			}
			writeJSON(w, map[string]any{"checkpoint": d.Record, "analysisJSON": string(d.Analysis), "limits": checkpointLimits, "recomputed": false})
			return
		}
		var other checkpointData
		if b.Against != "" {
			other, err = a.loadCheckpoint(r.Context(), b.Against)
		} else {
			other, err = a.collectCheckpoint(r.Context(), b.Project, a.artifacts.Dir())
		}
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		difference, err := compareCheckpoints(d, other)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, difference)
	})
}

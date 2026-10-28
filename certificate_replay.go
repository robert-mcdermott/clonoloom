package main

// Replay is computational reproduction from captured inputs, not replication with
// new data: NASEM (2019), Reproducibility and Replicability in Science, doi:10.17226/25303.
// Run the ordinary Go workers in a disposable store. Never install expected outputs
// as computed results or replace original provenance with new job/time metadata.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
	"clonoloom/internal/jobs"
)

const replayScope = "captured-cohort-evidence"
const replayLimits = "Exact replay from captured cohort evidence only. Embedded parent bytes and recipe bindings are checked; original receptor files, source fingerprints, upstream diversity computations, project history and authorship are not independently verified. Count models refit the captured count matrix, not a recount of source sequences. Matching bytes do not validate study assumptions, calibration, causal claims or biological truth. Captured evidence is not de-identified."

type replayStep struct {
	Artifact        string `json:"artifact"`
	Kind            string `json:"kind"`
	Expected        string `json:"expectedSHA256"`
	Actual          string `json:"actualSHA256,omitempty"`
	Verdict         string `json:"verdict"`
	MetadataMatched bool   `json:"metadataMatched"`
}
type replayReport struct {
	Format     string              `json:"format"`
	Version    int                 `json:"version"`
	Scope      string              `json:"scope"`
	Verdict    string              `json:"verdict"`
	Recomputed bool                `json:"recomputed"`
	Integrity  certificate.Report  `json:"integrity"`
	Original   *artifacts.Manifest `json:"original,omitempty"`
	Verifier   artifacts.Software  `json:"verifier"`
	Platform   string              `json:"platform"`
	Steps      []replayStep        `json:"steps"`
	Limits     string              `json:"limits"`
	Findings   []string            `json:"findings"`
}

func replayDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func replayCertificate(ctx context.Context, in io.ReaderAt, size int64, expected string) replayReport {
	r := replayReport{Format: "clonoloom-replay", Version: 1, Scope: replayScope, Verdict: "unavailable", Verifier: artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}, Platform: runtime.GOOS + "/" + runtime.GOARCH, Steps: []replayStep{}, Limits: replayLimits, Findings: []string{}}
	evidence, integrity, err := certificate.OpenEvidence(ctx, in, size, expected)
	r.Integrity = integrity
	fail := func(err error) replayReport {
		if ctx.Err() != nil {
			r.Verdict = "unavailable"
			err = ctx.Err()
		}
		r.Findings = append(r.Findings, err.Error())
		return r
	}
	if err != nil {
		if integrity.Verdict != "integrity-match" {
			r.Verdict = "differs"
		}
		return fail(err)
	}
	raw, err := evidence.Read(ctx, "manifest.json", certificate.MaxManifestBytes)
	if err != nil {
		return fail(err)
	}
	m, err := certificate.Manifest(raw)
	if err != nil {
		return fail(err)
	}
	r.Original = &m
	support, supported := certificateSupportFor(m.Kind)
	if !supported {
		return fail(errors.New("this result kind has no supported certificate replay contract"))
	}
	r.Scope, r.Limits = support.Scope, support.Limits
	output, err := evidence.Read(ctx, "artifact/"+m.Kind+".json", certificate.MaxFileBytes)
	if err != nil {
		return fail(err)
	}
	dir, err := os.MkdirTemp("", "clonoloom-replay-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(dir)
	store, err := artifacts.Open(filepath.Join(dir, "artifacts"))
	if err != nil {
		return fail(err)
	}
	manager := jobs.NewManager()
	defer manager.Close()
	a := &app{artifacts: store, jobs: manager}
	a.handleReplayJobs()
	if m.Kind == batchqcKind || m.Kind == motifsKind {
		err = replayCaptured(ctx, a, m, output, &r)
	} else {
		err = replayCohort(ctx, a, m, output, &r)
	}
	if err != nil {
		return fail(err)
	}
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	r.Verdict = "replay-match"
	r.Recomputed = true
	return r
}

// Decode without silently dropping unknown fields, duplicate keys or changed
// numeric tokens. Saved worker recipes use precisely these typed JSON encodings.
func replaySpec(raw []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	canonical, err := json.Marshal(out)
	if err != nil {
		return err
	}
	var compact bytes.Buffer
	if err = json.Compact(&compact, raw); err != nil {
		return err
	}
	if !bytes.Equal(compact.Bytes(), canonical) {
		return errors.New("recipe encoding is unsupported or ambiguous")
	}
	return nil
}
func replayCohort(ctx context.Context, a *app, m artifacts.Manifest, raw []byte, r *replayReport) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.Lineage.Job == nil || m.Lineage.Job.Kind != m.Kind {
		return errors.New("recorded worker recipe is unavailable")
	}
	work, err := a.jobs.Remake(m.Kind, m.Lineage.Job.Spec)
	if err != nil {
		return err
	}
	if len(m.Files) != 1 || m.Files[0].Path != m.Kind+".json" || m.Files[0].Location != artifacts.InArtifact {
		return errors.New("replay requires the supported single-result inventory")
	}
	if int64(len(raw)) != m.Files[0].Size || replayDigest(raw) != m.Files[0].SHA256 {
		r.Verdict = "differs"
		return errors.New("embedded result differs from its parent manifest")
	}
	// Path validates the ID before any temporary filesystem operation.
	if _, err := a.artifacts.Path(m.ID, m.Kind+".json"); err != nil {
		return err
	}
	var fields struct {
		Parent         json.RawMessage `json:"parent"`
		Features       json.RawMessage `json:"features"`
		ParentManifest json.RawMessage `json:"parentManifest"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if len(fields.ParentManifest) == 0 {
		return errors.New("embedded parent manifest is unavailable")
	}
	pm, err := certificate.Manifest(fields.ParentManifest)
	if err != nil {
		return fmt.Errorf("embedded parent: %w", err)
	}
	var id, parent, hash string
	switch m.Kind {
	case cohortFeaturesKind:
		var s cohortFeaturesSpec
		if err = replaySpec(m.Lineage.Job.Spec, &s); err != nil {
			return err
		}
		id, parent, hash = s.artifactID(), s.Diversity, s.SHA256
		if pm.Kind != "diversity" {
			return errors.New("required diversity parent is unavailable")
		}
	case cohortCompareKind:
		var s cohortCompareSpec
		if err = replaySpec(m.Lineage.Job.Spec, &s); err != nil {
			return err
		}
		id, parent, hash = s.artifactID(), s.Features, s.SHA256
		if pm.Kind != cohortFeaturesKind {
			return errors.New("required feature parent is unavailable")
		}
	case cohortCountsKind:
		var s cohortCountsSpec
		if err = replaySpec(m.Lineage.Job.Spec, &s); err != nil {
			return err
		}
		id, parent, hash = s.artifactID(), s.Features, s.SHA256
		if len(s.Input.Samples) == 0 || len(s.Input.Totals) == 0 {
			return errors.New("captured count input is unavailable")
		}
		if pm.Kind != cohortFeaturesKind {
			return errors.New("required feature parent is unavailable")
		}
	default:
		return errors.New("unsupported replay kind")
	}
	if id != m.ID || parent != pm.ID || hash == "" {
		r.Verdict = "differs"
		return errors.New("recipe artifact or parent identity differs")
	}
	parentRaw := fields.Features
	if m.Kind == cohortFeaturesKind {
		parentRaw = fields.Parent
	}
	if len(parentRaw) == 0 || bytes.Equal(parentRaw, []byte("null")) {
		return errors.New("captured parent input is unavailable")
	}
	if replayDigest(parentRaw) != hash {
		r.Verdict = "differs"
		return errors.New("captured parent bytes differ from the recipe SHA-256")
	}
	if len(pm.Files) != 1 || pm.Files[0].Path != pm.Kind+".json" {
		return errors.New("unsupported embedded parent inventory")
	}
	if pm.Files[0].Size != int64(len(parentRaw)) || pm.Files[0].SHA256 != hash {
		r.Verdict = "differs"
		return errors.New("embedded parent manifest differs from its captured bytes")
	}
	bound := false
	for _, p := range m.Lineage.Parents {
		if p.ID == parent && p.Kind == pm.Kind && p.SHA256 == hash {
			bound = true
		}
	}
	if !bound {
		r.Verdict = "differs"
		return errors.New("manifest parent binding differs from the recipe")
	}
	if pm.Kind == cohortFeaturesKind {
		if err = replayCohort(ctx, a, pm, parentRaw, r); err != nil {
			return err
		}
	}
	// Install verified original parent evidence, preserving original provenance.
	// A replayed feature result has already been compared byte for byte above.
	if err = installReplayParent(a, pm, parentRaw); err != nil {
		return err
	}
	return replayWorker(ctx, a, m, raw, r, work)
}

func replayWorker(ctx context.Context, a *app, m artifacts.Manifest, raw []byte, r *replayReport, work jobs.Func) error {
	job, _, err := a.jobs.Submit(m.Kind, "Certificate replay", m.ID, m.Lineage.Job.Spec, work)
	if err != nil {
		return err
	}
	completed, err := a.jobs.Wait(ctx, job.ID)
	if err != nil {
		a.jobs.Cancel(job.ID)
		return err
	}
	if completed.State != jobs.Complete {
		return fmt.Errorf("%s replay unavailable: %s", m.Kind, completed.Error)
	}
	path, err := a.artifacts.Path(m.ID, m.Kind+".json")
	if err != nil {
		return err
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	computed, err := a.artifacts.Get(m.ID)
	if err != nil {
		return err
	}
	// Compare scientific metadata, retaining original execution provenance separately.
	metadata := func(v artifacts.Manifest) []byte {
		v.Created = m.Created
		v.Lineage.Software = m.Lineage.Software
		v.Lineage.Job = &artifacts.Job{Kind: v.Lineage.Job.Kind, Spec: v.Lineage.Job.Spec}
		v.Files = nil
		v.Findings = nil
		return mustJSON(v)
	}
	step := replayStep{Artifact: m.ID, Kind: m.Kind, Expected: replayDigest(raw), Actual: replayDigest(got), Verdict: "replay-match", MetadataMatched: bytes.Equal(metadata(computed), metadata(m))}
	if m.ID == r.Original.ID {
		r.Recomputed = true
	}
	if !bytes.Equal(got, raw) || !step.MetadataMatched {
		step.Verdict = "differs"
		r.Verdict = "differs"
	}
	r.Steps = append(r.Steps, step)
	if step.Verdict == "differs" {
		return fmt.Errorf("%s output bytes or scientific metadata differ; no tolerance fallback", m.Kind)
	}
	return nil
}
func installReplayParent(a *app, m artifacts.Manifest, raw []byte) error {
	if len(m.Files) != 1 || m.Files[0].Path != m.Kind+".json" || m.Files[0].Location != artifacts.InArtifact || m.Files[0].Size != int64(len(raw)) || m.Files[0].SHA256 != replayDigest(raw) {
		return errors.New("embedded parent inventory or checksum differs")
	}
	path, err := a.artifacts.Path(m.ID, m.Kind+".json")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Dir(path), "manifest.json"), mustJSON(m), 0600)
}

// These workers take complete captured inputs, without a separate artifact parent.
func replayCaptured(ctx context.Context, a *app, m artifacts.Manifest, raw []byte, r *replayReport) error {
	if m.Lineage.Job == nil || m.Lineage.Job.Kind != m.Kind {
		return errors.New("recorded worker recipe unavailable")
	}
	if len(m.Files) != 1 || m.Files[0].Location != artifacts.InArtifact || m.Files[0].Path != m.Kind+".json" || m.Files[0].Size != int64(len(raw)) || m.Files[0].SHA256 != replayDigest(raw) {
		r.Verdict = "differs"
		return errors.New("result inventory differs")
	}
	var id string
	switch m.Kind {
	case batchqcKind:
		var s batchqcSpec
		if err := replaySpec(m.Lineage.Job.Spec, &s); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		id = s.artifactID()
		if s.Notice.DatasetID != s.Dataset || len(s.Notice.Files) == 0 {
			return errors.New("captured source notice is incomplete")
		}
	case motifsKind:
		var s motifsSpec
		if err := replaySpec(m.Lineage.Job.Spec, &s); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		id = s.artifactID()
		if s.Notice.DatasetID != s.Dataset || len(s.Notice.Files) == 0 {
			return errors.New("captured source notice is incomplete")
		}
	default:
		return errors.New("unsupported captured worker")
	}
	if id != m.ID {
		r.Verdict = "differs"
		return errors.New("recipe artifact identity differs")
	}
	work, err := a.jobs.Remake(m.Kind, m.Lineage.Job.Spec)
	if err != nil {
		return err
	}
	return replayWorker(ctx, a, m, raw, r, work)
}

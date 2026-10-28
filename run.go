package main

// Named inputs, isolated execution, explicit tasks and run.json follow Apache-2.0
// CytoWeave 0.8.1 / MaveScape 0.8.0 run.go. ClonoLoom uses its native Go workers;
// it starts no browser/server and executes no shell or arbitrary recorded action.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
	"clonoloom/internal/export"
	"clonoloom/internal/jobs"
	"clonoloom/internal/reads"
)

const recipeLimit = 8 << 20
const runLimits = "Compute runs use only the explicitly named source snapshots; certificate replay uses captured evidence at its reported boundary. No task pooling, source discovery, external commands or network access. Results retain the existing method assumptions and limitations; successful execution is not scientific validation or evidence of antigen specificity. Outputs can contain identifiers and sequences and are not de-identified."

var runName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var runFileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type recipeInput struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type recipeTask struct {
	ID          string          `json:"id"`
	Operation   string          `json:"operation"`
	Kind        string          `json:"kind"`
	Study       string          `json:"study,omitempty"`
	Inputs      []recipeInput   `json:"inputs"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Credits     []export.Credit `json:"credits,omitempty"`
	Fingerprint string          `json:"fingerprint,omitempty"`
}
type runRecipe struct {
	Format  string       `json:"format"`
	Version int          `json:"version"`
	Time    string       `json:"time,omitempty"`
	Tasks   []recipeTask `json:"tasks"`
}
type runOutput struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type runRecord struct {
	Format       string             `json:"format"`
	Version      int                `json:"version"`
	Task         string             `json:"task"`
	Operation    string             `json:"operation"`
	Kind         string             `json:"kind"`
	Status       string             `json:"status"`
	RecipeSHA256 string             `json:"recipeSHA256"`
	Time         string             `json:"time"`
	Software     artifacts.Software `json:"software"`
	Inputs       []recipeInput      `json:"inputs"`
	Outputs      []runOutput        `json:"outputs"`
	Artifact     string             `json:"artifact,omitempty"`
	Findings     []string           `json:"findings"`
	Limits       string             `json:"limits"`
}
type inputBindings map[string]string

func (b *inputBindings) String() string { return "NAME=PATH" }
func (b *inputBindings) Set(s string) error {
	name, path, ok := strings.Cut(s, "=")
	if !ok || !runName.MatchString(name) || path == "" {
		return errors.New("--input needs NAME=PATH")
	}
	if *b == nil {
		*b = inputBindings{}
	}
	if _, ok := (*b)[name]; ok {
		return fmt.Errorf("duplicate input %q", name)
	}
	(*b)[name] = path
	return nil
}

// Inspect tokens before typed decoding: encoding/json otherwise accepts duplicate
// object keys. UseNumber keeps this structural check out of numeric conversion.
func strictRunJSON(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 64 {
			return errors.New("recipe JSON nesting exceeds 64")
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return errors.New("duplicate or invalid JSON key")
				}
				seen[key] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e := walk(depth + 1); e != nil {
					return e
				}
			}
		default:
			return errors.New("invalid JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("send exactly one JSON value")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}
func readRunRecipe(path, selected string) (runRecipe, recipeTask, []byte, error) {
	var recipe runRecipe
	var task recipeTask
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return recipe, task, nil, errors.New("recipe must be a regular file, not a link or special file")
	}
	f, err := os.Open(path)
	if err != nil {
		return recipe, task, nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return recipe, task, nil, errors.New("recipe must be a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, recipeLimit+1))
	if err != nil {
		return recipe, task, nil, err
	}
	if len(raw) > recipeLimit {
		return recipe, task, nil, errors.New("recipe exceeds 8 MiB")
	}
	if err = strictRunJSON(raw, &recipe); err != nil {
		return recipe, task, nil, err
	}
	if recipe.Format != "clonoloom-run-recipe" || recipe.Version != 1 || len(recipe.Tasks) == 0 || len(recipe.Tasks) > 64 {
		return recipe, task, nil, errors.New("need a version-1 recipe with 1–64 tasks")
	}
	if selected == "" {
		if len(recipe.Tasks) != 1 {
			return recipe, task, nil, errors.New("a batch recipe requires an explicit --task ID; tasks are never pooled")
		}
		selected = recipe.Tasks[0].ID
	}
	found := false
	names := map[string]bool{}
	for _, v := range recipe.Tasks {
		if !runName.MatchString(v.ID) || names[v.ID] {
			return recipe, task, nil, errors.New("invalid or duplicate task ID")
		}
		names[v.ID] = true
		if v.ID == selected {
			task = v
			found = true
		}
	}
	if !found {
		return recipe, task, nil, errors.New("selected task is absent")
	}
	if err = validateRunTask(task); err != nil {
		return recipe, task, nil, err
	}
	if recipe.Time == "" {
		recipe.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if t, e := time.Parse(time.RFC3339Nano, recipe.Time); e != nil || t.IsZero() {
		return recipe, task, nil, errors.New("recipe time must be RFC3339")
	}
	return recipe, task, raw, nil
}
func validateRunTask(t recipeTask) error {
	support, ok := certificateSupportFor(t.Kind)
	if !ok {
		return errors.New("unsupported kind; see the headless/replay support matrix")
	}
	if t.Operation != "compute" && t.Operation != "replay" {
		return errors.New("operation must be compute or replay")
	}
	if t.Operation == "compute" && (!support.SourceRun || t.Study == "" || len(t.Study) > 4096 || t.Fingerprint != "") {
		return errors.New("source computation requires batch-qc or motifs, a study name and no certificate fingerprint")
	}
	if t.Operation == "replay" && (len(t.Inputs) != 1 || len(t.Parameters) > 0 || len(t.Credits) > 0 || t.Study != "") {
		return errors.New("replay needs one certificate input and no replacement parameters, study or credits")
	}
	if t.Fingerprint != "" && !runDigest(t.Fingerprint) {
		return errors.New("invalid certificate fingerprint")
	}
	if len(t.Inputs) == 0 || len(t.Inputs) > 256 {
		return errors.New("task needs 1–256 explicitly named inputs")
	}
	names, files := map[string]bool{}, map[string]bool{}
	var total int64
	for _, in := range t.Inputs {
		if !runName.MatchString(in.Name) || names[in.Name] || !runFileName.MatchString(in.File) || strings.HasSuffix(in.File, ".") || files[strings.ToLower(in.File)] || in.Size < 0 || in.Size > certificate.MaxFileBytes || !runDigest(in.SHA256) {
			return errors.New("invalid, duplicate or oversized input declaration")
		}
		if t.Operation == "compute" && !(strings.HasSuffix(in.File, ".tsv") || strings.HasSuffix(in.File, ".tsv.gz") || strings.HasSuffix(in.File, ".airr.json")) {
			return errors.New("source recipes currently accept AIRR TSV, gzipped TSV and explicit AIRR Repertoire metadata only")
		}
		names[in.Name] = true
		files[strings.ToLower(in.File)] = true
		total += in.Size
	}
	if total > certificate.MaxBytes {
		return errors.New("task inputs exceed 512 MiB")
	}
	return nil
}
func runDigest(s string) bool {
	raw, e := hex.DecodeString(s)
	return e == nil && len(raw) == 32 && strings.ToLower(s) == s
}

func runHeadless(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("clonoloom run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	recipePath := flags.String("recipe", "", "versioned recipe JSON")
	selected := flags.String("task", "", "explicit task ID for a batch recipe")
	out := flags.String("out", "", "new output directory (never overwrite)")
	timeout := flags.Duration("timeout", 30*time.Minute, "maximum run time")
	accept := flags.Bool("accept-terms", false, "acknowledge recorded provider restrictions for private study")
	var bindings inputBindings
	flags.Var(&bindings, "input", "named input NAME=PATH; repeat for each selected-task input")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: clonoloom run --recipe RECIPE.json [--task ID] --input NAME=PATH [...] --out NEW_DIR\nExit 0: requested task completed; 1: computation/check differs or fails; 2: usage, unavailable replay, input/output or cancellation.\nNo browser, server or network; see docs/HEADLESS.md for supported kinds and exact replay boundaries.")
		flags.PrintDefaults()
	}
	positional, err := parseArgs(flags, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	bad := func(e error) int { fmt.Fprintln(stderr, "clonoloom run:", e); return 2 }
	if err != nil {
		return 2
	}
	if len(positional) != 0 || *recipePath == "" || *out == "" || *timeout <= 0 {
		return bad(errors.New("give --recipe, --out, named inputs and a positive timeout"))
	}
	recipe, task, raw, err := readRunRecipe(*recipePath, *selected)
	if err != nil {
		return bad(err)
	}
	if len(bindings) != len(task.Inputs) {
		return bad(errors.New("provide exactly the selected task's named inputs"))
	}
	for _, in := range task.Inputs {
		if bindings[in.Name] == "" {
			return bad(fmt.Errorf("missing input %s", in.Name))
		}
	}
	if err = os.Mkdir(*out, 0700); err != nil {
		return bad(fmt.Errorf("output must be a new directory: %w", err))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	record := runRecord{Format: "clonoloom-run", Version: 1, Task: task.ID, Operation: task.Operation, Kind: task.Kind, Status: "failed", RecipeSHA256: replayDigest(raw), Time: recipe.Time, Software: artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}, Inputs: task.Inputs, Outputs: []runOutput{}, Findings: []string{}, Limits: runLimits}
	code, err := executeRun(ctx, recipe, task, bindings, *out, *accept, &record)
	if err != nil {
		record.Findings = append(record.Findings, err.Error())
		fmt.Fprintln(stderr, "clonoloom run:", err)
	}
	if ctx.Err() != nil {
		record.Status = "cancelled"
		record.Findings = append(record.Findings, ctx.Err().Error())
		code = 2
	}
	if e := writeRunFile(*out, "run.json", mustJSON(record), nil); e != nil {
		return bad(e)
	}
	fmt.Fprintf(stdout, "%s: task %s; see %s\n", record.Status, task.ID, filepath.Join(*out, "run.json"))
	return code
}
func writeRunFile(dir, name string, raw []byte, record *runRecord) error {
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	closed := f.Close()
	if err == nil {
		err = closed
	}
	if err != nil {
		return err
	}
	if record != nil {
		record.Outputs = append(record.Outputs, runOutput{name, int64(len(raw)), replayDigest(raw)})
	}
	return nil
}

type runContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r runContextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func snapshotRunInput(ctx context.Context, in recipeInput, source, dest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if info, err := os.Lstat(source); err != nil || !info.Mode().IsRegular() {
		return errors.New("input must be a regular file, not a link or special file")
	}
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() != in.Size {
		return errors.New("input size or file type differs")
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), runContextReader{ctx, io.LimitReader(f, in.Size+1)})
	closed := out.Close()
	if err == nil {
		err = closed
	}
	if err != nil {
		return err
	}
	if n != in.Size || hex.EncodeToString(h.Sum(nil)) != in.SHA256 {
		return errors.New("input SHA-256 or size differs")
	}
	return nil
}
func executeRun(ctx context.Context, recipe runRecipe, task recipeTask, bindings inputBindings, out string, accept bool, record *runRecord) (int, error) {
	temp, err := os.MkdirTemp("", "clonoloom-run-")
	if err != nil {
		return 2, err
	}
	defer os.RemoveAll(temp)
	inputDir := filepath.Join(temp, "inputs")
	if err = os.Mkdir(inputDir, 0700); err != nil {
		return 2, err
	}
	var inputs []reads.Input
	for _, in := range task.Inputs {
		p := filepath.Join(inputDir, in.File)
		if err = snapshotRunInput(ctx, in, bindings[in.Name], p); err != nil {
			return 2, fmt.Errorf("input %s: %w", in.Name, err)
		}
		inputs = append(inputs, reads.Input{Path: p, Name: in.File})
	}
	recipe.Tasks = []recipeTask{task}
	if task.Operation == "replay" {
		f, e := os.Open(inputs[0].Path)
		if e != nil {
			return 2, e
		}
		defer f.Close()
		report := replayCertificate(ctx, f, task.Inputs[0].Size, task.Fingerprint)
		if report.Original != nil && report.Original.Kind != task.Kind {
			return 1, errors.New("certificate kind differs from the selected task")
		}
		if e = writeRunFile(out, "replay.json", mustJSON(report), record); e != nil {
			return 2, e
		}
		if e = writeRunFile(out, "effective-recipe.json", mustJSON(recipe), record); e != nil {
			return 2, e
		}
		record.Status = report.Verdict
		switch report.Verdict {
		case "replay-match":
			return 0, nil
		case "differs":
			return 1, errors.New("certificate replay differs; see replay.json")
		default:
			return 2, errors.New("certificate replay unavailable; see replay.json")
		}
	}
	store, err := artifacts.Open(filepath.Join(temp, "artifacts"))
	if err != nil {
		return 2, err
	}
	manager := jobs.NewManager()
	defer manager.Close()
	clock, _ := time.Parse(time.RFC3339Nano, recipe.Time)
	store.Clock = func() time.Time { return clock }
	manager.IDClock = func() time.Time { return clock }
	a := &app{artifacts: store, jobs: manager, datasets: newDatasets(), clonotypes: &clonotypeTables{}, examples: newExamples(filepath.Join(temp, "examples"), true)}
	a.handleReplayJobs()
	ds, err := a.openDataset(ctx, nil, inputs, task.Study)
	if err != nil {
		return 1, err
	}
	for _, c := range task.Credits {
		ds.Credits = append(ds.Credits, credit{Title: c.Title, Source: c.Source, License: c.License, LicenseURL: c.LicenseURL, LicenseClass: c.LicenseClass, Citation: c.Citation})
	}
	// Diagnostics can retain literal source examples too; preserve provider terms
	// before exporting them, as for the captured scientific evidence below.
	for _, c := range ds.Credits {
		if c.LicenseClass != "open" && !accept {
			return 2, errors.New("provider terms require --accept-terms for private study; no source diagnostics or scientific outputs were exported")
		}
	}
	if err = writeRunFile(out, "validation.json", mustJSON(ds.Report), record); err != nil {
		return 2, err
	}
	if !ds.Report.Usable() {
		return 1, errors.New("source validation has fatal findings; see validation.json")
	}
	var spec json.RawMessage
	var id string
	var notice export.Notice
	switch task.Kind {
	case batchqcKind:
		var request batchqcRequest
		if err = strictRunJSON(task.Parameters, &request); err != nil {
			return 2, err
		}
		if request.Dataset != "" {
			return 2, errors.New("dataset is bound from named inputs; omit its ID")
		}
		request.Dataset = ds.ID
		s, e := a.captureBatchQC(request)
		if e != nil {
			return 1, e
		}
		s.Notice.Made = clock
		spec = mustJSON(s)
		id = s.artifactID()
		notice = s.Notice
		s.Dataset = ""
		task.Parameters = mustJSON(s.batchqcRequest)
	case motifsKind:
		var request motifsRequest
		if err = strictRunJSON(task.Parameters, &request); err != nil {
			return 2, err
		}
		if request.Dataset != "" {
			return 2, errors.New("dataset is bound from named inputs; omit its ID")
		}
		request.Dataset = ds.ID
		s, e := a.captureMotifs(request)
		if e != nil {
			return 1, e
		}
		s.Notice.Made = clock
		spec = mustJSON(s)
		id = s.artifactID()
		notice = s.Notice
		s.Dataset = ""
		task.Parameters = mustJSON(s.motifsRequest)
	default:
		return 2, errors.New("unsupported source computation")
	}
	if notice.Restricted() && !accept {
		return 2, errors.New("provider terms require --accept-terms for private study; no scientific outputs were exported")
	}
	if err = ctx.Err(); err != nil {
		return 2, err
	}
	work, err := a.jobs.Remake(task.Kind, spec)
	if err != nil {
		return 1, err
	}
	j, _, err := manager.Submit(task.Kind, "Headless "+task.ID, id, spec, work)
	if err != nil {
		return 1, err
	}
	done, err := manager.Wait(ctx, j.ID)
	if err != nil {
		manager.Cancel(j.ID)
		return 2, err
	}
	if done.State != jobs.Complete {
		return 1, errors.New(done.Error)
	}
	m, n, err := a.certificateNotice(id)
	if err != nil {
		return 1, err
	}
	cert, _, err := a.makeCertificate(ctx, m, n)
	if err != nil {
		return 1, err
	}
	defer func() { cert.Close(); os.Remove(cert.Name()) }()
	raw, err := os.ReadFile(filepath.Join(store.Dir(), id, task.Kind+".json"))
	if err != nil {
		return 2, err
	}
	if err = writeRunFile(out, task.Kind+".json", raw, record); err != nil {
		return 2, err
	}
	manifest, err := os.ReadFile(filepath.Join(store.Dir(), id, "manifest.json"))
	if err != nil {
		return 2, err
	}
	if err = writeRunFile(out, "manifest.json", manifest, record); err != nil {
		return 2, err
	}
	if err = writeRunFile(out, "NOTICE.txt", []byte(n.Text()), record); err != nil {
		return 2, err
	}
	b, err := io.ReadAll(io.LimitReader(cert, certificate.MaxBytes+(8<<20)+1))
	if err != nil {
		return 2, err
	}
	if err = writeRunFile(out, "result.certificate.clz", b, record); err != nil {
		return 2, err
	}
	recipe.Tasks = []recipeTask{task}
	if err = writeRunFile(out, "effective-recipe.json", mustJSON(recipe), record); err != nil {
		return 2, err
	}
	if err = ctx.Err(); err != nil {
		return 2, err
	}
	record.Artifact = id
	record.Status = "complete"
	return 0, nil
}

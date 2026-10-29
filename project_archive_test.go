package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/expression"
	"clonoloom/internal/reads"
)

func archiveFixture(t *testing.T) (*app, []byte, string) {
	t.Helper()
	a, _, id := checkpointFixture(t)
	source := filepath.Join(t.TempDir(), "S01.tsv")
	raw := []byte("sequence_id\tjunction_aa\tv_call\tj_call\tduplicate_count\na\tCASSLGQETQYF\tTRBV5-1\tTRBJ2-5\t7\n")
	os.WriteFile(source, raw, 0600)
	p := map[string]any{"format": "clonoloom-project", "version": 1, "id": "p-archive", "name": "Artificial transfer", "created": "2026-10-28", "history": []any{map[string]any{"action": "Source opened", "detail": "Artificial S01"}}, "datasets": []any{map[string]any{"id": "d-1", "sources": []any{map[string]any{"path": source, "name": "S01.tsv", "size": len(raw), "sha256": replayDigest(raw), "format": "airr"}}, "credits": []any{}}}, "artifacts": []any{map[string]any{"id": id, "kind": cohortCompareKind}}, "decisions": []any{}, "views": []any{}, "exact": json.RawMessage(`{"big":9007199254740993,"decimal":0.12345678901234567890123456789,"negativeZero":-0.0,"null":null}`)}
	return a, mustJSON(p), id
}
func archiveRoundTrip(t *testing.T, d archiveData) archiveData {
	t.Helper()
	var buf bytes.Buffer
	if e := writeProjectArchive(context.Background(), &buf, d); e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e := writeProjectArchive(context.Background(), &again, d); e != nil || !bytes.Equal(buf.Bytes(), again.Bytes()) {
		t.Fatal("non-deterministic archive", e)
	}
	got, e := readProjectArchive(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()), d.Record.Fingerprint)
	if e != nil {
		t.Fatal(e)
	}
	return got
}
func TestPortableArchiveExactEvidenceSourcesCheckpoints(t *testing.T) {
	a, project, id := archiveFixture(t)
	cp, e := a.createCheckpoint(context.Background(), project, "Before transfer")
	if e != nil {
		t.Fatal(e)
	}
	// A valid local record may have different whitespace; transfer its bytes too.
	recordPath := filepath.Join(a.store.dir, "checkpoints", cp.ID, "checkpoint.json")
	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	recordBytes = append(recordBytes, '\n')
	if err = os.WriteFile(recordPath, recordBytes, 0600); err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	json.Unmarshal(project, &doc)
	doc["checkpoints"] = mustJSON([]any{map[string]any{"id": cp.ID}})
	project = mustJSON(doc)
	d, e := a.collectArchive(context.Background(), project, archiveOptions{IncludeSources: true, OwnSources: true})
	if e != nil {
		t.Fatal(e)
	}
	got := archiveRoundTrip(t, d)
	if !bytes.Equal(got.Files["checkpoints/"+cp.ID+"/checkpoint.json"], recordBytes) {
		t.Fatal("checkpoint record bytes changed")
	}
	if !bytes.Equal(project, got.Files["project.json"]) {
		t.Fatal("original project changed")
	}
	if len(got.Record.Sources) != 2 {
		t.Fatal(got.Record.Sources)
	}
	for _, s := range got.Record.Sources {
		if s.Status != "included" {
			t.Fatal(s)
		}
	}
	b, _ := testApp(t)
	defer b.jobs.Close()
	working, e := b.importArchive(context.Background(), got, false)
	if e != nil {
		t.Fatal(e)
	}
	var imported map[string]json.RawMessage
	json.Unmarshal(working, &imported)
	if !bytes.Equal(doc["exact"], imported["exact"]) || !bytes.Equal(doc["history"], imported["history"]) {
		t.Fatal("numeric tokens or history changed")
	}
	raw, e := os.ReadFile(filepath.Join(b.artifacts.Dir(), id, "manifest.json"))
	if e != nil || !bytes.Equal(raw, d.Files["artifacts/"+id+"/manifest.json"]) {
		t.Fatal("manifest changed", e)
	}
	sources, e := archiveSources("project.json", working)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(sources[0].Path, filepath.Join(b.store.dir, "imports")) {
		t.Fatal("source not rebound")
	}
	reread, e := os.ReadFile(sources[0].Path)
	if e != nil || replayDigest(reread) != sources[0].SHA256 {
		t.Fatal("source mismatch", e)
	}
	snapshot, e := b.loadCheckpoint(context.Background(), cp.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = b.portableCheckpoint(context.Background(), working, &snapshot); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(snapshot.Analysis, []byte(b.store.dir)) {
		t.Fatal("checkpoint not rebound")
	}
	if _, e = b.importArchive(context.Background(), got, false); e == nil {
		t.Fatal("overwrote existing project")
	}
	stored, e := b.loadImportedArchive(context.Background(), d.Record.Fingerprint)
	if e != nil || !bytes.Equal(stored.Files["project.json"], project) {
		t.Fatal(e)
	}
	// Original evidence still supports exact scientific replay after transfer.
	m, n, e := b.certificateNotice(id)
	if e != nil {
		t.Fatal(e)
	}
	f, _, e := b.makeCertificate(context.Background(), m, n)
	if e != nil {
		t.Fatal(e)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	st, _ := f.Stat()
	report := replayCertificate(context.Background(), f, st.Size(), "")
	if report.Verdict != "replay-match" {
		t.Fatal(report)
	}
}
func TestPortableArchiveSourcePolicyAndConflicts(t *testing.T) {
	a, project, id := archiveFixture(t)
	omitted, e := a.collectArchive(context.Background(), project, archiveOptions{IncludeSources: true})
	if e != nil {
		t.Fatal(e)
	}
	if omitted.Record.Sources[0].Status != "omitted" {
		t.Fatal("inferred source permission")
	}
	var doc map[string]json.RawMessage
	json.Unmarshal(project, &doc)
	var datasets []map[string]json.RawMessage
	json.Unmarshal(doc["datasets"], &datasets)
	datasets[0]["credits"] = mustJSON([]credit{{Title: "Provider control", LicenseClass: "terms", License: "Private study only"}})
	doc["datasets"] = mustJSON(datasets)
	if _, e = a.collectArchive(context.Background(), mustJSON(doc), archiveOptions{IncludeSources: true, OwnSources: true}); e == nil {
		t.Fatal("private evidence acknowledgment bypassed")
	}
	restricted, e := a.collectArchive(context.Background(), mustJSON(doc), archiveOptions{IncludeSources: true, OwnSources: true, Accept: true})
	if e != nil {
		t.Fatal(e)
	}
	if restricted.Record.Sources[0].Status != "restricted" || !restricted.Record.Restricted {
		t.Fatal("restricted source included")
	}
	b, _ := testApp(t)
	defer b.jobs.Close()
	if _, e = b.importArchive(context.Background(), archiveRoundTrip(t, restricted), false); e == nil {
		t.Fatal("import acknowledgment bypassed")
	}
	missing := archiveRoundTrip(t, omitted)
	bound, e := b.importArchive(context.Background(), missing, false)
	if e != nil {
		t.Fatal(e)
	}
	ss, _ := archiveSources("project.json", bound)
	if ss[0].Path != "" {
		t.Fatal("omitted source followed original path")
	}
	c, _ := testApp(t)
	defer c.jobs.Close()
	target := filepath.Join(c.artifacts.Dir(), id)
	os.MkdirAll(target, 0700)
	os.WriteFile(filepath.Join(target, "manifest.json"), []byte("conflict"), 0600)
	if _, e = c.importArchive(context.Background(), omitted, false); e == nil {
		t.Fatal("conflicting evidence overwritten")
	}
	if _, e = os.Stat(filepath.Join(c.store.dir, "projects", "p-archive.json")); !os.IsNotExist(e) {
		t.Fatal("partial project installed")
	}
	sources, _ := archiveSources("project.json", project)
	os.WriteFile(sources[0].Path, []byte("changed"), 0600)
	changed, e := a.collectArchive(context.Background(), project, archiveOptions{IncludeSources: true, OwnSources: true})
	if e != nil {
		t.Fatal(e)
	}
	if changed.Record.Sources[0].Status != "changed" {
		t.Fatal(changed.Record.Sources)
	}
	os.Remove(sources[0].Path)
	absent, e := a.collectArchive(context.Background(), project, archiveOptions{IncludeSources: true, OwnSources: true})
	if e != nil || absent.Record.Sources[0].Status != "missing" {
		t.Fatal(e, absent.Record.Sources)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = a.collectArchive(ctx, project, archiveOptions{}); e == nil {
		t.Fatal("ignored cancellation")
	}
}
func TestPortableArchiveRejectsHostileContainers(t *testing.T) {
	a, project, _ := archiveFixture(t)
	d, e := a.collectArchive(context.Background(), project, archiveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cases := []string{"../escape", "/absolute", "x\\y", "C:drive", "sources/CON", "sources/a.", "sources/a ", "sources/LPT1.txt", "sources/a\x00b", "Project.json", "link", "extra"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			z := zip.NewWriter(&buf)
			files := map[string][]byte{}
			for k, v := range d.Files {
				files[k] = v
			}
			files["archive.json"] = mustJSON(d.Record)
			files[name] = []byte("payload")
			for k, v := range files {
				h := &zip.FileHeader{Name: k, Method: zip.Deflate}
				if name == "link" && k == name {
					h.SetMode(os.ModeSymlink | 0777)
				}
				w, e := z.CreateHeader(h)
				if e != nil {
					t.Fatal(e)
				}
				w.Write(v)
			}
			z.Close()
			if _, e = readProjectArchive(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()), ""); e == nil {
				t.Fatal("accepted unsafe ZIP")
			}
		})
	}
	for _, mutation := range []string{"hash", "future", "project", "source", "duplicate", "checkpoint", "dependencies"} {
		t.Run(mutation, func(t *testing.T) {
			bad := archiveData{Record: d.Record, Files: map[string][]byte{}}
			for k, v := range d.Files {
				bad.Files[k] = append([]byte{}, v...)
			}
			bad.Record.Sources = append([]archiveSource{}, d.Record.Sources...)
			switch mutation {
			case "dependencies":
				bad.Record.Dependencies = make([]archiveDependency, 129)
				archiveInventory(&bad)
			case "hash":
				bad.Files["project.json"] = []byte("changed")
			case "future":
				bad.Record.Version = 99
				bad.Record.Fingerprint = archiveFingerprint(bad.Record)
			case "project":
				bad.Files["project.json"] = bytes.Replace(project, []byte(`"version":1`), []byte(`"version":99`), 1)
				archiveInventory(&bad)
			case "source":
				bad.Record.Sources[0].Pointer = "/history/0"
				archiveInventory(&bad)
			case "duplicate":
				bad.Files["project.json"] = []byte(`{"format":"clonoloom-project","version":1,"id":"p-archive","id":"evil"}`)
				archiveInventory(&bad)
			case "checkpoint":
				bad.Files["project.json"] = bytes.Replace(project, []byte(`"artifacts":`), []byte(`"checkpoints":[{"id":"cp-absent"}],"artifacts":`), 1)
				archiveInventory(&bad)
			}
			var buf bytes.Buffer
			writeProjectArchive(context.Background(), &buf, bad)
			if _, e = readProjectArchive(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()), ""); e == nil {
				t.Fatal("accepted invalid archive")
			}
		})
	}
	var buf bytes.Buffer
	writeProjectArchive(context.Background(), &buf, d)
	if _, e = readProjectArchive(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()), strings.Repeat("0", 64)); e == nil {
		t.Fatal("ignored pinned fingerprint")
	}
	if _, e = readProjectArchive(context.Background(), bytes.NewReader(buf.Bytes()[:buf.Len()-5]), int64(buf.Len()-5), ""); e == nil {
		t.Fatal("accepted truncated ZIP")
	}
}
func TestPortableArchivesReopenReleaseProjects(t *testing.T) {
	paths, _ := filepath.Glob("validation/projects/*/project.json")
	for _, p := range paths {
		t.Run(filepath.Base(filepath.Dir(p)), func(t *testing.T) {
			a, _ := testApp(t)
			defer a.jobs.Close()
			dir := filepath.Dir(p)
			project, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			ids, _ := archiveRefs(project, "artifacts")
			for _, id := range ids {
				folder := filepath.Join(dir, "artifacts", id)
				files, e := os.ReadDir(folder)
				if e != nil {
					t.Fatal(e)
				}
				out := filepath.Join(a.artifacts.Dir(), id)
				os.MkdirAll(out, 0700)
				for _, f := range files {
					name := f.Name()
					if f.IsDir() {
						continue
					}
					var b []byte
					if strings.HasSuffix(name, ".gz") {
						b = gunzipFile(t, filepath.Join(folder, name))
						name = strings.TrimSuffix(name, ".gz")
					} else {
						b, e = os.ReadFile(filepath.Join(folder, name))
						if e != nil {
							t.Fatal(e)
						}
					}
					os.WriteFile(filepath.Join(out, name), b, 0600)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "checkpoints")); err == nil {
				if err := os.CopyFS(filepath.Join(a.store.dir, "checkpoints"), os.DirFS(filepath.Join(dir, "checkpoints"))); err != nil {
					t.Fatal(err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			// Rebind fixture-relative source paths as a real saved project on this host;
			// retain the original release project separately for exact comparison below.
			sources, e := archiveSources("project.json", project)
			if e != nil {
				t.Fatal(e)
			}
			fakeRecord := archiveRecord{}
			for _, s := range sources {
				s.Status = "included"
				s.Entry = s.Path
				fakeRecord.Sources = append(fakeRecord.Sources, s)
			}
			absolute, _ := filepath.Abs(dir)
			working, e := bindArchiveProject(project, "project.json", absolute, fakeRecord)
			if e != nil {
				t.Fatal(e)
			}
			d, e := a.collectArchive(context.Background(), working, archiveOptions{IncludeSources: true, OwnSources: true, Accept: true})
			if e != nil {
				t.Fatal(e)
			}
			b, _ := testApp(t)
			defer b.jobs.Close()
			imported, e := b.importArchive(context.Background(), archiveRoundTrip(t, d), true)
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(d.Files["project.json"], working) {
				t.Fatal("saved project rewritten")
			}
			var fp fixtureProject
			if e = json.Unmarshal(imported, &fp); e != nil {
				t.Fatal(e)
			}
			for _, ds := range fp.Datasets {
				var inputs []reads.Input
				// Preserve original reader names as well as mapping and IMGT grouping.
				var doc struct {
					Datasets []struct {
						ID       string        `json:"id"`
						Sources  []reads.Input `json:"sources"`
						Metadata []reads.Input `json:"metadata"`
					} `json:"datasets"`
				}
				json.Unmarshal(imported, &doc)
				for _, v := range doc.Datasets {
					if v.ID == ds.ID {
						inputs = append(v.Sources, v.Metadata...)
					}
				}
				got, e := b.openDataset(context.Background(), nil, inputs, ds.Name)
				if e != nil {
					t.Fatal(e)
				}
				if got.ID != ds.ID {
					t.Fatalf("release dataset identity changed: %s != %s", got.ID, ds.ID)
				}
			}
			var expressionDoc struct {
				Expression []struct {
					ID        string                       `json:"id"`
					Path      string                       `json:"path"`
					CellsPath string                       `json:"cellsPath"`
					AnnData   *expression.AnnDataSelection `json:"anndata"`
				} `json:"expression"`
			}
			if e := json.Unmarshal(imported, &expressionDoc); e != nil {
				t.Fatal(e)
			}
			for _, x := range expressionDoc.Expression {
				in, e := expression.OpenSelected(context.Background(), x.Path, x.CellsPath, x.AnnData)
				if e != nil {
					t.Fatal(e)
				}
				if in.ID != x.ID {
					t.Fatal("expression source identity changed", in.ID, x.ID)
				}
			}
			for _, id := range ids {
				manifest, e := os.ReadFile(filepath.Join(b.artifacts.Dir(), id, "manifest.json"))
				if e != nil {
					t.Fatal(e)
				}
				var m artifacts.Manifest
				json.Unmarshal(manifest, &m)
				for _, f := range m.Files {
					got, e := os.ReadFile(filepath.Join(b.artifacts.Dir(), id, f.Path))
					if e != nil || replayDigest(got) != f.SHA256 {
						t.Fatal("release evidence changed", e)
					}
				}
			}
		})
	}
}
func FuzzPortableArchiveReader(f *testing.F) {
	d := archiveData{Record: archiveRecord{Format: "clonoloom-project-archive", Version: 1, Project: "p-fuzz", Limits: archiveLimits}, Files: map[string][]byte{"project.json": []byte(`{"format":"clonoloom-project","version":1,"id":"p-fuzz"}`), "NOTICE.txt": []byte(archiveLimits)}}
	archiveInventory(&d)
	var buf bytes.Buffer
	if err := writeProjectArchive(context.Background(), &buf, d); err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte("not an archive"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		readProjectArchive(context.Background(), bytes.NewReader(raw), int64(len(raw)), "")
	})
}

func TestPortableArchiveReexportAndNewCheckpoint(t *testing.T) {
	a, raw, _ := archiveFixture(t)
	cp, err := a.createCheckpoint(context.Background(), raw, "Original")
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]json.RawMessage
	json.Unmarshal(raw, &p)
	p["checkpoints"] = mustJSON([]any{map[string]any{"id": cp.ID}})
	raw = mustJSON(p)
	d, err := a.collectArchive(context.Background(), raw, archiveOptions{IncludeSources: true, OwnSources: true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := testApp(t)
	defer b.jobs.Close()
	working, err := b.importArchive(context.Background(), d, false)
	if err != nil {
		t.Fatal(err)
	}
	sources, _ := archiveSources("project.json", raw)
	if err = os.Remove(sources[0].Path); err != nil {
		t.Fatal(err)
	}
	again, err := b.collectArchive(context.Background(), working, archiveOptions{IncludeSources: true, OwnSources: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range again.Record.Sources {
		if s.Status != "included" {
			t.Fatal("re-export lost verified source", s)
		}
	}
	fresh, err := b.createCheckpoint(context.Background(), working, "Local after import")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := b.loadCheckpoint(context.Background(), fresh.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := string(snapshot.Analysis)
	if err = b.portableCheckpoint(context.Background(), working, &snapshot); err != nil || string(snapshot.Analysis) != before {
		t.Fatal("local checkpoint changed", err)
	}
	c, _ := testApp(t)
	defer c.jobs.Close()
	os.MkdirAll(filepath.Join(c.artifacts.Dir(), d.Record.Dependencies[0].ID), 0700)
	if _, err = c.importArchive(context.Background(), d, false); err == nil {
		t.Fatal("repaired incomplete existing result")
	}
}

func TestPortableArchiveHTTP(t *testing.T) {
	a, raw, _ := archiveFixture(t)
	h, err := a.handler()
	if err != nil {
		t.Fatal(err)
	}
	body := mustJSON(map[string]any{"project": json.RawMessage(raw), "includeSources": true, "ownSources": true})
	preview := request(t, h, "POST", "/api/project-archives/export?preview=1", bytes.NewReader(body), nil)
	if preview.Code != 200 {
		t.Fatal(preview.Body.String())
	}
	var record archiveRecord
	json.Unmarshal(preview.Body.Bytes(), &record)
	exported := request(t, h, "POST", "/api/project-archives/export", bytes.NewReader(body), nil)
	if exported.Code != 200 {
		t.Fatal(exported.Body.String())
	}
	b, bh := testApp(t)
	defer b.jobs.Close()
	inspected := request(t, bh, "POST", "/api/project-archives/inspect", bytes.NewReader(exported.Body.Bytes()), nil)
	if inspected.Code != 200 {
		t.Fatal(inspected.Body.String())
	}
	missing := request(t, bh, "POST", "/api/project-archives/import", bytes.NewReader(exported.Body.Bytes()), nil)
	if missing.Code != 400 {
		t.Fatal("import without fingerprint", missing.Code)
	}
	imported := request(t, bh, "POST", "/api/project-archives/import?expect="+record.Fingerprint, bytes.NewReader(exported.Body.Bytes()), nil)
	if imported.Code != 200 {
		t.Fatal(imported.Body.String())
	}
	original := request(t, bh, "GET", "/api/project-archives/"+record.Fingerprint+"/original", nil, nil)
	if original.Code != 200 || !bytes.Equal(original.Body.Bytes(), raw) {
		t.Fatal("changed original project")
	}
	// Local inspection never follows a symlink and never imports during inspection.
	path := filepath.Join(t.TempDir(), "archive.clz")
	os.WriteFile(path, exported.Body.Bytes(), 0600)
	local := request(t, bh, "POST", "/api/project-archives/inspect", bytes.NewReader(mustJSON(map[string]string{"path": path})), map[string]string{"Content-Type": "application/json"})
	if local.Code != 200 {
		t.Fatal(local.Body.String())
	}
	link := path + ".link"
	if err := os.Symlink(path, link); err == nil {
		local = request(t, bh, "POST", "/api/project-archives/inspect", bytes.NewReader(mustJSON(map[string]string{"path": link})), map[string]string{"Content-Type": "application/json"})
		if local.Code != 400 {
			t.Fatal("followed local archive link")
		}
	}
}

func TestPortableArchiveBoundsAndPortableNames(t *testing.T) {
	sources := make([]map[string]string, 1024)
	for i := range sources {
		sources[i] = map[string]string{"path": "source.tsv"}
	}
	raw := mustJSON(map[string]any{"datasets": []any{map[string]any{"sources": sources, "credits": []credit{{Title: strings.Repeat("x", 8192), LicenseClass: "open"}}}}})
	if _, e := archiveSources("project.json", raw); e == nil {
		t.Fatal("unbounded duplicated source metadata")
	}

	for _, name := range []string{"café.tsv", "café.tsv", "COM¹.txt", "a/../b", "a/" + strings.Repeat("b", 241), strings.Repeat("x/", 513) + "y"} {
		if archivePath(name) {
			t.Fatal("accepted nonportable path", name)
		}
	}
	if !archivePath("sources/abc/S01-day14.tsv") {
		t.Fatal("ordinary portable name refused")
	}
	for _, mode := range []string{"compressed", "file", "total", "count", "prefix", "encrypted", "method"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "compressed" {
				if _, e := readProjectArchive(context.Background(), bytes.NewReader(nil), (520<<20)+1, ""); e == nil {
					t.Fatal("compressed bound")
				}
				return
			}
			var buf bytes.Buffer
			z := zip.NewWriter(&buf)
			count := 3
			if mode == "count" {
				count = archiveMaxFiles + 2
			}
			for i := 0; i < count; i++ {
				h := &zip.FileHeader{Name: fmt.Sprintf("file%d", i), Method: zip.Store}
				switch mode {
				case "file":
					h.UncompressedSize64 = (256 << 20) + 1
				case "total":
					h.UncompressedSize64 = 256 << 20
				case "prefix":
					if i == 1 {
						h.Name = "file0/child"
					}
				case "encrypted":
					h.Flags = 1
				case "method":
					h.Method = 99
				}
				w, e := z.CreateRaw(h)
				if e != nil {
					t.Fatal(e)
				}
				w.Write(nil)
			}
			z.Close()
			if _, e := readProjectArchive(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()), ""); e == nil {
				t.Fatal("accepted bounded-input violation")
			}
		})
	}
}

package main

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
	"sync"
	"time"

	"clonoloom/internal/certificate"
)

var archiveImportMu sync.Mutex

func archiveMkdir(root *os.Root, name string) error {
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i := 1; i <= len(parts); i++ {
		p := filepath.FromSlash(strings.Join(parts[:i], "/"))
		if e := root.Mkdir(p, 0700); e != nil && !os.IsExist(e) {
			return e
		}
		st, e := root.Lstat(p)
		if e != nil {
			return e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("archive destination includes a link or non-directory")
		}
	}
	return nil
}
func archivePreflight(root *os.Root, name string, want []byte, project bool) error {
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		st, e := root.Lstat(filepath.FromSlash(strings.Join(parts[:i], "/")))
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("archive destination parent is not a local directory")
		}
	}
	st, e := root.Lstat(filepath.FromSlash(name))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if project {
		return errors.New("a project with this identity already exists; import into a different library or move the existing project to trash first")
	}
	if !st.Mode().IsRegular() {
		return errors.New("archive destination is not a regular file")
	}
	got, e := checkpointRead(context.Background(), root, filepath.FromSlash(name), int64(len(want)))
	if e != nil || !bytes.Equal(got, want) {
		return fmt.Errorf("existing evidence conflicts: %s; nothing was overwritten", name)
	}
	return nil
}
func (a *app) importArchive(ctx context.Context, d archiveData, accept bool) ([]byte, error) {
	if err := validateArchive(d); err != nil {
		return nil, err
	}
	if d.Record.Restricted && !accept {
		return nil, errors.New("restricted evidence requires explicit private-use acknowledgment")
	}
	archiveImportMu.Lock()
	defer archiveImportMu.Unlock()
	root, e := os.OpenRoot(a.store.dir)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	rel := "imports/" + d.Record.Fingerprint
	project, e := bindArchiveProject(d.Files["project.json"], "project.json", filepath.Join(a.store.dir, filepath.FromSlash(rel)), d.Record)
	if e != nil {
		return nil, e
	}
	files := map[string][]byte{}
	for name, b := range d.Files {
		files[rel+"/"+name] = b
		if strings.HasPrefix(name, "artifacts/") || strings.HasPrefix(name, "checkpoints/") {
			files[name] = b
		}
	}
	files[rel+"/archive.json"] = mustJSON(d.Record)
	projectName := "projects/" + d.Record.Project + ".json"
	if e = archivePreflight(root, projectName, project, true); e != nil {
		return nil, e
	}
	// Existing immutable result/checkpoint directories must be complete, never repaired
	// piecemeal by an import. An interrupted bundle may retain verified unused files.
	for name := range files {
		parts := strings.Split(name, "/")
		if parts[0] != "artifacts" && parts[0] != "checkpoints" {
			continue
		}
		folder := strings.Join(parts[:2], "/")
		if _, err := root.Lstat(filepath.FromSlash(folder)); err == nil {
			if _, err = root.Lstat(filepath.FromSlash(name)); err != nil {
				return nil, fmt.Errorf("existing evidence is incomplete: %s", folder)
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	names := []string{}
	for name, b := range files {
		if e = archivePreflight(root, name, b, false); e != nil {
			return nil, e
		}
		names = append(names, name)
	}
	sort.Strings(names)
	// Stage all bytes before installing any destination. Ordinary cancellation removes
	// the stage. Verified unreferenced immutable files may remain after interruption.
	stage, e := os.MkdirTemp(a.store.dir, ".archive-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(stage)
	staged := map[string][]byte{}
	for name, b := range files {
		staged[name] = b
	}
	staged[projectName] = project
	if e = checkpointWrite(ctx, stage, staged); e != nil {
		return nil, e
	}
	for _, name := range names {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		if e = archivePreflight(root, name, files[name], false); e != nil {
			return nil, e
		}
		if _, e = root.Lstat(filepath.FromSlash(name)); e == nil {
			continue
		}
		if e = archiveMkdir(root, filepath.Dir(filepath.FromSlash(name))); e != nil {
			return nil, e
		}
		f, e := root.OpenFile(filepath.FromSlash(name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		_, e = f.Write(files[name])
		if e == nil {
			e = f.Sync()
		}
		closed := f.Close()
		if e == nil {
			e = closed
		}
		if e != nil {
			root.Remove(filepath.FromSlash(name))
			return nil, e
		}
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if e = archivePreflight(root, projectName, project, true); e != nil {
		return nil, e
	}
	// A same-filesystem hard link publishes a complete staged file atomically and
	// refuses an existing destination (unlike rename). No archive-supplied link exists.
	if e = os.Link(filepath.Join(stage, filepath.FromSlash(projectName)), filepath.Join(a.store.dir, filepath.FromSlash(projectName))); e != nil {
		return nil, e
	}
	return project, nil
}
func (a *app) loadImportedArchive(ctx context.Context, fp string) (archiveData, error) {
	d := archiveData{Files: map[string][]byte{}}
	if !runDigest(fp) {
		return d, errors.New("invalid imported archive fingerprint")
	}
	dir := filepath.Join(a.store.dir, "imports", fp)
	raw, e := archiveRead(ctx, dir, "archive.json", archiveRecordLimit)
	if e != nil {
		return d, e
	}
	if e = strictRunJSON(raw, &d.Record); e != nil {
		return d, e
	}
	if d.Record.Fingerprint != fp || archiveFingerprint(d.Record) != fp || !bytes.Equal(raw, mustJSON(d.Record)) {
		return d, errors.New("imported archive inventory changed")
	}
	var total int64
	if len(d.Record.Files) > archiveMaxFiles {
		return d, errors.New("too many imported archive files")
	}
	for _, f := range d.Record.Files {
		if f.Size < 0 || f.Size > certificate.MaxFileBytes {
			return d, errors.New("oversized imported file")
		}
		total += f.Size
		if total > certificate.MaxBytes {
			return d, errors.New("imported evidence exceeds bound")
		}
		b, e := archiveRead(ctx, dir, f.Path, f.Size)
		if e != nil {
			return d, e
		}
		if int64(len(b)) != f.Size || replayDigest(b) != f.SHA256 {
			return d, errors.New("imported evidence changed")
		}
		d.Files[f.Path] = b
	}
	return d, validateArchive(d)
}
func (a *app) portableCheckpoint(ctx context.Context, current []byte, d *checkpointData) error {
	var doc map[string]json.RawMessage
	if e := json.Unmarshal(current, &doc); e != nil {
		return e
	}
	var fp string
	if b := doc["portableArchive"]; b != nil {
		if e := json.Unmarshal(b, &fp); e != nil {
			return e
		}
	}
	if fp == "" {
		return nil
	}
	imported, e := a.loadImportedArchive(ctx, fp)
	if e != nil {
		return e
	}
	name := "checkpoints/" + d.Record.ID + "/project.json"
	if b, ok := imported.Files[name]; !ok || !bytes.Equal(b, d.Files["project.json"]) {
		// A checkpoint made locally after import already records the verified local paths.
		var saved struct {
			PortableArchive string `json:"portableArchive"`
		}
		if json.Unmarshal(d.Files["project.json"], &saved) == nil && saved.PortableArchive == fp {
			return nil
		}
		return errors.New("checkpoint has no exact portable binding in this imported archive")
	}
	bound, e := bindArchiveProject(d.Files["project.json"], name, filepath.Join(a.store.dir, "imports", fp), imported.Record)
	if e != nil {
		return e
	}
	_, d.Analysis, _, e = checkpointProject(bound)
	return e
}
func (a *app) registerProjectArchives(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/project-archives/export", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Project json.RawMessage `json:"project"`
			archiveOptions
			Expected string `json:"expected,omitempty"`
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, checkpointProjectLimit+4096))
		if e == nil {
			e = strictRunJSON(raw, &body)
		}
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		d, e := a.collectArchive(r.Context(), body.Project, body.archiveOptions)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		if body.Expected != "" && body.Expected != d.Record.Fingerprint {
			writeError(w, 409, "Project or evidence changed since archive review.")
			return
		}
		if r.URL.Query().Get("preview") == "1" {
			writeJSON(w, d.Record)
			return
		}
		file, e := os.CreateTemp("", "clonoloom-project-*.clz")
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		defer func() { file.Close(); os.Remove(file.Name()) }()
		if e = writeProjectArchive(r.Context(), file, d); e != nil {
			writeError(w, 409, e.Error())
			return
		}
		file.Seek(0, io.SeekStart)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+d.Record.Project+`.clz"`)
		w.Header().Set("X-ClonoLoom-Archive", d.Record.Fingerprint)
		http.ServeContent(w, r, d.Record.Project+".clz", time.Time{}, file)
	})
	for _, operation := range []string{"inspect", "import"} {
		mux.HandleFunc("POST /api/project-archives/"+operation, func(w http.ResponseWriter, r *http.Request) {
			expected := r.URL.Query().Get("expect")
			accept := r.URL.Query().Get("accept") == "1"
			var file *os.File
			var size int64
			var e error
			if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				var request struct {
					Path     string `json:"path"`
					Expected string `json:"expected,omitempty"`
					Accept   bool   `json:"accept"`
				}
				raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
				if err == nil {
					err = strictRunJSON(raw, &request)
				}
				if err != nil || !filepath.IsAbs(request.Path) {
					writeError(w, 400, "Choose an absolute archive file path.")
					return
				}
				expected = request.Expected
				accept = request.Accept
				st, err := os.Lstat(request.Path)
				if err != nil || !st.Mode().IsRegular() {
					writeError(w, 400, "Archive must be an existing regular file, not a link or device.")
					return
				}
				file, e = os.Open(request.Path)
				if e == nil {
					var st os.FileInfo
					st, e = file.Stat()
					if e == nil {
						if !st.Mode().IsRegular() {
							e = errors.New("archive must be a regular file")
						} else {
							size = st.Size()
						}
					}
				}
			} else {
				file, e = os.CreateTemp("", "clonoloom-project-upload-*.clz")
				if e == nil {
					defer os.Remove(file.Name())
					size, e = io.Copy(file, http.MaxBytesReader(w, r.Body, certificate.MaxBytes+(8<<20)))
				}
			}
			if file != nil {
				defer file.Close()
			}
			if e != nil {
				writeError(w, 400, e.Error())
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
			defer cancel()
			d, e := readProjectArchive(ctx, file, size, expected)
			if e != nil {
				writeError(w, 409, e.Error())
				return
			}
			if operation == "inspect" {
				writeJSON(w, map[string]any{"archive": d.Record, "recomputed": false, "projectJSON": string(d.Files["project.json"])})
				return
			}
			if expected == "" {
				writeError(w, 400, "Inspect first and supply its fingerprint before importing.")
				return
			}
			project, e := a.importArchive(ctx, d, accept)
			if e != nil {
				writeError(w, 409, e.Error())
				return
			}
			writeJSON(w, map[string]any{"archive": d.Record, "projectJSON": string(project), "recomputed": false})
		})
	}
	mux.HandleFunc("GET /api/project-archives/{fingerprint}/original", func(w http.ResponseWriter, r *http.Request) {
		d, e := a.loadImportedArchive(r.Context(), r.PathValue("fingerprint"))
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="original-project.json"`)
		w.Write(d.Files["project.json"])
	})
}

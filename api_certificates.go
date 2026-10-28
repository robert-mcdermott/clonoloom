package main

// Evidence certificates preserve exact saved bytes and captured provider notices.
// certificateSupportMatrix bounds the mapped source/notice/replay contracts.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
	"clonoloom/internal/export"
)

func (a *app) certificateNotice(id string) (artifacts.Manifest, export.Notice, error) {
	// Path validates the artifact ID; manifests themselves are intentionally not
	// exposed by Store.Path. Open the validated directory through os.Root instead.
	path, err := a.artifacts.Path(id, "result.json")
	var m artifacts.Manifest
	if err != nil {
		return m, export.Notice{}, err
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return m, export.Notice{}, err
	}
	defer root.Close()
	read := func(name string, limit int64) ([]byte, error) {
		f, e := root.Open(name)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, limit+1))
		if e == nil && int64(len(b)) > limit {
			e = errors.New("saved evidence exceeds certificate size limit")
		}
		return b, e
	}
	raw, err := read("manifest.json", certificate.MaxManifestBytes)
	if err == nil {
		m, err = certificate.Manifest(raw)
	}
	if err != nil {
		return m, export.Notice{}, err
	}
	if m.ID != id {
		return m, export.Notice{}, errors.New("artifact identity differs from its directory")
	}
	var notice export.Notice
	if _, ok := certificateSupportFor(m.Kind); !ok {
		return m, notice, errors.New("this result kind has no supported certificate contract")
	}
	raw, err = read(m.Kind+".json", certificate.MaxFileBytes)
	if err != nil {
		return m, notice, err
	}
	h := sha256.Sum256(raw)
	found := false
	for _, f := range m.Files {
		if f.Path == m.Kind+".json" && f.Size == int64(len(raw)) && f.SHA256 == hex.EncodeToString(h[:]) {
			found = true
		}
	}
	if !found {
		return m, notice, errors.New("saved result failed checksum verification")
	}
	switch m.Kind {
	case cohortFeaturesKind:
		var doc cohortFeaturesDocument
		err = json.Unmarshal(raw, &doc)
		notice = doc.Notice
	case cohortCompareKind:
		var doc cohortCompareDocument
		err = json.Unmarshal(raw, &doc)
		notice = doc.Features.Notice
	case cohortCountsKind:
		var doc cohortCountsDocument
		err = json.Unmarshal(raw, &doc)
		notice = doc.Features.Notice
	case batchqcKind:
		var doc batchqcDocument
		err = json.Unmarshal(raw, &doc)
		notice = doc.Evidence.Notice
	case motifsKind:
		var doc motifsDocument
		err = json.Unmarshal(raw, &doc)
		notice = doc.Evidence.Notice
	default:
		return m, notice, errors.New("this result kind has no supported certificate contract")
	}
	if err != nil {
		return m, notice, err
	}
	if notice.DatasetID == "" || len(notice.Files) == 0 {
		return m, notice, errors.New("saved result has no complete source notice")
	}
	// The manifest recipe may contain raw counts and identifiers, so this is not
	// merely a derived summary; carry the same provider gate as data exports.
	notice.What = "Saved-result evidence certificate. " + certificate.Limits
	notice.Derived = false
	notice.Method = m.Lineage.Method.Name + "; " + m.Lineage.Method.Citation
	notice.Made = m.Created
	for _, s := range m.Lineage.Software {
		if s.Name == "clonoloom" {
			notice.Version, notice.Commit = s.Version, s.Commit
		}
	}
	return m, notice, nil
}

func (a *app) makeCertificate(ctx context.Context, m artifacts.Manifest, n export.Notice) (*os.File, certificate.Record, error) {
	root, err := os.OpenRoot(filepath.Join(a.artifacts.Dir(), m.ID))
	if err != nil {
		return nil, certificate.Record{}, err
	}
	defer root.Close()
	f, err := root.Open("manifest.json")
	if err != nil {
		return nil, certificate.Record{}, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, certificate.MaxManifestBytes+1))
	f.Close()
	if err != nil {
		return nil, certificate.Record{}, err
	}
	current, err := certificate.Manifest(raw)
	if err != nil {
		return nil, certificate.Record{}, err
	}
	if !bytes.Equal(mustJSON(current), mustJSON(m)) {
		return nil, certificate.Record{}, errors.New("saved manifest changed during certificate preparation")
	}
	out, err := os.CreateTemp("", "clonoloom-certificate-*.clz")
	if err != nil {
		return nil, certificate.Record{}, err
	}
	r, err := certificate.Build(ctx, out, raw, []byte(n.Text()), artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}, func(path string) (io.ReadCloser, error) { return root.Open(path) })
	if err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, r, err
	}
	if _, err = out.Seek(0, io.SeekStart); err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, r, err
	}
	return out, r, nil
}

func (a *app) registerCertificates(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/certificates/support", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.workflowSupport()) })
	serve := func(w http.ResponseWriter, r *http.Request) {
		m, n, err := a.certificateNotice(r.PathValue("id"))
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		if n.Restricted() && r.URL.Query().Get("accept") != "1" {
			restricted(w, n)
			return
		}
		file, record, err := a.makeCertificate(r.Context(), m, n)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		defer func() { file.Close(); os.Remove(file.Name()) }()
		if r.PathValue("download") == "" {
			writeJSON(w, map[string]any{"certificate": record, "limits": certificate.Limits})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+m.ID+`.certificate.clz"`)
		w.Header().Set("X-ClonoLoom-Certificate", record.Fingerprint)
		http.ServeContent(w, r, m.ID+".certificate.clz", time.Time{}, file)
	}
	mux.HandleFunc("GET /api/certificates/{id}", serve)
	mux.HandleFunc("GET /api/certificates/{id}/{download}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("download") != "download" {
			http.NotFound(w, r)
			return
		}
		serve(w, r)
	})
	mux.HandleFunc("POST /api/certificates/verify", func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		if mode != "" && mode != "integrity" && mode != "replay" {
			writeError(w, 400, "Choose integrity or replay.")
			return
		}
		file, err := os.CreateTemp("", "clonoloom-verify-*.clz")
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer func() { file.Close(); os.Remove(file.Name()) }()
		n, err := io.Copy(file, http.MaxBytesReader(w, r.Body, certificate.MaxBytes+(8<<20)))
		if err != nil {
			writeError(w, 413, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		if mode == "replay" {
			writeJSON(w, replayCertificate(ctx, file, n, r.URL.Query().Get("expect")))
			return
		}
		writeJSON(w, certificate.Verify(ctx, file, n, r.URL.Query().Get("expect")))
	})
}

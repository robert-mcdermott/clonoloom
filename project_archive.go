package main

// Portable archives retain original evidence bytes. Only the imported working
// project's documented source paths are rebound; no saved recipe is executed.
import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
)

const archiveMaxFiles = 2048
const archiveRecordLimit = 4 << 20
const archiveLimits = "Portable recorded evidence, not scientific recomputation or de-identification. Original project/history and result bytes remain unchanged. Only documented source paths in the working project are rebound. Missing, omitted, restricted or unsupported sources cannot be reopened until explicitly located and fingerprint-checked. References, caches, running jobs and external outputs are not bundled. Checksum integrity is not authenticity, redistribution permission or biological validity."

type archiveSource struct {
	Document string   `json:"document"`
	Pointer  string   `json:"pointer"`
	Path     string   `json:"originalPath"`
	Name     string   `json:"name"`
	SHA256   string   `json:"sha256"`
	Size     int64    `json:"size"`
	Entry    string   `json:"entry,omitempty"`
	Status   string   `json:"status"`
	Reason   string   `json:"reason"`
	Credits  []credit `json:"credits"`
}
type archiveDependency struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}
type archiveRecord struct {
	Format       string              `json:"format"`
	Version      int                 `json:"version"`
	Project      string              `json:"project"`
	Software     artifacts.Software  `json:"software"`
	Files        []runOutput         `json:"files"`
	Sources      []archiveSource     `json:"sources"`
	Dependencies []archiveDependency `json:"dependencies"`
	Restricted   bool                `json:"restricted"`
	Limits       string              `json:"limits"`
	Fingerprint  string              `json:"fingerprint,omitempty"`
}
type archiveData struct {
	Record archiveRecord
	Files  map[string][]byte
}
type archiveOptions struct {
	IncludeSources bool `json:"includeSources"`
	OwnSources     bool `json:"ownSources"`
	Accept         bool `json:"accept"`
}

func archivePath(p string) bool {
	if len(p) > 1024 || !utf8.ValidString(p) || !fs.ValidPath(p) || p == "." || strings.ContainsAny(p, "\\:<>\"|?*") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if len(part) > 240 || strings.TrimRight(part, ". ") != part || strings.IndexFunc(part, func(r rune) bool { return r < 32 || r >= 127 }) >= 0 {
			return false
		}
		base := strings.NewReplacer("¹", "1", "²", "2", "³", "3").Replace(strings.ToUpper(strings.SplitN(part, ".", 2)[0]))
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}
func archiveFingerprint(r archiveRecord) string { r.Fingerprint = ""; return replayDigest(mustJSON(r)) }
func (d *archiveData) add(name string, raw []byte) error {
	if !archivePath(name) || int64(len(raw)) > certificate.MaxFileBytes {
		return errors.New("unsafe or oversized archive member")
	}
	if old, ok := d.Files[name]; ok {
		if !bytes.Equal(old, raw) {
			return fmt.Errorf("conflicting evidence for %s", name)
		}
		return nil
	}
	var total int64 = int64(len(raw))
	for key, data := range d.Files {
		if strings.EqualFold(key, name) || strings.HasPrefix(strings.ToLower(key), strings.ToLower(name)+"/") || strings.HasPrefix(strings.ToLower(name), strings.ToLower(key)+"/") {
			return errors.New("case-colliding archive members")
		}
		total += int64(len(data))
	}
	if total > certificate.MaxBytes || len(d.Files) >= archiveMaxFiles {
		return errors.New("archive exceeds 512 MiB or 2048 files")
	}
	d.Files[name] = raw
	return nil
}
func archiveInventory(d *archiveData) {
	d.Record.Files = nil
	for name, raw := range d.Files {
		d.Record.Files = append(d.Record.Files, runOutput{name, int64(len(raw)), replayDigest(raw)})
	}
	sort.Slice(d.Record.Files, func(i, j int) bool { return d.Record.Files[i].Path < d.Record.Files[j].Path })
	d.Record.Fingerprint = archiveFingerprint(d.Record)
}
func writeProjectArchive(ctx context.Context, w io.Writer, d archiveData) error {
	if len(mustJSON(d.Record)) > archiveRecordLimit {
		return errors.New("archive record exceeds 4 MiB")
	}
	z := zip.NewWriter(w)
	for _, f := range append(append([]runOutput{}, d.Record.Files...), runOutput{Path: "archive.json"}) {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw := d.Files[f.Path]
		if f.Path == "archive.json" {
			raw = mustJSON(d.Record)
		}
		out, err := z.CreateHeader(&zip.FileHeader{Name: f.Path, Method: zip.Deflate})
		if err != nil {
			return err
		}
		if _, err = io.Copy(out, runContextReader{ctx, bytes.NewReader(raw)}); err != nil {
			return err
		}
	}
	return z.Close()
}
func readProjectArchive(ctx context.Context, in io.ReaderAt, size int64, expect string) (archiveData, error) {
	d := archiveData{Files: map[string][]byte{}}
	if size < 0 || size > certificate.MaxBytes+(8<<20) {
		return d, errors.New("compressed archive exceeds 520 MiB")
	}
	if expect != "" && !runDigest(expect) {
		return d, errors.New("expected fingerprint must be lowercase SHA-256")
	}
	z, err := zip.NewReader(in, size)
	if err != nil {
		return d, err
	}
	if len(z.File) < 3 || len(z.File) > archiveMaxFiles+1 {
		return d, errors.New("archive member count outside bounds")
	}
	entries := map[string]*zip.File{}
	fold := map[string]bool{}
	var total uint64
	for _, f := range z.File {
		key := strings.ToLower(f.Name)
		if !archivePath(f.Name) || fold[key] || !f.Mode().IsRegular() || f.Flags&1 != 0 || (f.Method != zip.Store && f.Method != zip.Deflate) || f.UncompressedSize64 > uint64(certificate.MaxFileBytes) {
			return d, fmt.Errorf("unsafe, duplicate or oversized ZIP member %q", f.Name)
		}
		total += f.UncompressedSize64
		if total > uint64(certificate.MaxBytes+archiveRecordLimit) {
			return d, errors.New("expanded archive exceeds bound")
		}
		fold[key] = true
		entries[f.Name] = f
	}
	for key := range fold {
		for parent := path.Dir(key); parent != "."; parent = path.Dir(parent) {
			if fold[parent] {
				return d, errors.New("archive file shadows a directory")
			}
		}
	}
	read := func(name string, limit int64) ([]byte, error) {
		f := entries[name]
		if f == nil || f.UncompressedSize64 > uint64(limit) {
			return nil, fmt.Errorf("missing or oversized %s", name)
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		b, e := io.ReadAll(runContextReader{ctx, io.LimitReader(r, limit+1)})
		if int64(len(b)) > limit {
			return nil, errors.New("expanded member exceeds bound")
		}
		return b, e
	}
	raw, err := read("archive.json", archiveRecordLimit)
	if err != nil {
		return d, err
	}
	if err = strictRunJSON(raw, &d.Record); err != nil {
		return d, err
	}
	r := d.Record
	if !bytes.Equal(raw, mustJSON(r)) || r.Format != "clonoloom-project-archive" || r.Version != 1 || r.Fingerprint != archiveFingerprint(r) || !runDigest(r.Fingerprint) {
		return d, errors.New("unsupported archive or record fingerprint mismatch")
	}
	if expect != "" && expect != r.Fingerprint {
		return d, errors.New("archive differs from expected fingerprint")
	}
	if len(r.Files)+1 != len(entries) {
		return d, errors.New("unlisted or missing archive members")
	}
	last := ""
	var bytesRead int64
	for _, f := range r.Files {
		if f.Path <= last || f.Path == "archive.json" || f.Size < 0 || f.Size > certificate.MaxFileBytes || !runDigest(f.SHA256) {
			return d, errors.New("invalid inventory")
		}
		last = f.Path
		if entries[f.Path] == nil || entries[f.Path].UncompressedSize64 != uint64(f.Size) {
			return d, errors.New("inventory size mismatch")
		}
		b, e := read(f.Path, f.Size)
		if e != nil {
			return d, e
		}
		if int64(len(b)) != f.Size || replayDigest(b) != f.SHA256 {
			return d, fmt.Errorf("checksum mismatch: %s", f.Path)
		}
		bytesRead += int64(len(b))
		if bytesRead > certificate.MaxBytes {
			return d, errors.New("inventory exceeds 512 MiB")
		}
		d.Files[f.Path] = b
	}
	if err = validateArchive(d); err != nil {
		return d, err
	}
	return d, nil
}

// Open a confined regular file; no archive member determines an arbitrary host path.
func archiveRead(ctx context.Context, dir, name string, limit int64) ([]byte, error) {
	if !archivePath(name) {
		return nil, errors.New("unsafe local evidence path")
	}
	if st, e := os.Lstat(dir); e != nil {
		return nil, e
	} else if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("evidence directory is not a local directory")
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	// Refuse intermediate symlinks too, although os.Root also confines traversal races.
	pieces := strings.Split(name, "/")
	for i := 1; i <= len(pieces); i++ {
		st, e := root.Lstat(strings.Join(pieces[:i], "/"))
		if e != nil {
			return nil, e
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("evidence links are not portable")
		}
	}
	return checkpointRead(ctx, root, filepath.FromSlash(name), limit)
}
func archiveManifest(raw []byte, id string) (artifacts.Manifest, error) {
	var m artifacts.Manifest
	var shape map[string]json.RawMessage
	if int64(len(raw)) > certificate.MaxManifestBytes {
		return m, errors.New("manifest exceeds bound")
	}
	if e := strictRunJSON(raw, &shape); e != nil {
		return m, e
	}
	if e := json.Unmarshal(raw, &m); e != nil {
		return m, e
	}
	if m.Format != "clonoloom-artifact" || m.Version != 1 || m.ID != id || (m.State != artifacts.Complete && m.State != artifacts.Invalid) || m.Kind == "" {
		return m, errors.New("unsupported artifact identity, version or state")
	}
	if !archivePath(id) || strings.Contains(id, "/") || id == "staging" || len(m.Files) > 256 {
		return m, errors.New("invalid artifact inventory")
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		if f.Location != "" || !archivePath(f.Path) || strings.EqualFold(f.Path, "manifest.json") || seen[strings.ToLower(f.Path)] || f.Size < 0 || f.Size > certificate.MaxFileBytes || !runDigest(f.SHA256) {
			return m, errors.New("artifact needs bounded, unique local files; external outputs are not portable")
		}
		seen[strings.ToLower(f.Path)] = true
	}
	return m, nil
}
func archiveRestricted(raw []byte) bool {
	// Inspect only explicit provider license-class fields, without floating conversion.
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if dec.Decode(&v) != nil {
		return false
	}
	var walk func(any) bool
	walk = func(v any) bool {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if strings.EqualFold(k, "licenseClass") {
					s, ok := val.(string)
					if !ok || s != "open" {
						return true
					}
				}
				if walk(val) {
					return true
				}
			}
		case []any:
			for _, val := range x {
				if walk(val) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}
func archiveBasename(p string) string { return path.Base(strings.ReplaceAll(p, "\\", "/")) }

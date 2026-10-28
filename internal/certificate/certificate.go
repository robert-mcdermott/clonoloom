// Package certificate records and checks saved artifact bytes. It performs no scientific
// computation and makes no claim of authenticity. Inspired by the certificate inventories
// in CytoWeave 0.8.1 and MaveScape; serialization and verification are native Go.
package certificate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"clonoloom/internal/artifacts"
)

const (
	Format                 = "clonoloom-certificate"
	Version                = 1
	Scope                  = "recorded-artifact-bytes"
	MaxBytes         int64 = 512 << 20
	MaxFileBytes     int64 = 256 << 20
	MaxManifestBytes int64 = 64 << 20
	MaxRecordBytes   int64 = 1 << 20
	MaxFiles               = 256
	Limits                 = "Integrity only: scientific computations, source files, parent artifacts and project history were not independently verified. A checksum is not a signature or proof of biological validity. Captured evidence may contain counts, sequences, subject identifiers and paths; this is not de-identified."
)

const readme = "ClonoLoom saved-result evidence certificate, version 1\n\n" + Limits + "\n\nThe archive contains the exact saved manifest and result files, provider NOTICE, and a SHA-256 inventory. Original source files and separate parent artifacts are not bundled. Their recorded fingerprints and the recipe remain in the saved evidence. No files are extracted or recipes executed during integrity verification.\n\nCheck with: clonoloom verify --integrity FILE.certificate.clz\nOptionally pin the separately recorded fingerprint with --expect SHA256.\nA successful check is an integrity match, not a recomputation verdict.\n"

type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Record's fingerprint is SHA-256 of encoding/json's compact encoding with the
// fingerprint field omitted. Files are strictly path-sorted. No floating values occur.
type Record struct {
	Format      string             `json:"format"`
	Version     int                `json:"version"`
	Scope       string             `json:"scope"`
	Artifact    string             `json:"artifact"`
	Kind        string             `json:"kind"`
	Software    artifacts.Software `json:"software"`
	Files       []File             `json:"files"`
	Fingerprint string             `json:"fingerprint,omitempty"`
}

type Report struct {
	Format       string   `json:"format"`
	Version      int      `json:"version"`
	Scope        string   `json:"scope"`
	Verdict      string   `json:"verdict"`
	Artifact     string   `json:"artifact,omitempty"`
	Fingerprint  string   `json:"fingerprint,omitempty"`
	FilesChecked int      `json:"filesChecked"`
	BytesChecked int64    `json:"bytesChecked"`
	Recomputed   bool     `json:"recomputed"`
	Limits       string   `json:"limits"`
	Findings     []string `json:"findings"`
}

func sum(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func digest(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && strings.ToLower(s) == s
}
func safePath(p string) bool {
	return len(p) <= 1024 && p != "." && fs.ValidPath(p) && !strings.ContainsAny(p, "\\:") && strings.IndexFunc(p, func(r rune) bool { return r < 32 || r == 127 }) < 0
}
func (r Record) fingerprint() string { r.Fingerprint = ""; raw, _ := json.Marshal(r); return sum(raw) }

// Manifest validates the bounded, self-contained artifact inventory before any files are read.
func Manifest(raw []byte) (artifacts.Manifest, error) {
	var m artifacts.Manifest
	if int64(len(raw)) > MaxManifestBytes {
		return m, errors.New("manifest exceeds 64 MiB")
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, err
	}
	if m.Format != "clonoloom-artifact" || m.Version != 1 || m.State != artifacts.Complete || m.ID == "" || m.Kind == "" || m.Lineage.Job == nil || len(m.Lineage.Job.Spec) == 0 {
		return m, errors.New("certificate requires a complete version-1 artifact with its recorded recipe")
	}
	if len(m.Files) == 0 || len(m.Files) > MaxFiles-3 {
		return m, errors.New("artifact file count is outside certificate limits")
	}
	seen := map[string]bool{}
	var total int64
	for _, f := range m.Files {
		if f.Location != artifacts.InArtifact || !safePath(f.Path) || !safePath("artifact/"+f.Path) || seen[f.Path] || !digest(f.SHA256) || f.Size < 0 || f.Size > MaxFileBytes {
			return m, fmt.Errorf("unsupported artifact file %q: needs a unique local path, bounded size and SHA-256", f.Path)
		}
		seen[f.Path] = true
		total += f.Size
	}
	if total+int64(len(raw)) > MaxBytes {
		return m, errors.New("artifact exceeds certificate size limit")
	}
	return m, nil
}

// Build writes a deterministic ZIP. open must confine paths to the artifact directory.
// Failure leaves an incomplete output, which the caller must discard, never publish.
func Build(ctx context.Context, out io.Writer, raw, notice []byte, software artifacts.Software, open func(string) (io.ReadCloser, error)) (Record, error) {
	m, err := Manifest(raw)
	if err != nil {
		return Record{}, err
	}
	if len(notice) == 0 || int64(len(notice)) > MaxRecordBytes {
		return Record{}, errors.New("missing or oversized provider notice")
	}
	r := Record{Format: Format, Version: Version, Scope: Scope, Artifact: m.ID, Kind: m.Kind, Software: software}
	contents := map[string][]byte{"manifest.json": raw, "NOTICE.txt": notice, "README.txt": []byte(readme)}
	for path, data := range contents {
		r.Files = append(r.Files, File{path, int64(len(data)), sum(data)})
	}
	for _, f := range m.Files {
		r.Files = append(r.Files, File{"artifact/" + f.Path, f.Size, f.SHA256})
	}
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })
	var total int64
	for _, f := range r.Files {
		total += f.Size
	}
	if total > MaxBytes {
		return Record{}, errors.New("certificate exceeds 512 MiB")
	}
	r.Fingerprint = r.fingerprint()
	z := zip.NewWriter(out)
	write := func(f File, source io.Reader) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		w, err := z.CreateHeader(&zip.FileHeader{Name: f.Path, Method: zip.Deflate})
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(w, h), &contextReader{ctx, io.LimitReader(source, f.Size+1)})
		if err != nil {
			return err
		}
		if n != f.Size || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
			return fmt.Errorf("artifact changed: %s", f.Path)
		}
		return nil
	}
	for _, f := range r.Files {
		if data, ok := contents[f.Path]; ok {
			err = write(f, bytes.NewReader(data))
		} else {
			var src io.ReadCloser
			src, err = open(strings.TrimPrefix(f.Path, "artifact/"))
			if err == nil {
				err = write(f, src)
				closeErr := src.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
		if err != nil {
			return Record{}, err
		}
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		return Record{}, err
	}
	if err = write(File{"certificate.json", int64(len(encoded)), sum(encoded)}, bytes.NewReader(encoded)); err != nil {
		return Record{}, err
	}
	if err = z.Close(); err != nil {
		return Record{}, err
	}
	return r, nil
}

type contextReader struct {
	ctx context.Context
	in  io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.in.Read(p)
}

// Verify reads bounded ZIP members in place. It never extracts, follows recorded
// source paths, fetches network data, opens a library, or executes the recorded recipe.
func Verify(ctx context.Context, in io.ReaderAt, size int64, expected string) Report {
	r := Report{Format: "clonoloom-verification", Version: 1, Scope: Scope, Verdict: "differs", Limits: Limits, Findings: []string{}}
	fail := func(err error) Report { r.Findings = append(r.Findings, err.Error()); return r }
	if expected != "" && !digest(expected) {
		return fail(errors.New("expected fingerprint must be 64 lowercase hexadecimal characters"))
	}
	if size < 0 || size > MaxBytes+(8<<20) {
		return fail(errors.New("certificate compressed size exceeds limit"))
	}
	z, err := zip.NewReader(in, size)
	if err != nil {
		return fail(err)
	}
	if len(z.File) < 4 || len(z.File) > MaxFiles+1 {
		return fail(errors.New("certificate member count is outside limits"))
	}
	entries := map[string]*zip.File{}
	var total uint64
	for _, f := range z.File {
		if !safePath(f.Name) || entries[f.Name] != nil || !f.Mode().IsRegular() || f.Flags&1 != 0 || f.UncompressedSize64 > uint64(MaxFileBytes) || (f.Method != zip.Store && f.Method != zip.Deflate) {
			return fail(fmt.Errorf("unsafe, duplicate or oversized ZIP member %q", f.Name))
		}
		total += f.UncompressedSize64
		entries[f.Name] = f
	}
	if total > uint64(MaxBytes+MaxRecordBytes) {
		return fail(errors.New("certificate expanded size exceeds limit"))
	}
	if record := entries["certificate.json"]; record != nil && total-record.UncompressedSize64 > uint64(MaxBytes) {
		return fail(errors.New("certificate inventory exceeds 512 MiB"))
	}
	read := func(name string, limit int64) ([]byte, error) {
		f := entries[name]
		if f == nil {
			return nil, fmt.Errorf("missing %s", name)
		}
		if f.UncompressedSize64 > uint64(limit) {
			return nil, fmt.Errorf("oversized %s", name)
		}
		src, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer src.Close()
		b, e := io.ReadAll(&contextReader{ctx, io.LimitReader(src, limit+1)})
		if e == nil && int64(len(b)) > limit {
			e = errors.New("member exceeds limit")
		}
		return b, e
	}
	encoded, err := read("certificate.json", MaxRecordBytes)
	if err != nil {
		return fail(err)
	}
	var record Record
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&record); err != nil {
		return fail(err)
	}
	// Exact canonical bytes reject duplicate keys, alternate encodings and trailing JSON.
	canonical, _ := json.Marshal(record)
	if !bytes.Equal(encoded, canonical) {
		return fail(errors.New("certificate.json is not the canonical version-1 record"))
	}
	if record.Format != Format || record.Version != Version || record.Scope != Scope || !digest(record.Fingerprint) || record.Fingerprint != record.fingerprint() {
		return fail(errors.New("unsupported certificate or fingerprint mismatch"))
	}
	r.Fingerprint, r.Artifact = record.Fingerprint, record.Artifact
	if expected != "" && expected != record.Fingerprint {
		return fail(errors.New("certificate differs from the independently supplied fingerprint"))
	}
	if len(record.Files)+1 != len(entries) {
		return fail(errors.New("unlisted or missing certificate members"))
	}
	previous := ""
	inventory := map[string]File{}
	for _, f := range record.Files {
		entry := entries[f.Path]
		if f.Path <= previous || f.Path == "certificate.json" || !safePath(f.Path) || !digest(f.SHA256) || f.Size < 0 || entry == nil || entry.UncompressedSize64 != uint64(f.Size) {
			return fail(fmt.Errorf("invalid inventory entry %q", f.Path))
		}
		previous = f.Path
		inventory[f.Path] = f
		src, e := entry.Open()
		if e != nil {
			return fail(e)
		}
		h := sha256.New()
		n, e := io.Copy(h, &contextReader{ctx, io.LimitReader(src, f.Size+1)})
		src.Close()
		if e != nil {
			return fail(e)
		}
		if n != f.Size || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
			return fail(fmt.Errorf("checksum mismatch: %s", f.Path))
		}
		r.FilesChecked++
		r.BytesChecked += n
	}
	raw, err := read("manifest.json", MaxManifestBytes)
	if err != nil {
		return fail(err)
	}
	m, err := Manifest(raw)
	if err != nil {
		return fail(err)
	}
	if m.ID != record.Artifact || m.Kind != record.Kind || len(m.Files)+3 != len(record.Files) {
		return fail(errors.New("artifact manifest differs from certificate inventory"))
	}
	for _, f := range m.Files {
		got := inventory["artifact/"+f.Path]
		if got.Size != f.Size || got.SHA256 != f.SHA256 {
			return fail(fmt.Errorf("manifest inventory differs: %s", f.Path))
		}
	}
	if n, ok := inventory["NOTICE.txt"]; !ok || n.Size == 0 || n.Size > MaxRecordBytes {
		return fail(errors.New("missing or oversized provider notice"))
	}
	if b, e := read("README.txt", MaxRecordBytes); e != nil || string(b) != readme {
		return fail(errors.New("missing or changed interpretation limits"))
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	r.Verdict = "integrity-match"
	return r
}

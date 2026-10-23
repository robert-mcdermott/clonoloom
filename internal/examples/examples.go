// Package examples downloads ClonoLoom's example datasets on request (clonoloom-spec/requirements.md,
// D13 and E2): public data from their publishers' addresses, each with its license and citation
// (validation/sources.json, embedded in the program). A data set is files under a base URL, each
// checked by its size and SHA-256, or members of a large remote zip taken by HTTP range, each
// checked by its CRC-32, size and SHA-256. Nothing is downloaded unless asked, nothing at all with
// --offline, and data under a provider's terms of use only once the user has accepted them. A
// download is written beside its folder and moved into place when every file is checked, so a
// cancelled or failed one leaves nothing behind. Adapted from SituCraft 0.9.0
// internal/examples.
package examples

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// File is one file of a data set.
type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Opening is a dataset an example opens: some of its files together, the dataset's name, and the
// steps that guide a first look at it.
type Opening struct {
	Title   string   `json:"title"`
	Name    string   `json:"name"`
	Summary string   `json:"summary"`
	Files   []string `json:"files"`
	Steps   []Step   `json:"steps,omitempty"`
}

// Step is a guided step of an example: what it does and what to look for, in words, and the
// remote control action (actions.go) that does it, with its arguments; the page performs it on the
// dataset the example opened.
type Step struct {
	Title  string         `json:"title"`
	Words  string         `json:"words"`
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

// Source is one data set of sources.json.
type Source struct {
	Name         string    `json:"name"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Source       string    `json:"source"`
	Publisher    string    `json:"publisher"`
	Page         string    `json:"page"`
	License      string    `json:"license"`
	LicenseURL   string    `json:"licenseUrl"`
	LicenseClass string    `json:"licenseClass"`
	Citation     string    `json:"citation"`
	Base         string    `json:"base,omitempty"`
	Members      *Members  `json:"members,omitempty"`
	Files        []File    `json:"files"`
	Opens        []Opening `json:"opens,omitempty"`
}

// Members are files taken from a large remote zip archive by HTTP range: each named member, less
// strip, into a folder of the data set.
type Members struct {
	URL   string   `json:"url"`
	Size  int64    `json:"size"`
	Names []string `json:"names"`
	Strip string   `json:"strip,omitempty"`
	Into  string   `json:"into,omitempty"`
}

// Parse reads sources.json, keeping the data sets offered as examples (those that say what they
// open) whose files are recorded, in the file's order.
func Parse(raw []byte) ([]Source, error) {
	all, err := ParseAll(raw)
	if err != nil {
		return nil, err
	}
	var out []Source
	for _, s := range all {
		if len(s.Opens) > 0 {
			out = append(out, s)
		}
	}
	return out, nil
}

// ParseAll reads every data set of sources.json whose files are recorded, offered or not (those
// queried from an ADC API, or for maintainers only), in the file's order: what a dataset's files
// are identified by.
func ParseAll(raw []byte) ([]Source, error) {
	var doc struct {
		Datasets map[string]Source `json:"datasets"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	order, err := keyOrder(raw, "datasets")
	if err != nil {
		return nil, err
	}
	var out []Source
	for _, name := range order {
		s := doc.Datasets[name]
		s.Name = name
		if len(s.Files) == 0 {
			continue
		}
		if len(s.Opens) > 0 && s.Base == "" && s.Members == nil {
			return nil, fmt.Errorf("%s: offered, but with no files to download", name)
		}
		for _, o := range s.Opens {
			if o.Name == "" || o.Title == "" {
				return nil, fmt.Errorf("%s opens a dataset without a title and a name", name)
			}
			for _, f := range o.Files {
				if !s.has(f) {
					return nil, fmt.Errorf("%s opens %s, which it does not download", name, f)
				}
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// ParseReferences reads the reference data of sources.json (its "references": data the program
// uses rather than opens, such as the V genes' loops TCRdist compares), downloaded on request as
// examples are, in the file's order.
func ParseReferences(raw []byte) ([]Source, error) {
	var doc struct {
		References map[string]Source `json:"references"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	order, err := keyOrder(raw, "references")
	if err != nil {
		return nil, err
	}
	var out []Source
	for _, name := range order {
		s := doc.References[name]
		s.Name = name
		if len(s.Files) == 0 || (s.Base == "" && (s.Members == nil || s.Members.URL == "" || len(s.Members.Names) == 0)) {
			return nil, fmt.Errorf("%s: reference data with no files to download", name)
		}
		out = append(out, s)
	}
	return out, nil
}

// keyOrder answers the names of the objects under one of sources.json's top-level keys, in the
// file's order (a map in Go has none).
func keyOrder(raw []byte, key string) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	depth := 0
	var names []string
	inDatasets := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return names, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case json.Delim:
			if t == '{' || t == '[' {
				depth++
			} else {
				depth--
				if depth == 1 {
					inDatasets = false
				}
			}
		case string:
			// Keys and values alternate in an object; at depth 1, the key opens the list, and at
			// depth 2 within it each key is a data set's name, followed by its object.
			if depth == 1 && t == key && dec.More() {
				inDatasets = true
				continue
			}
			if inDatasets && depth == 2 {
				names = append(names, t)
				var skip json.RawMessage
				if err := dec.Decode(&skip); err != nil {
					return nil, err
				}
			} else if depth == 1 {
				var skip json.RawMessage
				if err := dec.Decode(&skip); err != nil {
					return nil, err
				}
			}
		}
	}
}

func (s Source) has(p string) bool {
	for _, f := range s.Files {
		if f.Path == p {
			return true
		}
	}
	return false
}

func (s Source) size() int64 {
	var n int64
	for _, f := range s.Files {
		n += f.Size
	}
	return n
}

// Generated is an example made on this computer rather than downloaded (a simulated study): its
// description as the page shows it, what it takes on disk, the file that says it is made, how to
// make it, and, once made, the files each of its openings opens (relative to its folder).
type Generated struct {
	Source
	Size   int64
	Marker string
	Make   func(ctx context.Context, dir string, progress Progress) error
	Files  func(dir string) [][]string
}

// Library is the examples folder.
type Library struct {
	Dir string
	// Generated: the examples made on this computer.
	Generated []Generated
	// Sources are the examples offered; Recorded, every data set sources.json records (what
	// Identify knows), or the offered alone when it is nil.
	Sources  []Source
	Recorded []Source
	Client   *http.Client
	// UserAgent names ClonoLoom to the publisher's server.
	UserAgent string
	// Offline: no download at all (--offline).
	Offline bool
}

// State is an example as the page shows it.
type State struct {
	Source
	Size int64 `json:"size"`
	// State is "absent", "ready" (every file there with its size) or "incomplete".
	State string `json:"state"`
	Path  string `json:"path,omitempty"`
	// Paths are each opening's files on this computer, when ready.
	Paths [][]string `json:"paths,omitempty"`
	// Generated: made on this computer, nothing downloaded.
	Generated bool `json:"generated,omitempty"`
}

// List describes every example.
func (l *Library) List() []State {
	out := make([]State, 0, len(l.Sources))
	for _, s := range l.Sources {
		st := State{Source: s, Size: s.size(), State: "absent"}
		dir := filepath.Join(l.Dir, s.Name)
		present, complete := 0, true
		for _, f := range s.Files {
			info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.Path)))
			if err == nil && info.Size() == f.Size {
				present++
			} else {
				complete = false
			}
		}
		switch {
		case complete:
			st.State, st.Path = "ready", dir
			for _, o := range s.Opens {
				var paths []string
				for _, f := range o.Files {
					paths = append(paths, filepath.Join(dir, filepath.FromSlash(f)))
				}
				st.Paths = append(st.Paths, paths)
			}
		case present > 0:
			st.State, st.Path = "incomplete", dir
		}
		out = append(out, st)
	}
	// A generated example is ready once its marker is there.
	for _, g := range l.Generated {
		st := State{Source: g.Source, Size: g.Size, State: "absent", Generated: true}
		dir := filepath.Join(l.Dir, g.Name)
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(g.Marker))); err == nil && !info.IsDir() {
			st.State, st.Path = "ready", dir
			for _, files := range g.Files(dir) {
				var paths []string
				for _, f := range files {
					paths = append(paths, filepath.Join(dir, filepath.FromSlash(f)))
				}
				st.Paths = append(st.Paths, paths)
			}
		}
		out = append(out, st)
	}
	return out
}

func (l *Library) source(name string) (Source, bool) {
	for _, s := range l.Sources {
		if s.Name == name {
			return s, true
		}
	}
	if g, ok := l.generated(name); ok {
		return g.Source, true
	}
	return Source{}, false
}

func (l *Library) generated(name string) (Generated, bool) {
	for _, g := range l.Generated {
		if g.Name == name {
			return g, true
		}
	}
	return Generated{}, false
}

// Remove deletes a downloaded (or made) example.
func (l *Library) Remove(name string) error {
	if _, ok := l.source(name); !ok {
		return fmt.Errorf("no example %q", name)
	}
	return os.RemoveAll(filepath.Join(l.Dir, name))
}

// Identify answers the recorded data sets files of these SHA-256s belong to, wherever and however
// the files were downloaded: so that a dataset made of them carries their source, license and
// citation.
func (l *Library) Identify(sha256s []string) []Source {
	known := l.Recorded
	if known == nil {
		known = l.Sources
	}
	var out []Source
	for _, s := range known {
		for _, f := range s.Files {
			if contains(sha256s, f.SHA256) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Progress receives the fraction done and what is being done.
type Progress func(fraction float64, message string)

// ErrOffline: ClonoLoom was started with --offline.
var ErrOffline = errors.New("ClonoLoom was started with --offline: it downloads nothing")

// ErrTerms: a data set under a provider's terms of use, whose terms were not accepted.
var ErrTerms = errors.New("these data are under their provider's terms of use: accept them to download")

// Download fetches an example and checks every file, answering its folder. accepted says the user
// accepted the provider's terms of use, which data sets of the class "terms" need.
func (l *Library) Download(ctx context.Context, name string, accepted bool, progress Progress) (string, error) {
	if progress == nil {
		progress = func(float64, string) {}
	}
	if g, ok := l.generated(name); ok {
		return l.make(ctx, g, progress)
	}
	s, ok := l.source(name)
	if !ok {
		return "", fmt.Errorf("no example %q", name)
	}
	if l.Offline {
		return "", ErrOffline
	}
	if s.LicenseClass != "open" && !accepted {
		return "", ErrTerms
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return "", err
	}
	staging := filepath.Join(l.Dir, "."+name+".part")
	defer os.RemoveAll(staging)
	if err := os.RemoveAll(staging); err != nil {
		return "", err
	}
	total := float64(s.size())
	var done float64
	report := func(n int64, file string) {
		done += float64(n)
		// Below 1 until every file is checked (a file longer than recorded is read one byte past).
		progress(min(done/total, 0.999), "Downloading "+file)
	}
	if s.Members != nil {
		if err := l.members(ctx, s, staging, report); err != nil {
			return "", err
		}
	} else {
		for _, f := range s.Files {
			if err := l.file(ctx, s, f, staging, report); err != nil {
				return "", err
			}
		}
	}
	// Every file checked by its size and SHA-256 before the folder is put in place.
	for _, f := range s.Files {
		if err := check(filepath.Join(staging, filepath.FromSlash(f.Path)), f); err != nil {
			return "", err
		}
	}
	marker, _ := json.MarshalIndent(map[string]any{"example": name, "verified": time.Now().UTC().Format(time.RFC3339), "source": s.Source, "license": s.License, "citation": s.Citation}, "", "  ")
	if err := os.WriteFile(filepath.Join(staging, ".clonoloom-example.json"), marker, 0o644); err != nil {
		return "", err
	}
	target := filepath.Join(l.Dir, name)
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.Rename(staging, target); err != nil {
		return "", err
	}
	progress(1, "Ready")
	return target, nil
}

// make makes a generated example beside its folder and moves it into place once made: nothing is
// downloaded, so --offline does not stop it.
func (l *Library) make(ctx context.Context, g Generated, progress Progress) (string, error) {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return "", err
	}
	staging := filepath.Join(l.Dir, "."+g.Name+".part")
	defer os.RemoveAll(staging)
	if err := os.RemoveAll(staging); err != nil {
		return "", err
	}
	if err := g.Make(ctx, staging, progress); err != nil {
		return "", err
	}
	target := filepath.Join(l.Dir, g.Name)
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.Rename(staging, target); err != nil {
		return "", err
	}
	progress(1, "Ready")
	return target, nil
}

// safe: a recorded path stays inside its folder.
func safe(p string) (string, error) {
	clean := path.Clean(p)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("unsafe path %q", p)
	}
	return filepath.FromSlash(clean), nil
}

func (l *Library) get(ctx context.Context, address string, header map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", l.UserAgent)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	client := l.Client
	if client == nil {
		client = l.defaultClient()
	}
	// A server's failure (5xx) or a broken connection is tried again, twice; a refused redirect is
	// not.
	for attempt := 1; ; attempt++ {
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		if err == nil {
			resp.Body.Close()
			err = fmt.Errorf("HTTP %d from %s", resp.StatusCode, req.URL.Host)
		}
		if attempt == 3 || errors.Is(err, errRedirect) || ctx.Err() != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
}

// file downloads one file under the source's base URL, at most its recorded size.
func (l *Library) file(ctx context.Context, s Source, f File, dir string, report func(int64, string)) error {
	rel, err := safe(f.Path)
	if err != nil {
		return err
	}
	base, err := url.Parse(s.Base)
	if err != nil {
		return err
	}
	ref, err := url.Parse(f.Path)
	if err != nil {
		return err
	}
	resp, err := l.get(ctx, base.ResolveReference(ref).String(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d from %s", f.Path, resp.StatusCode, base.Host)
	}
	target := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, &counting{r: io.LimitReader(resp.Body, f.Size+1), report: func(k int64) { report(k, path.Base(f.Path)) }})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n != f.Size {
		return fmt.Errorf("%s: %d bytes, %d recorded: the publisher has changed it", f.Path, n, f.Size)
	}
	return nil
}

type counting struct {
	r      io.Reader
	report func(int64)
}

func (c *counting) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.report(int64(n))
	}
	return n, err
}

// check compares a file with its record.
func check(p string, f File) error {
	file, err := os.Open(p)
	if err != nil {
		return err
	}
	defer file.Close()
	h := sha256.New()
	n, err := io.Copy(h, file)
	if err != nil {
		return err
	}
	if n != f.Size || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
		return fmt.Errorf("%s does not match its record (size or SHA-256): the publisher has changed it, and ClonoLoom does not use a changed file", f.Path)
	}
	return nil
}

// members takes the named members of a remote zip by HTTP range: its central directory (ZIP64
// included), then each member, inflated and checked by its CRC-32 and size.
func (l *Library) members(ctx context.Context, s Source, dir string, report func(int64, string)) error {
	m := s.Members
	ranged := func(start, end int64) ([]byte, error) {
		resp, err := l.get(ctx, m.URL, map[string]string{"Range": fmt.Sprintf("bytes=%d-%d", start, end)})
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusPartialContent {
			return nil, fmt.Errorf("HTTP %d for a range of %s", resp.StatusCode, m.URL)
		}
		// "bytes start-end/size": an archive of another size is another archive.
		if total := resp.Header.Get("Content-Range"); total != "" && !strings.HasSuffix(total, fmt.Sprintf("/%d", m.Size)) {
			return nil, fmt.Errorf("the archive is not the %d bytes recorded (%s): the publisher has changed it", m.Size, total)
		}
		return io.ReadAll(io.LimitReader(resp.Body, end-start+1))
	}
	tailLen := min(m.Size, 65557)
	tail, err := ranged(m.Size-tailLen, m.Size-1)
	if err != nil {
		return err
	}
	eocd := bytes.LastIndex(tail, []byte{0x50, 0x4b, 0x05, 0x06})
	if eocd < 0 || eocd+22 > len(tail) {
		return errors.New("not a zip archive (no end of central directory): the publisher has changed it")
	}
	count := uint64(binary.LittleEndian.Uint16(tail[eocd+10:]))
	cdSize := uint64(binary.LittleEndian.Uint32(tail[eocd+12:]))
	cdOffset := uint64(binary.LittleEndian.Uint32(tail[eocd+16:]))
	if count == 0xffff || cdSize == 0xffffffff || cdOffset == 0xffffffff {
		loc := bytes.LastIndex(tail, []byte{0x50, 0x4b, 0x06, 0x07})
		if loc < 0 || loc+16 > len(tail) {
			return errors.New("a ZIP64 archive without its locator")
		}
		recordOffset := int64(binary.LittleEndian.Uint64(tail[loc+8:]))
		rec, err := ranged(recordOffset, recordOffset+55)
		if err != nil || len(rec) < 56 {
			return fmt.Errorf("the ZIP64 record: %v", err)
		}
		count, cdSize, cdOffset = binary.LittleEndian.Uint64(rec[32:]), binary.LittleEndian.Uint64(rec[40:]), binary.LittleEndian.Uint64(rec[48:])
	}
	if cdSize > 64<<20 || int64(cdOffset)+int64(cdSize) > m.Size {
		return errors.New("the archive's central directory is not where it says")
	}
	cd, err := ranged(int64(cdOffset), int64(cdOffset+cdSize)-1)
	if err != nil {
		return err
	}
	type entry struct {
		method                         uint16
		crc                            uint32
		compressed, uncompressed, offs uint64
	}
	wanted := map[string]bool{}
	for _, n := range m.Names {
		wanted[n] = true
	}
	found := map[string]entry{}
	p := 0
	for i := uint64(0); i < count && p+46 <= len(cd); i++ {
		if binary.LittleEndian.Uint32(cd[p:]) != 0x02014b50 {
			return errors.New("a corrupt central directory")
		}
		e := entry{method: binary.LittleEndian.Uint16(cd[p+10:]), crc: binary.LittleEndian.Uint32(cd[p+16:]),
			compressed: uint64(binary.LittleEndian.Uint32(cd[p+20:])), uncompressed: uint64(binary.LittleEndian.Uint32(cd[p+24:])), offs: uint64(binary.LittleEndian.Uint32(cd[p+42:]))}
		nameLen, extraLen, commentLen := int(binary.LittleEndian.Uint16(cd[p+28:])), int(binary.LittleEndian.Uint16(cd[p+30:])), int(binary.LittleEndian.Uint16(cd[p+32:]))
		if p+46+nameLen+extraLen > len(cd) {
			return errors.New("a corrupt central directory")
		}
		name := string(cd[p+46 : p+46+nameLen])
		extra := cd[p+46+nameLen : p+46+nameLen+extraLen]
		for q := 0; q+4 <= len(extra); {
			id, length := binary.LittleEndian.Uint16(extra[q:]), int(binary.LittleEndian.Uint16(extra[q+2:]))
			if id == 0x0001 {
				r := q + 4
				if e.uncompressed == 0xffffffff && r+8 <= len(extra) {
					e.uncompressed = binary.LittleEndian.Uint64(extra[r:])
					r += 8
				}
				if e.compressed == 0xffffffff && r+8 <= len(extra) {
					e.compressed = binary.LittleEndian.Uint64(extra[r:])
					r += 8
				}
				if e.offs == 0xffffffff && r+8 <= len(extra) {
					e.offs = binary.LittleEndian.Uint64(extra[r:])
				}
			}
			q += 4 + length
		}
		short := strings.TrimPrefix(name, m.Strip)
		if wanted[short] {
			found[short] = e
		}
		p += 46 + nameLen + extraLen + commentLen
	}
	for _, name := range m.Names {
		e, ok := found[name]
		if !ok {
			return fmt.Errorf("the archive lacks %s: the publisher has changed it", name)
		}
		if e.method != 0 && e.method != 8 {
			return fmt.Errorf("%s: compression method %d is not read", name, e.method)
		}
		rel, err := safe(m.Into + name)
		if err != nil {
			return err
		}
		local, err := ranged(int64(e.offs), int64(e.offs)+29)
		if err != nil || len(local) < 30 || binary.LittleEndian.Uint32(local) != 0x04034b50 {
			return fmt.Errorf("%s: a corrupt local header", name)
		}
		start := int64(e.offs) + 30 + int64(binary.LittleEndian.Uint16(local[26:])) + int64(binary.LittleEndian.Uint16(local[28:]))
		resp, err := l.get(ctx, m.URL, map[string]string{"Range": fmt.Sprintf("bytes=%d-%d", start, start+int64(e.compressed)-1)})
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return fmt.Errorf("HTTP %d for a range of %s", resp.StatusCode, m.URL)
		}
		var body io.Reader = io.LimitReader(resp.Body, int64(e.compressed))
		if e.method == 8 {
			body = flate.NewReader(body)
		}
		target := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			resp.Body.Close()
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			resp.Body.Close()
			return err
		}
		crc := crc32.NewIEEE()
		n, err := io.Copy(io.MultiWriter(out, crc), &counting{r: io.LimitReader(body, int64(e.uncompressed)+1), report: func(k int64) { report(k, name) }})
		resp.Body.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if uint64(n) != e.uncompressed || crc.Sum32() != e.crc {
			return fmt.Errorf("%s: its size or CRC-32 does not match the archive's record", name)
		}
	}
	return nil
}

var errRedirect = errors.New("a redirect to a host that is not one of the publishers'")

// defaultClient follows redirects only to the publishers' hosts, over HTTPS.
func (l *Library) defaultClient() *http.Client {
	hosts := l.Hosts()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 60 * time.Second
	return &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" || !contains(hosts, req.URL.Host) {
			return fmt.Errorf("%w: %s", errRedirect, req.URL.Host)
		}
		return nil
	}}
}

// Hosts are the hosts the examples are downloaded from: the only ones ClonoLoom reaches.
func (l *Library) Hosts() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range l.Sources {
		for _, a := range []string{s.Base, func() string {
			if s.Members != nil {
				return s.Members.URL
			}
			return ""
		}()} {
			if u, err := url.Parse(a); err == nil && u.Host != "" {
				if !seen[u.Host] {
					seen[u.Host] = true
					out = append(out, u.Host)
				}
				// GitHub serves pinned release assets from this dedicated CDN.
				// It is allowed only when the catalog contains a release URL.
				if u.Host == "github.com" && strings.Contains(u.Path, "/releases/download/") && !seen["release-assets.githubusercontent.com"] {
					seen["release-assets.githubusercontent.com"] = true
					out = append(out, "release-assets.githubusercontent.com")
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

package examples

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func record(path string, data []byte) File {
	sum := sha256.Sum256(data)
	return File{Path: path, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

// The examples sources.json offers: each opens files it downloads, and the data sets queried
// from an ADC API (no files to download as such) are not offered.
func TestTheRecordedExamples(t *testing.T) {
	raw, err := os.ReadFile("../../validation/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	sources, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]Source{}
	for _, s := range sources {
		names[s.Name] = s
		if s.Citation == "" || s.License == "" || s.Source == "" || s.Title == "" {
			t.Errorf("%s: its source, license and citation must be recorded", s.Name)
		}
		if s.LicenseClass != "open" && s.LicenseClass != "terms" {
			t.Errorf("%s: license class %q is not offered", s.Name, s.LicenseClass)
		}
	}
	for _, want := range []string{"10x-pbmc-1k", "10x-mouse-splenocytes-1k", "10x-pbmc-10k-v2", "10x-mouse-splenocytes-5k-gemx", "airr-standards-tests", "dash-2017-tcrdist3", "ots-healthy-pbmc", "emerson-2017-immunoseq"} {
		if _, ok := names[want]; !ok {
			t.Errorf("%s is not offered", want)
		}
	}
	if _, ok := names["emerson-2017-adc"]; ok {
		t.Error("an ADC query is offered")
	}
	// In the file's order: the smallest human dataset first, data under terms of use last.
	if sources[0].Name != "10x-pbmc-1k" || sources[len(sources)-1].LicenseClass != "terms" {
		t.Errorf("listed first %s, last %s", sources[0].Name, sources[len(sources)-1].Name)
	}
	// Every recorded data set identifies its files, offered or not: the two ADC queries, and
	// three kept for the readers' validation.
	all, err := ParseAll(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(sources)+5 {
		t.Errorf("%d recorded, %d offered", len(all), len(sources))
	}
	adc := all[0]
	for _, s := range all {
		if s.Name == "emerson-2017-adc" {
			adc = s
		}
	}
	if ids := (&Library{Sources: sources, Recorded: all}).Identify([]string{adc.Files[1].SHA256}); len(ids) != 1 || ids[0].Name != "emerson-2017-adc" || ids[0].LicenseClass != "terms" {
		t.Errorf("identified %v", ids)
	}
	lib := &Library{Sources: sources}
	if hosts := strings.Join(lib.Hosts(), " "); hosts != "adaptivepublic.blob.core.windows.net cf.10xgenomics.com opig.stats.ox.ac.uk raw.githubusercontent.com" {
		t.Errorf("hosts %s", hosts)
	}
}

// Files under a base URL: downloaded, checked, put in place; listed ready with each opening's
// paths; recognized by their SHA-256; removed.
func TestFilesDownloadAndCheck(t *testing.T) {
	files := map[string][]byte{"a_t_filtered_contig_annotations.csv": bytes.Repeat([]byte("contig,"), 300), "sub/b.tsv": []byte("sequence_id\tlocus\n")}
	s := Source{Name: "tiny", Title: "Tiny", Source: "a test", License: "CC BY 4.0", LicenseClass: "open", Citation: "A test.",
		Opens: []Opening{{Title: "T cells", Name: "Tiny T cells", Files: []string{"a_t_filtered_contig_annotations.csv"}}}}
	for _, p := range []string{"a_t_filtered_contig_annotations.csv", "sub/b.tsv"} {
		s.Files = append(s.Files, record(p, files[p]))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "ClonoLoom-test" {
			t.Errorf("User-Agent %q", r.Header.Get("User-Agent"))
		}
		data, ok := files[strings.TrimPrefix(r.URL.Path, "/data/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	defer server.Close()
	s.Base = server.URL + "/data/"
	dir := t.TempDir()
	lib := &Library{Dir: dir, Sources: []Source{s}, UserAgent: "ClonoLoom-test"}
	if st := lib.List(); st[0].State != "absent" || st[0].Size != s.size() {
		t.Fatalf("before: %+v", st[0])
	}
	var last float64
	got, err := lib.Download(context.Background(), "tiny", false, func(f float64, _ string) {
		if f < last {
			t.Errorf("progress went back from %v to %v", last, f)
		}
		last = f
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "tiny") || last != 1 {
		t.Errorf("at %s, progress %v", got, last)
	}
	st := lib.List()[0]
	if st.State != "ready" || len(st.Paths) != 1 || st.Paths[0][0] != filepath.Join(got, "a_t_filtered_contig_annotations.csv") {
		t.Errorf("after: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(got, ".clonoloom-example.json")); err != nil {
		t.Error("no record of the download beside it")
	}
	if ids := lib.Identify([]string{"other", s.Files[1].SHA256}); len(ids) != 1 || ids[0].Name != "tiny" {
		t.Errorf("identified %v", ids)
	}
	if ids := lib.Identify([]string{"other"}); len(ids) != 0 {
		t.Errorf("identified %v", ids)
	}
	if err := lib.Remove("tiny"); err != nil || lib.List()[0].State != "absent" {
		t.Errorf("removed: %v", err)
	}
}

// A file the publisher changed is refused, and a refused download leaves nothing behind.
func TestAChangedFileIsRefused(t *testing.T) {
	for _, serve := range []string{"changed", "longer", "missing"} {
		data := []byte("sequence_id\tlocus\nx\tTRB\n")
		s := Source{Name: "tiny", LicenseClass: "open", Files: []File{record("x.tsv", data)}}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch serve {
			case "changed":
				w.Write(bytes.ToUpper(data))
			case "longer":
				w.Write(append(data, 'x'))
			default:
				http.NotFound(w, r)
			}
		}))
		s.Base = server.URL + "/"
		dir := t.TempDir()
		lib := &Library{Dir: dir, Sources: []Source{s}}
		if _, err := lib.Download(context.Background(), "tiny", false, nil); err == nil {
			t.Errorf("%s: accepted", serve)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%s: left %v", serve, entries)
		}
		server.Close()
	}
}

// Nothing with --offline; data under terms of use only once they are accepted; nothing outside
// the folder.
func TestOfflineTermsAndPaths(t *testing.T) {
	s := Source{Name: "terms", LicenseClass: "terms", Base: "http://127.0.0.1:1/", Files: []File{{Path: "x.tsv", Size: 1}}}
	lib := &Library{Dir: t.TempDir(), Sources: []Source{s}, Offline: true}
	if _, err := lib.Download(context.Background(), "terms", true, nil); !errors.Is(err, ErrOffline) {
		t.Errorf("offline: %v", err)
	}
	lib.Offline = false
	if _, err := lib.Download(context.Background(), "terms", false, nil); !errors.Is(err, ErrTerms) {
		t.Errorf("terms: %v", err)
	}
	for _, p := range []string{"../x", "/etc/x", "a/../../x", ".."} {
		if _, err := safe(p); err == nil {
			t.Errorf("%s is safe", p)
		}
	}
	if _, err := lib.Download(context.Background(), "nothing", true, nil); err == nil {
		t.Error("an unknown example downloaded")
	}
}

// A redirect is followed only to a publisher's host over HTTPS.
func TestRedirectsStayWithThePublishers(t *testing.T) {
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("x")) }))
	defer elsewhere.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/x.tsv", http.StatusFound)
	}))
	defer server.Close()
	s := Source{Name: "r", LicenseClass: "open", Base: server.URL + "/", Files: []File{record("x.tsv", []byte("x"))}}
	lib := &Library{Dir: t.TempDir(), Sources: []Source{s}}
	if _, err := lib.Download(context.Background(), "r", false, nil); err == nil || !errors.Is(err, errRedirect) {
		t.Errorf("redirected: %v", err)
	}
}

// Members of a remote zip, taken by HTTP range (stored and deflated, under a folder stripped
// from their names); the rest of the archive is not read.
func TestMembersOfARemoteZip(t *testing.T) {
	members := map[string][]byte{"P1.tsv": bytes.Repeat([]byte("rearrangement\t1\n"), 5000), "P2.tsv": []byte("rearrangement\t2\n")}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, m := range []struct {
		name   string
		method uint16
	}{{"project/P1.tsv", zip.Deflate}, {"project/big.tsv", zip.Deflate}, {"project/P2.tsv", zip.Store}} {
		w, _ := zw.CreateHeader(&zip.FileHeader{Name: m.name, Method: m.method})
		if data, ok := members[strings.TrimPrefix(m.name, "project/")]; ok {
			w.Write(data)
		} else {
			noise := make([]byte, 200000) // incompressible: most of the archive
			for i, x := 0, uint32(1); i < len(noise); i++ {
				x ^= x << 13
				x ^= x >> 17
				x ^= x << 5
				noise[i] = byte(x)
			}
			w.Write(noise)
		}
	}
	zw.Close()
	archive := buf.Bytes()
	var served int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "" {
			t.Error("the whole archive was asked for")
		}
		cw := &countingWriter{ResponseWriter: w}
		http.ServeContent(cw, r, "a.zip", time.Time{}, bytes.NewReader(archive))
		served += cw.n
	}))
	defer server.Close()
	source := func(size int64) Source {
		s := Source{Name: "zip", LicenseClass: "terms", Files: []File{record("P1.tsv", members["P1.tsv"]), record("P2.tsv", members["P2.tsv"])}}
		s.Members = &Members{URL: server.URL + "/a.zip", Size: size, Names: []string{"P1.tsv", "P2.tsv"}, Strip: "project/"}
		return s
	}
	dir := t.TempDir()
	lib := &Library{Dir: dir, Sources: []Source{source(int64(len(archive)))}}
	got, err := lib.Download(context.Background(), "zip", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range members {
		if data, err := os.ReadFile(filepath.Join(got, name)); err != nil || !bytes.Equal(data, want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if served >= int64(len(archive))/2 {
		t.Errorf("%d of the archive's %d bytes read", served, len(archive))
	}
	// An archive of another size than recorded is another archive.
	lib = &Library{Dir: t.TempDir(), Sources: []Source{source(int64(len(archive)) + 10)}}
	if _, err := lib.Download(context.Background(), "zip", true, nil); err == nil {
		t.Error("a changed archive was read")
	}
}

type countingWriter struct {
	http.ResponseWriter
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n += int64(n)
	return n, err
}

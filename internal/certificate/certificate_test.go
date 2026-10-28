package certificate

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"clonoloom/internal/artifacts"
)

func fixture(t testing.TB) ([]byte, Record) {
	t.Helper()
	data := []byte(`{"value":1.2345678901234567,"missing":null,"zero":0}`)
	m := artifacts.Manifest{Format: "clonoloom-artifact", Version: 1, ID: "example-1", Kind: "example", State: artifacts.Complete, Lineage: artifacts.Lineage{Job: &artifacts.Job{Kind: "example", Spec: json.RawMessage(`{"seed":42,"source":"/never/read/this"}`)}}, Files: []artifacts.File{{Path: "result.json", Role: "result", Size: int64(len(data)), SHA256: sum(data)}}}
	raw, _ := json.Marshal(m)
	var out bytes.Buffer
	r, err := Build(context.Background(), &out, raw, []byte("Artificial data; test notice.\n"), artifacts.Software{Name: "clonoloom", Version: "test"}, func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil })
	if err != nil {
		t.Fatal(err)
	}
	return out.Bytes(), r
}

func rewrite(t *testing.T, b []byte, edit func(map[string][]byte), extra string) []byte {
	t.Helper()
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		r, _ := f.Open()
		files[f.Name], e = io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	edit(files)
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for name, data := range files {
		f, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		f.Write(data)
	}
	if extra != "" {
		f, _ := w.Create(extra)
		f.Write([]byte("extra"))
	}
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}

func TestCertificateExactBytesAndIndependentFingerprint(t *testing.T) {
	b, r := fixture(t)
	other, r2 := fixture(t)
	if !bytes.Equal(b, other) || r.Fingerprint != r2.Fingerprint {
		t.Fatal("certificate not deterministic")
	}
	// Pinned after independent Python zipfile/json/hashlib validation.
	if r.Fingerprint != "c369717caf5734101a945a6486383c89edd040bade10c1c407b242fe7e61e253" {
		t.Fatal("record encoding changed", r.Fingerprint)
	}
	got := Verify(context.Background(), bytes.NewReader(b), int64(len(b)), r.Fingerprint)
	if got.Verdict != "integrity-match" || got.Recomputed || got.FilesChecked != 4 || got.Fingerprint != r.Fingerprint {
		t.Fatal(got)
	}
	// The independent standard-library Python oracle checks ZIP bytes, source recipe,
	// JSON numeric tokens and the fingerprint; write only on an explicit local request.
	if path := os.Getenv("CLONOLOOM_CERTIFICATE_ORACLE_OUT"); path != "" {
		if e := os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestCertificateRefusesChangedOrAmbiguousArchives(t *testing.T) {
	b, r := fixture(t)
	cases := map[string]func(map[string][]byte){
		"result":    func(f map[string][]byte) { f["artifact/result.json"] = []byte(`{"value":2}`) },
		"notice":    func(f map[string][]byte) { f["NOTICE.txt"] = []byte("different provider") },
		"manifest":  func(f map[string][]byte) { f["manifest.json"] = append(f["manifest.json"], ' ') },
		"missing":   func(f map[string][]byte) { delete(f, "artifact/result.json") },
		"unlisted":  func(f map[string][]byte) { f["unlisted"] = []byte("x") },
		"traversal": func(f map[string][]byte) { f["../escape"] = []byte("x") },
		"absolute":  func(f map[string][]byte) { f["/escape"] = []byte("x") },
		"backslash": func(f map[string][]byte) { f[`artifact\escape`] = []byte("x") },
		"drive":     func(f map[string][]byte) { f["C:/escape"] = []byte("x") },
		"version": func(f map[string][]byte) {
			f["certificate.json"] = bytes.Replace(f["certificate.json"], []byte(`"version":1`), []byte(`"version":2`), 1)
		},
		"duplicate-key": func(f map[string][]byte) {
			f["certificate.json"] = bytes.Replace(f["certificate.json"], []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
		},
		"trailing-json": func(f map[string][]byte) { f["certificate.json"] = append(f["certificate.json"], []byte(`{}`)...) },
		"unknown-field": func(f map[string][]byte) {
			f["certificate.json"] = append([]byte(`{"unknown":1,`), f["certificate.json"][1:]...)
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			data := rewrite(t, b, edit, "")
			got := Verify(context.Background(), bytes.NewReader(data), int64(len(data)), "")
			if got.Verdict == "integrity-match" || got.Recomputed || len(got.Findings) == 0 {
				t.Fatal(got)
			}
		})
	}
	duplicate := rewrite(t, b, func(map[string][]byte) {}, "NOTICE.txt")
	if Verify(context.Background(), bytes.NewReader(duplicate), int64(len(duplicate)), "").Verdict == "integrity-match" {
		t.Fatal("accepted duplicate ZIP member")
	}
	if Verify(context.Background(), bytes.NewReader(b), int64(len(b)), strings.Repeat("0", 64)).Verdict == "integrity-match" {
		t.Fatal("accepted wrong external fingerprint", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Verify(ctx, bytes.NewReader(b), int64(len(b)), "").Verdict == "integrity-match" {
		t.Fatal("ignored cancellation")
	}
}

func TestCertificateManifestLimitsAndChangedInput(t *testing.T) {
	m := artifacts.Manifest{Format: "clonoloom-artifact", Version: 1, ID: "test", Kind: "example", State: artifacts.Complete, Lineage: artifacts.Lineage{Job: &artifacts.Job{Spec: json.RawMessage(`{}`)}}, Files: []artifacts.File{{Path: "file", Size: 1, SHA256: sum([]byte("x"))}}}
	for _, edit := range []func(*artifacts.Manifest){
		func(m *artifacts.Manifest) { m.State = artifacts.Invalid },
		func(m *artifacts.Manifest) { m.Files[0].Location = artifacts.InSource },
		func(m *artifacts.Manifest) { m.Files[0].Path = "../source" },
		func(m *artifacts.Manifest) { m.Files[0].Path = strings.Repeat("x", 1024-len("artifact/")+1) },
		func(m *artifacts.Manifest) { m.Files[0].Size = MaxFileBytes + 1 },
		func(m *artifacts.Manifest) { m.Files[0].SHA256 = "bad" },
	} {
		copy := m
		copy.Files = append([]artifacts.File(nil), m.Files...)
		edit(&copy)
		raw, _ := json.Marshal(copy)
		if _, e := Manifest(raw); e == nil {
			t.Fatal("accepted invalid manifest", copy)
		}
	}
	raw, _ := json.Marshal(m)
	var out bytes.Buffer
	if _, e := Build(context.Background(), &out, raw, []byte("notice"), artifacts.Software{}, func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("y")), nil }); e == nil {
		t.Fatal("accepted changed input bytes")
	}
	if got := Verify(context.Background(), bytes.NewReader(nil), MaxBytes+(9<<20), ""); got.Verdict == "integrity-match" {
		t.Fatal("accepted oversized container")
	}
}

func TestCertificateRejectsLinksAndDeclaredExpansion(t *testing.T) {
	valid, _ := fixture(t)
	for _, mode := range []string{"symlink", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			z, e := zip.NewReader(bytes.NewReader(valid), int64(len(valid)))
			if e != nil {
				t.Fatal(e)
			}
			var out bytes.Buffer
			w := zip.NewWriter(&out)
			for _, f := range z.File {
				src, e := f.Open()
				if e != nil {
					t.Fatal(e)
				}
				data, e := io.ReadAll(src)
				src.Close()
				if e != nil {
					t.Fatal(e)
				}
				header := zip.FileHeader{Name: f.Name, Method: zip.Store}
				if f.Name == "artifact/result.json" && mode == "symlink" {
					header.SetMode(os.ModeSymlink | 0600)
				}
				var dst io.Writer
				if f.Name == "artifact/result.json" && mode == "oversize" {
					header.UncompressedSize64 = uint64(MaxFileBytes) + 1
					header.CompressedSize64 = uint64(len(data))
					dst, e = w.CreateRaw(&header)
				} else {
					dst, e = w.CreateHeader(&header)
				}
				if e != nil {
					t.Fatal(e)
				}
				if _, e = dst.Write(data); e != nil {
					t.Fatal(e)
				}
			}
			if e = w.Close(); e != nil {
				t.Fatal(e)
			}
			got := Verify(context.Background(), bytes.NewReader(out.Bytes()), int64(out.Len()), "")
			if got.Verdict == "integrity-match" || !strings.Contains(strings.Join(got.Findings, " "), "unsafe, duplicate or oversized") {
				t.Fatal(got)
			}
		})
	}
}

func TestCertificateInventoryLimitExcludesRecord(t *testing.T) {
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for name, size := range map[string]uint64{"certificate.json": 1, "a": uint64(MaxFileBytes), "b": uint64(MaxFileBytes), "c": 1} {
		if _, err := w.CreateRaw(&zip.FileHeader{Name: name, Method: zip.Store, UncompressedSize64: size}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got := Verify(context.Background(), bytes.NewReader(out.Bytes()), int64(out.Len()), "")
	if got.Verdict == "integrity-match" || !strings.Contains(strings.Join(got.Findings, " "), "inventory exceeds 512 MiB") {
		t.Fatal(got)
	}
}

func FuzzCertificateIntegrity(f *testing.F) {
	valid, _ := fixture(f)
	f.Add(valid)
	f.Add([]byte("not a zip"))
	f.Add([]byte("PK\x05\x06"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 2<<20 {
			t.Skip()
		}
		got := Verify(context.Background(), bytes.NewReader(raw), int64(len(raw)), "")
		if got.Recomputed {
			t.Fatal("integrity check claims recomputation")
		}
	})
}

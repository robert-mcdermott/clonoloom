package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
	"clonoloom/internal/export"
)

func certificateFixture(t *testing.T, a *app, kind string, selected ...string) string {
	t.Helper()
	dir := "validation/projects/0.7.0"
	raw, e := os.ReadFile(filepath.Join(dir, "project.json"))
	if e != nil {
		t.Fatal(e)
	}
	var p struct{ Artifacts []struct{ ID, Kind string } }
	if e = json.Unmarshal(raw, &p); e != nil {
		t.Fatal(e)
	}
	for _, r := range p.Artifacts {
		if r.Kind != kind {
			continue
		}
		if len(selected) > 0 && r.ID != selected[0] {
			continue
		}
		base := filepath.Join(dir, "artifacts", r.ID)
		raw = gunzipFile(t, filepath.Join(base, "manifest.json.gz"))
		var m artifacts.Manifest
		if e = json.Unmarshal(raw, &m); e != nil {
			t.Fatal(e)
		}
		target := filepath.Join(a.artifacts.Dir(), r.ID)
		if e = os.MkdirAll(target, 0700); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(target, "manifest.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		for _, f := range m.Files {
			if e = os.WriteFile(filepath.Join(target, f.Path), gunzipFile(t, filepath.Join(base, f.Path+".gz")), 0600); e != nil {
				t.Fatal(e)
			}
		}
		return r.ID
	}
	t.Fatal("missing release fixture", kind)
	return ""
}

func TestCertificateAPIAndSourceFreeIntegrity(t *testing.T) {
	a, h := testApp(t)
	for _, kind := range []string{cohortFeaturesKind, cohortCompareKind, cohortCountsKind} {
		t.Run(kind, func(t *testing.T) {
			id := certificateFixture(t, a, kind)
			meta := request(t, h, http.MethodGet, "/api/certificates/"+id, nil, nil)
			if meta.Code != 200 {
				t.Fatal(meta.Code, meta.Body)
			}
			var record struct{ Certificate certificate.Record }
			if e := json.Unmarshal(meta.Body.Bytes(), &record); e != nil {
				t.Fatal(e)
			}
			got := request(t, h, http.MethodGet, "/api/certificates/"+id+"/download", nil, nil)
			if got.Code != 200 || got.Header().Get("X-ClonoLoom-Certificate") != record.Certificate.Fingerprint {
				t.Fatal(got.Code, got.Body)
			}
			// Remove the saved result too: the received certificate is now the only evidence.
			if e := os.RemoveAll(filepath.Join(a.artifacts.Dir(), id)); e != nil {
				t.Fatal(e)
			}
			check := request(t, h, http.MethodPost, "/api/certificates/verify?expect="+record.Certificate.Fingerprint, bytes.NewReader(got.Body.Bytes()), map[string]string{"Content-Type": "application/zip"})
			var report certificate.Report
			if e := json.Unmarshal(check.Body.Bytes(), &report); e != nil || check.Code != 200 || report.Verdict != "integrity-match" || report.Recomputed {
				t.Fatal(check.Code, check.Body, e)
			}
			if report.FilesChecked != len(record.Certificate.Files) {
				t.Fatal("incomplete inventory")
			}
			// The terminal reads the same bytes without a server, browser or library.
			path := filepath.Join(t.TempDir(), "result.certificate.clz")
			os.WriteFile(path, got.Body.Bytes(), 0600)
			var stdout, stderr bytes.Buffer
			out := filepath.Join(t.TempDir(), "report.json")
			if code := runVerify([]string{"--integrity", "--expect", record.Certificate.Fingerprint, "--report", out, path}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "integrity-match") || !strings.Contains(stdout.String(), "not independently verified") {
				t.Fatal(code, stdout.String(), stderr.String())
			}
			b, _ := os.ReadFile(out)
			var cli certificate.Report
			json.Unmarshal(b, &cli)
			if string(mustJSON(cli)) != string(mustJSON(report)) {
				t.Fatal("CLI/API reports differ")
			}
			if code := runVerify([]string{"--integrity", "--report", out, path}, &stdout, &stderr); code != 2 {
				t.Fatal("overwrote report", code)
			}
			if again, _ := os.ReadFile(out); !bytes.Equal(b, again) {
				t.Fatal("existing report changed")
			}
		})
	}
}

func TestCertificateProviderGateAndTampering(t *testing.T) {
	a, h := testApp(t)
	id := certificateFixture(t, a, cohortFeaturesKind)
	path := filepath.Join(a.artifacts.Dir(), id, cohortFeaturesKind+".json")
	raw, _ := os.ReadFile(path)
	var doc cohortFeaturesDocument
	if e := json.Unmarshal(raw, &doc); e != nil {
		t.Fatal(e)
	}
	doc.Notice.Credits = []export.Credit{{Title: "Artificial terms control", LicenseClass: "terms", License: "Private study only", Citation: "Fixture citation"}}
	raw = mustJSON(doc)
	os.WriteFile(path, raw, 0600)
	m, _ := a.artifacts.Get(id)
	hash := sha256.Sum256(raw)
	for i := range m.Files {
		if m.Files[i].Path == cohortFeaturesKind+".json" {
			m.Files[i].Size = int64(len(raw))
			m.Files[i].SHA256 = hex.EncodeToString(hash[:])
		}
	}
	os.WriteFile(filepath.Join(a.artifacts.Dir(), id, "manifest.json"), mustJSON(m), 0600)
	for _, suffix := range []string{"", "/download"} {
		r := request(t, h, http.MethodGet, "/api/certificates/"+id+suffix, nil, nil)
		if r.Code != 409 || !strings.Contains(r.Body.String(), `"code":"restricted"`) {
			t.Fatal(r.Code, r.Body)
		}
	}
	r := request(t, h, http.MethodGet, "/api/certificates/"+id+"/download?accept=1", nil, nil)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	// Stored bytes changed without their manifest: preparation must not bless them.
	os.WriteFile(path, append(raw, ' '), 0600)
	r = request(t, h, http.MethodGet, "/api/certificates/"+id+"?accept=1", nil, nil)
	if r.Code != 409 || !strings.Contains(r.Body.String(), "checksum") {
		t.Fatal(r.Code, r.Body)
	}
	// An unlisted source path in a forged manifest is refused before it can be opened.
	m.Files[0].Location = artifacts.InPlace
	m.Files[0].Path = "/must/not/read"
	os.WriteFile(filepath.Join(a.artifacts.Dir(), id, "manifest.json"), mustJSON(m), 0600)
	r = request(t, h, http.MethodGet, "/api/certificates/"+id+"?accept=1", nil, nil)
	if r.Code != 409 || !strings.Contains(r.Body.String(), "unsupported artifact file") {
		t.Fatal(r.Code, r.Body)
	}
}

func TestVerifyUsageAndFailureCodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.clz")
	os.WriteFile(path, []byte("invalid"), 0600)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, 0}, {nil, 2}, {[]string{path}, 2}, {[]string{"--integrity", path}, 1},
		{[]string{"--integrity", path + "missing"}, 2}, {[]string{"--integrity", "--expect", "wrong", path}, 2},
		{[]string{"--integrity", "--timeout", "0s", path}, 2}, {[]string{"--integrity", path, path}, 2},
		{[]string{"--integrity", "--replay", path}, 2}, {[]string{"--replay", "--timeout", "1ns", path}, 2},
	} {
		var out, err bytes.Buffer
		if got := runVerify(tc.args, &out, &err); got != tc.code {
			t.Fatal(tc.args, got, out.String(), err.String())
		}
	}
}

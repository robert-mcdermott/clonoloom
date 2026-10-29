package adc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func downloadSpec() DownloadSpec {
	return DownloadSpec{Repository: "ipa1.ireceptor.org", Repertoire: "R", Processing: "P", License: "Simulated provider permission; fixture only", Citation: "Artificial test study", Pages: 3, Size: 2, Accept: true}
}

const downloadMetadata = `{"Repertoire":[{"repertoire_id":"R","subject":{"subject_id":"S"},"data_processing":[{"data_processing_id":"P"}]}]}`
const downloadRow = `{"sequence_id":"a","repertoire_id":"R","data_processing_id":"P","duplicate_count":9007199254740993,"custom":1.234567890123456789,"productive":false}`

func TestDownloadExactCaptureAndBudgets(t *testing.T) {
	calls := 0
	c := Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if strings.HasSuffix(r.URL.Path, "/repertoire") {
			return response(200, downloadMetadata), nil
		}
		if !strings.Contains(string(body), `"field":"data_processing_id","value":"P"`) {
			t.Fatal(string(body))
		}
		if calls == 2 {
			return response(200, `{"Rearrangement":[`+downloadRow+`]}`), nil
		}
		if !strings.Contains(string(body), `"from":1`) {
			t.Fatal(string(body))
		}
		return response(200, `{"Rearrangement":[]}`), nil
	})}
	parent := t.TempDir()
	if auditParent := os.Getenv("CLONOLOOM_ADC_AUDIT_DIR"); auditParent != "" {
		parent = auditParent
	}
	out, err := c.Download(context.Background(), parent, downloadSpec())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || out.Manifest.Stop != "empty-page" || out.Manifest.Completeness != "not-established" || out.Manifest.Records != 1 {
		t.Fatalf("%+v", out)
	}
	for _, p := range out.Manifest.Pages {
		b, e := os.ReadFile(filepath.Join(out.Path, p.File))
		if e != nil || digest(b) != p.SHA256 || len(b) != p.Bytes || digest([]byte(p.RequestJSON)) != p.RequestSHA256 {
			t.Fatalf("bad evidence %+v %v", p, e)
		}
	}
	b, _ := os.ReadFile(filepath.Join(out.Path, "page-001.json"))
	if string(b) != `{"Rearrangement":[`+downloadRow+`]}` {
		t.Fatal("numbers changed")
	}
	b, _ = os.ReadFile(filepath.Join(out.Path, "manifest.json"))
	var m DownloadManifest
	if json.Unmarshal(b, &m) != nil || m.Spec.License != downloadSpec().License {
		t.Fatal("manifest lost permission evidence")
	}
	calls = 0
	s := downloadSpec()
	s.Pages = 1
	other, err := c.Download(context.Background(), parent, s)
	if err != nil || other.Path == out.Path || other.Manifest.Stop != "page-budget" {
		t.Fatalf("overwrite/budget %+v %v", other, err)
	}
}
func TestDownloadRefusalsAndCleanup(t *testing.T) {
	for _, bad := range []string{`{"Rearrangement":[` + downloadRow + `,` + downloadRow + `]}`, strings.Replace(`{"Rearrangement":[`+downloadRow+`]}`, `"P"`, `"other"`, 1), `{"Rearrangement":[null]}`, `{"Rearrangement":null}`, `{"Rearrangement":[],"Rearrangement":[]}`} {
		parent := t.TempDir()
		c := Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
			if strings.HasSuffix(r.URL.Path, "/repertoire") {
				return response(200, downloadMetadata), nil
			}
			return response(200, bad), nil
		})}
		if _, err := c.Download(context.Background(), parent, downloadSpec()); err == nil {
			t.Fatal("accepted", bad)
		}
		entries, _ := os.ReadDir(parent)
		if len(entries) != 0 {
			t.Fatal("failed capture left files")
		}
	}
	calls := 0
	c := Client{Transport: transport(func(*http.Request) (*http.Response, error) { calls++; return response(500, ""), nil })}
	parent := t.TempDir()
	for _, edit := range []func(*DownloadSpec){func(s *DownloadSpec) { s.Accept = false }, func(s *DownloadSpec) { s.License = "" }, func(s *DownloadSpec) { s.Pages = 11 }, func(s *DownloadSpec) { s.Size = 0 }, func(s *DownloadSpec) { s.Processing = "" }, func(s *DownloadSpec) { s.Repository = "evil.example" }} {
		s := downloadSpec()
		edit(&s)
		if _, e := c.Download(context.Background(), parent, s); e == nil {
			t.Fatal("invalid accepted")
		}
	}
	c.Offline = true
	if _, e := c.Download(context.Background(), parent, downloadSpec()); e != ErrOffline {
		t.Fatal(e)
	}
	c.Offline = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := c.Download(ctx, parent, downloadSpec()); e == nil {
		t.Fatal("cancel ignored")
	}
	if calls != 0 {
		t.Fatal("unapproved outbound request")
	}
	if _, e := c.Download(context.Background(), parent, downloadSpec()); e == nil {
		t.Fatal("server error ignored")
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 0 {
		t.Fatal("staging not removed")
	}
}
func TestDownloadRepeatedPagesAndChangedMetadata(t *testing.T) {
	for _, metadata := range []string{downloadMetadata, strings.Replace(downloadMetadata, `{"data_processing_id":"P"}`, `{"data_processing_id":"P"},{"data_processing_id":"P"}`, 1), strings.Replace(downloadMetadata, `"P"`, `"other"`, 1), strings.Replace(downloadMetadata, `"R"`, `"other"`, 1)} {
		c := Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
			if strings.HasSuffix(r.URL.Path, "/repertoire") {
				return response(200, metadata), nil
			}
			return response(200, `{"Rearrangement":[`+downloadRow+`]}`), nil
		})}
		parent := t.TempDir()
		if _, e := c.Download(context.Background(), parent, downloadSpec()); e == nil {
			t.Fatal("unstable scope/pages accepted")
		}
		entries, _ := os.ReadDir(parent)
		if len(entries) != 0 {
			t.Fatal("incomplete directory remains")
		}
	}
}

func TestDownloadCancellationAfterMetadata(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/repertoire") {
			return response(200, downloadMetadata), nil
		}
		cancel()
		return nil, ctx.Err()
	})}
	parent := t.TempDir()
	if _, err := c.Download(ctx, parent, downloadSpec()); err == nil {
		t.Fatal("cancellation ignored")
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 0 {
		t.Fatal("partial evidence left after cancellation")
	}
}

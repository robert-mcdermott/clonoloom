package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestADCDownloadReviewAndInstallation(t *testing.T) {
	a, h := testApp(t)
	calls := 0
	a.adc.Transport = adcTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"Rearrangement":[]}`
		if strings.HasSuffix(r.URL.Path, "/repertoire") {
			body = `{"Repertoire":[{"repertoire_id":"R","data_processing":[{"data_processing_id":"P"}]}]}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	body := `{"repository":"ipa1.ireceptor.org","repertoire":"R","processing":"P","license":"Fixture permission","citation":"Simulated study","size":10,"pages":1,"accept":true}`
	if w := postJSON(t, h, "/api/adc/download/preview", body); w.Code != 200 || !strings.Contains(w.Body.String(), "maxBytes") {
		t.Fatal(w.Code, w.Body)
	}
	if calls != 0 {
		t.Fatal("preview contacted provider")
	}
	if w := postJSON(t, h, "/api/adc/download", strings.Replace(body, `"accept":true`, `"accept":false`, 1)); w.Code == 200 {
		t.Fatal("no consent")
	}
	a.adc.Offline = true
	if w := postJSON(t, h, "/api/adc/download", body); w.Code != 403 {
		t.Fatal(w.Code)
	}
	a.adc.Offline = false
	r := httptest.NewRequest("POST", "/api/adc/download", strings.NewReader(body))
	r.Host = "127.0.0.1:8860"
	r.RemoteAddr = "203.0.113.1:1111"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	if calls != 0 {
		t.Fatal("refused request contacted provider")
	}
	if w := postJSON(t, h, "/api/adc/download", body); w.Code != 200 || !strings.Contains(w.Body.String(), `"completeness":"not-established"`) {
		t.Fatal(w.Code, w.Body)
	}
	if calls != 2 || len(a.datasets.byID) != 0 {
		t.Fatal("unexpected request or analysis installation")
	}
	entries, _ := os.ReadDir(a.store.dir)
	captures := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "adc-download-") {
			captures++
		}
	}
	if captures != 1 {
		t.Fatal("capture not installed")
	}
}

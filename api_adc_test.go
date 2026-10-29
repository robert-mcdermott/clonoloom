package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"clonoloom/internal/adc"
)

type adcTransport func(*http.Request) (*http.Response, error)

func (f adcTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestADCLocalReviewOfflineAndExactAPI(t *testing.T) {
	a, handler := testApp(t)
	calls := 0
	fixture, err := os.ReadFile("internal/adc/testdata/simulated-page.json")
	if err != nil {
		t.Fatal(err)
	}
	a.adc.Transport = adcTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "dataset") || strings.Contains(string(body), "junction") {
			t.Fatal("local data sent")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(fixture)))}, nil
	})
	base := `{"query":{"repository":"ipa1.ireceptor.org","species":"NCBITAXON:9606","locus":"TRB","study":"SIM-longitudinal-TRB","from":0,"size":20}`
	confirmed := base + `,"confirm":true}`
	if rec := request(t, handler, "GET", "/api/adc/repositories", nil, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), adc.RegistryRevision) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := postJSON(t, handler, "/api/adc/prepare", base+`}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), "/airr/v1/repertoire") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, op := range []string{"query", "info"} {
		if rec := postJSON(t, handler, "/api/adc/"+op, base+`}`); rec.Code != 403 {
			t.Fatalf("no confirmation: %d", rec.Code)
		}
	}
	for _, body := range []string{`{`, base + `,"confirm":true,"files":["/local/data"]}`, strings.Replace(confirmed, `"size":20`, `"size":20,"size":1`, 1), strings.Replace(confirmed, `"study":"SIM-longitudinal-TRB"`, `"study":"\ud800"`, 1), strings.Repeat(" ", 8193), strings.Replace(confirmed, `"size":20`, `"size":0`, 1)} {
		if rec := postJSON(t, handler, "/api/adc/query", body); rec.Code != 400 {
			t.Fatalf("malformed: %d %s", rec.Code, rec.Body)
		}
	}
	for _, host := range []string{"127.0.0.1:8860", "unapproved.example:8860"} {
		r := httptest.NewRequest("POST", "/api/adc/query", strings.NewReader(confirmed))
		r.Host = host
		r.RemoteAddr = "203.0.113.2:5555"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("remote: %d", w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/api/adc/query", strings.NewReader(confirmed))
	r.Host = "127.0.0.1:8860"
	r.RemoteAddr = "127.0.0.1:5555"
	r.Header.Set("Origin", "https://unapproved.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross origin: %d", w.Code)
	}
	a.adc.Offline = true
	for _, op := range []string{"query", "info"} {
		if rec := postJSON(t, handler, "/api/adc/"+op, confirmed); rec.Code != 403 || !strings.Contains(rec.Body.String(), "offline") {
			t.Fatalf("offline: %d %s", rec.Code, rec.Body)
		}
	}
	if calls != 0 {
		t.Fatalf("unconfirmed/refused request contacted repository %d times", calls)
	}
	a.adc.Offline = false
	rec := postJSON(t, handler, "/api/adc/query", confirmed)
	var page adc.Page
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &page) != nil || page.Evidence.ResponseJSON != string(fixture) || len(page.Records) != 3 || page.Completeness != "not-established" || page.NextOffset == nil || *page.NextOffset != 3 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if calls != 1 {
		t.Fatal("automatic pagination")
	}
	if len(a.datasets.byID) != 0 {
		t.Fatal("query installed a dataset")
	}
}

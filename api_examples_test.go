package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/examples"
)

type exampleList struct {
	Dir      string   `json:"dir"`
	Offline  bool     `json:"offline"`
	Hosts    []string `json:"hosts"`
	Examples []struct {
		Name         string     `json:"name"`
		LicenseClass string     `json:"licenseClass"`
		State        string     `json:"state"`
		Paths        [][]string `json:"paths"`
		Download     *download  `json:"download"`
	} `json:"examples"`
}

func listExamples(t *testing.T, handler http.Handler) exampleList {
	t.Helper()
	var list exampleList
	json.Unmarshal(request(t, handler, http.MethodGet, "/api/examples", nil, nil).Body.Bytes(), &list)
	return list
}

// The recorded examples are listed; an example is downloaded only from this computer, with its
// terms accepted where it has them, never offline; it opens as a dataset that carries its source,
// license and citation.
func TestExamplesDownloadAndOpenWithTheirCredits(t *testing.T) {
	a, handler := testApp(t)
	list := listExamples(t, handler)
	if len(list.Examples) != 6 || list.Offline || len(list.Hosts) != 3 {
		t.Fatalf("%+v", list)
	}
	terms := ""
	for _, e := range list.Examples {
		if e.LicenseClass == "terms" {
			terms = e.Name
		}
	}
	if rec := postJSON(t, handler, "/api/examples/"+terms, `{}`); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"terms"`) {
		t.Errorf("terms not accepted: %d %s", rec.Code, rec.Body)
	}
	if rec := postJSON(t, handler, "/api/examples/nothing", `{}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", rec.Code)
	}

	data := []byte(datasetTSV)
	sum := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer server.Close()
	a.examples.Sources = []examples.Source{{Name: "tiny", Title: "A tiny study", Source: "A test", License: "CC BY 4.0", LicenseClass: "open",
		Citation: "Someone. A tiny study. 2026.", Base: server.URL + "/",
		Files: []examples.File{{Path: "study.tsv", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}},
		Opens: []examples.Opening{{Title: "All", Name: "Tiny", Files: []string{"study.tsv"}}}}}
	if rec := postJSON(t, handler, "/api/examples/tiny", ``); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		list = listExamples(t, handler)
		if list.Examples[0].State == "ready" || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	e := list.Examples[0]
	if e.State != "ready" || len(e.Paths) != 1 {
		t.Fatalf("%+v", e)
	}
	rec := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(e.Paths[0][0])+`]}`)
	var s datasetSummary
	json.Unmarshal(rec.Body.Bytes(), &s)
	if len(s.Credits) != 1 || s.Credits[0].Example != "tiny" || s.Credits[0].Citation != "Someone. A tiny study. 2026." || s.Credits[0].License != "CC BY 4.0" {
		t.Fatalf("credits %+v", s.Credits)
	}
	// Removed only from this computer and when not downloading.
	del := httptest.NewRequest(http.MethodDelete, "/api/examples/tiny", nil)
	del.Host, del.RemoteAddr = "127.0.0.1:8860", "127.0.0.1:5555"
	del.Header.Set("Sec-Fetch-Site", "same-origin")
	removed := httptest.NewRecorder()
	handler.ServeHTTP(removed, del)
	if removed.Code != http.StatusOK {
		t.Errorf("remove: %d %s", removed.Code, removed.Body)
	}
	if listExamples(t, handler).Examples[0].State != "absent" {
		t.Error("not removed")
	}

	a.examples.Offline = true
	if rec := postJSON(t, handler, "/api/examples/tiny", `{}`); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"offline"`) {
		t.Errorf("offline: %d %s", rec.Code, rec.Body)
	}
}

func TestExamplesAreDownloadedOnlyFromThisComputer(t *testing.T) {
	_, handler := testApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/examples/10x-pbmc-1k", strings.NewReader(`{}`))
	req.Host = "127.0.0.1:8860"
	req.RemoteAddr = "192.0.2.1:5555"
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("%d", rec.Code)
	}
}

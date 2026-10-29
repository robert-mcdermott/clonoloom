package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func handoffFixture(t *testing.T) (*app, http.Handler, string, string) {
	t.Helper()
	raw, e := os.ReadFile("testdata/proteoscope/simulated-airr.tsv")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "artificial.tsv")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	a, h := testApp(t)
	w := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": []string{path}})))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var s datasetSummary
	json.Unmarshal(w.Body.Bytes(), &s)
	return a, h, s.ID, path
}

type reviewedHandoff struct {
	Review      string          `json:"review"`
	Fingerprint string          `json:"fingerprint"`
	Evidence    handoffEvidence `json:"evidence"`
}

func reviewHandoff(t *testing.T, h http.Handler, id string, row, port int) (handoffRequest, reviewedHandoff) {
	t.Helper()
	req := handoffRequest{Dataset: id, Row: &row, Port: port}
	w := postJSON(t, h, "/api/proteoscope/review", string(mustJSON(req)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var result reviewedHandoff
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	req.Review, req.Fingerprint = result.Review, result.Fingerprint
	return req, result
}
func handoffStandIn(t *testing.T, fn http.HandlerFunc) (*httptest.Server, int) {
	t.Helper()
	s := httptest.NewServer(fn)
	t.Cleanup(s.Close)
	u, _ := url.Parse(s.URL)
	p, _ := strconv.Atoi(u.Port())
	return s, p
}
func standInIdentity(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/":
		w.Write([]byte("<title>Proteoscope</title>"))
	case "/api/startup":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"0.10.0","offline":false,"remoteControl":true,"files":[]}`))
	default:
		return false
	}
	return true
}
func TestProteoscopeExactHandoff(t *testing.T) {
	a, h, id, _ := handoffFixture(t)
	ds, _ := a.datasets.get(id)
	ds.Credits = []credit{{Title: "Artificial permission control", License: "Review required", LicenseClass: "terms", Citation: "Artificial data, no biological claim"}}
	var calls atomic.Int32
	var command []byte
	const exactReply = " \n{\"ok\":true, \"message\":\"Artificial search intercepted\",\"data\":{\"sourceValue\":9007199254740993}}\n"
	_, port := handoffStandIn(t, func(w http.ResponseWriter, r *http.Request) {
		if standInIdentity(w, r) {
			return
		}
		if r.URL.Path != "/api/remote/command" || r.Method != "POST" {
			t.Error("unexpected destination", r.URL)
			return
		}
		calls.Add(1)
		var b bytes.Buffer
		b.ReadFrom(r.Body)
		command = b.Bytes()
		w.Write([]byte(exactReply))
	})
	req, review := reviewHandoff(t, h, id, 0, port)
	if !review.Evidence.Eligible || !review.Evidence.Destination.Ready || review.Evidence.SequenceID != "00123" || review.Evidence.Fields["exact_extension"] != "9007199254740993" {
		t.Fatal(review)
	}
	if calls.Load() != 0 {
		t.Fatal("preview sent sequence")
	}
	for _, perm := range [][3]bool{{false, false, false}, {true, false, true}, {false, true, true}, {true, true, false}} {
		req.AcceptLocal, req.AcceptPublic, req.AcceptTerms = perm[0], perm[1], perm[2]
		w := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req)))
		if w.Code != 403 || calls.Load() != 0 {
			t.Fatal("permission boundary", w.Code, w.Body)
		}
	}
	req.AcceptLocal, req.AcceptPublic, req.AcceptTerms = true, true, true
	evidence := postJSON(t, h, "/api/proteoscope/export", string(mustJSON(req)))
	if evidence.Code != 200 {
		t.Fatal(evidence.Body)
	}
	w := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req)))
	if w.Code != 200 || calls.Load() != 1 {
		t.Fatal(w.Code, w.Body)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("9007199254740993")) {
		t.Fatal("receipt rounded")
	}
	var receipt struct {
		ResponseText   string `json:"responseText"`
		ResponseSHA256 string `json:"responseSHA256"`
	}
	json.Unmarshal(w.Body.Bytes(), &receipt)
	if receipt.ResponseText != exactReply || receipt.ResponseSHA256 != replayDigest([]byte(exactReply)) {
		t.Fatal("receiver bytes changed")
	}
	var sent map[string]string
	json.Unmarshal(command, &sent)
	if len(sent) != 1 || sent["command"] != "search "+review.Evidence.Fields["sequence_aa"] || strings.Contains(string(command), "00123") {
		t.Fatal("query changed or identifiers disclosed", sent)
	}
	again := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req)))
	if again.Code != 409 || calls.Load() != 1 {
		t.Fatal("repeated transfer")
	}
	if dir := os.Getenv("CLONOLOOM_HANDOFF_OUT"); dir != "" {
		os.MkdirAll(dir, 0700)
		for name, b := range map[string][]byte{"handoff.json": evidence.Body.Bytes(), "receipt.json": w.Body.Bytes(), "command.json": command} {
			if e := os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	req, partial := reviewHandoff(t, h, id, 1, port)
	if !partial.Evidence.Eligible || partial.Evidence.Fields["complete_vdj"] != "F" || partial.Evidence.SequenceID != "=query?alpha&beta" {
		t.Fatal("partial evidence changed")
	}
	for i := 2; i < 6; i++ {
		_, bad := reviewHandoff(t, h, id, i, port)
		if bad.Evidence.Eligible || bad.Evidence.Command != "" || bad.Evidence.Destination.Ready {
			t.Fatal("repaired unavailable evidence", i)
		}
	}
}
func TestProteoscopeRefusals(t *testing.T) {
	a, h, id, path := handoffFixture(t)
	var calls atomic.Int32
	_, port := handoffStandIn(t, func(w http.ResponseWriter, r *http.Request) {
		if standInIdentity(w, r) {
			return
		}
		calls.Add(1)
		w.WriteHeader(503)
	})
	req, _ := reviewHandoff(t, h, id, 0, port)
	req.AcceptLocal, req.AcceptPublic = true, true
	// Failed dispatch still consumes review; outcome is not retried automatically.
	if w := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req))); w.Code != 502 {
		t.Fatal(w.Code, w.Body)
	}
	if w := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req))); w.Code != 409 || calls.Load() != 1 {
		t.Fatal("retry", w.Code)
	}
	req, _ = reviewHandoff(t, h, id, 0, port)
	req.AcceptLocal, req.AcceptPublic = true, true
	os.WriteFile(path, []byte("changed"), 0600)
	if w := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req))); w.Code != 409 || calls.Load() != 1 {
		t.Fatal("changed source sent")
	}
	for _, body := range []string{`{}`, `{"dataset":"` + id + `","row":null,"port":1}`, `{"dataset":"` + id + `","row":0,"port":65536}`, `{"dataset":"` + id + `","row":0,"port":1,"command":"arbitrary"}`} {
		if w := postJSON(t, h, "/api/proteoscope/review", body); w.Code == 200 {
			t.Fatal("bad request accepted")
		}
	}
	w := request(t, h, "POST", "/api/proteoscope/page", strings.NewReader(`{"dataset":"`+id+`"}`), nil)
	if w.Code != 403 {
		t.Fatal("remote peer", w.Code)
	}
	// An offline review must not even probe the destination.
	raw, _ := os.ReadFile("testdata/proteoscope/simulated-airr.tsv")
	os.WriteFile(path, raw, 0600)
	a.adc.Offline = true
	_, offline := reviewHandoff(t, h, id, 0, port)
	if offline.Evidence.Destination.Ready || !strings.Contains(offline.Evidence.Destination.Reason, "ClonoLoom is offline") {
		t.Fatal(offline)
	}
}
func TestProteoscopeDestinationAndConcurrency(t *testing.T) {
	_, h, id, _ := handoffFixture(t)
	var changed atomic.Bool
	var calls atomic.Int32
	_, port := handoffStandIn(t, func(w http.ResponseWriter, r *http.Request) {
		if changed.Load() && r.URL.Path == "/api/startup" {
			w.Write([]byte(`{"version":"0.11.0","remoteControl":true,"offline":false,"files":[]}`))
			return
		}
		if standInIdentity(w, r) {
			return
		}
		calls.Add(1)
		w.Write([]byte(`{"ok":true,"message":"Intercepted"}`))
	})
	req, _ := reviewHandoff(t, h, id, 0, port)
	req.AcceptLocal, req.AcceptPublic = true, true
	changed.Store(true)
	if w := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req))); w.Code != 409 || calls.Load() != 0 {
		t.Fatal("changed recipient")
	}
	changed.Store(false)
	req, _ = reviewHandoff(t, h, id, 0, port)
	req.AcceptLocal, req.AcceptPublic = true, true
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := postJSON(t, h, "/api/proteoscope/send", string(mustJSON(req)))
			if w.Code != 200 && w.Code != 409 {
				t.Error(w.Code, w.Body)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("concurrent dispatch", calls.Load())
	}
	for _, startup := range []string{`{"version":"0.10.0","remoteControl":false,"offline":false,"files":[]}`, `{"version":"0.10.0","remoteControl":true,"offline":true,"files":[]}`, `{"version":"0.11.0","remoteControl":true,"offline":false,"files":[]}`, `{"version":"0.10.0"}`} {
		_, p := handoffStandIn(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Write([]byte("<title>Proteoscope</title>"))
			} else {
				w.Write([]byte(startup))
			}
		})
		if probeHandoff(context.Background(), p).Ready {
			t.Fatal("unsupported destination", startup)
		}
	}
	var redirected atomic.Int32
	target, _ := handoffStandIn(t, func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) })
	_, p := handoffStandIn(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) })
	if probeHandoff(context.Background(), p).Ready || redirected.Load() != 0 {
		t.Fatal("redirect followed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if probeHandoff(ctx, port).Ready {
		t.Fatal("cancelled probe")
	}
}
func TestProteoscopeSourceBounds(t *testing.T) {
	a, _, id, path := handoffFixture(t)
	ds, _ := a.datasets.get(id)
	s := ds.Result.Sources[0]
	raw, _ := os.ReadFile(path)
	raw = append([]byte("\ufeff"), raw...)
	var b bytes.Buffer
	g := gzip.NewWriter(&b)
	g.Write(raw)
	g.Close()
	os.WriteFile(path, b.Bytes(), 0600)
	s.Size = int64(b.Len())
	s.SHA256 = replayDigest(b.Bytes())
	s.Gzipped = true
	header, row, fields, e := handoffSource(context.Background(), s, 2)
	if e != nil || fields["exact_extension"] != "9007199254740993" || !strings.HasPrefix(header, "\ufeff") || row == "" {
		t.Fatal(e)
	}
	s.Gzipped = false
	os.WriteFile(path, raw, 0600)
	s.Size = int64(len(raw))
	s.SHA256 = replayDigest(raw)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, e := handoffSource(ctx, s, 2); e == nil {
		t.Fatal("cancel")
	}
	os.Remove(path)
	os.Symlink("simulated-airr.tsv", path)
	if _, _, _, e := handoffSource(context.Background(), s, 2); e == nil {
		t.Fatal("symlink")
	}
	if len(handoffSequence.FindString(strings.Repeat("A", 25))) != 25 {
		t.Fatal("canonical sequence")
	}
}

func TestProteoscopeReviewBindingAndCapacity(t *testing.T) {
	a, h, id, _ := handoffFixture(t)
	a.adc.Offline = true
	req, review := reviewHandoff(t, h, id, 0, 8765)
	if review.Evidence.Destination.Ready {
		t.Fatal("offline receiver")
	}
	for _, change := range []func(*handoffRequest){
		func(r *handoffRequest) { r.Fingerprint = strings.Repeat("0", 64) },
		func(r *handoffRequest) { r.Review = "unknown" },
		func(r *handoffRequest) { r.Port = 8766 },
		func(r *handoffRequest) { row := 1; r.Row = &row },
	} {
		altered := req
		change(&altered)
		if w := postJSON(t, h, "/api/proteoscope/export", string(mustJSON(altered))); w.Code != 409 {
			t.Fatal("review not bound", w.Code, w.Body)
		}
	}
	if w := postJSON(t, h, "/api/proteoscope/export", string(mustJSON(req))); w.Code != 200 {
		t.Fatal("offline local evidence unavailable", w.Body)
	}
	for i := 1; i < 128; i++ {
		reviewHandoff(t, h, id, 0, 8765)
	}
	if w := postJSON(t, h, "/api/proteoscope/review", string(mustJSON(req))); w.Code != 429 {
		t.Fatal("unbounded reviews", w.Code)
	}
}

func TestProteoscopeSourceRefusals(t *testing.T) {
	a, _, id, path := handoffFixture(t)
	ds, _ := a.datasets.get(id)
	source := ds.Result.Sources[0]
	original, _ := os.ReadFile(path)
	for _, raw := range [][]byte{
		bytes.Replace(original, []byte("exact_extension"), []byte("sequence_id"), 1),
		bytes.Replace(original, []byte("9007199254740993"), []byte("9007199254740993\textra"), 1),
		bytes.Replace(original, []byte("9007199254740993"), []byte{255}, 1),
		bytes.Replace(original, []byte("9007199254740993"), []byte(strings.Repeat("x", 1<<20)), 1),
	} {
		os.WriteFile(path, raw, 0600)
		source.Size = int64(len(raw))
		source.SHA256 = replayDigest(raw)
		if _, _, _, e := handoffSource(context.Background(), source, 2); e == nil {
			t.Fatal("unsupported source accepted")
		}
	}
	os.WriteFile(path, original, 0600)
	source.Size = int64(len(original))
	source.SHA256 = replayDigest(original)
	f, e := os.OpenFile(path, os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	e = f.Truncate((64 << 20) + 1)
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if _, _, _, e := handoffSource(context.Background(), source, 2); e == nil {
		t.Fatal("source size")
	}
}

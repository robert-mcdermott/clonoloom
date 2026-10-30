package adc

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func query() Query {
	return Query{Repository: "ipa1.ireceptor.org", Species: "NCBITAXON:9606", Locus: "TRB", Study: "SIM-longitudinal-TRB", Size: 20}
}
func fixture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/simulated-page.json")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRegistryPinAndApprovedDestinations(t *testing.T) {
	if digest([]byte(registry)) != RegistrySHA256 {
		t.Fatal("registry fingerprint changed without review")
	}
	seen := map[string]bool{}
	for _, r := range Repositories() {
		u, err := url.Parse(r.URL)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Host != r.ID || u.Port() != "" || seen[r.ID] || r.Contact == "" {
			t.Fatalf("invalid repository: %+v", r)
		}
		seen[r.ID] = true
		p, err := Prepare(Query{Repository: r.ID, Size: 1})
		if err != nil || p.Endpoint != r.URL+"/airr/v1/repertoire" {
			t.Fatalf("registry query: %+v %v", p, err)
		}
	}
	if len(seen) != 16 || seen["hpap.ireceptor.org"] || seen["airr-seq.vdjbase.org"] {
		t.Fatal("stale/changed approved registry")
	}
	r := Repositories()
	r[0].URL = "https://unapproved.example"
	if Repositories()[0].URL == r[0].URL {
		t.Fatal("mutable registry")
	}
}

func TestIndependentRequestAndExactEvidence(t *testing.T) {
	var expected struct {
		RequestJSON, RequestSHA256, ResponseJSON, ResponseSHA256 string
		ResponseBytes, NextOffset                                int
		Completeness                                             string
		Records                                                  []Record
	}
	raw, err := os.ReadFile("../../validation/reference/adc.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	calls := 0
	c := Client{UserAgent: "ClonoLoom/test", Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.String() != "https://ipa1.ireceptor.org/airr/v1/repertoire" || string(body) != expected.RequestJSON {
			t.Errorf("unexpected outbound request: %s %s %s", r.Method, r.URL, body)
		}
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json" || r.Header.Get("User-Agent") != "ClonoLoom/test" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Errorf("headers: %v", r.Header)
		}
		return response(200, fixture(t)), nil
	})}
	p, err := c.Query(context.Background(), query())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || p.RequestSHA256 != expected.RequestSHA256 || p.Evidence.ResponseJSON != expected.ResponseJSON || p.Evidence.ResponseSHA256 != expected.ResponseSHA256 || p.Evidence.ResponseBytes != expected.ResponseBytes || p.NextOffset == nil || *p.NextOffset != expected.NextOffset || p.Completeness != expected.Completeness || !reflect.DeepEqual(p.Records, expected.Records) {
		t.Fatalf("independent evidence differs: %+v", p)
	}
	if _, err := time.Parse(time.RFC3339Nano, p.Evidence.RetrievedAt); err != nil {
		t.Fatal(err)
	}
}

func TestQueryRefusalsBeforeNetwork(t *testing.T) {
	calls := 0
	c := Client{Transport: transport(func(*http.Request) (*http.Response, error) { calls++; return response(200, `{"Repertoire":[]}`), nil })}
	for _, alter := range []func(*Query){
		func(q *Query) { q.Repository = "https://ipa1.ireceptor.org" }, func(q *Query) { q.Repository = "localhost" }, func(q *Query) { q.Repository = "ipa1.ireceptor.org@evil.example" },
		func(q *Query) { q.Size = 0 }, func(q *Query) { q.Size = 101 }, func(q *Query) { q.From = -1 }, func(q *Query) { q.From = 10001 }, func(q *Query) { q.Study = strings.Repeat("x", 1025) }, func(q *Query) { q.Study = "bad\nvalue" }, func(q *Query) { q.Study = string([]byte{0xff}) },
	} {
		q := query()
		alter(&q)
		if _, err := c.Query(context.Background(), q); err == nil {
			t.Fatalf("accepted %+v", q)
		}
	}
	if calls != 0 {
		t.Fatal("invalid query contacted a repository")
	}
	c.Offline = true
	if _, err := c.Query(context.Background(), query()); !errors.Is(err, ErrOffline) {
		t.Fatal(err)
	}
	if _, err := c.Info(context.Background(), query().Repository); !errors.Is(err, ErrOffline) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("offline mode made a request")
	}
	// Blank filters mean no restriction; they must not become comparisons with missing data.
	p, err := Prepare(Query{Repository: query().Repository, Size: 1})
	if err != nil || p.RequestJSON != `{"format":"json","from":0,"size":1}` {
		t.Fatalf("%+v %v", p, err)
	}
	q := query()
	q.Repertoire = "exact \" ID"
	p, err = Prepare(q)
	if err != nil || !strings.Contains(p.RequestJSON, `exact \" ID`) {
		t.Fatalf("ID changed: %+v %v", p, err)
	}
}

func TestErrorsBoundsAndAmbiguousResponses(t *testing.T) {
	for name, body := range map[string]string{
		"invalid": `{"Repertoire":[`, "array": `[]`, "missing": `{"Info":{}}`, "null": `{"Repertoire":null}`, "wrong-case": `{"repertoire":[]}`,
		"duplicate-key": `{"Repertoire":[],"Reper\u0074oire":[]}`, "bad-unicode": `{"Repertoire":[],"x":"\ud800"}`,
		"missing-id": `{"Repertoire":[{}]}`, "numeric-id": `{"Repertoire":[{"repertoire_id":9007199254740993}]}`, "duplicate-id": `{"Repertoire":[{"repertoire_id":"x"},{"repertoire_id":"x"}]}`,
		"null-row": `{"Repertoire":[null]}`, "bad-study": `{"Repertoire":[{"repertoire_id":"x","study":[]}]}`, "bad-title": `{"Repertoire":[{"repertoire_id":"x","study":{"study_title":false}}]}`,
		"too-many": `{"Repertoire":[{"repertoire_id":"1"},{"repertoire_id":"2"}]}`, "too-large": strings.Repeat(" ", MaxResponse+1),
	} {
		t.Run(name, func(t *testing.T) {
			c := Client{Transport: transport(func(*http.Request) (*http.Response, error) { return response(200, body), nil })}
			q := query()
			q.Size = 1
			if _, err := c.Query(context.Background(), q); err == nil {
				t.Fatal("accepted malformed/out-of-bounds response")
			}
		})
	}
	for _, code := range []int{204, 301, 302, 307, 400, 401, 403, 404, 429, 500, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			c := Client{Transport: transport(func(*http.Request) (*http.Response, error) {
				calls++
				r := response(code, "provider body must not be exposed")
				r.Header.Set("Location", "https://unapproved.example")
				return r, nil
			})}
			_, err := c.Query(context.Background(), query())
			if err == nil || strings.Contains(err.Error(), "provider body") || calls != 1 {
				t.Fatalf("%v %d", err, calls)
			}
		})
	}
}

func TestManualPaginationAndInfo(t *testing.T) {
	calls := 0
	body := fixture(t)
	c := Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if strings.HasSuffix(r.URL.Path, "/info") {
			if r.Method != "GET" {
				t.Error(r.Method)
			}
			return response(200, `{"version":"server-1","api":{"version":"1.2.0"},"schema":{"version":"1.4.1"},"attributes":{"max_size":9007199254740993}}`), nil
		}
		return response(200, body), nil
	})}
	q := query()
	q.From = 42
	p, err := c.Query(context.Background(), q)
	if err != nil || *p.NextOffset != 45 || calls != 1 {
		t.Fatalf("%+v %v %d", p, err, calls)
	}
	q.From = 10000
	p, err = c.Query(context.Background(), q)
	if err != nil || p.NextOffset != nil {
		t.Fatalf("%+v %v", p, err)
	}
	body = `{"Repertoire":[]}`
	p, err = c.Query(context.Background(), query())
	if err != nil || p.NextOffset != nil || p.Completeness != "not-established" {
		t.Fatalf("%+v %v", p, err)
	}
	e, err := c.Info(context.Background(), query().Repository)
	if err != nil || !strings.Contains(e.ResponseJSON, "9007199254740993") {
		t.Fatalf("%+v %v", e, err)
	}
	if calls != 4 {
		t.Fatalf("implicit requests: %d", calls)
	}
}

func TestCancellationAndBrokenConnection(t *testing.T) {
	var calls atomic.Int32
	c := Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Query(ctx, query()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Query(ctx, query()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	c.Transport = transport(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("connection lost") })
	if _, err := c.Query(context.Background(), query()); err == nil {
		t.Fatal("connection error hidden")
	}
	if calls.Load() > 3 {
		t.Fatal("automatic retry")
	}
}

// Real HTTPS and transparent gzip exercise the standard transport's body/cancellation behavior.
// The injected adapter reroutes the already-validated origin to a local test server only.
func TestHTTPSDecompressionAndBodyCancellation(t *testing.T) {
	for _, oversize := range []bool{false, true} {
		t.Run(map[bool]string{false: "gzip", true: "gzip-over-limit"}[oversize], func(t *testing.T) {
			body := fixture(t)
			if oversize {
				body = strings.Repeat(" ", MaxResponse+1)
			}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/airr/v1/repertoire" {
					t.Error("wrong HTTP operation")
				}
				w.Header().Set("Content-Encoding", "gzip")
				zw := gzip.NewWriter(w)
				_, _ = io.WriteString(zw, body)
				_ = zw.Close()
			}))
			defer server.Close()
			c := Client{Transport: localTLS(t, server)}
			page, err := c.Query(context.Background(), query())
			if oversize {
				if err == nil || !strings.Contains(err.Error(), "8 MiB") {
					t.Fatalf("%v", err)
				}
			} else if err != nil || page.Evidence.ResponseJSON != body {
				t.Fatalf("%v", err)
			}
		})
	}
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	c := Client{Transport: localTLS(t, server)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := c.Query(ctx, query()); done <- err }()
	select {
	case <-started:
		cancel()
	case <-ctx.Done():
		t.Fatal("server did not start response")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("body cancellation: %v", err)
	}
}

func localTLS(t *testing.T, server *httptest.Server) http.RoundTripper {
	t.Helper()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != query().Repository {
			t.Error("request escaped the approved origin")
		}
		local := r.Clone(r.Context())
		local.URL.Scheme = u.Scheme
		local.URL.Host = u.Host
		return server.Client().Transport.RoundTrip(local)
	})
}

func FuzzADCResponse(f *testing.F) {
	f.Add(`{"Repertoire":[{"repertoire_id":"example","x":9007199254740993}]}`)
	f.Add(`{"Repertoire":[]}`)
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		c := Client{Transport: transport(func(*http.Request) (*http.Response, error) { return response(200, raw), nil })}
		p, err := c.Query(context.Background(), query())
		if err == nil && (p.Evidence.ResponseJSON != raw || p.Evidence.ResponseSHA256 != digest([]byte(raw))) {
			t.Fatal("evidence changed")
		}
	})
}

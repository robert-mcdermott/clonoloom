package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mcpTestWriter struct{ frames chan []byte }

func (w mcpTestWriter) Write(p []byte) (int, error) { w.frames <- bytes.Clone(p); return len(p), nil }

type mcpHarness struct {
	s      *mcpServer
	in     *io.PipeWriter
	frames chan []byte
	done   chan error
	next   int
}

func newMCPHarness(t *testing.T, allow string) *mcpHarness {
	t.Helper()
	h := newRemoteHub("")
	h.allowed, _ = parseAutomationAllow(allow)
	frames := make(chan []byte, 256)
	r, w := io.Pipe()
	s := newMCP(h, mcpTestWriter{frames})
	s.progressInterval = 5 * time.Millisecond
	s.closeInput = func() { r.Close() }
	test := &mcpHarness{s: s, in: w, frames: frames, done: make(chan error, 1)}
	go func() { test.done <- s.serve(r) }()
	t.Cleanup(func() {
		w.Close()
		select {
		case err := <-test.done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("MCP did not stop")
		}
	})
	return test
}
func (h *mcpHarness) send(t *testing.T, raw string) {
	t.Helper()
	if _, err := io.WriteString(h.in, raw+"\n"); err != nil {
		t.Fatal(err)
	}
}
func (h *mcpHarness) request(t *testing.T, method string, params map[string]any) int {
	t.Helper()
	h.next++
	if params == nil {
		params = map[string]any{}
	}
	if _, ok := params["_meta"]; !ok {
		params["_meta"] = map[string]any{mcpVersionKey: mcpVersion, mcpCapabilitiesKey: map[string]any{}}
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": h.next, "method": method, "params": params})
	h.send(t, string(raw))
	return h.next
}
func (h *mcpHarness) read(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	select {
	case raw := <-h.frames:
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatal(string(raw), err)
		}
		return msg
	case <-time.After(3 * time.Second):
		t.Fatal("no MCP response")
		return nil
	}
}
func mcpTestError(t *testing.T, msg map[string]json.RawMessage, want int) {
	t.Helper()
	var e mcpError
	if json.Unmarshal(msg["error"], &e) != nil || e.Code != want {
		t.Fatalf("wanted error %d, got %s", want, msg)
	}
}
func mcpTestResult(t *testing.T, msg map[string]json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var result map[string]json.RawMessage
	if json.Unmarshal(msg["result"], &result) != nil {
		t.Fatalf("no result: %s", msg)
	}
	return result
}
func (h *mcpHarness) page() *remoteClient {
	p := &remoteClient{events: make(chan remoteEvent, 128), done: make(chan struct{})}
	h.s.hub.mu.Lock()
	h.s.hub.clients = append(h.s.hub.clients, p)
	h.s.hub.mu.Unlock()
	return p
}
func mcpEvent(t *testing.T, p *remoteClient) remoteEvent {
	t.Helper()
	for {
		select {
		case e := <-p.events:
			if e.Audit == nil {
				return e
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no window event")
			return remoteEvent{}
		}
	}
}
func (h *mcpHarness) answer(e remoteEvent, result remoteResult) {
	h.s.hub.mu.Lock()
	ch := h.s.hub.pending[e.ID]
	delete(h.s.hub.pending, e.ID)
	delete(h.s.hub.pendingEvents, e.ID)
	h.s.hub.mu.Unlock()
	ch <- result
}

func TestMCPDiscoveryAndLegacy(t *testing.T) {
	h := newMCPHarness(t, "")
	h.request(t, "server/discover", nil)
	result := mcpTestResult(t, h.read(t))
	if string(result["resultType"]) != `"complete"` || string(result["ttlMs"]) != "0" || string(result["cacheScope"]) != `"private"` {
		t.Fatal(result)
	}
	h.request(t, "tools/list", nil)
	result = mcpTestResult(t, h.read(t))
	var tools []remoteAction
	json.Unmarshal(result["tools"], &tools)
	if len(tools) != 1 || tools[0].Name != "get_state" {
		t.Fatal(tools)
	}
	h.send(t, `{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"1999-01-01","capabilities":{},"clientInfo":{"name":"legacy notebook","version":"1"}}}`)
	result = mcpTestResult(t, h.read(t))
	if string(result["protocolVersion"]) != `"2025-11-25"` || result["resultType"] != nil {
		t.Fatal(result)
	}
	h.send(t, `{"jsonrpc":"2.0","id":"early","method":"tools/list"}`)
	mcpTestError(t, h.read(t), -32602)
	h.send(t, `{"jsonrpc":"2.0","id":"ping","method":"ping"}`)
	mcpTestResult(t, h.read(t))
	h.send(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	h.send(t, `{"jsonrpc":"2.0","id":"list","method":"tools/list"}`)
	result = mcpTestResult(t, h.read(t))
	if result["resultType"] != nil {
		t.Fatal(result)
	}
	p := h.page()
	h.send(t, `{"jsonrpc":"2.0","id":"legacy","method":"tools/call","params":{"name":"get_state"}}`)
	e := mcpEvent(t, p)
	if e.Client != "legacy notebook" {
		t.Fatal(e)
	}
	h.answer(e, remoteResult{OK: true})
	mcpTestResult(t, h.read(t))
	h.request(t, "tools/call", map[string]any{"name": "get_state"})
	e = mcpEvent(t, p)
	if e.Client != "MCP client" {
		t.Fatal("identity inherited", e)
	}
	h.answer(e, remoteResult{OK: true})
	mcpTestResult(t, h.read(t))
}
func TestMCPStrictProtocol(t *testing.T) {
	h := newMCPHarness(t, "")
	for _, tc := range []struct {
		raw  string
		code int
	}{
		{`{`, -32700}, {`[]`, -32600}, {`{"jsonrpc":"2.0","jsonrpc":"2.0","id":1,"method":"ping"}`, -32600},
		{`{"jsonrpc":"2.0","id":null,"method":"ping"}`, -32600}, {`{"jsonrpc":"2.0","id":9007199254740992,"method":"ping"}`, -32600},
		{`{"jsonrpc":"2.0","id":"a","method":"ping"}`, -32602},
		{`{"jsonrpc":"2.0","id":"b","method":"ping","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":null}}}`, -32602},
		{`{"jsonrpc":"2.0","id":"c","method":"ping","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"future","io.modelcontextprotocol/clientCapabilities":{}}}}`, -32022},
		{`{"jsonrpc":"2.0","id":"a","method":"ping"}`, -32600},
	} {
		h.send(t, tc.raw)
		mcpTestError(t, h.read(t), tc.code)
	}
	h.request(t, "resources/list", nil)
	mcpTestError(t, h.read(t), -32601)
	h.request(t, "tools/list", map[string]any{"cursor": "anything"})
	mcpTestError(t, h.read(t), -32602)
	h.request(t, "tools/call", map[string]any{"name": "no_such_tool"})
	mcpTestError(t, h.read(t), -32602)
	h.send(t, `{"jsonrpc":"2.0","method":"unknown_notification"}`)
	h.request(t, "ping", nil)
	mcpTestResult(t, h.read(t))
}
func TestMCPPolicyAndExactResult(t *testing.T) {
	h := newMCPHarness(t, "")
	p := h.page()
	h.request(t, "tools/call", map[string]any{"name": "set_theme", "arguments": map[string]any{"theme": "dark"}})
	result := mcpTestResult(t, h.read(t))
	if string(result["isError"]) != "true" || !bytes.Contains(result["structuredContent"], []byte("permission-denied")) {
		t.Fatal(result)
	}
	h.request(t, "tools/call", map[string]any{"name": "get_state", "arguments": map[string]any{"unexpected": true}})
	result = mcpTestResult(t, h.read(t))
	if !bytes.Contains(result["structuredContent"], []byte("invalid-arguments")) {
		t.Fatal(result)
	}
	// Native tokens must survive both text and structured content, including integers not exactly
	// representable in IEEE doubles. This is a transport oracle, not invented scientific data.
	data := json.RawMessage(`{"count":9007199254740993,"x":0.1234567890123456789,"missing":null}`)
	h.request(t, "tools/call", map[string]any{"name": "get_state"})
	e := mcpEvent(t, p)
	if e.Action != "get_state" {
		t.Fatal(e)
	}
	h.answer(e, remoteResult{OK: true, Data: data})
	result = mcpTestResult(t, h.read(t))
	if !bytes.Contains(result["structuredContent"], data) {
		t.Fatal(string(result["structuredContent"]))
	}
	var content []struct{ Text string }
	json.Unmarshal(result["content"], &content)
	if len(content) != 1 || content[0].Text != string(result["structuredContent"]) {
		t.Fatal(content)
	}
	h.s.hub.mu.Lock()
	defer h.s.hub.mu.Unlock()
	if len(h.s.hub.audit) < 6 {
		t.Fatal("audit missing")
	}
}
func TestMCPProgressCancelAndEOF(t *testing.T) {
	h := newMCPHarness(t, "")
	p := h.page()
	id := h.request(t, "tools/call", map[string]any{"name": "get_state", "_meta": map[string]any{mcpVersionKey: mcpVersion, mcpCapabilitiesKey: map[string]any{}, "progressToken": "work"}})
	e := mcpEvent(t, p)
	var last float64
	for range 2 {
		msg := h.read(t)
		if string(msg["method"]) != `"notifications/progress"` {
			t.Fatal(msg)
		}
		var progress struct {
			Progress float64
			Total    *float64
		}
		json.Unmarshal(msg["params"], &progress)
		if progress.Progress <= last || progress.Total != nil {
			t.Fatal(progress)
		}
		last = progress.Progress
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": map[string]any{"requestId": id}})
	h.send(t, string(raw))
	cancel := mcpEvent(t, p)
	if !cancel.Cancel || cancel.ID != e.ID {
		t.Fatal(cancel)
	}
	// Ping provides an ordered reader barrier; pre-cancellation progress may already be in flight.
	h.request(t, "ping", nil)
	for {
		msg := h.read(t)
		if msg["id"] != nil {
			if string(msg["id"]) == "1" {
				t.Fatal("cancelled call replied")
			}
			break
		}
	}
	h.request(t, "tools/call", map[string]any{"name": "get_state"})
	e = mcpEvent(t, p)
	h.in.Close()
	cancel = mcpEvent(t, p)
	if !cancel.Cancel || cancel.ID != e.ID {
		t.Fatal(cancel)
	}
}
func TestMCPBoundsAndOutputFailures(t *testing.T) {
	for _, suffix := range []string{"", "\n{}\n"} {
		r := bufio.NewReader(strings.NewReader(strings.Repeat("x", mcpFrameLimit+1) + suffix))
		if _, err := readMCPFrame(r); !errors.Is(err, errMCPFrame) {
			t.Fatal(err)
		}
		if suffix != "" {
			raw, err := readMCPFrame(r)
			if err != nil || string(raw) != "{}" {
				t.Fatal(string(raw), err)
			}
		}
	}
	for _, raw := range []string{`"a"`, `1`, `-1`, `9007199254740991`} {
		if _, ok := mcpID([]byte(raw)); !ok {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{`null`, `1.2`, `1e3`, `9007199254740992`} {
		if _, ok := mcpID([]byte(raw)); ok {
			t.Fatal(raw)
		}
	}
	var out bytes.Buffer
	s := newMCP(newRemoteHub(""), &out)
	s.toolResult(context.Background(), json.RawMessage("1"), remoteResult{OK: true, Data: json.RawMessage(`"` + strings.Repeat("x", mcpOutputLimit/2) + `"`)}, true)
	if out.Len() > mcpOutputLimit || !strings.Contains(out.String(), "output-too-large") || !strings.Contains(out.String(), `"isError":true`) {
		t.Fatal("oversized scientific output was not refused")
	}
	s = newMCP(newRemoteHub(""), mcpBrokenWriter{})
	err := s.serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"))
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

type mcpBrokenWriter struct{}

func (mcpBrokenWriter) Write([]byte) (int, error) { return 0, nil }
func TestMCPConcurrentBoundAndQueuedCancellation(t *testing.T) {
	h := newMCPHarness(t, "")
	p := h.page()
	for range mcpCallLimit {
		h.request(t, "tools/call", map[string]any{"name": "get_state"})
	}
	mcpEvent(t, p)
	h.request(t, "tools/call", map[string]any{"name": "get_state"})
	result := mcpTestResult(t, h.read(t))
	if !bytes.Contains(result["structuredContent"], []byte(`"code":"busy"`)) {
		t.Fatal(result)
	}
	h.in.Close()
}
func TestMCPConfigAndLocalURLs(t *testing.T) {
	var out, errout bytes.Buffer
	for _, args := range [][]string{{"--help"}, {"--version"}} {
		if runMCP(args, strings.NewReader(""), &out, &errout) != 0 || out.Len() != 0 {
			t.Fatal(out.String(), errout.String())
		}
	}
	for _, args := range [][]string{{"--allow", "everything"}, {"--remote-control=false"}, {"--host", "192.0.2.1"}, {"input.tsv"}} {
		if runMCP(args, strings.NewReader(""), &out, &errout) == 0 || out.Len() != 0 {
			t.Fatal(args)
		}
	}
	for _, value := range []string{"https://127.0.0.1:8860", "http://example.org:8860", "http://user@127.0.0.1:8860", "http://127.0.0.1:8860/path", "http://127.0.0.1:8860?x=1"} {
		if validLocalURL(value) {
			t.Fatal(value)
		}
	}
	if !validLocalURL("http://127.0.0.1:8860") || !validLocalURL("http://[::1]:8860") {
		t.Fatal("loopback refused")
	}
}
func FuzzMCPFrame(f *testing.F) {
	for _, seed := range []string{`{"jsonrpc":"2.0","id":"x","method":"ping"}`, "{}\n{}\n", "\xff", `{"x":1,"x":2}`, ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		r := bufio.NewReader(strings.NewReader(value))
		frame, _ := readMCPFrame(r)
		if len(frame) > mcpFrameLimit {
			t.Fatal("bound")
		}
		if utf8Valid := json.Valid(frame); utf8Valid {
			var request mcpRequest
			_ = strictJSON(frame, &request)
			mcpID(request.ID)
		}
	})
}

func TestMCPCatalogGrantsAndReview(t *testing.T) {
	h := newMCPHarness(t, "edits,compute,files,network")
	h.request(t, "tools/list", nil)
	result := mcpTestResult(t, h.read(t))
	var tools []struct {
		Name string
		Meta map[string]struct {
			Effects        []string
			Review         bool
			API            int
			CatalogVersion int
			PolicyVersion  int
		} `json:"_meta"`
	}
	if err := json.Unmarshal(result["tools"], &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != len(remoteActions) {
		t.Fatal("catalog divergence")
	}
	for i, tool := range tools {
		want := remoteActions[i]
		meta := tool.Meta["org.clonoloom/automation"]
		if tool.Name != want.Name || meta.Review != want.Review || meta.API != 2 || meta.CatalogVersion != 3 || meta.PolicyVersion != 1 || strings.Join(meta.Effects, ",") != strings.Join(want.Effects, ",") {
			t.Fatal(tool, want)
		}
	}
	p := h.page()
	h.request(t, "tools/call", map[string]any{"name": "set_theme", "arguments": map[string]any{"theme": "dark"}})
	e := mcpEvent(t, p)
	if !e.Review || e.Title == "" || len(e.Effects) != 2 {
		t.Fatal("review metadata bypassed", e)
	}
	h.answer(e, remoteResult{Code: "rejected", Message: "Researcher rejected this proposal."})
	result = mcpTestResult(t, h.read(t))
	if string(result["isError"]) != "true" || !bytes.Contains(result["structuredContent"], []byte("rejected")) {
		t.Fatal(result)
	}
}
func TestMCPReadyAfterPolicyAndCancellation(t *testing.T) {
	hub := newRemoteHub("")
	hub.allowed, _ = parseAutomationAllow("")
	called := false
	ready := func(context.Context) error { called = true; return errNoPage }
	_, failure := hub.execute(context.Background(), remoteEvent{Action: "set_theme", Args: json.RawMessage(`{"theme":"dark"}`)}, true, ready)
	if called || failure.code != "permission-denied" {
		t.Fatal("denied call opened window", failure)
	}
	_, failure = hub.execute(context.Background(), remoteEvent{Action: "get_state", Args: json.RawMessage(`{"bad":true}`)}, true, ready)
	if called || failure.code != "invalid-arguments" {
		t.Fatal("invalid call opened window", failure)
	}
	_, failure = hub.execute(context.Background(), remoteEvent{Action: "get_state"}, true, ready)
	if !called || failure.code != "window-unavailable" {
		t.Fatal(failure)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &remoteClient{events: make(chan remoteEvent, 1), done: make(chan struct{})}
	hub.clients = []*remoteClient{p}
	for range 100 {
		if _, err := hub.dispatch(ctx, remoteEvent{Action: "get_state"}); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	if len(p.events) != 0 {
		t.Fatal("already cancelled request reached window")
	}
}
func TestMCPRefusesLiveLibrary(t *testing.T) {
	dir := t.TempDir()
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"name": "ClonoLoom", "dataDir": dir})
	}))
	defer live.Close()
	raw, _ := json.Marshal(connectionInfo{URL: live.URL})
	if err := os.WriteFile(filepath.Join(dir, "remote.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := mcpCheckHost(config{host: "127.0.0.1", port: 0, dataDir: dir}); err == nil {
		t.Fatal("live library allowed")
	}
	live.Close()
	if err := mcpCheckHost(config{host: "127.0.0.1", port: 0, dataDir: dir}); err != nil {
		t.Fatal("stale connection refused", err)
	}
}

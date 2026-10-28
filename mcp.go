package main

// Stdio hosting and lazy-window pattern adapted from Apache-2.0 CytoWeave 0.8.1
// (19d47bb) and MaveScape 0.8.0 (c997e39).
// Protocol: https://modelcontextprotocol.io/specification/2026-07-28 (and explicit
// 2025-11-25 lifecycle compatibility). Scientific actions remain in the shared catalog.
import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	mcpVersion         = "2026-07-28"
	mcpLegacyVersion   = "2025-11-25"
	mcpFrameLimit      = 2 << 20
	mcpOutputLimit     = 16 << 20
	mcpCallLimit       = 16
	mcpIDLimit         = 65536
	mcpVersionKey      = "io.modelcontextprotocol/protocolVersion"
	mcpCapabilitiesKey = "io.modelcontextprotocol/clientCapabilities"
	mcpClientKey       = "io.modelcontextprotocol/clientInfo"
)

var errMCPFrame = errors.New("MCP frame exceeds 2 MiB")

var mcpInstructions = "ClonoLoom uses the open local window and its current project. Grants are fixed at launch; edits require separate human review in the window. Client names are labels, not authority. Cancellation cannot undo completed file effects. Results use existing native methods; transport success is not scientific validation. Preserve JSON numbers exactly and consult docs/MCP.md."

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
type mcpCall struct {
	cancel context.CancelFunc
	token  string
}
type mcpServer struct {
	hub   *remoteHub
	ready func(context.Context) error
	out   io.Writer
	// outMu serializes complete frames and the cancellation boundary: after cancel returns,
	// no result/progress can be emitted for that request. mu only protects active calls.
	outMu                          sync.Mutex
	mu                             sync.Mutex
	active                         map[string]*mcpCall
	wg                             sync.WaitGroup
	writeErr                       error
	closeInput                     func()
	progressInterval               time.Duration
	legacyInitialized, legacyReady bool
	legacyClient                   string
	seen                           map[string]bool // reader goroutine only; bounded session request IDs
}

func newMCP(hub *remoteHub, out io.Writer) *mcpServer {
	return &mcpServer{hub: hub, out: out, active: map[string]*mcpCall{}, seen: map[string]bool{}, progressInterval: 5 * time.Second}
}

func runMCP(args []string, in io.Reader, out, stderr io.Writer) int {
	cfg, err := parseConfigFor(args, stderr, true)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if cfg.showVersion {
		fmt.Fprintf(stderr, "clonoloom %s\n", version)
		return 0
	}
	if !cfg.remote || len(cfg.files) > 0 {
		fmt.Fprintln(stderr, "MCP requires remote control and no positional files; use the reviewed open_data tool.")
		return 2
	}
	if ip := net.ParseIP(cfg.host); ip == nil || !ip.IsLoopback() {
		fmt.Fprintln(stderr, "MCP requires a numeric loopback --host.")
		return 2
	}
	if err := mcpCheckHost(cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	window := cfg.window
	cfg.window = "none"
	host, err := start(cfg, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer host.stop(2 * time.Second)
	go func() {
		if err := host.serve(); err != nil {
			fmt.Fprintln(stderr, err)
		}
	}()
	server := newMCP(host.app.remote, out)
	var windowMu sync.Mutex
	var lastOpen time.Time
	server.ready = func(ctx context.Context) error {
		timeout := time.NewTimer(40 * time.Second)
		defer timeout.Stop()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			host.app.remote.mu.Lock()
			connected := len(host.app.remote.clients) > 0
			host.app.remote.mu.Unlock()
			if connected {
				return nil
			}
			windowMu.Lock()
			if window != "none" && time.Since(lastOpen) > 20*time.Second {
				lastOpen = time.Now()
				if _, err := openWindow(host.url, window, cfg.dataDir); err != nil {
					fmt.Fprintln(stderr, "MCP window:", err)
				}
			}
			windowMu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timeout.C:
				return fmt.Errorf("open the ClonoLoom window at %s, then submit again", host.url)
			case <-ticker.C:
			}
		}
	}
	if closer, ok := in.(io.Closer); ok {
		server.closeInput = func() { _ = closer.Close() }
	}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-signals.Done():
			server.cancelAll()
			if server.closeInput != nil {
				server.closeInput()
			}
		case <-done:
		}
	}()
	err = server.serve(in)
	if err != nil && signals.Err() == nil {
		fmt.Fprintln(stderr, "MCP:", err)
		return 1
	}
	return 0
}

// Do not take over a running host's library or overwrite its connection/grants. This is a
// best-effort live-host check, not an interprocess filesystem lock; use a dedicated data dir.
func mcpCheckHost(cfg config) error {
	urls := []string{"http://" + net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port))}
	var connection connectionInfo
	if file, err := os.Open(filepath.Join(cfg.dataDir, "remote.json")); err == nil {
		data, readErr := io.ReadAll(io.LimitReader(file, 65537))
		file.Close()
		// Connection files are untrusted: bound reads, probe loopback only, no redirects.
		if readErr == nil && len(data) <= 65536 && json.Unmarshal(data, &connection) == nil && validLocalURL(connection.URL) {
			urls = append(urls, connection.URL)
		}
	}
	client := &http.Client{Timeout: 500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	wanted, _ := filepath.Abs(cfg.dataDir)
	if resolved, err := filepath.EvalSymlinks(wanted); err == nil {
		wanted = resolved
	}
	for _, url := range urls {
		response, err := client.Get(url + "/api/info")
		if err != nil {
			continue
		}
		var info struct{ Name, DataDir string }
		err = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&info)
		response.Body.Close()
		actual, _ := filepath.Abs(info.DataDir)
		if resolved, err := filepath.EvalSymlinks(actual); err == nil {
			actual = resolved
		}
		if err == nil && info.Name == "ClonoLoom" && actual == wanted {
			return fmt.Errorf("ClonoLoom already uses this data directory at %s; stop it or choose a dedicated --data-dir for MCP", url)
		}
	}
	return nil
}

func validLocalURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback() && u.Port() != ""
}

func mcpID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || len(raw) > 514 {
		return "", false
	}
	var value string
	if raw[0] == '"' && json.Unmarshal(raw, &value) == nil {
		return "s:" + value, true
	}
	n, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || n < -9007199254740991 || n > 9007199254740991 {
		return "", false
	}
	return "n:" + strconv.FormatInt(n, 10), true
}
func mcpObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if len(raw) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	err := strictJSON(raw, &obj)
	if err == nil && obj == nil {
		err = errors.New("expected an object")
	}
	return obj, err
}
func mcpClient(raw json.RawMessage) (string, error) {
	obj, err := mcpObject(raw)
	if err != nil {
		return "", err
	}
	var name, version string
	if json.Unmarshal(obj["name"], &name) != nil || json.Unmarshal(obj["version"], &version) != nil || name == "" || version == "" {
		return "", errors.New("clientInfo needs name and version")
	}
	return clientName(name), nil
}

// readMCPFrame drains an oversized line before returning, including an unterminated last line.
func readMCPFrame(r *bufio.Reader) ([]byte, error) {
	var data []byte
	oversized := false
	for {
		part, err := r.ReadSlice('\n')
		if !oversized && len(data)+len(part) <= mcpFrameLimit {
			data = append(data, part...)
		} else {
			oversized = true
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if oversized {
			return nil, errMCPFrame
		}
		if err == io.EOF && len(data) > 0 {
			err = nil
		}
		return bytes.TrimSpace(data), err
	}
}
func (s *mcpServer) serve(in io.Reader) (resultErr error) {
	defer func() {
		s.cancelAll()
		s.wg.Wait()
		s.outMu.Lock()
		defer s.outMu.Unlock()
		if resultErr == nil {
			resultErr = s.writeErr
		}
	}()
	reader := bufio.NewReader(in)
	for {
		s.outMu.Lock()
		writeErr := s.writeErr
		s.outMu.Unlock()
		if writeErr != nil {
			return writeErr
		}
		raw, err := readMCPFrame(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			if !errors.Is(err, errMCPFrame) {
				return err
			}
			s.failure(nil, -32700, err.Error(), nil)
			continue
		}
		if len(raw) == 0 {
			continue
		}
		var request mcpRequest
		if !utf8.Valid(raw) || !json.Valid(raw) {
			s.failure(nil, -32700, "Invalid JSON or UTF-8.", nil)
			continue
		}
		if strictJSON(raw, &request) != nil || request.JSONRPC != "2.0" || request.Method == "" {
			s.failure(nil, -32600, "Expected one JSON-RPC 2.0 request; batches are unsupported.", nil)
			continue
		}
		if len(request.ID) == 0 {
			s.notification(request)
			continue
		}
		key, valid := mcpID(request.ID)
		if !valid {
			s.failure(nil, -32600, "IDs must be strings (at most 512 bytes) or safe integers.", nil)
			continue
		}
		if s.seen[key] {
			s.failure(request.ID, -32600, "Request IDs cannot be reused within a stdio session.", nil)
			continue
		}
		if len(s.seen) >= mcpIDLimit {
			s.failure(request.ID, -32600, "Session request limit reached; restart the MCP process.", nil)
			return errors.New("session request limit reached")
		}
		s.seen[key] = true
		s.request(request, key)
		s.outMu.Lock()
		err = s.writeErr
		s.outMu.Unlock()
		if err != nil {
			return err
		}
	}
	s.outMu.Lock()
	err := s.writeErr
	s.outMu.Unlock()
	return err
}
func (s *mcpServer) notification(r mcpRequest) {
	switch r.Method {
	case "notifications/initialized":
		if s.legacyInitialized {
			s.legacyReady = true
		}
	case "notifications/cancelled":
		obj, err := mcpObject(r.Params)
		if err != nil {
			return
		}
		key, ok := mcpID(obj["requestId"])
		if !ok {
			return
		}
		s.outMu.Lock()
		s.mu.Lock()
		if call := s.active[key]; call != nil {
			call.cancel()
		}
		s.mu.Unlock()
		s.outMu.Unlock()
	}
}
func (s *mcpServer) cancelAll() {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, call := range s.active {
		call.cancel()
	}
}
func (s *mcpServer) failure(id json.RawMessage, code int, message string, data any) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	s.send(nil, map[string]any{"jsonrpc": "2.0", "id": id, "error": mcpError{code, message, data}})
}
func (s *mcpServer) send(ctx context.Context, message any) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	} // all payloads are checked before reaching this boundary
	s.outMu.Lock()
	defer s.outMu.Unlock()
	if s.writeErr != nil || (ctx != nil && ctx.Err() != nil) {
		return
	}
	data = append(data, '\n')
	if writer, ok := s.out.(interface{ SetWriteDeadline(time.Time) error }); ok {
		_ = writer.SetWriteDeadline(time.Now().Add(10 * time.Second))
		defer writer.SetWriteDeadline(time.Time{})
	}
	n, err := s.out.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		s.writeErr = err
		if s.closeInput != nil {
			s.closeInput()
		}
	}
}
func mcpInfo() map[string]any {
	return map[string]any{"name": "clonoloom", "title": "ClonoLoom", "version": version}
}
func mcpCapabilities() map[string]any {
	return map[string]any{"tools": map[string]any{"listChanged": false}}
}
func (s *mcpServer) result(ctx context.Context, id json.RawMessage, result map[string]any, modern bool) {
	if modern {
		result["resultType"] = "complete"
		result["_meta"] = map[string]any{"io.modelcontextprotocol/serverInfo": mcpInfo()}
	}
	s.send(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
func (s *mcpServer) request(r mcpRequest, key string) {
	params, err := mcpObject(r.Params)
	if err != nil {
		s.failure(r.ID, -32602, "params must be an object.", nil)
		return
	}
	if r.Method == "initialize" {
		s.initialize(r, params)
		return
	}
	meta, err := mcpObject(params["_meta"])
	if err != nil {
		s.failure(r.ID, -32602, "_meta must be an object.", nil)
		return
	}
	if r.Method == "ping" && s.legacyInitialized && !s.legacyReady && len(meta) == 0 {
		s.result(nil, r.ID, map[string]any{}, false)
		return
	}
	modern := meta[mcpVersionKey] != nil || meta[mcpCapabilitiesKey] != nil || meta[mcpClientKey] != nil || !s.legacyReady
	client := s.legacyClient
	if modern {
		var protocol string
		if json.Unmarshal(meta[mcpVersionKey], &protocol) != nil || meta[mcpCapabilitiesKey] == nil {
			s.failure(r.ID, -32602, "Each request needs protocolVersion and clientCapabilities in _meta.", nil)
			return
		}
		if _, err := mcpObject(meta[mcpCapabilitiesKey]); err != nil {
			s.failure(r.ID, -32602, "clientCapabilities must be an object.", nil)
			return
		}
		if protocol != mcpVersion {
			s.failure(r.ID, -32022, "Unsupported protocol version.", map[string]any{"requested": protocol, "supported": []string{mcpVersion, mcpLegacyVersion}})
			return
		}
		client = "MCP client"
		if raw := meta[mcpClientKey]; raw != nil {
			client, err = mcpClient(raw)
			if err != nil {
				s.failure(r.ID, -32602, err.Error(), nil)
				return
			}
		}
	}
	switch r.Method {
	case "ping":
		s.result(nil, r.ID, map[string]any{}, modern)
	case "server/discover":
		if !modern {
			s.failure(r.ID, -32601, "server/discover requires MCP 2026-07-28 metadata.", nil)
			return
		}
		s.result(nil, r.ID, map[string]any{"supportedVersions": []string{mcpVersion, mcpLegacyVersion}, "capabilities": mcpCapabilities(), "instructions": mcpInstructions, "ttlMs": 0, "cacheScope": "private"}, true)
	case "tools/list":
		if params["cursor"] != nil {
			s.failure(r.ID, -32602, "This catalog fits one page; omit cursor.", nil)
			return
		}
		tools := []any{}
		for _, action := range remoteActions {
			allowed := true
			for _, effect := range action.Effects {
				allowed = allowed && s.hub.allowed[effect]
			}
			if !allowed {
				continue
			}
			tools = append(tools, map[string]any{"name": action.Name, "title": action.Title, "description": action.Description, "inputSchema": action.InputSchema, "_meta": map[string]any{"org.clonoloom/automation": map[string]any{"effects": action.Effects, "review": action.Review, "api": apiVersion, "catalogVersion": actionCatalogVersion, "policyVersion": automationPolicyVersion}}})
		}
		result := map[string]any{"tools": tools}
		if modern {
			result["ttlMs"] = 0
			result["cacheScope"] = "private"
		}
		s.result(nil, r.ID, result, modern)
	case "tools/call":
		var name string
		if json.Unmarshal(params["name"], &name) != nil || name == "" || len(name) > 128 {
			s.failure(r.ID, -32602, "Tool name must contain 1–128 bytes.", nil)
			return
		}
		if _, ok := findAction(name); !ok {
			s.hub.record(remoteEvent{Action: name, Client: client}, "refused")
			s.failure(r.ID, -32602, "Unknown tool; consult tools/list.", nil)
			return
		}
		for field := range params {
			if field != "name" && field != "arguments" && field != "_meta" {
				s.failure(r.ID, -32602, "Unsupported tools/call parameter: "+field, nil)
				return
			}
		}
		args := params["arguments"]
		if len(args) == 0 {
			args = json.RawMessage("{}")
		}
		if len(args) > maxRemoteActionBytes {
			s.failure(r.ID, -32602, "Tool arguments exceed 1 MiB.", nil)
			return
		}
		token := ""
		if raw := meta["progressToken"]; raw != nil {
			var ok bool
			token, ok = mcpID(raw)
			if !ok {
				s.failure(r.ID, -32602, "Invalid progress token.", nil)
				return
			}
		}
		s.mu.Lock()
		busy := len(s.active) >= mcpCallLimit
		for _, call := range s.active {
			busy = busy || (token != "" && call.token == token)
		}
		if busy {
			s.mu.Unlock()
			s.toolResult(nil, r.ID, remoteResult{Code: "busy", Message: "At most 16 active calls; progress tokens must be unique among active calls."}, modern)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.active[key] = &mcpCall{cancel, token}
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			defer cancel()
			defer func() { s.mu.Lock(); delete(s.active, key); s.mu.Unlock() }()
			stopped := make(chan struct{})
			progressDone := make(chan struct{})
			go func() {
				defer close(progressDone)
				if token == "" {
					return
				}
				ticker := time.NewTicker(s.progressInterval)
				defer ticker.Stop()
				started := time.Now()
				for {
					select {
					case <-ctx.Done():
						return
					case <-stopped:
						return
					case tick := <-ticker.C:
						elapsed := tick.Sub(started).Seconds()
						s.send(ctx, map[string]any{"jsonrpc": "2.0", "method": "notifications/progress", "params": map[string]any{"progressToken": meta["progressToken"], "progress": elapsed, "message": "Elapsed seconds waiting for the window, review or action; not percent complete."}})
					}
				}
			}()
			result, failure := s.hub.execute(ctx, remoteEvent{Action: name, Args: args, Client: client}, true, s.ready)
			close(stopped)
			<-progressDone
			if failure != nil {
				result = remoteResult{Code: failure.code, Message: failure.message}
			}
			s.toolResult(ctx, r.ID, result, modern)
		}()
	default:
		s.failure(r.ID, -32601, "Method not supported.", nil)
	}
}
func (s *mcpServer) initialize(r mcpRequest, params map[string]json.RawMessage) {
	if s.legacyInitialized {
		s.failure(r.ID, -32600, "Legacy initialization already completed.", nil)
		return
	}
	var requested string
	client, err := mcpClient(params["clientInfo"])
	_, capsErr := mcpObject(params["capabilities"])
	if json.Unmarshal(params["protocolVersion"], &requested) != nil || requested == "" || params["capabilities"] == nil || capsErr != nil || err != nil {
		s.failure(r.ID, -32602, "initialize requires protocolVersion, capabilities and clientInfo name/version.", nil)
		return
	}
	s.legacyInitialized = true
	s.legacyClient = client
	s.result(nil, r.ID, map[string]any{"protocolVersion": mcpLegacyVersion, "capabilities": mcpCapabilities(), "serverInfo": mcpInfo(), "instructions": mcpInstructions}, false)
}
func (s *mcpServer) toolResult(ctx context.Context, id json.RawMessage, result remoteResult, modern bool) {
	// Never decode through float64. Both representations carry identical native numeric tokens.
	raw, err := json.Marshal(result)
	if err != nil {
		raw = []byte(`{"ok":false,"code":"invalid-result","message":"Window returned invalid JSON."}`)
		result.OK = false
	}
	body := map[string]any{"content": []any{map[string]any{"type": "text", "text": string(raw)}}, "structuredContent": json.RawMessage(raw), "isError": !result.OK}
	encoded, _ := json.Marshal(body)
	if len(encoded) > mcpOutputLimit-2048 {
		s.toolResult(ctx, id, remoteResult{Code: "output-too-large", Message: "Result exceeds the 16 MiB MCP output bound. The action may have completed; inspect state before another call. Request fewer rows or use native file exports. Nothing was truncated or automatically retried."}, modern)
		return
	}
	s.result(ctx, id, body, modern)
}

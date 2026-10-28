package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// The page performs exactly the actions the program lists.
func TestEveryActionHasAPageHandler(t *testing.T) {
	source, err := os.ReadFile("web/ui/remote.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "const actions = {")
	end := strings.Index(text[start:], "\n  };\n")
	if start < 0 || end < 0 {
		t.Fatal("no action table in web/ui/remote.js")
	}
	var page []string
	for _, m := range regexp.MustCompile(`(?m)^    async (\w+)\(`).FindAllStringSubmatch(text[start:start+end], -1) {
		page = append(page, m[1])
	}
	var program []string
	for _, a := range remoteActions {
		program = append(program, a.Name)
	}
	sort.Strings(page)
	sort.Strings(program)
	if strings.Join(page, " ") != strings.Join(program, " ") {
		t.Fatalf("the page performs %v; the program lists %v", page, program)
	}
}

// A page connected to the events, answering every action with its name: what a caller gets.
func remoteServer(t *testing.T, session string) (*remoteHub, *httptest.Server) {
	t.Helper()
	hub := newRemoteHub(session)
	hub.timeout = 5 * time.Second
	mux := http.NewServeMux()
	hub.register(mux)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return hub, server
}

func attachPage(t *testing.T, server *httptest.Server, session string) (stop func()) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/remote/events?session="+session, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("events: %d", resp.StatusCode)
	}
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event remoteEvent
			json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event)
			if event.Audit != nil || event.Cancel {
				continue
			}
			outcome, _ := json.Marshal(remoteResult{OK: true, Message: event.Action + " by " + event.Client})
			req, _ := http.NewRequest("POST", server.URL+"/api/remote/result/"+event.ID, strings.NewReader(string(outcome)))
			req.Header.Set("X-ClonoLoom-Session", session)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}
	}()
	return func() { resp.Body.Close() }
}

func act(t *testing.T, server *httptest.Server, body, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/remote/action", strings.NewReader(body))
	if token != "" {
		req.Header.Set(remoteTokenHeader, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	text, _ := json.Marshal(out)
	return resp.StatusCode, string(text)
}

func TestRemoteActionsArePerformedByThePage(t *testing.T) {
	hub, server := remoteServer(t, "s1")
	hub.allowed, _ = parseAutomationAllow("edits,compute,files,network")
	if code, _ := act(t, server, `{"action": "get_state"}`, ""); code != http.StatusServiceUnavailable {
		t.Errorf("no page: %d", code)
	}
	// A page of another run of the program is refused.
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/remote/events?session=other", nil)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusConflict {
		t.Errorf("another run's page: %v", resp.StatusCode)
	}
	stop := attachPage(t, server, "s1")
	defer stop()
	deadline := time.Now().Add(2 * time.Second)
	for {
		hub.mu.Lock()
		n := len(hub.clients)
		hub.mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if code, body := act(t, server, `{"action": "get_state", "client": "a test\u0007"}`, ""); code != http.StatusOK || !strings.Contains(body, `"get_state by a test"`) {
		t.Errorf("%d %s", code, body)
	}
	if code, _ := act(t, server, `{"action": "format_disk"}`, ""); code != http.StatusNotFound {
		t.Errorf("unknown action: %d", code)
	}
	if code, _ := act(t, server, `not json`, ""); code != http.StatusBadRequest {
		t.Errorf("not JSON: %d", code)
	}
	// Reading files by path needs the token.
	if code, _ := act(t, server, `{"action": "open_data", "args": {"paths": ["/x"]}}`, ""); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
	if code, _ := act(t, server, `{"action": "open_data", "args": {"paths": ["/x"]}}`, "wrong"); code != http.StatusUnauthorized {
		t.Errorf("a wrong token: %d", code)
	}
	if code, body := act(t, server, `{"action": "open_data", "args": {"paths": ["/x"]}}`, hub.token); code != http.StatusOK || !strings.Contains(body, "open_data") {
		t.Errorf("with the token: %d %s", code, body)
	}
}

// Remote control is refused from other computers and absent unless asked for.
func TestRemoteControlIsLocalAndOptIn(t *testing.T) {
	_, handler := testApp(t)
	if rec := request(t, handler, http.MethodGet, "/api/remote/tools", nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("without --remote-control: %d", rec.Code)
	}
	dir := t.TempDir()
	a, err := newApp(config{dataDir: dir, cacheDir: filepath.Join(dir, "cache"), remote: true})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := a.handler()
	guarded := protect(h, "127.0.0.1", defaultPort)
	req := httptest.NewRequest(http.MethodGet, "/api/remote/tools", nil)
	req.Host, req.RemoteAddr = "127.0.0.1:8860", "192.0.2.7:4000"
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("another computer: %d", rec.Code)
	}
	req.RemoteAddr = "127.0.0.1:4000"
	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)
	var tools struct {
		API   int `json:"api"`
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	json.Unmarshal(rec.Body.Bytes(), &tools)
	if rec.Code != http.StatusOK || tools.API != apiVersion || len(tools.Tools) != len(remoteActions) {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
	info := request(t, guarded, http.MethodGet, "/api/info", nil, nil)
	if !strings.Contains(info.Body.String(), `"remoteControl":true`) {
		t.Errorf("info %s", info.Body)
	}
}

// remote.json: readable by this user only, and removed only by the run that wrote it.
func TestTheConnectionFile(t *testing.T) {
	dir := t.TempDir()
	path, err := writeConnectionFile(dir, "http://127.0.0.1:8860", "token-1")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 || filepath.Base(path) != connectionFileName {
		t.Fatalf("%v %v", err, info.Mode())
	}
	removeConnectionFile(path, "token-2")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("another run removed it")
	}
	removeConnectionFile(path, "token-1")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("not removed")
	}
}

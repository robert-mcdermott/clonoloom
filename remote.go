package main

// Remote control (clonoloom-spec/requirements.md, A7), enabled with --remote-control: a program on
// this computer POSTs an action to /api/remote/action. The open ClonoLoom page receives it over
// server-sent events from /api/remote/events, performs it where the user sees it, and POSTs the
// outcome to /api/remote/result/{id}, which the server returns to the waiting caller. The actions
// are listed in actions.go and performed by web/ui/remote.js. Adapted from SituCraft 0.9.0
// remote.go and connection.go, without their MCP restrictions and call log (wave 8).
//
// Only requests from this computer that name it in their Host header are accepted, and browsers
// on other origins are refused (security.go). Reading files by path also needs the token printed
// at startup and written to remote.json: another user of a shared computer can reach the loopback
// port but not the owner's terminal or files.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxRemoteActionBytes = 1 << 20
	maxRemoteResultBytes = 16 << 20
	remoteHeartbeat      = 15 * time.Second
	remoteTokenHeader    = "X-ClonoLoom-Token"
	connectionFileName   = "remote.json"
)

type remoteHub struct {
	mu      sync.Mutex
	counter uint64
	clients []*remoteClient
	pending map[string]chan remoteResult
	timeout time.Duration
	// turn: one action at a time, in the order they came.
	turn  chan struct{}
	token string
	// session names this run of the program: only a page opened from it takes its actions, so a
	// tab left open while ClonoLoom was started again is refused when it reconnects. Empty: any
	// page (the tests' hubs).
	session string
}

type remoteClient struct {
	events chan remoteEvent
	done   chan struct{}
}

// An event carries an action for the page: { id, action, args } and who sent it (what a script
// calls itself).
type remoteEvent struct {
	ID     string          `json:"id"`
	Action string          `json:"action"`
	Args   json.RawMessage `json:"args,omitempty"`
	Client string          `json:"client,omitempty"`
}

type remoteResult struct {
	OK      bool            `json:"ok"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

var (
	errNoPage      = errors.New("No ClonoLoom page is connected. Open ClonoLoom in a browser first.")
	errPageBusy    = errors.New("The ClonoLoom page is busy; try again.")
	errPageTimeout = errors.New("The ClonoLoom page did not answer in time.")
	errPageGone    = errors.New("The ClonoLoom page was closed or reloaded before it answered.")
)

func newRemoteHub(session string) *remoteHub {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		panic(err)
	}
	return &remoteHub{pending: map[string]chan remoteResult{}, timeout: 10 * time.Minute, turn: make(chan struct{}, 1), token: base64.RawURLEncoding.EncodeToString(buffer), session: session}
}

func (h *remoteHub) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/remote/events", localOnly(h.serveEvents))
	mux.HandleFunc("POST /api/remote/result/{id}", localOnly(h.serveResult))
	mux.HandleFunc("POST /api/remote/action", localOnly(h.serveAction))
	mux.HandleFunc("GET /api/remote/tools", localOnly(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"version": version, "api": apiVersion, "tools": remoteActions})
	}))
	mux.HandleFunc("GET /api/remote/pages", localOnly(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		pages := len(h.clients)
		h.mu.Unlock()
		writeJSON(w, map[string]int{"pages": pages})
	}))
}

// localOnly refuses requests from other machines and from web pages that reach the loopback port
// under another name (DNS rebinding).
func localOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeError(w, http.StatusForbidden, "Forbidden: remote control only accepts requests from this computer, addressed to localhost or 127.0.0.1.")
			return
		}
		next(w, r)
	}
}

// clientName is a sender's name as the page shows it: printable, at most 60 characters.
func clientName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" {
		return "a program on this computer"
	}
	if runes := []rune(name); len(runes) > 60 {
		name = string(runes[:60])
	}
	return name
}

func (h *remoteHub) serveEvents(w http.ResponseWriter, r *http.Request) {
	if h.session != "" && r.URL.Query().Get("session") != h.session {
		writeError(w, http.StatusConflict, "This page was opened from another run of ClonoLoom: reload it to drive this one.")
		return
	}
	stream := http.NewResponseController(w)
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	fmt.Fprint(w, ": connected\n\n")
	if err := stream.Flush(); err != nil {
		return
	}
	client := &remoteClient{events: make(chan remoteEvent, 16), done: make(chan struct{})}
	h.mu.Lock()
	h.clients = append(h.clients, client)
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		for i, c := range h.clients {
			if c == client {
				h.clients = append(h.clients[:i], h.clients[i+1:]...)
				break
			}
		}
		h.mu.Unlock()
		close(client.done)
	}()
	ticker := time.NewTicker(remoteHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-client.events:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: action\ndata: %s\n\n", data)
			if err := stream.Flush(); err != nil {
				return
			}
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			if err := stream.Flush(); err != nil {
				return
			}
		}
	}
}

// serveAction performs {"action": "...", "args": {...}, "client": "..."} in the page. Actions that
// read files by path need the token.
func (h *remoteHub) serveAction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Action string          `json:"action"`
		Args   json.RawMessage `json:"args"`
		Client string          `json:"client"`
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRemoteActionBytes))
	if err == nil {
		err = json.Unmarshal(body, &request)
	}
	if err != nil || request.Action == "" {
		writeError(w, http.StatusBadRequest, `Send JSON such as {"action": "get_state"}.`)
		return
	}
	action, known := findAction(request.Action)
	if !known {
		writeError(w, http.StatusNotFound, fmt.Sprintf("No action %q: GET /api/remote/tools lists them.", request.Action))
		return
	}
	if action.reads && subtle.ConstantTimeCompare([]byte(r.Header.Get(remoteTokenHeader)), []byte(h.token)) != 1 {
		writeError(w, http.StatusUnauthorized, "Reading files needs the "+remoteTokenHeader+" header with the token in remote.json (in ClonoLoom's data folder).")
		return
	}
	outcome, err := h.dispatch(r.Context(), remoteEvent{Action: request.Action, Args: request.Args, Client: clientName(request.Client)})
	switch {
	case err == nil:
		writeJSON(w, outcome)
	case errors.Is(err, errPageTimeout):
		writeError(w, http.StatusGatewayTimeout, err.Error())
	case errors.Is(err, errNoPage), errors.Is(err, errPageBusy), errors.Is(err, errPageGone):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
	default:
		writeError(w, http.StatusBadGateway, err.Error())
	}
}

// dispatch sends an event to the most recently opened page and waits for its result, one at a
// time; a request waiting its turn can still be cancelled.
func (h *remoteHub) dispatch(ctx context.Context, event remoteEvent) (remoteResult, error) {
	select {
	case h.turn <- struct{}{}:
	case <-ctx.Done():
		return remoteResult{}, ctx.Err()
	}
	defer func() { <-h.turn }()
	h.mu.Lock()
	var client *remoteClient
	if len(h.clients) > 0 {
		client = h.clients[len(h.clients)-1]
	}
	h.counter++
	id := strconv.FormatUint(h.counter, 10)
	result := make(chan remoteResult, 1)
	h.pending[id] = result
	h.mu.Unlock()
	forget := func() {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
	}
	if client == nil {
		forget()
		return remoteResult{}, errNoPage
	}
	event.ID = id
	select {
	case client.events <- event:
	default:
		forget()
		return remoteResult{}, errPageBusy
	}
	timer := time.NewTimer(h.timeout)
	defer timer.Stop()
	select {
	case outcome := <-result:
		return outcome, nil
	case <-client.done:
		forget()
		return remoteResult{}, errPageGone
	case <-timer.C:
		forget()
		return remoteResult{}, errPageTimeout
	case <-ctx.Done():
		forget()
		return remoteResult{}, ctx.Err()
	}
}

func (h *remoteHub) serveResult(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	result, ok := h.pending[r.PathValue("id")]
	delete(h.pending, r.PathValue("id"))
	h.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "No action is waiting for that result.")
		return
	}
	var outcome remoteResult
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRemoteResultBytes))
	if err == nil {
		err = json.Unmarshal(body, &outcome)
	}
	if err != nil {
		outcome = remoteResult{Message: "The result could not be read."}
	}
	result <- outcome
	w.WriteHeader(http.StatusNoContent)
}

// connectionInfo is remote.json: how scripts find a running ClonoLoom (its address and the token
// reading files needs), readable by this user only, removed when it stops.
type connectionInfo struct {
	URL     string `json:"url"`
	Token   string `json:"token"`
	Version string `json:"version"`
	PID     int    `json:"pid"`
}

// writeConnectionFile writes dir/remote.json through a temporary file renamed into place, so a
// reader never sees half a file. Returns its path.
func writeConnectionFile(dir, url, token string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(connectionInfo{URL: url, Token: token, Version: version, PID: os.Getpid()}, "", "  ")
	if err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(dir, ".remote-*.json")
	if err != nil {
		return "", err
	}
	name := temp.Name()
	if err = temp.Chmod(0o600); err == nil {
		_, err = temp.Write(append(data, '\n'))
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	path := filepath.Join(dir, connectionFileName)
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return path, nil
}

// removeConnectionFile removes the file if it still describes this ClonoLoom (one started later
// with the same data folder replaces it, and keeps it).
func removeConnectionFile(path, token string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var info connectionInfo
	if json.Unmarshal(data, &info) == nil && info.Token == token {
		os.Remove(path)
	}
}

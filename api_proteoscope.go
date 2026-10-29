package main

// Reviewed single-observation handoff. Proteoscope 0.10's public command contract;
// loopback discovery follows Apache-2.0 MaveScape, without background port scanning.
import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"clonoloom/internal/airr"
	"clonoloom/internal/qc"
	"clonoloom/internal/reads"
)

const handoffLimits = "One supplied AIRR sequence_aa observation, not a reconstructed receptor or paired complex. No translation, trimming, junction substitution, chain inference or structure prediction. complete_vdj describes the annotated V(D)J region, not a complete protein or binding specificity. Similar structures are not evidence of antigen specificity, affinity or function. Only the exact amino-acid query goes to Proteoscope/RCSB; identifiers and annotations stay in this local record. Proteoscope may cache the query and use cached search results. This handoff is not scientific replay. Cancellation after dispatch cannot retract a query; do not retry an uncertain outcome automatically."

var handoffSequence = regexp.MustCompile(`^[ACDEFGHIKLMNPQRSTVWY]+$`)

type handoffRequest struct {
	Dataset      string `json:"dataset"`
	Row          *int   `json:"row,omitempty"`
	Offset       int    `json:"offset,omitempty"`
	Port         int    `json:"port"`
	Review       string `json:"review,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	AcceptLocal  bool   `json:"acceptLocal"`
	AcceptPublic bool   `json:"acceptPublic"`
	AcceptTerms  bool   `json:"acceptTerms"`
}
type handoffDestination struct {
	URL           string `json:"url"`
	Version       string `json:"version"`
	RemoteControl bool   `json:"remoteControl"`
	Offline       bool   `json:"offline"`
	Ready         bool   `json:"ready"`
	Reason        string `json:"reason"`
}
type handoffEvidence struct {
	Format         string             `json:"format"`
	Version        int                `json:"version"`
	Software       string             `json:"software"`
	Dataset        string             `json:"dataset"`
	Row            int                `json:"row"`
	Sample         string             `json:"sample"`
	SequenceID     string             `json:"sequenceId"`
	Locus          string             `json:"locus"`
	Source         reads.Source       `json:"source"`
	Line           uint32             `json:"line"`
	HeaderText     string             `json:"headerText"`
	RowText        string             `json:"rowText"`
	Fields         map[string]string  `json:"fields"`
	Sequence       string             `json:"sequence"`
	SequenceSHA256 string             `json:"sequenceSHA256"`
	Command        string             `json:"command"`
	Eligible       bool               `json:"eligible"`
	Reason         string             `json:"reason"`
	Destination    handoffDestination `json:"destination"`
	PublicService  string             `json:"publicService"`
	Notice         string             `json:"notice"`
	Restricted     bool               `json:"restricted"`
	Limits         string             `json:"limits"`
}

// Read the selected source, never a caller-provided path. Fingerprint compressed bytes
// before parsing; retain header/row text without line endings and decoded fields as text.
func handoffSource(ctx context.Context, source reads.Source, line uint32) (string, string, map[string]string, error) {
	if source.Format != reads.FormatAIRR || source.Mapping != nil {
		return "", "", nil, errors.New("This handoff supports native AIRR TSV sequence_aa only; no sequence is inferred from other formats or junctions.")
	}
	st, e := os.Lstat(source.Path)
	if e != nil {
		return "", "", nil, e
	}
	if !st.Mode().IsRegular() || st.Size() > 64<<20 {
		return "", "", nil, errors.New("AIRR handoff needs a regular source within 64 MiB")
	}
	f, e := os.Open(source.Path)
	if e != nil {
		return "", "", nil, e
	}
	defer f.Close()
	current, e := f.Stat()
	if e != nil || !os.SameFile(st, current) {
		return "", "", nil, errors.New("Source changed while opening")
	}
	raw, e := io.ReadAll(io.LimitReader(runContextReader{ctx, f}, (64<<20)+1))
	if e != nil {
		return "", "", nil, e
	}
	if len(raw) > 64<<20 || int64(len(raw)) != source.Size || replayDigest(raw) != source.SHA256 {
		return "", "", nil, errors.New("Source bytes differ from the loaded dataset; reopen and review")
	}
	var input io.Reader = bytes.NewReader(raw)
	if source.Gzipped {
		g, e := gzip.NewReader(input)
		if e != nil {
			return "", "", nil, e
		}
		defer g.Close()
		input = g
	}
	decoded, e := io.ReadAll(io.LimitReader(runContextReader{ctx, input}, (64<<20)+1))
	if e != nil {
		return "", "", nil, e
	}
	if len(decoded) > 64<<20 {
		return "", "", nil, errors.New("Decompressed AIRR source exceeds 64 MiB")
	}
	if !utf8.Valid(decoded) {
		return "", "", nil, errors.New("AIRR text must be valid UTF-8 for exact handoff evidence")
	}
	scanner := bufio.NewScanner(bytes.NewReader(decoded))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var header, row string
	for n := uint32(1); scanner.Scan(); n++ {
		if e := ctx.Err(); e != nil {
			return "", "", nil, e
		}
		if n == 1 {
			header = scanner.Text()
		}
		if n == line {
			row = scanner.Text()
			break
		}
	}
	if e := scanner.Err(); e != nil {
		return "", "", nil, e
	}
	if row == "" || line < 2 {
		return "", "", nil, errors.New("Selected AIRR source row is unavailable")
	}
	columns, values := nativeReportValues(strings.TrimPrefix(header, "\ufeff")), nativeReportValues(row)
	if !reflect.DeepEqual(columns, source.Header) || len(columns) != len(values) {
		return "", "", nil, errors.New("AIRR header or row width differs from the loaded evidence")
	}
	fields := map[string]string{}
	for i, k := range columns {
		if _, ok := fields[k]; ok {
			return "", "", nil, errors.New("Repeated source column")
		}
		fields[k] = values[i]
	}
	return header, row, fields, nil
}

func handoffClient() *http.Client {
	return &http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Proteoscope redirects are refused") }}
}
func handoffRead(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return nil, e
	}
	r, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("Destination returned HTTP %d", r.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if len(b) > 1<<20 {
		return nil, errors.New("Destination response exceeds 1 MiB")
	}
	return b, e
}
func probeHandoff(ctx context.Context, port int) handoffDestination {
	out := handoffDestination{URL: fmt.Sprintf("http://127.0.0.1:%d/", port)}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	client := handoffClient()
	page, e := handoffRead(ctx, client, out.URL)
	if e != nil || !bytes.Contains(page, []byte("<title>Proteoscope</title>")) {
		out.Reason = "No recognized Proteoscope at this port. Start Proteoscope and open its browser page."
		return out
	}
	raw, e := handoffRead(ctx, client, out.URL+"api/startup")
	if e != nil {
		out.Reason = e.Error()
		return out
	}
	var s struct {
		Version string          `json:"version"`
		Offline *bool           `json:"offline"`
		Remote  *bool           `json:"remoteControl"`
		Files   json.RawMessage `json:"files"`
	}
	if json.Unmarshal(raw, &s) != nil || s.Offline == nil || s.Remote == nil || len(s.Files) == 0 {
		out.Reason = "Unrecognized Proteoscope startup contract"
		return out
	}
	out.Version, out.Offline, out.RemoteControl = s.Version, *s.Offline, *s.Remote
	switch {
	case s.Version != "0.10.0":
		out.Reason = "Only Proteoscope 0.10.0's sequence-search contract has been validated"
	case out.Offline:
		out.Reason = "Proteoscope is offline; sequence search is unavailable"
	case !out.RemoteControl:
		out.Reason = "Start Proteoscope with --remote-control and open its browser page to receive a reviewed search"
	default:
		out.Ready = true
	}
	return out
}

func (a *app) handoffEvidence(ctx context.Context, req handoffRequest) (handoffEvidence, error) {
	out := handoffEvidence{Format: "clonoloom-proteoscope-handoff", Version: 1, Software: "ClonoLoom " + version + "; " + buildCommit(), Dataset: req.Dataset, PublicService: "RCSB PDB sequence search (https://search.rcsb.org/)", Limits: handoffLimits}
	ds, ok := a.datasets.get(req.Dataset)
	if !ok {
		return out, errors.New("Open the dataset again")
	}
	t := ds.Result.Table
	if req.Row == nil || *req.Row < 0 || *req.Row >= t.Len() || req.Port < 1 || req.Port > 65535 {
		return out, errors.New("Choose one explicit row and a loopback port from 1 to 65535")
	}
	i := *req.Row
	out.Row = i
	out.Source = ds.Result.Sources[t.Source[i]]
	out.Line = t.Line[i]
	out.Sample = t.Samples[t.Sample[i]].Name
	out.SequenceID = t.IDs.Get(t.SequenceID[i])
	out.Locus = t.LocusName(i)
	var e error
	out.HeaderText, out.RowText, out.Fields, e = handoffSource(ctx, out.Source, out.Line)
	if e != nil {
		return out, e
	}
	if out.Fields["sequence_id"] != out.SequenceID || out.SequenceID == "" {
		return out, errors.New("The exact source sequence identifier is missing or inconsistent")
	}
	out.Sequence = out.Fields["sequence_aa"]
	out.SequenceSHA256 = replayDigest([]byte(out.Sequence))
	out.Eligible = len(out.Sequence) >= 25 && len(out.Sequence) <= 5000 && handoffSequence.MatchString(out.Sequence) && out.Locus != "" && out.Fields["locus"] == out.Locus
	if out.Eligible {
		out.Command = "search " + out.Sequence
	} else {
		out.Reason = "Need a declared receptor locus and unchanged sequence_aa of 25–5000 uppercase canonical amino acids. Missing, ambiguous, stopped, gapped or other sequences are not repaired; junctions are not substituted."
	}
	n := a.notice(ds, string(t.Unit), qc.Filter{}, "One source observation; no clonotype grouping or QC selection", "")
	n.Made = ds.Opened
	n.What = handoffLimits
	out.Notice, out.Restricted = n.Text(), n.Restricted()
	out.Destination = handoffDestination{URL: fmt.Sprintf("http://127.0.0.1:%d/", req.Port), Reason: "Sequence is not eligible; destination was not contacted"}
	if a.adc != nil && a.adc.Offline {
		out.Destination.Reason = "ClonoLoom is offline; no destination was contacted"
	} else if out.Eligible {
		out.Destination = probeHandoff(ctx, req.Port)
	}
	return out, nil
}

func (a *app) registerProteoscope(mux *http.ServeMux) {
	type approval struct {
		Fingerprint string
		Expires     time.Time
	}
	var mu sync.Mutex
	pending := map[string]approval{}
	mux.HandleFunc("POST /api/proteoscope/{operation}", localOnly(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var req handoffRequest
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
		if e == nil {
			e = strictRunJSON(raw, &req)
		}
		if e != nil {
			writeError(w, 400, "Send one explicit dataset, row and destination; unknown fields are refused.")
			return
		}
		op := r.PathValue("operation")
		if op == "page" {
			ds, ok := a.datasets.get(req.Dataset)
			if !ok {
				writeError(w, 404, "Open the dataset again")
				return
			}
			t := ds.Result.Table
			if req.Offset < 0 || req.Offset > t.Len() {
				writeError(w, 400, "Invalid row offset")
				return
			}
			rows := []map[string]any{}
			for i := req.Offset; i < t.Len() && i < req.Offset+25; i++ {
				rows = append(rows, map[string]any{"row": i, "sequenceId": t.IDs.Get(t.SequenceID[i]), "sample": t.Samples[t.Sample[i]].Name, "locus": t.LocusName(i), "line": t.Line[i], "source": ds.Result.Sources[t.Source[i]].Name})
			}
			writeJSON(w, map[string]any{"rows": rows, "total": t.Len()})
			return
		}
		if op != "review" && op != "send" && op != "export" {
			writeError(w, 404, "No such handoff operation")
			return
		}
		if op == "send" && (!req.AcceptLocal || !req.AcceptPublic) {
			writeError(w, 403, "Explicit approval of the local Proteoscope transfer and public RCSB sequence search is required")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		doc, e := a.handoffEvidence(ctx, req)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		fingerprint := replayDigest(mustJSON(doc))
		if op == "review" {
			mu.Lock()
			for k, v := range pending {
				if time.Now().After(v.Expires) {
					delete(pending, k)
				}
			}
			if len(pending) >= 128 {
				mu.Unlock()
				writeError(w, 429, "Review capacity reached; wait five minutes")
				return
			}
			id := rand.Text()
			pending[id] = approval{fingerprint, time.Now().Add(5 * time.Minute)}
			mu.Unlock()
			writeJSON(w, map[string]any{"review": id, "fingerprint": fingerprint, "evidence": doc})
			return
		}
		if doc.Restricted && !req.AcceptTerms {
			writeError(w, 403, "Acknowledge provider terms before export or transfer; acknowledgment does not grant disclosure rights")
			return
		}
		mu.Lock()
		p, ok := pending[req.Review]
		valid := ok && time.Now().Before(p.Expires) && p.Fingerprint == fingerprint && req.Fingerprint == fingerprint
		if valid && op == "send" {
			delete(pending, req.Review)
		}
		mu.Unlock()
		if !valid {
			writeError(w, 409, "Review expired, was already sent, or source/destination changed; review again")
			return
		}
		if op == "export" {
			w.Header().Set("Content-Disposition", `attachment; filename="clonoloom-proteoscope-handoff.json"`)
			w.Header().Set("X-ClonoLoom-Handoff", fingerprint)
			writeJSON(w, doc)
			return
		}
		if !doc.Eligible || !doc.Destination.Ready {
			writeError(w, 409, "Sequence or destination is unavailable; review again")
			return
		}
		body := mustJSON(map[string]string{"command": doc.Command})
		transfer, _ := http.NewRequestWithContext(ctx, "POST", doc.Destination.URL+"api/remote/command", bytes.NewReader(body))
		transfer.Header.Set("Content-Type", "application/json")
		response, e := handoffClient().Do(transfer)
		if e != nil {
			writeError(w, 502, "Handoff outcome is uncertain after dispatch. Check Proteoscope before reviewing again; no automatic retry.")
			return
		}
		defer response.Body.Close()
		reply, e := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		var result struct {
			OK      bool            `json:"ok"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		if e != nil || len(reply) > 1<<20 || response.StatusCode != 200 || airr.CheckExactJSON(ctx, reply) != nil || json.Unmarshal(reply, &result) != nil || !result.OK {
			writeError(w, 502, "Proteoscope did not confirm the command. Check its page; the query may already have been sent. No automatic retry.")
			return
		}
		writeJSON(w, map[string]any{"format": "clonoloom-proteoscope-receipt", "version": 1, "fingerprint": fingerprint, "destination": doc.Destination, "sequenceSHA256": doc.SequenceSHA256, "acceptedLocal": true, "acceptedPublic": true, "acceptedTerms": req.AcceptTerms, "response": json.RawMessage(reply), "responseText": string(reply), "responseSHA256": replayDigest(reply), "limits": "Proteoscope acknowledged the command, not a scientific result or complete remote search. Inspect its search errors and hits. The review is consumed; no automatic retry."})
	}))
}

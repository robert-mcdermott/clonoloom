package adc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"clonoloom/internal/airr"
)

var ErrOffline = errors.New("ClonoLoom was started with --offline: ADC requests are disabled")

// Query supports exact, case-sensitive string equality, ANDed in the order below.
// No receptor sequences, free-form URLs, credentials, facets or unbounded query shapes.
type Query struct {
	Repository string `json:"repository"`
	Species    string `json:"species"`
	Locus      string `json:"locus"`
	Study      string `json:"study"`
	Repertoire string `json:"repertoire"`
	From       int    `json:"from"`
	Size       int    `json:"size"`
}

type Preview struct {
	Repository    Repository `json:"repository"`
	Endpoint      string     `json:"endpoint"`
	RequestJSON   string     `json:"requestJSON"`
	RequestSHA256 string     `json:"requestSHA256"`
	From          int        `json:"from"`
	Size          int        `json:"size"`
}

func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

func Prepare(q Query) (Preview, error) {
	r, ok := repository(q.Repository)
	if !ok {
		return Preview{}, errors.New("choose a repository from the pinned approved registry")
	}
	if q.Size < 1 || q.Size > MaxPage || q.From < 0 || q.From > MaxOffset {
		return Preview{}, fmt.Errorf("page size must be 1–%d and offset 0–%d", MaxPage, MaxOffset)
	}
	filters := []any{}
	for _, f := range []struct{ field, value string }{
		{"subject.species.id", q.Species}, {"sample.pcr_target.pcr_target_locus", q.Locus},
		{"study.study_id", q.Study}, {"repertoire_id", q.Repertoire},
	} {
		if !utf8.ValidString(f.value) || len(f.value) > 1024 || strings.ContainsAny(f.value, "\x00\r\n") {
			return Preview{}, fmt.Errorf("%s must be a UTF-8 string of at most 1024 bytes without line breaks or NUL", f.field)
		}
		if f.value != "" {
			filters = append(filters, map[string]any{"op": "=", "content": map[string]string{"field": f.field, "value": f.value}})
		}
	}
	payload := map[string]any{"format": "json", "from": q.From, "size": q.Size}
	if len(filters) == 1 {
		payload["filters"] = filters[0]
	}
	if len(filters) > 1 {
		payload["filters"] = map[string]any{"op": "and", "content": filters}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Preview{}, err
	}
	return Preview{r, r.URL + "/airr/v1/repertoire", string(raw), digest(raw), q.From, q.Size}, nil
}

type Client struct {
	Offline   bool
	UserAgent string
	// Transport is injectable for deterministic protocol tests. Destinations still come only
	// from the registry. A transport cannot alter the client's redirect or timeout policy.
	Transport http.RoundTripper
}

type Evidence struct {
	Endpoint       string `json:"endpoint"`
	RetrievedAt    string `json:"retrievedAt"`
	ResponseSHA256 string `json:"responseSHA256"`
	ResponseBytes  int    `json:"responseBytes"`
	ResponseJSON   string `json:"responseJSON"`
}

type Record struct {
	ID      string `json:"id"`
	Study   string `json:"study"`
	Title   string `json:"title"`
	Subject string `json:"subject"`
	RawJSON string `json:"rawJSON"`
}

type Page struct {
	Preview
	Evidence     Evidence `json:"evidence"`
	Records      []Record `json:"records"`
	NextOffset   *int     `json:"nextOffset"`
	Completeness string   `json:"completeness"`
}

// get performs exactly one request. No retries, authentication, redirects or installations.
func (c *Client) get(ctx context.Context, endpoint, payload string) (Evidence, error) {
	if c.Offline {
		return Evidence{}, ErrOffline
	}
	if err := ctx.Err(); err != nil {
		return Evidence{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	method := http.MethodGet
	if payload != "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(payload))
	if err != nil {
		return Evidence{}, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", c.UserAgent)
	client := &http.Client{Transport: c.Transport, Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return Evidence{}, fmt.Errorf("ADC request failed (no automatic retry): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Evidence{}, fmt.Errorf("ADC returned HTTP %d; check the repository status or query before trying again (redirects and automatic retries are disabled)", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponse+1))
	if err != nil {
		return Evidence{}, fmt.Errorf("read ADC response: %w", err)
	}
	if len(raw) > MaxResponse {
		return Evidence{}, errors.New("ADC response exceeds 8 MiB; reduce the page size or narrow the query")
	}
	if err := ctx.Err(); err != nil {
		return Evidence{}, err
	}
	if err := airr.CheckExactJSON(ctx, raw); err != nil {
		return Evidence{}, fmt.Errorf("invalid ADC response: %w", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return Evidence{}, errors.New("ADC response must be a JSON object")
	}
	return Evidence{endpoint, time.Now().UTC().Format(time.RFC3339Nano), digest(raw), len(raw), string(raw)}, nil
}

func (c *Client) Info(ctx context.Context, id string) (Evidence, error) {
	r, ok := repository(id)
	if !ok {
		return Evidence{}, errors.New("choose a repository from the pinned approved registry")
	}
	return c.get(ctx, r.URL+"/airr/v1/info", "")
}

func (c *Client) Query(ctx context.Context, q Query) (Page, error) {
	p, err := Prepare(q)
	if err != nil {
		return Page{}, err
	}
	e, err := c.get(ctx, p.Endpoint, p.RequestJSON)
	if err != nil {
		return Page{}, err
	}
	return parsePage(ctx, p, e)
}

// parsePage preserves source objects verbatim. It checks the response envelope and identifiers,
// not MiAIRR completeness, the provider's filter implementation or biological validity.
func parsePage(ctx context.Context, p Preview, e Evidence) (Page, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(e.ResponseJSON), &envelope); err != nil {
		return Page{}, err
	}
	raw, ok := envelope["Repertoire"]
	if !ok || len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '[' {
		return Page{}, errors.New("ADC response must contain a Repertoire array")
	}
	var records []json.RawMessage
	if err := json.Unmarshal(raw, &records); err != nil {
		return Page{}, err
	}
	if len(records) > p.Size {
		return Page{}, errors.New("ADC returned more repertoires than requested; the page was refused")
	}
	page := Page{Preview: p, Evidence: e, Records: []Record{}, Completeness: "not-established"}
	seen := map[string]bool{}
	for _, raw := range records {
		if err := ctx.Err(); err != nil {
			return Page{}, err
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
			return Page{}, errors.New("each ADC repertoire must be an object")
		}
		id, err := text(obj, "repertoire_id")
		if err != nil || id == "" || len(id) > 4096 || seen[id] {
			return Page{}, errors.New("ADC repertoire IDs must be nonempty unique strings of at most 4096 bytes within the page")
		}
		seen[id] = true
		row := Record{ID: id, RawJSON: string(raw)}
		for _, pair := range []struct {
			object, field string
			target        *string
		}{{"study", "study_id", &row.Study}, {"study", "study_title", &row.Title}, {"subject", "subject_id", &row.Subject}} {
			var nested map[string]json.RawMessage
			if v, ok := obj[pair.object]; ok {
				if err := json.Unmarshal(v, &nested); err != nil {
					return Page{}, fmt.Errorf("ADC %s must be an object or null", pair.object)
				}
			}
			value, err := text(nested, pair.field)
			if err != nil {
				return Page{}, err
			}
			*pair.target = value
		}
		page.Records = append(page.Records, row)
	}
	// A repository may cap size below the requested limit; advance by the returned count.
	// Even an empty/short page does not establish a complete, stable cohort.
	if next := p.From + len(records); len(records) > 0 && next <= MaxOffset {
		page.NextOffset = &next
	}
	return page, nil
}

func text(obj map[string]json.RawMessage, key string) (string, error) {
	raw, ok := obj[key]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("ADC %s must be a string or null", key)
	}
	return value, nil
}

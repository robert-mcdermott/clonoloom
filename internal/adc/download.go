package adc

// ADC v1 retrieval follows the pinned contract cited in docs/ADC.md. A download
// captures a bounded observation, not a stable snapshot or a complete cohort.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type DownloadSpec struct {
	Repository string `json:"repository"`
	Repertoire string `json:"repertoire"`
	Processing string `json:"processing"`
	License    string `json:"license"`
	Citation   string `json:"citation"`
	Pages      int    `json:"pages"`
	Size       int    `json:"size"`
	Accept     bool   `json:"accept"`
}
type DownloadPlan struct {
	Metadata    Preview `json:"metadata"`
	Endpoint    string  `json:"endpoint"`
	RequestJSON string  `json:"requestJSON"`
	MaxBytes    int     `json:"maxBytes"`
	MaxRecords  int     `json:"maxRecords"`
}
type CapturedPage struct {
	File          string `json:"file"`
	RequestJSON   string `json:"requestJSON"`
	RequestSHA256 string `json:"requestSHA256"`
	Endpoint      string `json:"endpoint"`
	RetrievedAt   string `json:"retrievedAt"`
	SHA256        string `json:"sha256"`
	Bytes         int    `json:"bytes"`
	Records       int    `json:"records"`
}
type DownloadManifest struct {
	Version          int            `json:"version"`
	Contract         string         `json:"contract"`
	RegistryRevision string         `json:"registryRevision"`
	Spec             DownloadSpec   `json:"spec"`
	Pages            []CapturedPage `json:"pages"`
	Records          int            `json:"records"`
	Stop             string         `json:"stop"`
	Completeness     string         `json:"completeness"`
	Notice           string         `json:"notice"`
}
type DownloadResult struct {
	Path     string           `json:"path"`
	Manifest DownloadManifest `json:"manifest"`
}

func PlanDownload(s DownloadSpec) (DownloadPlan, error) {
	if s.Repertoire == "" || s.Processing == "" || !utf8.ValidString(s.Processing) || len(s.Processing) > 1024 || strings.ContainsAny(s.Processing, "\x00\r\n") {
		return DownloadPlan{}, errors.New("choose one exact repertoire and data-processing ID")
	}
	if s.Pages < 1 || s.Pages > 10 || s.Size < 1 || s.Size > 1000 {
		return DownloadPlan{}, errors.New("download budget: 1–10 pages, 1–1000 records per page")
	}
	for _, value := range []string{s.License, s.Citation} {
		if !utf8.ValidString(value) || len(value) > 4096 {
			return DownloadPlan{}, errors.New("permission evidence and citation are valid UTF-8 and limited to 4096 bytes each")
		}
	}
	p, err := Prepare(Query{Repository: s.Repository, Repertoire: s.Repertoire, Size: 1})
	if err != nil {
		return DownloadPlan{}, err
	}
	return DownloadPlan{p, p.Repository.URL + "/airr/v1/rearrangement", rearrangementRequest(s, 0), (s.Pages + 1) * MaxResponse, s.Pages * s.Size}, nil
}
func rearrangementRequest(s DownloadSpec, from int) string {
	predicates := []any{}
	for _, pair := range [][2]string{{"repertoire_id", s.Repertoire}, {"data_processing_id", s.Processing}} {
		predicates = append(predicates, map[string]any{"op": "=", "content": map[string]string{"field": pair[0], "value": pair[1]}})
	}
	b, _ := json.Marshal(map[string]any{"filters": map[string]any{"op": "and", "content": predicates}, "format": "json", "from": from, "size": s.Size})
	return string(b)
}

// Download creates a random, exclusive directory below the application's library.
// A completion manifest is written last. No remote identifier is a filesystem path.
func (c *Client) Download(ctx context.Context, parent string, s DownloadSpec) (out DownloadResult, err error) {
	plan, err := PlanDownload(s)
	if err != nil {
		return out, err
	}
	if !s.Accept || strings.TrimSpace(s.License) == "" || strings.TrimSpace(s.Citation) == "" {
		return out, errors.New("review and record provider permission evidence and citation, then explicitly accept this bounded download")
	}
	if c.Offline {
		return out, ErrOffline
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err = ctx.Err(); err != nil {
		return out, err
	}
	// parent is the configured library, not user/provider input. MkdirTemp never replaces a path.
	dir, err := os.MkdirTemp(parent, "adc-download-")
	if err != nil {
		return out, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(dir)
		}
	}()
	manifest := DownloadManifest{Version: 1, Contract: Contract, RegistryRevision: RegistryRevision, Spec: s, Pages: []CapturedPage{}, Completeness: "not-established", Notice: "Captured live pages only. No stable ordering, snapshot, cohort completeness, subject equivalence, scientific validity or redistribution permission is certified. JSON is original evidence, not an installed analysis dataset."}
	save := func(file, request string, e Evidence, count int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(dir, file), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = f.WriteString(e.ResponseJSON)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		manifest.Pages = append(manifest.Pages, CapturedPage{file, request, digest([]byte(request)), e.Endpoint, e.RetrievedAt, e.ResponseSHA256, e.ResponseBytes, count})
		return nil
	}
	metadata, err := c.Query(ctx, Query{Repository: s.Repository, Repertoire: s.Repertoire, Size: 1})
	if err != nil {
		return out, err
	}
	if len(metadata.Records) != 1 || metadata.Records[0].ID != s.Repertoire {
		return out, errors.New("repository did not return the exact selected repertoire")
	}
	var record map[string]json.RawMessage
	_ = json.Unmarshal([]byte(metadata.Records[0].RawJSON), &record)
	var processing []map[string]json.RawMessage
	if json.Unmarshal(record["data_processing"], &processing) != nil {
		return out, errors.New("metadata lacks a data_processing array")
	}
	matches := 0
	for _, p := range processing {
		id, _ := text(p, "data_processing_id")
		if id == s.Processing {
			matches++
		}
	}
	if matches != 1 {
		return out, errors.New("selected processing ID is absent or ambiguous in retrieved metadata")
	}
	if err = save("metadata.json", metadata.RequestJSON, metadata.Evidence, 1); err != nil {
		return out, err
	}
	seen := map[string]bool{}
	manifest.Stop = "page-budget"
	for page := 0; page < s.Pages; page++ {
		request := rearrangementRequest(s, manifest.Records)
		evidence, e := c.get(ctx, plan.Endpoint, request)
		if e != nil {
			return out, e
		}
		var envelope map[string]json.RawMessage
		_ = json.Unmarshal([]byte(evidence.ResponseJSON), &envelope)
		raw := envelope["Rearrangement"]
		if len(raw) == 0 || raw[0] != '[' {
			return out, errors.New("ADC response lacks a Rearrangement array")
		}
		var rows []map[string]json.RawMessage
		if json.Unmarshal(raw, &rows) != nil || len(rows) > s.Size {
			return out, errors.New("malformed or oversized rearrangement page")
		}
		for _, row := range rows {
			id, e := text(row, "sequence_id")
			if e != nil || id == "" || len(id) > 4096 || seen[id] {
				return out, errors.New("missing, malformed or repeated sequence ID; unstable pages cannot be pooled")
			}
			for _, pair := range [][2]string{{"repertoire_id", s.Repertoire}, {"data_processing_id", s.Processing}} {
				v, e := text(row, pair[0])
				if e != nil || v != pair[1] {
					return out, errors.New("rearrangement scope differs from the reviewed repertoire/processing pair")
				}
			}
			seen[id] = true
		}
		if err = save(fmt.Sprintf("page-%03d.json", page+1), request, evidence, len(rows)); err != nil {
			return out, err
		}
		manifest.Records += len(rows)
		if len(rows) == 0 {
			manifest.Stop = "empty-page"
			break
		}
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return out, err
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), append(b, '\n'), 0600); err != nil {
		return out, err
	}
	complete = true
	return DownloadResult{dir, manifest}, nil
}

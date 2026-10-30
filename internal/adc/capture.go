package adc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"clonoloom/internal/airr"
)

// Capture is checked source evidence, not a certified cohort snapshot.
type Capture struct {
	Manifest       DownloadManifest
	SHA256         string
	Study, Subject string
	Rows           []map[string]json.RawMessage
	Bytes          int
}

func ReadCapture(ctx context.Context, path string, budget int) (Capture, error) {
	var out Capture
	st, err := os.Lstat(path)
	if err != nil {
		return out, err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return out, errors.New("capture is not a regular directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return out, err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil {
		return out, err
	}
	if !os.SameFile(st, opened) {
		return out, errors.New("capture directory changed while opening")
	}
	read := func(name string, limit int) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		limit = min(limit, budget-out.Bytes)
		if limit < 0 {
			return nil, errors.New("capture evidence exceeds the remaining byte budget")
		}
		st, e := root.Lstat(name)
		if e != nil {
			return nil, e
		}
		if !st.Mode().IsRegular() || st.Size() > int64(limit) {
			return nil, errors.New("capture member is not a bounded regular file")
		}
		f, e := root.Open(name)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		actual, e := f.Stat()
		if e != nil {
			return nil, e
		}
		if !os.SameFile(st, actual) {
			return nil, errors.New("capture member changed while opening")
		}
		b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
		if e != nil {
			return nil, e
		}
		if len(b) > limit {
			return nil, errors.New("capture member exceeds budget")
		}
		if e = airr.CheckExactJSON(ctx, b); e != nil {
			return nil, e
		}
		out.Bytes += len(b)
		return b, nil
	}
	raw, err := read("manifest.json", 256<<10)
	if err != nil {
		return out, err
	}
	out.SHA256 = digest(raw)
	if err = decodeCaptureManifest(raw, &out.Manifest); err != nil {
		return out, err
	}
	m := out.Manifest
	plan, err := PlanDownload(m.Spec)
	if err != nil {
		return out, err
	}
	if m.Version != 1 || m.Contract != Contract || m.RegistryRevision != RegistryRevision || m.Completeness != "not-established" || !m.Spec.Accept || strings.TrimSpace(m.Spec.License) == "" || strings.TrimSpace(m.Spec.Citation) == "" || len(m.Pages) < 2 || len(m.Pages) > m.Spec.Pages+1 {
		return out, errors.New("unsupported or inconsistent capture manifest")
	}
	seen := map[string]bool{}
	total := 0
	last := -1
	for i, p := range m.Pages {
		name := fmt.Sprintf("page-%03d.json", i)
		endpoint := plan.Endpoint
		request := rearrangementRequest(m.Spec, total)
		if i == 0 {
			name = "metadata.json"
			endpoint = plan.Metadata.Endpoint
			request = plan.Metadata.RequestJSON
		}
		if p.File != name || p.Endpoint != endpoint || p.RequestJSON != request || p.RequestSHA256 != digest([]byte(request)) {
			return out, errors.New("capture request or filename differs from its scope")
		}
		if _, err = time.Parse(time.RFC3339Nano, p.RetrievedAt); err != nil {
			return out, err
		}
		b, e := read(name, MaxResponse)
		if e != nil {
			return out, e
		}
		if digest(b) != p.SHA256 || len(b) != p.Bytes {
			return out, errors.New("capture source checksum or size changed")
		}
		var doc map[string]json.RawMessage
		_ = json.Unmarshal(b, &doc)
		if i == 0 {
			page, e := parsePage(ctx, plan.Metadata, Evidence{ResponseJSON: string(b)})
			if e != nil {
				return out, e
			}
			if len(page.Records) != 1 || page.Records[0].ID != m.Spec.Repertoire || p.Records != 1 {
				return out, errors.New("metadata scope changed")
			}
			out.Study, out.Subject = page.Records[0].Study, page.Records[0].Subject
			var row map[string]json.RawMessage
			_ = json.Unmarshal([]byte(page.Records[0].RawJSON), &row)
			var processing []map[string]json.RawMessage
			if json.Unmarshal(row["data_processing"], &processing) != nil {
				return out, errors.New("missing processing metadata")
			}
			n := 0
			for _, x := range processing {
				id, _ := text(x, "data_processing_id")
				if id == m.Spec.Processing {
					n++
				}
			}
			if n != 1 {
				return out, errors.New("ambiguous processing scope")
			}
			continue
		}
		if last == 0 {
			return out, errors.New("pages after a terminal empty page")
		}
		data := doc["Rearrangement"]
		if len(data) == 0 || data[0] != '[' {
			return out, errors.New("missing Rearrangement array")
		}
		var rows []map[string]json.RawMessage
		if json.Unmarshal(data, &rows) != nil || len(rows) != p.Records || len(rows) > m.Spec.Size {
			return out, errors.New("invalid capture page cardinality")
		}
		for _, row := range rows {
			id, e := text(row, "sequence_id")
			if e != nil || id == "" || seen[id] {
				return out, errors.New("ambiguous sequence identity")
			}
			seen[id] = true
			for _, pair := range [][2]string{{"repertoire_id", m.Spec.Repertoire}, {"data_processing_id", m.Spec.Processing}} {
				v, e := text(row, pair[0])
				if e != nil || v != pair[1] {
					return out, errors.New("mixed capture scope")
				}
			}
		}
		out.Rows = append(out.Rows, rows...)
		last = len(rows)
		total += last
	}
	if total != m.Records || !(m.Stop == "empty-page" && last == 0 || m.Stop == "page-budget" && last > 0 && len(m.Pages) == m.Spec.Pages+1) {
		return out, errors.New("capture stop/count evidence is inconsistent")
	}
	return out, nil
}

// Native manifests have an exact schema. In particular, Go's case-insensitive
// struct-field aliases must not let a second spelling silently override evidence.
func decodeCaptureManifest(raw []byte, m *DownloadManifest) error {
	check := func(raw []byte, fields string) (map[string]json.RawMessage, error) {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, errors.New("capture manifest object is missing")
		}
		allowed := map[string]bool{}
		for _, field := range strings.Fields(fields) {
			allowed[field] = true
			if value, ok := obj[field]; !ok || string(value) == "null" {
				return nil, fmt.Errorf("capture manifest field %q is missing or null", field)
			}
		}
		for field := range obj {
			if !allowed[field] {
				return nil, fmt.Errorf("unsupported capture manifest field %q", field)
			}
		}
		return obj, nil
	}
	obj, err := check(raw, "version contract registryRevision spec pages records stop completeness notice")
	if err != nil {
		return err
	}
	if _, err = check(obj["spec"], "repository repertoire processing license citation pages size accept"); err != nil {
		return err
	}
	var pages []json.RawMessage
	if err = json.Unmarshal(obj["pages"], &pages); err != nil {
		return err
	}
	for _, page := range pages {
		if _, err = check(page, "file requestJSON requestSHA256 endpoint retrievedAt sha256 bytes records"); err != nil {
			return err
		}
	}
	return json.Unmarshal(raw, m)
}

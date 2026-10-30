package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"clonoloom/internal/reads"
)

// Captured evidence only; no path from a query is opened and no numeric source
// field is decoded through a JSON number.
func (a *app) handleNativeReport(w http.ResponseWriter, r *http.Request) {
	ds, ok := a.datasets.get(r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Open the dataset again to inspect its native report evidence.")
		return
	}
	parse := func(key string, fallback int) (int, bool) {
		values, ok := r.URL.Query()[key]
		if !ok {
			return fallback, true
		}
		if len(values) != 1 {
			return 0, false
		}
		n, err := strconv.Atoi(values[0])
		return n, err == nil
	}
	source, valid := parse("source", -1)
	if !valid || source < 0 || source >= len(ds.Result.Sources) || ds.Result.Sources[source].Native == nil {
		writeError(w, 400, "source must identify one captured native report")
		return
	}
	s := ds.Result.Sources[source]
	n := s.Native
	if _, detail := r.URL.Query()["record"]; detail {
		index, valid := parse("record", -1)
		if !valid || index < 0 || index >= len(n.Rows) {
			writeError(w, 400, "record must identify one captured row")
			return
		}
		row := n.Rows[index]
		if len(row.Text)+len(n.Header) > 4<<20 {
			writeError(w, 422, "The row and header exceed the 4 MiB preview limit; evidence was retained without truncation.")
			return
		}
		if n.Layout == reads.CellRangerJSONLayout {
			var fields map[string]json.RawMessage
			if json.Unmarshal([]byte(row.Text), &fields) != nil {
				writeError(w, 422, "Captured JSON evidence is unavailable; reopen the source.")
				return
			}
			values := []map[string]string{}
			for _, key := range []string{"contig_name", "barcode", "sample", "is_cell", "high_confidence", "productive", "full_length", "read_count", "umi_count", "cdr3", "cdr3_seq"} {
				value := "(absent)"
				if raw, ok := fields[key]; ok {
					value = string(raw)
				}
				values = append(values, map[string]string{"name": key, "value": value})
			}
			writeJSON(w, map[string]any{"record": index + 1, "rawJSON": row.Text, "fields": values})
			return
		}
		writeJSON(w, map[string]any{"line": row.Line, "headerText": n.Header, "rowText": row.Text,
			"columns": s.Header, "values": nativeReportValues(row.Text)})
		return
	}
	offset, v1 := parse("offset", 0)
	limit, v2 := parse("limit", 20)
	if !v1 || !v2 || offset < 0 || offset > len(n.Rows) || limit < 1 || limit > 50 {
		writeError(w, 400, "offset must be within the source and limit must be 1–50")
		return
	}
	rows := []map[string]any{}
	for i := offset; i < len(n.Rows) && i-offset < limit; i++ {
		rows = append(rows, map[string]any{"index": i, "line": n.Rows[i].Line, "id": n.Rows[i].ID})
	}
	writeJSON(w, map[string]any{"source": source, "name": s.Name, "format": s.Format, "layout": n.Layout, "sha256": s.SHA256, "total": len(n.Rows), "rows": rows})
}

// Match the native reader's single-line TSV quoting, preserving all other text.
func nativeReportValues(line string) []string {
	values := strings.Split(line, "\t")
	for i, v := range values {
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			values[i] = strings.ReplaceAll(v[1:len(v)-1], `""`, `"`)
		}
	}
	return values
}

package main

// Offline reports retain the siblings' traceable tables/methods pattern. Go owns
// evidence and exact table values; the browser only checks bytes and places marks.
import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
	"clonoloom/internal/export"
)

const reportMaxBytes = 64 << 20
const reportLimits = "Integrity only: the browser checks embedded report content and download bytes, without scientific recomputation. Original source files, separate parent files, study design, statistical calibration, authenticity and biological validity are not verified. Captured evidence can contain identifiers, sequences and paths; it is not de-identified. Provider terms still apply."

type reportResult struct {
	Artifact    string `json:"artifact"`
	Kind        string `json:"kind"`
	Certificate string `json:"certificate"`
	Fingerprint string `json:"fingerprint"`
	Result      string `json:"result"`
}
type reportRecord struct {
	Format   string             `json:"format"`
	Version  int                `json:"version"`
	Scope    string             `json:"scope"`
	Software artifacts.Software `json:"software"`
	Results  []reportResult     `json:"results"`
	Files    []runOutput        `json:"files"`
	Limits   string             `json:"limits"`
}
type reportSelection struct {
	Manifest artifacts.Manifest
	Notice   export.Notice
}

// Refuse the entire report on bounds; no hidden row truncation. Scalar numbers
// remain raw JSON tokens, even when JavaScript cannot represent them exactly.
func reportTable(raw json.RawMessage, caption string, cells *int) (string, error) {
	if err := strictRunJSON(raw, new(any)); err != nil {
		return "", err
	}
	cell := func(value json.RawMessage) string {
		if len(value) == 0 {
			return "(absent)"
		}
		var s string
		if value[0] == '"' && json.Unmarshal(value, &s) == nil {
			return html.EscapeString(s)
		}
		var pretty bytes.Buffer
		json.Indent(&pretty, value, "", "  ")
		return "<pre>" + html.EscapeString(pretty.String()) + "</pre>"
	}
	var rows [][]json.RawMessage
	headers := []string{}
	trim := bytes.TrimSpace(raw)
	if len(trim) > 0 && trim[0] == '[' {
		var array []json.RawMessage
		json.Unmarshal(trim, &array)
		keys := map[string]bool{}
		objects := make([]map[string]json.RawMessage, len(array))
		for i, v := range array {
			if json.Unmarshal(v, &objects[i]) != nil || objects[i] == nil {
				objects = nil
				break
			}
			for k := range objects[i] {
				keys[k] = true
			}
		}
		if objects != nil && len(keys) > 0 {
			for k := range keys {
				headers = append(headers, k)
			}
			sort.Strings(headers)
			for _, o := range objects {
				row := []json.RawMessage{}
				for _, k := range headers {
					row = append(row, o[k])
				}
				rows = append(rows, row)
			}
		} else {
			headers = []string{"Index (zero-based)", "Recorded value"}
			for i, v := range array {
				rows = append(rows, []json.RawMessage{mustJSON(i), v})
			}
		}
	} else {
		var object map[string]json.RawMessage
		if json.Unmarshal(trim, &object) != nil || object == nil {
			return "<p>" + cell(trim) + "</p>", nil
		}
		headers = []string{"Field", "Recorded value"}
		keys := []string{}
		for k := range object {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			rows = append(rows, []json.RawMessage{mustJSON(k), object[k]})
		}
	}
	*cells += len(rows) * len(headers)
	if *cells > 350000 {
		return "", errors.New("report exceeds 350,000 table cells; export fewer results")
	}
	var b strings.Builder
	b.WriteString("<div class=table-wrap tabindex=0><table><caption>" + html.EscapeString(caption) + "</caption><thead><tr>")
	for _, h := range headers {
		b.WriteString("<th scope=col>" + html.EscapeString(h) + "</th>")
	}
	b.WriteString("</tr></thead><tbody>")
	for _, row := range rows {
		b.WriteString("<tr>")
		for _, v := range row {
			b.WriteString("<td>" + cell(bytes.TrimSpace(v)) + "</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table></div>")
	return b.String(), nil
}

func reportSection(m artifacts.Manifest, notice export.Notice, raw, manifest []byte, resultPath string, cells *int) (string, error) {
	var doc map[string]json.RawMessage
	if err := strictRunJSON(raw, &doc); err != nil {
		return "", err
	}
	var format string
	var v int
	json.Unmarshal(doc["format"], &format)
	json.Unmarshal(doc["version"], &v)
	if format != "clonoloom-"+m.Kind || v != 1 {
		return "", errors.New("unsupported saved report document version")
	}
	var description string
	json.Unmarshal(doc["description"], &description)
	var b strings.Builder
	b.WriteString("<section><h2>" + html.EscapeString(m.Kind) + "</h2><p class=mono>" + html.EscapeString(m.ID) + "</p><p>" + html.EscapeString(description) + "</p>")
	support, _ := certificateSupportFor(m.Kind)
	b.WriteString("<p class=callout>" + html.EscapeString(support.Limits) + "</p><h3>Recorded methods and references</h3><p>" + html.EscapeString(m.Lineage.Method.Name) + "</p><p>" + html.EscapeString(m.Lineage.Method.Citation) + "</p>")
	b.WriteString("<div class=figures data-result=\"" + html.EscapeString(resultPath) + "\" data-kind=\"" + html.EscapeString(m.Kind) + "\"></div>")
	b.WriteString("<h3>Complete recorded tables</h3><p>All rows and fields are retained. Numbers below are exact saved JSON tokens, without display rounding; null is unavailable, distinct from an absent field. Nested records retain their complete JSON. Counts, denominators, missingness, exclusions and testing families retain their original scope.</p>")
	keys := []string{}
	for k := range doc {
		if k != "format" && k != "version" && k != "description" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		value := doc[k]
		if k == "analysis" || k == "table" {
			var fields map[string]json.RawMessage
			if json.Unmarshal(value, &fields) != nil || fields == nil {
				return "", errors.New("saved analysis/table must be an object")
			}
			names := []string{}
			for n := range fields {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				table, err := reportTable(fields[n], k+" / "+n, cells)
				if err != nil {
					return "", err
				}
				b.WriteString("<details><summary>" + html.EscapeString(k+" / "+n) + "</summary>" + table + "</details>")
			}
		} else {
			table, err := reportTable(value, k, cells)
			if err != nil {
				return "", err
			}
			b.WriteString("<details><summary>" + html.EscapeString(k) + "</summary>" + table + "</details>")
		}
	}
	var metadata map[string]json.RawMessage
	json.Unmarshal(manifest, &metadata)
	var lineage map[string]json.RawMessage
	json.Unmarshal(metadata["lineage"], &lineage)
	delete(lineage, "job") // The complete captured recipe stays in the original certificate bytes.
	metadata["lineage"] = mustJSON(lineage)
	table, err := reportTable(mustJSON(metadata), "Recorded manifest metadata (recipe in certificate)", cells)
	if err != nil {
		return "", err
	}
	b.WriteString("<details><summary>Recorded provenance and parameters</summary><p>The complete original manifest, including the captured job recipe, remains in the downloadable native certificate. This table omits the job record to avoid duplicating captured input matrices.</p>" + table + "</details>")
	b.WriteString("<details><summary>Source/provider notice and credits</summary><pre>" + html.EscapeString(notice.Text()) + "</pre></details></section>")
	return b.String(), nil
}

func (a *app) makeHTMLReport(ctx context.Context, selected []reportSelection, title string) ([]byte, string, error) {
	if len(selected) < 1 || len(selected) > 8 || len(strings.TrimSpace(title)) == 0 || len(title) > 200 {
		return nil, "", errors.New("choose 1–8 results and a title of 1–200 bytes")
	}
	record := reportRecord{Format: "clonoloom-html-report", Version: 1, Scope: "embedded-report-bytes", Software: artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}, Limits: reportLimits}
	files := map[string][]byte{}
	var body strings.Builder
	body.WriteString("<h1>" + html.EscapeString(title) + "</h1><p>Explicitly selected saved results · ClonoLoom " + html.EscapeString(version) + "</p><p class=callout>" + html.EscapeString(reportLimits) + "</p>")
	var total int64
	cells := 0
	for _, selection := range selected {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		m, n := selection.Manifest, selection.Notice
		if _, ok := certificateSupportFor(m.Kind); !ok {
			return nil, "", errors.New("unsupported report kind")
		}
		if _, ok := files[m.ID+".certificate.clz"]; ok {
			return nil, "", errors.New("duplicate selected result")
		}
		// Check declared sizes before certificate compression or allocating its bytes.
		info, err := os.Stat(filepath.Join(a.artifacts.Dir(), m.ID, "manifest.json"))
		if err != nil {
			return nil, "", err
		}
		declared := info.Size()
		for _, f := range m.Files {
			declared += f.Size
		}
		if declared > 32<<20 {
			return nil, "", errors.New("one report result exceeds 32 MiB of saved evidence")
		}
		archive, cert, err := a.makeCertificate(ctx, m, n)
		if err != nil {
			return nil, "", err
		}
		packed, err := func() ([]byte, error) {
			defer archive.Close()
			defer os.Remove(archive.Name())
			return io.ReadAll(io.LimitReader(runContextReader{ctx, archive}, (33<<20)+1))
		}()
		if err != nil {
			return nil, "", err
		}
		if len(packed) > 33<<20 {
			return nil, "", errors.New("report certificate exceeds 33 MiB")
		}
		checked := certificate.Verify(ctx, bytes.NewReader(packed), int64(len(packed)), cert.Fingerprint)
		if checked.Verdict != "integrity-match" {
			return nil, "", errors.New("report certificate integrity failed")
		}
		z, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
		if err != nil {
			return nil, "", err
		}
		var raw, manifest []byte
		for _, f := range z.File {
			if f.Name == "artifact/"+m.Kind+".json" || f.Name == "manifest.json" {
				in, e := f.Open()
				if e != nil {
					return nil, "", e
				}
				data, e := io.ReadAll(runContextReader{ctx, in})
				in.Close()
				if e != nil {
					return nil, "", e
				}
				if f.Name == "manifest.json" {
					manifest = data
				} else {
					raw = data
				}
			}
		}
		resultPath := m.ID + ".result.json"
		section, err := reportSection(m, n, raw, manifest, resultPath, &cells)
		if err != nil {
			return nil, "", err
		}
		body.WriteString(section)
		if body.Len() > 32<<20 {
			return nil, "", errors.New("report content exceeds 32 MiB; export fewer results")
		}
		files[m.ID+".certificate.clz"] = packed
		files[resultPath] = raw
		total += int64(len(packed) + len(raw))
		if total > reportMaxBytes {
			return nil, "", errors.New("report embedded bytes exceed 64 MiB")
		}
		record.Results = append(record.Results, reportResult{m.ID, m.Kind, m.ID + ".certificate.clz", cert.Fingerprint, resultPath})
	}
	files["report.html"] = []byte(body.String())
	total += int64(body.Len())
	if total > reportMaxBytes {
		return nil, "", errors.New("report embedded bytes exceed 64 MiB")
	}
	// Wrap each existing geometry module to keep its local helpers private. The
	// dependency list is explicit and tested; no downloaded code or dynamic imports.
	modules := []struct{ Path, Name, Exports string }{{"sha256", "hash", "SHA256"}, {"cohort-plot", "cohort", "cohortPoints,cohortEffect"}, {"batch-qc-plot", "batch", "batchQCPlot"}, {"count-plot", "counts", "countMA"}}
	var code strings.Builder
	for _, m := range modules {
		raw, err := content.ReadFile("web/lib/" + m.Path + ".js")
		if err != nil {
			return nil, "", err
		}
		src := strings.ReplaceAll(string(raw), "export ", "")
		src = strings.ReplaceAll(src, "import { cohortPoints } from './cohort-plot.js';", "")
		code.WriteString("const " + m.Name + "=(()=>{\n" + src + "\nreturn {" + m.Exports + "};})();\n")
	}
	geometry := code.String()
	code.Reset()
	code.WriteString("/* report geometry start */\n" + geometry + "/* report geometry end */\n")
	checker, err := content.ReadFile("web/lib/report-check.js")
	if err != nil {
		return nil, "", err
	}
	code.WriteString(strings.ReplaceAll(string(checker), "export ", ""))
	runtime, err := content.ReadFile("web/ui/offline-report.js")
	if err != nil {
		return nil, "", err
	}
	code.Write(runtime)
	style, err := content.ReadFile("web/ui/offline-report.css")
	if err != nil {
		return nil, "", err
	}
	files["geometry.js"] = []byte(geometry)
	files["report.css"] = style
	total += int64(len(geometry) + len(style))
	if total > reportMaxBytes {
		return nil, "", errors.New("report embedded bytes exceed 64 MiB")
	}
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := files[name]
		record.Files = append(record.Files, runOutput{name, int64(len(raw)), replayDigest(raw)})
	}
	recordBytes := mustJSON(record)
	fingerprint := replayDigest(recordBytes)
	var payload strings.Builder
	payload.WriteString(`<script type="application/octet-stream" id="report-record" data-sha256="` + fingerprint + `">` + base64.StdEncoding.EncodeToString(recordBytes) + `</script>`)
	for i, name := range names {
		payload.WriteString(fmt.Sprintf(`<script type="application/octet-stream" id="report-file-%d">%s</script>`, i, base64.StdEncoding.EncodeToString(files[name])))
	}
	// Script hashes allow only this generated checker; metadata never enters code.
	script := code.String()
	scriptHash := sha256.Sum256([]byte(script))
	digest := base64.StdEncoding.EncodeToString(scriptHash[:])
	page := `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'sha256-` + digest + `'; style-src 'unsafe-inline'; img-src data:; connect-src 'none'; base-uri 'none'; form-action 'none'"><title>ClonoLoom saved-result report</title><style>` + string(style) + `</style><body><header><strong>ClonoLoom · saved-result report</strong><button id="theme" type="button">Switch theme</button><p id="status" role="status">Not checked. JavaScript is required to check and display this report.</p><label>Independently retained report SHA-256 (optional)<input id="expected" spellcheck="false"></label><button id="check" type="button">Check embedded bytes</button><p>Report fingerprint: <code id="fingerprint">` + fingerprint + `</code></p><p>Checksums are not signatures. Replacing the contents, checker and hashes can forge an internally consistent report. Retain a fingerprint separately.</p></header><noscript>No check was performed. Open this file in a modern browser with JavaScript enabled, or obtain the native certificates separately.</noscript><main id="report"></main><section id="downloads"></section>` + payload.String() + `<script>` + script + `</script></body></html>`
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	return []byte(page), fingerprint, nil
}

func (a *app) registerHTMLReports(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/reports/html", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Title     string   `json:"title"`
			Artifacts []string `json:"artifacts"`
			Accept    bool     `json:"accept"`
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
		if err == nil {
			err = strictRunJSON(raw, &request)
		}
		if err != nil || len(request.Artifacts) < 1 || len(request.Artifacts) > 8 {
			writeError(w, 400, "Choose 1–8 explicit result identities in a supported report request.")
			return
		}
		selected := []reportSelection{}
		seen := map[string]bool{}
		terms := export.Notice{}
		for _, id := range request.Artifacts {
			if seen[id] {
				writeError(w, 400, "Duplicate selected result.")
				return
			}
			seen[id] = true
			m, n, e := a.certificateNotice(id)
			if e != nil {
				writeError(w, 409, e.Error())
				return
			}
			if n.Restricted() {
				terms.Credits = append(terms.Credits, n.Credits...)
			}
			selected = append(selected, reportSelection{m, n})
		}
		if len(terms.Credits) > 0 && !request.Accept {
			restricted(w, terms)
			return
		}
		page, fingerprint, err := a.makeHTMLReport(r.Context(), selected, request.Title)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="clonoloom-report.html"`)
		w.Header().Set("X-ClonoLoom-Report", fingerprint)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(page)
	})
}

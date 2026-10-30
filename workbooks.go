package main

// Workbooks are a presentation of verified saved evidence, never a new analysis.
// The certificate source/notice contracts stay unchanged; no local source is reopened.
import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"clonoloom/internal/certificate"
	"clonoloom/internal/export"
	"clonoloom/internal/workbook"
)

type workbookResult struct {
	Artifact               string         `json:"artifact"`
	Kind                   string         `json:"kind"`
	ResultSheet            string         `json:"resultSheet"`
	ProvenanceSheet        string         `json:"provenanceSheet"`
	ResultSHA256           string         `json:"resultSHA256"`
	CertificateFingerprint string         `json:"certificateFingerprint"`
	Values                 workbook.Stats `json:"values"`
	Provenance             workbook.Stats `json:"provenance"`
}
type workbookRecord struct {
	Format  string           `json:"format"`
	Version int              `json:"version"`
	Title   string           `json:"title"`
	Results []workbookResult `json:"results"`
	Files   []runOutput      `json:"files"`
	Limits  string           `json:"limits"`
}

func (a *app) makeWorkbook(ctx context.Context, selected []reportSelection, title string) ([]byte, workbookRecord, error) {
	record := workbookRecord{Format: "clonoloom-workbook", Version: 1, Title: title, Limits: workbook.Limits}
	fail := func(err error) ([]byte, workbookRecord, error) { return nil, record, err }
	if len(selected) < 1 || len(selected) > 8 || strings.TrimSpace(title) == "" || len(title) > 200 {
		return fail(errors.New("choose 1–8 results and a title of 1–200 bytes"))
	}
	overview := workbook.Sheet{Name: "Read me", Widths: []int{28, 100}, Rows: [][]workbook.Cell{{workbook.Text("Recorded context"), workbook.Text("Value")}}}
	add := func(label, value string) {
		for _, line := range strings.Split(value, "\n") {
			overview.Rows = append(overview.Rows, []workbook.Cell{workbook.Text(label), workbook.Text(line)})
		}
	}
	add("Title", title)
	add("Software", "ClonoLoom "+version+"; "+buildCommit())
	add("Reading the sheets", "Result sheets index every JSON node using RFC 6901 pointers. Object keys are sorted; array indices retain recorded order. A worksheet row is a JSON node, not a participant or observation. Child count describes the immediate JSON container.")
	add("Precision", workbook.Limits)
	add("Excel numeric view", "Numbers with at most 15 significant digits and magnitude zero or 1e-300 through 1e300 use numeric cells. All other numeric values, including negative zero, use text. Exact JSON scalar cells always retain original token text. Do not coerce them to Excel numbers when exactness matters.")
	add("Missingness", "Null has a node row and an empty value cell. An empty string has type string and exact token \"\". Empty arrays/objects have child count zero. An absent field has no path row. Unavailable measurements are not zero.")
	add("Evidence", "Keep the ZIP companions: unchanged result JSON, original manifest, provider NOTICE and native certificate for each selection, plus inventory hashes. Workbook edits do not update companion evidence or certify replay. Use native certificates for their existing explicit verification/replay workflow.")
	sheets := []workbook.Sheet{overview}
	files := map[string][]byte{}
	budget := workbook.MaxNodes
	total := 0
	seen := map[string]bool{}
	for i, selection := range selected {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		m, n := selection.Manifest, selection.Notice
		if _, ok := certificateSupportFor(m.Kind); !ok || seen[m.ID] {
			return fail(errors.New("unsupported or repeated workbook result"))
		}
		seen[m.ID] = true
		st, err := os.Stat(filepath.Join(a.artifacts.Dir(), m.ID, "manifest.json"))
		if err != nil {
			return fail(err)
		}
		declared := st.Size()
		for _, f := range m.Files {
			declared += f.Size
		}
		if declared > 32<<20 {
			return fail(errors.New("one workbook result exceeds 32 MiB of saved evidence"))
		}
		archive, cert, err := a.makeCertificate(ctx, m, n)
		if err != nil {
			return fail(err)
		}
		packed, err := func() ([]byte, error) {
			defer archive.Close()
			defer os.Remove(archive.Name())
			return io.ReadAll(io.LimitReader(runContextReader{ctx, archive}, (33<<20)+1))
		}()
		if err != nil {
			return fail(err)
		}
		if len(packed) > 33<<20 {
			return fail(errors.New("workbook certificate exceeds 33 MiB"))
		}
		checked := certificate.Verify(ctx, bytes.NewReader(packed), int64(len(packed)), cert.Fingerprint)
		if checked.Verdict != "integrity-match" {
			return fail(errors.New("workbook source certificate failed verification"))
		}
		z, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
		if err != nil {
			return fail(err)
		}
		var raw, manifest []byte
		for _, f := range z.File {
			if f.Name != "artifact/"+m.Kind+".json" && f.Name != "manifest.json" {
				continue
			}
			in, e := f.Open()
			if e != nil {
				return fail(e)
			}
			data, e := io.ReadAll(runContextReader{ctx, in})
			in.Close()
			if e != nil {
				return fail(e)
			}
			if f.Name == "manifest.json" {
				manifest = data
			} else {
				raw = data
			}
		}
		var doc struct {
			Format  string `json:"format"`
			Version int    `json:"version"`
		}
		if json.Unmarshal(raw, &doc) != nil || doc.Format != "clonoloom-"+m.Kind || doc.Version != 1 {
			return fail(errors.New("unsupported saved workbook result version"))
		}
		prefix := fmt.Sprintf("R%d", i+1)
		values, vs, err := workbook.Evidence(ctx, raw, prefix+" values", &budget)
		if err != nil {
			return fail(err)
		}
		provenance, ps, err := workbook.Evidence(ctx, manifest, prefix+" provenance", &budget)
		if err != nil {
			return fail(err)
		}
		sheets = append(sheets, values, provenance)
		add(prefix+" result", m.Kind+" · "+m.ID)
		add(prefix+" sheets", values.Name+"; "+provenance.Name)
		add(prefix+" method", m.Lineage.Method.Name+"; version "+m.Lineage.Method.Version)
		add(prefix+" citation", m.Lineage.Method.Citation)
		support, _ := certificateSupportFor(m.Kind)
		add(prefix+" scientific limits", support.Limits)
		add(prefix+" source notice", n.Text())
		files[prefix+".result.json"] = raw
		files[prefix+".manifest.json"] = manifest
		files[prefix+".NOTICE.txt"] = []byte(n.Text())
		files[prefix+".certificate.clz"] = packed
		total += len(raw) + len(manifest) + len(packed) + len(n.Text())
		if total > 64<<20 {
			return fail(errors.New("workbook companion evidence exceeds 64 MiB"))
		}
		record.Results = append(record.Results, workbookResult{m.ID, m.Kind, values.Name, provenance.Name, replayDigest(raw), cert.Fingerprint, vs, ps})
	}
	sheets[0] = overview
	xlsx, err := workbook.Write(ctx, sheets)
	if err != nil {
		return fail(err)
	}
	files["results.xlsx"] = xlsx
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := files[name]
		record.Files = append(record.Files, runOutput{name, int64(len(raw)), replayDigest(raw)})
	}
	files["workbook.json"] = mustJSON(record)
	names = append(names, "workbook.json")
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for _, name := range names {
		if err = ctx.Err(); err != nil {
			return fail(err)
		}
		f, e := z.Create(name)
		if e != nil {
			return fail(e)
		}
		if _, e = f.Write(files[name]); e != nil {
			return fail(e)
		}
	}
	if err = z.Close(); err != nil {
		return fail(err)
	}
	return out.Bytes(), record, nil
}
func (a *app) registerWorkbooks(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/reports/workbook", func(w http.ResponseWriter, r *http.Request) {
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
			writeError(w, 400, "Choose 1–8 explicit saved results in a supported workbook request.")
			return
		}
		selected := []reportSelection{}
		terms := export.Notice{}
		seen := map[string]bool{}
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
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		data, record, err := a.makeWorkbook(ctx, selected, request.Title)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="clonoloom-workbook.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-ClonoLoom-Workbook", replayDigest(data))
		w.Header().Set("X-ClonoLoom-Workbook-Results", fmt.Sprint(len(record.Results)))
		w.Write(data)
	})
}

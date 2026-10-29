package main

// Prism exports retain original evidence beside a subject-level interchange.
import (
	"archive/zip"
	"bytes"
	"context"
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
	"clonoloom/internal/prism"
)

type prismRecord struct {
	Software               string      `json:"software"`
	Format                 string      `json:"format"`
	Version                int         `json:"version"`
	Artifact               string      `json:"artifact"`
	Kind                   string      `json:"kind"`
	Title                  string      `json:"title"`
	ResultSHA256           string      `json:"resultSHA256"`
	CertificateFingerprint string      `json:"certificateFingerprint"`
	Files                  []runOutput `json:"files"`
	Limits                 string      `json:"limits"`
}

func (a *app) makePrism(ctx context.Context, s reportSelection, title string) ([]byte, prismRecord, error) {
	m, n := s.Manifest, s.Notice
	record := prismRecord{Software: "ClonoLoom " + version + "; " + buildCommit(), Format: "clonoloom-prism", Version: 1, Artifact: m.ID, Kind: m.Kind, Title: title, Limits: prism.Limits}
	fail := func(e error) ([]byte, prismRecord, error) { return nil, record, e }
	if (m.Kind != cohortFeaturesKind && m.Kind != cohortCompareKind) || strings.TrimSpace(title) == "" || len(title) > 200 {
		return fail(errors.New("choose one saved subject-feature/comparison result and a title of 1–200 bytes"))
	}
	st, e := os.Stat(filepath.Join(a.artifacts.Dir(), m.ID, "manifest.json"))
	if e != nil {
		return fail(e)
	}
	size := st.Size()
	for _, f := range m.Files {
		size += f.Size
	}
	if size > 32<<20 {
		return fail(errors.New("Prism saved evidence exceeds 32 MiB"))
	}
	source, cert, e := a.makeCertificate(ctx, m, n)
	if e != nil {
		return fail(e)
	}
	defer source.Close()
	defer os.Remove(source.Name())
	packed, e := io.ReadAll(io.LimitReader(runContextReader{ctx, source}, (33<<20)+1))
	if e != nil {
		return fail(e)
	}
	if len(packed) > 33<<20 {
		return fail(errors.New("Prism certificate exceeds 33 MiB"))
	}
	verified := certificate.Verify(ctx, bytes.NewReader(packed), int64(len(packed)), cert.Fingerprint)
	if verified.Verdict != "integrity-match" {
		return fail(errors.New("Prism source certificate failed verification"))
	}
	z, e := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	if e != nil {
		return fail(e)
	}
	files := map[string][]byte{"certificate.clz": packed, "NOTICE.txt": []byte(n.Text())}
	for _, f := range z.File {
		dest := ""
		if f.Name == "manifest.json" {
			dest = "manifest.json"
		}
		if f.Name == "artifact/"+m.Kind+".json" {
			dest = "result.json"
		}
		if dest == "" {
			continue
		}
		r, e := f.Open()
		if e != nil {
			return fail(e)
		}
		raw, e := io.ReadAll(runContextReader{ctx, r})
		r.Close()
		if e != nil {
			return fail(e)
		}
		files[dest] = raw
	}
	mapping, e := prism.Map(ctx, files["result.json"])
	if e != nil {
		return fail(e)
	}
	notes := fmt.Sprintf("Design %s; experimental unit %s; layout %s; grouping field %s.\nReference: %s; comparison: %s.\n%d excluded/outside-contrast samples remain in mapping.json and native evidence.\nMethod: %s; version %s.\nCitation: %s.\n%s", mapping.DesignID, mapping.Unit, mapping.Layout, mapping.GroupField, mapping.Levels[0], mapping.Levels[1], len(mapping.Excluded), m.Lineage.Method.Name, m.Lineage.Method.Version, m.Lineage.Method.Citation, n.Text())
	files["data.pzfx"], e = prism.Write(ctx, mapping, title, notes, version)
	if e != nil {
		return fail(e)
	}
	files["mapping.json"] = mustJSON(mapping)
	files["README.txt"] = []byte("ClonoLoom Prism subject-table interchange\n\n" + prism.Limits + "\n\nOpen data.pzfx in Prism 7 or later. Read its Info sheet and mapping.json before analyzing. The file has Column data tables only, one per feature; no analyses or graphs are configured. Paired rows are aligned by explicit subject ID. Independent rows must not be analyzed as matched pairs. Blank values are not zero. Keep row titles and both group columns together if sorting. Do not compact away missing values.\n\nOriginal numeric tokens are written unchanged, but Prism may round imported values. Display precision is not storage precision. Native JSON/certificates remain authoritative. XML/schema and independent-reader checks do not certify a native Prism import or an equivalent statistical analysis.\n\n" + notes + "\n")
	record.ResultSHA256, record.CertificateFingerprint = replayDigest(files["result.json"]), cert.Fingerprint
	names := []string{}
	total := 0
	for name, b := range files {
		names = append(names, name)
		total += len(b)
	}
	if total > 64<<20 {
		return fail(errors.New("Prism export exceeds 64 MiB uncompressed"))
	}
	sort.Strings(names)
	for _, name := range names {
		b := files[name]
		record.Files = append(record.Files, runOutput{name, int64(len(b)), replayDigest(b)})
	}
	files["prism.json"] = mustJSON(record)
	if total+len(files["prism.json"]) > 64<<20 {
		return fail(errors.New("Prism export exceeds 64 MiB uncompressed"))
	}
	names = append(names, "prism.json")
	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	for _, name := range names {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		w, e := archive.Create(name)
		if e != nil {
			return fail(e)
		}
		if _, e = w.Write(files[name]); e != nil {
			return fail(e)
		}
	}
	if e := archive.Close(); e != nil {
		return fail(e)
	}
	return out.Bytes(), record, nil
}
func (a *app) registerPrism(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/reports/prism", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Artifact string `json:"artifact"`
			Title    string `json:"title"`
			Accept   bool   `json:"accept"`
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
		if e == nil {
			e = strictRunJSON(raw, &request)
		}
		if e != nil {
			writeError(w, 400, "Choose one explicit saved result in a supported Prism request.")
			return
		}
		m, n, e := a.certificateNotice(request.Artifact)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		if m.Kind != cohortFeaturesKind && m.Kind != cohortCompareKind {
			writeError(w, 400, "Prism maps subject features and cohort comparisons only.")
			return
		}
		if n.Restricted() && !request.Accept {
			restricted(w, n)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		data, _, e := a.makePrism(ctx, reportSelection{m, n}, request.Title)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="clonoloom-prism.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-ClonoLoom-Prism", replayDigest(data))
		w.Write(data)
	})
}

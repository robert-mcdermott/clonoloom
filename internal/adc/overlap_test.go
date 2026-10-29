package adc

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPublicOverlapObservedSets(t *testing.T) {
	seq := func(j string) Sequence { return Sequence{"TRB", j, true} }
	q := []Sequence{seq("CASSF"), seq("CASRF"), seq("CASSF"), seq("CAXXF")}
	capture := func(subject string, aa ...string) Capture {
		c := Capture{Study: "study", Subject: subject, Manifest: DownloadManifest{Spec: DownloadSpec{Repository: "repo", Repertoire: subject, Processing: "P"}, Stop: "page-budget"}}
		for _, j := range aa {
			c.Rows = append(c.Rows, map[string]json.RawMessage{"locus": json.RawMessage(`"TRB"`), "junction_aa": json.RawMessage(`"` + j + `"`), "productive": json.RawMessage(`true`)})
		}
		return c
	}
	r, e := CompareCaptures(context.Background(), q, "TRB", map[string]Capture{"a": capture("A", "CASSF", "CATSF"), "b": capture("B", "CAXXF"), "c": capture("", "CASRF")})
	if e != nil {
		t.Fatal(e)
	}
	// Hand calculation: A={CASSF,CASRF}, B={CASSF,CATSF}; intersection 1, union 3.
	if r.Query.Unique != 2 || r.Query.Excluded != 1 || r.Pairs[0].Shared != 1 || r.Pairs[0].Union != 3 || *r.Pairs[0].Jaccard != 1.0/3 || r.Pairs[1].Jaccard != nil || r.AssignedSubjects != 2 || r.EvaluableSubjects != 1 || r.MissingSubjects != 1 || r.UnassignedCaptures != 1 || r.QueryKeysShared != 2 {
		t.Fatalf("%+v", r)
	}
	c := capture("A")
	d := capture("A")
	d.Manifest.Spec.Repertoire = "different-visit"
	if _, e = CompareCaptures(context.Background(), q, "TRB", map[string]Capture{"a": c, "b": d}); e == nil {
		t.Fatal("pooled subject")
	}
	for _, bad := range []Sequence{{"TRB", "CASSF", false}, {"TRA", "CASSF", true}, {"TRB", "ASSF", true}, {"TRB", "CASSX", true}, {"TRB", "cassf", true}} {
		if SequenceKey(bad, "TRB") != "" {
			t.Fatal("invalid eligibility")
		}
	}
}
func TestCaptureVerification(t *testing.T) {
	parent := t.TempDir()
	c := Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		body := downloadMetadata
		if strings.HasSuffix(r.URL.Path, "rearrangement") {
			body = `{"Rearrangement":[` + downloadRow + `]}`
		}
		return response(200, body), nil
	})}
	s := downloadSpec()
	s.Pages = 1
	out, e := c.Download(context.Background(), parent, s)
	if e != nil {
		t.Fatal(e)
	}
	checked, e := ReadCapture(context.Background(), out.Path, 64<<20)
	if e != nil || len(checked.Rows) != 1 {
		t.Fatalf("%+v %v", checked, e)
	}
	file := filepath.Join(out.Path, "page-001.json")
	original, _ := os.ReadFile(file)
	os.WriteFile(file, []byte(`{"Rearrangement":[]}`), 0600)
	if _, e = ReadCapture(context.Background(), out.Path, 64<<20); e == nil {
		t.Fatal("accepted altered source")
	}
	os.WriteFile(file, original, 0600)
	target := filepath.Join(parent, "external.json")
	os.WriteFile(target, original, 0600)
	os.Remove(file)
	if e = os.Symlink(target, file); e != nil {
		t.Skip(e)
	}
	if _, e = ReadCapture(context.Background(), out.Path, 64<<20); e == nil {
		t.Fatal("followed source symlink")
	}
}

func TestPublicOverlapIndependentReference(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "public-overlap")
	f, err := os.Open(filepath.Join(base, "query.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	all, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	query := []Sequence{}
	for _, row := range all[1:] {
		query = append(query, Sequence{row[2], row[3], row[4] == "T"})
	}
	captures := map[string]Capture{}
	for _, name := range []string{"a", "b", "c", "d"} {
		name = "adc-download-sim-" + name
		c, err := ReadCapture(context.Background(), filepath.Join(base, name), 64<<20)
		if err != nil {
			t.Fatal(err)
		}
		captures[name] = c
	}
	got, err := CompareCaptures(context.Background(), query, "TRB", captures)
	if err != nil {
		t.Fatal(err)
	}
	var want PublicOverlap
	b, err := os.ReadFile(filepath.Join(base, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.SharedKeys, want.SharedKeys) || got.Query != want.Query || got.QueryKeysShared != want.QueryKeysShared || got.AssignedSubjects != want.AssignedSubjects || got.EvaluableSubjects != want.EvaluableSubjects || got.MissingSubjects != want.MissingSubjects || got.UnassignedCaptures != want.UnassignedCaptures {
		t.Fatalf("summary %+v; want %+v", got, want)
	}
	for i, p := range got.Pairs {
		w := want.Pairs[i]
		if p.Capture != w.Capture || p.Eligibility != w.Eligibility || p.Shared != w.Shared || p.Union != w.Union || !reflect.DeepEqual(p.Jaccard, w.Jaccard) {
			t.Fatalf("pair %+v; want %+v", p, w)
		}
	}
	empty, err := CompareCaptures(context.Background(), nil, "TRB", captures)
	if err != nil || empty.EvaluableSubjects != 0 || empty.MissingSubjects != 3 {
		t.Fatalf("empty query %+v %v", empty, err)
	}
	for _, p := range empty.Pairs {
		if p.Jaccard != nil {
			t.Fatal("empty query became biological zero")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = CompareCaptures(ctx, query, "TRB", captures); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err = ReadCapture(ctx, filepath.Join(base, "adc-download-sim-a"), 64<<20); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err = ReadCapture(context.Background(), filepath.Join(base, "adc-download-sim-a"), 1); err == nil {
		t.Fatal("ignored byte budget")
	}
}

func TestCaptureManifestRefusals(t *testing.T) {
	source := filepath.Join("..", "..", "testdata", "public-overlap", "adc-download-sim-b")
	for _, test := range []struct {
		name   string
		change func(*DownloadManifest)
	}{
		{"request", func(m *DownloadManifest) { m.Pages[1].RequestJSON = `{}` }},
		{"path", func(m *DownloadManifest) { m.Pages[1].File = "../page-001.json" }},
		{"stop", func(m *DownloadManifest) { m.Stop = "page-budget" }},
		{"counts", func(m *DownloadManifest) { m.Records++ }},
		{"scope", func(m *DownloadManifest) { m.Spec.Processing = "OTHER" }},
		{"complete", func(m *DownloadManifest) { m.Completeness = "complete" }},
		{"permission", func(m *DownloadManifest) { m.Spec.Accept = false }},
		{"timestamp", func(m *DownloadManifest) { m.Pages[0].RetrievedAt = "yesterday" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture")
			if err := os.CopyFS(path, os.DirFS(source)); err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(filepath.Join(path, "manifest.json"))
			var m DownloadManifest
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			test.change(&m)
			raw, _ = json.Marshal(m)
			if err := os.WriteFile(filepath.Join(path, "manifest.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadCapture(context.Background(), path, 64<<20); err == nil {
				t.Fatal("accepted inconsistent evidence")
			}
		})
	}
}

func TestPublicOverlapPresenceAndScopeControls(t *testing.T) {
	query := []Sequence{{"IGH", "CARQGW", true}}
	makeCapture := func(rep, study, subject, aa string) Capture {
		return Capture{Manifest: DownloadManifest{Spec: DownloadSpec{Repository: "repo", Repertoire: rep, Processing: "P"}}, Study: study, Subject: subject, Rows: []map[string]json.RawMessage{{"locus": json.RawMessage(`"IGH"`), "productive": json.RawMessage(`true`), "junction_aa": json.RawMessage(`"` + aa + `"`)}}}
	}
	a := makeCapture("A", "study", "S1", "CARGW")
	b := makeCapture("B", "study", "S2", "CARQGW")
	b.Rows = append(b.Rows, b.Rows[0]) // duplicated eligible sequence key, separate record IDs are checked on read
	r, err := CompareCaptures(context.Background(), query, "IGH", map[string]Capture{"a": a, "b": b})
	if err != nil {
		t.Fatal(err)
	}
	if *r.Pairs[0].Jaccard != 0 || *r.Pairs[1].Jaccard != 1 || len(r.SharedKeys) != 1 || r.SharedKeys[0].Subjects != 1 || r.SharedKeys[0].Captures != 1 {
		t.Fatalf("%+v", r)
	}
	b.Manifest.Spec = a.Manifest.Spec
	if _, err = CompareCaptures(context.Background(), query, "IGH", map[string]Capture{"a": a, "b": b}); err == nil {
		t.Fatal("duplicate scope accepted")
	}
}

func TestCaptureManifestExactKeys(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "public-overlap", "adc-download-sim-a")
	raw, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{`"spec"`, `"Spec"`}, {`"repertoire"`, `"Repertoire"`}, {`"sha256"`, `"SHA256"`}, {`"records": 48`, `"records": null`}} {
		var m DownloadManifest
		if err := decodeCaptureManifest([]byte(strings.Replace(string(raw), pair[0], pair[1], 1)), &m); err == nil {
			t.Fatal("case alias accepted", pair)
		}
	}
}

func TestCaptureRehashedScopeRefusals(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "public-overlap", "adc-download-sim-a")
	for _, kind := range []string{"repertoire", "processing", "duplicate-sequence", "duplicate-processing"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture")
			if err := os.CopyFS(path, os.DirFS(base)); err != nil {
				t.Fatal(err)
			}
			manifest, _ := os.ReadFile(filepath.Join(path, "manifest.json"))
			var m DownloadManifest
			if err := json.Unmarshal(manifest, &m); err != nil {
				t.Fatal(err)
			}
			i := 1
			if kind == "duplicate-processing" {
				i = 0
			}
			file := filepath.Join(path, m.Pages[i].File)
			raw, _ := os.ReadFile(file)
			var doc map[string]json.RawMessage
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				var meta []map[string]json.RawMessage
				json.Unmarshal(doc["Repertoire"], &meta)
				meta[0]["data_processing"] = json.RawMessage(`[{"data_processing_id":"SIM-process"},{"data_processing_id":"SIM-process"}]`)
				doc["Repertoire"], _ = json.Marshal(meta)
			} else {
				var rows []map[string]json.RawMessage
				json.Unmarshal(doc["Rearrangement"], &rows)
				switch kind {
				case "repertoire":
					rows[0]["repertoire_id"] = json.RawMessage(`"OTHER"`)
				case "processing":
					rows[0]["data_processing_id"] = json.RawMessage(`"OTHER"`)
				case "duplicate-sequence":
					rows[1]["sequence_id"] = rows[0]["sequence_id"]
				}
				doc["Rearrangement"], _ = json.Marshal(rows)
			}
			raw, _ = json.Marshal(doc)
			m.Pages[i].SHA256 = digest(raw)
			m.Pages[i].Bytes = len(raw)
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			manifest, _ = json.Marshal(m)
			if err := os.WriteFile(filepath.Join(path, "manifest.json"), manifest, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadCapture(context.Background(), path, 64<<20); err == nil {
				t.Fatal("accepted inconsistent scope despite self-consistent hashes")
			}
		})
	}
}

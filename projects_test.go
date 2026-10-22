package main

// Every release's project keeps opening (clonoloom-spec/roadmap.md, "Projects that keep opening"):
// each project under validation/projects/<version>/ is read as that release saved it, its datasets
// opened again from the fixture's files under the same IDs, and each result it kept made again from
// what its manifest records, under the same artifact ID, and compared with the document that
// release wrote.
//
// The documents are compared value by value, numbers to a relative 1e-9, rather than by the
// checksums their manifests record: results are not yet the same on every processor (Go fuses
// multiply-adds on arm64, not on amd64, and its logarithm and exponential differ between
// processors), so the last digits of a number can differ between the computer that saved a fixture
// and the one testing it (validation/projects/README.md). Whether the bytes are the same is logged.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"clonoloom/internal/reads"
)

// fixtureProject is what the test reads of a saved project.
type fixtureProject struct {
	Datasets []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Sources []struct {
			Path    string         `json:"path"`
			SHA256  string         `json:"sha256"`
			Mapping *reads.Mapping `json:"mapping"`
		} `json:"sources"`
		Metadata []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"metadata"`
	} `json:"datasets"`
	Artifacts []struct {
		ID      string `json:"id"`
		Kind    string `json:"kind"`
		Dataset string `json:"dataset"`
	} `json:"artifacts"`
}

func TestEveryReleasesProjectStillOpens(t *testing.T) {
	projects, _ := filepath.Glob(filepath.Join("validation", "projects", "*", "project.json"))
	if len(projects) == 0 {
		t.Fatal("no project under validation/projects")
	}
	for _, path := range projects {
		dir := filepath.Dir(path)
		t.Run(filepath.Base(dir), func(t *testing.T) { reopenFixture(t, dir) })
	}
}

func reopenFixture(t *testing.T, dir string) {
	raw, err := os.ReadFile(filepath.Join(dir, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p fixtureProject
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	a, handler := testApp(t)
	for _, ds := range p.Datasets {
		var paths []string
		mappings := map[string]*reads.Mapping{}
		add := func(path, sum string, m *reads.Mapping) {
			abs, _ := filepath.Abs(filepath.Join(dir, filepath.FromSlash(path)))
			if got := fileSHA256(abs); got != sum {
				t.Fatalf("%s: the fixture's file has changed (SHA-256 %s, the project records %s)", path, got, sum)
			}
			paths = append(paths, abs)
			if m != nil {
				mappings[abs] = m
			}
		}
		for _, s := range ds.Sources {
			add(s.Path, s.SHA256, s.Mapping)
		}
		for _, m := range ds.Metadata {
			add(m.Path, m.SHA256, nil)
		}
		body := map[string]any{"paths": paths, "name": ds.Name}
		if len(mappings) > 0 {
			body["mappings"] = mappings
		}
		rec := postJSON(t, handler, "/api/datasets/open", string(mustJSON(body)))
		var s datasetSummary
		json.Unmarshal(rec.Body.Bytes(), &s)
		if rec.Code != http.StatusOK || s.ID != ds.ID {
			t.Fatalf("%s opens as %q, not %q: %d %s", ds.Name, s.ID, ds.ID, rec.Code, rec.Body)
		}
	}
	for _, kept := range p.Artifacts {
		t.Run(kept.ID, func(t *testing.T) {
			var m struct {
				Lineage struct {
					Parameters json.RawMessage `json:"parameters"`
				} `json:"lineage"`
				Files []struct {
					Path   string `json:"path"`
					SHA256 string `json:"sha256"`
				} `json:"files"`
			}
			raw, err := os.ReadFile(filepath.Join(dir, "artifacts", kept.ID, "manifest.json"))
			if err == nil {
				err = json.Unmarshal(raw, &m)
			}
			if err != nil {
				t.Fatal(err)
			}
			rec := postJSON(t, handler, "/api/"+kept.Kind, string(m.Lineage.Parameters))
			var started struct {
				Artifact string `json:"artifact"`
				Job      string `json:"job"`
			}
			json.Unmarshal(rec.Body.Bytes(), &started)
			if rec.Code != http.StatusOK || started.Artifact != kept.ID {
				t.Fatalf("made again, the result is %q, not %q (what it measures or its method's version changed): %d %s", started.Artifact, kept.ID, rec.Code, rec.Body)
			}
			waitForJob(t, a, started.Job, time.Minute)
			for _, f := range m.Files {
				want := gunzipFile(t, filepath.Join(dir, "artifacts", kept.ID, f.Path+".gz"))
				if sum := sha256.Sum256(want); hex.EncodeToString(sum[:]) != f.SHA256 {
					t.Fatalf("%s: the fixture's document is not the one its manifest records", f.Path)
				}
				path, err := a.artifacts.Path(kept.ID, f.Path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(got, want) {
					t.Logf("%s: the same bytes", f.Path)
					continue
				}
				t.Logf("%s: not the same bytes; comparing the values", f.Path)
				var w, g any
				if err := json.Unmarshal(want, &w); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(got, &g); err != nil {
					t.Fatal(err)
				}
				var c valueComparison
				c.compare(f.Path, w, g)
				for _, d := range c.differences {
					t.Error(d)
				}
				t.Logf("%s: the largest difference is %.2g of its tolerance", f.Path, c.largest)
			}
		})
	}
}

// valueComparison lists where a document differs from the one a fixture recorded (at most 10
// places): every value the fixture holds, the document holds too (a later version may add to a
// document, not change what it said), numbers to a relative 1e-9 (and an absolute 1e-12 near
// zero). largest is the largest difference between two numbers as a share of its tolerance.
type valueComparison struct {
	differences []string
	largest     float64
}

func (c *valueComparison) compare(where string, want, got any) {
	if len(c.differences) >= 10 {
		return
	}
	differ := func(format string, args ...any) {
		c.differences = append(c.differences, where+": "+fmt.Sprintf(format, args...))
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			differ("%v, was an object", got)
			return
		}
		keys := make([]string, 0, len(w))
		for k := range w {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if gv, ok := g[k]; ok {
				c.compare(where+"."+k, w[k], gv)
			} else {
				differ("%s is missing", k)
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			differ("not a list of %d", len(w))
			return
		}
		for i := range w {
			c.compare(fmt.Sprintf("%s[%d]", where, i), w[i], g[i])
		}
	case float64:
		g, ok := got.(float64)
		if !ok {
			differ("%v, was %v", got, w)
			return
		}
		tolerance := 1e-9*math.Max(math.Abs(w), math.Abs(g)) + 1e-12
		c.largest = math.Max(c.largest, math.Abs(w-g)/tolerance)
		if math.Abs(w-g) > tolerance {
			differ("%v, was %v", g, w)
		}
	default:
		if want != got {
			differ("%v, was %v", got, want)
		}
	}
}

func waitForJob(t *testing.T, a *app, id string, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for {
		j, ok := a.jobs.Get(id)
		if !ok {
			t.Fatalf("no job %q", id)
		}
		if j.State.Finished() {
			if j.Error != "" {
				t.Fatal(j.Error)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the job did not finish within %s", limit)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func gunzipFile(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

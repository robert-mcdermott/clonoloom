// Adapted from SituCraft 0.9.0 browse_test.go.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// browseRequest asks for a folder's listing as the page does, from this computer or another.
func browseRequest(t *testing.T, handler http.Handler, path string, hidden bool, peer string) (*httptest.ResponseRecorder, browseAnswer) {
	t.Helper()
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	if hidden {
		q.Set("hidden", "1")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/browse?"+q.Encode(), nil)
	req.Host = "127.0.0.1:8860"
	req.RemoteAddr = peer
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var answer browseAnswer
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
			t.Fatal(err)
		}
	}
	return rec, answer
}

// A folder of what a user keeps beside their data: Cell Ranger outputs (a run, and a run's outs/),
// a plain folder, a link to a run, an immunoSEQ export, AIRR metadata, a project, a file ClonoLoom
// does not open, and a hidden file; listed folders first, each kind as ClonoLoom reads it; a
// file's path lists its folder with it chosen; the home folder by default; what is not there, and
// another computer, refused.
func TestBrowse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	data := filepath.Join(home, "Repertoire data")
	run := filepath.Join(data, "run 10")
	for _, dir := range []string{run, filepath.Join(data, "run 9", "outs"), filepath.Join(data, "notes"), filepath.Join(home, "Documents")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, text string) {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(run, "filtered_contig_annotations.csv"), "barcode,contig_id\n")
	write(filepath.Join(data, "run 9", "outs", "filtered_contig_annotations.csv"), "barcode,contig_id\n")
	write(filepath.Join(data, "P00207.tsv"), "rearrangement\tamino_acid\n")
	write(filepath.Join(data, "repertoires.airr.json"), "{}")
	write(filepath.Join(data, "study.clz"), "zip")
	write(filepath.Join(data, "README.docx"), "notes")
	write(filepath.Join(data, ".DS_Store"), "")
	// A link to a run is the run (where this computer lets a test make links).
	linked := os.Symlink(run, filepath.Join(data, "a link")) == nil
	_, handler := testApp(t)
	rec, answer := browseRequest(t, handler, data, false, "127.0.0.1:5555")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got []string
	for _, e := range answer.Entries {
		got = append(got, e.Name+":"+e.Kind)
	}
	want := "notes:folder run 9:cellranger run 10:cellranger P00207.tsv:table README.docx:file repertoires.airr.json:metadata study.clz:project"
	if linked {
		want = "a link:cellranger " + want
	}
	if strings.Join(got, " ") != want {
		t.Errorf("entries %s,\nwant %s", strings.Join(got, " "), want)
	}
	if answer.Path != data || answer.Parent != home || answer.Home != home || answer.Separator != string(filepath.Separator) || answer.Kind != "" {
		t.Errorf("the folder: %+v", answer)
	}
	for _, e := range answer.Entries {
		if e.Name == "P00207.tsv" && (e.Size != int64(len("rearrangement\tamino_acid\n")) || e.Modified == "") {
			t.Errorf("the table: %+v", e)
		}
	}
	// Places: the home folder, its Documents (its Desktop is not there), the disks.
	names := map[string]string{}
	for _, p := range answer.Places {
		names[p.Name] = p.Path
	}
	if names["Home"] != home || names["Documents"] != filepath.Join(home, "Documents") || names["Desktop"] != "" {
		t.Errorf("places %+v", answer.Places)
	}
	// Hidden entries when asked for.
	if _, hidden := browseRequest(t, handler, data, true, "127.0.0.1:5555"); len(hidden.Entries) != len(answer.Entries)+1 || !anyHidden(hidden.Entries) {
		t.Errorf("hidden: %d entries", len(hidden.Entries))
	}
	// A run's own folder says so; a file's path lists its folder with it chosen.
	if _, in := browseRequest(t, handler, run, false, "127.0.0.1:5555"); in.Kind != "cellranger" {
		t.Errorf("inside the run: %+v", in)
	}
	if _, file := browseRequest(t, handler, filepath.Join(data, "P00207.tsv"), false, "127.0.0.1:5555"); file.Path != data || file.Select != "P00207.tsv" {
		t.Errorf("a file's path: %s, chosen %q", file.Path, file.Select)
	}
	// The home folder by default, and ~ for it.
	if _, def := browseRequest(t, handler, "", false, "127.0.0.1:5555"); def.Path != home {
		t.Errorf("by default: %s", def.Path)
	}
	if _, tilde := browseRequest(t, handler, "~/Repertoire data", false, "127.0.0.1:5555"); tilde.Path != data {
		t.Errorf("~: %s", tilde.Path)
	}
	// Refused: what is not there, and another computer.
	if rec, _ := browseRequest(t, handler, filepath.Join(home, "gone"), false, "127.0.0.1:5555"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "nothing is at") {
		t.Errorf("not there: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := browseRequest(t, handler, data, false, "10.0.0.5:5555"); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "Only this computer") {
		t.Errorf("another computer: %d %s", rec.Code, rec.Body)
	}
}

func anyHidden(entries []browseEntry) bool {
	for _, e := range entries {
		if e.Hidden && e.Name == ".DS_Store" {
			return true
		}
	}
	return false
}

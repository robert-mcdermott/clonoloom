package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The tests between two time points are an analysis like the others: the comparison makes their
// ID (and is ignored by the analyses that do not compare); without one, they are refused; their
// result exports as CSV with its NOTICE.
func TestExpansionIsTestedKeptAndExported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var sum datasetSummary
	json.Unmarshal(open.Body.Bytes(), &sum)
	base := `"dataset": "` + sum.ID + `", "definition": {"name": "AA", "junction": "aa", "v": "gene", "j": "gene"}, "study": [{"name": "R1", "subject": "S1", "timepoint": "day 0"}, {"name": "R2", "subject": "S1", "timepoint": "week 1"}]`
	idOf := func(kind, body string) string {
		t.Helper()
		rec := postJSON(t, handler, "/api/"+kind, `{"start": false, `+body+`}`)
		var answer map[string]any
		json.Unmarshal(rec.Body.Bytes(), &answer)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", kind, rec.Code, rec.Body)
		}
		return answer["artifact"].(string)
	}
	body := base + `, "compare": {"from": "day 0", "to": "day 7"}`
	id := idOf("expansion", body)
	if !strings.HasPrefix(id, "exp-") || id == idOf("expansion", base+`, "compare": {"from": "day 0", "to": "day 7", "test": "fisher"}`) {
		t.Errorf("the test does not make the id: %s", id)
	}
	if idOf("tracking", base) != idOf("tracking", base+`, "compare": {"from": "day 0", "to": "day 7"}`) {
		t.Error("a comparison changes tracking's id")
	}
	for _, bad := range []string{base, base + `, "compare": {"from": "day 0", "to": "day 0"}`, base + `, "compare": {"from": "day 0", "to": "day 7", "alpha": 3}`} {
		if rec := postJSON(t, handler, "/api/expansion", `{`+bad+`}`); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", bad, rec.Code, rec.Body)
		}
	}
	started := postJSON(t, handler, "/api/expansion", `{`+body+`}`)
	var job map[string]any
	json.Unmarshal(started.Body.Bytes(), &job)
	if job["artifact"] != id || job["job"] == nil {
		t.Fatalf("started: %s", started.Body)
	}
	waitForJob(t, a, job["job"].(string), 10*time.Second)
	var read struct {
		Result expansionDocument `json:"result"`
	}
	if err := json.Unmarshal(request2(t, handler, "/api/expansion/"+id), &read); err != nil {
		t.Fatal(err)
	}
	// R1 and R2's one productive clonotype, the whole of each: 3 and 5, as their depths expect.
	doc := read.Result
	if len(doc.Subjects) != 1 || len(doc.Samples) != 2 || doc.Comparison.Test != "binomial" || doc.Comparison.MinTotal != 5 {
		t.Fatalf("%+v", doc)
	}
	s := doc.Subjects[0]
	if s.From.Label != "day 0" || s.To.Label != "week 1" || s.Tested != 1 || s.Clonotypes[0].P != 1 || s.Clonotypes[0].Call != "" {
		t.Errorf("%+v", s)
	}
	rec := request(t, handler, http.MethodGet, "/api/export?what=result&artifact="+id, nil, nil)
	files := unzip(t, rec.Body.Bytes())
	if !strings.Contains(files["tests.csv"], "S1,day 0,week 1,"+s.Clonotypes[0].ID+",TRB CASSLAGGPEAFF TRBV5-1 TRBJ2-7,3,5,3,5,1,1,0,1,1,") || !strings.HasPrefix(files["subjects.csv"], "subject,from,to") || !strings.Contains(files["NOTICE.txt"], "binomial test, two-sided") {
		t.Errorf("tests.csv:\n%s\nsubjects.csv:\n%s\nNOTICE:\n%s", files["tests.csv"], files["subjects.csv"], files["NOTICE.txt"])
	}
}

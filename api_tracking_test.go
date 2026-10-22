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

// Tracking is an analysis like the others: the study's annotation and the clonotypes chosen make
// its id (and are ignored by the analyses that do not follow a study); its result follows each
// clonotype through the subject's time points, and exports as CSV with its NOTICE.
func TestTrackingIsFollowedKeptAndExported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var sum datasetSummary
	json.Unmarshal(open.Body.Bytes(), &sum)
	base := `"dataset": "` + sum.ID + `", "definition": {"name": "AA", "junction": "aa", "v": "gene", "j": "gene"}`
	studyJSON := `"study": [{"name": "R1", "subject": "S1", "timepoint": "day 0"}, {"name": "R2", "subject": "S1", "timepoint": "week 1"}]`
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
	if idOf("diversity", base) != idOf("diversity", base+`, `+studyJSON+`, "track": {"by": "list", "list": ["x"]}`) {
		t.Error("a study changes an analysis that does not follow one")
	}
	body := base + `, ` + studyJSON + `, "track": {"by": "largest", "n": 1}`
	id := idOf("tracking", body)
	if !strings.HasPrefix(id, "trk-") || id == idOf("tracking", base+`, `+studyJSON) || id == idOf("tracking", base+`, "track": {"by": "largest", "n": 1}`) {
		t.Errorf("the choice and the study do not make the id: %s", id)
	}
	if rec := postJSON(t, handler, "/api/tracking", `{`+base+`, "track": {"by": "list"}}`); rec.Code != http.StatusBadRequest {
		t.Errorf("an empty list: %d %s", rec.Code, rec.Body)
	}
	started := postJSON(t, handler, "/api/tracking", `{`+body+`}`)
	var job map[string]any
	json.Unmarshal(started.Body.Bytes(), &job)
	if job["artifact"] != id || job["job"] == nil {
		t.Fatalf("started: %s", started.Body)
	}
	waitForJob(t, a, job["job"].(string), 10*time.Second)
	var read struct {
		Artifact struct {
			Kind  string `json:"kind"`
			State string `json:"state"`
		} `json:"artifact"`
		Result trackingDocument `json:"result"`
	}
	if err := json.Unmarshal(request2(t, handler, "/api/tracking/"+id), &read); err != nil {
		t.Fatal(err)
	}
	doc := read.Result
	if read.Artifact.State != "complete" || doc.Unit != "umis" || !doc.Intervals || len(doc.Samples) != 2 || len(doc.Subjects) != 1 {
		t.Fatalf("%+v", read)
	}
	// R1 and R2's one productive clonotype, the whole of each: 3 of 3, then 5 of 5.
	s := doc.Subjects[0]
	if len(s.Points) != 2 || s.Points[1].Label != "week 1" || len(s.Clonotypes) != 1 {
		t.Fatalf("%+v", s)
	}
	c := s.Clonotypes[0]
	if c.Chains[0].Junction != "CASSLAGGPEAFF" || c.Points[0].Count != 3 || float64(c.Points[1].Frequency) != 1 || float64(c.Points[1].Upper) != 1 || float64(c.Points[0].Lower) > 0.5 {
		t.Errorf("%+v", c)
	}
	rec := request(t, handler, http.MethodGet, "/api/export?what=result&artifact="+id, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	files := unzip(t, rec.Body.Bytes())
	lines := strings.Split(strings.TrimSpace(files["tracking.csv"]), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[1], "S1,"+c.ID+",TRB CASSLAGGPEAFF TRBV5-1 TRBJ2-7,rank 1 in R1; rank 1 in R2,R1,day 0,0,,3,3,1,") {
		t.Errorf("tracking.csv:\n%s", files["tracking.csv"])
	}
	if !strings.Contains(files["time-points.csv"], "S1,"+c.ID+",TRB CASSLAGGPEAFF TRBV5-1 TRBJ2-7,week 1,7,,R2,5,5,1,") || !strings.Contains(files["NOTICE.txt"], "Wilson") {
		t.Errorf("time-points.csv:\n%s\nNOTICE:\n%s", files["time-points.csv"], files["NOTICE.txt"])
	}
}

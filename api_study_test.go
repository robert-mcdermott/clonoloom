package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"clonoloom/internal/study"
)

func TestTheStudyDesign(t *testing.T) {
	_, handler := testApp(t)
	rec := postJSON(t, handler, "/api/study", `{"samples": [{"name": "P1_d0", "subject": "P1", "timepoint": "day 0"}, {"name": "P1_IP", "subject": "P1", "reference": "infusion product"}, {"name": "P1_w4", "subject": "P1", "timepoint": "week 4"}]}`)
	var d study.Design
	json.Unmarshal(rec.Body.Bytes(), &d)
	if rec.Code != http.StatusOK || len(d.Subjects) != 1 || len(d.Subjects[0].Samples) != 3 || d.Subjects[0].Samples[0].Name != "P1_IP" || *d.LastDay != 28 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := postJSON(t, handler, "/api/study", `{"samples": 3}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "samples") {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}

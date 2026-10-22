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

// A subject's infusion product, two samples a week before and one two weeks after: CASSA grows
// from 1 to 30, CASSD shrinks.
const seriesTSV = "sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\trepertoire_id\n" +
	"p1\tTRBV5-1*01\tTRBJ2-7*01\tCASSAF\tT\t50\tP\n" + "p2\tTRBV5-1*01\tTRBJ2-7*01\tCASSCF\tT\t20\tP\n" +
	"a1\tTRBV5-1*01\tTRBJ2-7*01\tCASSAF\tT\t1\tA\n" + "a2\tTRBV5-1*01\tTRBJ2-7*01\tCASSCF\tT\t4\tA\n" + "a3\tTRBV6-2*01\tTRBJ1-1*01\tCASSDF\tT\t95\tA\n" +
	"b1\tTRBV5-1*01\tTRBJ2-7*01\tCASSCF\tT\t4\tB\n" + "b2\tTRBV6-2*01\tTRBJ1-1*01\tCASSDF\tT\t94\tB\n" +
	"c1\tTRBV5-1*01\tTRBJ2-7*01\tCASSAF\tT\t30\tC\n" + "c2\tTRBV5-1*01\tTRBJ2-7*01\tCASSCF\tT\t4\tC\n" + "c3\tTRBV6-2*01\tTRBJ1-1*01\tCASSDF\tT\t66\tC\n"

// Classes and attribution are analyses like the others: kept, read back and exported.
func TestClassesAndAttributionAreKeptAndExported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "series.tsv")
	os.WriteFile(path, []byte(seriesTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var sum datasetSummary
	json.Unmarshal(open.Body.Bytes(), &sum)
	base := `"dataset": "` + sum.ID + `", "definition": {"name": "AA", "junction": "aa", "v": "gene", "j": "gene"}, "study": [{"name": "P", "subject": "S1", "tissue": "infusion product"}, {"name": "A", "subject": "S1", "timepoint": "day -7"}, {"name": "B", "subject": "S1", "timepoint": "day -7"}, {"name": "C", "subject": "S1", "timepoint": "day 14"}]`
	run := func(kind, body string) []byte {
		t.Helper()
		rec := postJSON(t, handler, "/api/"+kind, `{`+body+`}`)
		var job map[string]any
		json.Unmarshal(rec.Body.Bytes(), &job)
		if rec.Code != http.StatusOK || job["job"] == nil {
			t.Fatalf("%s: %d %s", kind, rec.Code, rec.Body)
		}
		waitForJob(t, a, job["job"].(string), 10*time.Second)
		id := job["artifact"].(string)
		files := unzip(t, request(t, handler, http.MethodGet, "/api/export?what=result&artifact="+id, nil, nil).Body.Bytes())
		for name, text := range files {
			if text == "" {
				t.Errorf("%s: %s is empty", kind, name)
			}
		}
		return request2(t, handler, "/api/"+kind+"/"+id)
	}
	if rec := postJSON(t, handler, "/api/classes", `{`+base+`}`); rec.Code != http.StatusBadRequest {
		t.Errorf("classes without a base: %d", rec.Code)
	}
	var classes struct {
		Result classesDocument `json:"result"`
	}
	json.Unmarshal(run("classes", base+`, "class": {"base": "day -7", "test": "binomial"}`), &classes)
	c := classes.Result
	if len(c.Subjects) != 1 || c.Test != "binomial" || len(c.Samples) != 3 || c.Subjects[0].Counts.Emerging != 1 || c.Subjects[0].Clonotypes[0].Chains[0].Junction != "CASSAF" {
		t.Errorf("classes %+v", c)
	}
	var attribution struct {
		Result attributionDocument `json:"result"`
	}
	json.Unmarshal(run("attribution", base), &attribution)
	r := attribution.Result
	if len(r.Subjects) != 1 || len(r.Samples) != 3 || r.Subjects[0].Clonotypes != 2 || r.Subjects[0].Derived || !strings.Contains(r.Caveat, "infusion product") {
		t.Errorf("attribution %+v", r)
	}
	// At day 14, 34 of 100 in the product's clonotypes.
	if p := r.Subjects[0].Points[1]; p.Value.Count != 34 || p.Value.Total != 100 {
		t.Errorf("%+v", p)
	}
}

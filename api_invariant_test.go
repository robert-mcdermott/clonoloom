package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvariantPagesAndExports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invariant.tsv")
	os.WriteFile(path, []byte("sequence_id\trepertoire_id\tlocus\tv_call\tj_call\tjunction_aa\tproductive\tduplicate_count\nmait\tS1\tTRA\tTRAV1-2\tTRAJ33\tCAVRDSNYQLIW\tT\t2\ngem\tS1\tTRA\tTRAV1-2\tTRAJ9\tCAVRNTGGFKTIF\tT\t3\nbeta\tS1\tTRB\tTRBV6-2\tTRBJ1-1\tCASSLAGGPEAFF\tT\t5\n"), 0644)
	_, h := testApp(t)
	var ds datasetSummary
	json.Unmarshal(postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`).Body.Bytes(), &ds)
	body := `{"dataset":"` + ds.ID + `","definition":{},"species":"human","population":"MAIT"}`
	rec := postJSON(t, h, "/api/clonotypes", body)
	var p clonotypePage
	json.Unmarshal(rec.Body.Bytes(), &p)
	if rec.Code != 200 || p.Matched != 1 || len(p.Rows) != 1 || p.Rows[0].Invariant[0] != "match" || p.Invariant.Samples[0].Total != 10 || *p.Invariant.Samples[0].Share[0] != .2 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// Repeated pages must not clear the cache's flags when serving the summary.
	rec = postJSON(t, h, "/api/clonotypes", body)
	json.Unmarshal(rec.Body.Bytes(), &p)
	if len(p.Rows) != 1 || p.Rows[0].Invariant[0] != "match" {
		t.Fatal(rec.Body)
	}
	one := postJSON(t, h, "/api/clonotypes/one", `{"dataset":"`+ds.ID+`","definition":{},"species":"human","id":"`+p.Rows[0].ID+`"}`)
	if !strings.Contains(one.Body.String(), `"invariant":["match","no match","no match"]`) {
		t.Fatal(one.Body)
	}
	q := url.Values{"what": {"clonotypes"}, "dataset": {ds.ID}, "definition": {"{}"}, "species": {"human"}}
	out := request(t, h, http.MethodGet, "/api/export?"+q.Encode(), nil, nil)
	files := unzip(t, out.Body.Bytes())
	if !strings.Contains(files["clonotypes.csv"], "invariant_species,invariant_rule_version,MAIT,iNKT,GEM") || !strings.Contains(files["invariant-samples.csv"], "S1,human,1,MAIT,10,5,2,0.2") || !strings.Contains(files["NOTICE.txt"], "1401380") {
		t.Fatal(files)
	}
	for _, field := range []string{`"species":"guess"`, `"population":"guess"`} {
		rec := postJSON(t, h, "/api/clonotypes", `{"dataset":"`+ds.ID+`",`+field+`}`)
		if rec.Code != 400 {
			t.Fatal(rec.Code, rec.Body)
		}
	}
}

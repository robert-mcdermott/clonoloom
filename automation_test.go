package main

import (
	"clonoloom/internal/clonotype"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutomationCatalogAndGrants(t *testing.T) {
	for _, bad := range []string{"read", "all", "files,files", "files, network", "shell", "sql"} {
		if _, err := parseAutomationAllow(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, err := parseConfig([]string{"--allow", "files"}); err == nil {
		t.Fatal("allow without remote")
	}
	seen := map[string]bool{}
	for _, a := range remoteActions {
		if seen[a.Name] || len(a.Effects) == 0 || a.Effects[0] != "read" {
			t.Fatalf("invalid catalog: %+v", a)
		}
		seen[a.Name] = true
		edits := false
		for _, e := range a.Effects {
			if e == "edits" {
				edits = true
			}
			if !map[string]bool{"read": true, "edits": true, "compute": true, "files": true, "network": true}[e] {
				t.Fatal(e)
			}
		}
		if edits != a.Review {
			t.Fatalf("review drift: %s", a.Name)
		}
	}
	hub, server := remoteServer(t, "policy")
	stop := attachPage(t, server, "policy")
	defer stop()
	for _, tc := range []struct {
		body, token string
		code        int
	}{
		{`{"action":"set_theme","args":{"theme":"dark"}}`, hub.token, 403},
		{`{"action":"get_state","args":{"shell":"echo no"}}`, "", 400},
		{`{"action":"get_state","args":null}`, "", 400},
		{`{"action":"get_state","allow":["edits"]}`, "", 400},
		{`{"action":"set_theme","args":{"theme":"dark"}}`, "", 401},
	} {
		if code, body := act(t, server, tc.body, tc.token); code != tc.code {
			t.Fatalf("%d %s", code, body)
		}
	}
	if code, body := act(t, server, `{"action":"get_state","client":"research notebook"}`, ""); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.audit) < 8 {
		t.Fatalf("missing refusals: %+v", hub.audit)
	}
	for _, row := range hub.audit {
		if len(row.ArgsSHA256) != 64 {
			t.Fatal(row)
		}
	}
}
func TestAutomationCancellationReachesWindow(t *testing.T) {
	h := newRemoteHub("")
	client := &remoteClient{events: make(chan remoteEvent, 16), done: make(chan struct{})}
	h.clients = []*remoteClient{client}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := h.dispatch(ctx, remoteEvent{Action: "get_state"}); done <- err }()
	first := <-client.events
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("did not cancel")
	}
	event := <-client.events
	if !event.Cancel || event.ID != first.ID {
		t.Fatal(event)
	}
	if len(h.pending) != 0 || len(h.pendingEvents) != 0 {
		t.Fatal("cancelled request remains")
	}
}
func TestTypedClonotypePredicates(t *testing.T) {
	table := &clonotype.Table{Clonotypes: []clonotype.Clonotype{
		{ID: "a", Total: 12, Samples: 2, Chains: []clonotype.Chain{{Locus: "TRA", Junction: "CAAA", V: "TRAV1"}, {Locus: "TRB", Junction: "CASS", V: "TRBV2"}}},
		{ID: "b", Total: 2, Samples: 1, Chains: []clonotype.Chain{{Locus: "TRB", Junction: "CATS"}}},
	}}
	for _, tc := range []struct {
		filter string
		want   []bool
	}{
		{`{"all":[{"field":"total","op":"gte","value":3},{"field":"v","op":"eq","value":"TRBV2"}]}`, []bool{true, false}},
		{`{"not":{"field":"v","op":"contains","value":"TRBV"}}`, []bool{false, true}},
		{`{"any":[{"field":"samples","op":"lte","value":1},{"field":"id","op":"eq","value":"a"}]}`, []bool{true, true}},
		{`{"field":"junction","op":"contains","value":"cass"}`, []bool{false, false}},
	} {
		var f clonotypeFilter
		if err := strictJSON([]byte(tc.filter), &f); err != nil {
			t.Fatal(err)
		}
		n := 0
		if err := f.validate(1, &n); err != nil {
			t.Fatal(err)
		}
		for i, want := range tc.want {
			if f.matches(table, i) != want {
				t.Fatalf("%s row %d", tc.filter, i)
			}
		}
	}
	for _, bad := range []string{`{}`, `{"all":[]}`, `{"field":"total","op":"gte","value":null}`, `{"field":"total","op":"eq","value":"12"}`, `{"field":"id","op":"sql","value":"a"}`, `{"field":"v","op":"eq","value":""}`, `{"sql":"select *"}`, `{"all":null,"field":"id","op":"eq","value":"a"}`, `{"field":"id","field":"total","op":"eq","value":2}`, `{"all":[{}],"not":{}}`} {
		var f clonotypeFilter
		err := strictJSON([]byte(bad), &f)
		if err == nil {
			n := 0
			err = f.validate(1, &n)
		}
		if err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	f := &clonotypeFilter{Field: "total", Op: "gte", Value: json.RawMessage(`0`)}
	for i := 0; i < 8; i++ {
		f = &clonotypeFilter{Not: f}
	}
	n := 0
	if f.validate(1, &n) == nil {
		t.Fatal("depth unbounded")
	}
}
func TestTypedClonotypeQueryDenominators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "study.tsv")
	os.WriteFile(path, []byte(datasetTSV+"d\tTRBV7-2*01\tTRBJ1-1*01\tCASSQGQGAYEQYF\tT\t4\tR2\n"), 0600)
	_, handler := testApp(t)
	var ds datasetSummary
	json.Unmarshal(postJSON(t, handler, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`).Body.Bytes(), &ds)
	base := `{"dataset":"` + ds.ID + `","definition":{"junction":"aa","v":"gene","j":"gene"},"sort":"total"`
	var full, filtered clonotypePage
	response := postJSON(t, handler, "/api/clonotypes/query", base+`}`)
	if response.Code != 200 {
		t.Fatal(response.Body)
	}
	json.Unmarshal(response.Body.Bytes(), &full)
	if full.Count != 2 || len(full.Rows) != 2 || full.Rows[0].Total != 8 || full.Rows[1].Total != 4 || full.Rows[1].Frequency != 1.0/3 {
		t.Fatalf("literal abundance oracle: %+v", full)
	}
	response = postJSON(t, handler, "/api/clonotypes/query", base+`,"where":{"field":"id","op":"eq","value":"`+full.Rows[1].ID+`"},"limit":1}`)
	if response.Code != 200 {
		t.Fatal(response.Body)
	}
	json.Unmarshal(response.Body.Bytes(), &filtered)
	if filtered.Count != full.Count || filtered.Matched != 1 || len(filtered.Rows) != 1 || filtered.Rows[0].Frequency != full.Rows[1].Frequency || filtered.Unit != full.Unit || filtered.DefinitionID != full.DefinitionID {
		t.Fatalf("denominator or definition changed: %+v", filtered)
	}

	response = postJSON(t, handler, "/api/clonotypes/query", base+`,"filter":{"minCount":5}}`)
	if response.Code != 200 {
		t.Fatal(response.Body)
	}
	var qcPage clonotypePage
	json.Unmarshal(response.Body.Bytes(), &qcPage)
	if qcPage.Count != 1 || len(qcPage.Rows) != 1 || qcPage.Rows[0].Total != 5 || qcPage.Rows[0].Frequency != 1 || qcPage.Filter.MinCount != 5 {
		t.Fatalf("QC rules or denominator lost: %+v", qcPage)
	}
	response = postJSON(t, handler, "/api/clonotypes/query", base+`,"offset":1,"limit":1}`)
	var paged clonotypePage
	json.Unmarshal(response.Body.Bytes(), &paged)
	if response.Code != 200 || len(paged.Rows) != 1 || paged.Rows[0].ID != full.Rows[1].ID || paged.Rows[0].Rank != 2 || paged.Count != 2 || paged.Matched != 2 {
		t.Fatalf("pagination drift: %+v", paged)
	}
	for _, tail := range []string{`,"where":{"all":[{"shell":"echo no"}]}}`, `,"limit":1001}`, `,"offset":-1}`, `,"sql":"select *"}`} {
		rec := postJSON(t, handler, "/api/clonotypes/query", base+tail)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("accepted %s: %s", tail, rec.Body)
		}
	}
	if rec := postJSON(t, handler, "/api/clonotypes/query", base+`} {}`); rec.Code != 400 {
		t.Fatal("trailing JSON")
	}
}
func FuzzTypedClonotypeFilter(f *testing.F) {
	f.Add(`{"field":"total","op":"gte","value":2}`)
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 1<<16 {
			return
		}
		var filter clonotypeFilter
		if strictJSON([]byte(input), &filter) != nil {
			return
		}
		n := 0
		if filter.validate(1, &n) != nil {
			return
		}
		filter.matches(&clonotype.Table{Clonotypes: []clonotype.Clonotype{{ID: strings.Repeat("a", 3)}}}, 0)
	})
}

func TestAutomationNumericAndSchemaRefusals(t *testing.T) {
	a, _ := findAction("query_clonotypes")
	for _, raw := range []string{`{"limit":9007199254740993}`, `{"where":{"field":"total","op":"eq","value":1e999}}`, `{"limit":1,"limit":2}`, `{"limit":"2"}`, `{"offset":null}`} {
		if validateActionArgs(json.RawMessage(raw), a.InputSchema) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestAutomationAuditBound(t *testing.T) {
	h := newRemoteHub("")
	for i := 0; i < 600; i++ {
		h.record(remoteEvent{Client: "notebook", Action: "get_state"}, "completed")
	}
	if len(h.audit) != 500 {
		t.Fatal(len(h.audit))
	}
}

func TestTypedFilterNodeBound(t *testing.T) {
	leaf := clonotypeFilter{Field: "total", Op: "gte", Value: json.RawMessage(`1`)}
	tree := clonotypeFilter{All: make([]clonotypeFilter, 63)}
	for i := range tree.All {
		tree.All[i] = leaf
	}
	nodes := 0
	if err := tree.validate(1, &nodes); err != nil {
		t.Fatal(err)
	}
	tree.All = append(tree.All, leaf)
	nodes = 0
	if tree.validate(1, &nodes) == nil {
		t.Fatal("65-node tree accepted")
	}
}

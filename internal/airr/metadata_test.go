package airr

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExactMetadataEvidence(t *testing.T) {
	raw := `{ "repertoire_id":"R", "subject":{"subject_id":9007199254740993}, "sample":[{"single_cell":false,"collection_time_point_relative":0}], "extra":{"a":1.234567890123456789,"b":1e400,"c":-0,"d":null,"escaped":"\ud83d\ude00"} }`
	doc, err := ReadRepertoireDocument(context.Background(), []byte(`{"Info":{"version":"2.0"},"Repertoire":[`+raw+`]}`))
	if err != nil {
		t.Fatal(err)
	}
	r := doc.Repertoires[0]
	if string(r.Raw) != raw || r.SubjectID != "9007199254740993" || r.TimePoint != "0" || r.SingleCell == nil || *r.SingleCell || doc.SchemaVersion != "2.0" {
		t.Fatalf("evidence changed: %+v %s", r, r.Raw)
	}
	mixed, err := ReadRepertoires([]byte(`{"repertoire_id":"R","sample":[{"single_cell":false},{"single_cell":true},{"single_cell":false}]}`))
	if err != nil || !mixed[0].SingleCellMixed || mixed[0].SingleCell != nil {
		t.Fatalf("mixed processing collapsed: %+v %v", mixed, err)
	}
}

func TestMetadataAmbiguityAndBounds(t *testing.T) {
	for _, raw := range []string{
		``, `null`, `[] true`, `{"Repertoire":null}`, `{"Repertoire":{}}`, `{"Repertoire":[null]}`,
		`{"repertoire_id":null}`, `{"repertoire_id":123}`, `{"repertoire_id":""}`, `{"repertoire_id":" R"}`, `{"repertoire_id":"R\n"}`,
		`[{"repertoire_id":"R"},{"repertoire_id":"R"}]`,
		`{"repertoire_id":"R","repertoire\u005fid":"S"}`,
		`{"repertoire_id":"R","sample":[{"x":0,"x":1}]}`,
		`{"repertoire_id":"\ud800"}`, `{"repertoire_id":"\udc00"}`, `{"repertoire_id":"\ud800\u0041"}`,
		"{\"repertoire_id\":\"\xff\"}",
		`{"Info":{"version":2},"Repertoire":[]}`, `{"Info":null,"Repertoire":[]}`,
		`{"repertoire_id":"R","x":` + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + "}",
		`{"repertoire_id":"` + strings.Repeat("x", 4097) + `"}`,
	} {
		if _, err := ReadRepertoires([]byte(raw)); err == nil {
			t.Errorf("accepted ambiguous/invalid metadata %.100s", raw)
		}
	}
	for _, raw := range []string{`[]`, `{"Repertoire":[]}`, `[{"repertoire_id":"R"},{"repertoire_id":"r"}]`, `{"repertoire_id":"\\ud800"}`} {
		if _, err := ReadRepertoires([]byte(raw)); err != nil {
			t.Errorf("valid metadata refused: %s: %v", raw, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadRepertoireDocument(ctx, []byte(`[]`)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := ReadRepertoires(make([]byte, maxMetadataBytes+1)); err == nil {
		t.Fatal("oversize accepted")
	}
}

func TestMetadataStructuralBudgets(t *testing.T) {
	// Few bytes per value/record exercise work limits without approaching the byte budget.
	values := `{"repertoire_id":"R","extra":[` + strings.Repeat("0,", maxMetadataValues) + `0]}`
	if _, err := ReadRepertoires([]byte(values)); err == nil || !strings.Contains(err.Error(), "JSON values") {
		t.Fatalf("work budget: %v", err)
	}
	records := `{"Repertoire":[` + strings.Repeat(`{"repertoire_id":"R"},`, maxRepertoires) + `{"repertoire_id":"last"}]}`
	if _, err := ReadRepertoires([]byte(records)); err == nil || !strings.Contains(err.Error(), "repertoire records") {
		t.Fatalf("record budget: %v", err)
	}
}

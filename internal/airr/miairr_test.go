package airr

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestMiAIRREvidenceStatesAndLevels(t *testing.T) {
	raw := []byte(`{"repertoire_id":"R1","study":{"study_id":null,"study_title":"","study_type":{},"contributors":[],"pub_ids":[null]},"subject":{"synthetic":false,"species":"unknown","sex":"not_collected","age":0,"race":"not_applicable","ethnicity":"missing","strain_name":"N/A"},"sample":[{"single_cell":false,"cell_number":9007199254740993,"cells_per_reaction":0,"template_class":null,"template_amount":{"quantity":1.234567890123456789},"physical_linkage":"none","pcr_target":[{"pcr_target_locus":"TRB"},{"pcr_target_locus":"TRA"}],"sequencing_files":{"paired_filename":"not_applicable"}},{"cell_number":0,"template_class":"RNA"}],"data_processing":[{"software_versions":"simulated protocol v1"},{"quality_thresholds":null}]}`)
	before := string(raw)
	r := AssessMiAIRR(context.Background(), raw, "2.0")
	if r.Status != "review" || r.Version != MiAIRRChecklistVersion || string(raw) != before {
		t.Fatalf("assessment changed evidence: %+v", r)
	}
	checks := map[string][2]string{
		"/study/study_id": {"null", "important"}, "/study/study_title": {"empty", "important"}, "/study/study_type": {"empty", "important"},
		"/study/contributors": {"empty", "essential"}, "/study/pub_ids": {"structure-review", "important"},
		"/study/grants": {"absent", "important"}, "/subject/synthetic": {"value-present", "essential"}, "/subject/species": {"unknown-marker", "essential"},
		"/subject/sex": {"not_collected", "important"}, "/subject/race": {"not_applicable", "important"}, "/subject/ethnicity": {"missing", "important"},
		"/subject/strain_name": {"unknown-marker", "important"}, "/subject/age": {"shape-review", "important"},
		"/sample/0/single_cell": {"value-present", "important"}, "/sample/0/cell_number": {"value-present", "important"}, "/sample/0/cells_per_reaction": {"value-present", "important"},
		"/sample/0/template_class": {"null", "essential"}, "/sample/0/template_amount": {"structure-review", "important"}, "/sample/0/physical_linkage": {"value-present", "essential"},
		"/sample/0/pcr_target/0/pcr_target_locus": {"value-present", "important"}, "/sample/0/pcr_target/1/pcr_target_locus": {"value-present", "important"},
		"/sample/0/sequencing_files/paired_filename": {"not_applicable", "important"}, "/sample/1/cell_number": {"value-present", "important"},
		"/sample/1/pcr_target": {"absent", "scope"}, "/subject/diagnosis": {"absent", "scope"}, "/sample/0/cell_label": {"absent", "defined"},
		"/data_processing/0/software_versions": {"value-present", "important"}, "/data_processing/1/software_versions": {"absent", "important"},
		"/data_processing/1/quality_thresholds": {"null", "important"},
	}
	for _, item := range r.Items {
		if want, ok := checks[item.Path]; ok {
			if item.State != want[0] || item.Level != want[1] {
				t.Errorf("%s: %s/%s, want %v", item.Path, item.State, item.Level, want)
			}
			delete(checks, item.Path)
		}
		if item.Remedy == "" {
			t.Errorf("no remedy: %+v", item)
		}
		if strings.Contains(item.Path, "pcr_target") && item.Condition == "" {
			t.Errorf("no applicability review: %+v", item)
		}
	}
	if len(checks) != 0 {
		t.Fatalf("missing paths: %v", checks)
	}
	n := 0
	for _, count := range r.Counts {
		n += count
	}
	if n != len(r.Items) {
		t.Fatal("counts do not cover exactly returned evidence rows")
	}
}

func TestMiAIRRScopeVersionAndBounds(t *testing.T) {
	for _, version := range []string{"", "1.3", "1.4", "1.5", "2.1", "2.0.1", "v2.0", "2.0 "} {
		r := AssessMiAIRR(context.Background(), []byte(`{"repertoire_id":"R"}`), version)
		if r.Status != "not-assessed" || len(r.Items) != 0 || len(r.Counts) != 0 {
			t.Fatalf("unsupported %q: %+v", version, r)
		}
	}
	for _, input := range []string{`{"study":null,"subject":false,"sample":[],"data_processing":[null]}`, `{"study":{},"subject":{"diagnosis":[{},null]},"sample":[{},false],"data_processing":{}}`} {
		r := AssessMiAIRR(context.Background(), []byte(input), "2.0.0")
		if r.Status != "review" {
			t.Fatal(r)
		}
		for _, item := range r.Items {
			if item.Level == "scope" && !strings.Contains(item.Remedy, "not assessed") {
				t.Fatal(item)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		ctx context.Context
		raw []byte
	}{
		{ctx, []byte(`{}`)}, {context.Background(), []byte(`{"subject":{},"subject":{}}`)}, {context.Background(), []byte(`null`)},
		{context.Background(), []byte(strings.Repeat(" ", 4<<20+1))},
		{context.Background(), []byte(`{"sample":[` + strings.TrimSuffix(strings.Repeat(`{},`, 100), ",") + `]}`)},
	}
	for _, tc := range cases {
		r := AssessMiAIRR(tc.ctx, tc.raw, "2.0")
		if r.Status != "not-assessed" || len(r.Items) != 0 || len(r.Counts) != 0 || r.Notice == "" {
			t.Fatalf("partial assessment: %+v", r)
		}
	}
}

func TestMiAIRRJSONYAMLEquivalence(t *testing.T) {
	j := []byte(`{"repertoire_id":"R","subject":{"synthetic":false},"sample":[{"cell_number":9007199254740993,"template_class":"RNA"}]}`)
	y := []byte("repertoire_id: R\nsubject:\n  synthetic: false\nsample:\n- cell_number: 9007199254740993\n  template_class: RNA\n")
	yd, err := ReadRepertoireYAML(context.Background(), y)
	if err != nil {
		t.Fatal(err)
	}
	a := AssessMiAIRR(context.Background(), j, "2.0")
	b := AssessMiAIRR(context.Background(), yd.Repertoires[0].Raw, "2.0")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("format changed checklist")
	}
}

func TestMiAIRRInventoryAndReference(t *testing.T) {
	// Export the native catalogue for the independent official-table validator.
	inventory := map[string]map[string]string{}
	for _, g := range miairrGroups {
		for _, f := range g.Fields {
			if inventory[f.Name] != nil {
				t.Fatalf("duplicate field: %s", f.Name)
			}
			inventory[f.Name] = map[string]string{"level": f.Level, "kind": f.Kind, "path": g.Path + "/" + f.Name}
		}
	}
	if len(inventory) != 82 {
		t.Fatalf("inventory: %d", len(inventory))
	}
	// The committed reference is an extraction of the official standards table,
	// not a native result. Keep this check in ordinary offline CI as well.
	data, err := os.ReadFile("../../validation/reference/miairr_fields.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Fields []struct{ Field, Level, Type string }
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	if len(reference.Fields) != len(inventory) {
		t.Fatal("official inventory count differs")
	}
	for _, field := range reference.Fields {
		kind := field.Type
		if strings.HasPrefix(kind, "array") {
			kind = "array"
		} else if kind != "string" && kind != "boolean" && kind != "integer" {
			kind = "object"
		}
		if inventory[field.Field]["level"] != field.Level || inventory[field.Field]["kind"] != kind {
			t.Fatalf("official field differs: %+v", field)
		}
	}
	if dest := os.Getenv("CLONOLOOM_MIAIRR_INVENTORY"); dest != "" {
		data, _ := json.MarshalIndent(inventory, "", "  ")
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.ReadFile("../../testdata/metadata/miairr.airr.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ReadRepertoireDocument(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	var reports []MiAIRRChecklist
	for _, rep := range doc.Repertoires {
		reports = append(reports, AssessMiAIRR(context.Background(), rep.Raw, doc.SchemaVersion))
	}
	if dest := os.Getenv("CLONOLOOM_MIAIRR_EVIDENCE"); dest != "" {
		data, _ := json.Marshal(reports)
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func FuzzMiAIRR(f *testing.F) {
	f.Add([]byte(`{"repertoire_id":"R","subject":{"synthetic":false}}`))
	f.Add([]byte(`{"study":{},"sample":[null,{}],"data_processing":[{}]}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		r := AssessMiAIRR(context.Background(), raw, "2.0")
		n := 0
		for _, v := range r.Counts {
			n += v
		}
		if n != len(r.Items) || len(r.Items) > miAIRRRowLimit {
			t.Fatal("incorrect/bounded counts")
		}
		if r.Status == "not-assessed" && len(r.Items) != 0 {
			t.Fatal("partial evidence")
		}
	})
}

package airr

import "testing"

func TestRepertoireMetadataOfSchemaOneAndTwo(t *testing.T) {
	v13 := `{"Info": {"title": "x"}, "Repertoire": [{
		"repertoire_id": "R1",
		"study": {"study_id": "PRJ1", "study_title": "Vaccine time course"},
		"subject": {"subject_id": "S01", "species": {"id": "NCBITAXON:9606", "label": "Homo sapiens"}, "sex": "female",
			"age_min": 30, "age_max": 40, "age_unit": {"id": "UO:0000036", "label": "year"},
			"diagnosis": [{"study_group_description": "Cohort 01", "disease_diagnosis": {"id": null, "label": null}}],
			"mhc": ["HLA-A*01", "HLA-B*08"]},
		"sample": [{"sample_id": "S01_d14", "tissue": {"id": "UBERON:0013756", "label": "venous blood"},
			"cell_subset": {"label": "CD8-positive T cell"}, "collection_time_point_relative": 14,
			"collection_time_point_relative_unit": {"label": "day"}, "collection_time_point_reference": "vaccination",
			"template_class": "DNA", "single_cell": false, "pcr_target": [{"pcr_target_locus": "TRB"}]}]
	}]}`
	reps, err := ReadRepertoires([]byte(v13))
	if err != nil {
		t.Fatal(err)
	}
	r := reps[0]
	if r.RepertoireID != "R1" || r.StudyID != "PRJ1" || r.SubjectID != "S01" || r.Species != "Homo sapiens" || r.Sex != "female" || r.Age != "30–40 year" {
		t.Fatalf("%+v", r)
	}
	if r.Tissue != "venous blood" || r.CellSubset != "CD8-positive T cell" || r.TimePoint != "14 day (vaccination)" || r.Template != "DNA" || r.StudyGroup != "Cohort 01" || r.Diagnosis != "" {
		t.Fatalf("%+v", r)
	}
	if len(r.HLA) != 2 || len(r.Loci) != 1 || r.Loci[0] != "TRB" || r.SingleCell == nil || *r.SingleCell || len(r.Raw) == 0 {
		t.Fatalf("%+v", r)
	}
	v20 := `[{"repertoire_id": "R2", "subject": {"subject_id": "S02", "age": {"age_min": 7, "age_max": 7, "age_unit": {"label": "week"}}},
		"sample": [{"sample_id": "a", "collection_time_point_relative": {"value": 28, "unit": {"label": "day"}}}]}]`
	reps, err = ReadRepertoires([]byte(v20))
	if err != nil || reps[0].Age != "7 week" || reps[0].TimePoint != "28 day" {
		t.Fatalf("%+v %v", reps, err)
	}
	if _, err := ReadRepertoires([]byte(`{"x": 1}`)); err == nil {
		t.Fatal("a file without repertoires was read")
	}
	if _, err := ReadRepertoires([]byte(`{`)); err == nil {
		t.Fatal("not JSON")
	}
}

// Any bytes give repertoires or an error, never a panic.
func FuzzRepertoires(f *testing.F) {
	f.Add([]byte(`{"Repertoire": [{"repertoire_id": "R1", "subject": {"age": {"age_min": 1}}, "sample": [{"collection_time_point_relative": {"value": 2}}]}]}`))
	f.Add([]byte(`[{"repertoire_id": 5, "sample": "x", "subject": [1]}]`))
	f.Add([]byte(`{"repertoire_id": "R", "subject": {"diagnosis": {"disease_diagnosis": "flu"}, "mhc": "HLA-A*02"}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		ReadRepertoires(data)
	})
}

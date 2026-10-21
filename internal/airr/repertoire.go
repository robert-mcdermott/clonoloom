package airr

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Repertoire is what ClonoLoom takes from an AIRR Repertoire object (MiAIRR metadata) to
// annotate a sample: its identifiers, the subject, the sample's tissue, cells and time point, and
// the study. Fields hold labels as given; ontology terms ({"id": …, "label": …}) are read as
// their labels. The whole object is kept as JSON in Raw.
type Repertoire struct {
	RepertoireID string          `json:"repertoireId"`
	StudyID      string          `json:"studyId,omitempty"`
	StudyTitle   string          `json:"studyTitle,omitempty"`
	SubjectID    string          `json:"subjectId,omitempty"`
	Species      string          `json:"species,omitempty"`
	Sex          string          `json:"sex,omitempty"`
	Age          string          `json:"age,omitempty"`
	Diagnosis    string          `json:"diagnosis,omitempty"`
	StudyGroup   string          `json:"studyGroup,omitempty"`
	SampleID     string          `json:"sampleId,omitempty"`
	Tissue       string          `json:"tissue,omitempty"`
	CellSubset   string          `json:"cellSubset,omitempty"`
	TimePoint    string          `json:"timePoint,omitempty"`
	Template     string          `json:"template,omitempty"`
	Loci         []string        `json:"loci,omitempty"`
	SingleCell   *bool           `json:"singleCell,omitempty"`
	HLA          []string        `json:"hla,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

// maxMetadataBytes: an AIRR metadata file of a large study is a few megabytes.
const maxMetadataBytes = 64 << 20

// ReadRepertoires reads the Repertoire objects of an AIRR metadata file in JSON: an AIRR Data
// File ({"Repertoire": [...]}, as the ADC API answers), a list of Repertoire objects, or one. It
// reads schema 1.3 to 2.0, where several fields changed from text to objects or lists.
func ReadRepertoires(data []byte) ([]Repertoire, error) {
	if len(data) > maxMetadataBytes {
		return nil, errors.New("the metadata file is larger than 64 MB")
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("not JSON: %w", err)
	}
	var objects []any
	switch v := doc.(type) {
	case map[string]any:
		if list, ok := v["Repertoire"].([]any); ok {
			objects = list
		} else if _, ok := v["repertoire_id"]; ok {
			objects = []any{v}
		}
	case []any:
		objects = v
	}
	if objects == nil {
		return nil, errors.New("no AIRR Repertoire objects in the file")
	}
	out := make([]Repertoire, 0, len(objects))
	for i, o := range objects {
		m, ok := o.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("repertoire %d is not an object", i+1)
		}
		raw, _ := json.Marshal(m)
		r := Repertoire{Raw: raw, RepertoireID: text(m["repertoire_id"])}
		if study, ok := m["study"].(map[string]any); ok {
			r.StudyID, r.StudyTitle = text(study["study_id"]), text(study["study_title"])
		}
		if subject, ok := m["subject"].(map[string]any); ok {
			r.SubjectID = text(subject["subject_id"])
			r.Species = label(subject["species"])
			r.Sex = label(subject["sex"])
			r.Age = age(subject)
			var diagnoses, groups []string
			for _, d := range list(subject["diagnosis"]) {
				if dm, ok := d.(map[string]any); ok {
					diagnoses = appendNew(diagnoses, label(dm["disease_diagnosis"]))
					groups = appendNew(groups, text(dm["study_group_description"]))
				}
			}
			r.Diagnosis, r.StudyGroup = strings.Join(diagnoses, "; "), strings.Join(groups, "; ")
			for _, h := range list(subject["mhc"]) {
				r.HLA = appendNew(r.HLA, text(h))
			}
		}
		var samples []string
		for _, s := range list(m["sample"]) {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			samples = appendNew(samples, text(sm["sample_id"]))
			r.Tissue = joinNew(r.Tissue, label(sm["tissue"]))
			r.CellSubset = joinNew(r.CellSubset, label(sm["cell_subset"]))
			r.Template = joinNew(r.Template, label(sm["template_class"]))
			r.TimePoint = joinNew(r.TimePoint, timePoint(sm))
			if b, ok := sm["single_cell"].(bool); ok {
				r.SingleCell = &b
			}
			for _, t := range list(sm["pcr_target"]) {
				if tm, ok := t.(map[string]any); ok {
					r.Loci = appendNew(r.Loci, text(tm["pcr_target_locus"]))
				}
			}
		}
		r.SampleID = strings.Join(samples, "; ")
		sort.Strings(r.Loci)
		out = append(out, r)
	}
	return out, nil
}

// text is a scalar as text ("" for null, objects and lists).
func text(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

// label is an ontology term's label (or its id when it has no label), or a scalar's text.
func label(v any) string {
	if m, ok := v.(map[string]any); ok {
		if l := text(m["label"]); l != "" {
			return l
		}
		return text(m["id"])
	}
	return text(v)
}

func list(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case nil:
		return nil
	}
	return []any{v}
}

func appendNew(values []string, v string) []string {
	if v == "" {
		return values
	}
	for _, have := range values {
		if have == v {
			return values
		}
	}
	return append(values, v)
}

func joinNew(joined, v string) string {
	if v == "" || joined == v || strings.Contains("; "+joined+"; ", "; "+v+"; ") {
		return joined
	}
	if joined == "" {
		return v
	}
	return joined + "; " + v
}

// age: "34 year", "30–40 year" (schema 1.3's age_min, age_max and age_unit, and 2.0's age
// object with the same fields).
func age(subject map[string]any) string {
	src := subject
	if a, ok := subject["age"].(map[string]any); ok {
		src = a
	} else if s := text(subject["age"]); s != "" {
		return s
	}
	lo, hi, unit := text(src["age_min"]), text(src["age_max"]), label(src["age_unit"])
	if lo == "" && hi == "" {
		return ""
	}
	value := lo
	if hi != "" && hi != lo {
		if lo == "" {
			value = hi
		} else {
			value = lo + "–" + hi
		}
	}
	return strings.TrimSpace(value + " " + unit)
}

// timePoint: "14 day (vaccination)" from collection_time_point_relative, its unit and reference
// (2.0 nests them in an object).
func timePoint(sample map[string]any) string {
	src := sample
	if t, ok := sample["collection_time_point_relative"].(map[string]any); ok {
		src = t
		if v := text(t["value"]); v != "" {
			return strings.TrimSpace(v + " " + label(t["unit"]) + reference(sample))
		}
	}
	v := text(src["collection_time_point_relative"])
	if v == "" {
		return ""
	}
	return strings.TrimSpace(v + " " + label(src["collection_time_point_relative_unit"]) + reference(sample))
}

func reference(sample map[string]any) string {
	if r := text(sample["collection_time_point_reference"]); r != "" {
		return " (" + r + ")"
	}
	return ""
}

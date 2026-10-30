package airr

// MiAIRR reporting inventory: AIRR Standards 2.0, data_elements.html and
// datarep/airr_schema_requirement_levels.html (AIRR Community, reviewed 2026-10-29).
// ClonoLoom's choice: describe evidence, never certify compliance or validity.
// Each repeated object has its own JSON-pointer path; no field is pooled across it.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const MiAIRRChecklistVersion = "clonoloom-miairr-2.0-v1"
const miAIRRRowLimit = 2000

// MiAIRRItem describes presence only. Level is AIRR's reporting level, not the
// schema's required-key flag. Scope rows carry no field-level requirement.
type MiAIRRItem struct {
	Path      string `json:"path"`
	Group     string `json:"group"`
	Level     string `json:"level"`
	State     string `json:"state"`
	Condition string `json:"condition,omitempty"`
	Remedy    string `json:"remedy"`
}

type MiAIRRChecklist struct {
	Version         string         `json:"version"`
	SchemaVersion   string         `json:"schemaVersion"`
	DeclaredVersion string         `json:"declaredVersion"`
	Status          string         `json:"status"`
	Notice          string         `json:"notice"`
	Items           []MiAIRRItem   `json:"items"`
	Counts          map[string]int `json:"counts"`
}

type miairrField struct{ Name, Level, Kind string }
type miairrGroup struct {
	Name, Path, Condition string
	Fields                []miairrField
}

// AssessMiAIRR reads the captured record, not extracted display annotations.
// Original numeric tokens are never converted to floats or copied into results.
// The 4 MiB/2000-row bounds refuse the whole assessment; no truncated denominator.
func AssessMiAIRR(ctx context.Context, raw json.RawMessage, schemaVersion string) MiAIRRChecklist {
	r := MiAIRRChecklist{Version: MiAIRRChecklistVersion, SchemaVersion: "2.0", DeclaredVersion: schemaVersion,
		Status: "not-assessed", Items: []MiAIRRItem{}, Counts: map[string]int{}}
	if schemaVersion != "2.0" && schemaVersion != "2.0.0" {
		r.Notice = "This checklist supports declared AIRR schema 2.0 or 2.0.0 only. Review the source's actual schema version; do not relabel older metadata without migration. Original evidence remains available."
		return r
	}
	if len(raw) > 4<<20 {
		r.Notice = "Record exceeds the 4 MiB checklist limit; no assessment was returned."
		return r
	}
	if err := checkMetadataJSON(ctx, raw); err != nil {
		r.Notice = "Could not assess the captured record: " + err.Error()
		return r
	}
	var object map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&object); err != nil || object == nil {
		r.Notice = "Checklist requires a repertoire object."
		return r
	}
	add := func(item MiAIRRItem) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(r.Items) >= miAIRRRowLimit {
			return fmt.Errorf("checklist exceeds %d evidence rows", miAIRRRowLimit)
		}
		r.Items = append(r.Items, item)
		r.Counts[item.State]++
		return nil
	}
	for _, group := range miairrGroups {
		if err := walkMiAIRR(object, strings.Split(group.Path, "/"), "", group, add); err != nil {
			r.Items = []MiAIRRItem{}
			r.Counts = map[string]int{}
			r.Notice = "No assessment returned: " + err.Error() + ". Original evidence is unchanged."
			return r
		}
	}
	r.Status = "review"
	r.Notice = "Reporting presence only, not MiAIRR compliance or scientific validity. Covers Repertoire metadata sets 1–5; set 6 processed sequences, raw-file availability, nested schema conformance, vocabulary/ontology validity and scientific truth are not assessed. Counts are evidence rows, not independent samples or a completeness score."
	return r
}

func walkMiAIRR(value any, parts []string, path string, group miairrGroup, add func(MiAIRRItem) error) error {
	if len(parts) == 0 {
		object, ok := value.(map[string]any)
		if !ok {
			return add(miairrScope(group, path, value, true))
		}
		for _, field := range group.Fields {
			v, present := object[field.Name]
			state := miairrState(v, present, field.Kind)
			condition := group.Condition
			if field.Name == "library_generation_kit_version" {
				condition = "Review whether a commercial library protocol was used; absence does not mean not applicable."
			}
			if strings.HasPrefix(field.Name, "paired_") {
				condition = "Review paired-read applicability; absence does not establish single-end sequencing."
			}
			if err := add(MiAIRRItem{Path: path + "/" + field.Name, Group: group.Name, Level: field.Level, State: state, Condition: condition, Remedy: miairrRemedy(state, field.Level)}); err != nil {
				return err
			}
		}
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return add(miairrScope(group, path, value, true))
	}
	key := strings.TrimSuffix(parts[0], "[]")
	next := path + "/" + key
	v, present := object[key]
	if strings.HasSuffix(parts[0], "[]") {
		list, ok := v.([]any)
		if !ok || len(list) == 0 {
			return add(miairrScope(group, next, v, present))
		}
		for i, child := range list {
			if err := walkMiAIRR(child, parts[1:], fmt.Sprintf("%s/%d", next, i), group, add); err != nil {
				return err
			}
		}
		return nil
	}
	if _, ok := v.(map[string]any); !ok {
		return add(miairrScope(group, next, v, present))
	}
	return walkMiAIRR(v, parts[1:], next, group, add)
}

func miairrScope(group miairrGroup, path string, value any, present bool) MiAIRRItem {
	state := miairrState(value, present, "scope")
	return MiAIRRItem{Path: path, Group: group.Name, Level: "scope", State: state, Condition: group.Condition,
		Remedy: "Review this parent scope in the source. Its child fields were not assessed; supply the appropriate object/list or document applicability. No not-applicable value is inferred."}
}

func miairrState(value any, present bool, kind string) string {
	if !present {
		return "absent"
	}
	if value == nil {
		return "null"
	}
	if s, ok := value.(string); ok {
		if strings.TrimSpace(s) == "" {
			return "empty"
		}
		// Exact AIRR tokens only. Nonstandard unknown text is visible, never
		// promoted to a standard reporting reason or substantive information.
		switch s {
		case "not_applicable", "not_collected", "missing":
			return s
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "unknown", "n/a", "na", "not known", "not applicable":
			return "unknown-marker"
		}
		if kind == "string" {
			return "value-present"
		}
		return "shape-review"
	}
	switch v := value.(type) {
	case bool:
		if kind == "boolean" {
			return "value-present"
		}
	case json.Number:
		// Integer range/positivity are semantic checks outside this checklist.
		if kind == "integer" && !strings.ContainsAny(v.String(), ".eE") {
			return "value-present"
		}
	case map[string]any:
		if len(v) == 0 {
			return "empty"
		}
		if kind == "object" {
			return "structure-review"
		}
	case []any:
		if len(v) == 0 {
			return "empty"
		}
		if kind == "array" {
			return "structure-review"
		}
	}
	return "shape-review"
}

func miairrRemedy(state, level string) string {
	if level == "defined" && state == "absent" {
		return "Optional (defined): use this field if reporting information with this meaning; absence is not a required-field gap."
	}
	switch state {
	case "value-present":
		return "A value is supplied. Review its meaning, units, vocabulary and scientific accuracy against the source."
	case "structure-review":
		return "A nonempty structure is supplied. Review its members, required subfields, units and identifiers; nested completeness and ontology conformance are not assessed."
	case "absent":
		return "Report this field from source evidence; if information is unavailable, explicitly report the reason where permitted. Do not invent a value."
	case "shape-review":
		return "Value shape differs from this checklist's expected field type. Review the AIRR schema and original evidence; nothing was coerced."
	case "unknown-marker":
		return "Nonstandard unknown marker. Review and explicitly encode the appropriate AIRR missingness reason where permitted; the original text is unchanged."
	default:
		if level == "essential" {
			return "Essential information must not be null-like. Recover substantive information from the source; this entry cannot supply it."
		}
		return "Explicit null-like reporting, not biological information. For important fields AIRR permits null-like reporting; retain the field and review its reason, applicability and schema representation."
	}
}

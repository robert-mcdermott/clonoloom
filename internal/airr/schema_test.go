package airr

import (
	"math"
	"testing"
)

func TestBooleansAsTheAIRRLibrariesReadThem(t *testing.T) {
	for _, s := range []string{"T", "TRUE", "True", "true", "t", "1"} {
		if v, missing, ok := ParseBool(s); !v || missing || !ok {
			t.Errorf("%q", s)
		}
	}
	for _, s := range []string{"F", "FALSE", "False", "false", "f", "0"} {
		if v, missing, ok := ParseBool(s); v || missing || !ok {
			t.Errorf("%q", s)
		}
	}
	if _, missing, ok := ParseBool(""); !missing || !ok {
		t.Error("empty is missing")
	}
	for _, s := range []string{"yes", "Y", "2", "None"} {
		if _, _, ok := ParseBool(s); ok {
			t.Errorf("%q is not a boolean", s)
		}
	}
	if FormatBool(true, false) != "T" || FormatBool(false, false) != "F" || FormatBool(true, true) != "" {
		t.Error("FormatBool")
	}
}

func TestIntegersAndNumbers(t *testing.T) {
	if v, missing, exact, ok := ParseInt("42"); v != 42 || missing || !exact || !ok {
		t.Error("42")
	}
	if v, _, exact, ok := ParseInt("12.0"); v != 12 || exact || !ok {
		t.Error("12.0 is read, inexactly written")
	}
	if _, _, _, ok := ParseInt("12.5"); ok {
		t.Error("12.5 is not an integer")
	}
	if _, missing, _, ok := ParseInt(""); !missing || !ok {
		t.Error("empty")
	}
	if v, missing, ok := ParseNumber("0.97"); v != 0.97 || missing || !ok {
		t.Error("0.97")
	}
	if v, missing, ok := ParseNumber("NaN"); !math.IsNaN(v) || !missing || !ok {
		t.Error("NaN is missing")
	}
	if _, _, ok := ParseNumber("abc"); ok {
		t.Error("abc")
	}
	for _, f := range Required {
		if _, ok := Fields[f]; !ok {
			t.Errorf("required field %s has no type", f)
		}
	}
	if Fields["duplicate_count"] != Integer || Fields["productive"] != Boolean || Fields["v_identity"] != Number || Fields["junction_aa"] != String {
		t.Error("types")
	}
}

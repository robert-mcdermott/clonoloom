package bcell

import (
	"clonoloom/internal/rep"
	"testing"
)

func raw(values map[string]string) [rep.EvidenceCount]string {
	var r [rep.EvidenceCount]string
	for name, value := range values {
		r[fields[name]] = value
	}
	return r
}
func TestEvidenceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		valid  bool
		code   string
	}{
		{"gaps and ambiguity", map[string]string{"sequence_alignment": "AcG.N-T", "germline_alignment": "ACG.N-T"}, true, ""},
		{"reverse", map[string]string{"sequence": "ACGT", "sequence_alignment": "ACGT", "germline_alignment": "ACGT", "rev_comp": "T", "fwr1_start": "2", "fwr1_end": "4"}, true, ""},
		{"unequal", map[string]string{"sequence_alignment": "ACT", "germline_alignment": "AC"}, false, "evidence-alignment"},
		{"absent", map[string]string{}, false, ""},
		{"unknown symbol", map[string]string{"sequence_alignment": "AC?", "germline_alignment": "ACT"}, false, "evidence-alphabet"},
		{"partial bounds", map[string]string{"sequence": "AC", "fwr1_end": "3"}, false, "evidence-coordinate"},
		{"reversed", map[string]string{"v_germline_start": "9", "v_germline_end": "3"}, false, "evidence-coordinate"},
		{"unknown reference length", map[string]string{"sequence_alignment": "AC", "germline_alignment": "AC", "v_germline_start": "900", "v_germline_end": "901"}, true, ""},
		{"explicit zero", map[string]string{"sequence_alignment": "AC", "germline_alignment": "AC", "np1_length": "0"}, true, ""},
		{"zero position", map[string]string{"fwr1_start": "0"}, false, "evidence-coordinate"},
		{"huge position", map[string]string{"fwr1_start": "99999999999999999999999"}, false, "evidence-coordinate"},
		{"CIGAR offsets", map[string]string{"sequence": "ACGTACGT", "v_cigar": "2S3N4M", "v_sequence_start": "3", "v_sequence_end": "6", "v_germline_start": "4", "v_germline_end": "7"}, false, ""},
		{"CIGAR mismatch", map[string]string{"v_cigar": "2S3N4M", "v_sequence_start": "1"}, false, "evidence-cigar"},
		{"CIGAR query bounds", map[string]string{"sequence": "AC", "v_cigar": "3M"}, false, "evidence-cigar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := raw(tc.values)
			before := r
			e := Check(r)
			if r != before || e.Valid != tc.valid {
				t.Fatalf("%+v", e)
			}
			if tc.code == "" {
				if len(e.Issues) != 0 {
					t.Fatal(e.Issues)
				}
			} else {
				found := false
				for _, i := range e.Issues {
					found = found || i.Code == tc.code
				}
				if !found {
					t.Fatal(e.Issues)
				}
			}
		})
	}
	a, b, err := Interval(2, 4, 4)
	if err != nil || a != 1 || b != 4 {
		t.Fatal(a, b, err)
	}
	for _, v := range [][3]int64{{0, 2, 3}, {3, 2, 3}, {1, 4, 3}} {
		if _, _, err := Interval(v[0], v[1], v[2]); err == nil {
			t.Fatal(v)
		}
	}
}
func TestAIRRCigar(t *testing.T) {
	c, err := ParseCigar("418S10N16M71S5N")
	if err != nil || c.QueryStart != 418 || c.ReferenceStart != 10 || c.QueryLength != 16 || c.ReferenceLength != 16 || c.QueryTotal != 505 {
		t.Fatal(c, err)
	}
	c, err = ParseCigar("3=2X1I2D4=")
	if err != nil || c.QueryLength != 10 || c.ReferenceLength != 11 {
		t.Fatal(c, err)
	}
	for _, s := range []string{"", "0M", "2H", "2P", "4", "M", "2M2=", "1N1S4M", "4M1N1S", "2M1S2M", "99999999999999999999M", "2147483647M1I"} {
		if _, err := ParseCigar(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func FuzzCigar(f *testing.F) {
	for _, s := range []string{"418S10N16M71S5N", "2M1D3M", "999999999999M", "0M"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		c, e := ParseCigar(s)
		if e == nil && (c.QueryLength <= 0 || c.ReferenceLength <= 0 || c.QueryTotal < c.QueryLength) {
			t.Fatal(c)
		}
	})
}
func FuzzInterval(f *testing.F) {
	f.Add(int64(1), int64(2), int64(2))
	f.Fuzz(func(t *testing.T, a, b, n int64) {
		start, end, err := Interval(a, b, n)
		if err == nil && (start < 0 || end <= start || (n >= 0 && end > n)) {
			t.Fatal(start, end)
		}
	})
}

func TestExactCoordinateDecimals(t *testing.T) {
	for _, s := range []string{"12", "12.0", "12.000"} {
		n, err := integer(s)
		if err != nil || n != 12 {
			t.Fatal(s, n, err)
		}
	}
	for _, s := range []string{"12.1", "12.", "9223372036854775808", "9007199254740993.1"} {
		if _, err := integer(s); err == nil {
			t.Fatal(s)
		}
	}
	n, err := integer("9007199254740993.000")
	if err != nil || n != 9007199254740993 {
		t.Fatal(n, err)
	}
}

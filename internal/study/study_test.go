package study

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTimePointsAreRead(t *testing.T) {
	for _, c := range []struct {
		text  string
		kind  string
		days  float64
		label string
		event string
	}{
		{"day 28", Day, 28, "day 28", ""},
		{"Day28", Day, 28, "day 28", ""},
		{"D+28", Day, 28, "day 28", ""},
		{"d-7", Day, -7, "day -7", ""},
		{"D0", Day, 0, "day 0", ""},
		{"28d", Day, 28, "day 28", ""},
		{"28 days", Day, 28, "day 28", ""},
		{"wk4", Day, 28, "week 4", ""},
		{"4 weeks", Day, 28, "week 4", ""},
		{"W4", Day, 28, "week 4", ""},
		{"month 3", Day, 3 * 365.25 / 12, "month 3", ""},
		{"3mo", Day, 3 * 365.25 / 12, "month 3", ""},
		{"M6", Day, 6 * 365.25 / 12, "month 6", ""},
		{"1 year", Day, 365.25, "year 1", ""},
		{"y2", Day, 730.5, "year 2", ""},
		{"6h", Day, 0.25, "hour 6", ""},
		{"24 hours", Day, 1, "hour 24", ""},
		{"day 1.5", Day, 1.5, "day 1.5", ""},
		// AIRR Repertoire metadata, as the reader gives them.
		{"14 day (vaccination)", Day, 14, "day 14", "vaccination"},
		{"28 days post-infusion", Day, 28, "day 28", "infusion"},
		{"21", Day, 21, "day 21", ""},
		{"baseline", Before, 0, "baseline", ""},
		{"Pre-infusion", Before, 0, "baseline", ""},
		{"screening", Before, 0, "baseline", ""},
		{"post", After, 0, "after", ""},
		{"post-vaccination", After, 0, "after", ""},
		{"T1", Visit, 0, "visit 1", ""},
		{"visit 3", Visit, 0, "visit 3", ""},
		{"tp2", Visit, 0, "visit 2", ""},
	} {
		got, ok := ParseTime(c.text)
		if !ok || got.Kind != c.kind || got.Label != c.label || got.Event != c.event {
			t.Errorf("%q: %+v %v", c.text, got, ok)
			continue
		}
		if c.kind == Day && (got.Days == nil || *got.Days != c.days) {
			t.Errorf("%q: %v days, want %v", c.text, got.Days, c.days)
		}
		if c.kind != Day && got.Days != nil {
			t.Errorf("%q: days %v", c.text, *got.Days)
		}
	}
	for _, text := range []string{"", "soon", "day", "week four", "28 furlongs", "d28x", "relapse", strings.Repeat("9", 400) + " years"} {
		if got, ok := ParseTime(text); ok {
			t.Errorf("%q read as %+v", text, got)
		}
	}
	if got, _ := ParseTime("21"); !got.Bare {
		t.Error("a bare number is not marked")
	}
}

func TestADesignIsOrderedAndChecked(t *testing.T) {
	d := Describe([]Sample{
		{Name: "P10_d28", Subject: "P10", Time: "day 28"},
		{Name: "P2_wk4", Subject: "P2", Time: "week 4"},
		{Name: "P2_d28b", Subject: "P2", Time: "day 28"},
		{Name: "P2_pre", Subject: "P2", Time: "baseline"},
		{Name: "P2_IP", Subject: "P2", Tissue: "infusion product"},
		{Name: "P2_d7", Subject: "P2", Time: "d7"},
		{Name: "P2_x", Subject: "P2"},
		{Name: "P10_donor", Subject: "P10", Reference: "donor", Time: "day -30"},
		{Name: "P10_odd", Subject: "P10", Time: "relapse"},
		{Name: "loose", Time: "day 7"},
		{Name: "P3_t1", Subject: "P3", Time: "T1"},
		{Name: "P3_d14", Subject: "P3", Time: "14"},
		{Name: "P3_ref", Subject: "P3", Tissue: "PBMC", Reference: "no"},
	})
	ids := []string{}
	for _, s := range d.Subjects {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, " ") != "P2 P3 P10" {
		t.Fatalf("subjects %v", ids)
	}
	p2 := d.Subjects[0]
	var order []string
	for _, p := range p2.Samples {
		order = append(order, p.Name)
	}
	// The product (a reference without a time) first, then baseline, then by days; week 4 and day
	// 28 are one time, and replicates.
	if strings.Join(order, " ") != "P2_IP P2_pre P2_d7 P2_d28b P2_wk4" {
		t.Errorf("P2's order %v", order)
	}
	if len(p2.Replicates) != 1 || strings.Join(p2.Replicates[0], " ") != "P2_d28b P2_wk4" {
		t.Errorf("P2's replicates %v", p2.Replicates)
	}
	if p2.Samples[0].Reference != "infusion product" || p2.Samples[0].ReferenceFrom != "tissue" || strings.Join(p2.Untimed, "") != "P2_x" {
		t.Errorf("P2: %+v", p2)
	}
	p10 := d.Subjects[2]
	if p10.Samples[0].Name != "P10_donor" || p10.Samples[0].ReferenceFrom != "annotation" || len(p10.Samples) != 2 {
		t.Errorf("P10: %+v", p10)
	}
	if strings.Join(d.Unassigned, "") != "loose" || *d.FirstDay != 7 || *d.LastDay != 28 {
		t.Errorf("unassigned %v, span %v–%v", d.Unassigned, *d.FirstDay, *d.LastDay)
	}
	found := map[string]Finding{}
	for _, f := range d.Findings {
		found[f.Code] = f
	}
	for code, want := range map[string]int{
		"study-no-subject": 1, "study-time-unread": 1, "study-no-time": 2, "study-bare-number": 1,
		"study-replicates": 1, "study-one-sample": 1, "study-no-reference": 1, "study-mixed-times": 1,
	} {
		if found[code].Count != want {
			t.Errorf("%s: %+v", code, found[code])
		}
	}
	// P10 has the donor and one time point; P3's "no" is not a reference.
	if strings.Join(found["study-one-sample"].Examples, "") != "P10" || strings.Join(found["study-no-reference"].Examples, "") != "P3" {
		t.Errorf("%+v %+v", found["study-one-sample"], found["study-no-reference"])
	}
	// Warnings first.
	if d.Findings[0].Severity != "warning" || d.Findings[len(d.Findings)-1].Severity != "note" {
		t.Errorf("order: %+v", d.Findings)
	}
	if raw, err := json.Marshal(d); err != nil || !strings.Contains(string(raw), `"days":28`) {
		t.Errorf("%v %s", err, raw)
	}
}

func TestAnEmptyDesign(t *testing.T) {
	d := Describe(nil)
	if d.Subjects == nil || d.Findings == nil || len(d.Subjects) != 0 {
		t.Errorf("%+v", d)
	}
	// A subject none of whose samples has a time point: its samples a list still, empty.
	if raw, _ := json.Marshal(Describe([]Sample{{Name: "a", Subject: "S"}})); !strings.Contains(string(raw), `"samples":[]`) {
		t.Errorf("%s", raw)
	}
}

// Any text: no panic; a time point read is of a kind, and its days a finite number.
func FuzzParseTime(f *testing.F) {
	for _, s := range []string{"day 28", "D+28", "4 weeks", "14 day (vaccination)", "baseline", "T1", "21", "((", "d-1e308", "9999999999999999999999 years"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, ok := ParseTime(s)
		if !ok {
			return
		}
		switch got.Kind {
		case Day:
			if got.Days == nil || *got.Days != *got.Days || *got.Days > 1e308 || *got.Days < -1e308 {
				t.Fatalf("%q: days %v", s, got.Days)
			}
		case Before, After, Visit:
		default:
			t.Fatalf("%q: kind %q", s, got.Kind)
		}
		Describe([]Sample{{Name: "a", Subject: "S", Time: s}, {Name: "b", Subject: "S", Time: s}})
	})
}

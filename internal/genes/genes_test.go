package genes

import "testing"

func TestParseIMGTNames(t *testing.T) {
	cases := []struct {
		raw                               string
		locus                             string
		segment                           Segment
		family, gene, allele, allelelevel string
	}{
		{"TRBV5-1*01", "TRB", V, "TRBV5", "TRBV5-1", "01", "TRBV5-1*01"},
		{"TRBV30", "TRB", V, "TRBV30", "TRBV30", "", "TRBV30"},
		{"TRAV29/DV5*01", "TRA", V, "TRAV29", "TRAV29/DV5", "01", "TRAV29/DV5*01"},
		{"TRBV20/OR9-2*03", "TRB", V, "TRBV20", "TRBV20/OR9-2", "03", "TRBV20/OR9-2*03"},
		{"TRBD2*02", "TRB", D, "TRBD2", "TRBD2", "02", "TRBD2*02"},
		{"TRBJ2-7*01", "TRB", J, "TRBJ2", "TRBJ2-7", "01", "TRBJ2-7*01"},
		{"TRBC2", "TRB", C, "TRBC2", "TRBC2", "", "TRBC2"},
		{"IGHV3-23D*01", "IGH", V, "IGHV3", "IGHV3-23D", "01", "IGHV3-23D*01"},
		{"IGHV1-69-2*01", "IGH", V, "IGHV1", "IGHV1-69-2", "01", "IGHV1-69-2*01"},
		{"IGKV1D-39*01", "IGK", V, "IGKV1", "IGKV1D-39", "01", "IGKV1D-39*01"},
		{"IGHD3-10*01", "IGH", D, "IGHD3", "IGHD3-10", "01", "IGHD3-10*01"},
		{"IGHD", "IGH", C, "IGHD", "IGHD", "", "IGHD"},
		{"IGHG1", "IGH", C, "IGHG1", "IGHG1", "", "IGHG1"},
		{"IGHM*01", "IGH", C, "IGHM", "IGHM", "01", "IGHM*01"},
		{"IGLC2", "IGL", C, "IGLC2", "IGLC2", "", "IGLC2"},
		{"TRGV9*01", "TRG", V, "TRGV9", "TRGV9", "01", "TRGV9*01"},
		{"trbv5-1*01", "TRB", V, "TRBV5", "TRBV5-1", "01", "TRBV5-1*01"},
		{"TRBV5-1*00(1250)", "TRB", V, "TRBV5", "TRBV5-1", "00", "TRBV5-1*00"},
	}
	for _, c := range cases {
		n := Parse(c.raw)
		if n.Locus != c.locus || n.Segment != c.segment || n.Family != c.family || n.Gene != c.gene || n.Allele != c.allele || n.At(LevelAllele) != c.allelelevel {
			t.Errorf("Parse(%q) = %+v", c.raw, n)
		}
	}
	for _, raw := range []string{"", "None", "NA", "unresolved", "  "} {
		if n := Parse(raw); n.Locus != "" || n.Raw != "" {
			t.Errorf("%q should be no call: %+v", raw, n)
		}
	}
	for _, raw := range []string{"CD3E", "XYZV1", "TRB", "IGHX"} {
		if n := Parse(raw); n.Locus != "" || n.Raw != raw {
			t.Errorf("%q is not a receptor gene: %+v", raw, n)
		}
	}
}

func TestCallsAtEachLevel(t *testing.T) {
	c := ParseCall("TRBV6-2*01,TRBV6-3*01,TRBV6-2*02")
	if len(c.Names) != 3 || c.Locus() != "TRB" {
		t.Fatalf("%+v", c)
	}
	if got := c.Key(LevelGene); got != "TRBV6-2|TRBV6-3" {
		t.Errorf("gene key %q", got)
	}
	if got := c.Key(LevelFamily); got != "TRBV6" || !c.Resolved(LevelFamily) || c.Resolved(LevelGene) {
		t.Errorf("family key %q", got)
	}
	if got := c.Key(LevelAllele); got != "TRBV6-2*01|TRBV6-2*02|TRBV6-3*01" {
		t.Errorf("allele key %q", got)
	}
	if c.Key(LevelNone) != "" {
		t.Error("no level, no key")
	}
	one := ParseCall("IGHV3-23*01")
	if one.Key(LevelGene) != "IGHV3-23" || !one.Resolved(LevelAllele) {
		t.Errorf("%+v", one)
	}
	mixed := ParseCall("TRAV29/DV5*01, TRDV5*01")
	if mixed.Locus() != "" || mixed.Key(LevelGene) != "TRAV29/DV5|TRDV5" {
		t.Errorf("%+v", mixed)
	}
	junk := ParseCall("TRBV5-1*01,foo")
	if len(junk.Names) != 1 || junk.Unreadable != 1 {
		t.Errorf("%+v", junk)
	}
	if empty := ParseCall(""); len(empty.Names) != 0 || empty.Resolved(LevelGene) || empty.Key(LevelGene) != "" {
		t.Errorf("%+v", empty)
	}
}

func TestAdaptiveNamesMapToIMGT(t *testing.T) {
	cases := []struct{ adaptive, imgt string }{
		{"TCRBV05-01*01", "TRBV5-1*01"},
		{"TCRBV09-01", "TRBV9"},
		{"TCRBV30-01*01", "TRBV30*01"},
		{"TCRBV20-01*01", "TRBV20-1*01"},
		{"TCRBV29-01*01", "TRBV29-1*01"},
		{"TCRBV20-or09_02", "TRBV20/OR9-2"},
		{"TCRBV20", "TRBV20"},
		{"TCRBD01-01*01", "TRBD1*01"},
		{"TCRBD02-01*02", "TRBD2*02"},
		{"TCRBJ02-07*01", "TRBJ2-7*01"},
		{"TCRAV29-01", "TRAV29/DV5"},
		{"TCRAV12-02*02", "TRAV12-2*02"},
		{"IGHV03-23*01", "IGHV3-23*01"},
		{"unresolved", ""},
		{"", ""},
	}
	for _, c := range cases {
		got, ok := FromAdaptive(c.adaptive, true)
		if got != c.imgt || !ok {
			t.Errorf("%s: %q %v, want %q", c.adaptive, got, ok, c.imgt)
		}
		if g := Parse(got); got != "" && g.Locus == "" {
			t.Errorf("%s maps to %q, which Parse does not read", c.adaptive, got)
		}
	}
	if got, _ := FromAdaptive("TCRBV09-01", false); got != "TRBV9-1" {
		t.Errorf("mouse names keep their numbers: %q", got)
	}
	if _, ok := FromAdaptive("XYZ", true); ok {
		t.Error("an unknown name was mapped")
	}
	if got, ok := FromAdaptiveCall("TCRBV12-03,TCRBV12-04", true); got != "TRBV12-3,TRBV12-4" || !ok {
		t.Errorf("ties %q", got)
	}
	if got, ok := FromAdaptiveCall("TCRBV12-03/12-04", true); got != "TRBV12-3,TRBV12-4" || !ok {
		t.Errorf("combined %q", got)
	}
	if got, ok := FromAdaptive("TCRBV20-X", true); got != "TRBV20" || !ok {
		t.Errorf("-X %q", got)
	}
	if known, pseudo := IsHumanGene("TRBV1"); !known || !pseudo {
		t.Error("TRBV1 is a human pseudogene")
	}
}

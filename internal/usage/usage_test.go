package usage

import (
	"encoding/json"
	"math"
	"runtime"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/rep"
)

// Two samples by hand. S1: clonotype A (TRBV5-1, TRBJ2-7, CASSLAGGPEAFF) of two rearrangements, 6
// with TRBD1 and 2 with TRBD2; B (a TRBV6-2/TRBV6-3 tie, no D, TRBJ1-1, CASSPGQGAYEQYF), 2; C
// (TRBV20-1, TRBJ2-1, TRBC2, CSARDRGNEQFF), 4. S2: TRBV5-1 (CASSLAGGPEAFF), 1; TRBV10-3
// (CASSQDRGYGYTF), 9.
func handTable(t *testing.T) (*rep.Table, *clonotype.Table) {
	t.Helper()
	b := rep.NewBuilder(rep.Templates)
	add := func(sample, v, d, j, c, aa string, count float64) {
		r := rep.NewRow()
		r.Sample, r.Locus, r.V, r.D, r.J, r.C, r.JunctionAA, r.Count, r.Productive = sample, "TRB", v, d, j, c, aa, count, rep.True
		if err := b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	add("S1", "TRBV5-1*01", "TRBD1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", 6)
	add("S1", "TRBV6-2*01,TRBV6-3*01", "", "TRBJ1-1*01", "", "CASSPGQGAYEQYF", 2)
	add("S1", "TRBV5-1*01", "TRBD2*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", 2)
	add("S1", "TRBV20-1*01", "", "TRBJ2-1*01", "TRBC2", "CSARDRGNEQFF", 4)
	add("S2", "TRBV5-1*01", "", "TRBJ2-7*01", "", "CASSLAGGPEAFF", 1)
	add("S2", "TRBV10-3*01", "", "TRBJ1-2*01", "", "CASSQDRGYGYTF", 9)
	rt := b.Table()
	ct, err := clonotype.Compute(rt, nil, clonotype.Definition{Junction: "aa", V: "gene", J: "gene", ProductiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	return rt, ct
}

func find(t *testing.T, l Locus, seg, level string) map[string]Share {
	t.Helper()
	for _, g := range l.Genes {
		if g.Segment == seg && g.Level == level {
			out := map[string]Share{}
			for _, s := range g.Shares {
				out[s.Name] = s
			}
			return out
		}
	}
	t.Fatalf("no %s %s", seg, level)
	return nil
}

func near(a Number, b float64) bool { return math.Abs(float64(a)-b) < 1e-12 }

func TestSharesByHand(t *testing.T) {
	rt, ct := handTable(t)
	res := Measure(rt, ct, map[string]string{"S1": "a", "S2": "a"}, diversity.Options{})
	s1 := res.Samples[0]
	if s1.Name != "S1" || s1.Clonotypes != 3 || s1.Abundance != 14 || len(s1.Loci) != 1 {
		t.Fatalf("%+v", s1)
	}
	l := s1.Loci[0]
	if l.Locus != "TRB" || l.Clonotypes != 3 || l.Abundance != 14 {
		t.Fatalf("%+v", l)
	}
	for _, c := range []struct {
		seg, level, name      string
		clonotypes, abundance float64
	}{
		{"V", "gene", "TRBV5-1", 1.0 / 3, 8.0 / 14}, {"V", "gene", "TRBV6-2", 1.0 / 6, 1.0 / 14}, {"V", "gene", "TRBV6-3", 1.0 / 6, 1.0 / 14},
		{"V", "gene", "TRBV20-1", 1.0 / 3, 4.0 / 14}, {"V", "family", "TRBV6", 1.0 / 3, 2.0 / 14},
		// D: A's rearrangements split it 6:2.
		{"D", "gene", "TRBD1", 0.25, 6.0 / 14}, {"D", "gene", "TRBD2", 1.0 / 12, 2.0 / 14}, {"D", "gene", "", 2.0 / 3, 6.0 / 14},
		{"J", "family", "TRBJ2", 2.0 / 3, 12.0 / 14}, {"C", "gene", "TRBC2", 1.0 / 3, 4.0 / 14}, {"C", "gene", "", 2.0 / 3, 10.0 / 14},
		{"VJ", "gene", "TRBV6-3+TRBJ1-1", 1.0 / 6, 1.0 / 14}, {"VJ", "family", "TRBV20+TRBJ2", 1.0 / 3, 4.0 / 14},
	} {
		s, ok := find(t, l, c.seg, c.level)[c.name]
		if !ok || !near(s.Clonotypes, c.clonotypes) || !near(s.Abundance, c.abundance) {
			t.Errorf("%s %s %q: %+v, want %v and %v", c.seg, c.level, c.name, s, c.clonotypes, c.abundance)
		}
	}
	lengths := map[string]Share{}
	for _, s := range l.Lengths {
		lengths[s.Name] = s
	}
	if !near(lengths["12"].Clonotypes, 1.0/3) || !near(lengths["13"].Abundance, 8.0/14) || !near(lengths["14"].Abundance, 2.0/14) {
		t.Errorf("lengths %+v", l.Lengths)
	}
	if !near(l.MeanLength.Clonotypes, 13) || !near(l.MeanLength.Abundance, 180.0/14) {
		t.Errorf("mean length %+v", l.MeanLength)
	}
	// Composition over residues: S 5 of 39 by clonotypes, 24 of 180 by abundance.
	comp := map[string]Share{}
	for _, s := range l.Composition {
		comp[s.Name] = s
	}
	if !near(comp["S"].Clonotypes, 5.0/39) || !near(comp["S"].Abundance, 24.0/180) || !near(comp["C"].Clonotypes, 3.0/39) || l.Composition[0].Name != "A" {
		t.Errorf("composition %+v", l.Composition)
	}
	// Profiled: lengths 12, 13 and 14 (each at least 5% of the clonotypes over both samples); at
	// length 13, position 104 is always C, and position 107 S.
	if got := res.Profiled["TRB"]; len(got) != 3 || got[0] != 12 || got[2] != 14 {
		t.Fatalf("profiled %v", got)
	}
	var p13 Profile
	for _, p := range l.Profiles {
		if p.Length == 13 {
			p13 = p
		}
	}
	if len(p13.Positions) != 13 || p13.Positions[0].IMGT != "104" || p13.Positions[0].Shares[0].Name != "C" || !near(p13.Positions[0].Shares[0].Clonotypes, 1) || p13.Positions[3].IMGT != "107" || p13.Positions[3].Shares[0].Name != "S" {
		t.Errorf("profile of 13: %+v", p13)
	}
	// The group: each sample counted once. TRBV5-1 is 1/3 of S1's clonotypes and 1/2 of S2's.
	if len(res.Groups) != 1 || len(res.Groups[0].Samples) != 2 {
		t.Fatalf("groups %+v", res.Groups)
	}
	g := find(t, res.Groups[0].Loci[0], "V", "gene")
	if s := g["TRBV5-1"]; !near(s.Clonotypes, 5.0/12) || !near(s.ClonotypesInterval[0], 1.0/3) || !near(s.ClonotypesInterval[1], 0.5) {
		t.Errorf("group TRBV5-1 %+v", s)
	}
	if s := g["TRBV6-2"]; !near(s.Clonotypes, 1.0/12) || !near(s.ClonotypesInterval[0], 0) {
		t.Errorf("group TRBV6-2 %+v", s)
	}
	// Without a bootstrap, no intervals for the samples.
	if find(t, l, "V", "gene")["TRBV5-1"].ClonotypesInterval != nil {
		t.Error("an interval without replicates")
	}
}

func TestTheBootstrap(t *testing.T) {
	rt, ct := handTable(t)
	opt := diversity.DefaultOptions()
	a := Measure(rt, ct, nil, opt)
	before := runtime.GOMAXPROCS(1)
	b := Measure(rt, ct, nil, opt)
	runtime.GOMAXPROCS(before)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("different results on one processor")
	}
	l := a.Samples[0].Loci[0]
	for _, g := range l.Genes {
		for _, s := range g.Shares {
			if g.Segment == "VJ" {
				if s.ClonotypesInterval != nil {
					t.Errorf("pairs have no intervals: %+v", s)
				}
				continue
			}
			for _, v := range []struct {
				est Number
				iv  *Pair
			}{{s.Clonotypes, s.ClonotypesInterval}, {s.Abundance, s.AbundanceInterval}} {
				if v.iv == nil || v.iv[0] > v.est+1e-12 || v.iv[1] < v.est-1e-12 || v.iv[0] < 0 || v.iv[1] > 1 {
					t.Errorf("%s %s %q: %v outside %v", g.Segment, g.Level, s.Name, v.est, v.iv)
				}
			}
		}
	}
	// TRBV5-1 is one clonotype of three: its interval is wide, and its mean length's holds 13.
	if s := find(t, l, "V", "gene")["TRBV5-1"]; float64(s.ClonotypesInterval[1]-s.ClonotypesInterval[0]) < 0.3 {
		t.Errorf("TRBV5-1 %+v", s)
	}
	if m := l.MeanLength; m.ClonotypesInterval == nil || m.ClonotypesInterval[0] > 13 || m.ClonotypesInterval[1] < 13 {
		t.Errorf("mean length %+v", m)
	}
}

func TestIMGTPositions(t *testing.T) {
	for n, want := range map[int]string{
		5:  "104 105 106 117 118",
		11: "104 105 106 107 108 109 114 115 116 117 118",
		13: "104 105 106 107 108 109 110 113 114 115 116 117 118",
		14: "104 105 106 107 108 109 110 112 113 114 115 116 117 118",
		15: "104 105 106 107 108 109 110 111 112 113 114 115 116 117 118",
		17: "104 105 106 107 108 109 110 111 111.1 112.1 112 113 114 115 116 117 118",
		18: "104 105 106 107 108 109 110 111 111.1 111.2 112.1 112 113 114 115 116 117 118",
	} {
		if got := strings.Join(IMGTPositions(n), " "); got != want {
			t.Errorf("%d: %s, want %s", n, got, want)
		}
	}
}

func TestOrders(t *testing.T) {
	names := []string{"TRBV10-1", "", "TRBV2", "TRBV29/OR9-2", "TRBV29-1", "TRBV6-2"}
	s := make([]Share, len(names))
	for i, n := range names {
		s[i].Name = n
	}
	sortShares(s, geneLess)
	var got []string
	for _, x := range s {
		got = append(got, x.Name)
	}
	if strings.Join(got, ",") != "TRBV2,TRBV6-2,TRBV10-1,TRBV29-1,TRBV29/OR9-2," {
		t.Errorf("%v", got)
	}
}

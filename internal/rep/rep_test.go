package rep

import (
	"math"
	"testing"
)

func TestStringsInternAndKeepOrder(t *testing.T) {
	s := NewStrings(true)
	a, _ := s.Add("CASSLAGGPEAFF")
	b, _ := s.Add("CASSPGQGAYEQYF")
	a2, _ := s.Add("CASSLAGGPEAFF")
	empty, _ := s.Add("")
	if a != 1 || b != 2 || a2 != 1 || empty != 0 || s.Len() != 2 {
		t.Fatalf("codes %d %d %d %d", a, b, a2, empty)
	}
	if s.Get(1) != "CASSLAGGPEAFF" || s.Get(2) != "CASSPGQGAYEQYF" || s.Get(0) != "" || s.Get(9) != "" {
		t.Fatal("Get")
	}
	if code, ok := s.Lookup("CASSPGQGAYEQYF"); !ok || code != 2 {
		t.Fatal("Lookup")
	}
	s.Freeze()
	if _, err := s.Add("CASRF"); err == nil {
		t.Fatal("a frozen interning store accepted a new string")
	}
	if s.Get(2) != "CASSPGQGAYEQYF" {
		t.Fatal("a frozen store lost its strings")
	}
	ids := NewStrings(false)
	x, _ := ids.Add("contig_1")
	y, _ := ids.Add("contig_1")
	if x == y || ids.Get(y) != "contig_1" {
		t.Fatal("a store that does not intern keeps every string")
	}
}

func TestBuilderMakesOptionalColumnsWhenFirstNeeded(t *testing.T) {
	b := NewBuilder(Templates)
	r := NewRow()
	r.Sample, r.Locus, r.V, r.J, r.JunctionAA, r.Count, r.Productive, r.Line = "S1", "TRB", "TRBV5-1*01", "TRBJ2-7*01", "CASSLAGGPEAFF", 12, True, 2
	if err := b.Append(&r); err != nil {
		t.Fatal(err)
	}
	r2 := NewRow()
	r2.Sample, r2.Locus, r2.V, r2.JunctionAA, r2.Count, r2.Reads, r2.Cell, r2.Line = "S2", "TRB", "TRBV5-1*01", "CASSLAGGPEAFF", 3, 40, "AAACCTG-1", 3
	if err := b.Append(&r2); err != nil {
		t.Fatal(err)
	}
	tab := b.Table()
	if tab.Len() != 2 || len(tab.Samples) != 2 || tab.Samples[1].Name != "S2" || tab.Unit != Templates {
		t.Fatalf("%d rows, samples %+v", tab.Len(), tab.Samples)
	}
	if tab.V[0] != tab.V[1] || tab.JunctionAA[0] != tab.JunctionAA[1] || tab.Dict.Get(tab.V[0]) != "TRBV5-1*01" {
		t.Fatal("repeated strings are not interned")
	}
	if len(tab.Reads) != 2 || !math.IsNaN(tab.Reads[0]) || tab.Reads[1] != 40 {
		t.Fatalf("reads %v", tab.Reads)
	}
	if len(tab.Cell) != 2 || tab.Cell[0] != 0 || tab.Dict.Get(tab.Cell[1]) != "AAACCTG-1" {
		t.Fatalf("cells %v", tab.Cell)
	}
	if tab.UMIs != nil || tab.IsCell != nil || tab.Clone != nil {
		t.Fatal("columns no row gave were made")
	}
	if tab.LocusName(0) != "TRB" || tab.Productive[1] != Missing || tab.Line[1] != 3 {
		t.Fatal("row values")
	}
	if c := tab.Call(tab.V[0]); c.Key(2) != "TRBV5-1" {
		t.Fatalf("call %+v", c)
	}
	if tab.Size() <= 0 {
		t.Fatal("size")
	}
}

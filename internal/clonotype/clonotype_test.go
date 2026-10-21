package clonotype

import (
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

func bulkTable(t *testing.T) *rep.Table {
	t.Helper()
	b := rep.NewBuilder(rep.Templates)
	add := func(sample, v, j, nt, aa string, count float64, productive rep.Flag) {
		r := rep.NewRow()
		r.Sample, r.Locus, r.V, r.J, r.Junction, r.JunctionAA, r.Count, r.Productive = sample, "TRB", v, j, nt, aa, count, productive
		if err := b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	add("S1", "TRBV5-1*01", "TRBJ2-7*01", "TGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT", "CASSLAGGPEAFF", 5, rep.True)
	add("S1", "TRBV5-1*02", "TRBJ2-7*01", "TGCGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT", "CASSLAGGPEAFF", 3, rep.True) // another allele, a silent change
	add("S1", "TRBV6-2*01,TRBV6-3*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", 2, rep.True)                             // same junction, a V tie
	add("S2", "TRBV5-1*01", "TRBJ2-7*01", "TGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT", "CASSLAGGPEAFF", 7, rep.True)
	add("S2", "TRBV5-1*01", "TRBJ2-7*01", "TGTGCCAGC", "", 4, rep.False)
	return b.Table()
}

func TestBulkClonotypesByDefinition(t *testing.T) {
	tab := bulkTable(t)
	aa, err := Compute(tab, nil, Definition{Junction: "aa", V: "gene", J: "gene"})
	if err != nil {
		t.Fatal(err)
	}
	// TRBV5-1 (alleles 01 and 02 together) and the tie TRBV6-2|TRBV6-3 apart.
	if len(aa.Clonotypes) != 2 || aa.Excluded[ReasonNotProductive] != 1 {
		t.Fatalf("%+v %v", aa.Clonotypes, aa.Excluded)
	}
	var v51 int
	for i, c := range aa.Clonotypes {
		if c.Chains[0].V == "TRBV5-1" {
			v51 = i
		}
	}
	if c := aa.Clonotypes[v51]; c.Total != 15 || c.Samples != 2 || aa.In(v51, 0) != 8 || aa.In(v51, 1) != 7 || len(aa.MembersOf(v51)) != 3 {
		t.Fatalf("%+v", c)
	}
	if aa.SampleTotals[0] != 10 || aa.SampleTotals[1] != 7 {
		t.Fatalf("totals %v", aa.SampleTotals)
	}
	// By allele, the two alleles are two clonotypes; without V, one; by nucleotides, the silent
	// change too.
	allele, _ := Compute(tab, nil, Definition{Junction: "aa", V: "allele", J: "gene"})
	noV, _ := Compute(tab, nil, Definition{Junction: "aa", V: "none", J: "none"})
	nt, _ := Compute(tab, nil, Definition{Junction: "nt", V: "gene", J: "gene", ProductiveOnly: true})
	if len(allele.Clonotypes) != 3 || len(noV.Clonotypes) != 1 || noV.Clonotypes[0].Total != 17 || len(nt.Clonotypes) != 2 || nt.Excluded[ReasonNoJunction] != 1 {
		t.Fatalf("allele %d, no V %d, nt %d %v", len(allele.Clonotypes), len(noV.Clonotypes), len(nt.Clonotypes), nt.Excluded)
	}
	// IDs: the same for the same definition and key, whatever the definition's name, and different
	// under another definition.
	again, _ := Compute(tab, nil, Definition{Name: "Another name", Junction: "aa", V: "gene", J: "gene"})
	if again.Clonotypes[0].ID != aa.Clonotypes[0].ID || !strings.HasPrefix(aa.Clonotypes[0].ID, "c-") || noV.Clonotypes[0].ID == aa.Clonotypes[0].ID {
		t.Fatal("IDs")
	}
	// A filter's verdict: rows it leaves out are in no clonotype.
	keep := []bool{true, true, true, false, true}
	filtered, _ := Compute(tab, keep, Definition{Junction: "aa"})
	if filtered.Excluded[ReasonFiltered] != 1 || filtered.SampleTotals[1] != 0 {
		t.Fatalf("%+v", filtered.Excluded)
	}
	if _, err := Compute(tab, nil, Definition{Chains: []string{"TRA", "TRB"}}); err != ErrPairedBulk {
		t.Fatalf("paired chains on bulk data: %v", err)
	}
	if _, err := Compute(tab, nil, Definition{Junction: "protein"}); err == nil {
		t.Fatal("a junction kind that is not one")
	}
}

func cellTable(t *testing.T) *rep.Table {
	t.Helper()
	b := rep.NewBuilder(rep.Cells)
	add := func(cell, id, locus, v, j, aa string, umis float64, clone string) {
		r := rep.NewRow()
		r.Sample, r.Cell, r.SequenceID, r.Locus, r.V, r.J, r.JunctionAA, r.Productive, r.UMIs, r.Count, r.Clone = "S", cell, id, locus, v, j, aa, rep.True, umis, umis, clone
		b.Append(&r)
	}
	// a and b share both chains (b has a second, weaker α); c has the same β alone; d has a
	// different α.
	add("a", "a1", "TRA", "TRAV12-1", "TRAJ33", "CAVMDSNYQLIW", 5, "clonotype1")
	add("a", "a2", "TRB", "TRBV5-1", "TRBJ2-7", "CASSLAGGPEAFF", 9, "clonotype1")
	add("b", "b1", "TRA", "TRAV12-1", "TRAJ33", "CAVMDSNYQLIW", 4, "clonotype1")
	add("b", "b2", "TRA", "TRAV8-1", "TRAJ9", "CAVNTGGFKTIF", 1, "clonotype1")
	add("b", "b3", "TRB", "TRBV5-1", "TRBJ2-7", "CASSLAGGPEAFF", 7, "clonotype1")
	add("c", "c1", "TRB", "TRBV5-1", "TRBJ2-7", "CASSLAGGPEAFF", 3, "clonotype2")
	add("d", "d1", "TRA", "TRAV1-2", "TRAJ33", "CAVRDSNYQLIW", 6, "clonotype3")
	add("d", "d2", "TRB", "TRBV5-1", "TRBJ2-7", "CASSLAGGPEAFF", 8, "clonotype3")
	return b.Table()
}

func TestSingleCellClonotypes(t *testing.T) {
	tab := cellTable(t)
	paired := Definition{Chains: []string{"TRA", "TRB"}, Junction: "aa", V: "gene", J: "gene", Cells: "primary"}
	p, err := Compute(tab, nil, paired)
	if err != nil {
		t.Fatal(err)
	}
	// {a, b} by their primary chains; c (β alone); d.
	if len(p.Clonotypes) != 3 || p.Cells != 4 {
		t.Fatalf("%d clonotypes, %d cells", len(p.Clonotypes), p.Cells)
	}
	sizes := map[float64]int{}
	for _, c := range p.Clonotypes {
		sizes[c.Total]++
	}
	if sizes[2] != 1 || sizes[1] != 2 {
		t.Fatalf("sizes %v", sizes)
	}
	// Every chain: b's second α makes it a clonotype of its own.
	all := paired
	all.Cells = "all"
	if a, _ := Compute(tab, nil, all); len(a.Clonotypes) != 4 {
		t.Fatalf("all chains: %d", len(a.Clonotypes))
	}
	// Both chains required: c is left out.
	req := paired
	req.RequireAll = true
	if r, _ := Compute(tab, nil, req); len(r.Clonotypes) != 2 || r.Excluded[ReasonMissingChain] != 1 {
		t.Fatalf("require all: %d %v", len(r.Clonotypes), r.Excluded)
	}
	// β alone: a, b, c and d share it.
	if beta, _ := Compute(tab, nil, Definition{Chains: []string{"TRB"}}); len(beta.Clonotypes) != 1 || beta.Clonotypes[0].Total != 4 {
		t.Fatalf("β: %+v", beta.Clonotypes)
	}
	// Cell Ranger's own clonotypes, as given.
	v, _ := Compute(tab, nil, Definition{Vendor: true})
	if len(v.Clonotypes) != 3 || v.Clonotypes[0].Vendor != "clonotype1" || v.Clonotypes[0].Total != 2 {
		t.Fatalf("vendor: %+v", v.Clonotypes)
	}
	// Each locus separately: every cell once per locus.
	if each, _ := Compute(tab, nil, Definition{}); each.Cells != 4 || len(each.Clonotypes) != 3 {
		t.Fatalf("each locus: %d cells, %d clonotypes", each.Cells, len(each.Clonotypes))
	}
}

func TestDefinitionsSayWhatTheyAre(t *testing.T) {
	d, _ := Definition{Chains: []string{"TRA", "TRB"}}.Normalize()
	if got := d.Describe(); got != "paired TRA and TRB chains by amino-acid junction, V gene and J gene; single cells by their primary chain of each locus (most UMIs), cells with one chain grouped with cells of the same chain alone" {
		t.Fatal(got)
	}
	if d.ID() != (Definition{Name: "x", Chains: []string{"TRA", "TRB"}, Junction: "aa", V: "gene", J: "gene", ProductiveOnly: true, Cells: "primary"}).ID() {
		t.Fatal("a definition's ID depends on its name")
	}
	for _, p := range Presets {
		if _, err := p.Normalize(); err != nil {
			t.Errorf("%s: %v", p.Name, err)
		}
	}
	if _, err := (Definition{Chains: []string{"TRB", "TRB"}}).Normalize(); err == nil {
		t.Error("a chain named twice")
	}
}

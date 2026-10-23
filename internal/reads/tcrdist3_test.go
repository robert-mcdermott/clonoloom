package reads

import (
	"context"
	"math"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

func TestReadsTCRdistCloneTables(t *testing.T) {
	text := "subject,epitope,count,v_a_gene,j_a_gene,cdr3_a_aa,cdr3_a_nucseq,v_b_gene,j_b_gene,cdr3_b_aa,cdr3_b_nucseq,clone_id\n" +
		"mouse_subject0050,PA,2,TRAV7-3*01,TRAJ33*01,CAVSLDSNYQLIW,tgtgcagtgagcctcgatagcaactatcagttgatctgg,TRBV13-1*01,TRBJ2-3*01,CASSDFDWGGDAETLYF,tgtgccagcagtgatttcgactggggaggggatgcagaaacgctgtatttt,tcr1\n" +
		"mouse_subject0050,PA,1,TRAV6D-6*01,TRAJ56*01,CALGDRATGGNNKLTF,,TRBV29*01,TRBJ1-1*01,CASSPDRGEVFF,,tcr2\n" +
		"mouse_subject0046,NP,1.5,TRAV4*01,TRAJ2*01,CA*F,,TRBV1*01,TRBJ1-1*01,CTSSQ,,tcr3\n"
	res, err := ReadReader(context.Background(), strings.NewReader(text), "dash.csv")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	// tcr1's two cells, tcr2's one and tcr3's (a count that is not whole: one), two chains each.
	if res.Sources[0].Format != FormatTCRdist || res.Unit != rep.Cells || tab.Len() != 8 || res.Problems.Count("cell-count") != 1 {
		t.Fatalf("%+v unit %q rows %d %+v", res.Sources, res.Unit, tab.Len(), res.Problems.Summaries())
	}
	if len(tab.Samples) != 2 || tab.Samples[0].Name != "mouse_subject0050_PA" || tab.Samples[1].Metadata["epitope"] != "NP" || tab.Samples[1].Metadata["subject"] != "mouse_subject0046" {
		t.Fatalf("samples %+v", tab.Samples)
	}
	cell := func(i int) string { return tab.Dict.Get(tab.Cell[i]) }
	if cell(0) != "tcr1#1" || cell(1) != "tcr1#1" || cell(2) != "tcr1#2" || cell(4) != "tcr2" || tab.Dict.Get(tab.Clone[2]) != "tcr1" {
		t.Fatalf("cells %q %q %q %q", cell(0), cell(1), cell(2), cell(4))
	}
	if tab.LocusName(0) != "TRA" || tab.LocusName(1) != "TRB" || tab.IDs.Get(tab.SequenceID[1]) != "tcr1#1_b" {
		t.Fatal("loci and ids")
	}
	if tab.Junctions.Get(tab.Junction[0]) != "TGTGCAGTGAGCCTCGATAGCAACTATCAGTTGATCTGG" || tab.Productive[0] != rep.True || tab.Productive[6] != rep.False {
		t.Fatal("junctions and productivity")
	}
	// One chain: a bulk repertoire, its clones counted by their count, in a unit to confirm.
	beta := "cdr3_b_aa,v_b_gene,j_b_gene,count\nCASSLAPGATNEKLFF,TRBV7-9*01,TRBJ1-4*01,14\nCASSQDRGNTEAFF,TRBV4-1*01,TRBJ1-1*01,3\n"
	res, err = ReadReader(context.Background(), strings.NewReader(beta), "bulk.csv")
	if err != nil {
		t.Fatal(err)
	}
	if res.Unit != rep.UnitUnknown || res.Table.Cell != nil || res.Table.Count[0] != 14 || res.Table.Samples[0].Name != "bulk" || res.Table.LocusName(1) != "TRB" {
		t.Fatalf("unit %q counts %v", res.Unit, res.Table.Count)
	}
	// Tab-separated, without counts: each clone counts once.
	tabbed := "v_b_gene\tj_b_gene\tcdr3_b_aa\nTRBV9*01\tTRBJ2-7*01\tCASSVGEQYF\n"
	res, err = ReadReader(context.Background(), strings.NewReader(tabbed), "x.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if res.Sources[0].Format != FormatTCRdist || res.Unit != rep.Sequences || res.Table.Count[0] != 1 || math.IsNaN(res.Table.Count[0]) {
		t.Fatalf("%+v unit %q", res.Sources, res.Unit)
	}
}

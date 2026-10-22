package reads

import (
	"context"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

const mixcr3Header = "cloneId\tcloneCount\tcloneFraction\tallVHitsWithScore\tallDHitsWithScore\tallJHitsWithScore\tallCHitsWithScore\tnSeqCDR3\taaSeqCDR3\trefPoints"

func TestReadsMiXCRCloneTables(t *testing.T) {
	text := mixcr3Header + "\n" +
		"0\t276\t0.17\tTRBV7-3*00(248)\tTRBD1*00(40),TRBD2*00(40)\tTRBJ2-3*00(200.5)\tTRBC2*00(60.4)\tTGTGCCAGCAGCCCCGGGACTAGAGGAACAGATACGCAGTATTTT\tCASSPGTRGTDTQYF\t:::\n" +
		"1\t12.5\t0.01\tTRBV6-5*01(310),TRBV6-6*02(290)\t\tTRBJ1-1*00(180)\t\tTGTGCCAGCAGTtactcc\tCASS_y\t:::\n" +
		"2\t3\t0.002\tTRAV12-1*00(200)\t\tTRAJ33*00(150)\t\tTGTGTGGTGAACTAGGATAGCAACTATCAGTTGATCTGG\tCVVN*DSNYQLIW\t:::\n"
	res, err := ReadReader(context.Background(), strings.NewReader(text), "/runs/S1.clones_TRB.tsv")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if res.Sources[0].Format != FormatMiXCR || tab.Len() != 3 || res.Unit != rep.Reads {
		t.Fatalf("%+v unit %q", res.Sources, res.Unit)
	}
	// The sample is the file's, less the chain MiXCR splits files by.
	if len(tab.Samples) != 1 || tab.Samples[0].Name != "S1" {
		t.Fatalf("samples %+v", tab.Samples)
	}
	call := func(c []uint32, i int) string { return tab.Dict.Get(c[i]) }
	if call(tab.V, 0) != "TRBV7-3" || call(tab.D, 0) != "TRBD1,TRBD2" || call(tab.J, 0) != "TRBJ2-3" || call(tab.C, 0) != "TRBC2" {
		t.Fatalf("calls %s %s %s %s", call(tab.V, 0), call(tab.D, 0), call(tab.J, 0), call(tab.C, 0))
	}
	// The best hit alone, its allele kept when MiXCR gives one.
	if call(tab.V, 1) != "TRBV6-5*01" || call(tab.D, 1) != "" {
		t.Fatalf("calls of the second clone %q %q", call(tab.V, 1), call(tab.D, 1))
	}
	if tab.LocusName(0) != "TRB" || tab.LocusName(2) != "TRA" || tab.IDs.Get(tab.SequenceID[2]) != "2" {
		t.Fatal("loci and ids")
	}
	// In frame; out of frame (no amino acids, imputed lower case read as upper case); a stop.
	if tab.Productive[0] != rep.True || tab.Productive[1] != rep.False || tab.Productive[2] != rep.False {
		t.Fatalf("productive %v", tab.Productive)
	}
	if tab.JunctionAA[1] != 0 || tab.Junctions.Get(tab.Junction[1]) != "TGTGCCAGCAGTTACTCC" || tab.StopCodon[2] != rep.True || tab.JunctionsAA.Get(tab.JunctionAA[2]) != "CVVN*DSNYQLIW" {
		t.Fatal("junctions")
	}
	if tab.Count[1] != 12.5 || tab.Reads[0] != 276 {
		t.Fatalf("counts %v reads %v", tab.Count, tab.Reads)
	}
}

func TestMiXCRMoleculesAndNames(t *testing.T) {
	header := "cloneId\treadCount\treadFraction\tuniqueMoleculeCount\tuniqueMoleculeFraction\tallVHitsWithScore\tallJHitsWithScore\tnSeqCDR3\taaSeqCDR3"
	rows := "7\t248.0\t0.06\t3\t0.04\tIGHV3-21*00(358.5),IGHV3-33*00(356.7)\tIGHJ4*00(262.8)\tTGTGCGAGAAACGACTGG\tCARNDW\n" +
		"9\t40.0\t0.01\t2\t0.03\tIGHV1-2*00(300)\tIGHJ4*00(262.8)\tTGTGCGAGAGGGTGG\tCARGW\n"
	res, err := ReadReader(context.Background(), strings.NewReader(header+"\n"+rows), "Sample1.clones_IGH.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if res.Unit != rep.UMIs || res.Table.Count[0] != 3 || res.Table.Reads[0] != 248 || res.Table.Samples[0].Name != "Sample1" {
		t.Fatalf("unit %q counts %v", res.Unit, res.Table.Count)
	}
	// A clone without molecules: the file is counted in reads.
	some := strings.Replace(rows, "\t2\t0.03", "\t\t", 1)
	res, err = ReadReader(context.Background(), strings.NewReader(header+"\n"+some), "x.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if res.Unit != rep.Reads || res.Table.Count[0] != 248 || res.Table.Count[1] != 40 || res.Problems.Count("molecules-missing") != 1 {
		t.Fatalf("unit %q counts %v", res.Unit, res.Table.Count)
	}
	// MiXCR 2's readable names.
	v2 := "Clone ID\tClone count\tClone fraction\tAll V hits\tAll J hits\tN. Seq. CDR3\tAA. Seq. CDR3\n0\t5\t0.5\tTRBV9*00(100)\tTRBJ2-7*00(90)\tTGTGCCAGCAGCGTAGGGGAGCAGTACTTC\tCASSVGEQYF\n"
	res, err = ReadReader(context.Background(), strings.NewReader(v2), "clones.clonotypes.TRB.txt")
	if err != nil {
		t.Fatal(err)
	}
	if res.Sources[0].Format != FormatMiXCR || res.Table.Count[0] != 5 || res.Table.Samples[0].Name != "clones" || res.Table.Dict.Get(res.Table.V[0]) != "TRBV9" {
		t.Fatalf("%+v", res.Sources)
	}
	// A single-cell export: each row a clone's chain in a cell.
	sc := "cloneId\tcellGroup\ttagValueCELL\treadCount\tuniqueMoleculeCount\tallVHitsWithScore\tallJHitsWithScore\tnSeqCDR3\taaSeqCDR3\ttopChains\n" +
		"0\t430\tGAGTCC\t58161.0\t17924\tIGLV2-14*00(1200)\tIGLJ3*00(200)\tTGCTGCTCGTATGCAGGTGGTCGCCCAAGGGTGTTC\tCCSYAGGRPRVF\tIGL\n" +
		"4\t430\tGAGTCC\t900.0\t310\tIGHV3-23*00(1300)\tIGHJ4*00(200)\tTGTGCGAAAGATTGG\tCAKDW\tIGH\n"
	res, err = ReadReader(context.Background(), strings.NewReader(sc), "pbmc_b.tsv")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if res.Unit != rep.Cells || tab.Dict.Get(tab.Cell[1]) != "GAGTCC" || tab.Dict.Get(tab.Clone[0]) != "430" || tab.IDs.Get(tab.SequenceID[1]) != "GAGTCC_clone_4" || tab.LocusName(0) != "IGL" || tab.Count[0] != 17924 || tab.IsCell[0] != rep.True {
		t.Fatalf("single cell: unit %q %+v", res.Unit, res.Sources)
	}
	for in, want := range map[string]string{
		"TRBV7-3*00(248),TRBV7-2*00(200)": "TRBV7-3", "TRBV7-3*00": "TRBV7-3", "TRBV12-3": "TRBV12-3",
		"A*00(5),B*01(5),C(4)": "A,B*01", "": "", "X(oops)": "X(oops)",
	} {
		if got := mixcrCall(in); got != want {
			t.Errorf("mixcrCall(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"S1.clones_TRB": "S1", "a.clonotypes.ALL": "a", "S1.clones_X": "S1.clones_X", "plain": "plain"} {
		if got := mixcrSample(in); got != want {
			t.Errorf("mixcrSample(%q) = %q", in, got)
		}
	}
}

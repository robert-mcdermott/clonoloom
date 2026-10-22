package reads

import (
	"context"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

const otsHeader = "sequence_id_beta,locus_beta,productive_beta,v_call_beta,d_call_beta,j_call_beta,junction_beta,junction_aa_beta,Redundancy_beta,sequence_id_alpha,locus_alpha,productive_alpha,v_call_alpha,d_call_alpha,j_call_alpha,junction_alpha,junction_aa_alpha"

func TestReadsPairedDataUnits(t *testing.T) {
	text := `"{""Run"": ""SRR1"", ""Link"": ""https://example.org/​paper"", ""Subject"": ""no"", ""TSource"": ""pbmc"", ""Age"": 37, ""Sequences"": 2}"` + "\n" +
		otsHeader + "\n" +
		"AAAC-1_contig_1,B,T,TRBV3-1*01,TRBD2*01,TRBJ1-4*01,TGTGCCAGCATGCGAGGGGGGCAGGAAAAACTGTTTTTT,CASMRGGQEKLFF,2.0,AAAC-1_contig_2,A,T,TRAV38-1*03,,TRAJ28*01,TGTGCTTTCCTAG,CAFLDSGAGSYQLTF\n" +
		"GGGT-1_contig_3,B,F,TRBV2*01,,TRBJ2-7*01,,,,odd,A,T,TRAV1-2*01,,TRAJ33*01,,CAVMDSNYQLIW\n"
	res, err := ReadReader(context.Background(), strings.NewReader(text), "SRR1_1_Paired_All.csv.gz")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if res.Sources[0].Format != FormatUnit || res.Unit != rep.Cells || tab.Len() != 4 || res.Sources[0].Rows != 4 {
		t.Fatalf("%+v unit %q rows %d", res.Sources, res.Unit, tab.Len())
	}
	s := tab.Samples[0]
	if s.Name != "SRR1_1_Paired_All" || s.Metadata["Run"] != "SRR1" || s.Metadata["Age"] != "37" || s.Metadata["Link"] != "https://example.org/paper" || s.Metadata["Subject"] != "no" {
		t.Fatalf("sample %+v", s)
	}
	cell := func(i int) string { return tab.Dict.Get(tab.Cell[i]) }
	if cell(0) != "AAAC-1" || cell(1) != "AAAC-1" || cell(2) != "row 4" || cell(3) != "row 4" {
		t.Fatalf("cells %q %q %q", cell(0), cell(1), cell(2))
	}
	if tab.LocusName(0) != "TRB" || tab.LocusName(1) != "TRA" || tab.Productive[2] != rep.False || tab.Count[0] != 1 || tab.IsCell[0] != rep.True {
		t.Fatal("loci, productive, counts")
	}
	if tab.IDs.Get(tab.SequenceID[1]) != "AAAC-1_contig_2" || tab.JunctionsAA.Get(tab.JunctionAA[1]) != "CAFLDSGAGSYQLTF" {
		t.Fatal("ids and junctions")
	}
}

func TestReadsUnpairedDataUnits(t *testing.T) {
	text := `"{""Run"": ""ERR1"", ""Chain"": ""Heavy"", ""Subject"": ""Subject-Healthy-10""}"` + "\n" +
		"sequence,locus,stop_codon,vj_in_frame,productive,v_call,d_call,j_call,junction,junction_aa,ANARCI_numbering,Redundancy\n" +
		`ACGT,H,F,T,T,IGHV3-7*01,,IGHJ2*01,TGTGCGAGCCTCCAAGTCGACTGGTACTTCGATCTCTGG,CASLQVDWYFDLW,"{'fwh1': {'1 ': 'E'}, 'cdrh1': {}}",2` + "\n" +
		"ACGT,K,F,T,T,IGKV1-5*03,,IGKJ1*01,TGTCAACAGTATAATAGTTACTGGACGTTC,CQQYNSYWTF,{},1.0\n"
	res, err := ReadReader(context.Background(), strings.NewReader(text), "ERR1_Heavy_IGHG.csv")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if res.Unit != rep.UnitUnknown || tab.Len() != 2 || tab.Cell != nil || tab.Count[0] != 2 || tab.Count[1] != 1 {
		t.Fatalf("unit %q rows %d counts %v", res.Unit, tab.Len(), tab.Count)
	}
	if tab.LocusName(0) != "IGH" || tab.LocusName(1) != "IGK" || tab.Samples[0].Metadata["Subject"] != "Subject-Healthy-10" {
		t.Fatal("loci and metadata")
	}
	// A first line that is not JSON: reported, and the unit read without it.
	broken := strings.Replace(text, `"{""Run""`, `"{Run""`, 1)
	res, err = ReadReader(context.Background(), strings.NewReader(broken), "x.csv")
	if err != nil {
		t.Fatal(err)
	}
	if res.Problems.Count("unit-metadata") != 1 || res.Table.Len() != 2 || len(res.Table.Samples[0].Metadata) != 0 {
		t.Fatalf("problems %+v", res.Problems.Summaries())
	}
	if _, err := ReadReader(context.Background(), strings.NewReader(`"{""Run"": 1}"`+"\n"), "x.csv"); err == nil {
		t.Fatal("a unit without a header was read")
	}
	if _, err := ReadReader(context.Background(), strings.NewReader(`"{""Run"": 1}"`+"\na,b\n1,2\n"), "x.csv"); err != ErrUnknownFormat {
		t.Fatalf("a unit of other columns: %v", err)
	}
}

func TestUnitBarcodes(t *testing.T) {
	for _, c := range []struct {
		ids  []string
		want string
	}{
		{[]string{"AAAC-1_contig_1", "AAAC-1_contig_2"}, "AAAC-1"},
		{[]string{"AAAC-1_contig_1", "GGGT-1_contig_2"}, "row 9"},
		{[]string{"seq1", "seq2"}, "row 9"},
		{nil, "row 9"},
	} {
		if got := unitBarcode(c.ids, 9); got != c.want {
			t.Errorf("unitBarcode(%v) = %q, want %q", c.ids, got, c.want)
		}
	}
}

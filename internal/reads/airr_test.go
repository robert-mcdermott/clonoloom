package reads

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

const airrHeader = "sequence_id\tsequence\trev_comp\tproductive\tv_call\td_call\tj_call\tsequence_alignment\tgermline_alignment\tjunction\tjunction_aa\tv_cigar\td_cigar\tj_cigar\tlocus\tduplicate_count\tumi_count\trepertoire_id\tv_identity"

func airrRow(id, productive, v, j, junction, aa, dup, umi, rep string) string {
	return strings.Join([]string{id, "", "F", productive, v, "", j, "", "", junction, aa, "", "", "", "TRB", dup, umi, rep, "0.98"}, "\t")
}

func TestReadsAnAIRRFileIntoSamples(t *testing.T) {
	text := "\xef\xbb\xbf" + airrHeader + "\r\n" +
		airrRow("s1", "T", "TRBV5-1*01", "TRBJ2-7*01", "TGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT", "CASSLAGGPEAFF", "10", "3", "R1") + "\r\n" +
		airrRow("s2", "F", "TRBV6-2*01,TRBV6-3*01", "TRBJ1-1*01", "", "", "2", "", "R2") + "\r\n" +
		airrRow("s3", "true", "TRBV5-1*01", "TRBJ2-7*01", "TGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT", "CASSLAGGPEAFF", "5", "1", "R1") + "\r\n"
	res, err := ReadReader(context.Background(), strings.NewReader(text), "/data/study.tsv")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if res.Sources[0].Format != FormatAIRR || res.Sources[0].Rows != 3 || tab.Len() != 3 {
		t.Fatalf("%+v", res.Sources)
	}
	if len(tab.Samples) != 2 || tab.Samples[0].Name != "R1" || tab.Samples[1].Name != "R2" || tab.Sample[2] != 0 {
		t.Fatalf("samples %+v %v", tab.Samples, tab.Sample)
	}
	// Row s2 has no umi_count but a duplicate_count: counted by it, so the file counts by two
	// columns and its unit is left unknown.
	if res.Unit != rep.UnitUnknown || tab.Count[0] != 3 || tab.Count[1] != 2 || tab.Reads[0] != 10 || res.Problems.Count("mixed-units") != 1 {
		t.Fatalf("unit %q counts %v reads %v", res.Unit, tab.Count, tab.Reads)
	}
	// Every row with a UMI count: the file counts UMIs.
	uniform := strings.Replace(text, "\t2\t\tR2", "\t2\t4\tR2", 1)
	if res, _ := ReadReader(context.Background(), strings.NewReader(uniform), "x.tsv"); res.Unit != rep.UMIs || res.Table.Count[1] != 4 {
		t.Fatalf("uniform: unit %q counts %v", res.Unit, res.Table.Count)
	}
	if tab.Productive[0] != rep.True || tab.Productive[1] != rep.False || tab.Productive[2] != rep.True {
		t.Fatalf("productive %v", tab.Productive)
	}
	if tab.JunctionsAA.Get(tab.JunctionAA[0]) != "CASSLAGGPEAFF" || tab.JunctionAA[0] != tab.JunctionAA[2] || tab.JunctionAA[1] != 0 {
		t.Fatal("junctions")
	}
	if tab.IDs.Get(tab.SequenceID[1]) != "s2" || tab.Line[0] != 2 || tab.LocusName(0) != "TRB" {
		t.Fatal("ids, lines, loci")
	}
	if got := tab.Call(tab.V[1]).Key(2); got != "TRBV6-2|TRBV6-3" {
		t.Fatalf("ambiguous V %q", got)
	}
	if n := res.Problems.Count("not-boolean"); n != 0 {
		t.Fatalf("%d boolean problems", n)
	}
}

func TestAIRRProblemsAreCountedWithTheirPlace(t *testing.T) {
	header := "sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tduplicate_count"
	text := header + "\n" +
		"a\tTRBV5-1\tTRBJ2-7\tCASSF\tyes\t3\n" +
		"b\tTRBV5-1\tTRBJ2-7\tCASSF\tT\t2.0\n" +
		"c\tTRBV5-1\tTRBJ2-7\tCASSF\tT\tmany\n" +
		"d\tTRBV5-1\tTRBJ2-7\n"
	res, err := ReadReader(context.Background(), strings.NewReader(text), "x.tsv")
	if err != nil {
		t.Fatal(err)
	}
	p := res.Problems
	if p.Count("not-boolean") != 1 || p.Count("integer-as-decimal") != 1 || p.Count("not-integer") != 1 || p.Count("field-count") != 1 || p.Count("airr-missing-required") != 1 {
		t.Fatalf("%+v", p.Summaries())
	}
	ex := p.examples["not-integer"][0]
	if ex.Line != 4 || ex.Column != "duplicate_count" || ex.Value != "many" {
		t.Fatalf("%+v", ex)
	}
	// A row with too few fields is not read; the unit of duplicate_count alone is left open.
	if res.Table.Len() != 3 || res.Unit != rep.UnitUnknown || res.Table.Samples[0].Name != "x" {
		t.Fatalf("%d rows, unit %q", res.Table.Len(), res.Unit)
	}
}

func TestGzippedAIRR(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte(airrHeader + "\n" + airrRow("s1", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "1", "1", "") + "\n"))
	gz.Close()
	res, err := ReadReader(context.Background(), &buf, "Subject07_day28.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sources[0].Gzipped || res.Table.Len() != 1 || res.Table.Samples[0].Name != "Subject07_day28" {
		t.Fatalf("%+v %+v", res.Sources, res.Table.Samples)
	}
}

func TestUnknownAndEmptyFilesAreRefused(t *testing.T) {
	if _, err := ReadReader(context.Background(), strings.NewReader("a\tb\n1\t2\n"), "x.tsv"); err != ErrUnknownFormat {
		t.Fatalf("got %v", err)
	}
	if _, err := ReadReader(context.Background(), strings.NewReader(""), "x.tsv"); err == nil {
		t.Fatal("an empty file was read")
	}
	long := strings.Repeat("x", maxLine+10)
	if _, err := ReadReader(context.Background(), strings.NewReader(long), "x.tsv"); err != ErrLineTooLong {
		t.Fatalf("got %v", err)
	}
}

func TestSplitCSV(t *testing.T) {
	cases := map[string][]string{
		`a,b,c`:              {"a", "b", "c"},
		`a,"b,c",d`:          {"a", "b,c", "d"},
		`"say ""hi""",,x`:    {`say "hi"`, "", "x"},
		`a,`:                 {"a", ""},
		``:                   {""},
		`AAAC-1,True,"None"`: {"AAAC-1", "True", "None"},
	}
	for line, want := range cases {
		got, ok := splitCSV([]byte(line), nil)
		if !ok || strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%q: %q %v", line, got, ok)
		}
	}
	if _, ok := splitCSV([]byte(`a,"b`), nil); ok {
		t.Error("unbalanced quotes accepted")
	}
	if _, ok := splitCSV([]byte(`a,"b"c`), nil); ok {
		t.Error("text after a closing quote accepted")
	}
}

func TestSampleNames(t *testing.T) {
	cases := map[string]string{
		"/d/Subject07_day28.tsv.gz":                                                  "Subject07_day28",
		"/d/sc5p_v2_hs_PBMC_10k_t_filtered_contig_annotations.csv":                   "sc5p_v2_hs_PBMC_10k_t",
		"/runs/PBMC_10k/outs/filtered_contig_annotations.csv":                        "PBMC_10k",
		"/runs/m1/outs/per_sample_outs/Donor3/vdj_b/filtered_contig_annotations.csv": "Donor3",
		"filtered_contig_annotations.csv":                                            "filtered_contig_annotations",
		"/d/study.airr.tsv":                                                          "study",
	}
	for path, want := range cases {
		if got := sampleName(path); got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}

func TestReadsCellRangerContigs(t *testing.T) {
	text := "barcode,is_cell,contig_id,high_confidence,length,chain,v_gene,d_gene,j_gene,c_gene,full_length,productive,cdr3,cdr3_nt,reads,umis,raw_clonotype_id,raw_consensus_id\n" +
		"AAACCTGAGATCCGAG-1,True,AAACCTGAGATCCGAG-1_contig_1,True,556,TRB,TRBV5-1,None,TRBJ2-7,TRBC2,True,True,CASSLAGGPEAFF,TGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT,9318,14,clonotype1,clonotype1_consensus_1\n" +
		"AAACCTGAGATCCGAG-1,True,AAACCTGAGATCCGAG-1_contig_2,True,512,TRA,TRAV12-1,None,TRAJ33,TRAC,True,None,None,None,1200,3,clonotype1,None\n"
	res, err := ReadReader(context.Background(), strings.NewReader(text), "/runs/PBMC/outs/filtered_contig_annotations.csv")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if res.Sources[0].Format != FormatCellRanger || res.Unit != rep.Cells || tab.Len() != 2 || tab.Samples[0].Name != "PBMC" {
		t.Fatalf("%+v %q", res.Sources, res.Unit)
	}
	if tab.Dict.Get(tab.Cell[0]) != "AAACCTGAGATCCGAG-1" || tab.Cell[0] != tab.Cell[1] || tab.LocusName(1) != "TRA" {
		t.Fatal("cells and chains")
	}
	if tab.Productive[0] != rep.True || tab.Productive[1] != rep.Missing || tab.JunctionAA[1] != 0 || tab.D[0] != 0 {
		t.Fatal("None is missing")
	}
	if tab.UMIs[0] != 14 || tab.Reads[0] != 9318 || tab.Count[0] != 14 || tab.Dict.Get(tab.Clone[0]) != "clonotype1" {
		t.Fatal("counts and clonotypes")
	}
	if tab.IsCell[0] != rep.True || tab.HighConfidence[1] != rep.True || tab.FullLength[0] != rep.True {
		t.Fatal("flags")
	}
}

func TestReadsImmunoSEQv3AndV2(t *testing.T) {
	v3 := "sample_name\tspecies\tlocus\trearrangement\tamino_acid\tframe_type\ttemplates\tseq_reads\tcdr3_length\tv_index\tv_resolved\tv_gene\tv_gene_ties\td_resolved\td_gene\td_gene_ties\tj_resolved\tj_gene\tj_gene_ties\n" +
		"P1\tHuman\tTCRB\tAACGCCTTGTTGCTGGGGGACTCGGCCCTGTATCTCTGTGCCAGCAGCTTGGCCTGGAGGTTTCACGGGGAGCTGTTTTTTGGAGAA\tCASSLAWRFHGELFF\tIn\t5\t603\t45\t36\tTCRBV05-05*01\tTCRBV05-05\t\tTCRBD02-01*02\tTCRBD02-01\t\tTCRBJ02-02*01\tTCRBJ02-02\t\n" +
		"P1\tHuman\tTCRB\tAACGCCTTGTTGCTGGGGGACTCGGCCCTGTATCTC\t\tOut\t2\t209\t10\t3\tTCRBV20\t\tTCRBV20-01,TCRBV20-or09_02\tunresolved\t\t\tTCRBJ01-01*01\tTCRBJ01-01\t\n"
	res, err := ReadReader(context.Background(), strings.NewReader(v3), "P1.tsv")
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if res.Sources[0].Format != FormatImmunoSEQv3 || res.Unit != rep.Templates || tab.Count[0] != 5 || tab.Reads[1] != 209 {
		t.Fatalf("%+v %q %v", res.Sources[0].Format, res.Unit, tab.Count)
	}
	if tab.Junctions.Get(tab.Junction[0]) != "TGTGCCAGCAGCTTGGCCTGGAGGTTTCACGGGGAGCTGTTTTTT" || tab.JunctionAA[1] != 0 {
		t.Fatalf("junction %q", tab.Junctions.Get(tab.Junction[0]))
	}
	if tab.Dict.Get(tab.V[0]) != "TRBV5-5*01" || tab.Dict.Get(tab.D[0]) != "TRBD2*02" || tab.Dict.Get(tab.V[1]) != "TRBV20-1,TRBV20/OR9-2" || tab.D[1] != 0 {
		t.Fatalf("genes %q %q %q", tab.Dict.Get(tab.V[0]), tab.Dict.Get(tab.D[0]), tab.Dict.Get(tab.V[1]))
	}
	if tab.Productive[0] != rep.True || tab.Productive[1] != rep.False || tab.VJInFrame[1] != rep.False || tab.LocusName(0) != "TRB" {
		t.Fatal("frame and locus")
	}
	if res.Sources[0].GeneNames["TCRBV20-01,TCRBV20-or09_02"] != "TRBV20-1,TRBV20/OR9-2" {
		t.Fatalf("%v", res.Sources[0].GeneNames)
	}
	// Without templates for every row, the file is counted in reads.
	noTemplates := strings.Replace(v3, "\t5\t603\t", "\tnull\t603\t", 1)
	res, _ = ReadReader(context.Background(), strings.NewReader(noTemplates), "P1.tsv")
	if res.Unit != rep.Reads || res.Table.Count[0] != 603 || res.Table.Count[1] != 209 || res.Problems.Count("templates-missing") != 1 {
		t.Fatalf("%q %v", res.Unit, res.Table.Count)
	}
	v2 := "nucleotide\taminoAcid\tcount (templates/reads)\tfrequencyCount (%)\tcdr3Length\tvMaxResolved\tvGeneName\tvGeneNameTies\tjMaxResolved\tjGeneName\tjGeneNameTies\tvIndex\testimatedNumberGenomes\tsequenceStatus\n" +
		"CTGAATGTGAACGCCTTGTTGCTGGGGGACTCGGCCCTGTATCTCTGTGCCAGCAGCTTCGACAGCTACGAGCAGTACTTCGGGCCG\tCASSFDSYEQYF\t4\t0.5\t36\tTCRBV09-01*01\tTCRBV09-01\t\tTCRBJ02-07*01\tTCRBJ02-07\t\t45\t4\tIn\n"
	res, err = ReadReader(context.Background(), strings.NewReader(v2), "HIP0001.tsv")
	if err != nil {
		t.Fatal(err)
	}
	tab = res.Table
	if res.Sources[0].Format != FormatImmunoSEQv2 || res.Unit != rep.Templates || tab.Count[0] != 4 || tab.Dict.Get(tab.V[0]) != "TRBV9*01" || tab.Samples[0].Name != "HIP0001" {
		t.Fatalf("%v %q %v %q", res.Sources[0].Format, res.Unit, tab.Count, tab.Dict.Get(tab.V[0]))
	}
	if tab.Junctions.Get(tab.Junction[0]) != "TGTGCCAGCAGCTTCGACAGCTACGAGCAGTACTTC" {
		t.Fatal("v2 junction")
	}
}

// Every code the readers report is listed in ProblemCodes.
func TestProblemCodesAreListed(t *testing.T) {
	listed := map[string]bool{}
	for _, c := range ProblemCodes {
		listed[c] = true
	}
	files, _ := filepath.Glob("*.go")
	pattern := regexp.MustCompile(`addf\("([a-z-]+)"`)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range pattern.FindAllStringSubmatch(string(src), -1) {
			if !listed[m[1]] {
				t.Errorf("%s reports %s, which ProblemCodes does not list", f, m[1])
			}
		}
	}
}

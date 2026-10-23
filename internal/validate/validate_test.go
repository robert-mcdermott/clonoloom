package validate

import (
	"context"
	"strings"
	"testing"

	"clonoloom/internal/reads"
)

func check(t *testing.T, text string) Report {
	t.Helper()
	res, err := reads.ReadReader(context.Background(), strings.NewReader(text), "sample.tsv")
	if err != nil {
		t.Fatal(err)
	}
	return Check(res)
}

func codes(r Report) map[string]int {
	out := map[string]int{}
	for _, f := range r.Findings {
		out[f.Code] = f.Count
	}
	return out
}

const header = "sequence_id\tsequence\trev_comp\tproductive\tv_call\td_call\tj_call\tsequence_alignment\tgermline_alignment\tjunction\tjunction_aa\tv_cigar\td_cigar\tj_cigar\tlocus\tumi_count\tcell_id"

func row(id, productive, v, j, junction, aa, umi, cell string) string {
	return strings.Join([]string{id, "", "F", productive, v, "", j, "", "", junction, aa, "", "", "", "TRB", umi, cell}, "\t")
}

func TestEachPlantedFaultIsFoundOnce(t *testing.T) {
	good := "TGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT"
	text := header + "\n" + strings.Join([]string{
		row("ok1", "T", "TRBV5-1*01", "TRBJ2-7*01", good, "CASSLAGGPEAFF", "3", ""),
		row("tr", "T", "TRBV5-1*01", "TRBJ2-7*01", good, "CASSLAGGPEAWF", "3", ""),        // junction-translation
		row("frame", "T", "TRBV5-1*01", "TRBJ2-7*01", good+"A", "CASSLAGGPEAFF", "3", ""), // productive-out-of-frame
		row("stop", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CAS*LAGGPEAFF", "3", ""),        // productive-with-stop
		row("anch", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "ASSLAGGPEAF", "3", ""),          // junction-anchors
		row("gene", "T", "XYZ1", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "3", ""),              // gene-unreadable
		row("mix", "T", "TRAV12-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "3", ""),        // locus-mismatch
		row("nj", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "", "3", ""),                       // no-junction-row
		row("nc", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "", ""),           // count-missing
		row("zero", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "0", ""),        // count-zero
		row("ok1", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "3", ""),         // duplicate-sequence-id
		row("nt", "T", "TRBV5-1*01", "TRBJ2-7*01", "TGTGCXAGC", "", "3", ""),              // junction-not-nucleotides
		row("aa", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CASS1F", "3", ""),                 // junction-aa-not-protein
	}, "\n") + "\n"
	report := check(t, text)
	got := codes(report)
	// TRDV1 joined to a TRAJ gene in an α chain is biology, not a mismatch.
	hybrid := check(t, header+"\n"+strings.Replace(row("ad", "T", "TRDV1*01", "TRAJ11*01", "", "CALGELGTTNKLIF", "3", ""), "TRB", "TRA", 1)+"\n")
	if codes(hybrid)["locus-mismatch"] != 0 {
		t.Error("a TRDV–TRAJ rearrangement was called a mismatch")
	}
	for _, code := range []string{"junction-translation", "productive-out-of-frame", "productive-with-stop", "junction-anchors", "gene-unreadable", "locus-mismatch", "no-junction-row", "count-missing", "count-zero", "duplicate-sequence-id", "junction-not-nucleotides", "junction-aa-not-protein"} {
		if got[code] != 1 {
			t.Errorf("%s found %d times, want once (all: %v)", code, got[code], got)
			for _, f := range report.Findings {
				if f.Code == code {
					t.Logf("%+v", f.Examples)
				}
			}
		}
	}
	for code := range got {
		switch code {
		case "airr-missing-required":
		case "junction-translation", "productive-out-of-frame", "productive-with-stop", "junction-anchors", "gene-unreadable", "locus-mismatch", "no-junction-row", "count-missing", "count-zero", "duplicate-sequence-id", "junction-not-nucleotides", "junction-aa-not-protein":
		default:
			t.Errorf("unexpected finding %s", code)
		}
	}
}

func TestCellsWithTooManyChainsAndNonProductiveSamples(t *testing.T) {
	text := header + "\n" + strings.Join([]string{
		row("c1_1", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "3", "AAAC-1"),
		row("c1_2", "T", "TRBV6-2*01", "TRBJ2-7*01", "", "CASSPGQGAYEQYF", "3", "AAAC-1"),
		row("c1_3", "T", "TRBV7-2*01", "TRBJ2-7*01", "", "CASSLGQAYEQYF", "3", "AAAC-1"),
		row("c2_1", "T", "TRBV5-1*01", "TRBJ2-7*01", "", "CASSLAGGPEAFF", "3", "GGGT-1"),
	}, "\n") + "\n"
	r := check(t, text)
	if got := codes(r); got["cell-many-chains"] != 1 {
		t.Fatalf("%v", got)
	}
	bulk := "sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\n" +
		"a\tTRBV5-1\tTRBJ2-7\tCASSLAGGPEAFF\tT\t40\n" +
		"b\tTRBV5-1\tTRBJ2-7\tCASSLAGGPE\tF\t60\n"
	r = check(t, bulk)
	got := codes(r)
	if got["nonproductive-share"] != 1 || !r.Usable() {
		t.Fatalf("%v", got)
	}
	empty := check(t, "sequence_id\tjunction_aa\tv_call\n")
	if empty.Usable() || codes(empty)["empty"] != 1 {
		t.Fatal("an empty file must be fatal")
	}
	unknown := check(t, "sequence_id\tjunction_aa\tv_call\tduplicate_count\na\tCASSF\tTRBV5-1\t3\n")
	if codes(unknown)["unit-unknown"] != 1 {
		t.Fatalf("%v", codes(unknown))
	}
}

// Every problem a reader reports is a finding with a title and a remedy of its own.
func TestEveryReaderProblemHasAKind(t *testing.T) {
	for _, code := range reads.ProblemCodes {
		k, ok := readerKinds[code]
		if !ok || k.title == "" || k.remedy == "" {
			t.Errorf("%s has no title or remedy", code)
		}
	}
}

func TestEvidenceFindingsPreserveUsableJunctions(t *testing.T) {
	res, err := reads.ReadFiles(context.Background(), []string{"../../testdata/bcell-inputs.tsv"})
	if err != nil {
		t.Fatal(err)
	}
	report := Check(res)
	got := codes(report)
	for code, want := range map[string]int{"evidence-alignment": 1, "evidence-coordinate": 2, "evidence-cigar": 1} {
		if got[code] != want {
			t.Errorf("%s: %d, want %d", code, got[code], want)
		}
	}
	if !report.Usable() || res.Table.Len() != 7 {
		t.Fatal(report)
	}
	for _, finding := range report.Findings {
		if strings.HasPrefix(finding.Code, "evidence-") {
			if finding.Remedy == "" || len(finding.Examples) == 0 || finding.Examples[0].Line == 0 || finding.Examples[0].SequenceID == "" {
				t.Fatal(finding)
			}
		}
	}
}

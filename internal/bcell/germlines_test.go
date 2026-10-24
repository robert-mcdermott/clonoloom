package bcell

import (
	"clonoloom/internal/rep"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func germlineEvidence(values map[string]string) [rep.EvidenceCount]string {
	var e [rep.EvidenceCount]string
	for k, v := range values {
		e[fields[k]] = v
	}
	return e
}
func TestGermlineProjection(t *testing.T) {
	// V: first two query bases are clipped. Reference starts at base 2;
	// 2M, one query insertion, 1M, one reference deletion, 1M. J starts at q10.
	raw := germlineEvidence(map[string]string{"sequence": "TTACGTACCGG", "rev_comp": "F", "v_cigar": "2S1N2M1I1M1D1M4S", "j_cigar": "9S2M", "v_sequence_start": "3", "v_sequence_end": "7", "v_germline_start": "2", "v_germline_end": "6"})
	var r GermlineRow
	if why := projectGermline(raw, "AACGTA", "GG", &r); why != "" {
		t.Fatal(why)
	}
	if r.Query != "ACGT-ACCGG" {
		t.Fatal(r.Query)
	}
	if r.Germline != "AC-GTANNGG" || r.Regions != "VVVVVVNNJJ" || !reflect.DeepEqual(r.QueryPositions, []int{3, 4, 5, 6, 0, 7, 8, 9, 10, 11}) || !reflect.DeepEqual(r.ReferencePositions, []int{2, 3, 0, 4, 5, 6, 0, 0, 1, 2}) {
		t.Fatalf("%+v", r)
	}
	// Reverse complement the raw input; output remains in oriented query coordinates.
	raw[fields["sequence"]] = "CCGGTACGTAA"
	raw[fields["rev_comp"]] = "T"
	var reverse GermlineRow
	if why := projectGermline(raw, "AACGTA", "GG", &reverse); why != "" || !reflect.DeepEqual(r, reverse) {
		t.Fatalf("%s %+v", why, reverse)
	}
	// Supplied germline_alignment is deliberately false and never used as the reference.
	raw[fields["germline_alignment"]] = "AAAAAAAAAAA"
	if why := projectGermline(raw, "AACGTA", "GG", &reverse); why != "" || reverse.Germline != r.Germline {
		t.Fatal(why, reverse)
	}
}
func TestGermlineProjectionRefusals(t *testing.T) {
	base := germlineEvidence(map[string]string{"sequence": "AAACCCGGG", "rev_comp": "F", "v_cigar": "3M6S", "j_cigar": "6S3M"})
	for _, tc := range []struct{ field, value, want string }{
		{"sequence", "", "ungapped"}, {"sequence", "A.ACCC", "ungapped"}, {"rev_comp", "", "orientation"}, {"rev_comp", "perhaps", "orientation"},
		{"v_cigar", "", "missing"}, {"v_cigar", "9999999999999M", "invalid"}, {"v_cigar", "1M1N1M", "invalid"}, {"v_cigar", "100M", "exceeds"},
		{"v_germline_start", "0", "coordinates"}, {"v_sequence_end", "2", "coordinates"}, {"v_germline_end", "99999999999999999999999", "coordinates"},
		{"j_cigar", "2S3M", "overlap"}, {"v_cigar", "3X6S", "assertion"},
	} {
		t.Run(tc.field+tc.value, func(t *testing.T) {
			raw := base
			raw[fields[tc.field]] = tc.value
			var r GermlineRow
			why := projectGermline(raw, "AAA", "GGG", &r)
			if !strings.Contains(why, tc.want) || r.Germline != "" {
				t.Fatal(why, r)
			}
		})
	}
	var r GermlineRow
	base[fields["v_cigar"]] = "3=6S"
	base[fields["sequence"]] = "ANACCCGGG"
	if why := projectGermline(base, "ARA", "GGG", &r); why != "" || r.Germline != "ARANNNGGG" {
		t.Fatal(why, r)
	}
}
func TestGermlineFamilyAllelesAndIdentity(t *testing.T) {
	a := familyRow("a", "a", "", "IGH", "AAAAAAAAAAAA")
	a.Evidence = germlineEvidence(map[string]string{"sequence": "AAACCCGGG", "rev_comp": "F", "v_cigar": "3M6S", "j_cigar": "6S3M"})
	b := a
	b.SequenceID = "b"
	b.V = "IGHV1-1*01,IGHV1-1*02"
	src := []FamilySource{{Name: "test.tsv", SHA256: "sha"}}
	ref := GermlineReference{"IGHV1-1*01": "AAA", "IGHJ1*01": "GGG"}
	run := func(rows []rep.Row, ref GermlineReference) GermlineResult {
		tab := familyTable(t, rows)
		f, e := Families(context.Background(), tab, nil, src, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
		if e != nil {
			t.Fatal(e)
		}
		r, e := Germlines(context.Background(), tab, src, f, ref)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	r := run([]rep.Row{a, b}, ref)
	if r.Reconstructed != 2 || len(r.Families) != 1 || r.Rows[0].Germline != "AAANNNGGG" {
		t.Fatal(r)
	}
	// Neither a high-abundance member nor a sequence match overrides conflicting calls.
	b.V = "IGHV1-1*02"
	b.Count = 1000000
	if r = run([]rep.Row{a, b}, ref); r.Reconstructed != 0 || r.Excluded["conflicting family allele calls"] != 2 {
		t.Fatal(r)
	}
	a.V = "IGHV1-1*01,IGHV1-1*02"
	b.V = a.V
	if r = run([]rep.Row{a, b}, ref); r.Excluded["unresolved family allele ambiguity"] != 2 {
		t.Fatal(r)
	}
	a.V = "IGHV1-1*03"
	if r = run([]rep.Row{a}, ref); r.Excluded["allele absent from selected reference"] != 1 {
		t.Fatal(r)
	}
	a.V = "IGHV1-1"
	if r = run([]rep.Row{a}, ref); r.Reconstructed != 0 {
		t.Fatal(r)
	}
	a.V = "IGHV1-1*01"
	tab := familyTable(t, []rep.Row{a})
	f, _ := Families(context.Background(), tab, nil, src, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Germlines(ctx, tab, src, f, ref); e == nil {
		t.Fatal("cancelled")
	}
	if _, e := Germlines(context.Background(), tab, []FamilySource{{Name: "changed", SHA256: "sha"}}, f, ref); e == nil {
		t.Fatal("changed source accepted")
	}
	f.Options.Threshold = nil
	if _, e := Germlines(context.Background(), tab, src, f, ref); e == nil {
		t.Fatal("diagnostic accepted")
	}
}
func TestGermlineFASTA(t *testing.T) {
	r, e := ReadGermlineFASTA(strings.NewReader(">IGHV1-1*01\naacg\nTT\n>IGHJ1*01\nNNR\n"))
	if e != nil || r["IGHV1-1*01"] != "AACGTT" {
		t.Fatal(r, e)
	}
	for _, raw := range []string{"", "AAA", ">IGHV1-1*01\n", ">IGHV1-1\nAAA", ">IGHV1-1*01\nAA.A", ">IGHV1-1*01\nAAA\n>IGHV1-1*01\nAAA", ">IGHV1-1*01 annotation\nAAA", ">IGHV1-1*01\n" + strings.Repeat("A", 10001)} {
		if _, e := ReadGermlineFASTA(strings.NewReader(raw)); e == nil {
			t.Fatal("accepted invalid FASTA")
		}
	}
}
func TestGermlineReferenceByteBoundIncludesWhitespace(t *testing.T) {
	_, err := ReadGermlineFASTA(strings.NewReader(">IGHV1-1*01\nAAA\n" + strings.Repeat("\n", 10<<20)))
	if err == nil || !strings.Contains(err.Error(), "10 MiB") {
		t.Fatal(err)
	}
}
func FuzzGermlineProjection(f *testing.F) {
	f.Add("3M6S", "6S3M", "AAACCCGGG")
	f.Fuzz(func(t *testing.T, v, j, query string) {
		var r GermlineRow
		why := projectGermline(germlineEvidence(map[string]string{"sequence": query, "rev_comp": "F", "v_cigar": v, "j_cigar": j}), "AAA", "GGG", &r)
		if why == "" {
			if len(r.Query) != len(r.Germline) || len(r.Query) != len(r.QueryPositions) || len(r.Query) != len(r.ReferencePositions) {
				t.Fatal(r)
			}
			if _, e := json.Marshal(r); e != nil {
				t.Fatal(e)
			}
		}
	})
}

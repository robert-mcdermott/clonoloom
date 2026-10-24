package bcell

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

func TestLineagePanelIdentityAndMetadata(t *testing.T) {
	a := familyRow("a", "a", "", "IGH", "TGCAAATGG")
	a.Evidence = germlineEvidence(map[string]string{"sequence": "AAATGCAAATGG", "rev_comp": "F", "v_cigar": "6M6S", "j_cigar": "9S3M"})
	a.C = "IGHM"
	b := a
	b.SequenceID = "b"
	b.C = "IGHG1"
	b.Count = 10000
	c := a
	c.SequenceID = "c"
	c.Evidence[fields["sequence"]] = "ACATGCAAATGG"
	tab := familyTable(t, []rep.Row{a, b, c})
	sources := []FamilySource{{Name: "test.tsv", SHA256: "hash"}}
	f, err := Families(context.Background(), tab, nil, sources, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := Germlines(context.Background(), tab, sources, f, GermlineReference{"IGHV1-1*01": "AAATGC", "IGHJ1*01": "TGG"})
	if err != nil {
		t.Fatal(err)
	}
	ann := []study.Sample{{Name: "a", Time: "day 14", Tissue: "blood"}}
	result, err := Lineages(context.Background(), tab, sources, parent, ann)
	if err != nil || result.Built != 1 {
		t.Fatal(err, result)
	}
	tree := result.Trees[0]
	if tree.Sites != 9 || tree.Taxa != 2 || tree.Score != 1 || tree.Included != 3 || *tree.OptimalTrees != 1 || len(tree.Nodes) != 2 {
		t.Fatal(tree)
	}
	if tree.Nodes[0].Rows != 2 || len(tree.Nodes[0].Annotations) != 2 || tree.Nodes[0].Annotations[0].Timepoint != "day 14" {
		t.Fatal(tree.Nodes)
	}
	for _, m := range result.Members {
		if m.Node == "" || m.Tissue != "blood" || m.Excluded != "" {
			t.Fatal(m)
		}
	}
	// Indel/ambiguous/uncovered reference columns are removed globally, not treated
	// as a fifth nucleotide or as a mutation-free base in only some rows.
	parent.Rows[0].Query = "N" + parent.Rows[0].Query[1:]
	result, err = Lineages(context.Background(), tab, sources, parent, ann)
	if err != nil || result.Trees[0].Sites != 8 || result.Trees[0].Omitted != 1 {
		t.Fatal(result, err)
	}
	before := result
	parent.Rows[0], parent.Rows[2] = parent.Rows[2], parent.Rows[0]
	result, err = Lineages(context.Background(), tab, sources, parent, ann)
	if err != nil || !reflect.DeepEqual(before, result) {
		t.Fatal("row order changed result", err)
	}
	// A failed first projection must not supply the allele choice for usable rows.
	saved := parent.Rows[0]
	parent.Rows[0].Excluded, parent.Rows[0].V, parent.Rows[0].J = "missing evidence", "", ""
	result, err = Lineages(context.Background(), tab, sources, parent, ann)
	if err != nil || result.Built != 1 || result.Trees[0].Included != 2 || result.Excluded["missing evidence"] != 1 {
		t.Fatal(result, err)
	}
	parent.Rows[0] = saved
	parent.Rows[0].Sample = "wrong"
	if _, err = Lineages(context.Background(), tab, sources, parent, ann); err == nil {
		t.Fatal("source mismatch accepted")
	}
}
func TestLineageRefusalsAndBounds(t *testing.T) {
	g, raw := shmFixture(t)
	_ = raw
	g.ReferencePositions[0] = g.ReferencePositions[1]
	if _, why := lineagePanelRow(g); why == "" {
		t.Fatal("duplicate mapping")
	}
	seqs := make([]string, 8)
	for i := 0; i < 2000; i++ {
		for j := range seqs {
			seqs[j] += string("ACGT"[(i>>(2*j))&3])
		}
	}
	if _, err := searchTree(context.Background(), seqs, false); err != errLineageWork {
		t.Fatal(err)
	}
	for _, r := range []GermlineRow{{}, {Query: "A", Germline: "A"}} {
		if _, why := lineagePanelRow(r); why == "" {
			t.Fatal("invalid maps")
		}
	}
}
func FuzzLineageProjection(f *testing.F) {
	f.Add("ACG", "VVJ", 1)
	f.Fuzz(func(t *testing.T, q, regions string, p int) {
		if len(q) > 20000 {
			return
		}
		r := GermlineRow{Query: q, Germline: strings.Repeat("A", len(q)), Regions: regions, ReferencePositions: make([]int, len(q)), QueryPositions: make([]int, len(q))}
		for i := range q {
			r.ReferencePositions[i] = p
		}
		lineagePanelRow(r)
	})
}

func TestLineageStateLimitAndEmptyPanel(t *testing.T) {
	var rows []rep.Row
	for i := 0; i < 65; i++ {
		r := familyRow(string(rune('a'+i)), "a", "", "IGH", "TGCAAATGG")
		q := []byte("AAATGCAAATGG")
		for j := 0; j < 4; j++ {
			q[j] = "ACGT"[(i>>(2*j))&3]
		}
		r.Evidence = germlineEvidence(map[string]string{"sequence": string(q), "rev_comp": "F", "v_cigar": "6M6S", "j_cigar": "9S3M"})
		rows = append(rows, r)
	}
	tab := familyTable(t, rows)
	sources := []FamilySource{{Name: "test.tsv", SHA256: "hash"}}
	families, err := Families(context.Background(), tab, nil, sources, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := Germlines(context.Background(), tab, sources, families, GermlineReference{"IGHV1-1*01": "AAATGC", "IGHJ1*01": "TGG"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Lineages(context.Background(), tab, sources, parent, nil)
	if err != nil || result.Built != 0 || len(result.Trees) != 1 || !strings.Contains(result.Trees[0].Excluded, "64 distinct") {
		t.Fatal(result, err)
	}
	for _, m := range result.Members {
		if m.Node != "" || m.Excluded == "" {
			t.Fatal(m)
		}
	}
	for i := range parent.Rows {
		parent.Rows[i].Query = strings.Repeat("N", len(parent.Rows[i].Query))
	}
	result, err = Lineages(context.Background(), tab, sources, parent, nil)
	if err != nil || result.Built != 0 || !strings.Contains(result.Trees[0].Excluded, "no shared") {
		t.Fatal(result, err)
	}
}

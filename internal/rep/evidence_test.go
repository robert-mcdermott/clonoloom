package rep

import "testing"

func TestOptionalEvidenceBackfill(t *testing.T) {
	b := NewBuilder(Sequences)
	r := NewRow()
	r.Sample = "S"
	r.Count = 1
	if err := b.Append(&r); err != nil {
		t.Fatal(err)
	}
	r.Evidence[0] = "acGn.-"
	if err := b.Append(&r); err != nil {
		t.Fatal(err)
	}
	r.Evidence[0] = ""
	if err := b.Append(&r); err != nil {
		t.Fatal(err)
	}
	table := b.Table()
	if table.EvidenceValue(0, 0) != "" || table.EvidenceValue(1, 0) != "acGn.-" || table.EvidenceValue(2, 0) != "" {
		t.Fatal("backfill or original text differs")
	}
}

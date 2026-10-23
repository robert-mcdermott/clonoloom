package export

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

func TestEvidenceRoundTrip(t *testing.T) {
	data, err := os.ReadFile("../../testdata/bcell-inputs.tsv")
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), data...)
	first, err := reads.ReadReader(context.Background(), bytes.NewReader(data), "evidence.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Rearrangements(&out, first.Table, nil, nil); err != nil {
		t.Fatal(err)
	}
	second, err := reads.ReadReader(context.Background(), strings.NewReader(out.String()), "roundtrip.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if first.Table.Len() != second.Table.Len() {
		t.Fatal("rows differ")
	}
	for row := 0; row < first.Table.Len(); row++ {
		for i, name := range rep.EvidenceFields {
			if first.Table.EvidenceValue(row, i) != second.Table.EvidenceValue(row, i) {
				t.Fatalf("row %d field %s", row, name)
			}
		}
	}
	after, _ := os.ReadFile("../../testdata/bcell-inputs.tsv")
	if !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
}

package export

import (
	"bytes"
	"encoding/csv"
	"math"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/specificity"
)

func TestSpecificityEvidenceAndEmptyShares(t *testing.T) {
	result := specificity.Result{
		Records:  []specificity.Record{{Row: 3, Complex: "7", CDR3: "CASSLQETQYF", Reference: "reference, with comma", Score: 2}},
		Matches:  []specificity.Match{{Clonotype: 0, Chain: 0, Record: 0, Distance: 12}},
		Epitopes: []specificity.Epitope{{Epitope: "PEPTIDE", Counts: []float64{0}, Shares: []diversity.Number{diversity.Number(math.NaN())}}},
	}
	var b bytes.Buffer
	if err := SpecificityMatches(&b, []clonotype.Clonotype{{ID: "one", Chains: []clonotype.Chain{{Locus: "TRB", Junction: "CASSLGQETQYF"}}}}, result); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][4] != "12" || rows[1][5] != "3" || rows[1][18] != "reference, with comma" {
		t.Fatal(rows)
	}
	b.Reset()
	if err := SpecificityShares(&b, []string{"empty"}, result); err != nil {
		t.Fatal(err)
	}
	rows, err = csv.NewReader(&b).ReadAll()
	if err != nil || rows[1][5] != "" {
		t.Fatal(rows, err)
	}
}

package validation

import (
	"clonoloom/internal/reads"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/specificity"
)

func checkSpecificityLookup(t *testing.T, database, reference string) {
	t.Helper()
	raw, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(database)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	records, err := specificity.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy, SHA256 string
		Queries             []struct {
			Locus, CDR3, V, J string
			Genes             bool
			Score             int
			Rows              []int
		}
	}
	data, err := os.ReadFile(reference)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != ref.SHA256 {
		t.Fatal("database checksum differs from the independent lookup")
	}
	def, _ := (clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}).Normalize()
	// Batch requests by options to build the reference index once for each choice.
	for _, genes := range []bool{false, true} {
		for _, score := range []int{0, 2} {
			tb := &clonotype.Table{Definition: def, Start: []int32{0}, SampleTotals: []float64{1}}
			want := [][]int{}
			for _, q := range ref.Queries {
				if q.Genes != genes || q.Score != score {
					continue
				}
				tb.Clonotypes = append(tb.Clonotypes, clonotype.Clonotype{ID: fmt.Sprint(len(want)), Chains: []clonotype.Chain{{Locus: q.Locus, Junction: q.CDR3, V: q.V, J: q.J}}})
				tb.Start = append(tb.Start, 0)
				want = append(want, q.Rows)
			}
			got, err := specificity.Find(context.Background(), tb, records, specificity.Options{Genes: genes, MinScore: score}, nil)
			if err != nil {
				t.Fatal(err)
			}
			rows := make([][]int, len(want))
			for i := range rows {
				rows[i] = []int{}
			}
			for _, m := range got.Matches {
				rows[m.Clonotype] = append(rows[m.Clonotype], got.Records[m.Record].Row)
			}
			if !reflect.DeepEqual(rows, want) {
				t.Fatalf("lookup disagreement: genes %t, minimum score %d", genes, score)
			}
		}
	}
	t.Logf("%d exact row-identity lookups agree with %s (%d database records)", len(ref.Queries), ref.GeneratedBy, len(records))
}
func TestSpecificityPandas(t *testing.T) {
	path, ok := referencePath(t, "simulated-vaccine/database/vdjdb.txt")
	if !ok {
		return
	}
	checkSpecificityLookup(t, path, filepath.Join("reference", "specificity.json"))
}
func TestSpecificityVDJdbMaintainer(t *testing.T) {
	db, ref := os.Getenv("CLONOLOOM_VDJDB"), os.Getenv("CLONOLOOM_VDJDB_LOOKUP")
	if db == "" || ref == "" {
		t.Skip("maintainer-only VDJdb lookup: set CLONOLOOM_VDJDB and CLONOLOOM_VDJDB_LOOKUP; no real database fixtures are distributed")
	}
	checkSpecificityLookup(t, db, ref)
}

// The independent lookup also joins each sample to the simulator's truth. The
// database contains other donors' family members, not every true receptor.
func TestSpecificityVaccineShares(t *testing.T) {
	path, ok := referencePath(t, "simulated-vaccine/database/vdjdb.txt")
	if !ok {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	records, err := specificity.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	var want []struct {
		Sample, Epitope                    string
		Total, Matched, TrueMatched, Truth float64
	}
	raw, err := os.ReadFile("reference/specificity-shares.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(dataRoot(), "simulated-vaccine/S*-*.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	read, err := reads.ReadFiles(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	table, err := clonotype.Compute(read.Table, nil, clonotype.Definition{Chains: []string{"TRB"}, Junction: "aa", V: "gene", J: "gene"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := specificity.Find(context.Background(), table, records, specificity.Options{Genes: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bySample := map[string]int{}
	for i, s := range read.Table.Samples {
		bySample[s.Name] = i
	}
	byEpitope := map[string]specificity.Epitope{}
	for _, ep := range result.Epitopes {
		byEpitope[ep.Epitope] = ep
	}
	matched, truth, supported := 0.0, 0.0, 0.0
	for _, w := range want {
		i, ok := bySample[w.Sample]
		if !ok {
			t.Fatal("missing sample", w.Sample)
		}
		got := 0.0
		share := 0.0
		if ep, ok := byEpitope[w.Epitope]; ok {
			got = ep.Counts[i]
			share = float64(ep.Shares[i])
		}
		if table.SampleTotals[i] != w.Total || got != w.Matched || math.Abs(share-w.Matched/w.Total) > 1e-14 {
			t.Fatalf("%s %s: count %g want %g, total %g want %g", w.Sample, w.Epitope, got, w.Matched, table.SampleTotals[i], w.Total)
		}
		matched += got
		truth += w.Truth
		supported += w.TrueMatched
	}
	if matched == 0 || truth <= matched {
		t.Fatal("expected partial database coverage", matched, truth)
	}
	t.Logf("%d sample/epitope shares match pandas; %g matched molecules, %g supported by truth, %g total truth molecules", len(want), matched, supported, truth)
}

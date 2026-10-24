package validation

// Compare the simulator's observations, as the application's readers and paired clonotypes see
// them, against independent Python CSV/translation counts (reference/bcell_families.py). The
// script additionally replays every mutation edge and checks every cell assignment and weight.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"clonoloom/internal/bcell"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/genes"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
	"clonoloom/internal/simulate"
	"clonoloom/internal/validate"
)

func TestSimulatedBCellFamiliesAgainstIndependentCSV(t *testing.T) {
	var reference struct {
		Seed    uint64 `json:"seed"`
		Samples []struct {
			Sample           string         `json:"sample"`
			Cells            int            `json:"cells"`
			Rows             int            `json:"rows"`
			Loci             map[string]int `json:"loci"`
			Isotypes         map[string]int `json:"isotypes"`
			PairedClonotypes int            `json:"pairedClonotypes"`
			NetDifferences   int            `json:"netDifferences"`
			SHA256           string         `json:"sha256"`
		} `json:"samples"`
	}
	data, err := os.ReadFile("reference/bcell-families.json")
	if err != nil || json.Unmarshal(data, &reference) != nil {
		t.Fatalf("reference: %v", err)
	}
	s := simulate.Presets["bcell-immunization"]
	if reference.Seed != s.Seed || len(reference.Samples) != 9 {
		t.Fatal("reference and preset differ")
	}
	dir := filepath.Join(t.TempDir(), "study")
	if _, err := simulate.Run(context.Background(), s, dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, sample := range reference.Samples {
		path := filepath.Join(dir, sample.Sample+".tsv")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != sample.SHA256 {
			t.Fatalf("%s: file differs from independent reference", sample.Sample)
		}
		paths = append(paths, path)
	}
	paths = append(paths, filepath.Join(dir, "repertoires.airr.json"))
	res, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if report := validate.Check(res); len(report.Findings) != 0 {
		t.Fatalf("unexpected findings: %+v", report.Findings)
	}
	table := res.Table
	if res.Unit != rep.Cells || len(res.Repertoires) != 9 || len(table.Reads)+len(table.UMIs)+len(table.Clone) != 0 {
		t.Fatalf("unit, metadata or invented counts: %s; %d repertoires", res.Unit, len(res.Repertoires))
	}
	for _, r := range res.Repertoires {
		if r.SubjectID == "" || r.TimePoint == "" || r.SingleCell == nil || !*r.SingleCell || r.StudyID != "clonoloom-simulated-clonoloom-artificial-bcell-v2" {
			t.Fatalf("metadata: %+v", r)
		}
	}
	definition := clonotype.Presets[4]
	definition.RequireAll = true
	clones, err := clonotype.Compute(table, nil, definition)
	if err != nil {
		t.Fatal(err)
	}
	for si, sample := range table.Samples {
		want := reference.Samples[si]
		if sample.Name != want.Sample {
			t.Fatal("sample order differs")
		}
		loci, isotypes := map[string]int{}, map[string]int{}
		cells := map[uint32]bool{}
		rows, net, observedClones := 0, 0, 0
		for row, index := range table.Sample {
			if int(index) != si {
				continue
			}
			rows++
			cells[table.Cell[row]] = true
			locus := genes.Loci[table.Locus[row]-1]
			loci[locus]++
			if locus == "IGH" {
				isotypes[table.Dict.Get(table.C[row])]++
			}
			evidence := table.EvidenceRow(row)
			check := bcell.Check(evidence)
			if !check.Sequence || !check.Paired || !check.Valid || !check.Regions || len(check.Issues) != 0 {
				t.Fatalf("row %d evidence: %+v", row, check)
			}
			for i := range evidence[2] {
				if evidence[2][i] != evidence[3][i] {
					net++
				}
			}
		}
		for ci := range clones.Clonotypes {
			if clones.In(ci, uint32(si)) != 0 {
				observedClones++
			}
		}
		if rows != want.Rows || len(cells) != want.Cells || clones.SampleTotals[si] != float64(want.Cells) || observedClones != want.PairedClonotypes || net != want.NetDifferences || !reflect.DeepEqual(loci, want.Loci) || !reflect.DeepEqual(isotypes, want.Isotypes) {
			t.Fatalf("%s: rows %d cells %d paired total %g clonotypes %d net %d loci %v isotypes %v; want %+v", sample.Name, rows, len(cells), clones.SampleTotals[si], observedClones, net, loci, isotypes, want)
		}
	}
	t.Log("9 samples, 2160 paired cells, 4320 sequence rows agree with independent CSV counts; all alignments structurally valid")
}

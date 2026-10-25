package validation

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/countmodel"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"clonoloom/internal/study"
)

// Independent truth defines the positive/unchanged panel before any model result.
// Ratios use realized repertoire totals; do not assert equality to generator exposures.
func TestReplicatedCountExamplesAgainstTruth(t *testing.T) {
	for _, preset := range []string{"counts-independent", "counts-paired", "counts-null"} {
		t.Run(preset, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "study")
			s := simulate.Presets[preset]
			if _, err := simulate.Run(context.Background(), s, dir, "validation", nil); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(filepath.Join(dir, "truth/observations.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rd := csv.NewReader(f)
			rd.Comma = '\t'
			rows, err := rd.ReadAll()
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			effects := map[string]float64{}
			samples := map[string]study.CohortSample{}
			names := []string{}
			for _, r := range rows[1:] {
				e, err := strconv.ParseFloat(r[9], 64)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(r[4], "0:") {
					effects[r[5]] = e
				}
				if _, ok := samples[r[0]]; !ok {
					names = append(names, r[0])
					samples[r[0]] = study.CohortSample{Name: r[0], Subject: r[1], Condition: r[2], Batch: r[3]}
				}
			}
			paths := []string{}
			design := study.CohortInput{Dataset: "truth", Spec: study.CohortSpec{Unit: "subject", Layout: s.Counts.Layout, GroupField: "condition", Levels: []string{"0", "1"}}}
			for _, n := range names {
				paths = append(paths, filepath.Join(dir, n+".tsv"))
				design.Samples = append(design.Samples, samples[n])
			}
			raw, err := reads.Open(context.Background(), paths)
			if err != nil {
				t.Fatal(err)
			}
			clones, err := clonotype.Compute(raw.Table, nil, clonotype.Definition{Junction: "nt", V: "gene", J: "gene"})
			if err != nil {
				t.Fatal(err)
			}
			m, err := countmodel.Design(design, true)
			if err != nil {
				t.Fatal(err)
			}
			labels := []string{}
			for _, sample := range raw.Table.Samples {
				labels = append(labels, sample.Name)
			}
			in, err := countmodel.Capture(clones, labels, m, raw.Table)
			if err != nil {
				t.Fatal(err)
			}
			result, err := countmodel.Compute(context.Background(), design, in, countmodel.Options{AdjustBatch: true}, nil)
			if err != nil {
				t.Fatal(err)
			}
			changed, fittedChanged, wrongDirection, unchanged, declared := 0, 0, 0, 0, 0
			for _, r := range result.Rows {
				effect, shared := effects[r.Chains[0].Junction]
				if !shared && r.Eligible {
					t.Fatal("private receptor entered inference family")
				}
				if !shared {
					continue
				}
				if effect != 0 {
					changed++
					if math.IsNaN(float64(r.P)) {
						continue
					}
					fittedChanged++
					if float64(r.Log2Fold)*effect <= 0 {
						wrongDirection++
					}
				} else {
					unchanged++
					if !math.IsNaN(float64(r.Adjusted)) && r.Adjusted <= .05 {
						declared++
					}
				}
			}
			t.Logf("%s: %d/%d fitted; %d/%d changed-panel fits, %d wrong directions; %d/%d unchanged with BH<=0.05", preset, result.Tested, result.Family, fittedChanged, changed, wrongDirection, declared, unchanged)
			if result.Family > 48 || result.Tested < 30 || wrongDirection != 0 || !s.Counts.Null && (changed != 8 || fittedChanged < 6) {
				t.Fatal(fmt.Sprintf("unexpected benchmark coverage/direction: %+v", result.Options))
			}
			// Report null outcomes; a single null study is not a calibration guarantee.
		})
	}
}

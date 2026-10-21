// QC on the development files against independent computations: each bulk sample's rows,
// abundance, productive share, clonotypes, singletons, doubletons and largest clonotype's share as
// data.table tallies them, and its sample coverage as iNEXT computes it
// (validation/reference/qc.json, made by qc.R); and every number of Cell Ranger's metrics summaries
// that the contig annotations allow recomputed exactly.
package validation

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/qc"
	"clonoloom/internal/reads"
)

func TestQCAgreesWithAnIndependentImplementationAndINEXT(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "qc.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string `json:"generated_by"`
		Files       map[string]map[string]struct {
			Rows       int      `json:"rows"`
			Abundance  float64  `json:"abundance"`
			Productive float64  `json:"productive"`
			Clonotypes int      `json:"clonotypes"`
			Singletons int      `json:"singletons"`
			Doubletons int      `json:"doubletons"`
			Coverage   *float64 `json:"coverage"`
			TopShare   float64  `json:"top_share"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	close := func(a, b float64) bool { return math.Abs(a-b) <= 1e-12*math.Max(1, math.Abs(b)) }
	compared := 0
	for rel, samples := range ref.Files {
		dataset := strings.Split(filepath.ToSlash(rel), "/")[0]
		path := filepath.Join(dataRoot(), filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			if required(dataset) {
				t.Errorf("%s is required and missing", rel)
			}
			continue
		}
		res, err := reads.ReadFiles(context.Background(), []string{path})
		if err != nil {
			t.Fatal(err)
		}
		report := qc.Compute(res.Table, qc.Filter{})
		for _, s := range report.Samples {
			want, ok := samples[s.Name]
			if !ok {
				t.Errorf("%s: sample %s is not in the reference", rel, s.Name)
				continue
			}
			if s.Rows != want.Rows || s.Clonotypes != want.Clonotypes || s.Singletons != want.Singletons || s.Doubletons != want.Doubletons {
				t.Errorf("%s %s: rows %d clonotypes %d f1 %d f2 %d; the reference %d %d %d %d", rel, s.Name, s.Rows, s.Clonotypes, s.Singletons, s.Doubletons, want.Rows, want.Clonotypes, want.Singletons, want.Doubletons)
			}
			if !close(s.Abundance, want.Abundance) || !close(s.Productive, want.Productive) || !close(s.TopShare, want.TopShare) {
				t.Errorf("%s %s: abundance %v productive %v top %v; the reference %v %v %v", rel, s.Name, s.Abundance, s.Productive, s.TopShare, want.Abundance, want.Productive, want.TopShare)
			}
			if want.Coverage != nil && !close(float64(s.Coverage), *want.Coverage) {
				t.Errorf("%s %s: coverage %.15f, iNEXT %.15f", rel, s.Name, float64(s.Coverage), *want.Coverage)
			}
			compared++
		}
	}
	t.Logf("%d samples compared with %s", compared, ref.GeneratedBy)
}

// Cell Ranger's own metrics, recomputed from its contig annotations: cells, cells with a
// productive pair and with a productive chain of each locus, as counts and shares.
func TestCellRangerMetricsAreReproduced(t *testing.T) {
	runs := []struct{ dataset, contigs, metrics string }{
		{"10x-pbmc-1k", "sc5p_v1p1_hs_PBMC_1k_t_filtered_contig_annotations.csv", "sc5p_v1p1_hs_PBMC_1k_t_metrics_summary.csv"},
		{"10x-pbmc-1k", "sc5p_v1p1_hs_PBMC_1k_b_filtered_contig_annotations.csv", "sc5p_v1p1_hs_PBMC_1k_b_metrics_summary.csv"},
		{"10x-pbmc-10k-v2", "10k_PBMC_5pv2_nextgem_Chromium_Controller_10k_PBMC_5pv2_nextgem_Chromium_Controller_vdj_t_filtered_contig_annotations.csv", "10k_PBMC_5pv2_nextgem_Chromium_Controller_10k_PBMC_5pv2_nextgem_Chromium_Controller_metrics_summary.csv"},
		{"10x-pbmc-10k-v2", "10k_PBMC_5pv2_nextgem_Chromium_Controller_10k_PBMC_5pv2_nextgem_Chromium_Controller_vdj_b_filtered_contig_annotations.csv", "10k_PBMC_5pv2_nextgem_Chromium_Controller_10k_PBMC_5pv2_nextgem_Chromium_Controller_metrics_summary.csv"},
		{"10x-mouse-splenocytes-5k-gemx", "5k_Mouse_Splenocytes_5p_gem-x_5k_Mouse_Splenocytes_5p_gem-x_vdj_t_filtered_contig_annotations.csv", "5k_Mouse_Splenocytes_5p_gem-x_5k_Mouse_Splenocytes_5p_gem-x_metrics_summary.csv"},
		{"10x-mouse-splenocytes-5k-gemx", "5k_Mouse_Splenocytes_5p_gem-x_5k_Mouse_Splenocytes_5p_gem-x_vdj_b_filtered_contig_annotations.csv", "5k_Mouse_Splenocytes_5p_gem-x_5k_Mouse_Splenocytes_5p_gem-x_metrics_summary.csv"},
	}
	for _, run := range runs {
		contigs := filepath.Join(dataRoot(), run.dataset, run.contigs)
		if _, err := os.Stat(contigs); err != nil {
			if required(run.dataset) {
				t.Errorf("%s is required and missing", run.dataset)
			}
			continue
		}
		res, err := reads.ReadFiles(context.Background(), []string{contigs})
		if err != nil {
			t.Fatal(err)
		}
		metrics, err := qc.CellRangerMetrics(filepath.Join(dataRoot(), run.dataset, run.metrics))
		if err != nil {
			t.Fatal(err)
		}
		s := qc.Compute(res.Table, qc.Filter{}).Samples[0]
		var loci []string
		for l := range s.Chains.ByLocus {
			loci = append(loci, l)
		}
		checks := qc.CompareCellRanger(metrics, s.Chains, loci)
		compared := 0
		for _, c := range checks {
			if c.Agrees == nil {
				continue
			}
			compared++
			if !*c.Agrees {
				t.Errorf("%s: %s reported %s, recomputed %s", run.contigs, c.Metric, c.Reported, c.Recomputed)
			}
		}
		if compared < 3 {
			t.Errorf("%s: only %d metrics compared", run.contigs, compared)
		}
	}
}

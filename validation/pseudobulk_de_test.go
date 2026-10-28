package validation

import (
	"clonoloom/internal/expression"
	"clonoloom/internal/pseudobulk"
	"clonoloom/internal/pseudobulkde"
	"clonoloom/internal/simulate"
	"clonoloom/internal/study"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPseudobulkDEIndependentReference(t *testing.T) {
	raw, e := os.ReadFile("reference/pseudobulk_de.json")
	if e != nil {
		t.Fatal(e)
	}
	var ref struct {
		Input                pseudobulkde.Input
		Options              pseudobulkde.Options
		SizeFactors, Offsets []float64
		NormalizationGenes   []string
		X                    [][]float64
		Rows                 []struct {
			ID                                                      string
			Eligible                                                bool
			Normalized                                              []float64
			Mean                                                    float64
			Log2Fold, SE, Lower, Upper, Dispersion, LR, P, Adjusted *float64
			Beta, Mu                                                []float64
			Covariance                                              [][]float64
			LogLikelihood, NullLogLikelihood                        float64
		}
	}
	if e = json.Unmarshal(raw, &ref); e != nil {
		t.Fatal(e)
	}
	r, e := pseudobulkde.Compute(context.Background(), ref.Input, ref.Options, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(r)
	if digest := fmt.Sprintf("%x", sha256.Sum256(b)); digest != "d6f7d73fa3bd28d6e0111a8814949c644201df8afe78a0476cee22c8e73d4b33" {
		t.Fatal("result digest", digest)
	}
	close := func(label string, a, b, tol float64) {
		t.Helper()
		if math.IsNaN(a) || math.Abs(a-b) > tol*math.Max(1, math.Abs(b)) {
			t.Errorf("%s %.12g want %.12g", label, a, b)
		}
	}
	if r.Family != 38 || r.Tested != 37 || len(r.Rows) != 40 || r.Design.ResidualDF != 27 {
		t.Fatal("family", r.Family, r.Tested, r.Design.ResidualDF)
	}
	for i, v := range r.SizeFactors {
		close("size factor", v, ref.SizeFactors[i], 1e-12)
		close("offset", r.Offsets[i], ref.Offsets[i], 1e-12)
		for j, x := range r.Design.X[i] {
			close("design", x, ref.X[i][j], 0)
		}
	}
	if stringJSON(r.NormalizationGenes) != stringJSON(ref.NormalizationGenes) {
		t.Fatal("normalization gene set")
	}
	for i, row := range r.Rows {
		want := ref.Rows[i]
		if row.Feature.ID != want.ID || row.Eligible != want.Eligible {
			t.Fatal("identity/filter", i)
		}
		close("mean", row.Mean, want.Mean, 1e-12)
		for j, v := range row.Normalized {
			close("normalized", v, want.Normalized[j], 1e-12)
		}
		for _, pair := range []struct {
			name string
			got  float64
			want *float64
		}{{"log2Fold", float64(row.Log2Fold), want.Log2Fold}, {"se", float64(row.SE), want.SE}, {"lower", float64(row.Lower), want.Lower}, {"upper", float64(row.Upper), want.Upper}, {"dispersion", float64(row.Dispersion), want.Dispersion}, {"lr", float64(row.LR), want.LR}, {"p", float64(row.P), want.P}, {"BH", float64(row.Adjusted), want.Adjusted}} {
			if pair.want == nil {
				if !math.IsNaN(pair.got) {
					t.Errorf("%s should be missing", pair.name)
				}
			} else {
				close(pair.name, pair.got, *pair.want, 3e-5)
			}
		}
		if want.P != nil {
			close("loglik", row.Full.LogLikelihood, want.LogLikelihood, 2e-6)
			close("null loglik", row.Null.LogLikelihood, want.NullLogLikelihood, 2e-6)
			for j, b := range row.Full.Beta {
				close("beta", b, want.Beta[j], 3e-5)
				for k, v := range row.Full.Covariance[j] {
					close("covariance", v, want.Covariance[j][k], 3e-5)
				}
			}
			for j, v := range row.Full.Mu {
				close("mu", v, want.Mu[j], 3e-5)
			}
		}
	}
}
func stringJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// Prespecified independent 300-gene study: 30 balanced positive controls and 270
// generating nulls, including the explicit zero-block and low-count controls.
// A finite simulation diagnostic, not universal calibration or independent studies.
func TestPseudobulkDENullPositiveControls(t *testing.T) {
	raw, e := os.ReadFile("reference/pseudobulk_de_controls.json")
	if e != nil {
		t.Fatal(e)
	}
	var ref struct {
		Input              pseudobulkde.Input
		Options            pseudobulkde.Options
		GeneratingLog2Fold []float64
		Rows               []struct{ P, Adjusted, Log2Fold, Lower, Upper, Dispersion *float64 }
		Diagnostics        struct{ NullFitted, NullRejected, NullCovered, PositiveFitted, PositiveCorrectDirection int }
	}
	if e = json.Unmarshal(raw, &ref); e != nil {
		t.Fatal(e)
	}
	r, e := pseudobulkde.Compute(context.Background(), ref.Input, ref.Options, nil)
	if e != nil {
		t.Fatal(e)
	}
	null, fpr, covered, positive, direction := 0, 0, 0, 0, 0
	for i, row := range r.Rows {
		want := ref.Rows[i]
		for _, p := range []struct {
			got  float64
			want *float64
		}{{float64(row.P), want.P}, {float64(row.Adjusted), want.Adjusted}, {float64(row.Log2Fold), want.Log2Fold}, {float64(row.Lower), want.Lower}, {float64(row.Upper), want.Upper}, {float64(row.Dispersion), want.Dispersion}} {
			if p.want == nil {
				if !math.IsNaN(p.got) {
					t.Fatal("unexpected fit", i)
				}
			} else if math.IsNaN(p.got) || math.Abs(p.got-*p.want) > 3e-5*math.Max(1, math.Abs(*p.want)) {
				t.Fatal("independent control differs", i, p.got, *p.want)
			}
		}
		if math.IsNaN(float64(row.P)) {
			continue
		}
		if ref.GeneratingLog2Fold[i] == 0 {
			null++
			if row.P <= .05 {
				fpr++
			}
			if row.Lower <= 0 && row.Upper >= 0 {
				covered++
			}
		} else {
			positive++
			if float64(row.Log2Fold)*ref.GeneratingLog2Fold[i] > 0 {
				direction++
			}
		}
	}
	d := ref.Diagnostics
	if null != d.NullFitted || fpr != d.NullRejected || covered != d.NullCovered || positive != d.PositiveFitted || direction != d.PositiveCorrectDirection {
		t.Fatal("control accounting differs")
	}
	t.Logf("null: %d fitted, %d nominal p<=.05 (%.3f), %d conditional 95%% intervals cover zero (%.3f); positive: %d/%d correct direction", null, fpr, float64(fpr)/float64(null), covered, float64(covered)/float64(null), direction, positive)
	if null < 260 || float64(fpr)/float64(null) > .1 || float64(covered)/float64(null) < .9 || positive != 30 || direction != positive {
		t.Fatal("prespecified approximation guard failed")
	}
}

func TestPseudobulkDEExampleControls(t *testing.T) {
	for _, preset := range []string{"pseudobulk-de-study", "pseudobulk-de-null"} {
		t.Run(preset, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "study")
			if _, e := simulate.Run(context.Background(), simulate.Presets[preset], dir, "validation", nil); e != nil {
				t.Fatal(e)
			}
			read := func(name string) [][]string {
				t.Helper()
				f, e := os.Open(filepath.Join(dir, "truth", name))
				if e != nil {
					t.Fatal(e)
				}
				defer f.Close()
				c := csv.NewReader(f)
				c.Comma = '\t'
				rows, e := c.ReadAll()
				if e != nil {
					t.Fatal(e)
				}
				return rows[1:]
			}
			in := pseudobulkde.Input{Bins: []string{"1", ">1"}, MinCells: 10}
			indices := map[string]int{}
			cellCounts := map[[2]string]int{}
			for _, v := range read("observations.tsv") {
				if _, ok := indices[v[0]]; !ok {
					indices[v[0]] = len(in.Samples)
					in.Samples = append(in.Samples, study.Sample{Name: v[0], Subject: v[1], Time: "baseline"})
				}
				cellCounts[[2]string{v[0], v[4]}]++
			}
			for g := 0; g < 40; g++ {
				in.Features = append(in.Features, expression.Feature{ID: fmt.Sprintf("SIM-G%02d", g+1), Name: fmt.Sprintf("Artificial gene %02d", g+1), Type: "Gene Expression"})
			}
			groups := map[[2]string]int{}
			for _, v := range read("counts.tsv") {
				key := [2]string{v[0], v[1]}
				j, ok := groups[key]
				if !ok {
					j = len(in.Groups)
					groups[key] = j
					total := uint64(0)
					in.Groups = append(in.Groups, pseudobulk.Group{Sample: indices[v[0]], Type: "Artificial T cells", Bin: v[1], Cells: cellCounts[key], Counts: []uint64{}, Total: &total, Eligible: true})
				}
				y, e := strconv.ParseUint(v[3], 10, 64)
				if e != nil {
					t.Fatal(e)
				}
				g := &in.Groups[j]
				g.Counts = append(g.Counts, y)
				*g.Total += y
			}
			r, e := pseudobulkde.Compute(context.Background(), in, pseudobulkde.Options{Type: "Artificial T cells", Time: "baseline", Reference: "1", Comparison: ">1"}, nil)
			if e != nil {
				t.Fatal(e)
			}
			if r.Tested != 37 || r.Family != 38 {
				t.Fatal("example coverage", r.Tested, r.Family)
			}
			declared := 0
			for i, row := range r.Rows {
				if !math.IsNaN(float64(row.Adjusted)) && row.Adjusted <= .05 {
					declared++
				}
				if preset == "pseudobulk-de-study" && i < 4 {
					direction := 1.
					if i >= 2 {
						direction = -1
					}
					if math.IsNaN(float64(row.P)) || float64(row.Log2Fold)*direction <= 0 {
						t.Fatal("wrong positive control direction", i)
					}
				}
			}
			t.Logf("%s: %d/%d fitted, %d with BH<=.05; single examples are not calibration", preset, r.Tested, r.Family, declared)
		})
	}
}

package countmodel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/dmath"
	"clonoloom/internal/random"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

func fixture(paired bool) (study.CohortInput, Input) {
	n := 32
	if paired {
		n = 56
	}
	return fixtureN(paired, n)
}
func fixtureN(paired bool, n int) (study.CohortInput, Input) {
	d := study.CohortInput{Dataset: "counts", Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
	if paired {
		d.Spec.Layout = "paired"
	}
	in := Input{}
	for i := 0; i < n; i++ {
		group := i / (n / 2)
		subject := fmt.Sprintf("S%02d", i)
		if paired {
			group = i % 2
			subject = fmt.Sprintf("S%02d", i/2)
		}
		name := fmt.Sprintf("sample%02d", i)
		d.Samples = append(d.Samples, study.CohortSample{Name: name, Subject: subject, Condition: d.Spec.Levels[group], Batch: fmt.Sprintf("B%d", (i/2+i%2)%2)})
		in.Samples = append(in.Samples, name)
		in.Totals = append(in.Totals, 0)
	}
	rng := random.New(913, 1)
	for j := 0; j < 8; j++ {
		c := Counts{ID: fmt.Sprintf("clone%d", j)}
		for i := range in.Samples {
			mean := float64(50 + j*20)
			if j == 0 && (i/(n/2) == 1 && !paired || paired && i%2 == 1) {
				mean *= 3
			}
			v := float64(rng.Poisson(rng.Gamma(5) * mean / 5))
			c.Values = append(c.Values, v)
			in.Totals[i] += v
		}
		in.Clonotypes = append(in.Clonotypes, c)
	}
	return d, in
}
func TestCountDesignAndRefusals(t *testing.T) {
	d, in := fixture(true)
	m, e := Design(d, true)
	if e != nil || m.ResidualDF != 26 || len(m.Columns) != 30 {
		t.Fatal(m, e)
	}
	for i := range d.Samples {
		d.Samples[i].Batch = d.Samples[i].Subject
	}
	if _, e = Design(d, true); e == nil {
		t.Fatal("accepted batch aliased with subject")
	}
	d, in = fixture(false)
	in.Clonotypes[0].Values[0] = .5
	if _, e = Compute(context.Background(), d, in, Options{}, nil); e == nil {
		t.Fatal("fractional count accepted")
	}
	d, in = fixture(false)
	in.Totals[0]++
	if _, e = Compute(context.Background(), d, in, Options{}, nil); e == nil {
		t.Fatal("wrong library total accepted")
	}
	d, in = fixture(false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Compute(ctx, d, in, Options{}, nil); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e = FitNB(context.Background(), [][]float64{{1}, {1}}, []float64{1, 2}, []float64{-1000, -1000}, .2); e == nil {
		t.Fatal("invalid exposure accepted")
	}
	if _, _, e = FitDispersion(context.Background(), [][]float64{{1}, {1}, {1}, {1}}, []float64{20, 20, 20, 20}, []float64{0, 0, 0, 0}); e == nil {
		t.Fatal("boundary dispersion accepted")
	}
}
func TestCountModelFamilyAndDeterminism(t *testing.T) {
	d, in := fixture(true)
	// All-zero group is count-filter eligible and stays in BH without invented inference.
	values := make([]float64, len(in.Samples))
	for i := range values {
		if i%2 == 0 {
			values[i] = 100
			in.Totals[i] += 100
		}
	}
	in.Clonotypes = append(in.Clonotypes, Counts{ID: "separated", Values: values})
	r, e := Compute(context.Background(), d, in, Options{}, nil)
	if e != nil {
		t.Fatal(e)
	}
	last := r.Rows[len(r.Rows)-1]
	if r.Family != 9 || r.Tested < 6 || !math.IsNaN(float64(last.P)) || last.Reason == "" {
		t.Fatal(r.Tested, last)
	}
	raw, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	const want = "8968c37aca3a09f581cda25d8799721d0b63891688b9dc2c939c723c47d894dd"
	if digest != want {
		t.Fatalf("digest %s; fitted %d", digest, r.Tested)
	}
}
func TestCountModelNullCalibration(t *testing.T) {
	for _, layout := range []string{"independent", "paired", "unbalanced"} {
		paired := layout == "paired"
		n := 32
		if paired {
			n = 56
		}
		d, _ := fixtureN(paired, n)
		if layout == "unbalanced" {
			for i := range d.Samples {
				if i >= 12 {
					d.Samples[i].Condition = "B"
				}
			}
		}
		m, e := Design(d, false)
		if e != nil {
			t.Fatal(e)
		}
		rng := random.New(20261026, 1)
		tested, rejected, covered := 0, 0, 0
		for trial := 0; trial < 400; trial++ {
			offset := make([]float64, n)
			y := make([]float64, n)
			subject := make([]float64, n/2)
			for i := range subject {
				subject[i] = .4 * rng.Normal()
			}
			alpha := []float64{.08, .2, .5}[trial%3]
			for i := range y {
				offset[i] = dmath.Log(40000 + float64(i%5)*18000)
				mu := dmath.Exp(offset[i] - 7)
				if paired {
					mu *= dmath.Exp(subject[i/2])
				}
				y[i] = float64(rng.Poisson(rng.Gamma(1/alpha) * alpha * mu))
			}
			a, f, e := FitDispersion(context.Background(), m.X, y, offset)
			if e != nil {
				continue
			}
			nx := make([][]float64, n)
			for i, x := range m.X {
				nx[i] = append([]float64{x[0]}, x[2:]...)
			}
			null, e := FitNB(context.Background(), nx, y, offset, a)
			if e != nil {
				continue
			}
			tested++
			lr := math.Max(0, 2*(f.LogLikelihood-null.LogLikelihood))
			if dmath.Erfc(math.Sqrt(lr/2)) <= .05 {
				rejected++
			}
			se := math.Sqrt(f.Covariance[1][1])
			if math.Abs(f.Beta[1]) <= 1.959963984540054*se {
				covered++
			}
		}
		t.Logf("layout=%s fitted=%d/400 null rejects=%d (%.3f) Wald coverage=%.3f", layout, tested, rejected, float64(rejected)/float64(tested), float64(covered)/float64(tested))
		if tested < 380 || float64(rejected)/float64(tested) > .08 || float64(covered)/float64(tested) < .92 {
			t.Fatal("count-model calibration outside declared approximation guard")
		}
	}
}

func TestCountPolicyBoundaries(t *testing.T) {
	for _, paired := range []bool{false, true} {
		d, _ := fixtureN(paired, 24)
		if _, err := Design(d, false); err == nil {
			t.Fatal("accepted insufficient residual replication", paired)
		}
	}
	d, in := fixture(true)
	// A missing subject block remains a family slot; never delete its pair to fit.
	for i := 0; i < 2; i++ {
		in.Totals[i] -= in.Clonotypes[0].Values[i]
		in.Clonotypes[0].Values[i] = 0
	}
	r, err := Compute(context.Background(), d, in, Options{}, nil)
	if err != nil || !r.Rows[0].Eligible || r.Rows[0].Reason != "All-zero subject block: no subject removed to obtain a fit" || r.Family != 8 {
		t.Fatal(r, err)
	}
	if _, err = Compute(context.Background(), d, in, Options{MinSamples: 97}, nil); err == nil {
		t.Fatal("accepted invalid prevalence")
	}
}

func TestCaptureRefusesUnknownAndFractionalSourceCounts(t *testing.T) {
	for _, count := range []float64{math.NaN(), .5, 1} {
		b := rep.NewBuilder(rep.UMIs)
		for i := 0; i < 2; i++ {
			row := rep.NewRow()
			row.Sample = "s"
			row.Locus = "TRB"
			row.V = "TRBV1*01"
			row.J = "TRBJ1*01"
			row.JunctionAA = "CASSF"
			row.Count = count
			row.Productive = rep.True
			if err := b.Append(&row); err != nil {
				t.Fatal(err)
			}
		}
		raw := b.Table()
		table, err := clonotype.Compute(raw, nil, clonotype.Definition{Junction: "aa", V: "gene", J: "gene"})
		if err != nil {
			t.Fatal(err)
		}
		input, err := Capture(table, []string{"s"}, Matrix{Rows: []study.CohortRow{{CohortSample: study.CohortSample{Name: "s"}}}}, raw)
		if count == 1 {
			if err != nil || input.Totals[0] != 2 {
				t.Fatal(input, err)
			}
		} else if err == nil {
			t.Fatal("accepted missing/fractional row counts", count)
		}
	}
}

func TestCountNonconvergenceAndWorkBound(t *testing.T) {
	// Separation has no finite group coefficient; the low-level fitter must not
	// manufacture a converged estimate even when called outside the feature guard.
	if _, err := FitNB(context.Background(), [][]float64{{1, 0}, {1, 0}, {1, 1}, {1, 1}}, []float64{0, 0, 10, 12}, []float64{0, 0, 0, 0}, .2); err == nil {
		t.Fatal("separation converged")
	}
	d, in := fixture(true)
	base := in.Clonotypes[0]
	for j := 0; j < 60; j++ {
		c := base
		c.ID = fmt.Sprintf("work-bound-%d", j)
		in.Clonotypes = append(in.Clonotypes, c)
		for i, y := range c.Values {
			in.Totals[i] += y
		}
	}
	if _, err := Compute(context.Background(), d, in, Options{}, nil); err == nil || !strings.Contains(err.Error(), "work bound") {
		t.Fatal("did not refuse excessive fitting work", err)
	}
}

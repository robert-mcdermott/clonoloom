package batchqc

import (
	"clonoloom/internal/diversity"
	"context"
	"math"
	"testing"
)

func fixture() Input {
	in := Input{Unit: "umis"}
	for i, x := range []float64{.70, .72, .74, .76, .78, .98} {
		v := []Value{}
		for _, y := range []float64{100 + float64(i)*10, 100 + float64(i), x, 1 - x, .9, .95} {
			v = append(v, Value{Value: diversity.Number(y), Numerator: diversity.Number(y), Denominator: 1})
		}
		in.Samples = append(in.Samples, Sample{Assignment: Assignment{Name: string(rune('a' + i)), Subject: string(rune('a' + i)), Batch: "run"}, Values: v})
	}
	return in
}
func TestBatchQCPolicy(t *testing.T) {
	in := fixture()
	r, e := Compute(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	g := r.Groups[2]
	if g.Flagged != 1 || g.Scored != 6 || !g.Observations[5].Flag {
		t.Fatal(g)
	}
	if r.Groups[4].Scored != 0 || r.Groups[4].Reason == "" || !math.IsNaN(float64(r.Groups[4].Observations[0].Z)) {
		t.Fatal("zero MAD produced a score")
	}
	in.Options.Threshold = math.Abs(float64(g.Observations[5].Z))
	boundary, _ := Compute(context.Background(), in)
	if boundary.Groups[2].Observations[5].Flag {
		t.Fatal("threshold must be strictly exceeded")
	}
	in.Options.Threshold = math.Nextafter(in.Options.Threshold, 0)
	boundary, _ = Compute(context.Background(), in)
	if !boundary.Groups[2].Observations[5].Flag {
		t.Fatal("just-above-threshold observation not flagged")
	}
	in.Options.Threshold = 20
	other, _ := Compute(context.Background(), in)
	if other.Groups[2].Flagged != 0 || other.Groups[2].Median != g.Median || other.Groups[2].MAD != g.MAD || len(other.Groups[2].Observations) != 6 {
		t.Fatal("threshold changed evidence")
	}
	in.Samples[0].Assignment.Subject = in.Samples[1].Assignment.Subject
	if _, e = Compute(context.Background(), in); e == nil {
		t.Fatal("repeat inflated reference")
	}
	in.Samples[0].Assignment.Exclude = "Technical repeat"
	if _, e = Compute(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Compute(ctx, fixture()); e != context.Canceled {
		t.Fatal(e)
	}
	for _, mutate := range []func(*Input){func(x *Input) { x.Options.MinSamples = 2 }, func(x *Input) { x.Options.Threshold = math.Inf(1) }, func(x *Input) { x.Samples[0].Values[2].Value = diversity.Number(math.NaN()) }, func(x *Input) { x.Samples[0].Assignment.Batch = "" }, func(x *Input) { x.Samples[0].Assignment.Name = x.Samples[1].Assignment.Name }} {
		v := fixture()
		mutate(&v)
		if _, e = Compute(context.Background(), v); e == nil {
			t.Fatal("invalid accepted")
		}
	}
}

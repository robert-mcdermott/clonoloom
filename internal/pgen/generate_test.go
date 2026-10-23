package pgen

import (
	"math"
	"testing"

	"clonoloom/internal/random"
)

// A VJ model with correlated alleles. Its only receptors are CF and AF, in a 1:3 ratio.
// Independent draws of the V and J marginals would make two impossible allele pairs.
func correlatedVJ() *Model {
	m := &Model{
		V:    []Gene{{Name: "TRAV1*01", Seq: "TGT", Used: true}, {Name: "TRAV1*02", Seq: "GCT", Used: true}},
		J:    []Gene{{Name: "TRAJ1*01", Seq: "TTT", Used: true}, {Name: "TRAJ1*02", Seq: "TTT", Used: true}},
		PVJ:  [][]float64{{0.25, 0}, {0, 0.75}},
		DelV: [][]float64{{1}, {1}}, DelJ: [][]float64{{1}, {1}}, InsVD: []float64{1},
	}
	m.prepare()
	return m
}

func TestDrawWithPreservesJointVJProbability(t *testing.T) {
	g := NewGenerator(correlatedVJ())
	r := random.New(7, 0)
	first := 0
	for range 20000 {
		d, ok := g.DrawWith(r, "TRAV1", "TRAJ1")
		if !ok || d.V[len(d.V)-2:] != d.J[len(d.J)-2:] {
			t.Fatalf("a draw outside the model's support: %+v, %v", d, ok)
		}
		if d.V == "TRAV1*01" {
			first++
		}
	}
	if math.Abs(float64(first)/20000-0.25) > 0.015 {
		t.Errorf("first allele pair: %d/20000, want 0.25", first)
	}
	for range 100 {
		d, ok := g.DrawWith(r, "TRAV1", "TRAJ1*01")
		if !ok || d.V != "TRAV1*01" {
			t.Fatalf("conditioning on J lost the V–J correlation: %+v, %v", d, ok)
		}
	}
}

func TestDrawWithRefusesZeroProbabilityPair(t *testing.T) {
	g := NewGenerator(correlatedVJ())
	if g.Genes("TRAV1*01", "TRAJ1*02") {
		t.Error("a zero-probability V–J pair is available")
	}
	if d, ok := g.DrawWith(random.New(1, 0), "TRAV1*01", "TRAJ1*02"); ok {
		t.Errorf("a zero-probability pair was drawn: %+v", d)
	}
}

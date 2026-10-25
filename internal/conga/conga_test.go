package conga

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/tcrdist"
)

func fixture() (*phenotype.Result, *expression.AnalysisResult, []clonotype.Clonotype) {
	d, _ := clonotype.Presets[3].Normalize()
	p := &phenotype.Result{Definition: d}
	m := &expression.AnalysisResult{Components: 2}
	cs := []clonotype.Clonotype{}
	for i, motif := range []string{"AAAA", "AAAS", "AASS", "SSSS", "CCCC", "CCCT", "CCTT", "TTTT"} {
		id := fmt.Sprint(i)
		cs = append(cs, clonotype.Clonotype{ID: id, Chains: []clonotype.Chain{{Locus: "TRA", Junction: "CAV" + motif + "AF", V: "TRAV1", J: "TRAJ1"}, {Locus: "TRB", Junction: "CAS" + motif + "TF", V: "TRBV1", J: "TRBJ1"}}})
		p.Cells = append(p.Cells, phenotype.Cell{Barcode: id, Clone: id, Status: "Assigned", Chains: "Paired"})
		m.Cells = append(m.Cells, expression.AnalysisCell{Barcode: id, Scores: []float64{float64(i), 0}})
	}
	return p, m, cs
}
func TestCongaDeterminismAndUnit(t *testing.T) {
	p, m, c := fixture()
	o := Options{CDR3Only: true, Neighbors: []int{1, 2, 3}}
	r, err := Compute(context.Background(), p, m, c, nil, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if digest != "717774abaae1fbf8eb80f0f4936e3cc4523ab8daf9d5b3de969eceb8a59b05bc" {
		t.Fatal("Conga digest", digest)
	}
	// Reordering sources/clone snapshots cannot change graph/statistics or floating summation.
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c[i], c[j] = c[j], c[i]
		p.Cells[i], p.Cells[j] = p.Cells[j], p.Cells[i]
		m.Cells[i], m.Cells[j] = m.Cells[j], m.Cells[i]
	}
	r2, err := Compute(context.Background(), p, m, c, nil, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Tests, r2.Tests) || !reflect.DeepEqual(r.Nodes, r2.Nodes) {
		t.Fatal("order changed inference")
	}
	// A duplicated profile within a clone changes cell count, not its vertex or tests.
	pc, mc := p.Cells[0], m.Cells[0]
	pc.Barcode = "extra"
	mc.Barcode = "extra"
	p.Cells = append(p.Cells, pc)
	m.Cells = append(m.Cells, mc)
	r2, err = Compute(context.Background(), p, m, c, nil, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Tests, r2.Tests) || r2.Eligible != r.Eligible+1 {
		t.Fatal("clone abundance changed inference")
	}
}
func TestCongaTiesExclusionsAndUntested(t *testing.T) {
	p, m, c := fixture()
	for i := range m.Cells {
		m.Cells[i].Scores = []float64{0, 0}
		m.Cells[i].Position = &[2]float64{float64(i) * 100, 0}
	}
	c[1].Chains[0].Junction = c[0].Chains[0].Junction
	p.Cells[2].Status = "Ambiguous clonotype"
	p.Cells[3].Chains = "Single arm"
	c[4].Chains[0].Junction = "CAVXXXAF"
	r, err := Compute(context.Background(), p, m, c, nil, Options{CDR3Only: true, Neighbors: []int{1, 9}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Eligible != 5 || r.Tested != 5 || r.Nodes[0].SharedChain != 1 {
		t.Fatal(r)
	}
	for _, x := range r.Tests {
		if x.K == 9 {
			if x.LogP != nil || x.Expected != nil || x.Overlap != nil || x.Reason == "" {
				t.Fatal(x)
			}
		} else if x.P == nil || *x.P != 1 || len(x.Expression) != r.Nodes[x.Node].Candidates {
			t.Fatal("ties produced false signal", x)
		}
	}
	if r.Cells[2].Reason != "Ambiguous clonotype" || r.Cells[3].Reason == "" || r.Cells[4].Reason == "" {
		t.Fatal(r.Cells)
	}
}
func TestCongaValidationAndCancellation(t *testing.T) {
	for _, edit := range []func(*phenotype.Result, *expression.AnalysisResult, []clonotype.Clonotype){
		func(p *phenotype.Result, m *expression.AnalysisResult, c []clonotype.Clonotype) {
			m.Cells[0].Scores[0] = math.NaN()
		},
		func(p *phenotype.Result, m *expression.AnalysisResult, c []clonotype.Clonotype) {
			m.Cells[0].Barcode = m.Cells[1].Barcode
		},
		func(p *phenotype.Result, m *expression.AnalysisResult, c []clonotype.Clonotype) {
			p.Cells[0].Barcode = p.Cells[1].Barcode
		},
		func(p *phenotype.Result, m *expression.AnalysisResult, c []clonotype.Clonotype) { c[0].ID = "absent" },
		func(p *phenotype.Result, m *expression.AnalysisResult, c []clonotype.Clonotype) {
			p.Definition.Vendor = true
		},
		func(p *phenotype.Result, m *expression.AnalysisResult, c []clonotype.Clonotype) { m.Components = 3 },
	} {
		p, m, c := fixture()
		edit(p, m, c)
		if _, err := Compute(context.Background(), p, m, c, nil, Options{CDR3Only: true}, nil); err == nil {
			t.Fatal("accepted bad input")
		}
	}
	for _, o := range []Options{{}, {Organism: "rat"}, {CDR3Only: true, Neighbors: []int{0}}, {CDR3Only: true, Neighbors: []int{1, 1}}, {CDR3Only: true, Neighbors: []int{201}}} {
		if _, err := o.Normalize(); err == nil {
			t.Fatal(o)
		}
	}
	p, m, c := fixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Compute(ctx, p, m, c, nil, Options{CDR3Only: true}, nil); err != context.Canceled {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if _, err := Compute(ctx, p, m, c, nil, Options{CDR3Only: true}, func(float64) { cancel() }); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestCongaFullLoopsAndUnderflow(t *testing.T) {
	l, err := tcrdist.ParseLoops(strings.NewReader("id\torganism\tregion\tcdrs\nTRAV1*01\thuman\tV\tAAAA;AAAA;AAAA\nTRAV2*01\thuman\tV\tCCCC;CCCC;CCCC\nTRBV1*01\thuman\tV\tAAAA;AAAA;AAAA\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, m, c := fixture()
	c[1].Chains[0].V = "TRAV2"
	c[2].Chains[0].V = "missing"
	r, err := Compute(context.Background(), p, m, c, l, Options{Organism: "human", Neighbors: []int{1}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Eligible != 7 || r.Cells[2].Reason == "" || r.Alleles["TRAV2"] != "TRAV2*01" {
		t.Fatal(r)
	}
	set := tcrdist.NewSet(c[:2], l, "human")
	cdr := tcrdist.NewSet(c[:2], nil, "")
	if set.Distance(0, 1)-cdr.Distance(0, 1) != 48 {
		t.Fatal("V-loop contribution missing")
	}
	lp := LogTail(2000, 1000, 1000, 1000)
	if !math.IsInf(lp, 0) && lp < -1300 && probability(lp) == nil {
		return
	}
	t.Fatal("underflow must retain finite log, null probability", lp)
}

func TestCongaBounds(t *testing.T) {
	build := func(n, dimensions int) (*phenotype.Result, *expression.AnalysisResult, []clonotype.Clonotype) {
		p, m, c := fixture()
		p.Cells, m.Cells, c = nil, nil, nil
		m.Components = dimensions
		const alphabet = "ARNDCQEGHILKMFPSTWYV"
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("%04d", i)
			motif := string([]byte{alphabet[i/400%20], alphabet[i/20%20], alphabet[i%20]})
			c = append(c, clonotype.Clonotype{ID: id, Chains: []clonotype.Chain{{Locus: "TRA", Junction: "CAV" + motif + "AF", V: "TRAV1", J: "TRAJ1"}, {Locus: "TRB", Junction: "CAS" + motif + "TF", V: "TRBV1", J: "TRBJ1"}}})
			p.Cells = append(p.Cells, phenotype.Cell{Barcode: id, Clone: id, Status: "Assigned", Chains: "Paired"})
			m.Cells = append(m.Cells, expression.AnalysisCell{Barcode: id, Scores: make([]float64, dimensions)})
		}
		return p, m, c
	}
	for _, tc := range []struct {
		n, dim int
		ks     []int
		want   string
	}{
		{2001, 2, []int{1}, "eligible paired clonotypes"},
		{700, 2, []int{1}, "work units"},
		{510, 2, []int{1, 2, 3, 4, 5, 6, 7, 8}, "neighbor output"},
	} {
		p, m, c := build(tc.n, tc.dim)
		_, err := Compute(context.Background(), p, m, c, nil, Options{CDR3Only: true, Neighbors: tc.ks}, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	p, m, c := fixture()
	m.Cells = make([]expression.AnalysisCell, MaxCells+1)
	if _, err := Compute(context.Background(), p, m, c, nil, Options{CDR3Only: true}, nil); err == nil {
		t.Fatal("cell bound")
	}
}

func TestCongaConcentratedNeighborhoods(t *testing.T) {
	p, m, c := fixture()
	p.Cells = nil
	m.Cells = nil
	c = nil
	const letters = "ARNDCQEGHI"
	for group, center := range []byte{'A', 'C', 'S', 'W'} {
		for j := 0; j < 10; j++ {
			id := fmt.Sprintf("%02d", group*10+j)
			motif := strings.Repeat(string(center), 8) + string(letters[j])
			c = append(c, clonotype.Clonotype{ID: id, Chains: []clonotype.Chain{{Locus: "TRA", Junction: "CAV" + motif + "AF", V: "TRAV1", J: "TRAJ1"}, {Locus: "TRB", Junction: "CAS" + motif + "TF", V: "TRBV1", J: "TRBJ1"}}})
			p.Cells = append(p.Cells, phenotype.Cell{Barcode: id, Clone: id, Status: "Assigned", Chains: "Paired"})
			m.Cells = append(m.Cells, expression.AnalysisCell{Barcode: id, Scores: []float64{float64(group) * 100, 0}})
		}
	}
	r, err := Compute(context.Background(), p, m, c, nil, Options{CDR3Only: true, Neighbors: []int{5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tested != 40 {
		t.Fatal(r.Tested)
	}
	for _, x := range r.Tests {
		if len(x.Expression) != 9 || len(x.Receptor) < 5 || len(x.Receptor) > 9 || x.Overlap == nil || *x.Overlap != len(x.Receptor) || x.Adjusted == nil || *x.Adjusted >= .05 {
			t.Fatal("concentrated graph control", x)
		}
	}
}

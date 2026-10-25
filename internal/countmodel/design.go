package countmodel

import (
	"errors"
	"sort"

	"clonoloom/internal/study"
)

type Matrix struct {
	Rows       []study.CohortRow `json:"rows"`
	Columns    []string          `json:"columns"`
	X          [][]float64       `json:"x"`
	Contrast   []float64         `json:"contrast"`
	ResidualDF int               `json:"residualDF"`
}

// Design makes an intercept, comparison indicator, paired subject blocks and optional
// treatment-coded batch columns. No row is deleted or redundant column silently dropped.
func Design(input study.CohortInput, batch bool) (Matrix, error) {
	d, err := study.CheckCohort(input)
	if err != nil {
		return Matrix{}, err
	}
	if !d.OK {
		return Matrix{}, errors.New("saved cohort design does not pass structural checks")
	}
	m := Matrix{Columns: []string{"Intercept", "Comparison minus reference"}}
	subjects, batches := map[string]bool{}, map[string]bool{}
	for _, r := range d.Rows {
		if r.Included {
			m.Rows = append(m.Rows, r)
			subjects[r.Subject] = true
			if batch {
				if r.Batch == "" {
					return m, errors.New("batch adjustment needs every selected batch annotation")
				}
				batches[r.Batch] = true
			}
		}
	}
	if len(m.Rows) > 96 {
		return m, errors.New("count models support at most 96 selected observations")
	}
	if d.Groups[0].Subjects < 12 || d.Groups[1].Subjects < 12 {
		return m, errors.New("count models require at least twelve subjects per selected group")
	}
	keys := func(v map[string]bool) []string {
		out := []string{}
		for k := range v {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	ss, bs := keys(subjects), keys(batches)
	if d.Input.Spec.Layout == "paired" {
		for _, s := range ss[1:] {
			m.Columns = append(m.Columns, "Subject: "+s)
		}
	}
	if batch {
		if len(bs) < 2 {
			return m, errors.New("batch adjustment needs at least two recorded batches")
		}
		for _, b := range bs[1:] {
			m.Columns = append(m.Columns, "Batch: "+b)
		}
	}
	p := len(m.Columns)
	if p > 40 {
		return m, errors.New("count model supports at most 40 coefficients")
	}
	m.ResidualDF = len(m.Rows) - p
	if m.ResidualDF < 24 {
		return m, errors.New("unmoderated dispersion estimation requires at least twenty-four residual degrees of freedom; add biological replication")
	}
	for _, r := range m.Rows {
		row := []float64{1, 0}
		if r.Group == d.Input.Spec.Levels[1] {
			row[1] = 1
		}
		if d.Input.Spec.Layout == "paired" {
			for _, s := range ss[1:] {
				v := 0.0
				if s == r.Subject {
					v = 1
				}
				row = append(row, v)
			}
		}
		if batch {
			for _, b := range bs[1:] {
				v := 0.0
				if b == r.Batch {
					v = 1
				}
				row = append(row, v)
			}
		}
		m.X = append(m.X, row)
	}
	mu := make([]float64, len(m.Rows))
	for i := range mu {
		mu[i] = 1
	}
	if _, _, err := factor(information(m.X, mu, 0)); err != nil {
		return m, errors.New("design is rank deficient: group, subject and batch effects cannot be separately estimated")
	}
	m.Contrast = make([]float64, p)
	m.Contrast[1] = 1
	return m, nil
}

package main

import (
	"clonoloom/internal/clonotype"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Typed predicates select rows, never recompute abundance or renormalize frequency.
// Multiple chains use existential matching. Missing chain fields match nothing,
// including equality with an empty string; negation inverts the row predicate.
type clonotypeFilter struct {
	number float64
	text   string
	All    []clonotypeFilter `json:"all,omitempty"`
	Any    []clonotypeFilter `json:"any,omitempty"`
	Not    *clonotypeFilter  `json:"not,omitempty"`
	Field  string            `json:"field,omitempty"`
	Op     string            `json:"op,omitempty"`
	Value  json.RawMessage   `json:"value,omitempty"`
}

func (f *clonotypeFilter) validate(depth int, nodes *int) error {
	if f == nil {
		return nil
	}
	*nodes++
	if depth > 8 || *nodes > 64 {
		return fmt.Errorf("filter exceeds depth 8 or 64 nodes")
	}
	forms := 0
	if f.All != nil {
		forms++
	}
	if f.Any != nil {
		forms++
	}
	if f.Not != nil {
		forms++
	}
	if f.Field != "" || f.Op != "" || f.Value != nil {
		forms++
	}
	if forms != 1 {
		return fmt.Errorf("filter needs exactly one of all, any, not, or field/op/value")
	}
	if f.All != nil || f.Any != nil {
		children := f.All
		if f.Any != nil {
			children = f.Any
		}
		if len(children) == 0 {
			return fmt.Errorf("empty boolean filter")
		}
		for i := range children {
			if err := children[i].validate(depth+1, nodes); err != nil {
				return err
			}
		}
		return nil
	}
	if f.Not != nil {
		return f.Not.validate(depth+1, nodes)
	}
	switch f.Field {
	case "total", "samples":
		if f.Op != "eq" && f.Op != "gte" && f.Op != "lte" {
			return fmt.Errorf("numeric operator must be eq, gte or lte")
		}
		var value any
		if err := json.Unmarshal(f.Value, &value); err != nil {
			return err
		}
		n, ok := value.(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 9007199254740991 {
			return fmt.Errorf("numeric value must be finite, nonnegative and at most 2^53-1")
		}
		f.number = n
	case "id", "junction", "v", "j", "locus":
		if f.Op != "eq" && f.Op != "contains" {
			return fmt.Errorf("text operator must be eq or contains")
		}
		var v string
		if err := json.Unmarshal(f.Value, &v); err != nil || len(v) == 0 || len(v) > 256 {
			return fmt.Errorf("text value needs 1–256 bytes")
		}
		f.text = v
	default:
		return fmt.Errorf("unknown filter field %q", f.Field)
	}
	return nil
}
func (f *clonotypeFilter) matches(t *clonotype.Table, i int) bool {
	if f == nil {
		return true
	}
	if f.All != nil {
		for _, c := range f.All {
			if !c.matches(t, i) {
				return false
			}
		}
		return true
	}
	if f.Any != nil {
		for _, c := range f.Any {
			if c.matches(t, i) {
				return true
			}
		}
		return false
	}
	if f.Not != nil {
		return !f.Not.matches(t, i)
	}
	c := t.Clonotypes[i]
	if f.Field == "total" || f.Field == "samples" {
		want := f.number
		got := c.Total
		if f.Field == "samples" {
			got = float64(c.Samples)
		}
		switch f.Op {
		case "eq":
			return got == want
		case "gte":
			return got >= want
		case "lte":
			return got <= want
		}
	}
	want := f.text
	hit := func(got string) bool {
		return got != "" && (f.Op == "eq" && got == want || f.Op == "contains" && strings.Contains(got, want))
	}
	if f.Field == "id" {
		return hit(c.ID)
	}
	for _, ch := range c.Chains {
		var got string
		switch f.Field {
		case "junction":
			got = ch.Junction
		case "v":
			got = ch.V
		case "j":
			got = ch.J
		case "locus":
			got = ch.Locus
		}
		if hit(got) {
			return true
		}
	}
	return false
}

func (f *clonotypeFilter) UnmarshalJSON(data []byte) error {
	type plain clonotypeFilter
	var value plain
	if err := strictJSON(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range []string{"all", "any", "not"} {
		if raw, ok := fields[key]; ok && string(raw) == "null" {
			return fmt.Errorf("%s cannot be null", key)
		}
	}
	*f = clonotypeFilter(value)
	return nil
}

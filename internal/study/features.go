package study

// Subject features join a checked design to saved sample measurements. No averaging,
// pooling or statistical fitting occurs here. Hill numbers and entropy have distinct
// units (Jost 2006, doi:10.1111/j.2006.0030-1299.14714.x); coverage is the existing
// Chao et al. 2014 estimate (doi:10.1890/13-0133.1), not fraction of clones observed.
import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"

	"clonoloom/internal/diversity"
)

const FeatureVersion = 1
const FeatureCitation = "Jost 2006, Entropy and diversity, doi:10.1111/j.2006.0030-1299.14714.x; Chao & Jost 2012, doi:10.1890/11-1952.1; Chao et al. 2014, doi:10.1890/13-0133.1. Selection, missingness and the fixed feature catalog are ClonoLoom policies."
const FeatureDescription = "One saved sample per subject and selected group, joined by exact sample name to a checksum-pinned diversity result. No pooling, averaging, depth normalization, imputation or group testing. Paired rows remain linked by subject. Observed richness and diversity depend on sampling depth; Hill numbers are effective clonotype counts, Shannon entropy uses natural logarithms. Coverage estimates abundance completeness, not the fraction of all clonotypes observed. The table carries point estimates only; parent intervals describe within-sample measurement uncertainty, not between-subject uncertainty. Empty samples retain zero abundance and richness; undefined values remain missing with reasons."

type Feature struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	Definition string `json:"definition"`
}

func FeatureCatalog(unit string) []Feature {
	return []Feature{
		{"abundance", "Assigned abundance", unit, "Sum of positive clonotype abundances after the parent definition and QC; not raw sequencing depth."},
		{"richness", "Observed clonotypes", "clonotypes", "Number S of clonotypes with positive abundance."},
		{"shannon", "Shannon entropy", "nats", "H = -sum(p ln p), p = clone abundance / assigned abundance."},
		{"hill1", "Hill diversity q=1", "effective clonotypes", "exp(H), observed plug-in estimate."},
		{"hill2", "Hill diversity q=2", "effective clonotypes", "1 / sum(p²), observed plug-in estimate."},
		{"evenness", "Pielou evenness", "dimensionless", "H / ln(S); undefined for S <= 1."},
		{"clonality", "Shannon clonality", "dimensionless", "1 - H / ln(S); undefined for S <= 1."},
		{"simpson", "Simpson concentration", "dimensionless", "sum(p²); larger means more concentrated."},
		{"giniSimpson", "Gini–Simpson index", "dimensionless", "1 - sum(p²); not an effective clonotype count."},
		{"gini", "Gini coefficient", "dimensionless", "sum over i,j of |x_i-x_j| / (2 S sum(x)); positive observed clones only, no finite-S rescaling."},
		{"maxFrequency", "Largest clone share", "fraction", "Maximum p among observed clones."},
		{"top10", "Top 10 clone share", "fraction", "Sum of p for the largest min(10,S) observed clones."},
		{"coverage", "Estimated sample coverage", "fraction", "Parent Chao sample-coverage estimate from singleton/doubleton counts; requires whole abundances."},
	}
}

type FeatureValue struct {
	Value   diversity.Number `json:"value"`
	Missing string           `json:"missing"`
}
type FeatureRow struct {
	CohortSample
	Group  string         `json:"group"`
	Values []FeatureValue `json:"values"` // fixed catalog order
}
type FeatureTable struct {
	Catalog []Feature    `json:"catalog"`
	Rows    []FeatureRow `json:"rows"`
}

// SubjectFeatures always rechecks the complete design. A parent must cover every
// design sample, including excluded ones, so an absent measurement cannot look like zero.
func SubjectFeatures(input CohortInput, samples map[string]diversity.Result, unit string) (FeatureTable, error) {
	design, err := CheckCohort(input)
	if err != nil {
		return FeatureTable{}, err
	}
	if !design.OK {
		return FeatureTable{}, errors.New("the saved design must pass structural checks")
	}
	switch unit {
	case "cells", "reads", "umis", "templates", "sequences":
	default:
		return FeatureTable{}, errors.New("unsupported abundance unit")
	}
	if len(samples) != len(design.Rows) {
		return FeatureTable{}, errors.New("diversity parent must cover the complete design sample set")
	}
	out := FeatureTable{Catalog: FeatureCatalog(unit), Rows: []FeatureRow{}}
	for _, row := range design.Rows {
		r, ok := samples[row.Name]
		if !ok {
			return FeatureTable{}, fmt.Errorf("sample %q absent from diversity parent", row.Name)
		}
		if math.IsNaN(r.Abundance) || math.IsInf(r.Abundance, 0) || r.Abundance < 0 || r.Clonotypes < 0 || (r.Abundance == 0) != (r.Clonotypes == 0) {
			return FeatureTable{}, fmt.Errorf("inconsistent abundance or richness for %q", row.Name)
		}
		if !row.Included {
			continue
		}
		v := FeatureRow{CohortSample: row.CohortSample, Group: row.Group, Values: make([]FeatureValue, len(out.Catalog))}
		for i, feature := range out.Catalog {
			x := FeatureValue{Value: diversity.Number(math.NaN())}
			switch {
			case feature.ID == "abundance":
				x.Value = diversity.Number(r.Abundance)
			case feature.ID == "richness":
				x.Value = diversity.Number(r.Clonotypes)
			case r.Abundance == 0:
				x.Missing = "No positive assigned abundance after definition and QC"
			case (feature.ID == "evenness" || feature.ID == "clonality") && r.Clonotypes == 1:
				x.Missing = "Undefined for a single observed clonotype"
			case feature.ID == "coverage" && !r.Whole:
				x.Missing = "Coverage requires whole abundances"
			default:
				m, exists := r.Measures[feature.ID]
				if !exists || math.IsNaN(float64(m.Estimate)) || math.IsInf(float64(m.Estimate), 0) {
					x.Missing = "Estimate unavailable in saved diversity result"
				} else {
					x.Value = m.Estimate
				}
			}
			v.Values[i] = x
		}
		out.Rows = append(out.Rows, v)
	}
	return out, nil
}

// FeatureCSV is a wide subject/group table. Empty numeric cells carry companion
// reasons; exact round-trip decimals preserve the saved Go estimates.
func FeatureCSV(w io.Writer, t FeatureTable) error {
	c := csv.NewWriter(w)
	header := []string{"sample", "subject", "group", "condition", "timepoint", "tissue", "cell_subset", "batch"}
	for _, f := range t.Catalog {
		header = append(header, f.ID, f.ID+"_missing")
	}
	if err := c.Write(header); err != nil {
		return err
	}
	for _, r := range t.Rows {
		row := []string{r.Name, r.Subject, r.Group, r.Condition, r.Timepoint, r.Tissue, r.CellSubset, r.Batch}
		for _, x := range r.Values {
			value := ""
			if !math.IsNaN(float64(x.Value)) && !math.IsInf(float64(x.Value), 0) {
				value = strconv.FormatFloat(float64(x.Value), 'g', -1, 64)
			}
			row = append(row, value, x.Missing)
		}
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

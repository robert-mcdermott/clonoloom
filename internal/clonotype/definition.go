// Package clonotype groups rearrangements (bulk) or cells (single cell) into clonotypes under an
// explicit, named definition (clonoloom-spec/requirements.md, C1–C3, C6), so that every result
// can say which definition it used. A clonotype's ID is derived from the definition and the
// clonotype's key, so the same clonotype has the same ID in every run and on every computer.
package clonotype

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"clonoloom/internal/genes"
)

// Light is the light chain of a B cell, whichever of IGK and IGL it is.
const Light = "light"

// Definition is how rearrangements are grouped into clonotypes.
type Definition struct {
	// Name is how the user calls it; it is not part of what the definition means (renaming a
	// definition does not change its clonotypes' IDs).
	Name string `json:"name,omitempty"`
	// Chains: one locus ("TRB"), a pair ("TRA", "TRB"; "IGH", "light"; "TRG", "TRD"), or none for
	// every locus, each separately (bulk data of several loci).
	Chains []string `json:"chains"`
	// Junction: "aa" (amino acids; productive rearrangements only) or "nt" (nucleotides).
	Junction string `json:"junction"`
	// V and J: "gene", "allele", "family" or "none" (not used).
	V string `json:"v"`
	J string `json:"j"`
	// ProductiveOnly: only productive rearrangements (always, for amino-acid junctions).
	ProductiveOnly bool `json:"productiveOnly"`
	// Cells (single cell): "primary", the chain of each locus with the most UMIs (then reads), or
	// "all", every productive chain of each locus.
	Cells string `json:"cells,omitempty"`
	// RequireAll (single cell, paired chains): a cell must have every chain of the definition;
	// otherwise a cell with some of them forms clonotypes with cells of the same chains.
	RequireAll bool `json:"requireAll,omitempty"`
	// Vendor: the source's own clonotypes (Cell Ranger's raw_clonotype_id, an AIRR clone_id), as
	// given.
	Vendor bool `json:"vendor,omitempty"`
}

// Level reads a definition's V or J resolution.
func Level(s string) (genes.Level, error) {
	switch s {
	case "gene", "":
		return genes.LevelGene, nil
	case "allele":
		return genes.LevelAllele, nil
	case "family":
		return genes.LevelFamily, nil
	case "none":
		return genes.LevelNone, nil
	}
	return 0, fmt.Errorf("V and J are compared by gene, allele, family or none, not %q", s)
}

// Normalize checks a definition and fills its defaults: amino-acid junctions, V and J genes,
// primary chains.
func (d Definition) Normalize() (Definition, error) {
	if d.Junction == "" {
		d.Junction = "aa"
	}
	if d.Junction != "aa" && d.Junction != "nt" {
		return d, fmt.Errorf("the junction is compared as amino acids (aa) or nucleotides (nt), not %q", d.Junction)
	}
	if d.V == "" {
		d.V = "gene"
	}
	if d.J == "" {
		d.J = "gene"
	}
	if _, err := Level(d.V); err != nil {
		return d, err
	}
	if _, err := Level(d.J); err != nil {
		return d, err
	}
	if d.Junction == "aa" {
		d.ProductiveOnly = true
	}
	if d.Cells == "" {
		d.Cells = "primary"
	}
	if d.Cells != "primary" && d.Cells != "all" {
		return d, fmt.Errorf("a cell's chains are its primary ones or all, not %q", d.Cells)
	}
	if len(d.Chains) > 2 {
		return d, fmt.Errorf("a clonotype has one or two chains, not %d", len(d.Chains))
	}
	seen := map[string]bool{}
	for _, c := range d.Chains {
		if c != Light && !isLocus(c) {
			return d, fmt.Errorf("%q is not a locus", c)
		}
		if seen[c] {
			return d, fmt.Errorf("the chain %s is named twice", c)
		}
		seen[c] = true
	}
	return d, nil
}

func isLocus(s string) bool {
	for _, l := range genes.Loci {
		if l == s {
			return true
		}
	}
	return false
}

// Canonical is the definition's meaning as text: the same for definitions that group alike,
// whatever their names. Clonotype IDs are made from it.
func (d Definition) Canonical() string {
	m := map[string]any{"chains": d.Chains, "junction": d.Junction, "v": d.V, "j": d.J, "productiveOnly": d.ProductiveOnly, "cells": d.Cells, "requireAll": d.RequireAll, "vendor": d.Vendor}
	if d.Chains == nil {
		m["chains"] = []string{}
	}
	b, _ := json.Marshal(m) // keys in sorted order
	return string(b)
}

// ID of a definition: a short hash of its canonical form.
func (d Definition) ID() string {
	h := sha256.Sum256([]byte("clonoloom-definition/1\n" + d.Canonical()))
	return "def-" + hex.EncodeToString(h[:])[:12]
}

// clonotypeID is a clonotype's ID: a hash of the definition's canonical form and its key.
func clonotypeID(canonical, key string) string {
	h := sha256.New()
	h.Write([]byte("clonoloom-clonotype/1\n"))
	h.Write([]byte(canonical))
	h.Write([]byte{'\n'})
	h.Write([]byte(key))
	return "c-" + hex.EncodeToString(h.Sum(nil))[:16]
}

// Describe says the definition in words, as methods text and legends say it.
func (d Definition) Describe() string {
	if d.Vendor {
		return "the source's own clonotypes (Cell Ranger's raw_clonotype_id, or the AIRR clone_id), as given"
	}
	var b strings.Builder
	chains := "each locus separately"
	switch len(d.Chains) {
	case 1:
		chains = d.Chains[0] + " chains"
	case 2:
		chains = "paired " + d.Chains[0] + " and " + d.Chains[1] + " chains"
	}
	junction := "amino-acid junction"
	if d.Junction == "nt" {
		junction = "nucleotide junction"
	}
	b.WriteString(fmt.Sprintf("%s by %s", chains, junction))
	parts := []string{}
	for _, g := range []struct{ name, level string }{{"V", d.V}, {"J", d.J}} {
		switch g.level {
		case "gene":
			parts = append(parts, g.name+" gene")
		case "allele":
			parts = append(parts, g.name+" allele")
		case "family":
			parts = append(parts, g.name+" family")
		}
	}
	if len(parts) > 0 {
		b.WriteString(", " + strings.Join(parts, " and "))
	}
	if d.ProductiveOnly && d.Junction == "nt" {
		b.WriteString(", productive rearrangements only")
	}
	if len(d.Chains) == 2 {
		if d.Cells == "all" {
			b.WriteString("; single cells by every productive chain")
		} else {
			b.WriteString("; single cells by their primary chain of each locus (most UMIs)")
		}
		if d.RequireAll {
			b.WriteString(", cells without both chains left out")
		} else {
			b.WriteString(", cells with one chain grouped with cells of the same chain alone")
		}
	}
	return b.String()
}

// Presets: the definitions most studies use.
var Presets = []Definition{
	{Name: "Amino-acid junction, V and J genes", Chains: nil, Junction: "aa", V: "gene", J: "gene"},
	{Name: "Nucleotide junction, V and J genes", Chains: nil, Junction: "nt", V: "gene", J: "gene", ProductiveOnly: true},
	{Name: "Amino-acid junction only", Chains: nil, Junction: "aa", V: "none", J: "none"},
	{Name: "Paired αβ: amino-acid junctions, V and J genes", Chains: []string{"TRA", "TRB"}, Junction: "aa", V: "gene", J: "gene", Cells: "primary"},
	{Name: "Paired heavy and light: amino-acid junctions, V and J genes", Chains: []string{"IGH", Light}, Junction: "aa", V: "gene", J: "gene", Cells: "primary"},
	{Name: "The source's own clonotypes", Vendor: true},
}

// KnownLocus answers whether s is a locus ClonoLoom knows (TRA, TRB, TRG, TRD, IGH, IGK, IGL).
func KnownLocus(s string) bool { return isLocus(s) }

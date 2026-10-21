// Package airr holds what ClonoLoom uses of the AIRR Community's standards (Vander Heiden et al.
// 2018, Front Immunol 9:2206, doi:10.3389/fimmu.2018.02206; https://docs.airr-community.org): the
// Rearrangement schema's fields and their types, how a TSV writes booleans, numbers and missing
// values, and the Repertoire metadata (MiAIRR; Rubelt et al. 2017, Nat Immunol 18:1274,
// doi:10.1038/ni.3873). The field names and types are those the AIRR schema defines (CC BY 4.0,
// the AIRR Community).
package airr

import (
	"math"
	"strconv"
	"strings"
)

// Type of a schema field.
type Type byte

const (
	String  Type = 's'
	Boolean Type = 'b'
	Integer Type = 'i'
	Number  Type = 'n'
)

// Required are the Rearrangement fields every AIRR TSV must have as columns (their values may be
// missing), schema 1.3 to 1.5.
var Required = []string{
	"sequence_id", "sequence", "rev_comp", "productive", "v_call", "d_call", "j_call",
	"sequence_alignment", "germline_alignment", "junction", "junction_aa", "v_cigar", "d_cigar", "j_cigar",
}

// Fields are the Rearrangement schema's fields with their types (1.3 to 1.5; fields added later
// are added here as they are used). A column not listed is kept as text.
var Fields = func() map[string]Type {
	f := map[string]Type{}
	set := func(t Type, names ...string) {
		for _, n := range names {
			f[n] = t
		}
	}
	set(String, "sequence_id", "sequence", "quality", "sequence_aa", "locus", "v_call", "d_call", "d2_call", "j_call", "c_call",
		"sequence_alignment", "sequence_alignment_aa", "quality_alignment", "germline_alignment", "germline_alignment_aa",
		"junction", "junction_aa", "np1", "np1_aa", "np2", "np2_aa", "np3", "np3_aa",
		"cdr1", "cdr1_aa", "cdr2", "cdr2_aa", "cdr3", "cdr3_aa", "fwr1", "fwr1_aa", "fwr2", "fwr2_aa", "fwr3", "fwr3_aa", "fwr4", "fwr4_aa",
		"v_cigar", "d_cigar", "d2_cigar", "j_cigar", "c_cigar",
		"v_sequence_alignment", "v_sequence_alignment_aa", "d_sequence_alignment", "d_sequence_alignment_aa",
		"d2_sequence_alignment", "d2_sequence_alignment_aa", "j_sequence_alignment", "j_sequence_alignment_aa",
		"c_sequence_alignment", "c_sequence_alignment_aa", "v_germline_alignment", "v_germline_alignment_aa",
		"d_germline_alignment", "d_germline_alignment_aa", "d2_germline_alignment", "d2_germline_alignment_aa",
		"j_germline_alignment", "j_germline_alignment_aa", "c_germline_alignment", "c_germline_alignment_aa",
		"cell_id", "clone_id", "repertoire_id", "sample_processing_id", "data_processing_id", "rearrangement_id",
		"rearrangement_set_id", "germline_database")
	set(Boolean, "rev_comp", "productive", "vj_in_frame", "stop_codon", "complete_vdj", "v_frameshift", "j_frameshift")
	set(Number, "v_score", "v_identity", "v_support", "d_score", "d_identity", "d_support", "d2_score", "d2_identity",
		"d2_support", "j_score", "j_identity", "j_support", "c_score", "c_identity", "c_support")
	set(Integer, "v_sequence_start", "v_sequence_end", "v_germline_start", "v_germline_end", "v_alignment_start", "v_alignment_end",
		"d_sequence_start", "d_sequence_end", "d_germline_start", "d_germline_end", "d_alignment_start", "d_alignment_end",
		"d2_sequence_start", "d2_sequence_end", "d2_germline_start", "d2_germline_end", "d2_alignment_start", "d2_alignment_end",
		"j_sequence_start", "j_sequence_end", "j_germline_start", "j_germline_end", "j_alignment_start", "j_alignment_end",
		"c_sequence_start", "c_sequence_end", "c_germline_start", "c_germline_end", "c_alignment_start", "c_alignment_end",
		"cdr1_start", "cdr1_end", "cdr2_start", "cdr2_end", "cdr3_start", "cdr3_end",
		"fwr1_start", "fwr1_end", "fwr2_start", "fwr2_end", "fwr3_start", "fwr3_end", "fwr4_start", "fwr4_end",
		"junction_length", "junction_aa_length", "np1_length", "np2_length", "np3_length",
		"n1_length", "n2_length", "n3_length", "p3v_length", "p5d_length", "p3d_length", "p5d2_length", "p3d2_length", "p5j_length",
		"d_frame", "d2_frame", "consensus_count", "duplicate_count", "umi_count")
	return f
}()

// ParseBool reads an AIRR boolean as the AIRR libraries accept it: T, F, TRUE, FALSE, True, False,
// t, f, 1 and 0. "" is missing (ok, missing); anything else is not a boolean (ok false).
func ParseBool(s string) (value, missing, ok bool) {
	switch s {
	case "":
		return false, true, true
	case "T", "TRUE", "True", "true", "t", "1":
		return true, false, true
	case "F", "FALSE", "False", "false", "f", "0":
		return false, false, true
	}
	return false, false, false
}

// ParseInt reads an AIRR integer. "" is missing. An integral value written as a decimal ("12.0")
// is read, with exact false so the validator can note it; anything else is not an integer.
func ParseInt(s string) (value int64, missing, exact, ok bool) {
	if s == "" {
		return 0, true, true, true
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, false, true, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, false, false, false
	}
	return int64(f), false, false, true
}

// ParseNumber reads an AIRR number. "" is missing; "NaN", "nan" and "NA" are read as missing too.
func ParseNumber(s string) (value float64, missing, ok bool) {
	switch strings.ToLower(s) {
	case "", "nan", "na":
		return math.NaN(), true, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) {
		return math.NaN(), false, false
	}
	return f, false, true
}

// FormatBool writes an AIRR boolean as the standard does: T or F, "" when missing.
func FormatBool(value, missing bool) string {
	switch {
	case missing:
		return ""
	case value:
		return "T"
	}
	return "F"
}

// Package seq holds what ClonoLoom needs of nucleotide and amino-acid sequences: the standard
// genetic code, translation, reverse complements, and the checks a junction must pass.
//
// The junction is IMGT's (Lefranc et al. 2003, Dev Comp Immunol 27:55–77,
// doi:10.1016/S0145-305X(02)00039-3): the CDR3 (IMGT positions 105–117) with the conserved
// 2nd-CYS 104 before it and J-PHE or J-TRP 118 after it. The AIRR Rearrangement schema's
// `junction` and `junction_aa` follow it.
package seq

import "strings"

// The standard genetic code (NCBI translation table 1), codons in TCAG order.
const (
	bases    = "TCAG"
	aminoTCA = "FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG"
)

var baseIndex = func() [256]int8 {
	var index [256]int8
	for i := range index {
		index[i] = -1
	}
	for i := 0; i < 4; i++ {
		index[bases[i]] = int8(i)
		index[bases[i]+'a'-'A'] = int8(i)
	}
	index['U'], index['u'] = 0, 0
	return index
}()

// Codon is the amino acid a codon encodes ('*' for a stop), or 'X' when the codon has a base
// other than A, C, G, T or U (an N, a gap, an ambiguity code).
func Codon(a, b, c byte) byte {
	i, j, k := baseIndex[a], baseIndex[b], baseIndex[c]
	if i < 0 || j < 0 || k < 0 {
		return 'X'
	}
	return aminoTCA[int(i)*16+int(j)*4+int(k)]
}

// Translate translates nt from its first base in frame 1. A last codon that is not whole is
// dropped; codons with other bases than A, C, G, T or U become 'X'.
func Translate(nt string) string {
	n := len(nt) / 3
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = Codon(nt[3*i], nt[3*i+1], nt[3*i+2])
	}
	return string(out)
}

// ReverseComplement returns the reverse complement of nt, keeping the case of each base. IUPAC
// ambiguity codes are complemented; anything else is kept as it is.
func ReverseComplement(nt string) string {
	out := make([]byte, len(nt))
	for i := 0; i < len(nt); i++ {
		out[len(nt)-1-i] = complement[nt[i]]
	}
	return string(out)
}

var complement = func() [256]byte {
	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	pairs := "ATTAUAGCCGRYYRKMMKSSWWBVVBDHHDNN"
	for i := 0; i < len(pairs); i += 2 {
		table[pairs[i]] = pairs[i+1]
		table[pairs[i]+'a'-'A'] = pairs[i+1] + 'a' - 'A'
	}
	return table
}()

// IsNucleotide reports whether s is a non-empty sequence of A, C, G, T, U or N (either case).
func IsNucleotide(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'A', 'C', 'G', 'T', 'U', 'N', 'a', 'c', 'g', 't', 'u', 'n':
		default:
			return false
		}
	}
	return true
}

// The twenty standard amino acids, in alphabetical order of their one-letter codes.
const AminoAcids = "ACDEFGHIKLMNPQRSTVWY"

// IsProtein reports whether s is a non-empty sequence of the twenty standard amino acids, X
// (unknown) and * (a stop), in upper case.
func IsProtein(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(AminoAcids+"X*", rune(s[i])) {
			return false
		}
	}
	return true
}

// HasStop reports whether an amino-acid sequence has a stop codon ('*').
func HasStop(aa string) bool {
	return strings.IndexByte(aa, '*') >= 0
}

// Anchors says whether a junction's amino acids begin with the conserved cysteine (2nd-CYS 104)
// and end with the conserved phenylalanine or tryptophan (J-PHE or J-TRP 118). Heavy chains end in
// tryptophan (W) and the other loci in phenylalanine (F); either is accepted here, as germline
// genes of every locus have exceptions, and the validator counts each kind.
func Anchors(junctionAA string) (cys, pheTrp bool) {
	if len(junctionAA) == 0 {
		return false, false
	}
	last := junctionAA[len(junctionAA)-1]
	return junctionAA[0] == 'C', last == 'F' || last == 'W'
}

// CDR3 is the IMGT CDR3 of a junction: the junction without its first and last residues (or
// codons). A junction shorter than two residues has no CDR3.
func CDR3(junction string, codon int) string {
	if len(junction) < 2*codon {
		return ""
	}
	return junction[codon : len(junction)-codon]
}

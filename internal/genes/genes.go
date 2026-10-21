// Package genes reads immune receptor gene names: IMGT nomenclature (Lefranc 2014, Front Immunol
// 5:22, doi:10.3389/fimmu.2014.00022; the WHO-IUIS/IMGT nomenclature used by the AIRR Community),
// the calls annotation tools write (one name, or several separated by commas when a segment could
// not be told apart), and vendors' own names, which are mapped to IMGT's (adaptive.go).
//
// A name such as "TRBV5-1*01" is read as:
//
//	locus    TRB         the chain's locus: TRA, TRB, TRG, TRD, IGH, IGK or IGL
//	segment  V           V, D, J or C
//	family   TRBV5       IMGT's subgroup: the name up to the first '-' or '/'
//	gene     TRBV5-1     the name without its allele
//	allele   01          after '*'; "" when not given
//
// Shared TRA/TRD genes ("TRAV29/DV5") and orphons ("TRBV20/OR9-2") keep their full names.
package genes

import (
	"sort"
	"strings"
)

// Segment is a gene segment's kind.
type Segment byte

const (
	SegmentUnknown Segment = 0
	V              Segment = 'V'
	D              Segment = 'D'
	J              Segment = 'J'
	C              Segment = 'C'
)

// Loci, as IMGT and the AIRR schema's `locus` field name them.
var Loci = []string{"IGH", "IGK", "IGL", "TRA", "TRB", "TRD", "TRG"}

// Name is a gene name read into its parts. The zero Name is "no call".
type Name struct {
	Raw     string // as given (trimmed)
	Locus   string // "TRB"; "" when the name is not an immune receptor gene
	Segment Segment
	Family  string // "TRBV5"
	Gene    string // "TRBV5-1"
	Allele  string // "01"; "" when not given
}

// Constant-region genes of the heavy chain named without a number or with a subclass: IgM, IgD,
// IgG1–4, IgA1–2, IgE (and mouse IgG2a/2b/2c, IgG3). "IGHD" alone is the δ constant gene, not a
// diversity segment.
var heavyConstant = map[string]bool{
	"IGHM": true, "IGHD": true, "IGHE": true, "IGHA": true, "IGHA1": true, "IGHA2": true,
	"IGHG": true, "IGHG1": true, "IGHG2": true, "IGHG3": true, "IGHG4": true,
	"IGHG2A": true, "IGHG2B": true, "IGHG2C": true, "IGHEP1": true, "IGHGP": true,
}

// Parse reads one gene name. A name it does not recognize as an immune receptor gene (or an
// empty or "None" value) comes back with an empty Locus, and Raw as given.
func Parse(raw string) Name {
	raw = strings.TrimSpace(raw)
	n := Name{Raw: raw}
	if isMissing(raw) {
		n.Raw = ""
		return n
	}
	name, allele, _ := strings.Cut(raw, "*")
	// MiXCR writes the allele's score in parentheses ("TRBV5-1*00(1250)"); IgBLAST may give an
	// allele "00" for an unknown one.
	if i := strings.IndexByte(allele, '('); i >= 0 {
		allele = allele[:i]
	}
	name = strings.ToUpper(name)
	if len(name) < 4 {
		return n
	}
	locus := name[:3]
	if !isLocus(locus) {
		return n
	}
	var segment Segment
	switch {
	case locus == "IGH" && heavyConstant[name]:
		segment = C
	case name[3] == 'V' || name[3] == 'D' || name[3] == 'J' || name[3] == 'C':
		segment = Segment(name[3])
	default:
		return n
	}
	n.Locus, n.Segment, n.Gene, n.Allele = locus, segment, name, allele
	n.Family = family(name, segment)
	return n
}

// family is IMGT's subgroup of a V, D or J gene: the name up to its first '-' or '/', with the
// "D" that marks a duplicated gene of the distal kappa cluster (IGKV1D-39) or the second copy of a
// heavy-chain gene (IGHV1-69D) removed. Constant genes are their own family.
func family(gene string, segment Segment) string {
	if segment == C {
		return gene
	}
	end := len(gene)
	if i := strings.IndexAny(gene, "-/"); i >= 0 {
		end = i
	}
	f := gene[:end]
	if len(f) > 5 && f[len(f)-1] == 'D' && isDigit(f[len(f)-2]) {
		f = f[:len(f)-1]
	}
	return f
}

func isLocus(s string) bool {
	for _, l := range Loci {
		if s == l {
			return true
		}
	}
	return false
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isMissing reports whether a value means "no call" as files write it.
func isMissing(s string) bool {
	switch strings.ToLower(s) {
	case "", "none", "na", "nan", "null", "unresolved", "-", ".":
		return true
	}
	return false
}

// Call is an annotation tool's call of one segment: one or more names, best first, as an AIRR
// file writes them ("TRBV6-2*01,TRBV6-3*01"). Names that are not immune receptor genes are kept
// out of Names and counted in Unreadable.
type Call struct {
	Names      []Name
	Unreadable int
}

// ParseCall reads a call: names separated by commas (AIRR, IgBLAST, Change-O), or by '|' or
// whitespace as some tools write them.
func ParseCall(raw string) Call {
	var c Call
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '|' || r == ' ' || r == ';' }) {
		n := Parse(part)
		if n.Raw == "" {
			continue
		}
		if n.Locus == "" {
			c.Unreadable++
			continue
		}
		c.Names = append(c.Names, n)
	}
	return c
}

// Level is how finely a clonotype definition tells genes apart.
type Level int

const (
	LevelNone   Level = iota // not used
	LevelFamily              // TRBV5
	LevelGene                // TRBV5-1
	LevelAllele              // TRBV5-1*01
)

// Key is the call at a level: the distinct names at that level in sorted order, joined by '|'
// ("TRBV6-2|TRBV6-3" for a call between two genes). A call is resolved at a level when its key
// has one name. LevelNone and an empty call give "".
func (c Call) Key(level Level) string {
	if level == LevelNone || len(c.Names) == 0 {
		return ""
	}
	seen := make(map[string]bool, len(c.Names))
	var keys []string
	for _, n := range c.Names {
		k := n.At(level)
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	if len(keys) > 1 {
		sort.Strings(keys)
	}
	return strings.Join(keys, "|")
}

// Resolved reports whether the call names one gene (or family, or allele) at the level.
func (c Call) Resolved(level Level) bool {
	return len(c.Names) > 0 && !strings.Contains(c.Key(level), "|")
}

// At is the name at a level: its family, its gene, or its gene and allele ("TRBV5-1*01"; the gene
// alone when no allele was given).
func (n Name) At(level Level) string {
	switch level {
	case LevelFamily:
		return n.Family
	case LevelGene:
		return n.Gene
	case LevelAllele:
		if n.Allele == "" {
			return n.Gene
		}
		return n.Gene + "*" + n.Allele
	}
	return ""
}

// Locus of a call: the locus all its names share, or "" when they disagree or there are none.
func (c Call) Locus() string {
	locus := ""
	for i, n := range c.Names {
		if i == 0 {
			locus = n.Locus
		} else if n.Locus != locus {
			return ""
		}
	}
	return locus
}

// Chain type of a locus: "TR" or "IG".
func Receptor(locus string) string {
	if strings.HasPrefix(locus, "TR") {
		return "TR"
	}
	if strings.HasPrefix(locus, "IG") {
		return "IG"
	}
	return ""
}

// HeavyOrBeta reports whether a locus is the chain with a D segment (IGH, TRB, TRD).
func HeavyOrBeta(locus string) bool {
	return locus == "IGH" || locus == "TRB" || locus == "TRD"
}

package genes

import (
	"regexp"
	"strconv"
	"strings"
)

// Adaptive Biotechnologies' immunoSEQ names genes its own way: "TCRBV05-01*01" for IMGT's
// "TRBV5-1*01", with the locus spelled TCRB, TCRA, TCRG or TCRD, family and gene numbers
// zero-padded to two digits, a gene number even where IMGT's subgroup has one gene
// ("TCRBV09-01" for "TRBV9", "TCRBD01-01" for "TRBD1"), and orphons as "TCRBV20-or09_02" for
// "TRBV20/OR9-2". FromAdaptive maps them by these rules; for human genes the gene number is
// dropped when IMGT's subgroup has one gene, by the list of human genes (human.go). Mouse names
// keep their gene numbers (ClonoLoom's choice until a mouse gene list is added: IMGT's mouse
// subgroups with one gene are named without a number too, so such calls keep a "-1" IMGT does not
// write).

var adaptiveCombined = regexp.MustCompile(`^(TCR[ABGD][VDJ]|IG[HKL][VDJ])([0-9]+-[0-9]+)/([0-9]+-[0-9]+)$`)

var adaptiveName = regexp.MustCompile(`^(TCR[ABGD]|IG[HKL])([VDJC])([0-9]+|[A-Z])(?:-([0-9]+)|-or([0-9]+)_([0-9]+))?$`)

var adaptiveLocus = map[string]string{"TCRA": "TRA", "TCRB": "TRB", "TCRG": "TRG", "TCRD": "TRD", "IGH": "IGH", "IGK": "IGK", "IGL": "IGL"}

// FromAdaptive maps one Adaptive gene name (with or without an allele) to IMGT's. ok is false for
// a name it does not recognize, which comes back as given; "" and "unresolved" come back as "".
func FromAdaptive(name string, human bool) (imgt string, ok bool) {
	name = strings.TrimSpace(name)
	if isMissing(name) {
		return "", true
	}
	base, allele, hasAllele := strings.Cut(name, "*")
	// "TCRBV20-X": the family is known, the gene is not.
	base = strings.TrimSuffix(base, "-X")
	m := adaptiveName.FindStringSubmatch(strings.ToUpper(base[:min(len(base), 6)]) + base[min(len(base), 6):])
	if m == nil {
		return name, false
	}
	locus := adaptiveLocus[m[1]]
	family := m[3]
	if n, err := strconv.Atoi(family); err == nil {
		family = strconv.Itoa(n)
	}
	gene := locus + m[2] + family
	switch {
	case m[4] != "":
		n, _ := strconv.Atoi(m[4])
		numbered := gene + "-" + strconv.Itoa(n)
		if human && n == 1 && !humanGenes.has(numbered) && (humanGenes.has(gene) || humanGenes.sharedName(gene) != "") {
			// IMGT names a subgroup with one gene without a number ("TRBV9", "TRAV29/DV5").
			break
		}
		gene = numbered
	case m[5] != "":
		chromosome, _ := strconv.Atoi(m[5])
		n, _ := strconv.Atoi(m[6])
		gene += "/OR" + strconv.Itoa(chromosome) + "-" + strconv.Itoa(n)
	}
	if human && !humanGenes.has(gene) {
		// TRAV and TRDV genes shared by the two loci carry both names in IMGT's ("TRAV29/DV5").
		if shared := humanGenes.sharedName(gene); shared != "" {
			gene = shared
		}
	}
	if hasAllele && allele != "" {
		if n, err := strconv.Atoi(allele); err == nil {
			allele = twoDigits(n)
		}
		return gene + "*" + allele, true
	}
	return gene, true
}

// FromAdaptiveCall maps a list of Adaptive names (immunoSEQ's ties, separated by commas) to an
// IMGT call, its names separated by commas as an AIRR call is. A combined name between two genes
// of a family ("TCRBV12-03/12-04") is the two genes.
func FromAdaptiveCall(names string, human bool) (string, bool) {
	var out []string
	allOK := true
	var parts []string
	for _, part := range strings.Split(names, ",") {
		part = strings.TrimSpace(part)
		if m := adaptiveCombined.FindStringSubmatch(part); m != nil {
			parts = append(parts, m[1]+m[2], m[1]+m[3])
			continue
		}
		parts = append(parts, part)
	}
	for _, part := range parts {
		if part == "" {
			continue
		}
		g, ok := FromAdaptive(part, human)
		allOK = allOK && ok
		if g != "" {
			out = append(out, g)
		}
	}
	return strings.Join(out, ","), allOK
}

func twoDigits(n int) string {
	if n < 10 && n >= 0 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

type geneSet map[string]bool

func (s geneSet) has(name string) bool {
	_, ok := s[name]
	return ok
}

// sharedName: the human gene named name + "/DV…" (TRAV14 → TRAV14/DV4), or "".
func (s geneSet) sharedName(name string) string {
	for g := range s {
		if strings.HasPrefix(g, name+"/DV") {
			return g
		}
	}
	return ""
}

// IsHumanGene reports whether name (without an allele) is a human TR or IG gene in IMGT's
// spelling, and whether it is a pseudogene.
func IsHumanGene(name string) (known, pseudo bool) {
	pseudo, known = humanGenes[name]
	return known, pseudo
}

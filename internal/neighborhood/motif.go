package neighborhood

import (
	"sort"
	"strings"

	"clonoloom/internal/tcrdist"
)

// MaxAmbiguity is how many residues a position of a motif may name before it is written as "any"
// (tcrdist3's code uses 3, Mayer-Blackwell et al.'s paper 5).
const MaxAmbiguity = 3

// Motif is a neighborhood's pattern: at each position of the centroid's CDR3 that TCRdist
// compares (its first three and last two residues left out, as the distance leaves them out), the
// residues its neighbors have there, aligned to the centroid as the distance aligned them. A
// position is written as its residue, as a set of up to MaxAmbiguity of them in brackets, or as a
// dot where there are more; a position some neighbor has no residue at (its CDR3 is shorter) is
// followed by a question mark, as a pattern that may match without it.
//
// The pattern is written as a regular expression over the trimmed CDR3, so that it can be read
// beside tcrdist3's, and the view shows it between the centroid's own trimmed ends.
func Motif(sample *Repertoire, members []int, self *pairing) string {
	if len(members) == 0 {
		return ""
	}
	centroid := self.a[members[0]]
	const ntrim, ctrim = 3, 2
	if len(centroid.CDR3) <= ntrim+ctrim {
		return ""
	}
	// At each position, the residues the neighbors align against it, and whether any has none.
	at := make([]map[byte]bool, len(centroid.CDR3))
	gap := make([]bool, len(centroid.CDR3))
	for i := range at {
		at[i] = map[byte]bool{}
	}
	for _, m := range members {
		other := sample.Receptors[m].CDR3
		for i, k := range tcrdist.AlignCDR3(centroid, self.a[m]) {
			if k < 0 {
				gap[i] = true
				continue
			}
			at[i][other[k]] = true
		}
	}

	var b strings.Builder
	b.WriteByte('(')
	for i := ntrim; i < len(centroid.CDR3)-ctrim; i++ {
		residues := make([]byte, 0, len(at[i]))
		for r := range at[i] {
			residues = append(residues, r)
		}
		sort.Slice(residues, func(x, y int) bool { return residues[x] < residues[y] })
		switch {
		case len(residues) == 0:
			b.WriteString(".?")
			continue
		case len(residues) == 1:
			b.WriteByte(residues[0])
		case len(residues) <= MaxAmbiguity:
			b.WriteByte('[')
			b.Write(residues)
			b.WriteByte(']')
		default:
			b.WriteByte('.')
		}
		if gap[i] {
			b.WriteByte('?')
		}
	}
	b.WriteByte(')')
	return b.String()
}

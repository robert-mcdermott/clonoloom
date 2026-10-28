package motifs

// Profile is descriptive residue frequency among unique query members. Selection
// by a signature explains its conserved positions; this is not independent evidence.
type Profile struct {
	Alphabet    string      `json:"alphabet"`
	N           int         `json:"n"`
	Counts      [][]int     `json:"counts"`
	Frequencies [][]float64 `json:"frequencies"`
}

func Residues(r *Result, g Group) Profile {
	p := Profile{Alphabet: Alphabet, N: len(g.Query), Counts: make([][]int, g.Length), Frequencies: make([][]float64, g.Length)}
	for at := 0; at < g.Length; at++ {
		p.Counts[at] = make([]int, len(Alphabet))
		p.Frequencies[at] = make([]float64, len(Alphabet))
		for _, i := range g.Query {
			for j := range Alphabet {
				if r.Query[i].Junction[at] == Alphabet[j] {
					p.Counts[at][j]++
				}
			}
		}
		if p.N > 0 {
			for j, c := range p.Counts[at] {
				p.Frequencies[at][j] = float64(c) / float64(p.N)
			}
		}
	}
	return p
}

package tcrdist

// The residues' distances (Dash et al. 2017, Methods): 0 between a residue and itself, and
// min(4, 4 − BLOSUM62(a, b)) between two of the 20 amino acids, so that only the pairs BLOSUM62
// scores above 0 are nearer than 4. Any other character (B, Z, X, a stop, the gap of an aligned
// loop) is 0 from everything: tcrdist3 encodes such characters outside its matrix's alphabet,
// whose rows it leaves at 0.

// aminoAcids in BLOSUM62's order.
const aminoAcids = "ARNDCQEGHILKMFPSTWYV"

// blosum62 is the BLOSUM62 matrix (Henikoff and Henikoff 1992, PNAS 89:10915) over aminoAcids.
var blosum62 = [20][20]int8{
	{4, -1, -2, -2, 0, -1, -1, 0, -2, -1, -1, -1, -1, -2, -1, 1, 0, -3, -2, 0},
	{-1, 5, 0, -2, -3, 1, 0, -2, 0, -3, -2, 2, -1, -3, -2, -1, -1, -3, -2, -3},
	{-2, 0, 6, 1, -3, 0, 0, 0, 1, -3, -3, 0, -2, -3, -2, 1, 0, -4, -2, -3},
	{-2, -2, 1, 6, -3, 0, 2, -1, -1, -3, -4, -1, -3, -3, -1, 0, -1, -4, -3, -3},
	{0, -3, -3, -3, 9, -3, -4, -3, -3, -1, -1, -3, -1, -2, -3, -1, -1, -2, -2, -1},
	{-1, 1, 0, 0, -3, 5, 2, -2, 0, -3, -2, 1, 0, -3, -1, 0, -1, -2, -1, -2},
	{-1, 0, 0, 2, -4, 2, 5, -2, 0, -3, -3, 1, -2, -3, -1, 0, -1, -3, -2, -2},
	{0, -2, 0, -1, -3, -2, -2, 6, -2, -4, -4, -2, -3, -3, -2, 0, -2, -2, -3, -3},
	{-2, 0, 1, -1, -3, 0, 0, -2, 8, -3, -3, -1, -2, -1, -2, -1, -2, -2, 2, -3},
	{-1, -3, -3, -3, -1, -3, -3, -4, -3, 4, 2, -3, 1, 0, -3, -2, -1, -3, -1, 3},
	{-1, -2, -3, -4, -1, -2, -3, -4, -3, 2, 4, -2, 2, 0, -3, -2, -1, -2, -1, 1},
	{-1, 2, 0, -1, -3, 1, 1, -2, -1, -3, -2, 5, -1, -3, -1, 0, -1, -3, -2, -2},
	{-1, -1, -2, -3, -1, 0, -2, -3, -2, 1, 2, -1, 5, 0, -2, -1, -1, -1, -1, 1},
	{-2, -3, -3, -3, -2, -3, -3, -3, -1, 0, 0, -3, 0, 6, -4, -2, -2, 1, 3, -1},
	{-1, -2, -2, -1, -3, -1, -1, -2, -2, -3, -3, -1, -2, -4, 7, -1, -1, -4, -3, -2},
	{1, -1, 1, 0, -1, 0, 0, 0, -1, -2, -2, 0, -1, -2, -1, 4, 1, -3, -2, -2},
	{0, -1, 0, -1, -1, -1, -1, -2, -2, -1, -1, -1, -1, -2, -1, 1, 5, -2, -2, 0},
	{-3, -3, -4, -4, -2, -2, -3, -2, -2, -3, -2, -3, -1, 1, -4, -3, -2, 11, 2, -3},
	{-2, -2, -2, -3, -2, -1, -2, -3, 2, -1, -1, -2, -1, 3, -3, -2, -2, 2, 7, -1},
	{0, -3, -3, -3, -1, -2, -2, -3, -3, 3, 1, -2, 1, -1, -2, -2, 0, -3, -1, 4},
}

// other is the code of every character that is not one of the 20 amino acids.
const other = 20

// codes encodes a character as its index in aminoAcids, or other.
var codes = func() (c [256]uint8) {
	for i := range c {
		c[i] = other
	}
	for i := 0; i < len(aminoAcids); i++ {
		c[aminoAcids[i]] = uint8(i)
	}
	return
}()

// table is the residues' distances by code.
var table = func() (t [other + 1][other + 1]uint8) {
	for a := 0; a < 20; a++ {
		for b := 0; b < 20; b++ {
			if a != b {
				t[a][b] = uint8(min(4, 4-int(blosum62[a][b])))
			}
		}
	}
	return
}()

// Encode encodes a sequence of amino acids (upper case) for distances.
func Encode(s string) []uint8 {
	out := make([]uint8, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = codes[s[i]]
	}
	return out
}

// Residues is the distance between two residues.
func Residues(a, b byte) int {
	return int(table[codes[a]][codes[b]])
}

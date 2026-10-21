package seq

import "testing"

func TestTranslateWithTheStandardCode(t *testing.T) {
	cases := map[string]string{
		"TGTGCCAGCAGTTTAGCGGGAGGGCCTGAAGCTTTCTTT": "CASSLAGGPEAFF",
		"tgtgccagc":    "CAS",
		"ATGTAATAGTGA": "M***",
		"TGTGCCAGCAG":  "CAS", // the incomplete codon is dropped
		"TGNGCC":       "XA",
		"UGUGCC":       "CA",
		"":             "",
	}
	for nt, want := range cases {
		if got := Translate(nt); got != want {
			t.Errorf("Translate(%q) = %q, want %q", nt, got, want)
		}
	}
	// Every codon of the standard table, against the amino acid it is known to encode.
	known := map[string]byte{
		"TTT": 'F', "TTA": 'L', "CTG": 'L', "ATT": 'I', "ATG": 'M', "GTC": 'V', "TCT": 'S', "AGC": 'S',
		"CCA": 'P', "ACG": 'T', "GCA": 'A', "TAT": 'Y', "TAA": '*', "TAG": '*', "TGA": '*', "CAT": 'H',
		"CAA": 'Q', "AAT": 'N', "AAA": 'K', "GAT": 'D', "GAA": 'E', "TGT": 'C', "TGG": 'W', "CGA": 'R',
		"AGA": 'R', "GGG": 'G',
	}
	for codon, want := range known {
		if got := Codon(codon[0], codon[1], codon[2]); got != want {
			t.Errorf("%s = %c, want %c", codon, got, want)
		}
	}
}

func TestReverseComplement(t *testing.T) {
	if got := ReverseComplement("ACGTNacgtnRY-"); got != "-RYnacgtNACGT" {
		t.Fatalf("got %q", got)
	}
	if got := ReverseComplement(ReverseComplement("GATTACAKMSWBDHV")); got != "GATTACAKMSWBDHV" {
		t.Fatalf("not an involution: %q", got)
	}
}

func TestAlphabetsAndAnchors(t *testing.T) {
	if !IsNucleotide("ACGTN") || IsNucleotide("ACGTX") || IsNucleotide("") {
		t.Fatal("IsNucleotide")
	}
	if !IsProtein("CASSLAGGPEAFF") || !IsProtein("CAS*X") || IsProtein("cass") || IsProtein("CASB") || IsProtein("") {
		t.Fatal("IsProtein")
	}
	if c, f := Anchors("CASSLAGGPEAFF"); !c || !f {
		t.Fatal("a TRB junction has both anchors")
	}
	if c, w := Anchors("CARDYW"); !c || !w {
		t.Fatal("an IGH junction ends in W")
	}
	if c, f := Anchors("ASSLAGGPEAF"); c || !f {
		t.Fatal("a CDR3 given as a junction lacks its cysteine")
	}
	if CDR3("CASSLAGGPEAFF", 1) != "ASSLAGGPEAF" || CDR3("TGTGCCAGCTTT", 3) != "GCCAGC" || CDR3("C", 1) != "" {
		t.Fatal("CDR3")
	}
	if !HasStop("CAS*F") || HasStop("CASF") {
		t.Fatal("HasStop")
	}
}

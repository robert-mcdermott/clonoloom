package simulate

// Antigen-specific responses (clonoloom-spec/roadmap.md, wave 4, slice 4): T cells that recognize
// an epitope, as families of receptors around a few founders, the clusters Dash et al. 2017 found
// an epitope's receptors in. A founder is a rearrangement the generator makes, the same in every
// subject, as a public receptor is. A subject's family of it are receptors of the founder's V, D
// and J genes and deletions whose non-templated nucleotides differ, so that their CDR3s differ from
// the founder's by a few residues, mostly ones alike. Each subject has receptors of its own that
// recognize an epitope too, unlike any other (Dash et al. found a third or more of an epitope's
// receptors without a near neighbor). They are in each subject's blood from its first sample, rare,
// and hold the share of its cells that Shares gives at each time point by its condition: after a
// vaccination, they expand. The founders, and receptors of other donors
// around them, are written as a specificity database in VDJdb's format, with epitopes no subject
// responds to, as a database holds.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"clonoloom/internal/dmath"
	"clonoloom/internal/random"
	"clonoloom/internal/tcrdist"
)

// Response is a series' antigen-specific response: its Epitopes; how a founder's Family is made in
// each subject; the families' share of the blood's cells at each time point by condition (Shares);
// and Database receptors of other donors written to the database for each founder, besides it.
type Response struct {
	Epitopes []Epitope            `json:"epitopes"`
	Family   Family               `json:"family"`
	Shares   map[string][]float64 `json:"shares"`
	Database int                  `json:"database"`
}

// Epitope is a peptide of an antigen, its share of the response (Weight; 0: no subject responds to
// it, and only the database holds receptors that recognize it) and how many founders it has.
type Epitope struct {
	Peptide  string  `json:"peptide"`
	Antigen  string  `json:"antigen"`
	Weight   float64 `json:"weight"`
	Founders int     `json:"founders"`
}

// Family is how a founder's family is made in a subject: Members receptors on average (a Poisson
// count whose mean is drawn once for each founder from an exponential of this mean, so that some
// families are large in every subject and many small); each the founder with Substitutions residues
// changed on average (a Poisson count: a member with none is the founder itself), each to a residue
// TCRdist puts d units away with a chance in proportion to e^(−Similarity·d). Private: each
// subject's receptors of its own for an epitope, without a family, Private times the epitope's
// founders on average (a Poisson count). Each receptor's cells, relative to its epitope's other
// receptors, are from a power law (Alpha, at most MaxSize).
type Family struct {
	Members       float64 `json:"members"`
	Substitutions float64 `json:"substitutions"`
	Similarity    float64 `json:"similarity"`
	Private       float64 `json:"private"`
	Alpha         float64 `json:"alpha"`
	MaxSize       int     `json:"maxSize"`
}

func (resp *Response) check(s Study, ser *Series) error {
	if len(resp.Epitopes) == 0 {
		return fmt.Errorf("a response needs epitopes")
	}
	peptides := map[string]bool{}
	weight := 0.0
	for _, e := range resp.Epitopes {
		if e.Peptide == "" || peptides[e.Peptide] || strings.Trim(e.Peptide, "ACDEFGHIKLMNPQRSTVWY") != "" {
			return fmt.Errorf("epitope %q: each epitope a different peptide of amino acids", e.Peptide)
		}
		peptides[e.Peptide] = true
		if e.Founders < 1 || !(e.Weight >= 0) || e.Antigen == "" {
			return fmt.Errorf("epitope %s needs an antigen, founders and a weight of zero or more", e.Peptide)
		}
		weight += e.Weight
	}
	if !(weight > 0) {
		return fmt.Errorf("some epitope of the response needs a weight above zero")
	}
	f := resp.Family
	if !(f.Members > 0) || !(f.Substitutions >= 0) || !(f.Similarity >= 0) || !(f.Private >= 0) || !(f.Alpha > 0) || f.MaxSize < 1 || resp.Database < 0 {
		return fmt.Errorf("a response's families need members, substitutions, similarity and private receptors of zero or more, a positive alpha and a largest size; its database, receptors of zero or more")
	}
	for _, g := range s.Groups {
		shares := resp.Shares[g.Condition]
		if len(shares) != len(ser.Times) {
			return fmt.Errorf("the response needs a share at each time point for condition %q", g.Condition)
		}
		for t, x := range shares {
			ref := 0.0
			if ser.Reference != nil {
				ref = ser.Reference.Shares[g.Condition][t]
			}
			if !(x >= 0 && x+ref <= 1) {
				return fmt.Errorf("the response's shares must be from 0 to 1, with the reference's at most 1")
			}
		}
	}
	return nil
}

// founder is one of an epitope's founders: its rearrangement, the junction's nucleotides its
// insertions made (open), the codons a family may change (those with such a nucleotide, other than
// the first and the last), and its family's mean size; or a subject's private receptor.
type founder struct {
	epitope int
	name    string
	x       Rearrangement
	open    []bool
	free    []int
	mean    float64
	private bool
}

func rearrangementKey(x *Rearrangement) string {
	return x.V.Name + "|" + x.J.Name + "|" + x.JunctionAA
}

// nonTemplated marks the junction's nucleotides a rearrangement's insertions made.
func nonTemplated(x *Rearrangement) []bool {
	vPart, dPart, _ := x.parts()
	open := make([]bool, len(x.Junction))
	mark := func(from, n int) {
		for i := from; i < from+n; i++ {
			if j := i - x.V.Anchor; j >= 0 && j < len(open) {
				open[j] = true
			}
		}
	}
	mark(len(vPart), len(x.N1))
	mark(len(vPart)+len(x.N1)+len(dPart), len(x.N2))
	return open
}

// founders draws each epitope's founders: distinct productive rearrangements with at least two
// codons a family may change.
func founders(g *Generator, resp *Response, r *random.Random) []founder {
	var out []founder
	taken := map[string]bool{}
	for e, ep := range resp.Epitopes {
		for k := 0; k < ep.Founders; {
			x := g.Draw(r)
			if !x.Productive {
				continue
			}
			open := nonTemplated(&x)
			var free []int
			for c := 1; c < len(x.JunctionAA)-1; c++ {
				if open[3*c] || open[3*c+1] || open[3*c+2] {
					free = append(free, c)
				}
			}
			key := rearrangementKey(&x)
			if len(free) < 2 || taken[key] {
				continue
			}
			taken[key] = true
			k++
			mean := float64(resp.Family.Members * -dmath.Log(1-r.Float64()))
			out = append(out, founder{epitope: e, name: fmt.Sprintf("%s-F%02d", ep.Peptide, k), x: x, open: open, free: free, mean: mean})
		}
	}
	return out
}

// codonNames are the 64 codons in a fixed order.
var codonNames = func() []string {
	var out []string
	for _, a := range "TCAG" {
		for _, b := range "TCAG" {
			for _, c := range "TCAG" {
				out = append(out, string([]rune{a, b, c}))
			}
		}
	}
	return out
}()

// substitute changes a codon to one of another amino acid (not a stop) that differs from it only
// where open: the amino acid chosen by its TCRdist distance d from the codon's, in proportion to
// e^(−similarity·d), and the codon among its codons at random. It answers false where no codon
// can be made.
func substitute(codon []byte, open []bool, similarity float64, r *random.Random) bool {
	old := codons[string(codon)]
	var aas []byte
	byAA := map[byte][]string{}
	for _, c := range codonNames {
		aa := codons[c]
		if aa == '*' || aa == old {
			continue
		}
		fits := true
		for i := 0; i < 3; i++ {
			if !open[i] && c[i] != codon[i] {
				fits = false
			}
		}
		if !fits {
			continue
		}
		if len(byAA[aa]) == 0 {
			aas = append(aas, aa)
		}
		byAA[aa] = append(byAA[aa], c)
	}
	if len(aas) == 0 {
		return false
	}
	weights := make([]float64, len(aas))
	for i, aa := range aas {
		weights[i] = dmath.Exp(float64(-similarity * float64(tcrdist.Residues(old, aa))))
	}
	choices := byAA[aas[r.Discrete(random.Cumulate(weights))]]
	copy(codon, choices[r.IntN(len(choices))])
	return true
}

// member draws a receptor of a founder's family: the founder with residues substituted at distinct
// codons it may change, each by its non-templated nucleotides alone, so that the member is a
// rearrangement of the founder's genes and deletions with other insertions. It answers the
// residues changed.
func (f *founder) member(fam Family, r *random.Random) (Rearrangement, int) {
	x := f.x
	junction := []byte(x.Junction)
	order := append([]int(nil), f.free...)
	n := min(r.Poisson(fam.Substitutions), len(order))
	changed := 0
	for k := 0; k < n; k++ {
		j := k + r.IntN(len(order)-k)
		order[k], order[j] = order[j], order[k]
		c := order[k]
		if substitute(junction[3*c:3*c+3], f.open[3*c:3*c+3], fam.Similarity, r) {
			changed++
		}
	}
	vPart, dPart, _ := x.parts()
	n1, n2 := []byte(x.N1), []byte(x.N2)
	for i := range junction {
		if junction[i] == x.Junction[i] {
			continue
		}
		p := i + x.V.Anchor - len(vPart)
		if p < len(n1) {
			n1[p] = junction[i]
		} else {
			n2[p-len(n1)-len(dPart)] = junction[i]
		}
	}
	x.N1, x.N2 = string(n1), string(n2)
	x.assemble()
	return x, changed
}

// member is a subject's receptor that recognizes an epitope: its clone, its founder (or itself,
// a private receptor) and the residues by which it differs from it.
type member struct {
	Clone
	founder       *founder
	substitutions int
}

// families draws a subject's receptors that recognize the epitopes it responds to: of each
// founder, a Poisson number of members of the founder's mean; and its private receptors. Each is a
// clonotype the subject's repertoire does not already hold (taken), drawn again where it does.
func families(g *Generator, fs []founder, resp *Response, taken map[string]bool, id string, r *random.Random) []member {
	var out []member
	add := func(f *founder, x Rearrangement, changed int) {
		taken[rearrangementKey(&x)] = true
		out = append(out, member{Clone: Clone{Rearrangement: x, Size: powerLaw(r, resp.Family.Alpha, resp.Family.MaxSize)}, founder: f, substitutions: changed})
	}
	for fi := range fs {
		f := &fs[fi]
		if !(resp.Epitopes[f.epitope].Weight > 0) {
			continue
		}
		n := r.Poisson(f.mean)
		for range n {
			for try := 0; try < 20; try++ {
				if x, changed := f.member(resp.Family, r); !taken[rearrangementKey(&x)] {
					add(f, x, changed)
					break
				}
			}
		}
	}
	for e, ep := range resp.Epitopes {
		if !(ep.Weight > 0) {
			continue
		}
		n := r.Poisson(float64(resp.Family.Private * float64(ep.Founders)))
		for k := 1; k <= n; {
			x := g.Draw(r)
			if !x.Productive || taken[rearrangementKey(&x)] {
				continue
			}
			add(&founder{epitope: e, name: fmt.Sprintf("%s-%s-P%02d", ep.Peptide, id, k), x: x, private: true}, x, 0)
			k++
		}
	}
	for i := range out {
		out[i].ID = fmt.Sprintf("%s-R%06d", id, i+1)
	}
	return out
}

// responsePart is a subject's families as a part of its blood: each member's cells relative to its
// epitope's members, times the epitope's weight.
func responsePart(members []member, resp *Response, firstRow int) part {
	total := make([]float64, len(resp.Epitopes))
	for _, m := range members {
		total[m.founder.epitope] += float64(m.Size)
	}
	p := part{}
	for i, m := range members {
		e := m.founder.epitope
		p.row = append(p.row, firstRow+i)
		p.cells = append(p.cells, float64(float64(m.Size)*resp.Epitopes[e].Weight)/total[e])
		p.class = append(p.class, -1)
	}
	return p
}

// ResponseTruth is a subject's receptors that recognize an epitope, by epitope: the members of
// founders' families (each a clonotype of its own), the founders with a family in the subject, the
// members that are their founder itself, and the subject's private receptors.
type ResponseTruth struct {
	Epitopes []EpitopeTruth `json:"epitopes"`
}

type EpitopeTruth struct {
	Peptide  string `json:"peptide"`
	Members  int    `json:"members"`
	Founders int    `json:"founders"`
	Exact    int    `json:"exact"`
	Private  int    `json:"private"`
}

func responseTruth(members []member, resp *Response) *ResponseTruth {
	out := &ResponseTruth{}
	for e, ep := range resp.Epitopes {
		if !(ep.Weight > 0) {
			continue
		}
		t := EpitopeTruth{Peptide: ep.Peptide}
		with := map[*founder]bool{}
		for _, m := range members {
			switch {
			case m.founder.epitope != e:
			case m.founder.private:
				t.Private++
			default:
				t.Members++
				with[m.founder] = true
				if m.substitutions == 0 {
					t.Exact++
				}
			}
		}
		t.Founders = len(with)
		out.Epitopes = append(out.Epitopes, t)
	}
	return out
}

// SpecificityTruth names the files of a study's response: its founders, every subject's receptors
// that recognize an epitope (any clonotype not there recognizes none), and the database.
type SpecificityTruth struct {
	Founders   string `json:"founders"`
	Clonotypes string `json:"clonotypes"`
	Database   string `json:"database"`
	// DatabaseRows: the database's receptors, of which Founders' are the founders.
	DatabaseRows int `json:"databaseRows"`
}

// writeFounders writes the founders: each one's epitope, antigen, rearrangement and family's mean
// size in a subject.
func writeFounders(path string, fs []founder, resp *Response) error {
	return writeLines(path, func(w *bufio.Writer) {
		w.WriteString("founder\tepitope\tantigen\tv_call\td_call\tj_call\tjunction\tjunction_aa\tmean_members\n")
		for _, f := range fs {
			ep := resp.Epitopes[f.epitope]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", f.name, ep.Peptide, ep.Antigen, f.x.V.Name, f.x.D.Name, f.x.J.Name, f.x.Junction, f.x.JunctionAA, num(f.mean))
		}
	})
}

// specificRow is a subject's receptor that recognizes an epitope, for truth/specificity.tsv.
type specificRow struct {
	subject string
	m       member
}

// writeSpecificity writes every subject's receptors that recognize an epitope: each its subject,
// clone, genes, junction, epitope, antigen, founder (or itself, private) and the residues it
// differs from it by.
func writeSpecificity(path string, rows []specificRow, resp *Response) error {
	return writeLines(path, func(w *bufio.Writer) {
		w.WriteString("subject\tclone_id\tv_call\tj_call\tjunction\tjunction_aa\tepitope\tantigen\tfounder\tprivate\tsubstitutions\n")
		for _, r := range rows {
			f := r.m.founder
			ep := resp.Epitopes[f.epitope]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\n", r.subject, r.m.ID, r.m.V.Name, r.m.J.Name, r.m.Junction, r.m.JunctionAA, ep.Peptide, ep.Antigen, f.name, flag(f.private), r.m.substitutions)
		}
	})
}

// vdjdbColumns are VDJdb's (vdjdb.txt, its full format) up to its score.
var vdjdbColumns = []string{"complex.id", "gene", "cdr3", "v.segm", "j.segm", "species", "mhc.a", "mhc.b", "mhc.class",
	"antigen.epitope", "antigen.gene", "antigen.species", "reference.id", "method", "meta", "cdr3fix", "vdjdb.score"}

// writeDatabase writes the specificity database in VDJdb's format: each epitope's founders (score
// 3) and, for each, receptors of its family as other donors would have them (score 1), each
// receptor once. It answers the rows written.
func writeDatabase(path, study string, fs []founder, resp *Response, r *random.Random) (int, error) {
	rows := 0
	err := writeLines(path, func(w *bufio.Writer) {
		w.WriteString(strings.Join(vdjdbColumns, "\t") + "\n")
		row := func(x *Rearrangement, ep Epitope, score int) {
			fmt.Fprintf(w, "0\tTRB\t%s\t%s\t%s\tHomoSapiens\tsimulated\tB2M\tMHCI\t%s\t%s\tSimulated\tclonoloom-simulated-%s\t{\"identification\":\"simulated\"}\t{}\t{\"cdr3\":\"%s\",\"good\":true}\t%d\n",
				x.JunctionAA, x.V.Name, x.J.Name, ep.Peptide, ep.Antigen, study, x.JunctionAA, score)
			rows++
		}
		for _, f := range fs {
			ep := resp.Epitopes[f.epitope]
			taken := map[string]bool{rearrangementKey(&f.x): true}
			row(&f.x, ep, 3)
			for range resp.Database {
				for try := 0; try < 20; try++ {
					x, _ := f.member(resp.Family, r)
					if key := rearrangementKey(&x); !taken[key] {
						taken[key] = true
						row(&x, ep, 1)
						break
					}
				}
			}
		}
	})
	return rows, err
}

func num(v float64) string { return fmt.Sprintf("%.6g", v) }

// writeLines writes a text file through a buffer.
func writeLines(path string, write func(w *bufio.Writer)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	write(w)
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

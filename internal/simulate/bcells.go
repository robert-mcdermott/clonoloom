package simulate

// An artificial paired B-cell benchmark, ClonoLoom's choice throughout. This is not a fitted
// recombination, affinity-selection or context-dependent SHM model. Known ancestors and edge
// histories support later validation without treating inferred trees as truth. Biological
// context: Yaari et al. 2013, Front Immunol 4:358, doi:10.3389/fimmu.2013.00358 (context-dependent
// SHM, deliberately absent here); Horns et al. 2016, eLife 5:e16578, doi:10.7554/eLife.16578
// (class switching). See docs/SIMULATION.md and research.md for the model and its limits.

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/random"
)

const bcellModel = "clonoloom-artificial-bcell-v1"

// BCellStudy makes three samples per subject, at days -7, 14 and 28 of an immunization.
// Families is per subject; CellsPerSample counts paired cells, not rearrangements or molecules.
// MutationsPerBranch is accepted substitutions in each chain on each edge, including reversions.
// The tree, sampling weights, sequence geometry and mutation rules are versioned by bcellModel.
type BCellStudy struct {
	Subjects           int `json:"subjects"`
	Families           int `json:"families"`
	CellsPerSample     int `json:"cellsPerSample"`
	MutationsPerBranch int `json:"mutationsPerBranch"`
}

func (b BCellStudy) check(s Study) error {
	if s.Series != nil || len(s.Groups) != 0 || s.SamplesPerSubject != 0 || len(s.Templates) != 0 || s.ReadsPerTemplate != 0 || s.ReadsSize != 0 || !reflect.DeepEqual(s.Recombination, Recombination{}) {
		return fmt.Errorf("bcells replaces the TRB model: omit groups, samples, templates, reads, recombination and series parameters (or leave them zero)")
	}
	if b.Subjects < 1 || b.Subjects > 20 || b.Families < 1 || b.Families > 100 || b.CellsPerSample < 1 || b.CellsPerSample > 10000 || b.MutationsPerBranch < 1 || b.MutationsPerBranch > 20 {
		return fmt.Errorf("bcells needs subjects in 1–20, families per subject in 1–100, cellsPerSample in 1–10000 and mutationsPerBranch in 1–20")
	}
	if b.Subjects*b.CellsPerSample*3 > 100000 {
		return fmt.Errorf("bcells is limited to 100000 sampled cells across the study")
	}
	return nil
}

// bcellMutation describes one accepted event against the current parent-derived sequence.
// Position is 1-based; Effect compares the codon immediately before and after this event.
type bcellMutation struct {
	Position int    `json:"position"`
	From     string `json:"from"`
	To       string `json:"to"`
	Effect   string `json:"effect"`
}

type bcellChain struct {
	Locus     string          `json:"locus"`
	VCall     string          `json:"vCall"`
	JCall     string          `json:"jCall"`
	Sequence  string          `json:"sequence"`
	Mutations []bcellMutation `json:"mutations"`
}

type bcellNode struct {
	ID      string     `json:"id"`
	Subject string     `json:"subject"`
	Family  string     `json:"family"`
	Parent  string     `json:"parent"`
	Isotype string     `json:"isotype"`
	Heavy   bcellChain `json:"heavy"`
	Light   bcellChain `json:"light"`
}

type bcellAbundance struct {
	Node        string  `json:"node"`
	Probability float64 `json:"probability"`
	Cells       int     `json:"cells"`
}

type bcellSample struct {
	ID        string           `json:"id"`
	Subject   string           `json:"subject"`
	Day       int              `json:"day"`
	Cells     int              `json:"cells"`
	Abundance []bcellAbundance `json:"abundance"`
}

type bcellFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type bcellTruth struct {
	Model     string        `json:"model"`
	Simulator string        `json:"simulator"`
	Study     Study         `json:"study"`
	Nodes     []bcellNode   `json:"nodes"`
	Samples   []bcellSample `json:"samples"`
	Files     []bcellFile   `json:"files"`
}

// Synthetic coding sequences: shared backbones for two V calls per locus; independent family
// junctions; no imported gene segments. Geometry is contiguous and artificial, not IMGT numbering.
// Both chains have 120 codons; junction 283–321 includes Cys and Trp (IGH) or Phe (light).
func bcellAncestor(seed uint64, locus string, family int, r *random.Random) bcellChain {
	stream := map[string]uint64{"IGH": 7100, "IGK": 7200, "IGL": 7300}[locus] + uint64(family%2)
	backbone := random.New(seed, stream)
	var codons []string
	for _, a := range "ACGT" {
		for _, b := range "ACGT" {
			for _, c := range "ACGT" {
				codon := string([]rune{a, b, c})
				if Translate(codon) != "*" {
					codons = append(codons, codon)
				}
			}
		}
	}
	var seq strings.Builder
	for i := range 120 {
		if i >= 95 && i < 106 {
			seq.WriteString(codons[r.IntN(len(codons))])
		} else {
			seq.WriteString(codons[backbone.IntN(len(codons))])
		}
	}
	bases := []byte(seq.String())
	copy(bases[282:285], "TGT")
	anchor := "TGG"
	if locus != "IGH" {
		anchor = "TTT"
	}
	copy(bases[318:321], anchor)
	return bcellChain{Locus: locus, VCall: fmt.Sprintf("%sV%d-SIM*01", locus, family%2+1), JCall: locus + "J1-SIM*01", Sequence: string(bases), Mutations: []bcellMutation{}}
}

// Uniform proposed site and alternative base; reject anchor changes and stop codons. Sequential
// events may hit a site again. Node 3's first event deliberately reverts its parent's last event,
// exercising the difference between accumulated events and net differences from the ancestor.
func mutateBCell(parent bcellChain, n int, revert bool, r *random.Random) bcellChain {
	child := parent
	child.Mutations = make([]bcellMutation, 0, n)
	bases := []byte(parent.Sequence)
	for len(child.Mutations) < n {
		pos := r.IntN(len(bases))
		base := "ACGT"[r.IntN(4)]
		if revert && len(child.Mutations) == 0 {
			// Undo the last event first: even if this edge hit one site repeatedly, its previous
			// state is stop-free and the recorded 'from' base is still the child's current base.
			m := parent.Mutations[len(parent.Mutations)-1]
			pos, base = m.Position-1, m.From[0]
		}
		if pos >= 282 && pos < 285 || pos >= 318 && pos < 321 || bases[pos] == base {
			continue
		}
		codon := pos / 3 * 3
		before, old := Translate(string(bases[codon:codon+3])), bases[pos]
		bases[pos] = base
		after := Translate(string(bases[codon : codon+3]))
		if after == "*" {
			bases[pos] = old
			continue
		}
		effect := "replacement"
		if before == after {
			effect = "silent"
		}
		child.Mutations = append(child.Mutations, bcellMutation{pos + 1, string(old), string(base), effect})
	}
	child.Sequence = string(bases)
	return child
}

func bcellFamily(s Study, subject string, family, subjectIndex int) []bcellNode {
	r := random.New(s.Seed, uint64(10000+subjectIndex*100+family))
	id := fmt.Sprintf("%s-F%03d", subject, family+1)
	light := "IGK"
	if family%2 != 0 {
		light = "IGL"
	}
	root := bcellNode{ID: id + "-N0", Subject: subject, Family: id, Isotype: "IGHM",
		Heavy: bcellAncestor(s.Seed, "IGH", family, r), Light: bcellAncestor(s.Seed, light, family, r)}
	nodes := []bcellNode{root}
	isotypes := []string{"IGHM", "IGHM", "IGHG1", "IGHM", "IGHG1", "IGHG1", "IGHA1"}
	for i := 1; i < 7; i++ {
		parent := nodes[(i-1)/2]
		node := bcellNode{ID: fmt.Sprintf("%s-N%d", id, i), Subject: subject, Family: id, Parent: parent.ID, Isotype: isotypes[i]}
		node.Heavy = mutateBCell(parent.Heavy, s.BCells.MutationsPerBranch, i == 3, r)
		node.Light = mutateBCell(parent.Light, s.BCells.MutationsPerBranch, i == 3, r)
		nodes = append(nodes, node)
	}
	return nodes
}

// Probabilities are node weights normalized over all the subject's families. The first quarter
// (rounded up) expand at day 14 and partly contract at day 28. All nodes, including zeros, are
// recorded. No extra dispersion, dropout, sequencing errors or molecular counts are invented.
func bcellWeights(families, time int) []float64 {
	state := [3][7]float64{{1, 0, 0, 0, 0, 0, 0}, {1, 4, 4, 0, 0, 0, 0}, {1, 1, 1, 4, 4, 4, 4}}
	fold := [3]float64{1, 8, 3}
	weights := make([]float64, families*7)
	total := 0.0
	for f := range families {
		expansion := 1.0
		if f < (families+3)/4 {
			expansion = fold[time]
		}
		for n := range 7 {
			// Divide by the node-weight sum so that family expansion has the stated fold.
			weight := float64(state[time][n]*expansion) / [3]float64{1, 9, 19}[time]
			weights[f*7+n] = weight
			total += weight
		}
	}
	for i := range weights {
		weights[i] /= total
	}
	return weights
}

var bcellColumns = []string{"sequence_id", "cell_id", "repertoire_id", "productive", "vj_in_frame", "stop_codon", "locus", "v_call", "d_call", "j_call", "c_call", "junction", "junction_aa", "sequence", "sequence_alignment", "germline_alignment", "germline_database", "v_cigar", "d_cigar", "j_cigar", "rev_comp", "v_sequence_start", "v_sequence_end", "j_sequence_start", "j_sequence_end", "fwr1_start", "fwr1_end", "cdr1_start", "cdr1_end", "fwr2_start", "fwr2_end", "cdr2_start", "cdr2_end", "fwr3_start", "fwr3_end", "cdr3_start", "cdr3_end", "fwr4_start", "fwr4_end"}

func bcellFields(sample, cell string, node, root bcellNode, heavy bool) []string {
	chain, ancestor, constant := node.Light, root.Light, node.Light.Locus+"C"
	if chain.Locus == "IGL" {
		constant = "IGLC1"
	}
	if heavy {
		chain, ancestor, constant = node.Heavy, root.Heavy, node.Isotype
	}
	junction := chain.Sequence[282:321]
	return []string{cell + "-" + chain.Locus, cell, sample, "T", "T", "F", chain.Locus, chain.VCall, "", chain.JCall, constant,
		junction, Translate(junction), chain.Sequence, chain.Sequence, ancestor.Sequence, bcellModel, "", "", "", "F",
		"1", "285", "319", "360", "1", "78", "79", "114", "115", "165", "166", "195", "196", "285", "286", "318", "319", "360"}
}

func writeBCellSample(ctx context.Context, path, id string, nodes []bcellNode, weights []float64, cells int, r *random.Random, observations *csv.Writer) ([]bcellAbundance, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = '\t'
	if err := w.Write(bcellColumns); err != nil {
		return nil, err
	}
	abundance := make([]bcellAbundance, len(nodes))
	for i, node := range nodes {
		abundance[i] = bcellAbundance{Node: node.ID, Probability: weights[i]}
	}
	cumulative := random.Cumulate(weights)
	for c := range cells {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		i := r.Discrete(cumulative)
		abundance[i].Cells++
		cell := fmt.Sprintf("%s-C%06d", id, c+1)
		for _, heavy := range []bool{true, false} {
			if err := w.Write(bcellFields(id, cell, nodes[i], nodes[i/7*7], heavy)); err != nil {
				return nil, err
			}
		}
		if err := observations.Write([]string{id, cell, nodes[i].ID}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return abundance, f.Close()
}

func bcellRepertoire(s Study, subject, id, file, version string, day int) map[string]any {
	metadata := repertoire(s, Subject{ID: subject, Condition: "simulated immunization"}, id, file, version)
	sample := metadata["sample"].([]any)[0].(map[string]any)
	sample["template_class"] = "RNA"
	sample["single_cell"] = true
	sample["pcr_target"] = []any{map[string]any{"pcr_target_locus": "IGH"}, map[string]any{"pcr_target_locus": "IGK"}, map[string]any{"pcr_target_locus": "IGL"}}
	sample["collection_time_point_relative"] = day
	sample["collection_time_point_relative_unit"] = map[string]any{"id": "UO:0000033", "label": "day"}
	sample["collection_time_point_reference"] = "simulated immunization"
	// The model marker is independent of the editable study name; methods text can identify it.
	metadata["study"].(map[string]any)["study_id"] = "clonoloom-simulated-" + bcellModel
	return metadata
}

// Build beside the destination, then promote a complete study. An interrupted/error run leaves
// no partial example or truth marker. A nonempty destination is never overwritten.
func runBCells(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0o755); err != nil {
		return Summary{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-bcells-")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(stage)
	if err := os.Mkdir(filepath.Join(stage, "truth"), 0o755); err != nil {
		return Summary{}, err
	}
	summary := Summary{Dir: dir, Subjects: s.BCells.Subjects}
	truth := bcellTruth{Model: bcellModel, Simulator: "ClonoLoom simulate " + version, Study: s}
	obs, err := os.Create(filepath.Join(stage, "truth", "observations.tsv"))
	if err != nil {
		return Summary{}, err
	}
	defer obs.Close()
	observations := csv.NewWriter(obs)
	observations.Comma = '\t'
	if err := observations.Write([]string{"sample_id", "cell_id", "node_id"}); err != nil {
		return Summary{}, err
	}
	var repertoires []map[string]any
	for si := range s.BCells.Subjects {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		subject := fmt.Sprintf("S%02d", si+1)
		progress(float64(si)/float64(s.BCells.Subjects), "Making "+subject+"'s artificial B-cell families")
		var nodes []bcellNode
		for family := range s.BCells.Families {
			if err := ctx.Err(); err != nil {
				return Summary{}, err
			}
			nodes = append(nodes, bcellFamily(s, subject, family, si)...)
		}
		truth.Nodes = append(truth.Nodes, nodes...)
		for ti, day := range []int{-7, 14, 28} {
			label := "pre"
			if day >= 0 {
				label = "d" + strconv.Itoa(day)
			}
			id := subject + "-" + label
			file := id + ".tsv"
			abundance, err := writeBCellSample(ctx, filepath.Join(stage, file), id, nodes, bcellWeights(s.BCells.Families, ti), s.BCells.CellsPerSample, random.New(s.Seed, uint64(20000+si*3+ti)), observations)
			if err != nil {
				return Summary{}, err
			}
			truth.Samples = append(truth.Samples, bcellSample{id, subject, day, s.BCells.CellsPerSample, abundance})
			repertoires = append(repertoires, bcellRepertoire(s, subject, id, file, version, day))
			summary.Files = append(summary.Files, file)
			summary.Samples++
			summary.Rows += 2 * s.BCells.CellsPerSample
		}
	}
	observations.Flush()
	if err := observations.Error(); err != nil {
		return Summary{}, err
	}
	if err := obs.Close(); err != nil {
		return Summary{}, err
	}
	if err := writeJSON(filepath.Join(stage, "repertoires.airr.json"), map[string]any{"Repertoire": repertoires}); err != nil {
		return Summary{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "README.md"), []byte(bcellReadme(s, version)), 0o644); err != nil {
		return Summary{}, err
	}
	summary.Files = append(summary.Files, "repertoires.airr.json", "README.md", "truth/observations.tsv")
	sort.Strings(summary.Files)
	for _, file := range summary.Files {
		data, err := os.ReadFile(filepath.Join(stage, file))
		if err != nil {
			return Summary{}, err
		}
		truth.Files = append(truth.Files, bcellFile{file, fmt.Sprintf("%x", sha256.Sum256(data))})
	}
	if err := writeJSON(filepath.Join(stage, "truth", "truth.json"), truth); err != nil {
		return Summary{}, err
	}
	summary.Files = append(summary.Files, "truth/truth.json")
	sort.Strings(summary.Files)
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	// Remove only an empty existing directory. Remove fails safely if another writer filled it.
	if entries, err := os.ReadDir(dir); err == nil {
		if len(entries) != 0 {
			return Summary{}, fmt.Errorf("%s is not empty", dir)
		}
		if err := os.Remove(dir); err != nil {
			return Summary{}, err
		}
	} else if !os.IsNotExist(err) {
		return Summary{}, err
	}
	if err := os.Rename(stage, dir); err != nil {
		return Summary{}, err
	}
	progress(1, "Done")
	return summary, nil
}

func bcellReadme(s Study, version string) string {
	return fmt.Sprintf(`# %s

Artificial paired B-cell data made by ClonoLoom %s, model %s, seed %d.
Open this folder in ClonoLoom: the AIRR samples and repertoires.airr.json are observations;
truth/ is excluded from the dataset. Each cell has one heavy and one kappa or lambda chain.
No reads or UMIs are simulated. Counts are cells, not sequence rows.

truth/truth.json records all effective parameters, every subject-specific family, its seven
nodes (including the ancestor), parent IDs, isotypes, sequences, ordered per-edge substitutions
(1-based positions; silent/replacement relative to the codon immediately before the event), and
each sample's node probabilities and realized cell counts, including zero observations.
truth/observations.tsv links observed cell IDs to nodes. The manifest hashes every other file.
A node is a known simulated state, not an inferred clonotype or a distinct nucleotide sequence.

Three time points: days -7, 14 and 28. Nodes form a fixed binary tree: parents of N1/N2 are N0,
of N3/N4 are N1, of N5/N6 are N2. Isotypes N0–N6 are M, M, G1, M, G1, G1, A1.
Within each family the sampling weights are [1,0,0,0,0,0,0], [1,4,4,0,0,0,0], and
[1,1,1,4,4,4,4], normalized. The first ceil(families/4) families have abundance multipliers
1, 8 and 3 respectively; all others have multiplier 1. Independent categorical draws sample
cells from the normalized joint probabilities. There is no additional biological variance.

Sequences are artificial 360-nt coding strings, with shared backbones for two fictional V calls
per locus and random family junctions at 283–321. The junction anchors are Cys and Trp/Phe.
SIM gene names are not reference alleles. germline_alignment holds the known rearranged ancestor,
including its artificial junction, not a reconstructed germline or a D-masked reference.
FWR/CDR coordinates describe artificial contiguous regions, not IMGT numbering. D calls, CIGARs,
reference-segment coordinates and molecule counts are absent, rather than invented.

Each edge changes each chain by the configured number of sequential substitutions: propose a
uniform position and base, reject unchanged bases, anchor changes and stop codons. N3's first
event deliberately undoes N1's last event; further events can revisit sites. Accumulated event
counts therefore differ from net differences to an ancestor. Isotype changes are fixed forward
transitions in a subset of the human constant-region order; no switch rate is estimated.

This is ClonoLoom's deliberately simple benchmark, not a fitted SHM, recombination, germinal-center
or affinity model. It omits motif-dependent mutation (Yaari et al. 2013,
doi:10.3389/fimmu.2013.00358), indels, sequencing error, dropout, selection and alternative trees.
Class-switch context: Horns et al. 2016, doi:10.7554/eLife.16578. Neither paper's model is reproduced.
Known truth is for testing implementations, not evidence of accuracy on biological repertoires.

Reproduce: extract the study object from truth/truth.json into study.json, then run
clonoloom simulate --params study.json --out another-folder with the same program version.
The data describe no person and may be used and shared without restriction.
`, s.Title, version, bcellModel, s.Seed)
}

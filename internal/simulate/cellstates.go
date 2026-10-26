package simulate

// A controlled paired receptor/expression benchmark, not a fitted biological model. All state,
// clone, mean and layout choices are ClonoLoom's. Counts use the existing gamma–Poisson sampler
// (Marsaglia & Tsang 2000, doi:10.1145/358407.358414; Hörmann 1993,
// doi:10.1016/0167-7152(93)90097-4). MEX follows Boisvert et al. 1996, NISTIR 5935 and
// https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/cr-outputs-mex-matrices.
// See docs/SIMULATION.md for every effective parameter and deliberate missingness rule.
// Optional Gaussian layout reuses the polar normal sampler (Marsaglia & Bray 1964,
// doi:10.1137/1006063), with an independent stream; it is not an expression embedding.
import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/random"
)

const cellStateModel = "clonoloom-artificial-cell-states-v1"

var cellStateColumns = []string{"sequence_id", "sequence", "rev_comp", "productive", "v_call", "d_call", "j_call", "sequence_alignment", "germline_alignment", "junction", "junction_aa", "v_cigar", "d_cigar", "j_cigar", "cell_id", "repertoire_id", "locus"}

type CellStateStudy struct {
	Subjects       int `json:"subjects"`
	CellsPerSample int `json:"cellsPerSample"`
	Clones         int `json:"clones"`
	// Empty retains the original grid fixture and its exact source bytes.
	Layout string `json:"layout,omitempty"`
}

func (c CellStateStudy) check(s Study) error {
	if c.Layout != "" && c.Layout != "gaussian" {
		return fmt.Errorf("cellStates layout must be empty (legacy grid) or gaussian")
	}
	if s.BCells != nil || s.Series != nil || len(s.Groups) != 0 || s.SamplesPerSubject != 0 || len(s.Templates) != 0 || s.ReadsPerTemplate != 0 || s.ReadsSize != 0 || !reflect.DeepEqual(s.Recombination, Recombination{}) {
		return fmt.Errorf("cellStates replaces the other models: omit their parameters")
	}
	if c.Subjects < 1 || c.Subjects > 20 || c.CellsPerSample < 12 || c.CellsPerSample > 10000 || c.Clones < 3 || c.Clones > 100 || c.Subjects*c.CellsPerSample*2 > 100000 {
		return fmt.Errorf("cellStates needs subjects 1–20, cellsPerSample 12–10000, clones 3–100; at most 100000 expression cells")
	}
	return nil
}

type cellStateTruth struct {
	CloneSampling string      `json:"cloneSampling,omitempty"`
	Model         string      `json:"model"`
	Simulator     string      `json:"simulator"`
	Study         Study       `json:"study"`
	States        []string    `json:"states"`
	Means         [][]int     `json:"means"`
	Size          int         `json:"size"`
	Files         []bcellFile `json:"files"`
}

// Artificial junctions only: no copied reference alleles or implied antigen specificity.
func cellStateReceptors(seed uint64, subject, clone int) [2]string {
	r := random.New(seed, uint64(30000+subject*100+clone))
	var out [2]string
	for chain := range out {
		seq := "TGT"
		for range 12 {
			for {
				codon := string([]byte{"ACGT"[r.IntN(4)], "ACGT"[r.IntN(4)], "ACGT"[r.IntN(4)]})
				if Translate(codon) != "*" {
					seq += codon
					break
				}
			}
		}
		out[chain] = seq + "TTT"
	}
	return out
}

func runCellStates(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0755); err != nil {
		return Summary{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-cell-states-")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(stage)
	if err := os.Mkdir(filepath.Join(stage, "truth"), 0755); err != nil {
		return Summary{}, err
	}
	c := s.CellStates
	summary := Summary{Dir: dir, Subjects: c.Subjects, Samples: c.Subjects * 2}
	truth := cellStateTruth{Model: cellStateModel, Simulator: "ClonoLoom simulate " + version, Study: s, States: []string{"state A", "state B", "state C"}, Size: 4}
	if c.Layout == "gaussian" {
		truth.Model = "clonoloom-artificial-cell-states-clouds-v1"
	}
	cloneWeights := make([]float64, c.Clones)
	if s.Name == "pseudobulk-study" {
		truth.Model = "clonoloom-artificial-pseudobulk-v1"
		truth.CloneSampling = fmt.Sprintf("Pr(clone rank k) proportional to 1/k, k=1..%d; sampled separately per visit", c.Clones)
		var total float64
		for i := range cloneWeights {
			total += 1 / float64(i+1)
			cloneWeights[i] = total
		}
	}
	for state := range 3 {
		means := make([]int, 24)
		for gene := range means {
			means[gene] = 2
			if gene/8 == state {
				means[gene] = 16
			}
		}
		truth.Means = append(truth.Means, means)
	}
	write := func(name, data string) error {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(stage, name)), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(stage, name), []byte(data), 0644); err != nil {
			return err
		}
		summary.Files = append(summary.Files, name)
		return nil
	}
	var observations strings.Builder
	observations.WriteString("sample_id\tbarcode\tclone_id\tstate\texpression\treceptor\tannotation\tcoordinates\tgene_umis\n")
	var repertoires []map[string]any
	for subject := range c.Subjects {
		subjectID := fmt.Sprintf("S%02d", subject+1)
		clones := make([][2]string, c.Clones)
		for clone := range clones {
			clones[clone] = cellStateReceptors(s.Seed, subject, clone)
		}
		for sample := range 2 {
			if err := ctx.Err(); err != nil {
				return Summary{}, err
			}
			id := fmt.Sprintf("%s-%d", subjectID, sample+1)
			progress(float64(subject*2+sample)/float64(summary.Samples), "Making "+id+"'s expression and paired receptors")
			r := random.New(s.Seed, uint64(40000+subject*2+sample))
			layout := random.New(s.Seed, uint64(50000+subject*2+sample))
			var receptor, barcodes, features, entries, annotations strings.Builder
			receptor.WriteString(strings.Join(cellStateColumns, "\t") + "\n")
			annotations.WriteString("barcode\tstate\tmap_x\tmap_y\n")
			for gene := range 24 {
				fmt.Fprintf(&features, "SIM-G%02d\tArtificial gene %02d\tGene Expression\n", gene+1, gene+1)
			}
			nnz := 0
			for cell := 0; cell <= c.CellsPerSample; cell++ {
				if err := ctx.Err(); err != nil {
					return Summary{}, err
				}
				barcode := fmt.Sprintf("CELL%05d-1", cell+1) // deliberately reused across samples
				clone := r.IntN(c.Clones)
				if s.Name == "pseudobulk-study" {
					clone = r.Discrete(cloneWeights)
				}
				state := clone % 3
				if sample == 1 && r.IntN(2) == 0 {
					state = (state + 1) % 3
				}
				hasExpression := cell < c.CellsPerSample
				hasReceptor := cell != 0
				hasAnnotation := hasExpression && cell != 1
				hasCoordinates := hasAnnotation && cell != 2
				total := 0
				if hasExpression {
					fmt.Fprintln(&barcodes, barcode)
					for gene, mean := range truth.Means[state] {
						count := r.NegBinomial(float64(mean), float64(truth.Size))
						if cell == c.CellsPerSample-1 {
							count = 0
						}
						if count > 0 {
							fmt.Fprintf(&entries, "%d %d %d\n", gene+1, cell+1, count)
							nnz++
							total += count
						}
					}
				}
				if hasReceptor {
					for chain, locus := range []string{"TRA", "TRB"} {
						nt := clones[clone][chain]
						fields := []string{id + "-" + barcode + "-" + locus, nt, "F", "T", locus + "V1-SIM*01", "", locus + "J1-SIM*01", "", "", nt, Translate(nt), "", "", "", barcode, id, locus}
						receptor.WriteString(strings.Join(fields, "\t") + "\n")
						summary.Rows++
					}
				}
				if hasAnnotation {
					label := truth.States[state]
					if cell == 3 {
						label = ""
					}
					x, y := "", ""
					if hasCoordinates {
						x = fmt.Sprint(state*10 + r.IntN(5) - 2)
						y = fmt.Sprint(sample*3 + r.IntN(5) - 2)
						if c.Layout == "gaussian" {
							// Keep the original model's draws above: layout never changes biological
							// observations at fixed size/clone parameters. Round products before sums.
							px := float64(state*8) + float64(1.5*layout.Normal())
							py := float64(sample*3) + float64([3]float64{1.2, 2, 0.8}[state]*layout.Normal())
							x, y = strconv.FormatFloat(px, 'g', 17, 64), strconv.FormatFloat(py, 'g', 17, 64)
						}
					}
					fmt.Fprintf(&annotations, "%s\t%s\t%s\t%s\n", barcode, label, x, y)
				}
				umi := fmt.Sprint(total)
				if !hasExpression {
					umi = ""
				}
				fmt.Fprintf(&observations, "%s\t%s\t%s-C%03d\t%s\t%t\t%t\t%t\t%t\t%s\n", id, barcode, subjectID, clone+1, truth.States[state], hasExpression, hasReceptor, hasAnnotation, hasCoordinates, umi)
			}
			annotations.WriteString("ANNOTATION-ONLY\tstate A\t0\t0\n")
			prefix := "expression/" + id + "/"
			for _, file := range []struct{ name, data string }{{id + ".tsv", receptor.String()}, {prefix + "barcodes.tsv", barcodes.String()}, {prefix + "features.tsv", features.String()}, {prefix + "matrix.mtx", fmt.Sprintf("%%%%MatrixMarket matrix coordinate integer general\n24 %d %d\n", c.CellsPerSample, nnz) + entries.String()}, {prefix + "cells.tsv", annotations.String()}} {
				if err := write(file.name, file.data); err != nil {
					return Summary{}, err
				}
			}
			metadata := repertoire(s, Subject{ID: subjectID, Condition: "artificial cell states"}, id, id+".tsv", version)
			ms := metadata["sample"].([]any)[0].(map[string]any)
			if s.Name == "pseudobulk-study" {
				ms["collection_time_point_relative"] = sample * 14
				ms["collection_time_point_relative_unit"] = map[string]any{"label": "day"}
				ms["collection_time_point_reference"] = "artificial baseline"
			}
			ms["template_class"] = "RNA"
			ms["single_cell"] = true
			ms["pcr_target"] = []any{map[string]any{"pcr_target_locus": "TRA"}, map[string]any{"pcr_target_locus": "TRB"}}
			repertoires = append(repertoires, metadata)
		}
	}
	if err := write("truth/observations.tsv", observations.String()); err != nil {
		return Summary{}, err
	}
	if err := writeJSON(filepath.Join(stage, "repertoires.airr.json"), map[string]any{"Repertoire": repertoires}); err != nil {
		return Summary{}, err
	}
	summary.Files = append(summary.Files, "repertoires.airr.json")
	readme := fmt.Sprintf(`# %s

Artificial study, model %s, seed %d; ClonoLoom %s.

Open the root AIRR tables and repertoires.airr.json as one receptor dataset.
For each sample select expression/<sample> and its cells.tsv explicitly in Cells.
The supplied map is artificial layout, not UMAP, clustering or antigen specificity.
Repeated barcodes are scoped to samples. Truth is in truth/observations.tsv and truth/truth.json.

Two samples per subject, %d cells per matrix, %d clones per subject.
24 artificial genes in three blocks of eight; means 16 in the state's block, 2 elsewhere;
independent negative-binomial counts, size 4 (variance mean + mean squared / 4).
Clones are uniform; first-sample state is clone index modulo 3, second-sample state
advances modulo 3 with probability one half. These are chosen software test parameters.
Cell 1 lacks receptors; cell 2 lacks annotations; cell 3 lacks coordinates; cell 4 lacks a label;
the final matrix cell has all-zero expression. One extra paired receptor barcode and one
annotation-only barcode are included. No biological dropout or sequencing-error model is fitted.
Coordinates: x = 10*state + uniform integer -2..2; y = 3*sample + uniform integer -2..2
(zero-based state/sample), drawn only for mapped cells. Fictional SIM calls and random
stop-free 42-nt junctions; no real alleles. All data are artificial and unrestricted.
See docs/SIMULATION.md for the sampler references and reproducibility contract.
`, s.Title, truth.Model, s.Seed, version, c.CellsPerSample, c.Clones)
	if c.Layout == "gaussian" {
		readme = strings.Replace(readme, "Coordinates: x = 10*state + uniform integer -2..2; y = 3*sample + uniform integer -2..2\n(zero-based state/sample), drawn only for mapped cells.", "Supplied Gaussian layout: x = 8*state + 1.5*N(0,1); y = 3*sample + sigma*N(0,1),\nwith sigma [1.2,2,0.8] by state (zero-based state/sample). Independent layout stream\n50000 + 2*subject + sample; original integer jitter draws are retained and discarded\nto preserve all biological observations at fixed parameters. Layout is illustrative,\nnot fitted or computed from expression. Ten uniform clones give first-sample state\nprobabilities 0.4,0.3,0.3 in the preset; these probabilities depend on clone count.", 1)
	}
	if s.Name == "pseudobulk-study" {
		readme = strings.Replace(readme, "Clones are uniform;", fmt.Sprintf("Clone probabilities are proportional to 1/rank over %d clones, sampled per visit;", c.Clones), 1)
		readme = strings.Replace(readme, "Ten uniform clones give first-sample state\nprobabilities 0.4,0.3,0.3 in the preset; these probabilities depend on clone count.", "State probabilities follow the declared clone-rank weights modulo 3; they are construction choices.", 1)
	}
	if err := write("README.md", readme); err != nil {
		return Summary{}, err
	}
	sort.Strings(summary.Files)
	for _, name := range summary.Files {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		raw, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			return Summary{}, err
		}
		truth.Files = append(truth.Files, bcellFile{name, fmt.Sprintf("%x", sha256.Sum256(raw))})
	}
	if err := writeJSON(filepath.Join(stage, "truth/truth.json"), truth); err != nil {
		return Summary{}, err
	}
	summary.Files = append(summary.Files, "truth/truth.json")
	sort.Strings(summary.Files)
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
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

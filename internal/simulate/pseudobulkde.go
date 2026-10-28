package simulate

// Artificial paired pseudobulk study. Shared gene/category gamma rates create variation
// between subjects' pseudobulks, rather than pretending that cells replicate subjects.
// Gamma/Poisson samplers: Marsaglia & Tsang (2000); Hörmann (1993). Research §5.40.
import (
	"clonoloom/internal/random"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PseudobulkDEStudy struct {
	Subjects int  `json:"subjects"`
	Null     bool `json:"null"`
}

func (p PseudobulkDEStudy) check(s Study) error {
	if p.Subjects < 25 || p.Subjects > 32 {
		return fmt.Errorf("pseudobulk DE requires 25–32 artificial subjects")
	}
	if s.BatchQC != nil || s.Contamination != nil || s.Convergence != nil || s.Motifs != nil || s.Counts != nil || s.CellStates != nil || s.BCells != nil || s.Series != nil || len(s.Groups) > 0 || s.SamplesPerSubject != 0 || len(s.Templates) > 0 || s.ReadsPerTemplate != 0 || s.ReadsSize != 0 {
		return fmt.Errorf("pseudobulk DE replaces other simulation models")
	}
	return nil
}
func runPseudobulkDE(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if e := ctx.Err(); e != nil {
		return Summary{}, e
	}
	if e := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0755); e != nil {
		return Summary{}, e
	}
	stage, e := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-pseudobulk-de-")
	if e != nil {
		return Summary{}, e
	}
	defer os.RemoveAll(stage)
	sum := Summary{Dir: dir, Subjects: s.PseudobulkDE.Subjects, Samples: s.PseudobulkDE.Subjects}
	write := func(name, data string) error {
		p := filepath.Join(stage, name)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return e
		}
		if e := os.WriteFile(p, []byte(data), 0644); e != nil {
			return e
		}
		sum.Files = append(sum.Files, name)
		return nil
	}
	var truth, counts strings.Builder
	truth.WriteString("sample\tsubject\tbarcode\tclone\tsize_category\tcell_type\n")
	counts.WriteString("sample\tsize_category\tfeature_id\traw_sum\tmean_per_cell\tcategory_fold\n")
	var repertoires []map[string]any
	for subject := 0; subject < s.PseudobulkDE.Subjects; subject++ {
		if e := ctx.Err(); e != nil {
			return Summary{}, e
		}
		id := fmt.Sprintf("S%02d-baseline", subject+1)
		sid := fmt.Sprintf("S%02d", subject+1)
		progress(float64(subject)/float64(s.PseudobulkDE.Subjects), "Making "+id+"'s paired category counts")
		r := random.New(s.Seed, uint64(71000+subject))
		layout := random.New(s.Seed, uint64(72000+subject))
		singles := 48 + subject%5
		sizes := []int{7, 9, 11, 13, 17, 21, 26, 33}
		clones := []int{}
		for j := 0; j < singles; j++ {
			clones = append(clones, j)
		}
		for j, n := range sizes {
			for k := 0; k < n; k++ {
				clones = append(clones, singles+j)
			}
		}
		means := [2][]float64{}
		totals := [2][]uint64{}
		folds := make([]float64, 40)
		for bin := range 2 {
			means[bin] = make([]float64, 40)
			totals[bin] = make([]uint64, 40)
		}
		for g := range 40 {
			folds[g] = 1
			if !s.PseudobulkDE.Null {
				if g < 2 {
					folds[g] = 3
				} else if g < 4 {
					folds[g] = 1.0 / 3
				}
			}
			base := float64(2+g) / 3
			subjectRate := r.Gamma(16) / 16
			for bin := range 2 {
				rate := base * subjectRate
				rate = float64(rate * (r.Gamma(5) / 5))
				if bin == 1 {
					rate = float64(rate * folds[g])
				}
				if g == 38 {
					rate = .005
				}
				if g == 39 {
					rate = 0
				}
				if g == 37 && subject == 0 {
					rate = 0
				}
				means[bin][g] = rate
			}
		}
		var receptor, barcodes, features, entries, annotations strings.Builder
		receptor.WriteString(strings.Join(cellStateColumns, "\t") + "\n")
		annotations.WriteString("barcode\tcell_type\tmap_x\tmap_y\n")
		for g := range 40 {
			fmt.Fprintf(&features, "SIM-G%02d\tArtificial gene %02d\tGene Expression\n", g+1, g+1)
		}
		nnz := 0
		for cell, clone := range clones {
			if e := ctx.Err(); e != nil {
				return Summary{}, e
			}
			barcode := fmt.Sprintf("CELL%05d-1", cell+1)
			bin := 0
			if clone >= singles {
				bin = 1
			}
			category := []string{"1", ">1"}[bin]
			fmt.Fprintln(&barcodes, barcode)
			x := float64(1.7 * layout.Normal())
			y := float64(.6*x) + float64(1.2*layout.Normal())
			fmt.Fprintf(&annotations, "%s\tArtificial T cells\t%.17g\t%.17g\n", barcode, x, y)
			for g, mean := range means[bin] {
				n := r.Poisson(mean)
				totals[bin][g] += uint64(n)
				if n > 0 {
					fmt.Fprintf(&entries, "%d %d %d\n", g+1, cell+1, n)
					nnz++
				}
			}
			pair := cellStateReceptors(s.Seed, subject*10, clone)
			for chain, nt := range pair {
				locus := []string{"TRA", "TRB"}[chain]
				fields := []string{fmt.Sprintf("%s-%s-%s", id, barcode, locus), nt, "F", "T", locus + "V1-SIM*01", "", locus + "J1-SIM*01", "", "", nt, Translate(nt), "", "", "", barcode, id, locus}
				fmt.Fprintln(&receptor, strings.Join(fields, "\t"))
				sum.Rows++
			}
			fmt.Fprintf(&truth, "%s\t%s\t%s\t%s-C%03d\t%s\tArtificial T cells\n", id, sid, barcode, sid, clone+1, category)
		}
		for bin := range 2 {
			for g, total := range totals[bin] {
				fmt.Fprintf(&counts, "%s\t%s\tSIM-G%02d\t%d\t%.17g\t%.17g\n", id, []string{"1", ">1"}[bin], g+1, total, means[bin][g], folds[g])
			}
		}
		prefix := "expression/" + id + "/"
		for _, f := range []struct{ name, data string }{{id + ".tsv", receptor.String()}, {prefix + "barcodes.tsv", barcodes.String()}, {prefix + "features.tsv", features.String()}, {prefix + "matrix.mtx", fmt.Sprintf("%%%%MatrixMarket matrix coordinate integer general\n40 %d %d\n", len(clones), nnz) + entries.String()}, {prefix + "cells.tsv", annotations.String()}} {
			if e := write(f.name, f.data); e != nil {
				return Summary{}, e
			}
		}
		m := repertoire(s, Subject{ID: sid, Condition: "artificial paired category study"}, id, id+".tsv", version)
		ms := m["sample"].([]any)[0].(map[string]any)
		ms["collection_time_point_relative"] = 0
		ms["collection_time_point_relative_unit"] = map[string]any{"label": "day"}
		ms["collection_time_point_reference"] = "artificial baseline"
		ms["single_cell"] = true
		ms["template_class"] = "RNA"
		ms["pcr_target"] = []any{map[string]any{"pcr_target_locus": "TRA"}, map[string]any{"pcr_target_locus": "TRB"}}
		repertoires = append(repertoires, m)
	}
	if e := writeJSON(filepath.Join(stage, "repertoires.airr.json"), map[string]any{"Repertoire": repertoires}); e != nil {
		return Summary{}, e
	}
	sum.Files = append(sum.Files, "repertoires.airr.json")
	for _, f := range []struct{ name, data string }{{"truth/observations.tsv", truth.String()}, {"truth/counts.tsv", counts.String()}, {"README.md", fmt.Sprintf(`# %s

Artificial paired pseudobulk study; seed %d, %d independent simulated subjects,
one baseline sample each, 40 fictional genes. No biological identities or fitted biology.
Open all root AIRR files together. In Cells open expression/<sample> and cells.tsv,
save paired TRA/TRB phenotypes with cell_type and receptor-size breakpoints [1].
Aggregate all samples with minimum 10 cells. Compare type Artificial T cells,
literal timepoint 0 day (artificial baseline), reference 1, comparison >1; minCount 10, minSamples 25.

Each subject has 48 + zero-based subject index modulo 5 singleton clones and
8 larger clones of 7,9,11,13,17,21,26,33 cells. Receptors have fictional SIM calls.
For gene index g=0..39, base mean (2+g)/3; shared subject/gene Gamma(16,1)/16,
independent category/gene Gamma(5,1)/5, then conditionally Poisson counts per cell.
Positive preset: genes 1–2 have category fold 3, genes 3–4 fold 1/3; others 1.
Null preset: all category folds 1. Null preset selected: %t.
Gene 39 has per-cell mean .005; gene 40 has zero counts; gene 38 is zero in subject 1.
Counts are not copied between cells. Gamma variation is shared across a subject's
cells within each category. Raw pseudobulk sums and generating rates are in truth/counts.tsv.
Supplied x=1.7*N(0,1), y=.6*x+1.2*N(0,1) is an illustrative cloud, not an embedding.
Expression and receptor-size categories are deliberately constructed, not an antigen,
proliferation, dropout or clinical model. Truth lists all assignments and source hashes.
`, s.Title, s.Seed, s.PseudobulkDE.Subjects, s.PseudobulkDE.Null)}} {
		if e := write(f.name, f.data); e != nil {
			return Summary{}, e
		}
	}
	sort.Strings(sum.Files)
	files := []bcellFile{}
	for _, name := range sum.Files {
		b, e := os.ReadFile(filepath.Join(stage, name))
		if e != nil {
			return Summary{}, e
		}
		files = append(files, bcellFile{name, fmt.Sprintf("%x", sha256.Sum256(b))})
	}
	if e := os.MkdirAll(filepath.Join(stage, "truth"), 0755); e != nil {
		return Summary{}, e
	}
	if e := writeJSON(filepath.Join(stage, "truth/truth.json"), map[string]any{"model": "clonoloom-artificial-pseudobulk-de-v1", "simulator": "ClonoLoom simulate " + version, "study": s, "files": files}); e != nil {
		return Summary{}, e
	}
	sum.Files = append(sum.Files, "truth/truth.json")
	sort.Strings(sum.Files)
	if e := ctx.Err(); e != nil {
		return Summary{}, e
	}
	if files, e := os.ReadDir(dir); e == nil {
		if len(files) > 0 {
			return Summary{}, fmt.Errorf("%s is not empty", dir)
		}
		if e = os.Remove(dir); e != nil {
			return Summary{}, e
		}
	} else if !os.IsNotExist(e) {
		return Summary{}, e
	}
	if e := os.Rename(stage, dir); e != nil {
		return Summary{}, e
	}
	progress(1, "Done")
	return sum, nil
}

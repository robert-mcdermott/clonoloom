package simulate

// Replicated repertoire examples: explicit gamma-Poisson variation, uneven sampling,
// subject effects, two batches, sparse shared changes and a private long tail.
// This is a controlled benchmark, not a fitted human repertoire/response model.
// Existing random samplers cite Marsaglia–Tsang and Hörmann; research.md §5.33.
import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"clonoloom/internal/dmath"
	"clonoloom/internal/random"
)

type CountStudy struct {
	Subjects           int    `json:"subjects"`
	Layout             string `json:"layout"`
	Null               bool   `json:"null"`
	Unbalanced         bool   `json:"unbalanced,omitempty"`
	Confounded         bool   `json:"confounded,omitempty"`
	DefinitionVariants bool   `json:"definitionVariants,omitempty"`
}

func (c CountStudy) check(s Study) error {
	if c.Subjects < 12 || c.Subjects > 32 || (c.Layout != "paired" && c.Layout != "independent") || c.Layout == "independent" && c.Subjects%2 != 0 {
		return fmt.Errorf("count study needs 12–32 subjects, paired or independent layout, and even subjects for independent groups")
	}
	if (c.Unbalanced || c.Confounded) && (c.Layout != "independent" || c.Subjects != 32 || c.DefinitionVariants || c.Unbalanced && c.Confounded) {
		return fmt.Errorf("unbalanced/confounded controls require 32 independent subjects and one control variant")
	}
	if c.DefinitionVariants && (c.Layout != "independent" || c.Null) {
		return fmt.Errorf("definition-variant study requires independent groups and cannot be an all-null count study")
	}
	if s.BCells != nil || s.CellStates != nil || s.Series != nil || len(s.Groups) > 0 {
		return fmt.Errorf("count study replaces other simulation models")
	}
	return nil
}
func runCounts(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0755); err != nil {
		return Summary{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-counts-")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(stage)
	if err = os.Mkdir(filepath.Join(stage, "truth"), 0755); err != nil {
		return Summary{}, err
	}
	c := s.Counts
	visits := 1
	if c.Layout == "paired" {
		visits = 2
	}
	sum := Summary{Dir: dir, Subjects: c.Subjects, Samples: c.Subjects * visits}
	var truth strings.Builder
	truth.WriteString("sample\tsubject\tgroup\tbatch\tclone\tjunction\texposure\tmean\tdispersion\tlog2_effect\tcount")
	if c.DefinitionVariants {
		truth.WriteString("\tv_call\tj_call\tjunction_aa")
	}
	truth.WriteString("\n")
	var reps []map[string]any
	write := func(name, data string) error {
		if err := os.WriteFile(filepath.Join(stage, name), []byte(data), 0644); err != nil {
			return err
		}
		sum.Files = append(sum.Files, name)
		return nil
	}
	for subject := 0; subject < c.Subjects; subject++ {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		r := random.New(s.Seed, uint64(800000+subject))
		id := fmt.Sprintf("S%02d", subject+1)
		subjectEffects := make([]float64, 48)
		for i := range subjectEffects {
			subjectEffects[i] = dmath.Exp(.4 * r.Normal())
		}
		for visit := 0; visit < visits; visit++ {
			sample := fmt.Sprintf("%s-%d", id, visit+1)
			group := visit
			if c.Layout == "independent" {
				boundary := c.Subjects / 2
				if c.Unbalanced {
					boundary = 12
				}
				if subject >= boundary {
					group = 1
				}
			}
			batch := (subject + visit) % 2
			if c.Confounded {
				batch = group
			}
			exposure := dmath.Exp(.45 * r.Normal())
			var out strings.Builder
			out.WriteString("sequence_id\tsequence\trev_comp\tproductive\tv_call\td_call\tj_call\tsequence_alignment\tgermline_alignment\tjunction\tjunction_aa\tv_cigar\td_cigar\tj_cigar\tlocus\tumi_count\trepertoire_id\n")
			for clone := 0; clone < 648; clone++ {
				// Shared panel has distinct heterogeneity; private clones occur in one subject.
				// Stop-free synthetic identities reuse only the project's own sequence generator.
				owner := subject + 1
				if clone < 48 {
					owner = 0
				}
				identity := clone
				variant := 0
				if c.DefinitionVariants && clone >= 48 && group == 1 {
					identity = 48 + (clone-48)/3
					variant = (clone - 48) % 3
				}
				nt := cellStateReceptors(s.Seed, owner*100, identity)[1]
				vcall, jcall := "SIMTRBV1*01", "SIMTRBJ1*01"
				if c.DefinitionVariants {
					vcall, jcall = "TRBV5-1*01", "TRBJ2-7*01"
				}
				if c.DefinitionVariants && clone >= 48 && group == 1 {
					if variant == 1 {
						nt = "TGC" + nt[3:]
						vcall = "TRBV5-1*02"
					}
					if variant == 2 {
						nt = nt[:len(nt)-3] + "TTC"
						vcall = "TRBV5-2*01"
					}
				}
				mean := 0.0
				if clone >= 48 {
					mean = 5000 / dmath.Pow(float64(clone-48+1), .7)
					if c.DefinitionVariants {
						mean = 1800 / dmath.Pow(float64(clone-48+1), 1.2)
						if group == 1 {
							mean *= 1.6
						}
					}
				}
				alpha := .4
				effect := 0.0
				if clone < 48 {
					mean = 8 * dmath.Exp(float64(clone%16)/4)
					alpha = .05 + float64(float64(clone%6)*.06)
					mean *= subjectEffects[clone]
					if !c.Null {
						if clone < 4 {
							effect = 2
						}
						if clone >= 4 && clone < 8 {
							effect = -1.5
						}
					}
					if batch == 1 {
						mean *= 1.2
					}
					if group == 1 {
						mean *= dmath.Pow(2, effect)
					}
				}
				mean *= exposure
				count := r.Poisson(r.Gamma(1/alpha) * alpha * mean)
				fmt.Fprintf(&truth, "%s\t%s\t%d\tB%d\t%d:%d\t%s\t%.17g\t%.17g\t%.17g\t%.17g\t%d", sample, id, group, batch+1, owner, clone, nt, exposure, mean, alpha, effect, count)
				if c.DefinitionVariants {
					fmt.Fprintf(&truth, "\t%s\t%s\t%s", vcall, jcall, Translate(nt))
				}
				truth.WriteString("\n")
				if count == 0 {
					continue
				}
				fmt.Fprintf(&out, "%s-%04d\t%s\tF\tT\t%s\t\t%s\t\t\t%s\t%s\t\t\t\tTRB\t%d\t%s\n", sample, clone, nt, vcall, jcall, nt, Translate(nt), count, sample)
				sum.Rows++
			}
			if err = write(sample+".tsv", out.String()); err != nil {
				return Summary{}, err
			}
			condition := []string{"control", "stimulated"}[group]
			if c.Layout == "paired" {
				condition = "paired stimulation"
			}
			meta := repertoire(s, Subject{ID: id, Condition: condition}, sample, sample+".tsv", version)
			ms := meta["sample"].([]any)[0].(map[string]any)
			ms["collection_time_point_relative"] = 14 * visit
			ms["collection_time_point_relative_unit"] = map[string]any{"id": "UO:0000033", "label": "day"}
			ms["collection_time_point_reference"] = "simulated stimulation"
			meta["study"].(map[string]any)["study_id"] = "clonoloom-simulated-replicated-counts-v1"
			if c.Unbalanced || c.Confounded {
				meta["study"].(map[string]any)["study_id"] = "clonoloom-simulated-" + s.Name
			}
			if c.DefinitionVariants {
				meta["study"].(map[string]any)["study_id"] = "clonoloom-simulated-definition-variants-v1"
			}
			// Batch annotations are supplied explicitly from truth in the worked example.
			reps = append(reps, meta)
		}
		progress(float64(subject+1)/float64(c.Subjects), "Simulating replicated count variation")
	}
	if err = write("truth/observations.tsv", truth.String()); err != nil {
		return Summary{}, err
	}
	if err = writeJSON(filepath.Join(stage, "repertoires.airr.json"), map[string]any{"Repertoire": reps}); err != nil {
		return Summary{}, err
	}
	sum.Files = append(sum.Files, "repertoires.airr.json")
	readme := fmt.Sprintf("# %s\n\nArtificial replicated-count benchmark v1, seed %d. %d subjects, %s layout.\n48 shared fictional receptors and 600 private receptors per subject, with a long-tailed abundance distribution.\nCounts represent simulated UMIs (AIRR umi_count). Gamma-Poisson counts have heterogeneous dispersions; sampling exposures vary lognormally.\nShared receptors have subject effects and a 1.2-fold B2 batch effect. Four shared receptors increase fourfold,\nfour decrease by 2^1.5, and forty are unchanged (all effects zero in the null preset).\nBatch is B1/B2 alternating by zero-based subject plus visit. Truth records every mean, dispersion, exposure, effect and count.\nThe exposure is a generator scale, not the realized library total: total-count-offset effects are relative repertoire effects.\nFictional stop-free junctions and SIM genes do not imply specificity or a biological recombination model.\nOpen root TSVs and AIRR metadata together; annotate the recorded batches before batch-adjusted modeling.\n", s.Title, s.Seed, c.Subjects, c.Layout)
	if c.Unbalanced {
		readme += "\nUnbalanced control: 12 control and 20 stimulated subjects, with both batches represented in each group. No subjects were excluded or selected by outcome.\n"
	}
	if c.Confounded {
		readme = strings.ReplaceAll(readme, "Batch is B1/B2 alternating by zero-based subject plus visit.", "Batch is B1 for every control and B2 for every stimulated subject: batch and condition are fully confounded.")
		readme += "\nThis design cannot distinguish a batch effect from a group effect. Retain the failed design check; do not fit or interpret a group contrast.\n"
	}
	if c.DefinitionVariants {
		readme = strings.ReplaceAll(readme, "Fictional stop-free junctions and SIM genes", "Fictional stop-free junctions and arbitrary gene labels")
		readme += "\nDefinition-sensitivity model v1: private receptor means use 1800/k^1.2, multiplied by 1.6 in the comparison group.\nComparison subjects have triples of distinct synonymous nucleotide junctions sharing one amino-acid junction,\nwith arbitrary V labels TRBV5-1*01, TRBV5-1*02 and TRBV5-2*01 (J TRBJ2-7*01). Reference subjects have distinct amino-acid junctions.\nThis deliberately creates definition-dependent grouping and unequal observed depth; it is not a fitted population model.\nAll 600 private nucleotide identities remain distinct per subject. V/J/AA truth records the exact partitions.\nRichness is measured at observed depth, without rarefaction; the audit cannot resolve depth or ascertainment bias.\n"
	}
	if err = write("README.md", readme); err != nil {
		return Summary{}, err
	}
	sort.Strings(sum.Files)
	files := []bcellFile{}
	for _, name := range sum.Files {
		raw, e := os.ReadFile(filepath.Join(stage, name))
		if e != nil {
			return Summary{}, e
		}
		files = append(files, bcellFile{name, fmt.Sprintf("%x", sha256.Sum256(raw))})
	}
	if err = writeJSON(filepath.Join(stage, "truth/truth.json"), map[string]any{"model": countSimulationModel(c), "simulator": "ClonoLoom " + version, "study": s, "files": files}); err != nil {
		return Summary{}, err
	}
	sum.Files = append(sum.Files, "truth/truth.json")
	if err = ctx.Err(); err != nil {
		return Summary{}, err
	}
	if entries, e := os.ReadDir(dir); e == nil {
		if len(entries) > 0 {
			return Summary{}, fmt.Errorf("%s is not empty", dir)
		}
		if err = os.Remove(dir); err != nil {
			return Summary{}, err
		}
	} else if !os.IsNotExist(e) {
		return Summary{}, e
	}
	if err = os.Rename(stage, dir); err != nil {
		return Summary{}, err
	}
	progress(1, "Done")
	return sum, nil
}

func countSimulationModel(c *CountStudy) string {
	if c.DefinitionVariants {
		return "clonoloom-definition-variants-v1"
	}
	return "clonoloom-replicated-counts-v1"
}

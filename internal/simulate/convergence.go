package simulate

// Controlled subject recurrence/typing evidence, not fitted human haplotypes or
// antigen recognition. Repeated samples are deliberately not independent people.
import (
	"clonoloom/internal/convergence"
	"clonoloom/internal/dmath"
	"clonoloom/internal/random"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ConvergenceStudy struct{}

func runConvergence(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0755); err != nil {
		return Summary{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-convergence-")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(stage)
	if err = os.Mkdir(filepath.Join(stage, "truth"), 0755); err != nil {
		return Summary{}, err
	}
	sum := Summary{Dir: dir, Subjects: 32}
	write := func(name, data string) error {
		if err := os.WriteFile(filepath.Join(stage, name), []byte(data), 0644); err != nil {
			return err
		}
		sum.Files = append(sum.Files, name)
		return nil
	}
	var truth strings.Builder
	truth.WriteString("sample\tsubject\tarm\tv_call\tjunction_aa\tcount\tplanted_signature\n")
	var reps []map[string]any
	subjects := []convergence.Subject{}
	arms := map[string][]string{"query": {}, "background": {}}
	codons := map[byte]string{'A': "GCT", 'C': "TGT", 'D': "GAT", 'E': "GAA", 'F': "TTT", 'G': "GGT", 'H': "CAT", 'I': "ATT", 'K': "AAA", 'L': "CTG", 'M': "ATG", 'N': "AAT", 'P': "CCT", 'Q': "CAA", 'R': "CGT", 'S': "TCT", 'T': "ACT", 'V': "GTT", 'W': "TGG", 'Y': "TAT"}
	for i := 0; i < 32; i++ {
		if err = ctx.Err(); err != nil {
			return Summary{}, err
		}
		rng := random.New(s.Seed, uint64(910000+i))
		id := fmt.Sprintf("H%02d", i+1)
		a := []string{"HLA-A*01:01", "HLA-A*24:02"}
		if i < 20 {
			a = []string{"HLA-A*02:01:01", "HLA-A*03:01"}
		}
		if i >= 30 {
			a = []string{}
		} else if i == 29 {
			a = []string{"HLA-A*01"}
		} else if i == 28 {
			a = []string{"HLA-A*01:01N", "HLA-A*24:02"}
		}
		b := []string{"HLA-B*08:01", "HLA-B*15:01"}
		if i%2 == 0 {
			b = []string{"HLA-B*07:02", "HLA-B*08:01"}
		}
		dr := []string{"HLA-DRB1*03:01", "HLA-DRB1*04:01"}
		if i%4 < 2 {
			dr = []string{"HLA-DRB1*15:01", "HLA-DRB1*04:01"}
		}
		subject := convergence.Subject{ID: id, Samples: []string{}, Typing: map[string][]string{"A": a, "B": b, "DRB1": dr}}
		arm := "query"
		if i%2 != 0 {
			arm = "background"
		}
		visits := 1
		if i < 4 {
			visits = 2
		}
		// One sequence list per subject, observed again at a different depth on revisit.
		type receptor struct{ v, aa, planted string }
		rows := []receptor{}
		if i < 17 || i == 23 {
			for k := 0; k < 1+i%3; k++ {
				core := ""
				for j := 0; j < 5; j++ {
					core += string("ASGGNRTY"[rng.IntN(8)])
				}
				rows = append(rows, receptor{"TRBV5-1*01", "CASSQGHT" + core + "EQYF", "local QGHT"})
			}
		}
		if i%3 == 0 {
			rows = append(rows, receptor{"TRBV7-9*01", "CASSG" + string("ILMV"[i%4]) + "TGQANEQYF", "global ILMV"})
		}
		for len(rows) < 30 {
			core := ""
			for j, n := 0, 7+rng.IntN(3); j < n; j++ {
				core += string("ASSSGGGTNQDRY"[rng.IntN(13)])
			}
			rows = append(rows, receptor{[]string{"TRBV5-1*01", "TRBV6-5*01", "TRBV7-9*01"}[rng.IntN(3)], "CASS" + core + "EQYF", "none"})
		}
		for visit := 0; visit < visits; visit++ {
			sample := fmt.Sprintf("%s-%d", id, visit+1)
			subject.Samples = append(subject.Samples, sample)
			arms[arm] = append(arms[arm], sample)
			sum.Samples++
			var data strings.Builder
			data.WriteString("sequence_id\tproductive\tlocus\tv_call\tj_call\tjunction\tjunction_aa\tumi_count\trepertoire_id\n")
			exposure := dmath.Exp(.5 * rng.Normal())
			for j, rec := range rows {
				var nt strings.Builder
				for k := range rec.aa {
					nt.WriteString(codons[rec.aa[k]])
				}
				mean := float64(4000/dmath.Pow(float64(j+1), 1.15)) * exposure
				count := 1 + rng.Poisson(rng.Gamma(3)*mean/3)
				fmt.Fprintf(&data, "%s-%03d\tT\tTRB\t%s\tTRBJ2-7*01\t%s\t%s\t%d\t%s\n", sample, j, rec.v, nt.String(), rec.aa, count, sample)
				fmt.Fprintf(&truth, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n", sample, id, arm, rec.v, rec.aa, count, rec.planted)
				sum.Rows++
			}
			if err = write(sample+".tsv", data.String()); err != nil {
				return Summary{}, err
			}
			meta := repertoire(s, Subject{ID: id, Condition: arm}, sample, sample+".tsv", version)
			meta["study"].(map[string]any)["study_id"] = "clonoloom-simulated-convergence-study-v1"
			meta["subject"].(map[string]any)["mhc"] = append(append(append([]string{}, a...), b...), dr...)
			reps = append(reps, meta)
		}
		subjects = append(subjects, subject)
		progress(float64(i+1)/32, "Simulating subjects and typing")
	}
	if err = write("truth/receptors.tsv", truth.String()); err != nil {
		return Summary{}, err
	}
	for _, item := range []struct {
		name  string
		value any
	}{{"repertoires.airr.json", map[string]any{"Repertoire": reps}}, {"truth/subjects.json", subjects}, {"truth/arms.json", arms}} {
		if err = writeJSON(filepath.Join(stage, item.name), item.value); err != nil {
			return Summary{}, err
		}
		sum.Files = append(sum.Files, item.name)
	}
	if err = write("README.md", fmt.Sprintf("# Artificial subject recurrence and HLA study\n\nSeed %d; 32 fictional subjects, 36 samples and 30 rows per sample. Four subjects have a second observation at another depth.\nA local QGHT signature is deliberately associated with two-field HLA-A*02:01 carriage; a conservative ILMV group provides a weaker control.\nAlleles and parseable V labels are construction choices, not fitted haplotypes, recombination or antigen recognition.\nFour subjects have missing, incomplete or expression-suffix A typing; their non-carriage must not be inferred.\nUMIs are 1 + gamma-Poisson draws (shape 3), mean scale 4000/(rank+1)^1.15 times lognormal exposure (log SD 0.5); counts never weight subject tests.\nUse the exact arms and subject typing in truth/. Select QGHT length 17 at start window 5–8 and the ILMV global signature before inspecting HLA results.\nHLA-A*02:01, HLA-B*07:02 and HLA-DRB1*15:01 form the declared allele panel. Full records and hashes are retained.\n", s.Seed)); err != nil {
		return Summary{}, err
	}
	sort.Strings(sum.Files)
	files := []bcellFile{}
	for _, f := range sum.Files {
		raw, e := os.ReadFile(filepath.Join(stage, f))
		if e != nil {
			return Summary{}, e
		}
		files = append(files, bcellFile{f, fmt.Sprintf("%x", sha256.Sum256(raw))})
	}
	if err = writeJSON(filepath.Join(stage, "truth/truth.json"), map[string]any{"model": "clonoloom-convergence-study-v1", "study": s, "files": files, "simulator": "ClonoLoom " + version}); err != nil {
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
	return sum, nil
}

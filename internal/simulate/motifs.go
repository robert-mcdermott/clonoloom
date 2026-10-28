package simulate

// A deliberately planted signature benchmark, not a fitted recombination or
// antigen-specificity model. Every sequence, group and count is recorded.
import (
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

type MotifStudy struct{}

func runMotifs(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0755); err != nil {
		return Summary{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-motifs-")
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(stage)
	if err = os.Mkdir(filepath.Join(stage, "truth"), 0755); err != nil {
		return Summary{}, err
	}
	sum := Summary{Dir: dir, Subjects: 12, Samples: 12}
	write := func(name, data string) error {
		if e := os.WriteFile(filepath.Join(stage, name), []byte(data), 0644); e != nil {
			return e
		}
		sum.Files = append(sum.Files, name)
		return nil
	}
	var truth strings.Builder
	truth.WriteString("sample\tsubject\tarm\tv_call\tjunction_aa\tcount\tplanted_signature\n")
	var reps []map[string]any
	seen := map[string]bool{}
	codons := map[byte]string{'A': "GCT", 'C': "TGT", 'D': "GAT", 'E': "GAA", 'F': "TTT", 'G': "GGT", 'H': "CAT", 'I': "ATT", 'K': "AAA", 'L': "CTG", 'M': "ATG", 'N': "AAT", 'P': "CCT", 'Q': "CAA", 'R': "CGT", 'S': "TCT", 'T': "ACT", 'V': "GTT", 'W': "TGG", 'Y': "TAT"}
	for subject := 0; subject < 12; subject++ {
		if err = ctx.Err(); err != nil {
			return Summary{}, err
		}
		rng := random.New(s.Seed, uint64(900000+subject))
		id := fmt.Sprintf("S%02d", subject+1)
		arm := "background"
		n := 125
		if subject < 4 {
			arm = "query"
			n = 30
		}
		exposure := dmath.Exp(.45 * rng.Normal())
		var data strings.Builder
		data.WriteString("sequence_id\tproductive\tlocus\tv_call\tj_call\tjunction\tjunction_aa\tumi_count\trepertoire_id\n")
		for j := 0; j < n; j++ {
			v := []string{"TRBV5-1*01", "TRBV6-5*01", "TRBV7-9*01"}[rng.IntN(3)]
			if subject < 4 {
				v = []string{"TRBV5-1*01", "TRBV6-5*01"}[rng.IntN(2)]
			}
			aa, planted := "", "none"
			if subject < 4 && j < 4 {
				aa = "CASSG" + string("ILMV"[j]) + "TGQANEQYF"
				v = "TRBV7-9*01"
				planted = "global conservative ILMV"
			} else {
				for {
					var core strings.Builder
					alphabet := "ASSSGGGTNQDRY"
					length := 7 + rng.IntN(3)
					for k := 0; k < length; k++ {
						core.WriteByte(alphabet[rng.IntN(len(alphabet))])
					}
					aa = "CASS" + core.String() + "EQYF"
					if subject < 4 && j >= 4 && j < 13 {
						at := 4 + rng.IntN(3)
						aa = aa[:at] + "QGHT" + aa[at+4:]
						v = "TRBV5-1*01"
						planted = "local QGHT"
					}
					if !seen[v+aa] {
						seen[v+aa] = true
						break
					}
				}
			}
			var nt strings.Builder
			for k := range aa {
				nt.WriteString(codons[aa[k]])
			}
			mean := float64(5000/dmath.Pow(float64(j+1), 1.2)) * exposure
			count := 1 + rng.Poisson(rng.Gamma(4)*mean/4)
			fmt.Fprintf(&data, "%s-%03d\tT\tTRB\t%s\tTRBJ2-7*01\t%s\t%s\t%d\t%s\n", id, j, v, nt.String(), aa, count, id)
			fmt.Fprintf(&truth, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n", id, id, arm, v, aa, count, planted)
			sum.Rows++
		}
		if err = write(id+".tsv", data.String()); err != nil {
			return Summary{}, err
		}
		meta := repertoire(s, Subject{ID: id, Condition: arm}, id, id+".tsv", version)
		meta["study"].(map[string]any)["study_id"] = "clonoloom-simulated-motif-study-v1"
		reps = append(reps, meta)
		progress(float64(subject+1)/12, "Simulating query and background receptors")
	}
	if err = write("truth/receptors.tsv", truth.String()); err != nil {
		return Summary{}, err
	}
	if err = writeJSON(filepath.Join(stage, "repertoires.airr.json"), map[string]any{"Repertoire": reps}); err != nil {
		return Summary{}, err
	}
	sum.Files = append(sum.Files, "repertoires.airr.json")
	if err = write("README.md", fmt.Sprintf("# Artificial sequence-signature study\n\nSeed %d; four query subjects with 30 rows each and eight background subjects with 125 rows each.\nShort variable junctions use a biased central residue alphabet, CASS/EQYF flanks and arbitrary parseable TRBV labels.\nQuery subjects share four synthetic TRBV7-9 junctions differing only by ILMV at one position; nine further rows per query subject contain QGHT at starts 5–7 under TRBV5-1.\nOther junctions are unique random controls; planted motifs may have incidental shorter neighbors.\nCounts are 1 + gamma-Poisson draws (shape 4), with mean scale 5000/(rank+1)^1.2 times a lognormal subject exposure.\nThe added one guarantees positive observed records in this example; these counts are descriptive, never test weights.\nThese are artificial labels and signatures, not fitted human recombination or evidence of shared specificity.\nExplicit query/background selections, matched V/length strata and every truth record allow independent auditing.\n", s.Seed)); err != nil {
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
	if err = writeJSON(filepath.Join(stage, "truth/truth.json"), map[string]any{"model": "clonoloom-motif-study-v1", "study": s, "simulator": "ClonoLoom " + version, "files": files}); err != nil {
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

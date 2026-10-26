package simulate

// Artificial sample QC with uneven depths, shifted batches and explicit edge controls.
import (
	"clonoloom/internal/batchqc"
	"clonoloom/internal/dmath"
	"clonoloom/internal/seq"

	"clonoloom/internal/random"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type BatchQCStudy struct{}

func runBatchQC(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if e := ctx.Err(); e != nil {
		return Summary{}, e
	}
	if e := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0755); e != nil {
		return Summary{}, e
	}
	stage, e := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-batchqc-")
	if e != nil {
		return Summary{}, e
	}
	defer os.RemoveAll(stage)
	if e = os.Mkdir(filepath.Join(stage, "truth"), 0755); e != nil {
		return Summary{}, e
	}
	sum := Summary{Dir: dir, Subjects: 25, Samples: 26}
	write := func(name, data string) error {
		if e := os.WriteFile(filepath.Join(stage, name), []byte(data), 0644); e != nil {
			return e
		}
		sum.Files = append(sum.Files, name)
		return nil
	}
	rng := random.New(s.Seed, 930001)
	locus := HumanTRB()
	gen := NewGenerator(&locus, DefaultRecombination())
	assignments := []batchqc.Assignment{}
	reps := []map[string]any{}
	var truth strings.Builder
	truth.WriteString("sample\tsubject\tbatch\tclass\tproductive\tv_call\tj_call\tcount\n")
	for i := 0; i < 26; i++ {
		if ctx.Err() != nil {
			return Summary{}, ctx.Err()
		}
		subject := fmt.Sprintf("D%02d", i+1)
		name := subject + "-1"
		batch := "run-A"
		class := "reference construction"
		p := .91 + float64(.06*rng.Float64())
		rows := 65 + int(rng.IntN(65))
		exclude := ""
		if i >= 8 {
			batch = "run-B"
			p = .65 + float64(.15*rng.Float64())
			class = "shifted productivity construction"
		}
		if i >= 16 {
			batch = "run-C"
			p = .90 + float64(.07*rng.Float64())
			class = "small batch control"
		}
		if i >= 19 {
			batch = "run-D"
			p = .92
			rows = 80
			class = "constant row-count control"
		}
		if i == 7 {
			p = .25
			class = "planted low-productivity observation"
		}
		if i == 6 {
			class = "missing-count control"
		}
		if i == 24 {
			subject = "D01"
			name = "D01-2"
			batch = "run-A"
			exclude = "Repeated observation: retain D01-1"
			class = "excluded repeat"
		}
		if i == 25 {
			subject = "D25"
			name = "D25-1"
			batch = ""
			exclude = "Batch not recorded"
			class = "excluded missing batch"
		}
		assignments = append(assignments, batchqc.Assignment{Name: name, Subject: subject, Batch: batch, Exclude: exclude})
		var table strings.Builder
		table.WriteString("sequence_id\tproductive\tlocus\tv_call\tj_call\tjunction\tjunction_aa\tumi_count\trepertoire_id\n")
		exposure := dmath.Exp(.38 * rng.Normal())
		for k := 0; k < rows; k++ {
			x := gen.Draw(rng)
			for !x.Productive {
				x = gen.Draw(rng)
			}
			nt, aa, v, j, flag := x.Junction, x.JunctionAA, x.V.Name, x.J.Name, "T"
			if rng.Float64() > p {
				flag = "F"
				nt = nt[:6] + "TAA" + nt[9:]
				aa = seq.Translate(nt)
			}
			if rng.Float64() < .02 {
				flag = ""
			}
			if rng.Float64() < .035 {
				v = ""
			}
			if rng.Float64() < .025 {
				j = ""
			}
			count := fmt.Sprint(1 + rng.Poisson(rng.Gamma(3)*1800/dmath.Pow(float64(k+1), 1.05)*exposure/3))
			if i == 6 && k == 0 {
				count = ""
			}
			fmt.Fprintf(&table, "%s-%04d\t%s\tTRB\t%s\t%s\t%s\t%s\t%s\t%s\n", name, k, flag, v, j, nt, aa, count, name)
			fmt.Fprintf(&truth, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", name, subject, batch, class, flag, v, j, count)
			sum.Rows++
		}
		if e = write(name+".tsv", table.String()); e != nil {
			return Summary{}, e
		}
		meta := repertoire(s, Subject{ID: subject, Condition: batch}, name, name+".tsv", version)
		meta["study"].(map[string]any)["study_id"] = "clonoloom-simulated-batch-qc-study-v1"
		reps = append(reps, meta)
		progress(float64(i+1)/26, "Simulating sample QC and batch controls")
	}
	if e = write("truth/observations.tsv", truth.String()); e != nil {
		return Summary{}, e
	}
	for _, x := range []struct {
		name  string
		value any
	}{{"truth/samples.json", assignments}, {"repertoires.airr.json", map[string]any{"Repertoire": reps}}} {
		if e = writeJSON(filepath.Join(stage, x.name), x.value); e != nil {
			return Summary{}, e
		}
		sum.Files = append(sum.Files, x.name)
	}
	if e = write("README.md", fmt.Sprintf("# Artificial batch-QC study\n\nSeed %d; 26 samples, 25 fictional subjects. 24 included subjects across four batches (8,8,3,5); one explicitly excluded repeat and one missing-batch exclusion.\nHuman TRB junctions use ClonoLoom's length-selected model. Nonproductive controls insert an internal stop codon; flags and missing genes/counts are deliberate construction. Counts use shifted gamma-Poisson variation with lognormal sample exposure. Run A has a planted low-productivity sample and a missing-count control; run B has a productivity shift; run C is too small for scoring; run D has constant row counts. These demonstrate descriptive QC, not calibrated sensitivity or real biological batch effects. No correction or automatic removal.\nUse truth/samples.json for explicit assignments/exclusions. All row-level generating observations and source hashes are retained.\n", s.Seed)); e != nil {
		return Summary{}, e
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
	if e = writeJSON(filepath.Join(stage, "truth/truth.json"), map[string]any{"model": "clonoloom-batch-qc-study-v1", "study": s, "files": files, "simulator": "ClonoLoom " + version}); e != nil {
		return Summary{}, e
	}
	sum.Files = append(sum.Files, "truth/truth.json")
	if ctx.Err() != nil {
		return Summary{}, ctx.Err()
	}
	if entries, e := os.ReadDir(dir); e == nil {
		if len(entries) > 0 {
			return Summary{}, fmt.Errorf("%s is not empty", dir)
		}
		if e = os.Remove(dir); e != nil {
			return Summary{}, e
		}
	} else if !os.IsNotExist(e) {
		return Summary{}, e
	}
	if e = os.Rename(stage, dir); e != nil {
		return Summary{}, e
	}
	return sum, nil
}

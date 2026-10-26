package simulate

// Artificial carryover and independently shared receptor controls. Sequence draws
// use the existing ClonoLoom recombination model, not a fitted contamination model.
import (
	"clonoloom/internal/contamination"
	"clonoloom/internal/dmath"
	"clonoloom/internal/genes"

	"clonoloom/internal/random"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ContaminationStudy struct{}

func runContamination(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if e := ctx.Err(); e != nil {
		return Summary{}, e
	}
	if e := os.MkdirAll(filepath.Dir(filepath.Clean(dir)), 0755); e != nil {
		return Summary{}, e
	}
	stage, e := os.MkdirTemp(filepath.Dir(filepath.Clean(dir)), ".clonoloom-contamination-")
	if e != nil {
		return Summary{}, e
	}
	defer os.RemoveAll(stage)
	if e = os.Mkdir(filepath.Join(stage, "truth"), 0755); e != nil {
		return Summary{}, e
	}
	sum := Summary{Dir: dir, Subjects: 8, Samples: 9}
	write := func(name, data string) error {
		if e := os.WriteFile(filepath.Join(stage, name), []byte(data), 0644); e != nil {
			return e
		}
		sum.Files = append(sum.Files, name)
		return nil
	}
	rng := random.New(s.Seed, 920001)
	locus := HumanTRB()
	gen := NewGenerator(&locus, DefaultRecombination())
	type sequence struct {
		NT, AA, V, J string
		Productive   bool
	}
	seen := map[string]bool{}
	draw := func() sequence {
		for {
			x := gen.Draw(rng)
			d := sequence{x.Junction, x.JunctionAA, x.V.Name, x.J.Name, x.Productive}
			v, j := genes.ParseCall(d.V).Key(genes.LevelGene), genes.ParseCall(d.J).Key(genes.LevelGene)
			if d.Productive && contamination.Valid(v, j, d.NT) && !seen[d.NT] {
				seen[d.NT] = true
				return d
			}
		}
	}
	// Each subject has 75 private productive sequences. Three are deliberately
	// copied from subject 1 into subject 2; three others are shared controls.
	base := make([][]sequence, 8)
	for i := range base {
		for k := 0; k < 75; k++ {
			base[i] = append(base[i], draw())
		}
	}
	public := []sequence{draw(), draw(), draw()}
	var truth strings.Builder
	truth.WriteString("sample\tsubject\tclass\tv_call\tj_call\tjunction\tcount\n")
	assignments := []contamination.Sample{}
	reps := []map[string]any{}
	for i := 0; i < 9; i++ {
		if ctx.Err() != nil {
			return Summary{}, ctx.Err()
		}
		subject := i
		if i == 8 {
			subject = 0
		}
		id := fmt.Sprintf("D%02d", subject+1)
		sample := id + "-1"
		if i == 8 {
			sample = id + "-2"
		}
		batch := "lane-A"
		if i >= 4 {
			batch = "lane-B"
		}
		assignments = append(assignments, contamination.Sample{Name: sample, Subject: id, Batch: batch})
		var table strings.Builder
		table.WriteString("sequence_id\tproductive\tlocus\tv_call\tj_call\tjunction\tjunction_aa\tumi_count\trepertoire_id\n")
		exposure := dmath.Exp(.55 * rng.Normal())
		emit := func(d sequence, count int, class string) {
			fmt.Fprintf(&table, "%s-%04d\tT\tTRB\t%s\t%s\t%s\t%s\t%d\t%s\n", sample, sum.Rows, d.V, d.J, d.NT, d.AA, count, sample)
			fmt.Fprintf(&truth, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n", sample, id, class, d.V, d.J, d.NT, count)
			sum.Rows++
		}
		for k, d := range base[subject] {
			mean := float64(2500/dmath.Pow(float64(k+1), 1.1)) * exposure
			count := 1 + rng.Poisson(rng.Gamma(3)*mean/3)
			class := "private"
			if subject == 0 && k < 3 {
				count = 3500 + k*1200
				class = "carryover source"
			}
			emit(d, count, class)
		}
		for k, d := range public {
			emit(d, 12+int(rng.IntN(80))+k*5, "independent shared control")
		}
		if subject == 1 {
			for k, d := range base[0][:3] {
				emit(d, 7+k*4, "planted carryover")
			}
		}
		// An AA-identical but synonymous control must never become an NT match.
		if subject == 2 {
			d := public[0]
			third := "T"
			if d.NT[2] == 'T' {
				third = "C"
			}
			d.NT = d.NT[:2] + third + d.NT[3:]
			emit(d, 23, "synonymous control")
		}
		if e = write(sample+".tsv", table.String()); e != nil {
			return Summary{}, e
		}
		meta := repertoire(s, Subject{ID: id, Condition: batch}, sample, sample+".tsv", version)
		meta["study"].(map[string]any)["study_id"] = "clonoloom-simulated-contamination-study-v1"
		reps = append(reps, meta)
		progress(float64(i+1)/9, "Simulating sharing and carryover controls")
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
	if e = write("README.md", fmt.Sprintf("# Artificial contamination-evidence study\n\nSeed %d; eight fictional subjects and nine samples. D01 has a repeat in another batch.\nProductive sequences are drawn from ClonoLoom's human TRB recombination model. Private repertoires, three independently shared controls, three deliberate D01-to-D02 transfers and a synonymous control accompany uneven shifted gamma-Poisson counts.\nSharing/control labels are construction truth, not Pgen labels or proof of biological publicness. Thresholds are illustrative, not calibrated sensitivity/specificity. No sequencing error, empirical index-hopping mechanism or population selection model is fitted.\nUse truth/samples.json for assignments. Optionally compute a human NT TRB V/J-gene productive-only Pgen result under the same QC before screening; the chosen model probabilities, not planted labels, determine high-Pgen context.\nCounts use 1+Poisson(Gamma(shape=3)*[2500/(rank+1)^1.1*exposure]/3), exposure log-SD .55; planted counts are explicit in truth.\n", s.Seed)); e != nil {
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
	if e = writeJSON(filepath.Join(stage, "truth/truth.json"), map[string]any{"model": "clonoloom-contamination-study-v1", "study": s, "files": files, "simulator": "ClonoLoom " + version}); e != nil {
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

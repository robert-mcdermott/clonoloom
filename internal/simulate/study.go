package simulate

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"clonoloom/internal/dmath"
	"clonoloom/internal/random"
)

// Population is how a subject's repertoire is made: Clonotypes productive rearrangements (and
// nonproductive ones, Nonproductive of all), each a clone of a size drawn from a power law
// (P(size ≥ s) = s^-Alpha, at most MaxSize cells), and a few expanded clones holding the shares
// of all cells Expanded gives.
type Population struct {
	Clonotypes    int       `json:"clonotypes"`
	Alpha         float64   `json:"alpha"`
	MaxSize       int       `json:"maxSize"`
	Expanded      []float64 `json:"expanded,omitempty"`
	Nonproductive float64   `json:"nonproductive"`
}

// Group is subjects of one condition.
type Group struct {
	Condition  string     `json:"condition"`
	Subjects   int        `json:"subjects"`
	Population Population `json:"population"`
}

// Study is a simulated study's parameters: its groups of subjects; SamplesPerSubject samples of
// each (replicate draws from the same blood); each sample's templates (Templates, taken in turn,
// so depth varies between samples); and each template's reads, a negative binomial of mean
// ReadsPerTemplate and size ReadsSize (a template with none goes unseen).
type Study struct {
	Name              string        `json:"name"`
	Title             string        `json:"title"`
	Seed              uint64        `json:"seed"`
	Groups            []Group       `json:"groups"`
	SamplesPerSubject int           `json:"samplesPerSubject"`
	Templates         []int         `json:"templates"`
	ReadsPerTemplate  float64       `json:"readsPerTemplate"`
	ReadsSize         float64       `json:"readsSize"`
	Recombination     Recombination `json:"recombination"`
	// Series: a study over time (series.go), SamplesPerSubject then unused.
	Series *Series `json:"series,omitempty"`
	// BCells selects the artificial paired B-cell model; the TRB parameters above are unused.
	BCells        *BCellStudy         `json:"bcells,omitempty"`
	PseudobulkDE  *PseudobulkDEStudy  `json:"pseudobulkDE,omitempty"`
	CellStates    *CellStateStudy     `json:"cellStates,omitempty"`
	BatchQC       *BatchQCStudy       `json:"batchQC,omitempty"`
	Contamination *ContaminationStudy `json:"contamination,omitempty"`
	Convergence   *ConvergenceStudy   `json:"convergence,omitempty"`
	Motifs        *MotifStudy         `json:"motifs,omitempty"`
	Counts        *CountStudy         `json:"counts,omitempty"`
}

// Presets are the studies `clonoloom simulate` starts from.
var Presets = map[string]Study{
	"pseudobulk-de-study": {Name: "pseudobulk-de-study", Title: "An artificial paired pseudobulk expression study", Seed: 20261103, PseudobulkDE: &PseudobulkDEStudy{Subjects: 28}},
	"pseudobulk-de-null":  {Name: "pseudobulk-de-null", Title: "An artificial null paired pseudobulk study", Seed: 20261104, PseudobulkDE: &PseudobulkDEStudy{Subjects: 28, Null: true}},
	"batch-qc-study":      {Name: "batch-qc-study", Title: "An artificial batch-QC study", Seed: 20261101, BatchQC: &BatchQCStudy{}},
	"contamination-study": {Name: "contamination-study", Title: "An artificial contamination-evidence study", Seed: 20261031, Contamination: &ContaminationStudy{}},
	"convergence-study":   {Name: "convergence-study", Title: "An artificial subject recurrence and HLA study", Seed: 20261030, Convergence: &ConvergenceStudy{}},
	"motif-study":         {Name: "motif-study", Title: "An artificial sequence-signature study", Seed: 20261029, Motifs: &MotifStudy{}},
	"definition-study":    {Name: "definition-study", Title: "A definition-sensitive repertoire study", Seed: 20261028, Counts: &CountStudy{Subjects: 32, Layout: "independent", DefinitionVariants: true}},
	"counts-paired":       {Name: "counts-paired", Title: "A replicated paired repertoire study", Seed: 20261026, Counts: &CountStudy{Subjects: 28, Layout: "paired"}},
	"counts-independent":  {Name: "counts-independent", Title: "A replicated independent repertoire study", Seed: 20261026, Counts: &CountStudy{Subjects: 32, Layout: "independent"}},
	"counts-null":         {Name: "counts-null", Title: "A replicated null repertoire study", Seed: 20261027, Counts: &CountStudy{Subjects: 32, Layout: "independent", Null: true}},
	"cell-states":         {Name: "cell-states", Title: "An artificial paired T-cell and expression study with known states", Seed: 20261028, CellStates: &CellStateStudy{Subjects: 2, CellsPerSample: 120, Clones: 12}},
	"pseudobulk-study":    {Name: "pseudobulk-study", Title: "Artificial pseudobulk count evidence across subjects and timepoints", Seed: 20261102, CellStates: &CellStateStudy{Subjects: 4, CellsPerSample: 420, Clones: 48, Layout: "gaussian"}},
	"cell-states-clouds":  {Name: "cell-states-clouds", Title: "An artificial paired T-cell and expression study with supplied Gaussian clouds", Seed: 20261028, CellStates: &CellStateStudy{Subjects: 2, CellsPerSample: 360, Clones: 10, Layout: "gaussian"}},
	"bcell-immunization": {
		Name: "bcell-immunization", Title: "An artificial B-cell immunization study with known families and ancestors",
		Seed:   20261027,
		BCells: &BCellStudy{Subjects: 3, Families: 12, CellsPerSample: 240, MutationsPerBranch: 3},
	},
	"standard": {
		Name: "standard", Title: "A simulated TRB study: three control subjects and three with expanded clones, two samples each",
		Seed: 20261021,
		Groups: []Group{
			{Condition: "control", Subjects: 3, Population: Population{Clonotypes: 60000, Alpha: 1.3, MaxSize: 200000, Nonproductive: 0.15}},
			{Condition: "expanded", Subjects: 3, Population: Population{Clonotypes: 30000, Alpha: 1.1, MaxSize: 200000, Expanded: []float64{0.08, 0.05, 0.04, 0.03, 0.02}, Nonproductive: 0.15}},
		},
		SamplesPerSubject: 2,
		Templates:         []int{12000, 30000, 20000, 45000, 15000, 25000},
		ReadsPerTemplate:  8, ReadsSize: 2,
		Recombination: DefaultRecombination(),
	},
	"null": {
		Name: "null", Title: "A simulated TRB study with no difference between its groups: six subjects of one kind in two groups, two samples each",
		Seed: 20261022,
		Groups: []Group{
			{Condition: "control", Subjects: 3, Population: Population{Clonotypes: 60000, Alpha: 1.3, MaxSize: 200000, Nonproductive: 0.15}},
			{Condition: "treated", Subjects: 3, Population: Population{Clonotypes: 60000, Alpha: 1.3, MaxSize: 200000, Nonproductive: 0.15}},
		},
		SamplesPerSubject: 2,
		Templates:         []int{12000, 30000, 20000, 45000, 15000, 25000},
		ReadsPerTemplate:  8, ReadsSize: 2,
		Recombination: DefaultRecombination(),
	},
	"small": {
		Name: "small", Title: "A small simulated TRB study: two subjects, one sample each",
		Seed: 7,
		Groups: []Group{
			{Condition: "control", Subjects: 1, Population: Population{Clonotypes: 3000, Alpha: 1.3, MaxSize: 5000, Nonproductive: 0.15}},
			{Condition: "expanded", Subjects: 1, Population: Population{Clonotypes: 1500, Alpha: 1.1, MaxSize: 5000, Expanded: []float64{0.1}, Nonproductive: 0.15}},
		},
		SamplesPerSubject: 1,
		Templates:         []int{2000, 3000},
		ReadsPerTemplate:  8, ReadsSize: 2,
		Recombination: DefaultRecombination(),
	},
	// Studies over time (series.go).
	"car-t": {
		Name: "car-t", Title: "A simulated CAR-T study: three patients' blood before and after infusion, with their infusion products",
		Seed: 20261023,
		Groups: []Group{
			{Condition: "responder", Subjects: 2, Population: Population{Clonotypes: 30000, Alpha: 1.2, MaxSize: 100000, Nonproductive: 0.15}},
			{Condition: "non-responder", Subjects: 1, Population: Population{Clonotypes: 30000, Alpha: 1.2, MaxSize: 100000, Nonproductive: 0.15}},
		},
		SamplesPerSubject: 1,
		Templates:         []int{12000, 18000, 15000, 20000, 10000, 16000},
		ReadsPerTemplate:  8, ReadsSize: 2,
		Recombination: DefaultRecombination(),
		Series: &Series{
			Event:    "infusion",
			Times:    []TimePoint{{Day: -7, Replicates: 2}, {Day: 7, Replicates: 1}, {Day: 14, Replicates: 1}, {Day: 28, Replicates: 1}, {Day: 90, Replicates: 1}},
			Variance: 0.1,
			Reference: &Reference{Kind: "product", Label: "infusion product", Cells: 30000, Expansion: 1, Templates: 15000,
				Shares: map[string][]float64{"responder": {0, 0.10, 0.25, 0.10, 0.03}, "non-responder": {0, 0.02, 0.03, 0.01, 0.002}},
				Trajectories: []Trajectory{
					{Name: "expanding", Share: 0.02, Folds: []float64{1, 5, 20, 10, 5}},
					{Name: "contracting", Share: 0.2, Folds: []float64{1, 1, 0.3, 0.1, 0.02}},
				}},
		},
	},
	"transplant": {
		Name: "transplant", Title: "A simulated transplant study: three recipients' blood before and after an allogeneic transplant, with their donors",
		Seed: 20261024,
		Groups: []Group{
			{Condition: "no GVHD", Subjects: 2, Population: Population{Clonotypes: 30000, Alpha: 1.2, MaxSize: 100000, Nonproductive: 0.15}},
			{Condition: "GVHD", Subjects: 1, Population: Population{Clonotypes: 30000, Alpha: 1.2, MaxSize: 100000, Nonproductive: 0.15}},
		},
		SamplesPerSubject: 1,
		Templates:         []int{12000, 18000, 15000, 20000, 10000, 16000},
		ReadsPerTemplate:  8, ReadsSize: 2,
		Recombination: DefaultRecombination(),
		Series: &Series{
			Event:    "transplant",
			Times:    []TimePoint{{Day: -14, Replicates: 1}, {Day: 30, Replicates: 1}, {Day: 100, Replicates: 1}, {Day: 365, Replicates: 2}},
			Variance: 0.1,
			Reference: &Reference{Kind: "donor", Label: "donor blood", Population: &Population{Clonotypes: 40000, Alpha: 1.3, MaxSize: 100000, Nonproductive: 0.15}, Templates: 15000,
				Shares: map[string][]float64{"no GVHD": {0, 0.6, 0.85, 0.97}, "GVHD": {0, 0.6, 0.85, 0.97}},
				Trajectories: []Trajectory{
					{Name: "alloreactive", Share: 0.002, Folds: []float64{1, 10, 40, 5}, Conditions: []string{"GVHD"}},
				}},
		},
	},
	"vaccine": {
		Name: "vaccine", Title: "A simulated vaccination study: three vaccinated subjects and two given a placebo, before and after, with receptors that recognize three of the vaccine's epitopes",
		Seed: 20261026,
		Groups: []Group{
			{Condition: "vaccine", Subjects: 3, Population: Population{Clonotypes: 30000, Alpha: 1.2, MaxSize: 100000, Nonproductive: 0.15}},
			{Condition: "placebo", Subjects: 2, Population: Population{Clonotypes: 30000, Alpha: 1.2, MaxSize: 100000, Nonproductive: 0.15}},
		},
		SamplesPerSubject: 1,
		Templates:         []int{15000, 20000, 18000, 25000, 16000, 22000},
		ReadsPerTemplate:  8, ReadsSize: 2,
		Recombination: DefaultRecombination(),
		Series: &Series{
			Event:    "vaccination",
			Times:    []TimePoint{{Day: -7, Replicates: 2}, {Day: 7, Replicates: 1}, {Day: 14, Replicates: 1}, {Day: 28, Replicates: 1}, {Day: 90, Replicates: 1}},
			Variance: 0.1,
			Response: &Response{
				Epitopes: []Epitope{
					{Peptide: "KLDTEVYNA", Antigen: "simulated vaccine antigen", Weight: 0.5, Founders: 20},
					{Peptide: "RPQGFSWLV", Antigen: "simulated vaccine antigen", Weight: 0.3, Founders: 15},
					{Peptide: "YTNEAMPLC", Antigen: "simulated vaccine antigen", Weight: 0.2, Founders: 10},
					// In the database only: no subject responds to them.
					{Peptide: "GHSKLWDTF", Antigen: "simulated unrelated antigen", Founders: 10},
					{Peptide: "VNRQEPAIL", Antigen: "simulated unrelated antigen", Founders: 10},
				},
				Family:   Family{Members: 4, Substitutions: 1, Similarity: 0.5, Private: 2, Alpha: 1.3, MaxSize: 1000},
				Shares:   map[string][]float64{"vaccine": {0.001, 0.01, 0.03, 0.01, 0.003}, "placebo": {0.001, 0.001, 0.001, 0.001, 0.001}},
				Database: 3,
			},
		},
	},
	"null-series": {
		Name: "null-series", Title: "A simulated series with no change: four subjects sampled three times, two samples each time",
		Seed: 20261025,
		Groups: []Group{
			{Condition: "healthy", Subjects: 4, Population: Population{Clonotypes: 40000, Alpha: 1.3, MaxSize: 100000, Nonproductive: 0.15}},
		},
		SamplesPerSubject: 1,
		Templates:         []int{12000, 18000, 15000, 20000, 10000, 16000},
		ReadsPerTemplate:  8, ReadsSize: 2,
		Recombination: DefaultRecombination(),
		Series: &Series{
			Event:    "the first sample",
			Times:    []TimePoint{{Day: 0, Replicates: 2}, {Day: 14, Replicates: 2}, {Day: 28, Replicates: 2}},
			Variance: 0.1,
		},
	},
}

// Check says what is wrong with a study's parameters.
func (s Study) Check() error {
	if s.PseudobulkDE != nil {
		return s.PseudobulkDE.check(s)
	}
	if s.BatchQC != nil {
		if s.Contamination != nil || s.Convergence != nil || s.Motifs != nil || s.Counts != nil || s.CellStates != nil || s.BCells != nil || s.Series != nil || len(s.Groups) > 0 {
			return fmt.Errorf("batch QC study replaces other models")
		}
		return nil
	}

	if s.Contamination != nil {
		if s.Convergence != nil || s.Motifs != nil || s.Counts != nil || s.CellStates != nil || s.BCells != nil || s.Series != nil || len(s.Groups) > 0 {
			return fmt.Errorf("contamination study replaces other models")
		}
		return nil
	}
	if s.Convergence != nil {
		if s.Motifs != nil || s.Counts != nil || s.CellStates != nil || s.BCells != nil || s.Series != nil || len(s.Groups) > 0 {
			return fmt.Errorf("convergence study replaces other simulation models")
		}
		return nil
	}
	if s.Motifs != nil {
		if s.Counts != nil || s.CellStates != nil || s.BCells != nil || s.Series != nil || len(s.Groups) > 0 {
			return fmt.Errorf("motif study replaces other simulation models")
		}
		return nil
	}
	if s.Counts != nil {
		return s.Counts.check(s)
	}
	if s.CellStates != nil {
		return s.CellStates.check(s)
	}
	if s.BCells != nil {
		return s.BCells.check(s)
	}
	if len(s.Groups) == 0 || s.SamplesPerSubject < 1 || len(s.Templates) == 0 {
		return fmt.Errorf("a study needs groups, samples and templates")
	}
	for _, g := range s.Groups {
		p := g.Population
		if g.Subjects < 1 || p.Clonotypes <= len(p.Expanded) || p.Alpha <= 0 || p.MaxSize < 1 || p.Nonproductive < 0 || p.Nonproductive >= 1 {
			return fmt.Errorf("group %q: subjects must be at least 1, clonotypes more than the expanded clones, alpha positive and nonproductive in [0, 1)", g.Condition)
		}
		total := 0.0
		for _, e := range p.Expanded {
			total += e
		}
		if total >= 1 {
			return fmt.Errorf("group %q: the expanded clones' shares must sum to less than 1", g.Condition)
		}
	}
	for _, t := range s.Templates {
		if t < 1 {
			return fmt.Errorf("templates must be at least 1")
		}
	}
	if s.ReadsPerTemplate <= 0 || s.ReadsSize <= 0 {
		return fmt.Errorf("readsPerTemplate and readsSize must be positive")
	}
	if s.Series != nil {
		return s.Series.check(s)
	}
	return nil
}

// Clone is a clone of a subject's population: a rearrangement and its cells.
type Clone struct {
	ID string
	Rearrangement
	Size int
}

// Subject is a simulated subject and its population.
type Subject struct {
	ID, Condition string
	Clones        []Clone
}

// Progress receives the fraction done and what is being done.
type Progress func(fraction float64, message string)

// Summary is what a run made.
type Summary struct {
	Dir      string   `json:"dir"`
	Subjects int      `json:"subjects"`
	Samples  int      `json:"samples"`
	Rows     int      `json:"rows"`
	Files    []string `json:"files"`
}

// population makes a subject's population: productive and nonproductive rearrangements as the
// generator draws them until each pool is full, the same nucleotide rearrangement drawn twice
// being one clone; each a clone of a power-law size, the first productive ones expanded.
func population(g *Generator, group Group, id string, r *random.Random) Subject {
	p := group.Population
	wantP := p.Clonotypes
	wantN := int(math.Round(float64(p.Clonotypes) * p.Nonproductive / (1 - p.Nonproductive)))
	type key struct{ v, j, junction string }
	seen := map[key]int{}
	var productive, nonproductive []Clone
	for len(productive) < wantP || len(nonproductive) < wantN {
		x := g.Draw(r)
		pool := &nonproductive
		want := wantN
		if x.Productive {
			pool, want = &productive, wantP
		}
		if len(*pool) >= want {
			continue
		}
		k := key{x.V.Name, x.J.Name, x.Junction}
		size := powerLaw(r, p.Alpha, p.MaxSize)
		if at, ok := seen[k]; ok {
			// Convergent: the same rearrangement again is more cells of the same clone.
			if at >= 0 {
				productive[at].Size += size
			} else {
				nonproductive[-at-1].Size += size
			}
			continue
		}
		if x.Productive {
			seen[k] = len(productive)
		} else {
			seen[k] = -len(nonproductive) - 1
		}
		*pool = append(*pool, Clone{Rearrangement: x, Size: size})
	}
	// Expanded clones: shares of all the subject's cells.
	if len(p.Expanded) > 0 {
		rest, shares := 0, 0.0
		for i, c := range productive {
			if i >= len(p.Expanded) {
				rest += c.Size
			}
		}
		for _, c := range nonproductive {
			rest += c.Size
		}
		for _, e := range p.Expanded {
			shares += e
		}
		for i, e := range p.Expanded {
			productive[i].Size = int(math.Round(e / (1 - shares) * float64(rest)))
		}
	}
	clones := append(productive, nonproductive...)
	for i := range clones {
		clones[i].ID = fmt.Sprintf("%s-P%06d", id, i+1)
	}
	return Subject{ID: id, Condition: group.Condition, Clones: clones}
}

// SampleTruth is a sample as drawn.
type SampleTruth struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	// Templates drawn, those seen (a template with no reads is not), and reads.
	Templates           int `json:"templates"`
	TemplatesSeen       int `json:"templatesSeen"`
	ProductiveTemplates int `json:"productiveTemplates"`
	Reads               int `json:"reads"`
	Rows                int `json:"rows"`
	// Clonotypes: the productive clonotypes seen (amino-acid junction, V gene and J gene).
	Clonotypes int `json:"clonotypes"`
	// ExpectedClonotypes and ExpectedCoverage: the clonotypes a sample of ProductiveTemplates
	// productive templates from the population shows on average, Σ 1-(1-p)^m, and the share of
	// the population's cells they hold, 1 - Σ p(1-p)^m.
	ExpectedClonotypes float64 `json:"expectedClonotypes"`
	ExpectedCoverage   float64 `json:"expectedCoverage"`
	// In a study over time: the sample's day and replicate; or whether it is the subject's
	// reference sample; the reference population's expected share of its cells, and the expected
	// share of its productive cells in clonotypes the reference population holds (what attribution
	// measures).
	Day            *float64 `json:"day,omitempty"`
	Replicate      int      `json:"replicate,omitempty"`
	Reference      bool     `json:"reference,omitempty"`
	ReferenceShare *float64 `json:"referenceShare,omitempty"`
	Attribution    *float64 `json:"attribution,omitempty"`
}

// sampled draws a sample's templates from the population (each in proportion to its clone's
// cells) and each template's reads: the clones seen, with their templates and reads.
func sampled(subject Subject, s Study, templates int, r *random.Random) (seen map[int][2]int, truth SampleTruth) {
	weights := make([]float64, len(subject.Clones))
	for i, c := range subject.Clones {
		weights[i] = float64(c.Size)
	}
	cumulative := random.Cumulate(weights)
	seen = map[int][2]int{}
	truth.Templates = templates
	for range templates {
		i := r.Discrete(cumulative)
		reads := r.NegBinomial(s.ReadsPerTemplate, s.ReadsSize)
		if reads == 0 {
			continue
		}
		t := seen[i]
		seen[i] = [2]int{t[0] + 1, t[1] + reads}
		truth.TemplatesSeen++
		truth.Reads += reads
		if subject.Clones[i].Productive {
			truth.ProductiveTemplates++
		}
	}
	truth.Rows = len(seen)
	return seen, truth
}

// clonotypeKey is a productive clonotype by ClonoLoom's default definition for bulk data.
func clonotypeKey(c *Clone) string {
	return c.V.Name + "|" + c.J.Name + "|" + c.JunctionAA
}

// Run makes a study into dir (which must not exist or be empty): each sample's rearrangements as
// an AIRR table, their Repertoire metadata, and the truth in dir/truth.
func Run(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	if progress == nil {
		progress = func(float64, string) {}
	}
	if err := s.Check(); err != nil {
		return Summary{}, err
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return Summary{}, fmt.Errorf("%s is not empty", dir)
	}
	if s.PseudobulkDE != nil {
		return runPseudobulkDE(ctx, s, dir, version, progress)
	}
	if s.BatchQC != nil {
		return runBatchQC(ctx, s, dir, version, progress)
	}
	if s.Contamination != nil {
		return runContamination(ctx, s, dir, version, progress)
	}
	if s.Convergence != nil {
		return runConvergence(ctx, s, dir, version, progress)
	}
	if s.Motifs != nil {
		return runMotifs(ctx, s, dir, version, progress)
	}
	if s.Counts != nil {
		return runCounts(ctx, s, dir, version, progress)
	}
	if s.BCells != nil {
		return runBCells(ctx, s, dir, version, progress)
	}
	if s.CellStates != nil {
		return runCellStates(ctx, s, dir, version, progress)
	}
	if err := os.MkdirAll(filepath.Join(dir, "truth"), 0o755); err != nil {
		return Summary{}, err
	}
	if s.Series != nil {
		return runSeries(ctx, s, dir, version, progress)
	}
	g := NewGenerator(&humanTRB, s.Recombination)
	var subjects []Subject
	n := 0
	for _, group := range s.Groups {
		n += group.Subjects
	}
	steps := float64(2 * n)
	for _, group := range s.Groups {
		for range group.Subjects {
			if err := ctx.Err(); err != nil {
				return Summary{}, err
			}
			id := fmt.Sprintf("S%02d", len(subjects)+1)
			progress(float64(len(subjects))/steps, "Recombining "+id+"'s repertoire")
			subjects = append(subjects, population(g, group, id, random.New(s.Seed, uint64(1000+len(subjects)))))
		}
	}
	summary := Summary{Dir: dir, Subjects: len(subjects)}
	truth := studyTruth{Simulator: "ClonoLoom simulate " + version, Study: s, Germline: humanTRB.Source}
	var repertoires []map[string]any
	k := 0
	for si, subject := range subjects {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		progress(float64(n+si)/steps, "Sampling "+subject.ID)
		pop := populationTruth(subject)
		truth.Subjects = append(truth.Subjects, pop.subject)
		if err := writePopulation(filepath.Join(dir, "truth", subject.ID+".tsv.gz"), subject); err != nil {
			return Summary{}, err
		}
		summary.Files = append(summary.Files, filepath.Join("truth", subject.ID+".tsv.gz"))
		for replicate := 1; replicate <= s.SamplesPerSubject; replicate++ {
			id := fmt.Sprintf("%s-%d", subject.ID, replicate)
			templates := s.Templates[k%len(s.Templates)]
			seen, st := sampled(subject, s, templates, random.New(s.Seed, uint64(2000+k)))
			k++
			st.ID, st.Subject = id, subject.ID
			clonotypes := map[string]bool{}
			for i := range seen {
				if c := &subject.Clones[i]; c.Productive {
					clonotypes[clonotypeKey(c)] = true
				}
			}
			st.Clonotypes = len(clonotypes)
			st.ExpectedClonotypes, st.ExpectedCoverage = expectedAt(pop.frequencies, st.ProductiveTemplates)
			truth.Samples = append(truth.Samples, st)
			name := id + ".tsv"
			if err := writeSample(filepath.Join(dir, name), id, subject, seen); err != nil {
				return Summary{}, err
			}
			summary.Files = append(summary.Files, name)
			summary.Rows += st.Rows
			summary.Samples++
			repertoires = append(repertoires, repertoire(s, subject, id, name, version))
		}
	}
	truth.Sharing = sharing(subjects)
	if err := writeJSON(filepath.Join(dir, "repertoires.airr.json"), map[string]any{
		"Info":       map[string]any{"title": "AIRR Data File", "description": s.Title + " (simulated by ClonoLoom " + version + ")"},
		"Repertoire": repertoires,
	}); err != nil {
		return Summary{}, err
	}
	if err := writeJSON(filepath.Join(dir, "truth", "truth.json"), truth); err != nil {
		return Summary{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme(s, summary, version)), 0o644); err != nil {
		return Summary{}, err
	}
	summary.Files = append(summary.Files, "repertoires.airr.json", filepath.Join("truth", "truth.json"), "README.md")
	sort.Strings(summary.Files)
	progress(1, "Done")
	return summary, nil
}

// populationTruth: a subject's population as the analyses would measure it, were every cell
// sequenced, by the default definition (productive clonotypes by amino-acid junction, V gene
// and J gene).
type popTruth struct {
	subject     SubjectTruth
	frequencies []float64
}

// SubjectTruth is a subject's population's numbers.
type SubjectTruth struct {
	ID        string `json:"id"`
	Condition string `json:"condition"`
	// Clones: the productive clones (distinct nucleotide rearrangements), and the nonproductive.
	ProductiveClones    int `json:"productiveClones"`
	NonproductiveClones int `json:"nonproductiveClones"`
	// Cells: of productive clones.
	Cells int `json:"cells"`
	// The productive clonotypes and their diversity: Hill numbers of order 0, 1 and 2 (richness,
	// the exponential of Shannon's entropy, the inverse of Simpson's index), Shannon's entropy
	// (natural log), Simpson's index Σp², Pielou's evenness H/ln S, clonality 1 - evenness, the
	// Gini coefficient of the clonotypes' cells, and the largest ten's share.
	Clonotypes int     `json:"clonotypes"`
	Hill0      float64 `json:"hill0"`
	Hill1      float64 `json:"hill1"`
	Hill2      float64 `json:"hill2"`
	Shannon    float64 `json:"shannon"`
	Simpson    float64 `json:"simpson"`
	Evenness   float64 `json:"evenness"`
	Clonality  float64 `json:"clonality"`
	Gini       float64 `json:"gini"`
	Top10Share float64 `json:"top10Share"`
	// Homeostasis: the share of cells in clonotypes by frequency: rare (≤ 10⁻⁵), small (≤ 10⁻⁴),
	// medium (≤ 10⁻³), large (≤ 10⁻²) and hyperexpanded (> 10⁻²), as immunarch bins them.
	Homeostasis map[string]float64 `json:"homeostasis"`
}

func populationTruth(subject Subject) popTruth {
	t := SubjectTruth{ID: subject.ID, Condition: subject.Condition}
	cells := map[string]int{}
	for i := range subject.Clones {
		c := &subject.Clones[i]
		if !c.Productive {
			t.NonproductiveClones++
			continue
		}
		t.ProductiveClones++
		t.Cells += c.Size
		cells[clonotypeKey(c)] += c.Size
	}
	keys := make([]string, 0, len(cells))
	for key := range cells {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sizes := make([]float64, len(keys))
	freq := make([]float64, len(keys))
	for i, key := range keys {
		sizes[i] = float64(cells[key])
		freq[i] = sizes[i] / float64(t.Cells)
	}
	t.Clonotypes = len(keys)
	t.Hill0 = float64(len(keys))
	for _, p := range freq {
		t.Shannon -= float64(p * dmath.Log(p))
		t.Simpson += float64(p * p)
	}
	t.Hill1, t.Hill2 = dmath.Exp(t.Shannon), 1/t.Simpson
	if len(keys) > 1 {
		t.Evenness = t.Shannon / dmath.Log(float64(len(keys)))
		t.Clonality = 1 - t.Evenness
	}
	sorted := append([]float64(nil), sizes...)
	sort.Float64s(sorted)
	var weighted, total float64
	for i, x := range sorted {
		weighted += float64(float64(2*(i+1)-len(sorted)-1) * x)
		total += x
	}
	t.Gini = weighted / (float64(len(sorted)) * total)
	for i := len(sorted) - 1; i >= 0 && i >= len(sorted)-10; i-- {
		t.Top10Share += sorted[i] / total
	}
	t.Homeostasis = map[string]float64{"rare": 0, "small": 0, "medium": 0, "large": 0, "hyperexpanded": 0}
	for _, p := range freq {
		switch {
		case p <= 1e-5:
			t.Homeostasis["rare"] += p
		case p <= 1e-4:
			t.Homeostasis["small"] += p
		case p <= 1e-3:
			t.Homeostasis["medium"] += p
		case p <= 1e-2:
			t.Homeostasis["large"] += p
		default:
			t.Homeostasis["hyperexpanded"] += p
		}
	}
	return popTruth{subject: t, frequencies: freq}
}

// expectedAt answers the clonotypes and the coverage a sample of m from these frequencies has on
// average.
func expectedAt(freq []float64, m int) (clonotypes, coverage float64) {
	coverage = 1
	for _, p := range freq {
		miss := dmath.Exp(float64(m) * dmath.Log1p(-p)) // (1 − p)^m, without 1 − p's rounding
		clonotypes += 1 - miss
		coverage -= float64(p * miss)
	}
	return clonotypes, coverage
}

// SharingTruth: how many productive clonotypes two subjects' populations share (public
// clonotypes, from the same recombination making the same chain), and their Jaccard index.
type SharingTruth struct {
	A       string  `json:"a"`
	B       string  `json:"b"`
	Shared  int     `json:"shared"`
	Jaccard float64 `json:"jaccard"`
}

func sharing(subjects []Subject) []SharingTruth {
	sets := make([]map[string]bool, len(subjects))
	for i, s := range subjects {
		sets[i] = map[string]bool{}
		for j := range s.Clones {
			if c := &s.Clones[j]; c.Productive {
				sets[i][clonotypeKey(c)] = true
			}
		}
	}
	var out []SharingTruth
	for a := range subjects {
		for b := a + 1; b < len(subjects); b++ {
			shared := 0
			for key := range sets[a] {
				if sets[b][key] {
					shared++
				}
			}
			out = append(out, SharingTruth{A: subjects[a].ID, B: subjects[b].ID, Shared: shared, Jaccard: float64(shared) / float64(len(sets[a])+len(sets[b])-shared)})
		}
	}
	return out
}

type studyTruth struct {
	Simulator string         `json:"simulator"`
	Germline  string         `json:"germline"`
	Study     Study          `json:"study"`
	Subjects  []SubjectTruth `json:"subjects"`
	Samples   []SampleTruth  `json:"samples"`
	Sharing   []SharingTruth `json:"sharing"`
	Series    []SeriesTruth  `json:"series,omitempty"`
	// Specificity: a study with a response's founders, receptors that recognize an epitope, and
	// specificity database.
	Specificity *SpecificityTruth `json:"specificity,omitempty"`
}

var sampleColumns = []string{"sequence_id", "sequence", "rev_comp", "productive", "vj_in_frame", "stop_codon", "locus", "v_call", "d_call", "j_call",
	"sequence_alignment", "germline_alignment", "junction", "junction_aa", "v_cigar", "d_cigar", "j_cigar", "junction_length", "duplicate_count", "umi_count", "repertoire_id"}

func flag(b bool) string {
	if b {
		return "T"
	}
	return "F"
}

// writeSample writes a sample's rearrangements as an AIRR table: each clone seen, most templates
// first. Its sequence_id is the sample and the clone (truth/<subject>.tsv.gz describes it);
// umi_count, its templates, and duplicate_count, its reads; the gene calls are those recombined,
// at gene level (the segments are GRCh38's, not IMGT's alleles).
func writeSample(path, id string, subject Subject, seen map[int][2]int) error {
	order := make([]int, 0, len(seen))
	for i := range seen {
		order = append(order, i)
	}
	sort.Slice(order, func(a, b int) bool {
		if seen[order[a]][0] != seen[order[b]][0] {
			return seen[order[a]][0] > seen[order[b]][0]
		}
		return order[a] < order[b]
	})
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	w.WriteString(strings.Join(sampleColumns, "\t") + "\n")
	for _, i := range order {
		c := &subject.Clones[i]
		counts := seen[i]
		fields := []string{id + "-" + strings.TrimPrefix(c.ID, subject.ID+"-"), c.Sequence, "F", flag(c.Productive), flag(c.InFrame), flag(c.Stop), "TRB", c.V.Name, c.D.Name, c.J.Name,
			"", "", c.Junction, c.JunctionAA, "", "", "", fmt.Sprint(len(c.Junction)), fmt.Sprint(counts[1]), fmt.Sprint(counts[0]), id}
		w.WriteString(strings.Join(fields, "\t") + "\n")
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// writePopulation writes a subject's population: every clone, its rearrangement and its cells.
func writePopulation(path string, subject Subject) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	// No time in the gzip header: the same study is the same bytes.
	z := gzip.NewWriter(f)
	w := bufio.NewWriter(z)
	w.WriteString("clone_id\tproductive\tv_call\td_call\tj_call\tv_deletions\td5_deletions\td3_deletions\tj_deletions\tn1\tn2\tjunction\tjunction_aa\tcells\n")
	for i := range subject.Clones {
		c := &subject.Clones[i]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%d\n", c.ID, flag(c.Productive), c.V.Name, c.D.Name, c.J.Name, c.DelV, c.DelD5, c.DelD3, c.DelJ, c.N1, c.N2, c.Junction, c.JunctionAA, c.Size)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := z.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// repertoire is a sample's AIRR Repertoire metadata (schema 1.4 and later).
func repertoire(s Study, subject Subject, id, file, version string) map[string]any {
	return map[string]any{
		"repertoire_id": id,
		"study":         map[string]any{"study_id": "clonoloom-simulated-" + s.Name, "study_title": s.Title},
		"subject": map[string]any{
			"subject_id": subject.ID,
			"species":    map[string]any{"id": "NCBITAXON:9606", "label": "Homo sapiens"},
			"diagnosis":  []any{map[string]any{"study_group_description": subject.Condition}},
		},
		"sample": []any{map[string]any{
			"sample_id":      id,
			"tissue":         map[string]any{"id": "UBERON:0000178", "label": "blood"},
			"template_class": "DNA",
			"pcr_target":     []any{map[string]any{"pcr_target_locus": "TRB"}},
		}},
		"data_processing": []any{map[string]any{
			"data_processing_id":    "simulated",
			"primary_annotation":    true,
			"software_versions":     "ClonoLoom simulate " + version,
			"data_processing_files": []any{file},
		}},
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readme(s Study, summary Summary, version string) string {
	text := fmt.Sprintf(`# %s

Simulated by ClonoLoom %s (clonoloom simulate --preset %s --seed %d): %d subjects, %d samples, %d
rearrangements. Open this folder in ClonoLoom (or the samples' .tsv files with
repertoires.airr.json) as one dataset; its truth is in truth/.

- Each sample is an AIRR rearrangement table: umi_count holds the templates, duplicate_count the
  reads. A sequence_id is the sample and the clone of truth/<subject>.tsv.gz it came from.
- repertoires.airr.json: each sample's AIRR Repertoire metadata (subject, condition).
- truth/truth.json: the parameters and seed; each subject's population as the analyses would
  measure it were every cell sequenced (clonotypes, Hill numbers, Shannon, Simpson, evenness,
  clonality, Gini, the largest ten's share, homeostasis); each sample's templates, reads and the
  clonotypes a sample of its depth shows on average; the clonotypes the subjects share.
- truth/<subject>.tsv.gz: every clone of the subject's population, its recombination (genes,
  deletions, insertions) and its cells.

The model: T-cell receptor β rearrangements of GRCh38's gene segments (Ensembl's annotation; no
restrictions), with the V and J usage and junction lengths of 10x Genomics' 10k PBMC dataset (CC BY
4.0); deletions and insertions of ClonoLoom's own choosing; clone sizes from a power law. These
data are simulated: they describe no person, and may be used and shared without restriction.
`, s.Title, version, s.Name, s.Seed, summary.Subjects, summary.Samples, summary.Rows)
	if s.Series != nil && s.Series.Response != nil {
		text += `
Some receptors recognize an epitope of an antigen: families of receptors around founders the
subjects share, and receptors of each subject's own, a known share of its blood at each time point.
The epitopes and antigens are made up.

- truth/specificity.tsv: every subject's receptors that recognize an epitope, with their epitope
  and founder (any other clonotype recognizes none of them); truth/founders.tsv: the founders.
- database/vdjdb.txt: a specificity database in VDJdb's format, each epitope's founders and
  receptors of other donors around them, with epitopes no subject responds to. ClonoLoom does
  not open it with the samples.
`
	}
	return text
}

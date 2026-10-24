package expression

// Expression preprocessing follows the total-count normalization and natural-log transform
// described by Wolf, Angerer and Theis (2018), Genome Biology 19:15,
// doi:10.1186/s13059-017-1382-0. PCA follows Jolliffe and Cadima (2016), Phil Trans R Soc A
// 374:20150202, doi:10.1098/rsta.2015.0202. UMAP follows McInnes, Healy and Melville (2018),
// arXiv:1802.03426. Variance ranking, bounds, no variance scaling and PCA initialization are
// ClonoLoom's choices, not a claim to reproduce a named highly-variable-gene recipe.
// Covariance and sign conventions follow SituCraft's internal/cluster/matrix.go (Apache-2.0).
import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"clonoloom/internal/dmath"
	"clonoloom/internal/embed"
	"clonoloom/internal/graph"
	"clonoloom/internal/linalg"
)

const AnalysisVersion = 1
const MaxAnalysisCells = 10000
const MaxAnalysisFeatures = 1000
const MaxCovarianceWork int64 = 2000000000

// AnalysisOptions are user choices; zero resolves to the stated default. Neighbors excludes
// the cell itself. Effective dimensions and neighbor counts are also recorded after filtering.
type AnalysisOptions struct {
	MinGenes   int      `json:"minGenes"`
	MinCells   int      `json:"minCells"`
	Features   int      `json:"features"`
	Components int      `json:"components"`
	Neighbors  int      `json:"neighbors"`
	Target     float64  `json:"target"`
	Seed       uint64   `json:"seed"`
	Epochs     int      `json:"epochs"`
	MinDist    *float64 `json:"minDist"`
}

func (o AnalysisOptions) Normalize() (AnalysisOptions, error) {
	if o.MinGenes == 0 {
		o.MinGenes = 1
	}
	if o.MinCells == 0 {
		o.MinCells = 1
	}
	if o.Features == 0 {
		o.Features = 500
	}
	if o.Components == 0 {
		o.Components = 30
	}
	if o.Neighbors == 0 {
		o.Neighbors = 15
	}
	if o.Target == 0 {
		o.Target = 10000
	}
	if o.Seed == 0 {
		o.Seed = 20261024
	}
	if o.Epochs == 0 {
		o.Epochs = 500
	}
	if o.MinDist == nil {
		v := 0.5
		o.MinDist = &v
	}
	if o.MinGenes < 1 || o.MinGenes > MaxFeatures || o.MinCells < 1 || o.MinCells > MaxAnalysisCells {
		return o, errors.New("minimum detected genes must be 1–200000 and minimum cells per gene 1–10000")
	}
	if o.Features < 2 || o.Features > MaxAnalysisFeatures || o.Components < 2 || o.Components > 100 || o.Neighbors < 2 || o.Neighbors > 100 {
		return o, errors.New("choose 2–1000 features, 2–100 components and 2–100 other neighbors")
	}
	if !(o.Target >= 1 && o.Target <= 1e6) || o.Epochs < 10 || o.Epochs > 1000 || !(*o.MinDist >= 0 && *o.MinDist <= 1) {
		return o, errors.New("target must be 1–1000000, epochs 10–1000 and minimum distance 0–1")
	}
	return o, nil
}

// AnalysisRules pins choices that are fixed in this version, including numerical thresholds.
func AnalysisRules() map[string]any {
	return map[string]any{
		"features":      "Gene Expression only; decreasing sample variance of log-normalized counts; ties by feature ID",
		"normalization": "ln(1 + count / (all Gene Expression counts in cell / target)); before feature filtering",
		"scaling":       "center selected features; no variance scaling, clipping, whitening, regression or batch correction",
		"cellOrder":     "source barcode order", "featureOrder": "feature ID", "distance": "Euclidean in retained PCA scores",
		"neighbors":    "exact; excludes self; ties by source barcode order; capped at retained cells minus one",
		"pca":          "Householder tridiagonalization and implicit-shift QL; largest absolute loading positive",
		"minimumCells": 4, "minimumRank": 2, "pcaMachineEpsilon": 0x1p-52, "pcaMaxQLIterations": 61, "rankTolerance": 1e-12, "graph": "UMAP fuzzy union a+b-a*b; local connectivity 1; bandwidth 1",
		"graphTarget": "log2(other neighbors + 1)", "graphPruning": "weights below maximum weight / epochs are omitted from UMAP optimization only; at 10 epochs use maximum weight / 500", "smoothKIterations": 64, "smoothKTolerance": 1e-5, "minKDistanceScale": 1e-3,
		"map":    "UMAP; two dimensions; PCA initialization; serial SGD; PCG stream 0",
		"spread": 1, "negativeRate": 5, "learningRate": 1, "repulsion": 1, "gradientClip": 4,
		"initialScale": "largest absolute PCA coordinate to 10, then noise, then each axis to [0,10]", "initialNoiseSD": 1e-4, "initialAxisRange": 10, "repulsionDistanceOffset": 0.001,
		"curveFit":       "300 points in [0,3]; Levenberg–Marquardt; a=b=1; 500 iterations; 30 damping trials; relative improvement 1e-14; initial damping 1e-3; damping factor 10; floor 1e-12",
		"maxSourceCells": 100000, "maxCells": MaxAnalysisCells, "maxFeatures": MaxAnalysisFeatures, "maxCovarianceWork": MaxCovarianceWork,
	}
}

type AnalysisFeature struct {
	Feature
	Detected int       `json:"detected"`
	Mean     float64   `json:"mean"`
	Variance float64   `json:"variance"`
	Loadings []float64 `json:"loadings"`
}
type AnalysisCell struct {
	Barcode  string      `json:"barcode"`
	Total    uint64      `json:"total"`
	Genes    int         `json:"genes"`
	Reason   string      `json:"reason,omitempty"`
	Scores   []float64   `json:"scores,omitempty"`
	Position *[2]float64 `json:"position,omitempty"`
	Values   []string    `json:"values"`
}
type AnalysisResult struct {
	Options          AnalysisOptions   `json:"options"`
	Rules            map[string]any    `json:"rules"`
	InputCells       int               `json:"inputCells"`
	Excluded         []AnalysisCell    `json:"excluded"`
	Cells            []AnalysisCell    `json:"cells"`
	Features         []AnalysisFeature `json:"features"`
	EligibleFeatures int               `json:"eligibleFeatures"`
	Components       int               `json:"components"`
	Variances        []float64         `json:"variances"`
	VarianceRatios   []float64         `json:"varianceRatios"`
	TotalVariance    float64           `json:"totalVariance"`
	Neighbors        *graph.Neighbors  `json:"neighbors"`
	Graph            *graph.CSR        `json:"graph"`
	GraphComponents  int               `json:"graphComponents"`
	Map              embed.Info        `json:"map"`
	Axes             [2]string         `json:"axes"`
	Columns          []string          `json:"columns"`
}

func product(a, b float64) float64 { return float64(a * b) }

// Analyze never mutates its input or uses supplied coordinates to fit a map. All raw expression
// cells are eligible independently of receptor coverage/QC. Excluded cells retain their reason.
func Analyze(ctx context.Context, in *Input, options AnalysisOptions, workers int, progress func(float64, string)) (*AnalysisResult, error) {
	o, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	if in == nil || in.Matrix == nil {
		return nil, errors.New("open expression counts first")
	}
	m := in.Matrix
	if len(m.Barcodes) > 100000 {
		return nil, errors.New("computed maps accept at most 100000 source barcodes; select a smaller source")
	}
	say := func(f float64, s string) {
		if progress != nil {
			progress(f, s)
		}
	}
	r := &AnalysisResult{Options: o, Rules: AnalysisRules(), InputCells: len(m.Barcodes), Axes: [2]string{"UMAP 1", "UMAP 2"}, Excluded: []AnalysisCell{}, Columns: []string{}}
	if in.Cells != nil {
		r.Columns = append(r.Columns, in.Cells.Coverage.Columns...)
	}
	indices := []int{}
	say(0, "Selecting expression cells")
	for i, barcode := range m.Barcodes {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		cell := AnalysisCell{Barcode: barcode, Total: m.GeneUMIs[i], Genes: m.Genes[i], Values: make([]string, len(r.Columns))}
		if in.Cells != nil {
			if at, ok := in.Cells.ByBarcode[barcode]; ok {
				copy(cell.Values, in.Cells.Rows[at].Values)
			}
		}
		if cell.Total == 0 {
			cell.Reason = "no Gene Expression counts"
		} else if cell.Genes < o.MinGenes {
			cell.Reason = "below minimum detected genes"
		}
		if cell.Reason != "" {
			r.Excluded = append(r.Excluded, cell)
			continue
		}
		if len(indices) == MaxAnalysisCells {
			return nil, fmt.Errorf("computed maps support at most %d eligible cells; select a smaller source or raise the minimum detected genes", MaxAnalysisCells)
		}
		indices = append(indices, i)
		r.Cells = append(r.Cells, cell)
	}
	n := len(indices)
	if n < 4 {
		return nil, fmt.Errorf("only %d cells pass expression filtering; a computed map needs at least four", n)
	}
	// Sparse normalization; moments include implicit and explicit zeros exactly once.
	sums := make([]float64, len(m.Features))
	means := make([]float64, len(m.Features))
	detected := make([]int, len(m.Features))
	values := make([]float64, len(m.Counts))
	for _, i := range indices {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		scale := float64(m.GeneUMIs[i]) / o.Target
		for e := m.Indptr[i]; e < m.Indptr[i+1]; e++ {
			g := m.Indices[e]
			if m.Features[g].Type != "Gene Expression" || m.Counts[e] == 0 {
				continue
			}
			v := dmath.Log1p(float64(m.Counts[e]) / scale)
			values[e] = v
			means[g] += v
			detected[g]++
		}
	}
	for g := range means {
		means[g] /= float64(n)
	}
	for _, i := range indices {
		for e := m.Indptr[i]; e < m.Indptr[i+1]; e++ {
			if values[e] == 0 {
				continue
			}
			g := m.Indices[e]
			d := values[e] - means[g]
			sums[g] += product(d, d)
		}
	}
	selected := []int{}
	for g, f := range m.Features {
		if f.Type != "Gene Expression" {
			continue
		}
		sums[g] = (sums[g] + product(float64(n-detected[g]), product(means[g], means[g]))) / float64(n-1)
		if detected[g] >= o.MinCells && sums[g] > 0 {
			selected = append(selected, g)
		}
	}
	r.EligibleFeatures = len(selected)
	sort.Slice(selected, func(i, j int) bool {
		a, b := selected[i], selected[j]
		if sums[a] != sums[b] {
			return sums[a] > sums[b]
		}
		return m.Features[a].ID < m.Features[b].ID
	})
	selected = selected[:min(o.Features, len(selected))]
	sort.Slice(selected, func(i, j int) bool { return m.Features[selected[i]].ID < m.Features[selected[j]].ID })
	p := len(selected)
	if p < 2 {
		return nil, errors.New("fewer than two variable Gene Expression features pass filtering")
	}
	if int64(n)*int64(p)*int64(p) > MaxCovarianceWork {
		return nil, errors.New("exact PCA exceeds the covariance work limit; choose fewer features or a smaller source")
	}
	say(.08, "Centering selected expression features")
	x := make([]float64, n*p)
	positions := make(map[uint32]int, p)
	for j, g := range selected {
		positions[uint32(g)] = j
		r.Features = append(r.Features, AnalysisFeature{Feature: m.Features[g], Detected: detected[g], Mean: means[g], Variance: sums[g]})
	}
	for row, i := range indices {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out := x[row*p : (row+1)*p]
		for j, g := range selected {
			out[j] = -means[g]
		}
		for e := m.Indptr[i]; e < m.Indptr[i+1]; e++ {
			if j, ok := positions[m.Indices[e]]; ok {
				out[j] += values[e]
			}
		}
	}
	say(.12, "Computing principal components")
	cov := make([]float64, p*p)
	if workers < 1 {
		workers = runtime.NumCPU()
	}
	workers = min(workers, p)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for j := w; j < p; j += workers {
				if ctx.Err() != nil {
					return
				}
				for k := 0; k <= j; k++ {
					v := 0.0
					for i := 0; i < n; i++ {
						v += product(x[i*p+j], x[i*p+k])
					}
					v /= float64(n - 1)
					cov[j*p+k], cov[k*p+j] = v, v
				}
			}
		}(w)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	eigen, vectors, err := linalg.SymEigenContext(ctx, cov, p)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rank := 0
	for _, v := range eigen {
		if v > product(eigen[0], 1e-12) {
			rank++
		}
	}
	k := min(o.Components, n-1, p, rank)
	if k < 2 {
		return nil, errors.New("expression has fewer than two nonzero principal components")
	}
	r.Components = k
	r.Variances = append([]float64(nil), eigen[:k]...)
	for j := 0; j < p; j++ {
		r.TotalVariance += cov[j*p+j]
		r.Features[j].Loadings = make([]float64, k)
	}
	for c := 0; c < k; c++ {
		r.VarianceRatios = append(r.VarianceRatios, eigen[c]/r.TotalVariance)
		at := 0
		for j := 1; j < p; j++ {
			if math.Abs(vectors[j*p+c]) > math.Abs(vectors[at*p+c]) {
				at = j
			}
		}
		sign := 1.0
		if vectors[at*p+c] < 0 {
			sign = -1
		}
		for j := 0; j < p; j++ {
			r.Features[j].Loadings[c] = sign * vectors[j*p+c]
		}
	}
	scores := make([]float64, n*k)
	initial := make([]float64, 2*n)
	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for c := 0; c < k; c++ {
			v := 0.0
			for j := 0; j < p; j++ {
				v += product(x[i*p+j], r.Features[j].Loadings[c])
			}
			scores[i*k+c] = v
		}
		r.Cells[i].Scores = scores[i*k : (i+1)*k]
		initial[2*i], initial[2*i+1] = scores[i*k], scores[i*k+1]
	}
	r.Neighbors, err = graph.KNN(ctx, scores, n, k, min(o.Neighbors, n-1), workers, func(f float64) { say(.35+float64(.25*f), "Finding exact expression neighbors") })
	if err != nil {
		return nil, err
	}
	r.Graph, err = graph.Fuzzy(r.Neighbors)
	if err != nil {
		return nil, err
	}
	_, r.GraphComponents = graph.Components(r.Graph)
	y, info, err := embed.Layout(ctx, r.Graph, initial, embed.Params{MinDist: *o.MinDist, Spread: 1, Epochs: o.Epochs, NegativeRate: 5, Seed: o.Seed}, 1, func(f float64) { say(.6+float64(.39*f), "Computing UMAP") })
	if err != nil {
		return nil, err
	}
	r.Map = info
	for i := range r.Cells {
		r.Cells[i].Position = &[2]float64{y[2*i], y[2*i+1]}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	say(1, "Expression map complete")
	return r, nil
}

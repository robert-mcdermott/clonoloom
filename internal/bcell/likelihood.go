package bcell

// JC69: Jukes & Cantor 1969, doi:10.1016/B978-1-4832-3211-9.50009-7.
// Site likelihoods sum unobserved states by
// pruning (Felsenstein 1981, doi:10.1007/BF01734359). Independent implementation.
import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/dmath"
)

const MLVersion = 1
const MLMaxTaxa = 7
const MLMaxSites = 1024
const MLMaxBranch = 3.0
const MLMaxSweeps = 100
const MLWorkBudget = 2000000000
const MLDescription = "JC69 baseline: equal base frequencies and exchange rates, independent constant-rate sites. All unrooted binary topologies are enumerated for up to seven distinct panel states including the fixed reference; branch lengths are fitted on [0,3] substitutions/site from three deterministic starts. Enumeration is exhaustive, continuous optimization is not proven global. The reported joint log likelihood includes all tip states and a uniform root-state prior. Constant sites are retained; identical panel sequences are not abundance weighted. The supplied reference orients the display; reversibility does not infer the root. Marginal ancestral probabilities condition on this selected topology, fitted lengths, fixed reference and complete-case V/J panel; they omit topology, parameter, germline, alignment and family uncertainty. Marginal modes are not a joint assignment. No context-dependent SHM, selection, switching, migration, affinity or dates are inferred. Boundary branches and near ties are not biological support."

type MLNode struct {
	SourceNode   string              `json:"sourceNode,omitempty"`
	ID           string              `json:"id"`
	Parent       string              `json:"parent,omitempty"`
	Kind         string              `json:"kind"`
	BranchLength float64             `json:"branchLength"`
	Boundary     string              `json:"boundary"`
	Depth        int                 `json:"depth"`
	Distance     float64             `json:"distance"`
	Sequence     string              `json:"sequence"`  // marginal modes, not a joint assignment
	Posterior    [][4]float64        `json:"posterior"` // A,C,G,T; panel column order
	Rows         int                 `json:"rows"`
	Annotations  []LineageAnnotation `json:"annotations"`
}
type MLStart struct {
	InitialBranch     float64 `json:"initialBranch"`
	LogLikelihood     float64 `json:"logLikelihood"`
	Sweeps            int     `json:"sweeps"`
	Converged         bool    `json:"converged"`
	MaxCoordinateGain float64 `json:"maxCoordinateGain"`
}
type MLFit struct {
	Starts              []MLStart `json:"starts"`
	Topology            string    `json:"topology"`
	LogLikelihood       float64   `json:"logLikelihood"`
	Converged           bool      `json:"converged"`
	Sweeps              int       `json:"sweeps"`
	MaxCoordinateGain   float64   `json:"maxCoordinateGain"`
	StartLogLikelihoods []float64 `json:"startLogLikelihoods"`
}
type MLSettings struct {
	BisectionSteps        int       `json:"bisectionSteps"`
	MarginalTieTolerance  float64   `json:"marginalTieTolerance"`
	MaxTaxa               int       `json:"maxTaxa"`
	MaxSites              int       `json:"maxSites"`
	MaxBranch             float64   `json:"maxBranch"`
	MaxSweeps             int       `json:"maxSweeps"`
	WorkBudget            int       `json:"workBudget"`
	Starts                []float64 `json:"starts"`
	SweepTolerance        float64   `json:"sweepTolerance"`
	StationarityTolerance float64   `json:"stationarityTolerance"`
	TopologyTieTolerance  float64   `json:"topologyTieTolerance"`
	NearBestTolerance     float64   `json:"nearBestTolerance"`
}
type MLResult struct {
	Settings      MLSettings `json:"settings"`
	Model         string     `json:"model"`
	Description   string     `json:"description"`
	Taxa          int        `json:"taxa"`
	Sites         int        `json:"sites"`
	LogLikelihood float64    `json:"logLikelihood"`
	Topologies    int        `json:"topologies"`
	NearBest      int        `json:"nearBest"`
	Work          int        `json:"work"`
	Fits          []MLFit    `json:"fits"`
	Nodes         []MLNode   `json:"nodes"`
	Newick        string     `json:"newick"`
}
type mlPattern struct {
	states  []int
	weight  int
	columns []int
}
type mlEngine struct {
	ctx      context.Context
	patterns []mlPattern
	work     int
}

func (m *mlEngine) charge(n int) error {
	m.work += n
	if e := m.ctx.Err(); e != nil {
		return e
	}
	if m.work > MLWorkBudget {
		return errors.New("JC69 search exceeds two billion work units; no partial tree is returned")
	}
	return nil
}
func mlPatterns(seqs []string) []mlPattern {
	out := []mlPattern{}
	by := map[string]int{}
	for col := range seqs[0] {
		var key strings.Builder
		for _, s := range seqs {
			key.WriteByte(s[col])
		}
		k := key.String()
		i, ok := by[k]
		if !ok {
			i = len(out)
			by[k] = i
			p := mlPattern{}
			for j := range seqs {
				p.states = append(p.states, strings.IndexByte("ACGT", seqs[j][col]))
			}
			out = append(out, p)
		}
		out[i].weight++
		out[i].columns = append(out[i].columns, col)
	}
	return out
}
func jcTransition(a, b int, t float64) float64 {
	x := dmath.Exp(-4 * t / 3)
	if a == b {
		return .25 + float64(.75*x)
	}
	return -.25 * dmath.Expm1(-4*t/3)
}

// At <=7 tips each per-site message has at most eleven factors: no across-site
// multiplication. Logs accumulate sites, including invariant columns.
func mlMessage(g treeGraph, lens [][]float64, p mlPattern, u, parent int) [4]float64 {
	out := [4]float64{1, 1, 1, 1}
	if u < len(p.states) {
		out = [4]float64{}
		out[p.states[u]] = 1
	}
	for _, v := range g[u] {
		if v == parent {
			continue
		}
		child := mlMessage(g, lens, p, v, u)
		for a := 0; a < 4; a++ {
			sum := 0.0
			for b := 0; b < 4; b++ {
				sum += float64(jcTransition(a, b, lens[u][v]) * child[b])
			}
			out[a] *= sum
		}
	}
	return out
}
func (m *mlEngine) score(g treeGraph, lens [][]float64) (float64, error) {
	if e := m.charge(len(m.patterns) * len(g) * 16); e != nil {
		return 0, e
	}
	sum := 0.0
	for _, p := range m.patterns {
		v := mlMessage(g, lens, p, 0, -1)
		prob := .25 * (v[0] + v[1] + v[2] + v[3])
		if prob <= 0 {
			return math.Inf(-1), nil
		}
		sum += float64(float64(p.weight) * dmath.Log(prob))
	}
	return sum, nil
}

type mlCoefficient struct {
	a, b   float64
	weight int
}

// Cutting one edge gives per-site L(x)=a+b*x, x=exp(-4t/3).
// Sum log L is concave in x. Bisection on its monotone derivative solves this
// coordinate exactly to floating-point precision, including both boundaries.
func (m *mlEngine) edge(g treeGraph, lens [][]float64, u, v int) (float64, error) {
	if e := m.charge(len(m.patterns) * len(g) * 16); e != nil {
		return 0, e
	}
	coeff := make([]mlCoefficient, len(m.patterns))
	for i, p := range m.patterns {
		left, right := mlMessage(g, lens, p, u, v), mlMessage(g, lens, p, v, u)
		sl, sr, dot := 0.0, 0.0, 0.0
		for a := 0; a < 4; a++ {
			sl += left[a]
			sr += right[a]
			dot += float64(left[a] * right[a])
		}
		a := float64(float64(sl*sr) / 16)
		coeff[i] = mlCoefficient{a, float64(.25*dot) - a, p.weight}
	}
	derivative := func(x float64) float64 {
		sum := 0.0
		for _, c := range coeff {
			den := c.a + float64(c.b*x)
			if den <= 0 {
				return math.Inf(-1)
			}
			sum += float64(c.weight) * c.b / den
		}
		return sum
	}
	lo, hi := dmath.Exp(-4*MLMaxBranch/3), 1.0
	if derivative(lo) <= 0 {
		return MLMaxBranch, nil
	}
	if derivative(hi) >= 0 {
		return 0, nil
	}
	for i := 0; i < 48; i++ {
		if e := m.charge(len(coeff)); e != nil {
			return 0, e
		}
		mid := (lo + hi) / 2
		if derivative(mid) > 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return -.75 * dmath.Log((lo+hi)/2), nil
}
func mlLengths(n int) [][]float64 {
	v := make([][]float64, n)
	for i := range v {
		v[i] = make([]float64, n)
	}
	return v
}
func (m *mlEngine) fit(g treeGraph, taxa int, start float64, maxSweeps int) (MLFit, [][]float64, error) {
	lens := mlLengths(len(g))
	edges := g.edges()
	for _, e := range edges {
		lens[e[0]][e[1]], lens[e[1]][e[0]] = start, start
	}
	fit := MLFit{Topology: g.key(taxa)}
	previous, e := m.score(g, lens)
	if e != nil {
		return fit, nil, e
	}
	for sweep := 1; sweep <= maxSweeps; sweep++ {
		for _, edge := range edges {
			u, v := edge[0], edge[1]
			t, e := m.edge(g, lens, u, v)
			if e != nil {
				return fit, nil, e
			}
			lens[u][v], lens[v][u] = t, t
		}
		score, e := m.score(g, lens)
		if e != nil {
			return fit, nil, e
		}
		fit.Sweeps = sweep
		if score < previous-1e-7 {
			return fit, nil, errors.New("JC69 optimizer lost likelihood")
		}
		if score-previous < 1e-9 {
			break
		}
		previous = score
	}
	fit.LogLikelihood, e = m.score(g, lens)
	if e != nil {
		return fit, nil, e
	}
	for _, edge := range edges {
		u, v := edge[0], edge[1]
		t, e := m.edge(g, lens, u, v)
		if e != nil {
			return fit, nil, e
		}
		old := lens[u][v]
		lens[u][v], lens[v][u] = t, t
		score, e := m.score(g, lens)
		lens[u][v], lens[v][u] = old, old
		if e != nil {
			return fit, nil, e
		}
		fit.MaxCoordinateGain = math.Max(fit.MaxCoordinateGain, score-fit.LogLikelihood)
	}
	fit.Converged = fit.MaxCoordinateGain <= 1e-7 && !math.IsInf(fit.LogLikelihood, 0) && !math.IsNaN(fit.LogLikelihood)
	return fit, lens, nil
}

// Likelihood uses only the saved observed/reference sequences and annotations.
// The saved parsimony topology and ancestral assignments do not enter the fit.
func Likelihood(ctx context.Context, panel LineageTree) (MLResult, error) {
	out := MLResult{Model: "JC69", Description: MLDescription, Sites: panel.Sites, Fits: []MLFit{}, Nodes: []MLNode{}}
	if panel.Excluded != "" || panel.Sites < 1 || panel.Sites > MLMaxSites || len(panel.Columns) != panel.Sites {
		return out, errors.New("JC69 needs an eligible saved panel of 1–1024 sites")
	}
	tips := []LineageNode{}
	var reference *LineageNode
	seen := map[string]bool{}
	for _, n := range panel.Nodes {
		if n.Kind == "reference" {
			if reference != nil {
				return out, errors.New("Multiple reference nodes")
			}
			copy := n
			reference = &copy
		} else if n.Kind == "observed" {
			tips = append(tips, n)
		}
	}
	if reference == nil {
		return out, errors.New("Missing fixed reference")
	}
	sort.Slice(tips, func(i, j int) bool { return tips[i].Sequence < tips[j].Sequence })
	tips = append([]LineageNode{*reference}, tips...)
	out.Taxa = len(tips)
	if len(tips) > MLMaxTaxa || len(tips) != panel.Taxa {
		return out, errors.New("JC69 supports at most seven distinct states including the reference; no subsampling")
	}
	seqs := []string{}
	for _, n := range tips {
		if len(n.Sequence) != panel.Sites || seen[n.Sequence] {
			return out, errors.New("Invalid or repeated saved panel state")
		}
		seen[n.Sequence] = true
		for i := range n.Sequence {
			if !canonical(n.Sequence[i]) {
				return out, errors.New("JC69 requires canonical complete-case panel bases")
			}
		}
		seqs = append(seqs, n.Sequence)
	}
	out.Settings = MLSettings{BisectionSteps: 48, MarginalTieTolerance: 1e-10, MaxTaxa: MLMaxTaxa, MaxSites: MLMaxSites, MaxBranch: MLMaxBranch, MaxSweeps: MLMaxSweeps, WorkBudget: MLWorkBudget, Starts: []float64{.001, .05, .5}, SweepTolerance: 1e-9, StationarityTolerance: 1e-7, TopologyTieTolerance: 1e-8, NearBestTolerance: 1e-6}
	engine := mlEngine{ctx: ctx, patterns: mlPatterns(seqs)}
	base := make(treeGraph, len(tips))
	if len(tips) > 1 {
		base.connect(0, 1)
	}
	graphs := []treeGraph{}
	lengthSets := [][][]float64{}
	var enumerate func(treeGraph, int) error
	enumerate = func(g treeGraph, next int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if next < len(tips) {
			for _, edge := range g.edges() {
				if e := enumerate(g.insert(edge, next), next+1); e != nil {
					return e
				}
			}
			return nil
		}
		var chosen MLFit
		var lengths [][]float64
		starts := []float64{}
		diagnostics := []MLStart{}
		chosen.LogLikelihood = math.Inf(-1)
		for _, start := range []float64{.001, .05, .5} {
			fit, ls, e := engine.fit(g, len(tips), start, MLMaxSweeps)
			if e != nil {
				return e
			}
			starts = append(starts, fit.LogLikelihood)
			diagnostics = append(diagnostics, MLStart{start, fit.LogLikelihood, fit.Sweeps, fit.Converged, fit.MaxCoordinateGain})
			if fit.LogLikelihood > chosen.LogLikelihood {
				chosen, lengths = fit, ls
			}
		}
		chosen.StartLogLikelihoods = starts
		chosen.Starts = diagnostics
		if !chosen.Converged {
			return fmt.Errorf("JC69 branch optimization failed stationarity for topology %s; no tree is returned", chosen.Topology)
		}
		out.Fits = append(out.Fits, chosen)
		graphs = append(graphs, g.copy())
		lengthSets = append(lengthSets, lengths)
		return nil
	}
	if e := enumerate(base, 2); e != nil {
		return out, e
	}
	out.Topologies = len(out.Fits)
	selected := mlSelect(out.Fits)
	bestGraph, bestLengths := graphs[selected], lengthSets[selected]
	best := out.Fits[selected].LogLikelihood
	out.LogLikelihood = best
	for _, fit := range out.Fits {
		if best-fit.LogLikelihood <= 1e-6 {
			out.NearBest++
		}
	}
	parents, order := bestGraph.rooted()
	ids := make([]string, len(bestGraph))
	for i := range ids {
		ids[i] = fmt.Sprintf("ml-node-%d", i+1)
	}
	depth := make([]int, len(ids))
	distance := make([]float64, len(ids))
	for _, u := range order {
		node := MLNode{ID: ids[u], Kind: "inferred", Posterior: make([][4]float64, panel.Sites), Annotations: []LineageAnnotation{}, Boundary: "root"}
		if u == 0 {
			node.Kind = "reference"
		} else if u < len(tips) {
			node.Kind = "observed"
		}
		if u < len(tips) {
			node.SourceNode = tips[u].ID
			node.Rows = tips[u].Rows
			node.Annotations = tips[u].Annotations
		}
		if p := parents[u]; p >= 0 {
			node.Parent = ids[p]
			node.BranchLength = bestLengths[u][p]
			depth[u] = depth[p] + 1
			distance[u] = distance[p] + node.BranchLength
			node.Boundary = "interior"
			if node.BranchLength == 0 {
				node.Boundary = "zero"
			} else if node.BranchLength == MLMaxBranch {
				node.Boundary = "upper"
			}
		}
		node.Depth, node.Distance = depth[u], distance[u]
		modes := make([]byte, panel.Sites)
		for _, pattern := range engine.patterns {
			if e := engine.charge(len(bestGraph) * 16); e != nil {
				return out, e
			}
			v := mlMessage(bestGraph, bestLengths, pattern, u, -1)
			sum := v[0] + v[1] + v[2] + v[3]
			if sum <= 0 {
				return out, errors.New("Undefined conditional ancestral probability")
			}
			top := 0.0
			for i := range v {
				v[i] /= sum
				top = math.Max(top, v[i])
			}
			mask := 0
			for i := range v {
				if top-v[i] <= 1e-10 {
					mask |= 1 << i
				}
			}
			for _, col := range pattern.columns {
				node.Posterior[col] = v
				modes[col] = "?ACMGRSVTWYHKDBN"[mask]
			}
		}
		node.Sequence = string(modes)
		out.Nodes = append(out.Nodes, node)
	}
	var newick func(int) string
	newick = func(u int) string {
		children := []string{}
		for _, v := range bestGraph[u] {
			if v != parents[u] {
				children = append(children, newick(v))
			}
		}
		sort.Strings(children)
		s := ids[u]
		if len(children) > 0 {
			s = "(" + strings.Join(children, ",") + ")" + s
		}
		if parents[u] >= 0 {
			s += ":" + strconv.FormatFloat(bestLengths[u][parents[u]], 'g', 17, 64)
		}
		return s
	}
	out.Newick = newick(0) + ";"
	out.Work = engine.work
	return out, nil
}

// Anchor the tie window to the overall maximum, not the most recently selected
// near tie: approximate equality is not transitive.
func mlSelect(fits []MLFit) int {
	maximum := math.Inf(-1)
	for _, f := range fits {
		maximum = math.Max(maximum, f.LogLikelihood)
	}
	selected := -1
	for i, f := range fits {
		if maximum-f.LogLikelihood <= 1e-8 && (selected < 0 || f.Topology < fits[selected].Topology) {
			selected = i
		}
	}
	return selected
}

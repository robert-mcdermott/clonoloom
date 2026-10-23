// Package neighborhood finds the receptors around each of a dataset's receptors that a background
// does not explain (clonoloom-spec/requirements.md, S2; roadmap.md, wave 4, slice 5), as
// Mayer-Blackwell et al. 2021 define them (eLife 10:e68605): each receptor is a centroid, and its
// radius is the largest at which the background within it is a small enough share of the
// background's receptors; the receptors of the dataset within that radius are its neighborhood,
// and how much commoner they are there than in the background is its enrichment.
//
// A receptor recombination makes often has many background neighbors, and so takes a small radius;
// one recombination rarely makes can take a large one. A neighborhood of many receptors at a large
// radius is one the background does not explain: the receptors may share a specificity.
//
// Neighborhoods are of one locus's chains (a bulk study's β chains), as Mayer-Blackwell et al.
// work and as a background is drawn. background.go makes the backgrounds; motif.go, each
// neighborhood's pattern.
package neighborhood

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"clonoloom/internal/dmath"
	"clonoloom/internal/tcrdist"
)

// Receptor is one chain of a clonotype: its CDR3 (an amino-acid junction) and its V and J genes.
// Clonotype is the clonotype it came from (its index in a table; −1 in a background), Count its
// abundance and Samples the samples that hold it.
type Receptor struct {
	CDR3      string  `json:"cdr3"`
	V         string  `json:"v"`
	J         string  `json:"j"`
	Clonotype int     `json:"clonotype"`
	Count     float64 `json:"count"`
	Samples   int     `json:"samples"`
}

// Repertoire is receptors of one locus, each with its weight (a background's; nil, or empty,
// weighs each 1).
type Repertoire struct {
	Locus     string     `json:"locus"`
	Receptors []Receptor `json:"receptors"`
	Weight    []float64  `json:"-"`
}

// Len is the number of receptors.
func (r *Repertoire) Len() int { return len(r.Receptors) }

// weight answers receptor i's weight.
func (r *Repertoire) weight(i int) float64 {
	if i < len(r.Weight) {
		return r.Weight[i]
	}
	return 1
}

// Options are how neighborhoods are found: Control, the share of the background's receptors a
// centroid's radius may hold (Mayer-Blackwell et al.'s 10⁻⁶; tcrdist3's own default is 10⁻⁵);
// MaxRadius, the largest radius considered, in steps of Step from 0; and MinNeighbors, the
// neighbors a centroid needs to be reported (itself counted).
type Options struct {
	Control      float64 `json:"control"`
	MaxRadius    int     `json:"maxRadius"`
	Step         int     `json:"step"`
	MinNeighbors int     `json:"minNeighbors"`
}

// DefaultOptions are Mayer-Blackwell et al.'s.
func DefaultOptions() Options {
	return Options{Control: 1e-6, MaxRadius: 48, Step: 2, MinNeighbors: 2}
}

// Check says what is wrong with the options.
func (o Options) Check() error {
	switch {
	case !(o.Control > 0) || o.Control > 1:
		return errors.New("the control rate is a share of the background above 0 and at most 1")
	case o.MaxRadius < 0 || o.MaxRadius > 200:
		return errors.New("the largest radius is from 0 to 200 TCRdist units")
	case o.Step < 1 || o.Step > o.MaxRadius && o.MaxRadius > 0:
		return errors.New("the step between radii is at least 1 and at most the largest radius")
	case o.MinNeighbors < 1:
		return errors.New("a neighborhood holds its centroid, so it has a neighbor or more")
	}
	return nil
}

// thresholds are the radii considered: 0, Step, …, at most MaxRadius.
func (o Options) thresholds() []int {
	var out []int
	for t := 0; t <= o.MaxRadius; t += o.Step {
		out = append(out, t)
	}
	return out
}

// Center is a receptor's neighborhood: its radius, the receptors of the dataset within it (the
// centroid counted, as tcrdist3 counts it), the background within it, weighted and not, and how
// much commoner the neighborhood is in the dataset than in the background.
type Center struct {
	// Receptor: the centroid, by its index in the dataset's repertoire.
	Receptor int `json:"receptor"`
	Radius   int `json:"radius"`
	// Neighbors: the dataset's receptors within the radius; Abundance, their total; Samples, the
	// samples that hold one; Subjects, the subjects (0 where none are named).
	Neighbors int     `json:"neighbors"`
	Abundance float64 `json:"abundance"`
	Samples   int     `json:"samples"`
	Subjects  int     `json:"subjects"`
	// Background: the background's receptors within the radius, and Weighted, their weight.
	Background int     `json:"background"`
	Weighted   float64 `json:"weighted"`
	// Rate: the share of the dataset within the radius, (n + 1)/(N + 2); BackgroundRate, the
	// weighted share of the background; RelativeRate, the first over the second.
	Rate           float64 `json:"rate"`
	BackgroundRate float64 `json:"backgroundRate"`
	RelativeRate   float64 `json:"relativeRate"`
	// OddsRatio of being in the neighborhood, with one added to each count; ChiSquare, a χ² test
	// of the two with Yates's correction.
	OddsRatio float64 `json:"oddsRatio"`
	ChiSquare float64 `json:"chiSquare"`
	// Motif: the neighbors' residues at each position of the centroid's CDR3 (motif.go).
	Motif string `json:"motif"`
	// Members: the neighborhood's receptors, by their indices in the dataset's repertoire, the
	// centroid first and the rest nearest first; Distances, each one's distance from the centroid.
	Members   []int `json:"members"`
	Distances []int `json:"distances"`
}

// pairing encodes two repertoires' chains against one table of V alleles, so that a distance is a
// CDR3 distance and a lookup.
type pairing struct {
	a, b       []tcrdist.Chain
	alleleA    []int32
	alleleB    []int32
	loop       [][]int
	loops      bool
	Organism   string
	Alleles    map[string]string
	NoLoops    int
	maxGapCost int
}

// encode makes the pairing: each receptor's CDR3, and its V allele's loops where loops is given
// (nil: the CDR3s alone). A receptor whose V gene the database lacks keeps its CDR3 and is
// counted.
func encode(sample, background *Repertoire, loops *tcrdist.Loops, organism string) (*pairing, error) {
	p := &pairing{Alleles: map[string]string{}, Organism: organism, maxGapCost: 4 * 3}
	if loops != nil {
		p.loops = true
		if p.Organism == "" {
			var calls []string
			for _, r := range sample.Receptors {
				calls = append(calls, r.V)
			}
			p.Organism, _ = loops.Guess(calls)
		}
	}
	index := map[string]int32{}
	var chains []tcrdist.Chain
	one := func(rs []Receptor) ([]tcrdist.Chain, []int32) {
		out := make([]tcrdist.Chain, len(rs))
		ids := make([]int32, len(rs))
		for i, r := range rs {
			out[i] = tcrdist.Chain{CDR3: tcrdist.Encode(r.CDR3)}
			ids[i] = -1
			if !p.loops {
				continue
			}
			got, allele, ok := loops.Lookup(p.Organism, r.V)
			if !ok {
				p.NoLoops++
				continue
			}
			if allele != r.V {
				p.Alleles[r.V] = allele
			}
			id, seen := index[allele]
			if !seen {
				id = int32(len(chains))
				index[allele] = id
				chains = append(chains, tcrdist.NewChain("", got[:]))
			}
			ids[i] = id
		}
		return out, ids
	}
	p.a, p.alleleA = one(sample.Receptors)
	p.b, p.alleleB = one(background.Receptors)
	if p.loops {
		// Every pair of alleles' loops, once.
		p.loop = make([][]int, len(chains))
		for i := range chains {
			p.loop[i] = make([]int, len(chains))
			for j := range chains {
				p.loop[i][j] = tcrdist.LoopDistance(chains[i], chains[j])
			}
		}
	}
	if len(p.a) == 0 {
		return nil, errors.New("the dataset has no receptors of this locus with a junction")
	}
	return p, nil
}

// loopsBetween is the loops' distance between a sample receptor and a background one (0 where
// either's V gene has no loops).
func (p *pairing) loopsBetween(i, j int) int {
	if !p.loops || p.alleleA[i] < 0 || p.alleleB[j] < 0 {
		return 0
	}
	return p.loop[p.alleleA[i]][p.alleleB[j]]
}

func (p *pairing) distance(i, j int) int {
	return tcrdist.CDR3Distance(p.a[i], p.b[j]) + p.loopsBetween(i, j)
}

// within answers the dataset's receptors within the radius of receptor i (itself included), and
// their distances, by comparing it with every other.
func (p *pairing) within(i, radius int, self *pairing) ([]int, []int) {
	members, distances := []int{i}, []int{0}
	for j := range self.a {
		if j == i {
			continue
		}
		// A lower bound no alignment avoids: the loops', and the CDR3s' difference in length.
		lb := 0
		if self.loops && self.alleleA[i] >= 0 && self.alleleA[j] >= 0 {
			lb = self.loop[self.alleleA[i]][self.alleleA[j]]
		}
		if diff := len(self.a[i].CDR3) - len(self.a[j].CDR3); diff != 0 {
			if diff < 0 {
				diff = -diff
			}
			lb += diff * p.maxGapCost
		}
		if lb > radius {
			continue
		}
		d := tcrdist.CDR3Distance(self.a[i], self.a[j])
		if self.loops && self.alleleA[i] >= 0 && self.alleleA[j] >= 0 {
			d += self.loop[self.alleleA[i]][self.alleleA[j]]
		}
		if d <= radius {
			members = append(members, j)
			distances = append(distances, d)
		}
	}
	order := make([]int, len(members))
	for k := range order {
		order[k] = k
	}
	sort.SliceStable(order, func(x, y int) bool { return distances[order[x]] < distances[order[y]] })
	outM, outD := make([]int, len(order)), make([]int, len(order))
	for k, at := range order {
		outM[k], outD[k] = members[at], distances[at]
	}
	return outM, outD
}

// Result is a dataset's neighborhoods: the centroids reported, and what was compared.
type Result struct {
	Options Options `json:"options"`
	// Receptors compared, and Background, the background's receptors.
	Receptors  int `json:"receptors"`
	Background int `json:"background"`
	// Organism whose loops were used ("" for the CDR3s alone); NoLoops, the receptors whose V gene
	// the loops' database lacks (compared by their CDR3s alone); Alleles, V calls read as an
	// allele they do not name.
	Organism string            `json:"organism,omitempty"`
	NoLoops  int               `json:"noLoops"`
	Alleles  map[string]string `json:"alleles,omitempty"`
	Centers  []Center          `json:"centers"`
}

// Find answers the neighborhood of each of the dataset's receptors against the background: its
// radius, the receptors within it, and its enrichment. Each centroid is computed whole by one
// worker, so the answer does not depend on how many there are (workers 0: one for each
// processor). With loops (nil for the CDR3s alone), the V genes' loops are part of the distance.
func Find(ctx context.Context, sample, background *Repertoire, loops *tcrdist.Loops, organism string, o Options, workers int, progress func(done, of int)) (*Result, error) {
	if err := o.Check(); err != nil {
		return nil, err
	}
	if background.Len() == 0 {
		return nil, errors.New("the background has no receptors")
	}
	p, err := encode(sample, background, loops, organism)
	if err != nil {
		return nil, err
	}
	// The dataset against itself, with the same alleles.
	self, err := encode(sample, sample, loops, p.Organism)
	if err != nil {
		return nil, err
	}
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	thresholds := o.thresholds()
	n := sample.Len()
	total := float64(background.Len())
	// subjectsOf and samplesOf: a neighborhood's samples and subjects are its receptors' pooled.
	out := make([]Center, n)
	var next, done atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counts := make([]float64, len(thresholds))
			raw := make([]float64, len(thresholds))
			for {
				if ctx.Err() != nil {
					return
				}
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				for k := range counts {
					counts[k], raw[k] = 0, 0
				}
				// The background within each threshold of this centroid.
				for j := range p.b {
					lb := p.loopsBetween(i, j)
					if diff := len(p.a[i].CDR3) - len(p.b[j].CDR3); diff != 0 {
						if diff < 0 {
							diff = -diff
						}
						lb += diff * p.maxGapCost
					}
					if lb > o.MaxRadius {
						continue
					}
					d := p.distance(i, j)
					if d > o.MaxRadius {
						continue
					}
					at := sort.SearchInts(thresholds, d)
					if at < len(thresholds) {
						counts[at] += background.weight(j)
						raw[at]++
					}
				}
				// The largest threshold whose weighted share is within the control rate; where
				// none is, the smallest radius, whose background is counted all the same.
				at := 0
				cumW, cumN := 0.0, 0.0
				for k := range thresholds {
					cumW += counts[k]
					cumN += raw[k]
					counts[k], raw[k] = cumW, cumN
					if cumW/total <= o.Control {
						at = k
					}
				}
				radius, bg, weighted := thresholds[at], int(raw[at]), counts[at]
				members, distances := p.within(i, radius, self)
				out[i] = Center{Receptor: i, Radius: radius, Neighbors: len(members), Background: bg, Weighted: weighted, Members: members, Distances: distances}
				summarize(&out[i], sample, n, total)
				out[i].Motif = Motif(sample, members, self)
				if d := done.Add(1); progress != nil && d%64 == 0 {
					progress(int(d), n)
				}
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(n, n)
	}
	res := &Result{Options: o, Receptors: n, Background: background.Len(), Organism: p.Organism, NoLoops: p.NoLoops, Alleles: p.Alleles}
	for i := range out {
		if out[i].Neighbors >= o.MinNeighbors {
			res.Centers = append(res.Centers, out[i])
		}
	}
	sort.SliceStable(res.Centers, func(x, y int) bool {
		if a, b := res.Centers[x].RelativeRate < 1, res.Centers[y].RelativeRate < 1; a != b {
			return !a
		}
		if a, b := enrichmentScore(res.Centers[x]), enrichmentScore(res.Centers[y]); a != b {
			return a > b
		}
		return res.Centers[x].Receptor < res.Centers[y].Receptor
	})
	if !p.loops {
		res.Organism = ""
	}
	return res, nil
}

// The two-sided χ² measures departure in either direction. Rank enrichment first, then lack
// of a difference, then depletion. Keep the reported statistic unchanged for reference checks.
func enrichmentScore(c Center) float64 {
	if c.RelativeRate < 1 {
		return -c.ChiSquare
	}
	return c.ChiSquare
}

// summarize fills a center's abundance, samples and enrichment.
func summarize(c *Center, sample *Repertoire, n int, total float64) {
	samples := 0
	for _, m := range c.Members {
		r := sample.Receptors[m]
		c.Abundance += r.Count
		samples = max(samples, r.Samples)
	}
	c.Samples = samples
	c.Rate = rate(float64(c.Neighbors), float64(n-c.Neighbors))
	c.BackgroundRate = rate(c.Weighted, total-c.Weighted)
	c.RelativeRate = c.Rate / c.BackgroundRate
	c.OddsRatio = oddsRatio(float64(c.Neighbors), float64(n-c.Neighbors), c.Weighted, total-c.Weighted)
	c.ChiSquare = chiSquare(float64(c.Neighbors), float64(n-c.Neighbors), c.Weighted, total-c.Weighted)
}

// rate is (hits + 1)/(hits + misses + 2), as Mayer-Blackwell et al. take a rate.
func rate(hits, misses float64) float64 {
	return float64(hits+1) / float64(hits+misses+2)
}

// oddsRatio of being within the radius in the dataset against the background, one added to each
// count so that it is finite.
func oddsRatio(hits, misses, bHits, bMisses float64) float64 {
	return (float64(1+hits) / float64(1+misses)) / (float64(1+bHits) / float64(1+bMisses))
}

// chiSquare tests the dataset's receptors within the radius against the background's, one added
// to each count, with Yates's correction for continuity (as scipy's chi2_contingency applies it
// to a 2 × 2 table).
func chiSquare(hits, misses, bHits, bMisses float64) float64 {
	o := [2][2]float64{{1 + hits, 1 + misses}, {1 + bHits, 1 + bMisses}}
	rows := [2]float64{o[0][0] + o[0][1], o[1][0] + o[1][1]}
	cols := [2]float64{o[0][0] + o[1][0], o[0][1] + o[1][1]}
	all := rows[0] + rows[1]
	if all <= 0 {
		return 0
	}
	out := 0.0
	for i := range 2 {
		for j := range 2 {
			e := float64(rows[i]*cols[j]) / all
			if e <= 0 {
				continue
			}
			diff := math.Abs(o[i][j] - e)
			// Yates's correction, no larger than the difference itself.
			d := math.Max(0, diff-math.Min(0.5, diff))
			out += float64(d*d) / e
		}
	}
	return out
}

// Log10 of a value at least the smallest positive float (for plots of rates).
func Log10(v float64) float64 {
	if !(v > 0) {
		return math.Inf(-1)
	}
	return dmath.Log(v) / math.Ln10
}

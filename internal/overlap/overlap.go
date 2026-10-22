// Package overlap measures how much samples' repertoires overlap (clonoloom-spec/requirements.md,
// R4). For every pair of samples: the clonotypes they share; Jaccard's index, Sørensen–Dice's and
// the overlap coefficient over their clonotypes, each also as expected at a common depth (both
// samples rarefied to the smallest sample's size, exactly); the Morisita–Horn index (Horn 1966)
// and Morisita's own (Morisita 1959) over their abundances; and the share of each sample's
// abundance in the clonotypes the other holds too. Each with an interval from a bootstrap of the
// samples' cells. And the public clonotypes: those found in more than one subject.
//
// The clonotypes are a clonotype table's (under the dataset's definition), so that two samples
// share a clonotype when the definition calls their rearrangements the same.
// clonoloom-spec/research.md §5.6 has the choices and their sources.
package overlap

import (
	"math"
	"runtime"
	"sort"
	"sync"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/random"
)

// Measures' ids. The presence measures' rarefied forms add "Rarefied" to the id.
const (
	Shared       = "shared"
	Jaccard      = "jaccard"
	Sorensen     = "sorensen"
	Coefficient  = "overlap"
	MorisitaHorn = "morisitaHorn"
	Morisita     = "morisita"
	// AbundanceOfA: the share of sample A's abundance in clonotypes B holds too; AbundanceOfB the
	// other way.
	AbundanceOfA = "abundanceOfA"
	AbundanceOfB = "abundanceOfB"
	Rarefied     = "Rarefied"
)

// Presence are the measures over clonotypes seen, which grow with depth.
var Presence = []string{Shared, Jaccard, Sorensen, Coefficient}

// IDs are every measure's id, in the order they are listed.
var IDs = []string{Shared, Jaccard, Sorensen, Coefficient, Shared + Rarefied, Jaccard + Rarefied, Sorensen + Rarefied, Coefficient + Rarefied, MorisitaHorn, Morisita, AbundanceOfA, AbundanceOfB}

type (
	Number = diversity.Number
	Value  = diversity.Value
)

// Sample is a sample as the overlap sees it: its subject and group (condition) as given, its
// clonotypes and abundance, and whether its abundances are whole numbers (the rarefied measures,
// Morisita's index and the bootstrap need counts).
type Sample struct {
	Name       string  `json:"name"`
	Subject    string  `json:"subject,omitempty"`
	Group      string  `json:"group,omitempty"`
	Clonotypes int     `json:"clonotypes"`
	Abundance  float64 `json:"abundance"`
	Whole      bool    `json:"whole"`
}

// Pair is two samples (A < B, by index) and every measure of their overlap.
type Pair struct {
	A        int              `json:"a"`
	B        int              `json:"b"`
	Measures map[string]Value `json:"measures"`
}

// Range is a measure's mean, lowest and highest over pairs.
type Range struct {
	Mean Number `json:"mean"`
	Min  Number `json:"min"`
	Max  Number `json:"max"`
}

// Summary is the pairs of a kind (same subject, different subjects, same condition, different
// conditions), each measure's estimates summarized over them.
type Summary struct {
	Kind     string           `json:"kind"`
	Pairs    int              `json:"pairs"`
	Measures map[string]Range `json:"measures"`
}

// PublicClonotype is a clonotype found in more than one subject (or sample).
type PublicClonotype struct {
	ID        string            `json:"id"`
	Chains    []clonotype.Chain `json:"chains"`
	Units     int               `json:"units"`
	Samples   []string          `json:"samples"`
	Abundance float64           `json:"abundance"`
}

// Incidence is how many clonotypes were found in exactly Units subjects (or samples).
type Incidence struct {
	Units      int `json:"units"`
	Clonotypes int `json:"clonotypes"`
}

// Public is the clonotypes by the number of subjects that hold them, and the most public. By is
// "subject" when the samples' subjects are given (a sample without one its own), else "sample".
type Public struct {
	By        string            `json:"by"`
	Units     int               `json:"units"`
	Incidence []Incidence       `json:"incidence"`
	Top       []PublicClonotype `json:"top"`
}

// TopPublic is how many of the most public clonotypes are listed.
const TopPublic = 100

// Result is every sample, every pair, the summaries and the public clonotypes. Depth is the common
// size the rarefied measures are taken at (0 when no sample has whole counts).
type Result struct {
	Samples    []Sample  `json:"samples"`
	Depth      float64   `json:"depth"`
	Pairs      []Pair    `json:"pairs"`
	Summaries  []Summary `json:"summaries"`
	Public     Public    `json:"public"`
	Replicates int       `json:"replicates"`
}

// member is a clonotype of a sample: its index in the table and its abundance there.
type member struct {
	clonotype int
	abundance float64
}

// shared is a clonotype two samples share: its place in each one's members.
type shared struct{ a, b int32 }

type sampleData struct {
	members []member
	n       float64
	whole   bool
}

// Measure measures every pair of samples of a clonotype table (names: the samples' names, by index)
// and the public clonotypes. subjects and groups name each sample's subject and condition (by
// name; missing for none). opt is the bootstrap's.
func Measure(t *clonotype.Table, names []string, subjects, groups map[string]string, opt diversity.Options) Result {
	if opt.Confidence <= 0 || opt.Confidence >= 1 {
		opt.Confidence = 0.95
	}
	z := diversity.NormalQuantile(0.5 + opt.Confidence/2)
	ns := len(names)
	data := make([]sampleData, ns)
	for s := range data {
		data[s].whole = true
	}
	for i := range t.Clonotypes {
		samples, values := t.Abundances(i)
		for k, s := range samples {
			if int(s) >= ns || values[k] <= 0 {
				continue
			}
			d := &data[s]
			d.members = append(d.members, member{clonotype: i, abundance: values[k]})
			d.n += values[k]
			if values[k] != math.Trunc(values[k]) {
				d.whole = false
			}
		}
	}
	res := Result{}
	depth := math.Inf(1)
	for s, name := range names {
		d := data[s]
		res.Samples = append(res.Samples, Sample{Name: name, Subject: subjects[name], Group: groups[name], Clonotypes: len(d.members), Abundance: d.n, Whole: d.whole})
		if d.whole && len(d.members) > 0 {
			depth = min(depth, d.n)
		}
	}
	if math.IsInf(depth, 1) {
		depth = 0
	}
	res.Depth = depth
	// Where each clonotype is in each sample's members, and the pairs' shared clonotypes.
	place := make([]map[int]int32, ns)
	for s, d := range data {
		place[s] = make(map[int]int32, len(d.members))
		for k, m := range d.members {
			place[s][m.clonotype] = int32(k)
		}
	}
	pairIndex := func(a, b int) int { return a*ns + b }
	sharedOf := map[int][]shared{}
	for i := range t.Clonotypes {
		samples, _ := t.Abundances(i)
		var in []int
		for _, s := range samples {
			if int(s) < ns {
				if _, ok := place[s][i]; ok {
					in = append(in, int(s))
				}
			}
		}
		sort.Ints(in)
		for x := 0; x < len(in); x++ {
			for y := x + 1; y < len(in); y++ {
				a, b := in[x], in[y]
				sharedOf[pairIndex(a, b)] = append(sharedOf[pairIndex(a, b)], shared{place[a][i], place[b][i]})
			}
		}
	}
	// The estimates, and the bootstrap's replicates of them.
	counts := make([][]float64, ns)
	for s, d := range data {
		counts[s] = make([]float64, len(d.members))
		for k, m := range d.members {
			counts[s][k] = m.abundance
		}
	}
	point := measureAll(data, counts, sharedOf, depth, pairIndex)
	unseen, zeroUpper := sharingVariance(data, sharedOf, depth, pairIndex, point, opt.Confidence)
	var replicates [][][]float64
	bootable := true
	for _, d := range data {
		bootable = bootable && d.whole
	}
	if opt.Replicates > 1 && bootable && ns > 1 {
		replicates = bootstrap(data, sharedOf, depth, pairIndex, opt)
		res.Replicates = opt.Replicates
	}
	k := 0
	for a := 0; a < ns; a++ {
		for b := a + 1; b < ns; b++ {
			p := Pair{A: a, B: b, Measures: map[string]Value{}}
			for m, id := range IDs {
				est := point[k][m]
				v := Value{Estimate: Number(est), SE: Number(math.NaN()), Lower: Number(math.NaN()), Upper: Number(math.NaN())}
				if replicates != nil && !math.IsNaN(est) {
					column := make([]float64, 0, len(replicates))
					for _, r := range replicates {
						column = append(column, r[k][m])
					}
					if se := math.Sqrt(spread(column)*spread(column) + unseen[k][m]); !math.IsNaN(se) {
						hi := 1.0
						if id == Shared || id == Shared+Rarefied || id == Morisita {
							hi = math.Inf(1)
						}
						// On the logarithmic scale, as fits counts and indices near zero, whose
						// distributions are skewed; where none is shared, from zero to what a count
						// of none allows.
						v.SE = Number(se)
						if est > 0 {
							f := math.Exp(z * se / est)
							v.Lower = Number(min(est/f, hi))
							v.Upper = Number(min(est*f, hi))
						} else {
							v.Lower, v.Upper = 0, Number(min(zeroUpper[k][m], hi))
						}
					}
				}
				p.Measures[id] = v
			}
			res.Pairs = append(res.Pairs, p)
			k++
		}
	}
	res.Summaries = summarize(res.Samples, res.Pairs)
	res.Public = public(t, names, data, subjects)
	return res
}

// sharingVariance is, for every measure of every pair, the variance the bootstrap misses, and the
// upper bound of its interval where the pair shares no clonotype (as for a count of none, Poisson's
// exact −ln((1 − confidence)/2) shared clonotypes, each of one cell in each sample). Which
// clonotypes two samples share is itself random, each shared with a chance π (independently,
// nearly), and its variance over the shared clonotypes, Σ y²π(1 − π) for a sum of their parts y,
// is estimated without bias by Σ y²(1 − π) over those seen shared (Horvitz and Thompson 1952),
// with π = (1 − (1 − p)^A)(1 − (1 − q)^B) from each clonotype's frequencies p and q in the two
// repertoires (diversity.Detected: a clonotype seen once is rarer than 1/A). Resampling the
// samples' cells redraws only the clonotypes seen, each shared again with the chance π̂ its counts
// give, (1 − (1 − a/A)^A)(1 − (1 − b/B)^B), and so has Σ y²π̂(1 − π̂) of it: this adds the rest,
// through each measure's derivative in its sum. Where two samples share few clonotypes (different
// subjects), it is most of the uncertainty.
func sharingVariance(data []sampleData, sharedOf map[int][]shared, depth float64, pairIndex func(a, b int) int, point [][]float64, confidence float64) (variance, zeroUpper [][]float64) {
	ns := len(data)
	none := -math.Log((1 - confidence) / 2)
	detected := make([][]float64, ns)
	for s, d := range data {
		if d.whole {
			abundances := make([]float64, len(d.members))
			for k, m := range d.members {
				abundances[k] = m.abundance
			}
			detected[s] = diversity.Detected(abundances)
		}
	}
	var out [][]float64
	k := 0
	for a := 0; a < ns; a++ {
		for b := a + 1; b < ns; b++ {
			row := make([]float64, len(IDs))
			da, db := data[a], data[b]
			if !da.whole || !db.whole {
				out = append(out, row)
				zeroUpper = append(zeroUpper, make([]float64, len(IDs)))
				k++
				continue
			}
			A, B := da.n, db.n
			var vs, vr, vab, va, vb float64
			var sa, sb, sq, sqb, sq1, sq1b float64
			for _, m := range da.members {
				sa++
				sq += m.abundance * m.abundance
				sq1 += m.abundance * (m.abundance - 1)
			}
			for _, m := range db.members {
				sb++
				sqb += m.abundance * m.abundance
				sq1b += m.abundance * (m.abundance - 1)
			}
			var ea, eb float64
			if depth > 0 {
				for _, m := range da.members {
					ea += diversity.SeenChance(A, m.abundance, depth)
				}
				for _, m := range db.members {
					eb += diversity.SeenChance(B, m.abundance, depth)
				}
			}
			for _, sh := range sharedOf[pairIndex(a, b)] {
				x, y := da.members[sh.a].abundance, db.members[sh.b].abundance
				seenAgain := -math.Expm1(A*math.Log1p(-x/A)) * -math.Expm1(B*math.Log1p(-y/B))
				pa, pb := min(detected[a][sh.a], 1), min(detected[b][sh.b], 1)
				pi := -math.Expm1(A*math.Log1p(-pa)) * -math.Expm1(B*math.Log1p(-pb))
				miss := max((1-pi)-seenAgain*(1-seenAgain), 0)
				vs += miss
				if depth > 0 {
					rho := diversity.SeenChance(A, x, depth) * diversity.SeenChance(B, y, depth)
					vr += rho * rho * miss
				}
				vab += x * y * x * y * miss
				va += x * x * miss
				vb += y * y * miss
			}
			S, R := point[k][0], point[k][4]
			row[0] = vs
			row[1] = sq2((sa+sb)/((sa+sb-S)*(sa+sb-S))) * vs
			row[2] = sq2(2/(sa+sb)) * vs
			row[3] = sq2(1/min(sa, sb)) * vs
			if depth > 0 {
				row[4] = vr
				row[5] = sq2((ea+eb)/((ea+eb-R)*(ea+eb-R))) * vr
				row[6] = sq2(2/(ea+eb)) * vr
				row[7] = sq2(1/min(ea, eb)) * vr
			}
			row[8] = sq2(2/((sq/(A*A)+sqb/(B*B))*A*B)) * vab
			if A > 1 && B > 1 {
				row[9] = sq2(2/((sq1/(A*(A-1))+sq1b/(B*(B-1)))*A*B)) * vab
			}
			row[10] = va / (A * A)
			row[11] = vb / (B * B)
			for i, v := range row {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					row[i] = 0
				}
			}
			out = append(out, row)
			// None shared: up to `none` clonotypes of one cell in each, at the depth as likely seen as
			// the samples' clonotypes on average.
			up := make([]float64, len(IDs))
			up[0] = none
			up[1] = none / (sa + sb - none)
			up[2] = 2 * none / (sa + sb)
			up[3] = none / min(sa, sb)
			if depth > 0 && sa > 0 && sb > 0 {
				r := none * (ea / sa) * (eb / sb)
				up[4], up[5], up[6], up[7] = r, r/(ea+eb-r), 2*r/(ea+eb), r/min(ea, eb)
			}
			up[8] = 2 * none / ((sq/(A*A) + sqb/(B*B)) * A * B)
			if A > 1 && B > 1 {
				up[9] = 2 * none / ((sq1/(A*(A-1)) + sq1b/(B*(B-1))) * A * B)
			}
			up[10], up[11] = none/A, none/B
			zeroUpper = append(zeroUpper, up)
			k++
		}
	}
	return out, zeroUpper
}

func sq2(x float64) float64 { return x * x }

// measureAll computes every measure of every pair (a < b, in order) from the samples' counts.
func measureAll(data []sampleData, counts [][]float64, sharedOf map[int][]shared, depth float64, pairIndex func(a, b int) int) [][]float64 {
	ns := len(data)
	type totals struct{ s, sum, sq, sq1, rarefied float64 }
	tot := make([]totals, ns)
	seen := make([]func(x float64) float64, ns)
	for s, d := range data {
		var t totals
		cache := map[float64]float64{}
		n := d.n
		seen[s] = func(x float64) float64 {
			if v, ok := cache[x]; ok {
				return v
			}
			v := diversity.SeenChance(n, x, depth)
			cache[x] = v
			return v
		}
		for _, x := range counts[s] {
			if x <= 0 {
				continue
			}
			t.s++
			t.sum += x
			t.sq += x * x
			t.sq1 += x * (x - 1)
			if d.whole && depth > 0 {
				t.rarefied += seen[s](x)
			}
		}
		tot[s] = t
	}
	var out [][]float64
	for a := 0; a < ns; a++ {
		for b := a + 1; b < ns; b++ {
			var n, ab, inA, inB, rarefied float64
			rarefiable := data[a].whole && data[b].whole && depth > 0
			for _, sh := range sharedOf[pairIndex(a, b)] {
				x, y := counts[a][sh.a], counts[b][sh.b]
				if x <= 0 || y <= 0 {
					continue
				}
				n++
				ab += x * y
				inA += x
				inB += y
				if rarefiable {
					rarefied += seen[a](x) * seen[b](y)
				}
			}
			ta, tb := tot[a], tot[b]
			row := make([]float64, len(IDs))
			presence := func(shared, sa, sb float64, at int) {
				row[at] = shared
				row[at+1] = ratio(shared, sa+sb-shared)
				row[at+2] = ratio(2*shared, sa+sb)
				row[at+3] = ratio(shared, min(sa, sb))
			}
			presence(n, ta.s, tb.s, 0)
			if rarefiable {
				presence(rarefied, ta.rarefied, tb.rarefied, 4)
			} else {
				for i := 4; i < 8; i++ {
					row[i] = math.NaN()
				}
			}
			A, B := ta.sum, tb.sum
			row[8] = ratio(2*ab, (ta.sq/(A*A)+tb.sq/(B*B))*A*B)
			if data[a].whole && data[b].whole && A > 1 && B > 1 {
				row[9] = ratio(2*ab, (ta.sq1/(A*(A-1))+tb.sq1/(B*(B-1)))*A*B)
			} else {
				row[9] = math.NaN()
			}
			row[10] = ratio(inA, A)
			row[11] = ratio(inB, B)
			out = append(out, row)
		}
	}
	return out
}

func ratio(a, b float64) float64 {
	if b == 0 {
		return math.NaN()
	}
	return a / b
}

// bootstrap draws replicates of every sample, each its cells drawn again (a multinomial of its
// size over its clonotypes' frequencies), and measures every pair again. Each replicate has its
// own stream, so the result is the same with any number of processors.
func bootstrap(data []sampleData, sharedOf map[int][]shared, depth float64, pairIndex func(a, b int) int, opt diversity.Options) [][][]float64 {
	chances := make([][]float64, len(data))
	for s, d := range data {
		chances[s] = make([]float64, len(d.members))
		for k, m := range d.members {
			chances[s][k] = m.abundance / d.n
		}
	}
	out := make([][][]float64, opt.Replicates)
	work := make(chan int)
	var wg sync.WaitGroup
	for range max(1, runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var draws []int
			for b := range work {
				r := random.New(opt.Seed, opt.Stream<<24|uint64(b))
				counts := make([][]float64, len(data))
				for s, d := range data {
					draws = r.Multinomial(int(math.Round(d.n)), chances[s], draws)
					counts[s] = make([]float64, len(draws))
					for k, c := range draws {
						counts[s][k] = float64(c)
					}
				}
				out[b] = measureAll(data, counts, sharedOf, depth, pairIndex)
			}
		}()
	}
	for b := range opt.Replicates {
		work <- b
	}
	close(work)
	wg.Wait()
	return out
}

func spread(xs []float64) float64 {
	var sum, n float64
	for _, x := range xs {
		if !math.IsNaN(x) && !math.IsInf(x, 0) {
			sum += x
			n++
		}
	}
	if n < 2 {
		return math.NaN()
	}
	mean := sum / n
	var ss float64
	for _, x := range xs {
		if !math.IsNaN(x) && !math.IsInf(x, 0) {
			ss += (x - mean) * (x - mean)
		}
	}
	return math.Sqrt(ss / (n - 1))
}

// summarize summarizes the pairs of the same subject and of different ones, of the same condition
// and of different ones (pairs where both samples have one): each measure's mean, lowest and
// highest.
func summarize(samples []Sample, pairs []Pair) []Summary {
	kinds := []struct {
		kind string
		of   func(a, b Sample) bool
	}{
		{"same subject", func(a, b Sample) bool { return a.Subject != "" && a.Subject == b.Subject }},
		{"different subjects", func(a, b Sample) bool { return a.Subject != "" && b.Subject != "" && a.Subject != b.Subject }},
		{"same condition", func(a, b Sample) bool { return a.Group != "" && a.Group == b.Group }},
		{"different conditions", func(a, b Sample) bool { return a.Group != "" && b.Group != "" && a.Group != b.Group }},
	}
	var out []Summary
	for _, k := range kinds {
		s := Summary{Kind: k.kind, Measures: map[string]Range{}}
		sums := map[string][3]float64{}
		counted := map[string]int{}
		for _, p := range pairs {
			if !k.of(samples[p.A], samples[p.B]) {
				continue
			}
			s.Pairs++
			for id, v := range p.Measures {
				x := float64(v.Estimate)
				if math.IsNaN(x) {
					continue
				}
				r, ok := sums[id]
				if !ok {
					r = [3]float64{0, math.Inf(1), math.Inf(-1)}
				}
				sums[id] = [3]float64{r[0] + x, min(r[1], x), max(r[2], x)}
				counted[id]++
			}
		}
		if s.Pairs == 0 {
			continue
		}
		for id, r := range sums {
			s.Measures[id] = Range{Mean: Number(r[0] / float64(counted[id])), Min: Number(r[1]), Max: Number(r[2])}
		}
		out = append(out, s)
	}
	return out
}

// public counts each clonotype's subjects (a sample without a subject its own; every sample its own
// when no subject is given), and lists the most public: by subjects, then samples, then abundance.
func public(t *clonotype.Table, names []string, data []sampleData, subjects map[string]string) Public {
	unitOf := make([]string, len(names))
	by := "sample"
	for _, name := range names {
		if subjects[name] != "" {
			by = "subject"
		}
	}
	units := map[string]bool{}
	for s, name := range names {
		if by == "subject" && subjects[name] != "" {
			unitOf[s] = "subject\x1f" + subjects[name]
		} else {
			unitOf[s] = "sample\x1f" + name
		}
		if len(data[s].members) > 0 {
			units[unitOf[s]] = true
		}
	}
	out := Public{By: by, Units: len(units)}
	incidence := map[int]int{}
	var top []PublicClonotype
	for i, c := range t.Clonotypes {
		samples, values := t.Abundances(i)
		in := map[string]bool{}
		var sampleNames []string
		total := 0.0
		for k, s := range samples {
			if int(s) >= len(names) || values[k] <= 0 {
				continue
			}
			in[unitOf[s]] = true
			sampleNames = append(sampleNames, names[s])
			total += values[k]
		}
		if len(in) == 0 {
			continue
		}
		incidence[len(in)]++
		if len(in) > 1 {
			top = append(top, PublicClonotype{ID: c.ID, Chains: c.Chains, Units: len(in), Samples: sampleNames, Abundance: total})
		}
	}
	for k := 1; k <= out.Units; k++ {
		out.Incidence = append(out.Incidence, Incidence{Units: k, Clonotypes: incidence[k]})
	}
	sort.Slice(top, func(a, b int) bool {
		x, y := top[a], top[b]
		switch {
		case x.Units != y.Units:
			return x.Units > y.Units
		case len(x.Samples) != len(y.Samples):
			return len(x.Samples) > len(y.Samples)
		case x.Abundance != y.Abundance:
			return x.Abundance > y.Abundance
		}
		return x.ID < y.ID
	})
	if len(top) > TopPublic {
		top = top[:TopPublic]
	}
	out.Top = top
	return out
}

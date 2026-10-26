// Package motifs screens overlapping TRB sequence signatures. Published inspiration:
// Huang et al. 2020, doi:10.1038/s41587-020-0505-4. This is a documented
// GLIPH2-style adaptation, not GLIPH2 or its composite scores. Research §5.35.
package motifs

import (
	"clonoloom/internal/conga"
	"clonoloom/internal/diversity"
	"clonoloom/internal/dmath"
	"clonoloom/internal/tcrdist"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

const Version = 1
const Alphabet = "ACDEFGHIKLMNPQRSTVWY"
const Citation = "Huang H et al. Nat Biotechnol. 2020;38:1194–1202. doi:10.1038/s41587-020-0505-4. Published signature rules; ClonoLoom adaptation, not the GLIPH2 implementation or composite score. Henikoff S, Henikoff JG. PNAS. 1992;89:10915–10919. doi:10.1073/pnas.89.22.10915. Fisher RA. J R Stat Soc. 1922;85:87–94. doi:10.2307/2340521. Benjamini Y, Yekutieli D. Ann Stat. 2001;29:1165–1188. doi:10.1214/aos/1013699998."
const Description = "GLIPH2-style sequence-signature screen, a ClonoLoom adaptation. Unique V-gene/amino-acid-junction units per arm, not reads, cells or independent subjects. Match query and explicit background within exact TRBV gene and full junction length. Leave three residues at each end outside local motif search and global substitution positions. Local contiguous k-mers use fixed windows of four possible starts (maximum start separation three residues); a receptor counts once per signature. Global groups share every full-junction position except one; allowed variable residues form maximal pairwise-positive BLOSUM62 sets, with no transitive merging. Groups overlap. Enumerate signatures from the pooled query/background before using arm labels, then one-sided Fisher enrichment within each stratum and BY across all pooled signatures, including untestable slots. Reporting filters do not shrink this family. No N/P boost, composite GLIPH2 score, HLA test, clonal-expansion score, inferred antigen label, causal or subject-level inference. Conditional p-values require exchangeable sequence units within strata; clonally related receptors, donor structure, repertoire sampling and background ascertainment can violate this. BY cannot repair those assumptions. Treat groups as exploratory hypotheses requiring independent validation."

type Options struct {
	MinK       int     `json:"minK"`
	MaxK       int     `json:"maxK"`
	MinMembers int     `json:"minMembers"`
	Q          float64 `json:"q"`
	MinFold    float64 `json:"minFold"`
}

func (o Options) Normalize() (Options, error) {
	if o.MinK == 0 {
		o.MinK = 2
	}
	if o.MaxK == 0 {
		o.MaxK = 5
	}
	if o.MinMembers == 0 {
		o.MinMembers = 3
	}
	if o.Q == 0 {
		o.Q = .05
	}
	if o.MinFold == 0 {
		o.MinFold = 2
	}
	if o.MinK < 2 || o.MaxK > 5 || o.MaxK < o.MinK || o.MinMembers < 2 || o.MinMembers > 100 || !(o.Q > 0 && o.Q <= 1) || !(o.MinFold >= 1 && o.MinFold <= 1000) {
		return o, errors.New("motifs require k=2–5, 2–100 members, q in (0,1], fold 1–1000")
	}
	return o, nil
}

type Record struct {
	ID         string   `json:"id"`
	V          string   `json:"v"`
	Junction   string   `json:"junction"`
	Samples    []string `json:"samples"`
	Subjects   []string `json:"subjects"`
	Clonotypes []string `json:"clonotypes"`
	Abundance  float64  `json:"abundance"`
}

func ID(v, j string) string {
	sum := sha256.Sum256([]byte(v + "\n" + j))
	return "trb-" + hex.EncodeToString(sum[:])[:20]
}
func Valid(v, j string) bool {
	if !strings.HasPrefix(v, "TRBV") || strings.ContainsAny(v, "*|, \t\n") || len(j) < 9 || len(j) > 40 || j[0] != 'C' || (j[len(j)-1] != 'F' && j[len(j)-1] != 'W') {
		return false
	}
	for i := range j {
		if !strings.ContainsRune(Alphabet, rune(j[i])) {
			return false
		}
	}
	return true
}

type Stratum struct {
	V          string `json:"v"`
	Length     int    `json:"length"`
	Query      int    `json:"query"`
	Background int    `json:"background"`
}
type Group struct {
	ID              string           `json:"id"`
	Kind            string           `json:"kind"`
	Motif           string           `json:"motif"`
	Allowed         string           `json:"allowed"`
	Start           int              `json:"start"`
	End             int              `json:"end"`
	V               string           `json:"v"`
	Length          int              `json:"length"`
	Query           []int            `json:"query"`
	Background      []int            `json:"background"`
	QueryTotal      int              `json:"queryTotal"`
	BackgroundTotal int              `json:"backgroundTotal"`
	P               diversity.Number `json:"p"`
	Q               diversity.Number `json:"q"`
	LogP            diversity.Number `json:"logP"`
	LogQ            diversity.Number `json:"logQ"`
	Fold            diversity.Number `json:"fold"`
	Score           diversity.Number `json:"score"`
	Reason          string           `json:"reason"`
	Reported        bool             `json:"reported"`
}
type Result struct {
	Description string    `json:"description"`
	Options     Options   `json:"options"`
	Query       []Record  `json:"query"`
	Background  []Record  `json:"background"`
	Shared      int       `json:"shared"`
	Strata      []Stratum `json:"strata"`
	Groups      []Group   `json:"groups"`
	Family      int       `json:"family"`
	Tested      int       `json:"tested"`
	Reported    int       `json:"reported"`
}

func missing() diversity.Number { return diversity.Number(math.NaN()) }

// Maximal residue cliques prevent A~B~C from joining A and C when they are not
// conservative substitutes. Bron–Kerbosch with a pivot over <=20 residues.
func cliques(letters string) []string {
	n := len(letters)
	adj := make([]uint32, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if tcrdist.PositiveSubstitution(letters[i], letters[j]) {
				adj[i] |= 1 << j
			}
		}
	}
	out := []string{}
	var visit func(uint32, uint32, uint32)
	visit = func(r, p, x uint32) {
		if p|x == 0 {
			if r&(r-1) != 0 {
				var b strings.Builder
				for i := 0; i < n; i++ {
					if r&(1<<i) != 0 {
						b.WriteByte(letters[i])
					}
				}
				out = append(out, b.String())
			}
			return
		}
		pivot := 0
		for (p|x)&(1<<pivot) == 0 {
			pivot++
		}
		candidates := p &^ adj[pivot]
		for i := 0; i < n; i++ {
			bit := uint32(1 << i)
			if candidates&bit == 0 {
				continue
			}
			visit(r|bit, p&adj[i], x&adj[i])
			p &^= bit
			x |= bit
		}
	}
	visit(0, (1<<n)-1, 0)
	sort.Strings(out)
	return out
}
func Compute(ctx context.Context, query, background []Record, options Options, progress func(float64)) (*Result, error) {
	o, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	if len(query) == 0 || len(background) == 0 || len(query)+len(background) > 4000 {
		return nil, errors.New("choose nonempty query/background arms with at most 4000 total distinct TRBV/junction units")
	}
	for _, arm := range [][]Record{query, background} {
		seen := map[string]bool{}
		for _, r := range arm {
			if !Valid(r.V, r.Junction) || r.ID != ID(r.V, r.Junction) || seen[r.ID] || !(r.Abundance > 0) || math.IsInf(r.Abundance, 0) {
				return nil, errors.New("invalid or duplicate captured receptor")
			}
			seen[r.ID] = true
		}
	}
	r := &Result{Description: Description, Options: o, Query: query, Background: background, Groups: []Group{}, Strata: []Stratum{}}
	pooled := append(append([]Record{}, query...), background...)
	strata := map[string]*Stratum{}
	seenQ := map[string]bool{}
	for _, v := range query {
		seenQ[v.ID] = true
	}
	for _, v := range background {
		if seenQ[v.ID] {
			r.Shared++
		}
	}
	candidates := map[string]*Group{}
	global := map[string]map[byte][]int{}
	add := func(key string, g Group, index int) error {
		p := candidates[key]
		if p == nil {
			if len(candidates) >= 100000 {
				return errors.New("signature family exceeds 100000; select a smaller declared repertoire")
			}
			g.ID = "mg-" + fmt.Sprintf("%x", sha256.Sum256([]byte(key)))[:20]
			g.Query = []int{}
			g.Background = []int{}
			p = &g
			candidates[key] = p
		}
		if index < len(query) {
			p.Query = append(p.Query, index)
		} else {
			p.Background = append(p.Background, index-len(query))
		}
		return nil
	}
	for i, rec := range pooled {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sk := fmt.Sprintf("%s/%02d", rec.V, len(rec.Junction))
		s := strata[sk]
		if s == nil {
			s = &Stratum{V: rec.V, Length: len(rec.Junction)}
			strata[sk] = s
		}
		if i < len(query) {
			s.Query++
		} else {
			s.Background++
		}
		once := map[string]bool{}
		for k := o.MinK; k <= o.MaxK; k++ {
			for at := 3; at+k <= len(rec.Junction)-3; at++ {
				word := rec.Junction[at : at+k]
				for start := max(3, at-3); start <= at; start++ {
					end := min(start+3, len(rec.Junction)-3-k)
					key := fmt.Sprintf("%s/local/%s/%02d", sk, word, start)
					if once[key] {
						continue
					}
					once[key] = true
					if err := add(key, Group{Kind: "local", Motif: word, Start: start + 1, End: end + 1, V: rec.V, Length: len(rec.Junction)}, i); err != nil {
						return nil, err
					}
				}
			}
		}
		for at := 3; at < len(rec.Junction)-3; at++ {
			pattern := rec.Junction[:at] + "%" + rec.Junction[at+1:]
			key := sk + "/" + pattern
			g := global[key]
			if g == nil {
				g = map[byte][]int{}
				global[key] = g
			}
			g[rec.Junction[at]] = append(g[rec.Junction[at]], i)
		}
	}
	globalKeys := []string{}
	for key := range global {
		globalKeys = append(globalKeys, key)
	}
	sort.Strings(globalKeys)
	for _, key := range globalKeys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g := global[key]
		letters := []byte{}
		for aa := range g {
			letters = append(letters, aa)
		}
		sort.Slice(letters, func(i, j int) bool { return letters[i] < letters[j] })
		for _, set := range cliques(string(letters)) {
			index := g[set[0]][0]
			rec := pooled[index]
			pattern := key[strings.LastIndexByte(key, '/')+1:]
			at := strings.IndexByte(pattern, '%')
			for j := range set {
				for _, i := range g[set[j]] {
					if err := add(key+"/"+set, Group{Kind: "global", Motif: pattern, Allowed: set, Start: at + 1, End: at + 1, V: rec.V, Length: len(rec.Junction)}, i); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	keys := []string{}
	for k := range candidates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	r.Family = len(keys)
	sks := []string{}
	for k := range strata {
		sks = append(sks, k)
	}
	sort.Strings(sks)
	for _, k := range sks {
		r.Strata = append(r.Strata, *strata[k])
	}
	work := 0
	for i, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g := candidates[key]
		sort.Ints(g.Query)
		sort.Ints(g.Background)
		s := strata[fmt.Sprintf("%s/%02d", g.V, g.Length)]
		g.QueryTotal, g.BackgroundTotal = s.Query, s.Background
		g.P, g.Q, g.LogP, g.LogQ, g.Fold, g.Score = missing(), missing(), missing(), missing(), missing(), missing()
		if s.Query == 0 || s.Background == 0 {
			g.Reason = "Stratum lacks query or background; no matched test"
		} else {
			a, b := len(g.Query), len(g.Background)
			work += min(a+b, s.Query) + 1
			if work > 20000000 {
				return nil, errors.New("exact-test work exceeds 20 million support points")
			}
			lp := conga.LogTail(s.Query+s.Background, a+b, s.Query, a)
			g.LogP = diversity.Number(lp)
			g.P = diversity.Number(dmath.Exp(lp))
			r.Tested++
			if b > 0 {
				g.Fold = diversity.Number(float64(a) * float64(s.Background) / (float64(b) * float64(s.Query)))
			} else {
				g.Reason = "No background matches; fold enrichment is undefined, not a finite estimate"
			}
		}
		r.Groups = append(r.Groups, *g)
		if progress != nil && i%256 == 0 {
			progress(float64(i) / float64(max(1, len(keys))))
		}
	}
	// BY in log space retains ordering and scores for tails that underflow as p.
	order := make([]int, len(r.Groups))
	for i := range order {
		order[i] = i
	}
	logp := func(i int) float64 {
		p := float64(r.Groups[i].LogP)
		if math.IsNaN(p) {
			return 0
		}
		return p
	}
	sort.SliceStable(order, func(i, j int) bool { return logp(order[i]) < logp(order[j]) })
	harmonic := 0.0
	for i := 1; i <= len(order); i++ {
		harmonic += 1 / float64(i)
	}
	next := 0.0
	for rank := len(order) - 1; rank >= 0; rank-- {
		i := order[rank]
		g := &r.Groups[i]
		value := logp(i) + dmath.Log(float64(len(order))) + dmath.Log(harmonic) - dmath.Log(float64(rank+1))
		next = math.Min(next, value)
		if math.IsNaN(float64(g.LogP)) {
			continue
		}
		g.LogQ = diversity.Number(next)
		g.Q = diversity.Number(dmath.Exp(next))
		g.Score = diversity.Number(-next / dmath.Log(10))
		foldOK := len(g.Background) == 0 && len(g.Query) > 0 || float64(g.Fold) >= o.MinFold
		g.Reported = len(g.Query) >= o.MinMembers && foldOK && next <= dmath.Log(o.Q)
		if g.Reported {
			r.Reported++
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

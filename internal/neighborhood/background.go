package neighborhood

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"clonoloom/internal/pgen"
	"clonoloom/internal/random"
)

// Source says what a background is: its kind ("model", "shuffled" or "dataset"), what it was made
// from, and how many receptors it holds. Weighted says whether its receptors carry weights.
type Source struct {
	Kind string `json:"kind"`
	// Name: the model's, or the dataset's.
	Name      string `json:"name,omitempty"`
	Checksum  string `json:"checksum,omitempty"`
	Receptors int    `json:"receptors"`
	Seed      uint64 `json:"seed,omitempty"`
	Weighted  bool   `json:"weighted"`
	// Pairs: the dataset's V–J pairs the background covers, and Missing, those its model cannot
	// make (their receptors are compared against the rest).
	Pairs   int `json:"pairs,omitempty"`
	Missing int `json:"missing,omitempty"`
}

// usage counts a repertoire's V–J pairs, by the genes' names.
func usage(r *Repertoire) (map[[2]string]float64, float64) {
	out := map[[2]string]float64{}
	total := 0.0
	for _, x := range r.Receptors {
		if x.V == "" || x.J == "" {
			continue
		}
		out[[2]string{gene(x.V), gene(x.J)}]++
		total++
	}
	for k, n := range out {
		out[k] = n / total
	}
	return out, total
}

// gene is a call without its allele.
func gene(call string) string {
	for i := 0; i < len(call); i++ {
		if call[i] == '*' {
			return call[:i]
		}
	}
	return call
}

// ModelBackground draws a background from a recombination model, conditioned on the dataset's V–J
// usage: each pair of genes gets its share of size receptors, drawn from the model with those
// genes. Each is weighted by the model's own probability of its pair over the dataset's share of
// it, so that the weighted background is the repertoire recombination alone makes: a receptor of
// genes the dataset over-uses weighs less than one drawn in proportion.
//
// Only productive CDR3s are kept (a repertoire holds no others), and only pairs the model can
// make; the rest are counted in the Source.
func ModelBackground(ctx context.Context, sample *Repertoire, m *pgen.Model, name, checksum string, size int, seed uint64, progress func(done, of int)) (*Repertoire, *Source, error) {
	if size < 1 {
		return nil, nil, errors.New("a background needs receptors")
	}
	q, _ := usage(sample)
	if len(q) == 0 {
		return nil, nil, errors.New("the dataset's receptors name no V and J genes, which a drawn background needs")
	}
	p := m.Usage()
	g := pgen.NewGenerator(m)
	pairs := make([][2]string, 0, len(q))
	for k := range q {
		pairs = append(pairs, k)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	out := &Repertoire{Locus: sample.Locus}
	src := &Source{Kind: "model", Name: name, Checksum: checksum, Seed: seed, Weighted: true}
	// The pairs the model can make, and their share of the dataset.
	kept := 0.0
	for _, k := range pairs {
		if g.Genes(k[0], k[1]) {
			kept += q[k]
		}
	}
	if !(kept > 0) {
		return nil, nil, fmt.Errorf("the model %s uses none of the dataset's V and J genes", name)
	}
	r := random.New(seed, 11)
	for _, k := range pairs {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if !g.Genes(k[0], k[1]) {
			src.Missing++
			continue
		}
		src.Pairs++
		// Its share of the background, of the pairs the model can make.
		share := q[k] / kept
		n := int(math.Round(float64(share * float64(size))))
		if n < 1 {
			n = 1
		}
		// The weight: what recombination makes over what the dataset holds.
		weight := 0.0
		if share > 0 {
			weight = p[k] / share
		}
		for made := 0; made < n; {
			draw, ok := g.DrawWith(r, k[0], k[1])
			if !ok {
				break
			}
			if !draw.Productive || draw.AA == "" {
				continue
			}
			out.Receptors = append(out.Receptors, Receptor{CDR3: draw.AA, V: draw.V, J: draw.J, Clonotype: -1, Count: 1})
			out.Weight = append(out.Weight, weight)
			made++
			if progress != nil && len(out.Receptors)%2048 == 0 {
				progress(len(out.Receptors), size)
			}
		}
	}
	if len(out.Receptors) == 0 {
		return nil, nil, fmt.Errorf("no receptor could be drawn from the model %s", name)
	}
	src.Receptors = len(out.Receptors)
	if progress != nil {
		progress(src.Receptors, src.Receptors)
	}
	return out, src, nil
}

// ShuffledBackground is the dataset's own receptors with their CDR3s moved among them: each keeps
// its V and J genes and takes another's CDR3, so that the genes and the CDR3s are as the dataset
// has them but no longer together. Every receptor weighs 1.
func ShuffledBackground(sample *Repertoire, seed uint64) (*Repertoire, *Source) {
	out := &Repertoire{Locus: sample.Locus, Receptors: make([]Receptor, len(sample.Receptors))}
	copy(out.Receptors, sample.Receptors)
	r := random.New(seed, 12)
	for i := len(out.Receptors) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		out.Receptors[i].CDR3, out.Receptors[j].CDR3 = out.Receptors[j].CDR3, out.Receptors[i].CDR3
	}
	for i := range out.Receptors {
		out.Receptors[i].Clonotype = -1
		out.Receptors[i].Count = 1
		out.Receptors[i].Samples = 0
	}
	return out, &Source{Kind: "shuffled", Receptors: len(out.Receptors), Seed: seed}
}

// DatasetBackground is another dataset's receptors, each weighing 1.
func DatasetBackground(receptors *Repertoire, name string) (*Repertoire, *Source) {
	out := &Repertoire{Locus: receptors.Locus, Receptors: make([]Receptor, len(receptors.Receptors))}
	copy(out.Receptors, receptors.Receptors)
	for i := range out.Receptors {
		out.Receptors[i].Clonotype = -1
	}
	return out, &Source{Kind: "dataset", Name: name, Receptors: len(out.Receptors)}
}

package cohort

// Conditional randomization of observed midranks handles ties without a normal
// approximation. Monte Carlo includes the observed assignment via (tail+1)/(B+1):
// Phipson & Smyth 2010, doi:10.2202/1544-6115.1585. Absolute centered scores
// define our two-sided tail; with unbalanced tied data this need not equal twice
// the smaller one-sided tail. Exact zero paired differences are omitted from ranks.
import (
	"context"
	"errors"
	"hash/fnv"
	"math"
	"sort"

	"clonoloom/internal/diversity"
	"clonoloom/internal/random"
)

func doubledRanks(values []float64) []int {
	order := make([]int, len(values))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return values[order[i]] < values[order[j]] })
	ranks := make([]int, len(values))
	for i := 0; i < len(order); {
		j := i + 1
		for j < len(order) && values[order[j]] == values[order[i]] {
			j++
		}
		for k := i; k < j; k++ {
			ranks[order[k]] = i + 1 + j
		}
		i = j
	}
	return ranks
}
func combinations(n, k int) int {
	k = min(k, n-k)
	c := 1
	for i := 1; i <= k; i++ {
		c = c * (n - i + 1) / i
		if c > ExactLimit {
			return ExactLimit + 1
		}
	}
	return c
}
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func rankTest(ctx context.Context, a, b []float64, paired bool, o Options, id string, budget *int) (Test, Effect, error) {
	test := emptyTest("Mann–Whitney U (comparison group)")
	name := "Probability of superiority, half credit for ties"
	var ranks []int
	center, observed, nChoose := 0, 0, len(b)
	probability, scale := 0.0, 1/float64(len(a))+1/float64(len(b))
	assignments := combinations(len(a)+len(b), len(b))
	if paired {
		test.Name = "Wilcoxon signed-rank W+"
		name = "Within-subject probability of increase, half credit for zero difference"
		scale = 1 / float64(len(a))
		var differences, absolute []float64
		for i := range a {
			d := b[i] - a[i]
			if !finite(d) {
				test.Reason = "Non-finite paired difference"
				effect := emptyEffect(name, "fraction", "Conservative bounded-difference interval")
				effect.Reason = test.Reason
				return test, effect, nil
			}
			if d > 0 {
				probability++
			} else if d == 0 {
				probability += .5
			}
			if d != 0 {
				differences = append(differences, d)
				absolute = append(absolute, math.Abs(d))
			}
		}
		probability /= float64(len(a))
		ranks = doubledRanks(absolute)
		positive := 0
		total := 0
		for i, r := range ranks {
			total += r
			if differences[i] > 0 {
				positive += r
			}
		}
		observed = absInt(2*positive - total)
		test.Statistic = diversity.Number(float64(positive) / 2)
		test.Nonzero = len(ranks)
		assignments = 1
		for range ranks {
			assignments *= 2
			if assignments > ExactLimit {
				break
			}
		}
	} else {
		values := append(append([]float64(nil), a...), b...)
		ranks = doubledRanks(values)
		sum := 0
		for _, r := range ranks[len(a):] {
			sum += r
		}
		u2 := sum - len(b)*(len(b)+1)
		test.Statistic = diversity.Number(float64(u2) / 2)
		probability = float64(u2) / float64(2*len(a)*len(b))
		center = len(b) * (len(ranks) + 1)
		observed = absInt(sum - center)
	}
	effect := probabilityInterval(probability, scale, o.Confidence, name)
	draws := assignments
	test.NullMethod = "Exact conditional randomization"
	if draws > ExactLimit {
		draws = o.Permutations
		test.NullMethod = "Monte Carlo conditional randomization"
	}
	if draws*len(ranks) > *budget {
		return test, effect, errors.New("rank randomization exceeds 50000000 assignment-size units; select fewer features or fewer permutations")
	}
	*budget -= draws * len(ranks)
	tail, count := 0, 0
	visit := func(score int) {
		count++
		if score >= observed {
			tail++
		}
	}
	if assignments <= ExactLimit {
		if paired {
			total := 0
			for _, r := range ranks {
				total += r
			}
			for bits := 0; bits < assignments; bits++ {
				if bits%256 == 0 {
					if err := ctx.Err(); err != nil {
						return test, effect, err
					}
				}
				sum := 0
				for i, r := range ranks {
					if bits&(1<<i) != 0 {
						sum += r
					}
				}
				visit(absInt(2*sum - total))
			}
		} else {
			var walk func(int, int, int) error
			walk = func(start, left, sum int) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if left == 0 {
					visit(absInt(sum - center))
					return nil
				}
				for i := start; i <= len(ranks)-left; i++ {
					if err := walk(i+1, left-1, sum+ranks[i]); err != nil {
						return err
					}
				}
				return nil
			}
			if err := walk(0, nChoose, 0); err != nil {
				return test, effect, err
			}
		}
		test.P = diversity.Number(float64(tail) / float64(count))
	} else {
		hash := fnv.New64a()
		hash.Write([]byte(id))
		rng := random.New(o.Seed, hash.Sum64())
		shuffled := append([]int(nil), ranks...)
		for draw := 0; draw < draws; draw++ {
			if draw%64 == 0 {
				if err := ctx.Err(); err != nil {
					return test, effect, err
				}
			}
			if paired {
				sum := 0
				for _, r := range ranks {
					if rng.IntN(2) == 0 {
						sum += r
					} else {
						sum -= r
					}
				}
				visit(absInt(sum))
			} else {
				for i := len(shuffled) - 1; i > 0; i-- {
					j := rng.IntN(i + 1)
					shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
				}
				sum := 0
				for _, r := range shuffled[:nChoose] {
					sum += r
				}
				visit(absInt(sum - center))
			}
		}
		test.P = diversity.Number(float64(tail+1) / float64(count+1))
	}
	test.Draws = count
	test.Tail = tail
	return test, effect, nil
}

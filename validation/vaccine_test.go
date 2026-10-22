// The simulated vaccine study's receptors that recognize an epitope (internal/simulate,
// response.go) against Dash et al. 2017's epitope-specific TCRs: each one's nearest other receptor
// of the same epitope, by TCRdist with the V gene's loops on the β chain (identical β chains left
// out), as near as in Dash et al.'s human and mouse data. (Their shares in each sample against the
// truth are checked with the other series, series_test.go.)
package validation

import (
	"bufio"
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"clonoloom/internal/simulate"
	"clonoloom/internal/tcrdist"
)

// betaChain is a receptor's β chain with its epitope.
type betaChain struct {
	epitope, v, cdr3 string
	chain            tcrdist.Chain
}

// nearestShares answers, of the receptors with another of their epitope, the shares whose nearest
// is within each radius, and the median distance to it.
func nearestShares(receptors []betaChain, radii []int) ([]float64, int) {
	byEpitope := map[string][]betaChain{}
	for _, r := range receptors {
		byEpitope[r.epitope] = append(byEpitope[r.epitope], r)
	}
	var nearest []int
	for _, list := range byEpitope {
		for i := range list {
			best := -1
			for j := range list {
				if i == j || (list[i].v == list[j].v && list[i].cdr3 == list[j].cdr3) {
					continue
				}
				if d := tcrdist.ChainDistance(list[i].chain, list[j].chain); best < 0 || d < best {
					best = d
				}
			}
			if best >= 0 {
				nearest = append(nearest, best)
			}
		}
	}
	sort.Ints(nearest)
	out := make([]float64, len(radii))
	for k, r := range radii {
		out[k] = float64(sort.SearchInts(nearest, r+1)) / float64(len(nearest))
	}
	return out, nearest[len(nearest)/2]
}

func TestVaccineReceptorsAsNearAsDash(t *testing.T) {
	loops := loopsFile(t)
	radii := []int{12, 24, 48}
	// Dash et al.'s β chains, by epitope.
	var dash [][]float64
	for _, name := range []string{"dash_human.csv", "dash.csv"} {
		organism := map[string]string{"dash_human.csv": "human", "dash.csv": "mouse"}[name]
		f, err := os.Open(filepath.Join(dataRoot(), "dash-2017-tcrdist3", name))
		if err != nil {
			if required("dash-2017-tcrdist3") {
				t.Fatalf("dash-2017-tcrdist3 is required and missing")
			}
			t.Skip("dash-2017-tcrdist3 not fetched")
		}
		rows, err := csv.NewReader(f).ReadAll()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		col := map[string]int{}
		for i, h := range rows[0] {
			col[h] = i
		}
		var receptors []betaChain
		for _, r := range rows[1:] {
			if l, _, ok := loops.Lookup(organism, r[col["v_b_gene"]]); ok {
				receptors = append(receptors, betaChain{r[col["epitope"]], r[col["v_b_gene"]], r[col["cdr3_b_aa"]], tcrdist.NewChain(r[col["cdr3_b_aa"]], l[:])})
			}
		}
		shares, median := nearestShares(receptors, radii)
		t.Logf("Dash et al.'s %s β chains (%d): nearest of the same epitope within 12, 24 and 48: %.2f, %.2f, %.2f; median %d", organism, len(receptors), shares[0], shares[1], shares[2], median)
		dash = append(dash, shares)
	}
	// The vaccine study's, as its truth lists them.
	dir := filepath.Join(t.TempDir(), "vaccine")
	if _, err := simulate.Run(context.Background(), simulate.Presets["vaccine"], dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "truth", "specificity.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan()
	col := map[string]int{}
	for i, h := range strings.Split(sc.Text(), "\t") {
		col[h] = i
	}
	var receptors []betaChain
	for sc.Scan() {
		r := strings.Split(sc.Text(), "\t")
		l, _, ok := loops.Lookup("human", r[col["v_call"]])
		if !ok {
			t.Fatalf("no loops for %s", r[col["v_call"]])
		}
		receptors = append(receptors, betaChain{r[col["epitope"]], r[col["v_call"]], r[col["junction_aa"]], tcrdist.NewChain(r[col["junction_aa"]], l[:])})
	}
	shares, median := nearestShares(receptors, radii)
	t.Logf("the vaccine study's (%d): within 12, 24 and 48: %.2f, %.2f, %.2f; median %d", len(receptors), shares[0], shares[1], shares[2], median)
	// Within 0.1 of the range Dash et al.'s two sets span, at each radius.
	for k, r := range radii {
		lo, hi := min(dash[0][k], dash[1][k])-0.1, max(dash[0][k], dash[1][k])+0.1
		if shares[k] < lo || shares[k] > hi {
			t.Errorf("within %d: %.2f, Dash et al.'s %.2f and %.2f", r, shares[k], dash[0][k], dash[1][k])
		}
	}
}

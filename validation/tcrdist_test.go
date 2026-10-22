// TCRdist (internal/tcrdist) against tcrdist3 on Dash et al. 2017's epitope-specific TCRs, as
// tcrdist3 distributes them (dash.csv, 1,924 mouse clones; dash_human.csv, 415 human): every
// pairwise distance, α, β and paired, and each loop's alone (the CDR3's before its weight, as
// tcrdist3 keeps it), equal to tcrdist3's, with the loops both take from tcrdist3's database of
// genes (validation/reference/tcrdist3.json, made by tcrdist3_distances.py).
package validation

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/reads"
	"clonoloom/internal/tcrdist"
)

type tcrdistMatrix struct {
	Digest string `json:"digest"`
	First  []int  `json:"first"`
}

type tcrdistReference struct {
	Made  string `json:"made"`
	Loops struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
	} `json:"loops"`
	Files map[string]map[string]json.RawMessage `json:"files"`
}

// loopsFile is tcrdist3's database of genes as fetch.mjs fetches it (the reference data
// tcrdist3-loops).
func loopsFile(t *testing.T) *tcrdist.Loops {
	t.Helper()
	path := filepath.Join(dataRoot(), "tcrdist3-loops", "tcrdist", "db", "combo_xcr_2024-03-05.tsv")
	raw, err := os.ReadFile(path)
	if err != nil {
		if required("tcrdist3-loops") {
			t.Fatalf("tcrdist3-loops is required and missing (node validation/fetch.mjs tcrdist3-loops)")
		}
		t.Skip("tcrdist3-loops not fetched")
	}
	loops, err := tcrdist.ParseLoops(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	return loops
}

func TestTCRdistEqualsTCRdist3(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "tcrdist3.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref tcrdistReference
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	loops := loopsFile(t)
	db, _ := os.ReadFile(filepath.Join(dataRoot(), "tcrdist3-loops", "tcrdist", "db", ref.Loops.File))
	if sum := sha256.Sum256(db); hex.EncodeToString(sum[:]) != ref.Loops.SHA256 {
		t.Fatalf("the loops' database is not the one tcrdist3 used (%s)", ref.Loops.SHA256)
	}
	for name, entry := range ref.Files {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dataRoot(), "dash-2017-tcrdist3", name)
			f, err := os.Open(path)
			if err != nil {
				if required("dash-2017-tcrdist3") {
					t.Fatalf("dash-2017-tcrdist3 is required and missing")
				}
				t.Skip("dash-2017-tcrdist3 not fetched")
			}
			defer f.Close()
			rows, err := csv.NewReader(f).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			col := map[string]int{}
			for i, h := range rows[0] {
				col[h] = i
			}
			var organism string
			json.Unmarshal(entry["organism"], &organism)
			var clones int
			json.Unmarshal(entry["clones"], &clones)
			rows = rows[1:]
			if len(rows) != clones {
				t.Fatalf("%d clones, tcrdist3 %d", len(rows), clones)
			}
			// Each clone's chains, with their V alleles' loops (the allele as given: tcrdist3 takes
			// no other).
			alpha := make([]tcrdist.Chain, len(rows))
			beta := make([]tcrdist.Chain, len(rows))
			var inferred []string
			for i, r := range rows {
				var line []string
				for k, chain := range []string{"a", "b"} {
					v := r[col["v_"+chain+"_gene"]]
					l, allele, ok := loops.Lookup(organism, v)
					if !ok || allele != v {
						t.Fatalf("row %d: %s is not in the loops' database as given", i+2, v)
					}
					c := tcrdist.NewChain(r[col["cdr3_"+chain+"_aa"]], l[:])
					if k == 0 {
						alpha[i] = c
					} else {
						beta[i] = c
					}
					line = append(line, v, l[0], l[1], l[2])
				}
				inferred = append(inferred, strings.Join(line, "\t"))
			}
			var loopsDigest string
			json.Unmarshal(entry["loops"], &loopsDigest)
			if digestRows(inferred) != loopsDigest {
				t.Errorf("the loops differ from those tcrdist3 inferred")
			}
			// Every pair's distances, row by row.
			names := []string{"cdr1_a", "cdr2_a", "pmhc_a", "cdr3_a", "cdr1_b", "cdr2_b", "pmhc_b", "cdr3_b", "alpha", "beta", "paired"}
			matrices := map[string][]string{}
			first := map[string][]int{}
			n := len(rows)
			paired := make([]int, n*n)
			row := make(map[string][]string, len(names))
			for i := 0; i < n; i++ {
				for _, m := range names {
					row[m] = row[m][:0]
				}
				for j := 0; j < n; j++ {
					ra := tcrdist.Regions(alpha[i], alpha[j])
					rb := tcrdist.Regions(beta[i], beta[j])
					a, b := ra[0]+ra[1]+ra[2]+ra[3], rb[0]+rb[1]+rb[2]+rb[3]
					// tcrdist3 keeps each loop's matrix before its weight: the CDR3's is a third.
					values := []int{ra[0], ra[1], ra[2], ra[3] / 3, rb[0], rb[1], rb[2], rb[3] / 3, a, b, a + b}
					paired[i*n+j] = a + b
					for k, m := range names {
						row[m] = append(row[m], strconv.Itoa(values[k]))
						if i == 0 {
							first[m] = append(first[m], values[k])
						}
					}
				}
				for _, m := range names {
					matrices[m] = append(matrices[m], strings.Join(row[m], ","))
				}
			}
			for _, m := range names {
				var want tcrdistMatrix
				if err := json.Unmarshal(entry[m], &want); err != nil {
					t.Fatalf("%s: %v", m, err)
				}
				if digestRows(matrices[m]) == want.Digest {
					continue
				}
				detail := "the first row agrees"
				for j := range want.First {
					if first[m][j] != want.First[j] {
						detail = fmt.Sprintf("first row, column %d: %d, tcrdist3 %d", j, first[m][j], want.First[j])
						break
					}
				}
				t.Errorf("%s: the matrix differs from tcrdist3's (%s)", m, detail)
			}
			t.Logf("%s: %d clones, %d pairs of distances equal to %s", name, n, n*n, ref.Made)

			// The neighborhoods found by Set.Neighbors (pruned, in parallel) are those of every
			// distance: each clone's neighbors within the radius and its nearest other clone.
			var receptors []clonotype.Clonotype
			for _, r := range rows {
				receptors = append(receptors, clonotype.Clonotype{Chains: []clonotype.Chain{
					{Locus: "TRA", Junction: r[col["cdr3_a_aa"]], V: r[col["v_a_gene"]]},
					{Locus: "TRB", Junction: r[col["cdr3_b_aa"]], V: r[col["v_b_gene"]]}}})
			}
			set := tcrdist.NewSet(receptors, loops, "")
			if set.Organism != organism || set.Len() != n || len(set.Groups) != 1 {
				t.Fatalf("set: organism %s, %d receptors, %d groups", set.Organism, set.Len(), len(set.Groups))
			}
			for _, radius := range []int{0, 24, 48, 96} {
				got, err := set.Neighbors(context.Background(), radius, 0, nil)
				if err != nil {
					t.Fatal(err)
				}
				wrong := 0
				for i := 0; i < n; i++ {
					want := tcrdist.Neighborhood{Nearest: -1, Distance: -1}
					for j := 0; j < n; j++ {
						if j == i {
							continue
						}
						d := paired[i*n+j]
						if d <= radius {
							want.Neighbors++
						}
						if want.Nearest < 0 || d < want.Distance {
							want.Nearest, want.Distance = j, d
						}
					}
					if got[set.Receptor(i)] != want {
						wrong++
					}
				}
				if wrong > 0 {
					t.Errorf("radius %d: %d of %d neighborhoods differ from every distance's", radius, wrong, n)
				}
				for _, i := range []int{0, n / 2, n - 1} {
					nearest := set.Nearest(i, 5)
					if len(nearest) != 5 || nearest[0].Distance != got[i].Distance || nearest[0].Receptor != got[i].Nearest {
						t.Errorf("clone %d: nearest %v, neighborhood %+v", i, nearest, got[i])
					}
				}
			}
		})
	}
}

func digestRows(rows []string) string {
	h := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(h[:])
}

// Budget (clonoloom-spec/design.md, "Budgets"): the neighbors of 100,000 paired TCRs found within 2
// minutes on 10 cores (scaled to the processors here) and within 8 GB. The receptors are real
// chains paired at random: the α and β chains (with their V genes) of the three OTS donors' cells,
// drawn with a fixed seed, so that their CDR3s' lengths and V genes are a repertoire's.
func TestTCRdistNeighborsWithinBudget(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("budgets are measured without -short and without the race detector")
	}
	loops := loopsFile(t)
	var paths []string
	for _, unit := range []string{"Sureshchandra_2023/csv_paired/SRR18365371_1_Paired_All.csv.gz", "Bai_2023/csv_paired/SRR14045264_1_Paired_All.csv.gz", "Poon_2023/csv_paired/SRR19760479_1_Paired_All.csv.gz"} {
		path := filepath.Join(dataRoot(), "ots-healthy-pbmc", filepath.FromSlash(unit))
		if _, err := os.Stat(path); err != nil {
			if required("ots-healthy-pbmc") {
				t.Fatalf("ots-healthy-pbmc is required and missing")
			}
			t.Skip("ots-healthy-pbmc not fetched")
		}
		paths = append(paths, path)
	}
	res, err := reads.ReadFiles(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Chains: []string{"TRA", "TRB"}, Junction: "aa", V: "allele", J: "gene", ProductiveOnly: true, RequireAll: true})
	if err != nil {
		t.Fatal(err)
	}
	var alpha, beta []clonotype.Chain
	for _, c := range table.Clonotypes {
		alpha, beta = append(alpha, c.Chains[0]), append(beta, c.Chains[1])
	}
	r := rand.New(rand.NewPCG(20261022, 4))
	const n = 100_000
	receptors := make([]clonotype.Clonotype, n)
	for i := range receptors {
		receptors[i] = clonotype.Clonotype{Chains: []clonotype.Chain{alpha[r.IntN(len(alpha))], beta[r.IntN(len(beta))]}}
	}
	start := time.Now()
	set := tcrdist.NewSet(receptors, loops, "")
	hoods, err := set.Neighbors(context.Background(), 48, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	with := 0
	for _, h := range hoods {
		if h.Neighbors > 0 {
			with++
		}
	}
	procs := runtime.GOMAXPROCS(0)
	scaled := time.Duration(float64(elapsed) * float64(procs) / 10)
	t.Logf("the neighbors within 48 of %d paired TCRs (%s, from %d α and %d β chains) in %v on %d processors (%v on 10); %d have one; %.1f GB from the system",
		set.Len(), set.Organism, len(alpha), len(beta), elapsed.Round(time.Millisecond), procs, scaled.Round(time.Millisecond), with, float64(mem.Sys)/1e9)
	if set.Len() != n {
		t.Errorf("%d receptors compared of %d", set.Len(), n)
	}
	if scaled > 2*time.Minute {
		t.Errorf("took %v on %d processors, %v on 10: budget 2 minutes", elapsed, procs, scaled)
	}
	if mem.Sys > 8<<30 {
		t.Errorf("%.1f GB from the system, budget 8", float64(mem.Sys)/1e9)
	}
}

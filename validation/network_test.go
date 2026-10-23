package validation

// Compare complete cell partitions (not just cluster counts) with black-box
// scirpy 0.25.1 on 10x runs and OTS paired units. Reference generation is in
// reference/scirpy_networks.py. No external code is linked into ClonoLoom.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/network"
	"clonoloom/internal/reads"
)

func TestNetworksEqualScirpy(t *testing.T) {
	raw, err := os.ReadFile("reference/networks.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string `json:"generated_by"`
		Files       map[string]map[string]struct {
			Cells, Components int
			SHA256            string
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	partitions := 0
	for _, cases := range ref.Files {
		partitions += len(cases)
	}
	if len(ref.Files) != 11 || partitions != 420 || ref.GeneratedBy != "scirpy 0.25.1" {
		t.Fatalf("incomplete reference: %d datasets, %d partitions, %s", len(ref.Files), partitions, ref.GeneratedBy)
	}
	compared := 0
	for rel, cases := range ref.Files {
		path := filepath.Join(dataRoot(), filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			if required(strings.Split(rel, "/")[0]) {
				t.Error(rel, err)
			}
			continue
		}
		res, err := reads.ReadFiles(context.Background(), []string{path})
		if err != nil {
			t.Fatal(err)
		}
		chains := []string{"TRA", "TRB"}
		if strings.Contains(rel, "_b_") {
			chains = []string{"IGH", clonotype.Light}
		}
		table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Chains: chains, Junction: "aa", V: "gene", J: "gene", Cells: "primary", ProductiveOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(cases))
		for name := range cases {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			want := cases[name]
			parts := strings.Split(name, "/")
			var cutoff, sameV int
			fmt.Sscan(parts[1], &cutoff)
			fmt.Sscan(parts[3], &sameV)
			g, err := network.Build(context.Background(), table, network.Options{Metric: parts[0], Cutoff: cutoff, Arms: parts[2], SameV: sameV == 1}, nil, "", nil, nil)
			if err != nil {
				t.Fatalf("%s %s: %v", rel, name, err)
			}
			lines := []string{}
			cells := 0
			for _, c := range g.Components {
				seen := map[string]bool{}
				for _, n := range c.Nodes {
					for _, row := range table.MembersOf(n) {
						seen[res.Table.Dict.Get(res.Table.Cell[row])] = true
					}
				}
				bars := make([]string, 0, len(seen))
				for b := range seen {
					bars = append(bars, b)
				}
				sort.Strings(bars)
				cells += len(bars)
				lines = append(lines, strings.Join(bars, ","))
			}
			sort.Strings(lines)
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(lines, "\n"))))
			if cells != want.Cells || len(lines) != want.Components || digest != want.SHA256 {
				t.Errorf("%s %s: %d cells, %d components; reference %d cells, %d components; partition equal %t", rel, name, cells, len(lines), want.Cells, want.Components, digest == want.SHA256)
			}
			compared++
		}
	}
	if compared == 0 {
		t.Skip("no development datasets available")
	}
	t.Logf("%d complete partitions compared with %s", compared, ref.GeneratedBy)
}

// Check observed family recovery in the simulated day-14 repertoire, including
// its unrelated background. The simulator draws a Poisson number of substitutions,
// so a cutoff of two edits need not connect every family. TCRdist at 48 connects
// all observed families; private receptors are not part of this check.
func TestNetworksConnectSimulatedFamilies(t *testing.T) {
	path, ok := referencePath(t, "simulated-vaccine/S01-d14.tsv")
	if !ok {
		return
	}
	raw, err := os.ReadFile(filepath.Join(dataRoot(), "simulated-vaccine/truth/specificity.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	col := map[string]int{}
	for i, h := range strings.Split(lines[0], "\t") {
		col[h] = i
	}
	family := map[string]string{}
	for _, line := range lines[1:] {
		r := strings.Split(line, "\t")
		if r[col["subject"]] == "S01" && r[col["private"]] == "F" {
			family[r[col["v_call"]]+"|"+r[col["j_call"]]+"|"+r[col["junction_aa"]]] = r[col["founder"]]
		}
	}
	res, err := reads.ReadFiles(context.Background(), []string{path})
	if err != nil {
		t.Fatal(err)
	}
	table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Chains: []string{"TRB"}, Junction: "aa", V: "gene", J: "gene", ProductiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, metric := range []string{"hamming", "levenshtein", "tcrdist"} {
		cutoff := 2
		if metric == "tcrdist" {
			cutoff = 48
		}
		g, err := network.Build(context.Background(), table, network.Options{Metric: metric, Cutoff: cutoff, SameV: true, SameJ: true}, nil, "", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]int{}
		split := map[string]bool{}
		members := 0
		for n, c := range table.Clonotypes {
			ch := c.Chains[0]
			f := family[ch.V+"|"+ch.J+"|"+ch.Junction]
			if f == "" {
				continue
			}
			members++
			if id, ok := found[f]; ok && id != g.Membership[n] {
				split[f] = true
			}
			found[f] = g.Membership[n]
		}
		if len(found) != 34 || members != 97 {
			t.Fatalf("not enough truth: %d families, %d members", len(found), members)
		}
		wantSplit := 5
		if metric == "tcrdist" {
			wantSplit = 0
		}
		if len(split) != wantSplit {
			t.Errorf("%s: %d split families, want %d", metric, len(split), wantSplit)
		}
		t.Logf("%s cutoff %d: %d observed members, %d of %d families connected", metric, cutoff, members, len(found)-len(split), len(found))
	}
}

func TestNetworkDrawingPacksComponents(t *testing.T) {
	path, ok := referencePath(t, "simulated-vaccine/S01-d14.tsv")
	if !ok {
		return
	}
	res, err := reads.ReadFiles(context.Background(), []string{path})
	if err != nil {
		t.Fatal(err)
	}
	table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Junction: "aa", V: "gene", J: "gene", ProductiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	g, err := network.Build(context.Background(), table, network.Options{Metric: "levenshtein", Cutoff: 2, SameV: true, SameJ: true}, nil, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	d, err := network.Layout(context.Background(), g, 1000)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("drawing: %g by %g; %d points", d.Width, d.Height, len(d.Points))
	for _, p := range d.Points {
		if p.X < 0 || p.Y < 0 || p.X > d.Width || p.Y > d.Height {
			t.Fatal("point outside drawing", p)
		}
	}
	if d.Width/d.Height < 0.5 || d.Width/d.Height > 3 {
		t.Fatalf("poorly packed drawing: %g by %g", d.Width, d.Height)
	}
}

func TestNetworkTCRdistUsesVLoops(t *testing.T) {
	loops := loopsFile(t)
	a := clonotype.Chain{Locus: "TRB", Junction: "CASSLGGNTEAFF", V: "TRBV5-1*01"}
	b := a
	b.V = "TRBV6-5*01"
	missing := a
	missing.V = "TRBV999*01"
	table := &clonotype.Table{Clonotypes: []clonotype.Clonotype{{ID: "a", Chains: []clonotype.Chain{a}}, {ID: "b", Chains: []clonotype.Chain{b}}, {ID: "missing", Chains: []clonotype.Chain{missing}}}}
	o := network.Options{Metric: "tcrdist", Cutoff: 0}
	cdr3, err := network.Build(context.Background(), table, o, nil, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	full, err := network.Build(context.Background(), table, o, loops, "human", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cdr3.Edges) != 3 || len(full.Edges) != 0 || len(full.Membership) != 3 || full.Excluded["a V gene the loops' database lacks"] != 1 {
		t.Fatalf("CDR3 edges %v; with loops %+v", cdr3.Edges, full)
	}
}

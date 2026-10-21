package validation

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/reads"
	"clonoloom/internal/validate"
)

// Budget (clonoloom-spec/design.md, "Budgets"): a 1M-rearrangement immunoSEQ sample opened and
// validated within 5 s, and its table within 64 bytes a row plus its junctions. The file is made
// here (a seeded mix of junctions, genes and counts in the v3 layout, about 150 MB) and removed.
func TestOpenAMillionRearrangementsWithinBudget(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("budgets are measured without -short and without the race detector")
	}
	const rows = 1_000_000
	path := writeMillion(t, rows)
	start := time.Now()
	res, err := reads.ReadFiles(context.Background(), []string{path})
	if err != nil {
		t.Fatal(err)
	}
	report := validate.Check(res)
	elapsed := time.Since(start)
	tab := res.Table
	if tab.Len() != rows || !report.Usable() {
		t.Fatalf("%d rows, %d fatal findings", tab.Len(), report.Fatal)
	}
	junctions := tab.Junctions.Size() + tab.JunctionsAA.Size()
	perRow := float64(tab.Size()-junctions) / rows
	t.Logf("read and validated %d rows in %v; %.1f bytes a row plus %.1f MB of junctions", rows, elapsed.Round(time.Millisecond), perRow, float64(junctions)/1e6)
	if elapsed > 5*time.Second {
		t.Errorf("took %v, budget 5 s", elapsed)
	}
	if perRow > 64 {
		t.Errorf("%.1f bytes a row, budget 64", perRow)
	}
}

// writeMillion writes an immunoSEQ v3 file of n rearrangements: a seeded mix of junctions, genes
// and counts, about 150 bytes a row, in the test's temporary folder.
func writeMillion(t *testing.T, rows int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "million.tsv")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriterSize(f, 1<<20)
	fmt.Fprintln(w, "sample_name\tspecies\tlocus\trearrangement\tamino_acid\tframe_type\ttemplates\tseq_reads\tcdr3_length\tv_index\tv_resolved\tv_gene\tv_gene_ties\td_resolved\td_gene\td_gene_ties\tj_resolved\tj_gene\tj_gene_ties")
	codons := []string{"GCC", "AGC", "TTA", "GGG", "CCT", "GAA", "TAC", "CAG", "ACC", "GAT"}
	amino := "ASLGPEYQTD"
	vs := []string{"TCRBV05-01*01", "TCRBV06-05*01", "TCRBV07-09*01", "TCRBV20", "TCRBV28-01*01", "TCRBV09-01*01"}
	js := []string{"TCRBJ02-07*01", "TCRBJ01-01*01", "TCRBJ02-01*01", "TCRBJ01-02*01"}
	x := uint64(88172645463325252)
	next := func() uint64 {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		return x
	}
	for i := 0; i < rows; i++ {
		n := 6 + int(next()%8)
		nt, aa := make([]byte, 0, 60), make([]byte, 0, 20)
		nt, aa = append(nt, "TGTGCCAGCAGC"...), append(aa, "CASS"...)
		for k := 0; k < n; k++ {
			c := next() % 10
			nt, aa = append(nt, codons[c]...), append(aa, amino[c])
		}
		nt, aa = append(nt, "TTCTTT"...), append(aa, "FF"...)
		read := "CTGAATGTGAACGCCTTGTTGCTGGGG" + string(nt) + "GGAGAA"
		v := vs[next()%uint64(len(vs))]
		gene, ties := v, ""
		if v == "TCRBV20" {
			gene, ties = "", "TCRBV20-01,TCRBV20-or09_02"
		}
		fmt.Fprintf(w, "S1\tHuman\tTCRB\t%s\t%s\tIn\t%d\t%d\t%d\t27\t%s\t%s\t%s\tTCRBD01-01*01\tTCRBD01-01\t\t%s\t%s\t\n",
			read, aa, 1+next()%50, 100+next()%5000, len(nt), v, gene, ties, js[next()%uint64(len(js))], js[0][:10])
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

// Budget: a sorted, searched page of a clonotype table of about a million clonotypes within
// 100 ms once the table is made (sorting a new order is made once, and kept).
func TestAPageOfAMillionClonotypesWithinBudget(t *testing.T) {
	if testing.Short() || raceDetector {
		t.Skip("budgets are measured without -short and without the race detector")
	}
	res, err := reads.ReadFiles(context.Background(), []string{writeMillion(t, 1_000_000)})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Junction: "aa", V: "gene", J: "gene"})
	if err != nil {
		t.Fatal(err)
	}
	made := time.Since(start)
	n := len(table.Clonotypes)
	// The page: sorted by total (descending), searched for a junction motif, 200 rows.
	order := make([]int32, n)
	for i := range order {
		order[i] = int32(i)
	}
	sortStart := time.Now()
	sort.SliceStable(order, func(a, b int) bool { return table.Clonotypes[order[a]].Total > table.Clonotypes[order[b]].Total })
	sorted := time.Since(sortStart)
	pageStart := time.Now()
	var page []int32
	for _, i := range order {
		c := table.Clonotypes[i]
		hit := false
		for _, ch := range c.Chains {
			hit = hit || strings.Contains(ch.Junction, "GPEY") || strings.Contains(ch.V, "GPEY")
		}
		if hit {
			page = append(page, i)
			if len(page) == 200 {
				break
			}
		}
	}
	paged := time.Since(pageStart)
	t.Logf("%d clonotypes made in %v; sorted once in %v; a searched page of %d in %v", n, made.Round(time.Millisecond), sorted.Round(time.Millisecond), len(page), paged.Round(time.Microsecond))
	if n < 500_000 {
		t.Fatalf("only %d clonotypes", n)
	}
	if paged > 100*time.Millisecond {
		t.Errorf("a page took %v, budget 100 ms", paged)
	}
}

package validation

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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

package reads

import (
	"context"
	"strings"
	"testing"
)

// Every reader, given any bytes, answers a table or an error: never a panic, never a table whose
// columns disagree in length. The seeds are each format's header with a row or two, and the
// hostile cases found by fuzzing (testdata/fuzz/FuzzRead).
func FuzzRead(f *testing.F) {
	f.Add([]byte("sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tduplicate_count\na\tTRBV5-1*01\tTRBJ2-7\tCASSF\tT\t3\n"))
	f.Add([]byte("sequence_id\tjunction\tcell_id\tumi_count\nb\tTGTGCC\tAAAC-1\t2.0\n"))
	f.Add([]byte(contigHeader + "AAAC-1,True,AAAC-1_contig_1,True,500,TRB,TRBV5-1,None,TRBJ2-7,TRBC2,True,True,CASSF,TGT,9,1,clonotype1,None\n"))
	f.Add([]byte("sample,barcode,contig_id,chain,cdr3,umis\ns,\"A,B\",c,TRA,CAVF,1\n"))
	f.Add([]byte("rearrangement\tamino_acid\tframe_type\ttemplates\tseq_reads\tcdr3_length\tv_index\tv_resolved\tv_gene\tv_gene_ties\nACGTACGT\tCASSF\tIn\t-1\t20\t99\t-3\tTCRBV12-03/12-04\t\tTCRBV20-or09_02\n"))
	f.Add([]byte("nucleotide\taminoAcid\tcount (templates/reads)\tvMaxResolved\tvIndex\tcdr3Length\tsequenceStatus\nACGT\t\t4\tTCRBV09-01*1\t0\t4\tOut\n"))
	f.Add([]byte("\x1f\x8b\x08\x00"))
	f.Add([]byte("clonotype_id,frequency,proportion,cdr3s_aa\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		res, err := ReadReader(context.Background(), strings.NewReader(string(data)), "fuzz.tsv")
		if err != nil {
			return
		}
		tab := res.Table
		n := tab.Len()
		for name, length := range map[string]int{"locus": len(tab.Locus), "v": len(tab.V), "junction": len(tab.Junction), "junction_aa": len(tab.JunctionAA), "count": len(tab.Count), "productive": len(tab.Productive), "line": len(tab.Line), "id": len(tab.SequenceID)} {
			if length != n {
				t.Fatalf("column %s has %d rows, the table %d", name, length, n)
			}
		}
		for name, length := range map[string]int{"reads": len(tab.Reads), "umis": len(tab.UMIs), "cell": len(tab.Cell), "clone": len(tab.Clone)} {
			if length != 0 && length != n {
				t.Fatalf("optional column %s has %d rows, the table %d", name, length, n)
			}
		}
		for i := 0; i < n; i++ {
			if int(tab.Sample[i]) >= len(tab.Samples) {
				t.Fatalf("row %d names sample %d of %d", i, tab.Sample[i], len(tab.Samples))
			}
			tab.Call(tab.V[i])
		}
	})
}

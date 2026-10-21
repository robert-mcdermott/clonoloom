package reads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const contigHeader = "barcode,is_cell,contig_id,high_confidence,length,chain,v_gene,d_gene,j_gene,c_gene,full_length,productive,cdr3,cdr3_nt,reads,umis,raw_clonotype_id,raw_consensus_id\n"

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenACellRangerMultiRun(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "run1")
	contig := "AAAC-1,true,AAAC-1_contig_1,true,500,TRB,TRBV5-1,,TRBJ2-7,TRBC2,true,true,CASSLAGGPEAFF,TGTGCCAGCAGCTTAGCGGGAGGGCCTGAAGCTTTCTTT,900,9,clonotype1,clonotype1_consensus_1\n"
	write(t, filepath.Join(run, "outs/per_sample_outs/Donor1/vdj_t/filtered_contig_annotations.csv"), contigHeader+contig)
	write(t, filepath.Join(run, "outs/per_sample_outs/Donor1/vdj_t/clonotypes.csv"), "clonotype_id,frequency,proportion,cdr3s_aa,cdr3s_nt\nclonotype1,1,1,TRB:CASSLAGGPEAFF,TRB:TGT\n")
	write(t, filepath.Join(run, "outs/per_sample_outs/Donor1/metrics_summary.csv"), "Category,Library Type,Grouped By,Group Name,Metric Name,Metric Value\n")
	write(t, filepath.Join(run, "outs/per_sample_outs/Donor2/vdj_t/filtered_contig_annotations.csv"), contigHeader+contig)
	res, err := Open(context.Background(), []string{run})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sources) != 2 || res.Table.Len() != 2 || len(res.Table.Samples) != 2 || res.Table.Samples[0].Name != "Donor1" || res.Table.Samples[1].Name != "Donor2" {
		t.Fatalf("%+v %+v", res.Sources, res.Table.Samples)
	}
	if len(res.Companions) != 2 || res.Companions[0].Kind != CompanionClonotypes || res.Companions[1].Kind != CompanionMetrics || res.Companions[0].Sample != "Donor1" {
		t.Fatalf("%+v", res.Companions)
	}
	// A companion named alone says what to open instead.
	_, err = Open(context.Background(), []string{filepath.Join(run, "outs/per_sample_outs/Donor1/vdj_t/clonotypes.csv")})
	var companion ErrCompanion
	if !errors.As(err, &companion) || companion.Kind != CompanionClonotypes {
		t.Fatalf("got %v", err)
	}
	if _, err := Open(context.Background(), []string{dir}); err == nil {
		t.Fatal("a folder that is not a Cell Ranger output was opened")
	}
}

func TestOpenMatchesRepertoireMetadata(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "rearrangements.tsv"), "sequence_id\tv_call\tj_call\tjunction_aa\trepertoire_id\nx\tTRBV5-1\tTRBJ2-7\tCASSLAGGPEAFF\tR1\n")
	write(t, filepath.Join(dir, "repertoires.airr.json"), `{"Repertoire": [{"repertoire_id": "R1", "subject": {"subject_id": "S01"}, "sample": [{"sample_id": "a", "tissue": {"label": "blood"}}]}]}`)
	res, err := Open(context.Background(), []string{filepath.Join(dir, "rearrangements.tsv"), filepath.Join(dir, "repertoires.airr.json")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Repertoires) != 1 || res.Repertoires[0].SubjectID != "S01" || res.Table.Samples[0].RepertoireID != "R1" {
		t.Fatalf("%+v %+v", res.Repertoires, res.Table.Samples)
	}
}

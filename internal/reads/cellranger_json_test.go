package reads

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

const jsonContigSeed = `[{"barcode":"AAAC-1","contig_name":"AAAC-1_contig_1","sequence":"TGTTCTTTT","annotations":[{"feature":{"chain":"TRB","region_type":"J-REGION","gene_name":"TRBJ2-5"}}],"cdr3":"CSF","cdr3_seq":"TGTTCTTTT","read_count":12,"umi_count":2,"is_cell":true,"productive":null,"info":{"raw_clonotype_id":"clone1"},"extra":9007199254740993}]`

func TestCellRangerJSONSemantics(t *testing.T) {
	r, err := ReadReader(context.Background(), strings.NewReader(jsonContigSeed), "sample_all_contig_annotations.json")
	if err != nil {
		t.Fatal(err)
	}
	tab := r.Table
	if r.Unit != rep.Cells || tab.Len() != 1 || tab.Samples[0].Name != "sample" || tab.Count[0] != 2 || tab.Reads[0] != 12 || tab.UMIs[0] != 2 {
		t.Fatal("counts/scope", r)
	}
	if tab.Productive[0] != rep.Missing || tab.StopCodon[0] != rep.Missing || tab.VJInFrame[0] != rep.Missing || len(tab.FullLength) != 0 {
		t.Fatal("invented flags")
	}
	if r.Sources[0].Native.Rows[0].Text != jsonContigSeed[1:len(jsonContigSeed)-1] {
		t.Fatal("source changed")
	}
	if tab.EvidenceValue(0, 0) != "TGTTCTTTT" || tab.Dict.Get(tab.Cell[0]) != "AAAC-1" {
		t.Fatal("source evidence")
	}
	missing := strings.ReplaceAll(jsonContigSeed, `"umi_count":2`, `"umi_count":null`)
	r, err = ReadReader(context.Background(), strings.NewReader(missing), "s.json")
	if err != nil || !math.IsNaN(r.Table.Count[0]) {
		t.Fatal("missing count imputed", err)
	}
	r, err = ReadReader(context.Background(), strings.NewReader("\xef\xbb\xbf"+jsonContigSeed), "SAMPLE_all_contig_annotations.JSON")
	if err != nil || r.Table.Samples[0].Name != "SAMPLE" {
		t.Fatal("BOM/case-insensitive extension", err)
	}
	// Unknown extension keys must not collide through Go's struct-field folding.
	withCaseExtension := strings.Replace(jsonContigSeed, `"gene_name":"TRBJ2-5"}}]`, `"gene_name":"TRBJ2-5"},"Feature":{"chain":"TRA","region_type":"J-REGION","gene_name":"TRAJ33"}}]`, 1)
	r, err = ReadReader(context.Background(), strings.NewReader(withCaseExtension), "a.json")
	if err != nil || r.Table.LocusName(0) != "TRB" || r.Table.Dict.Get(r.Table.J[0]) != "TRBJ2-5" {
		t.Fatal("unknown key changed gene/chain evidence", err)
	}
}

func TestCellRangerJSONRefusals(t *testing.T) {
	for name, text := range map[string]string{
		"duplicate key":    strings.Replace(jsonContigSeed, `"umi_count":2`, `"umi_count":2,"umi_count":3`, 1),
		"duplicate contig": jsonContigSeed[:len(jsonContigSeed)-1] + "," + jsonContigSeed[1:],
		"flat layout":      `[{"barcode":"x","contig_id":"a","chain":"TRB","cdr3":"CSF"}]`,
		"flat conflict":    strings.Replace(jsonContigSeed, `"extra":`, `"chain":"TRA","extra":`, 1),
		"count precision":  strings.Replace(jsonContigSeed, `"umi_count":2`, `"umi_count":9007199254740993`, 1),
		"fraction":         strings.Replace(jsonContigSeed, `"umi_count":2`, `"umi_count":2.5`, 1),
		"exponent":         strings.Replace(jsonContigSeed, `"umi_count":2`, `"umi_count":2e0`, 1),
		"flag string":      strings.Replace(jsonContigSeed, `"is_cell":true`, `"is_cell":"true"`, 1),
		"padded barcode":   strings.ReplaceAll(jsonContigSeed, `"AAAC-1"`, `" AAAC-1"`),
		"surrogate":        strings.Replace(jsonContigSeed, `"AAAC-1"`, `"\ud800"`, 1),
		"invalid utf8":     strings.Replace(jsonContigSeed, `"AAAC-1"`, "\"\xff\"", 1),
		"annotations null": strings.Replace(jsonContigSeed, `[{"feature":{"chain":"TRB","region_type":"J-REGION","gene_name":"TRBJ2-5"}}]`, `null`, 1),
		"unknown region":   strings.Replace(jsonContigSeed, `J-REGION`, `UNKNOWN`, 1),
		"unknown chain":    strings.Replace(jsonContigSeed, `"TRB"`, `"TCRB"`, 1),
		"competing calls":  strings.Replace(jsonContigSeed, `"gene_name":"TRBJ2-5"}}]`, `"gene_name":"TRBJ2-5"}},{"feature":{"chain":"TRB","region_type":"J-REGION","gene_name":"TRBJ2-7"}}]`, 1),
		"trailing":         jsonContigSeed + "[]",
		"null":             `[null]`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadReader(context.Background(), strings.NewReader(text), "all_contig_annotations.json"); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadReader(ctx, strings.NewReader(jsonContigSeed), "x.json"); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestCellRangerJSONIdentityAndScope(t *testing.T) {
	var objects []map[string]any
	json.Unmarshal([]byte(jsonContigSeed), &objects)
	a := objects[0]
	a["sample"] = "S1"
	copyObject := func() map[string]any {
		b := map[string]any{}
		for k, v := range a {
			b[k] = v
		}
		b["contig_name"] = "contig2"
		return b
	}
	for _, kind := range []string{"cell", "clone", "different sample"} {
		b := copyObject()
		switch kind {
		case "cell":
			b["is_cell"] = false
		case "clone":
			b["info"] = map[string]any{"raw_clonotype_id": "other"}
		case "different sample":
			b["sample"] = "S2"
			b["is_cell"] = false
		}
		data, _ := json.Marshal([]any{a, b})
		_, err := ReadReader(context.Background(), bytes.NewReader(data), "a.json")
		if (err == nil) != (kind == "different sample") {
			t.Fatal(kind, err)
		}
	}
}

func TestCellRangerJSONGzipUploadAndFolder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "content-hash")
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	gz.Write([]byte(jsonContigSeed))
	gz.Close()
	os.WriteFile(path, data.Bytes(), 0600)
	r, err := OpenInputs(context.Background(), []Input{{Path: path, Name: "renamed.json.gz"}})
	if err != nil || !r.Sources[0].Gzipped || r.Sources[0].SHA256 == "" || r.Sources[0].Native.Layout != CellRangerJSONLayout {
		t.Fatal("uploaded JSON", err)
	}
	folder := filepath.Join(dir, "outs", "per_sample_outs", "S01", "vdj_t")
	os.MkdirAll(folder, 0700)
	jsonPath := filepath.Join(folder, "all_contig_annotations.json")
	os.WriteFile(jsonPath, []byte(jsonContigSeed), 0600)
	r, err = Open(context.Background(), []string{dir})
	if err != nil || r.Table.Samples[0].Name != "S01" {
		t.Fatal("folder fallback", err)
	}
	csvPath := filepath.Join(folder, "filtered_contig_annotations.csv")
	os.WriteFile(csvPath, []byte("barcode,contig_id,chain,cdr3,umis\nA-1,c1,TRB,CSF,2\n"), 0600)
	files, _, err := CellRangerFiles(dir)
	if err != nil || len(files) != 1 || files[0] != csvPath {
		t.Fatal("CSV precedence", files, err)
	}
	copyPath := filepath.Join(dir, "S01_all_contig_annotations.json")
	os.WriteFile(copyPath, []byte(jsonContigSeed), 0600)
	if _, err := Open(context.Background(), []string{jsonPath, copyPath}); err == nil {
		t.Fatal("cross-file duplicate accepted")
	}
}

func TestCellRangerJSONBounds(t *testing.T) {
	oversize := strings.Replace(jsonContigSeed, `"extra":9007199254740993`, `"extra":"`+strings.Repeat("x", maxContigJSON)+`"`, 1)
	if _, err := ReadReader(context.Background(), strings.NewReader(oversize), "x.json"); err == nil {
		t.Fatal("oversized record accepted")
	}
	if _, err := ReadReader(context.Background(), strings.NewReader("["+strings.Repeat(" ", maxNativeBytes)+"]"), "x.json"); err == nil {
		t.Fatal("oversized input accepted")
	}
}

func TestCellRangerJSONProbePreservesAIRRMetadata(t *testing.T) {
	dir := t.TempDir()
	table := filepath.Join(dir, "a.tsv")
	os.WriteFile(table, []byte("sequence_id\tjunction_aa\trepertoire_id\na\tCSF\tR1\n"), 0600)
	metadata := filepath.Join(dir, "study.json")
	os.WriteFile(metadata, []byte(`[{"repertoire_id":"R1","contig_name":"extension","barcode":"extension","annotations":[],"sequence":"ACGT"}]`), 0600)
	r, err := Open(context.Background(), []string{table, metadata})
	if err != nil || len(r.Repertoires) != 1 || r.Sources[0].Format != FormatAIRR {
		t.Fatal("native probe displaced explicit AIRR metadata", err)
	}
	// Existing metadata supports larger objects than a contig's bounded probe.
	os.WriteFile(metadata, []byte(`[{"repertoire_id":"R1","extension":"`+strings.Repeat("x", maxContigJSON+1)+`"}]`), 0600)
	r, err = Open(context.Background(), []string{table, metadata})
	if err != nil || len(r.Repertoires) != 1 || len(r.Repertoires[0].Raw) < maxContigJSON {
		t.Fatal("probe truncated/refused supported metadata", err)
	}
}

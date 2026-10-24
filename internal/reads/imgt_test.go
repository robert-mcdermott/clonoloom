package reads

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/rep"
)

func imgtFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range imgtNames {
		data, e := os.ReadFile("../../testdata/imgt-inputs/" + n + ".txt")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, n+".txt"), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	return dir
}
func evidence(t *rep.Table, row int, name string) string {
	for i, n := range rep.EvidenceFields {
		if n == name {
			return t.EvidenceValue(row, i)
		}
	}
	return ""
}
func TestIMGTJoinAndEvidence(t *testing.T) {
	dir := imgtFixture(t)
	before := map[string][32]byte{}
	for _, n := range imgtNames {
		data, _ := os.ReadFile(filepath.Join(dir, n+".txt"))
		before[n] = sha256.Sum256(data)
	}
	res, err := Open(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	tab := res.Table
	if tab.Len() != 4 || res.Unit != rep.Sequences || len(tab.Samples) != 1 || len(res.Sources) != 5 {
		t.Fatalf("%d rows, %s, %d sources", tab.Len(), res.Unit, len(res.Sources))
	}
	if tab.IDs.Get(tab.SequenceID[0]) != "heavy" || tab.IDs.Get(tab.SequenceID[2]) != "heavy" || tab.Dict.Get(tab.V[0]) != "IGHV1-2*01,IGHV1-2*02" {
		t.Fatal("identities or tied calls lost")
	}
	for i, want := range []string{"CAW", "CQF", "", ""} {
		if tab.JunctionsAA.Get(tab.JunctionAA[i]) != want {
			t.Fatalf("AA %d", i)
		}
		if tab.Count[i] != 1 {
			t.Fatal("not sequence counts")
		}
	}
	if tab.Reads != nil || tab.UMIs != nil || tab.Cell != nil {
		t.Fatal("invented counts/cells")
	}
	if evidence(tab, 0, "sequence_alignment") != "aaa...tgtgcttggccc" || evidence(tab, 1, "sequence") != "gggtgtcaatttaaa" || evidence(tab, 1, "rev_comp") != "F" || !strings.Contains(evidence(tab, 1, "imgt_annotations"), `"Orientation":"-"`) {
		t.Fatal("gaps/orientation lost")
	}
	if evidence(tab, 0, "cdr3_start") != "7" || evidence(tab, 2, "cdr3_start") != "" || evidence(tab, 0, "germline_alignment") != "" || res.Problems.Count("imgt-coordinate") != 3 {
		t.Fatal("coordinate/germline mapping")
	}
	if tab.Productive[0] != rep.True || tab.Productive[1] != rep.True || tab.Productive[2] != rep.False || tab.Productive[3] != rep.Missing {
		t.Fatal("functionality")
	}
	if !strings.Contains(res.Sources[4].AnnotationParameters, "Not run") {
		t.Fatal("parameters lost")
	}
	for _, n := range imgtNames {
		data, _ := os.ReadFile(filepath.Join(dir, n+".txt"))
		if sha256.Sum256(data) != before[n] {
			t.Fatal("source changed")
		}
	}
	// Selected files in any order have the same table and normalized source order.
	files, _, _ := IMGTFiles(dir)
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	again, e := OpenInputs(context.Background(), files)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(res, again) {
		t.Fatal("selection order changed results")
	}
}
func TestIMGTFailures(t *testing.T) {
	cases := []struct{ name, file, from, to, want string }{
		{"mismatch", imgtNames[1], "3\theavy", "3\twrong", "missing Sequence number"},
		{"duplicate", imgtNames[1], "2\tlight", "1\theavy", "duplicate Sequence number"},
		{"gene", imgtNames[2], "Homsap IGKJ1*01 F", "Homsap IGKJ2*01 F", "conflicts"},
		{"width", imgtNames[3], "4\tfailed", "4\tfailed\textra", "fields"},
		{"header", imgtNames[3], "Sequence ID", "Sequence number", "duplicate column"},
		{"missing id", imgtNames[0], "1\theavy", "1\t", "nonempty Sequence ID"},
		{"missing column", imgtNames[2], "V-REGION start", "wrong start", "missing V-REGION start"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := imgtFixture(t)
			path := filepath.Join(dir, c.file+".txt")
			data, _ := os.ReadFile(path)
			os.WriteFile(path, []byte(strings.Replace(string(data), c.from, c.to, 1)), 0600)
			_, err := Open(context.Background(), []string{dir})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %s", err, c.want)
			}
		})
	}
	t.Run("incomplete", func(t *testing.T) {
		dir := imgtFixture(t)
		os.Remove(filepath.Join(dir, imgtNames[2]+".txt"))
		_, err := Open(context.Background(), []string{dir})
		if err == nil || !strings.Contains(err.Error(), "missing 3_Nt") {
			t.Fatal(err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := Open(ctx, []string{imgtFixture(t)})
		if err != context.Canceled {
			t.Fatal(err)
		}
	})
}
func TestIMGTGzipAndStoredNames(t *testing.T) {
	dir := imgtFixture(t)
	var inputs []Input
	for i, n := range imgtNames {
		path := filepath.Join(dir, n+".txt")
		data, _ := os.ReadFile(path)
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		gz.Write(data)
		gz.Close()
		stored := filepath.Join(t.TempDir(), string(rune('a'+i)))
		os.WriteFile(stored, buf.Bytes(), 0600)
		inputs = append(inputs, Input{Path: stored, Name: n + ".txt.gz"})
	}
	res, e := OpenInputs(context.Background(), inputs)
	if e != nil {
		t.Fatal(e)
	}
	if res.Table.Len() != 4 || res.Table.Samples[0].Name != "IMGT" {
		t.Fatal("library")
	}
	for _, s := range res.Sources {
		if !s.Gzipped || len(s.SHA256) != 64 || s.IMGTGroup != "IMGT" {
			t.Fatal(s)
		}
	}
	inputs = append(inputs, inputs[0])
	if _, e := OpenInputs(context.Background(), inputs); e == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestIMGTCoordinateMismatch(t *testing.T) {
	dir := imgtFixture(t)
	path := filepath.Join(dir, imgtNames[2]+".txt")
	data, _ := os.ReadFile(path)
	data = bytes.Replace(data, []byte("gct\t7\t9"), []byte("gct\t8\t10"), 1)
	os.WriteFile(path, data, 0600)
	res, e := Open(context.Background(), []string{dir})
	if e != nil {
		t.Fatal(e)
	}
	if evidence(res.Table, 0, "cdr3_start") != "" || !strings.Contains(evidence(res.Table, 0, "imgt_annotations"), `"CDR3-IMGT start":"8"`) {
		t.Fatal("mismatch mapped or discarded")
	}
}
func FuzzIMGT(f *testing.F) {
	for role, n := range imgtNames[:4] {
		data, e := os.ReadFile("../../testdata/imgt-inputs/" + n + ".txt")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(data, uint8(role))
	}
	f.Fuzz(func(t *testing.T, data []byte, role uint8) {
		x, e := indexIMGT(context.Background(), bytes.NewReader(data), "fuzz.txt", int(role)%4)
		if e != nil {
			return
		}
		defer x.close()
		for _, k := range x.order {
			v, _, e := x.get(k)
			if e != nil || len(v) != len(x.source.Header) {
				t.Fatal("index roundtrip", e)
			}
		}
	})
}

func TestIMGTRejectsMappingWithGroup(t *testing.T) {
	_, err := PrepareIMGTInputs([]Input{{Path: "1_Summary.txt", IMGTGroup: "run", Mapping: &Mapping{}}})
	if err == nil {
		t.Fatal("mapping must not bypass grouped-input validation")
	}
}

func TestIMGTLegacyHeadersAndPrefixedCohort(t *testing.T) {
	dir := t.TempDir()
	for _, prefix := range []string{"sampleA_", "sampleB_"} {
		for _, name := range imgtNames {
			data, e := os.ReadFile("../../testdata/imgt-inputs/" + name + ".txt")
			if e != nil {
				t.Fatal(e)
			}
			data = bytes.ReplaceAll(data, []byte("V-DOMAIN Functionality"), []byte("Functionality"))
			data = bytes.ReplaceAll(data, []byte("Homsap "), nil)
			if e = os.WriteFile(filepath.Join(dir, prefix+name+".txt"), data, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	res, e := Open(context.Background(), []string{dir})
	if e != nil {
		t.Fatal(e)
	}
	if res.Table.Len() != 8 || len(res.Table.Samples) != 2 || len(res.Sources) != 10 || res.Table.Samples[0].Name != "sampleA" || res.Table.Samples[1].Name != "sampleB" || res.Table.Source[4] != 5 || res.Table.Productive[1] != rep.True {
		t.Fatal("cohort grouping or legacy functionality differs")
	}
}

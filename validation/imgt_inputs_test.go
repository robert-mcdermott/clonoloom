package validation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"clonoloom/internal/export"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

func TestIMGTAgainstIndependentCSV(t *testing.T) {
	var ref struct {
		Sources map[string]string
		Rows    []struct {
			Number, ID, V, Junction, JunctionAA, Orientation string
			Evidence                                         map[string]string
		}
	}
	data, e := os.ReadFile("reference/imgt-inputs.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &ref); e != nil {
		t.Fatal(e)
	}
	dir := "../testdata/imgt-inputs"
	for name, want := range ref.Sources {
		data, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("source drift", name)
		}
	}
	res, e := reads.Open(context.Background(), []string{dir})
	if e != nil {
		t.Fatal(e)
	}
	tab := res.Table
	if tab.Len() != len(ref.Rows) {
		t.Fatal("row count")
	}
	for i, want := range ref.Rows {
		if tab.IDs.Get(tab.SequenceID[i]) != want.ID || tab.Dict.Get(tab.V[i]) != want.V || tab.Junctions.Get(tab.Junction[i]) != want.Junction || tab.JunctionsAA.Get(tab.JunctionAA[i]) != want.JunctionAA {
			t.Fatalf("row %d differs", i)
		}
		for j, name := range rep.EvidenceFields {
			if name == "imgt_annotations" {
				var native map[string]any
				if e := json.Unmarshal([]byte(tab.EvidenceValue(i, j)), &native); e != nil {
					t.Fatal(e)
				}
				if native["Sequence number"] != want.Number {
					t.Fatal("provenance lost")
				}
				if want.Orientation != "" && native["Orientation"] != want.Orientation {
					t.Fatal("orientation lost")
				}
				continue
			}
			if tab.EvidenceValue(i, j) != want.Evidence[name] {
				t.Fatalf("row %d %s: %q != %q", i, name, tab.EvidenceValue(i, j), want.Evidence[name])
			}
		}
	}
	// The exported AIRR fields, including native annotation JSON, survive another import.
	var buf bytes.Buffer
	if e := export.Rearrangements(&buf, tab, nil, nil); e != nil {
		t.Fatal(e)
	}
	again, e := reads.ReadReader(context.Background(), &buf, "roundtrip.tsv")
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < tab.Len(); i++ {
		if tab.EvidenceRow(i) != again.Table.EvidenceRow(i) {
			t.Fatalf("AIRR evidence roundtrip %d", i)
		}
	}
	t.Logf("%d artificial rows: independent CSV joins, mapped values, native identity and AIRR round trips agree", tab.Len())
}

package reads

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAIRRObjectsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
	os.WriteFile(a, []byte(`{"Info":{"version":"2.0"},"Clone":[{"clone_id":"cl","repertoire_id":"r","nodes":[{"node_id":"n","repertoire_id":"r","cell_id":"c"}]}]}`), 0600)
	os.WriteFile(b, []byte(`{"Info":{"version":"2.0"},"Cell":[{"cell_id":"c","repertoire_id":"r","virtual_pairing":false}]}`), 0600)
	r, err := Open(context.Background(), []string{"../../testdata/airr-objects/study.tsv", a, b})
	if err != nil {
		t.Fatal(err)
	}
	if r.Objects[0].Links[1].Status != "loaded" || r.Objects[2].FileIndex != 1 {
		t.Fatal(r.Objects)
	}
	if _, err := Open(context.Background(), []string{"../../testdata/airr-objects/study.tsv", a, b, b}); err == nil {
		t.Fatal("cross-file duplicate accepted")
	}
	r, err = OpenInputs(context.Background(), []Input{{Path: "../../testdata/airr-objects/study.tsv"}, {Path: "../../testdata/airr-objects/study.yaml", Name: "upload.yml"}})
	if err != nil || len(r.Objects) != 7 {
		t.Fatalf("YAML: %v", err)
	}
}

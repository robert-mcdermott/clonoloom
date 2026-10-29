package cache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/reads"
)

func TestNativeReportEvidenceSurvivesCache(t *testing.T) {
	res, err := reads.ReadFiles(context.Background(), []string{"../../testdata/native-reports/simulated-trust4.tsv", "../../testdata/native-reports/simulated-vdjtools.tsv"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := encode("native-reports", res)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "table.clt")
	os.WriteFile(path, data, 0600)
	got, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	same(t, res, got)
	if got.Sources[1].Native.Rows[0].Text != res.Sources[1].Native.Rows[0].Text {
		t.Fatal("native extensions changed")
	}
	metadata, err := json.Marshal(got.Sources)
	if err != nil || strings.Contains(string(metadata), "assigned fractional support") || strings.Contains(string(metadata), `"native"`) {
		t.Fatal("raw evidence leaked into generic source metadata", err)
	}
}

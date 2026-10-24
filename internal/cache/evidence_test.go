package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/reads"
)

func TestEvidenceCache(t *testing.T) {
	res, err := reads.ReadFiles(context.Background(), []string{"../../testdata/bcell-inputs.tsv"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encode("evidence", res)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "table.clt")
	os.WriteFile(path, encoded, 0o644)
	got, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	same(t, res, got)
	for _, bad := range []string{"length", "code"} {
		t.Run(bad, func(t *testing.T) {
			res, err := reads.ReadFiles(context.Background(), []string{"../../testdata/bcell-inputs.tsv"})
			if err != nil {
				t.Fatal(err)
			}
			if bad == "length" {
				res.Table.Evidence[0] = res.Table.Evidence[0][:1]
			} else {
				res.Table.Evidence[0][0] = ^uint32(0)
			}
			bytes, err := encode("evidence", res)
			if err != nil {
				t.Fatal(err)
			}
			os.WriteFile(path, bytes, 0o644)
			if _, err := load(path); err == nil {
				t.Fatal("accepted corrupt evidence")
			}
		})
	}
	plain, err := reads.ReadReader(context.Background(), strings.NewReader(airrTSV), "plain.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if plain.Table.EvidenceText != nil {
		t.Fatal("allocated evidence store for junction-only data")
	}
	for _, column := range plain.Table.Evidence {
		if column != nil {
			t.Fatal("allocated absent column")
		}
	}
}

func TestOldEvidenceCacheVersionsAreRefused(t *testing.T) {
	res, err := reads.ReadFiles(context.Background(), []string{"../../testdata/bcell-inputs.tsv"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := encode("evidence", res)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"version", "readers"} {
		// Same-length header change leaves the body CRC intact: the version alone must reject it.
		current := formatVersion
		if field == "readers" {
			current = reads.ReaderVersion
		}
		old := strings.Replace(string(data), fmt.Sprintf(`"%s":%d`, field, current), `"`+field+`":1`, 1)
		if old == string(data) {
			t.Fatal("version marker absent")
		}
		path := filepath.Join(t.TempDir(), "old.clt")
		if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := load(path); err == nil {
			t.Fatalf("accepted old %s", field)
		}
	}
}

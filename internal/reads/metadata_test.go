package reads

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetadataSnapshotAndGzip(t *testing.T) {
	original := []byte(`{"Info":{"version":"2.0"},"Repertoire":[{"repertoire_id":"R","extra":9007199254740993}]}`)
	var zipped bytes.Buffer
	g := gzip.NewWriter(&zipped)
	g.Write(original)
	g.Close()
	for _, source := range [][]byte{original, zipped.Bytes()} {
		path := filepath.Join(t.TempDir(), "study.json")
		if err := os.WriteFile(path, source, 0600); err != nil {
			t.Fatal(err)
		}
		got, file, err := readMetadata(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(source)
		if !bytes.Equal(got, original) || file.Size != int64(len(source)) || file.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("snapshot/fingerprint differs: %+v %s", file, got)
		}
		os.WriteFile(path, []byte(`[]`), 0600)
		if !bytes.Equal(got, original) {
			t.Fatal("later mutation changed snapshot")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := readMetadata(ctx, path); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "broken.json.gz")
	os.WriteFile(path, zipped.Bytes()[:zipped.Len()-5], 0600)
	if _, _, err := readMetadata(context.Background(), path); err == nil {
		t.Fatal("truncated gzip accepted")
	}
}

func TestMetadataIDsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	table := filepath.Join(dir, "study.tsv")
	os.WriteFile(table, []byte("sequence_id\tv_call\tj_call\tjunction_aa\tproductive\trepertoire_id\n1\tTRBV5-1\tTRBJ2-7\tCASSLAGGPEAFF\tT\tR\n"), 0600)
	first, second := filepath.Join(dir, "one.json"), filepath.Join(dir, "two.json")
	os.WriteFile(first, []byte(`{"Info":{"version":"1.3"},"Repertoire":[{"repertoire_id":"R"}]}`), 0600)
	os.WriteFile(second, []byte(`{"repertoire_id":"R"}`), 0600)
	if _, err := Open(context.Background(), []string{table, first, second}); err == nil || !strings.Contains(err.Error(), "duplicate repertoire_id") {
		t.Fatalf("ambiguous join allowed: %v", err)
	}
	os.WriteFile(second, []byte(`{"Info":{"version":"2.0"},"Repertoire":[{"repertoire_id":"r"}]}`), 0600)
	r, err := Open(context.Background(), []string{table, first, second})
	if err != nil || len(r.Repertoires) != 2 || r.Metadata[0].SchemaVersion != "1.3" || r.Metadata[1].SchemaVersion != "2.0" {
		t.Fatalf("case-sensitive IDs: %v", err)
	}
}

package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

func TestBCellEvidenceAgainstCSV(t *testing.T) {
	var ref struct {
		Fields []string
		Files  []struct {
			Path, Source, Digest string
			Rows                 int
			Present              []int
		}
	}
	raw, err := os.ReadFile("reference/bcell-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ref.Fields, rep.EvidenceFields[:]) {
		t.Fatal("reference field contract changed")
	}
	for _, file := range ref.Files {
		t.Run(file.Path, func(t *testing.T) {
			path := filepath.Join("..", file.Path)
			if strings.HasPrefix(file.Path, "validation/cache/") {
				tail := strings.TrimPrefix(file.Path, "validation/cache/")
				path = filepath.Join(dataRoot(), tail)
				if _, err := os.Stat(path); err != nil {
					set := strings.Split(tail, "/")[0]
					if required(set) {
						t.Fatal(err)
					}
					t.Skip("development data not fetched")
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(before)
			if hex.EncodeToString(sum[:]) != file.Source {
				t.Fatal("source checksum differs")
			}
			result, err := reads.ReadFiles(context.Background(), []string{path})
			if err != nil {
				t.Fatal(err)
			}
			h := sha256.New()
			present := make([]int, rep.EvidenceCount)
			for row := 0; row < result.Table.Len(); row++ {
				values := result.Table.EvidenceRow(row)
				for i, v := range values {
					if v != "" {
						present[i]++
					}
				}
				fmt.Fprintln(h, strings.Join(values[:], "\t"))
			}
			if result.Table.Len() != file.Rows || !reflect.DeepEqual(present, file.Present) || hex.EncodeToString(h.Sum(nil)) != file.Digest {
				t.Fatal("retained evidence differs from independent CSV reader")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if sha256.Sum256(after) != sum {
				t.Fatal("source modified")
			}
			t.Logf("%d rows, all %d evidence fields equal independent CSV values", file.Rows, len(ref.Fields))
		})
	}
}
func TestEvidenceMemory(t *testing.T) {
	// Fixed synthetic data isolates the extra columns from existing rearrangement costs.
	const n = 100000
	makeTable := func(evidence bool) *rep.Table {
		b := rep.NewBuilder(rep.Sequences)
		r := rep.NewRow()
		r.Sample = "synthetic"
		r.Count = 1
		for i := 0; i < n; i++ {
			r.SequenceID = fmt.Sprintf("r%d", i)
			if evidence {
				r.Evidence[0] = strings.Repeat("ACGT", 75) + fmt.Sprintf("%08d", i)
				r.Evidence[2] = strings.Repeat("ACGT", 75)
				r.Evidence[3] = r.Evidence[2]
			}
			if err := b.Append(&r); err != nil {
				t.Fatal(err)
			}
		}
		return b.Table()
	}
	start := time.Now()
	base := makeTable(false)
	enriched := makeTable(true)
	if base.EvidenceText != nil {
		t.Fatal("junction-only data allocated evidence")
	}
	extra := enriched.Size() - base.Size()
	if extra > 64<<20 {
		t.Fatalf("optional evidence used %d bytes; budget 64 MiB for this fixed dataset", extra)
	}
	t.Logf("%d rows: additional sequence evidence %.2f MiB in %v (three populated columns, unique query and repeated alignments)", n, float64(extra)/(1<<20), time.Since(start).Round(time.Millisecond))
}

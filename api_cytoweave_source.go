package main

// This interchange needs exact source cell identities, beyond the general reader's
// analysis-oriented whitespace handling. Verify selected native AIRR sources once,
// not once per chain. Other reader formats need their own exact identity contract.
import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/reads"
)

func verifyCytoIdentities(ctx context.Context, ds *dataset, sample int) error {
	t := ds.Result.Table
	if sample < 0 || sample >= len(t.Samples) || t.Cell == nil {
		return errors.New("Select one receptor sample with cell IDs")
	}
	selected := map[uint16]map[uint32]int{}
	for row, s := range t.Sample {
		if row%65536 == 0 {
			if e := ctx.Err(); e != nil {
				return e
			}
		}
		if int(s) != sample {
			continue
		}
		source := t.Source[row]
		if selected[source] == nil {
			selected[source] = map[uint32]int{}
		}
		if _, ok := selected[source][t.Line[row]]; ok {
			return errors.New("Repeated source row identity")
		}
		selected[source][t.Line[row]] = row
	}
	total := 0
	for index, lines := range selected {
		source := ds.Result.Sources[index]
		if source.Format != reads.FormatAIRR || source.Mapping != nil {
			return errors.New("Exact phenotype links currently require native AIRR TSV receptor sources; other readers need separate source-identity validation")
		}
		raw, e := readCytoSource(ctx, source)
		if e != nil {
			return e
		}
		total += len(raw)
		if total > 64<<20 {
			return errors.New("Selected AIRR sources exceed 64 MiB decoded")
		}
		if !utf8.Valid(raw) {
			return errors.New("AIRR source must be valid UTF-8")
		}
		scan := bufio.NewScanner(bytes.NewReader(raw))
		scan.Buffer(make([]byte, 4096), 1<<20)
		cell, repID := -1, -1
		var columns []string
		found := 0
		for n := uint32(1); scan.Scan(); n++ {
			if e := ctx.Err(); e != nil {
				return e
			}
			if n == 1 {
				columns = strings.Split(strings.TrimPrefix(scan.Text(), "\ufeff"), "\t")
				seen := map[string]bool{}
				for i, k := range columns {
					if seen[k] {
						return errors.New("Repeated AIRR header")
					}
					seen[k] = true
					if k == "cell_id" {
						cell = i
					}
					if k == "repertoire_id" {
						repID = i
					}
				}
				if cell < 0 {
					return errors.New("Original AIRR cell_id column is absent or normalized")
				}
				continue
			}
			row, ok := lines[n]
			if !ok {
				continue
			}
			fields := strings.Split(scan.Text(), "\t")
			if len(fields) != len(columns) {
				return errors.New("AIRR row width differs from source header")
			}
			if fields[cell] != t.Dict.Get(t.Cell[row]) {
				return errors.New("Source cell_id differs from the loaded identifier (including whitespace); correct upstream and reopen")
			}
			if repID >= 0 && fields[repID] != t.Samples[sample].RepertoireID {
				return errors.New("Source repertoire_id differs from the loaded sample; correct upstream and reopen")
			}
			found++
		}
		if e := scan.Err(); e != nil {
			return e
		}
		if found != len(lines) {
			return errors.New("Selected AIRR source rows are unavailable")
		}
	}
	return nil
}
func readCytoSource(ctx context.Context, source reads.Source) ([]byte, error) {
	st, e := os.Lstat(source.Path)
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Size() > 64<<20 {
		return nil, errors.New("Exact phenotype links require regular AIRR sources within 64 MiB")
	}
	f, e := os.Open(source.Path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	current, e := f.Stat()
	if e != nil || !os.SameFile(st, current) {
		return nil, errors.New("AIRR source changed while opening")
	}
	raw, e := io.ReadAll(io.LimitReader(runContextReader{ctx, f}, (64<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > 64<<20 || int64(len(raw)) != source.Size || replayDigest(raw) != source.SHA256 {
		return nil, errors.New("AIRR source bytes changed; reopen the dataset and review")
	}
	if source.Gzipped {
		g, e := gzip.NewReader(bytes.NewReader(raw))
		if e != nil {
			return nil, e
		}
		defer g.Close()
		raw, e = io.ReadAll(io.LimitReader(runContextReader{ctx, g}, (64<<20)+1))
		if e != nil {
			return nil, e
		}
		if len(raw) > 64<<20 {
			return nil, errors.New("Decoded AIRR source exceeds 64 MiB")
		}
	}
	return raw, nil
}

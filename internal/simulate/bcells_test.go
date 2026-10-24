package simulate

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/random"
	"clonoloom/internal/seq"
)

func smallBCellStudy() Study {
	s := Presets["bcell-immunization"]
	s.BCells = &BCellStudy{Subjects: 2, Families: 4, CellsPerSample: 40, MutationsPerBranch: 3}
	return s
}

// Every event must reproduce the child, including repeated hits and reversions; no translation
// or event count is inferred from the final sequence alone. All families have both light loci
// across the study, a single ancestral pair and a forward isotype history.
func TestBCellFamiliesAndMutationHistory(t *testing.T) {
	s := smallBCellStudy()
	for f := range 4 {
		nodes := bcellFamily(s, "S01", f, 0)
		for i, node := range nodes {
			if i == 0 {
				if node.Parent != "" || len(node.Heavy.Mutations)+len(node.Light.Mutations) != 0 {
					t.Fatal("ancestor has history")
				}
				continue
			}
			parent := nodes[(i-1)/2]
			if node.Parent != parent.ID || node.Family != parent.Family || node.Subject != parent.Subject {
				t.Fatal("parent, family or subject changed")
			}
			order := map[string]int{"IGHM": 0, "IGHG1": 1, "IGHA1": 2}
			if order[node.Isotype] < order[parent.Isotype] {
				t.Fatal("backwards switch")
			}
			for k, child := range []bcellChain{node.Heavy, node.Light} {
				p := []bcellChain{parent.Heavy, parent.Light}[k]
				bases := []byte(p.Sequence)
				if len(child.Mutations) != s.BCells.MutationsPerBranch {
					t.Fatal("wrong event count")
				}
				for _, event := range child.Mutations {
					at := event.Position - 1
					if event.From != string(bases[at]) || event.From == event.To {
						t.Fatal("wrong event source")
					}
					codon := at / 3 * 3
					before := seq.Translate(string(bases[codon : codon+3]))
					bases[at] = event.To[0]
					after := seq.Translate(string(bases[codon : codon+3]))
					if (before == after) != (event.Effect == "silent") || after == "*" {
						t.Fatal("wrong event effect or introduced stop")
					}
				}
				if string(bases) != child.Sequence || strings.Contains(seq.Translate(child.Sequence), "*") || child.VCall != p.VCall || child.JCall != p.JCall {
					t.Fatal("events do not reproduce productive child")
				}
				if child.Sequence[282:285] != p.Sequence[282:285] || child.Sequence[318:321] != p.Sequence[318:321] {
					t.Fatal("anchor changed")
				}
				if i == 3 {
					last := p.Mutations[len(p.Mutations)-1]
					first := child.Mutations[0]
					if last.Position != first.Position || last.From != first.To || last.To != first.From {
						t.Fatal("missing deliberate reversal")
					}
				}
			}
		}
	}
	// A one-event edge followed by its deliberate reversal is exactly its ancestor: distinct
	// known nodes can have the same sequence. The truth must not collapse those nodes.
	a := bcellAncestor(7, "IGH", 0, random.New(7, 1))
	b := mutateBCell(a, 1, false, random.New(7, 2))
	c := mutateBCell(b, 1, true, random.New(7, 3))
	if c.Sequence != a.Sequence || len(c.Mutations) != 1 {
		t.Fatal("one-event reversal did not return to ancestor")
	}
}

func TestBCellStudyTruthAndBytes(t *testing.T) {
	s := smallBCellStudy()
	dir := filepath.Join(t.TempDir(), "study")
	summary, err := Run(context.Background(), s, dir, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Subjects != 2 || summary.Samples != 6 || summary.Rows != 480 {
		t.Fatalf("%+v", summary)
	}
	var truth bcellTruth
	raw, err := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	if err != nil || json.Unmarshal(raw, &truth) != nil || truth.Model != bcellModel || len(truth.Nodes) != 56 {
		t.Fatalf("bad truth: %s %v", raw, err)
	}
	if !reflect.DeepEqual(s, truth.Study) {
		t.Fatal("parameters changed in serialization")
	}
	for _, file := range truth.Files {
		data, err := os.ReadFile(filepath.Join(dir, file.Path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != file.SHA256 {
			t.Fatalf("%s hash differs", file.Path)
		}
	}
	file, _ := os.Open(filepath.Join(dir, "truth", "observations.tsv"))
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	observations, err := reader.ReadAll()
	if err != nil || len(observations) != 241 {
		t.Fatalf("observations %d: %v", len(observations), err)
	}
	counts := map[string]int{}
	for _, row := range observations[1:] {
		counts[row[0]+"/"+row[2]]++
	}
	for _, sample := range truth.Samples {
		total, probability := 0, 0.0
		for _, a := range sample.Abundance {
			if a.Cells != counts[sample.ID+"/"+a.Node] || a.Probability == 0 && a.Cells != 0 {
				t.Fatal("draw differs from truth")
			}
			total += a.Cells
			probability += a.Probability
		}
		if total != 40 || probability < 0.999999999999 || probability > 1.000000000001 {
			t.Fatal("wrong cell total or probabilities")
		}
	}
	again := filepath.Join(t.TempDir(), "study")
	if _, err := Run(context.Background(), truth.Study, again, "test", nil); err != nil {
		t.Fatal(err)
	}
	if digest(t, dir) != digest(t, again) {
		t.Fatal("same serialized parameters differ")
	}
	const want = "f2fa95c0206c104f97ede7427ac2d3ecdb1d006139f3879738d853d099146144"
	if got := digest(t, dir); got != want {
		t.Errorf("B-cell study digest %s; want %s (check arm64 and amd64)", got, want)
	}
	s.Seed++
	other := filepath.Join(t.TempDir(), "study")
	if _, err := Run(context.Background(), s, other, "test", nil); err != nil {
		t.Fatal(err)
	}
	if digest(t, dir) == digest(t, other) {
		t.Fatal("seed did not change study")
	}
	if _, err := Run(context.Background(), s, dir, "test", nil); err == nil {
		t.Fatal("overwrote nonempty destination")
	}
}

func TestBCellParametersAndCancellation(t *testing.T) {
	for _, b := range []BCellStudy{{0, 1, 1, 1}, {21, 1, 1, 1}, {1, 0, 1, 1}, {1, 101, 1, 1}, {1, 1, 0, 1}, {1, 1, 10001, 1}, {1, 1, 1, 0}, {1, 1, 1, 21}, {20, 1, 10000, 1}} {
		s := smallBCellStudy()
		s.BCells = &b
		if err := s.Check(); err == nil {
			t.Errorf("accepted %+v", b)
		}
	}
	s := smallBCellStudy()
	s.Series = &Series{}
	if s.Check() == nil {
		t.Fatal("accepted two models")
	}
	s = smallBCellStudy()
	s.Templates = []int{20}
	if s.Check() == nil {
		t.Fatal("silently ignored TRB parameters")
	}
	s = smallBCellStudy()
	for _, midRun := range []bool{false, true} {
		parent := t.TempDir()
		dir := filepath.Join(parent, "cancelled")
		ctx, cancel := context.WithCancel(context.Background())
		if !midRun {
			cancel()
		}
		_, err := Run(ctx, s, dir, "test", func(_ float64, _ string) { cancel() })
		cancel()
		entries, _ := os.ReadDir(parent)
		if err != context.Canceled || len(entries) != 0 {
			t.Fatalf("cancel: %v, left %v", err, entries)
		}
	}
	// An explicitly supplied empty directory is accepted, as on the TRB path.
	dir := t.TempDir()
	if _, err := Run(context.Background(), s, dir, "test", nil); err != nil {
		t.Fatal(err)
	}
}

// Fixed-seed draws should follow the stated probabilities, not just agree with the recorded
// realized counts. At 60,000 draws every nonzero state has at least 200 expected observations.
func TestBCellSamplingProbabilities(t *testing.T) {
	const draws = 60000
	for stage := range 3 {
		weights := bcellWeights(12, stage)
		cumulative := random.Cumulate(weights)
		r := random.New(73, uint64(stage))
		counts := make([]int, len(weights))
		for range draws {
			counts[r.Discrete(cumulative)]++
		}
		maxZ := 0.0
		for i, p := range weights {
			if p == 0 {
				if counts[i] != 0 {
					t.Fatal("sampled a zero probability")
				}
				continue
			}
			z := math.Abs(float64(counts[i])-float64(draws)*p) / math.Sqrt(float64(draws)*p*(1-p))
			maxZ = math.Max(maxZ, z)
			if z > 5 {
				t.Fatalf("stage %d state %d: %d draws, probability %g, z %.3f", stage, i, counts[i], p, z)
			}
		}
		t.Logf("stage %d: largest absolute z %.3f", stage, maxZ)
	}
}

package main

// Export the existing native replay controls for cross-platform container validation.
// No alternate scientific implementation or rounded result representation is introduced.
import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/diversity"
)

func TestContainerReplayFixtures(t *testing.T) {
	dir := os.Getenv("CLONOLOOM_CONTAINER_FIXTURES")
	if dir == "" {
		t.Skip("set CLONOLOOM_CONTAINER_FIXTURES to export container replay controls")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name+".certificate.clz"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{cohortFeaturesKind, cohortCompareKind, cohortCountsKind} {
		write(kind, replayFixture(t, kind, nil))
	}
	write("differs", replayFixture(t, cohortCompareKind, func(_ *artifacts.Manifest, raw []byte) []byte {
		var doc cohortCompareDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		v := doc.Result.Comparisons[0].Mean.Estimate
		doc.Result.Comparisons[0].Mean.Estimate = diversity.Number(math.Nextafter(float64(v), math.Inf(1)))
		return mustJSON(doc)
	}))
	write("unavailable", replayFixture(t, cohortCompareKind, func(m *artifacts.Manifest, raw []byte) []byte {
		var spec cohortCompareSpec
		if err := json.Unmarshal(m.Lineage.Job.Spec, &spec); err != nil {
			t.Fatal(err)
		}
		spec.Method = 99
		m.Lineage.Job.Spec = mustJSON(spec)
		return raw
	}))
}

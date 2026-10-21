// ClonoLoom's diversity measures (internal/diversity) on the development files' bulk samples and
// the simulated standard study, against independent implementations: iNEXT 3.0.2 (Chao1 and its
// interval, the asymptotic Hill numbers of order 1 and 2, sample coverage), immunarch's own
// functions (Chao1 and its interval, Hill numbers, Gini, Gini–Simpson, inverse Simpson), vegan
// (ACE) and R itself (the rest), as validation/reference/diversity.R computes them.
package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
)

type diversityReference struct {
	Abundance        float64             `json:"abundance"`
	Clonotypes       int                 `json:"clonotypes"`
	Singletons       int                 `json:"singletons"`
	Doubletons       int                 `json:"doubletons"`
	Richness         float64             `json:"richness"`
	Shannon          float64             `json:"shannon"`
	Hill1            float64             `json:"hill1"`
	Simpson          float64             `json:"simpson"`
	Hill2            float64             `json:"hill2"`
	Evenness         float64             `json:"evenness"`
	Clonality        float64             `json:"clonality"`
	SimpsonClonality float64             `json:"simpsonClonality"`
	Top10            float64             `json:"top10"`
	Top100           float64             `json:"top100"`
	Top1000          float64             `json:"top1000"`
	MaxFrequency     float64             `json:"maxFrequency"`
	Homeostasis      map[string]float64  `json:"homeostasis"`
	Profile          map[string]float64  `json:"profile"`
	Coverage         float64             `json:"coverage"`
	Immunarch        map[string]float64  `json:"immunarch"`
	INEXT            map[string]float64  `json:"inext"`
	Vegan            map[string]*float64 `json:"vegan"`
}

var simulatedOnce sync.Once
var simulatedDir string

// referencePath finds a reference file in the development data; the simulated standard study is
// made (the same, byte for byte) when it is not there.
func referencePath(t *testing.T, rel string) (string, bool) {
	t.Helper()
	path := filepath.Join(dataRoot(), filepath.FromSlash(rel))
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	dataset, file, _ := strings.Cut(filepath.ToSlash(rel), "/")
	if dataset != "simulated-standard" {
		return "", false
	}
	simulatedOnce.Do(func() {
		dir, err := os.MkdirTemp("", "clonoloom-simulated-")
		if err != nil {
			return
		}
		simulatedDir = filepath.Join(dir, "standard")
		if _, err := simulate.Run(context.Background(), simulate.Presets["standard"], simulatedDir, "validation", nil); err != nil {
			simulatedDir = ""
		}
	})
	if simulatedDir == "" {
		return "", false
	}
	return filepath.Join(simulatedDir, file), true
}

// sampleAbundances answers each sample's clonotype abundances by the default bulk definition.
func sampleAbundances(t *testing.T, path string) map[string][]float64 {
	t.Helper()
	res, err := reads.ReadFiles(context.Background(), []string{path})
	if err != nil {
		t.Fatal(err)
	}
	def, _ := clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}.Normalize()
	table, err := clonotype.Compute(res.Table, nil, def)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]float64{}
	for i := range table.Clonotypes {
		samples, abundances := table.Abundances(i)
		for k, s := range samples {
			name := res.Table.Samples[s].Name
			out[name] = append(out[name], abundances[k])
		}
	}
	return out
}

func TestDiversityAgreesWithINEXTImmunarchAndVegan(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "diversity.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string                                   `json:"generated_by"`
		Files       map[string]map[string]diversityReference `json:"files"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	close := func(a, b float64) bool {
		return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b)) || (math.IsNaN(a) && math.IsNaN(b))
	}
	var files []string
	for rel := range ref.Files {
		files = append(files, rel)
	}
	sort.Strings(files)
	compared, seRatios := 0, []float64{}
	for _, rel := range files {
		path, ok := referencePath(t, rel)
		if !ok {
			if required(strings.Split(rel, "/")[0]) {
				t.Errorf("%s is required and missing", rel)
			}
			continue
		}
		abundances := sampleAbundances(t, path)
		for name, want := range ref.Files[rel] {
			x, ok := abundances[name]
			if !ok {
				t.Errorf("%s: no sample %s", rel, name)
				continue
			}
			r := diversity.Measure(x, diversity.DefaultOptions())
			m := func(id string) float64 { return float64(r.Measures[id].Estimate) }
			check := func(what string, got, wantValue float64) {
				if !close(got, wantValue) {
					t.Errorf("%s %s %s: %.12g, the reference %.12g", rel, name, what, got, wantValue)
				}
			}
			if r.Clonotypes != want.Clonotypes || r.Singletons != want.Singletons || r.Doubletons != want.Doubletons {
				t.Errorf("%s %s: %d clonotypes, f1 %d, f2 %d; the reference %d, %d, %d", rel, name, r.Clonotypes, r.Singletons, r.Doubletons, want.Clonotypes, want.Singletons, want.Doubletons)
				continue
			}
			// R itself.
			for id, v := range map[string]float64{"richness": want.Richness, "shannon": want.Shannon, "hill1": want.Hill1, "simpson": want.Simpson, "hill2": want.Hill2,
				"evenness": want.Evenness, "clonality": want.Clonality, "simpsonClonality": want.SimpsonClonality, "top10": want.Top10, "top100": want.Top100, "top1000": want.Top1000,
				"maxFrequency": want.MaxFrequency, "coverage": want.Coverage} {
				check(id, m(id), v)
			}
			for bin, v := range want.Homeostasis {
				check("homeostasis "+bin, m("homeostasis."+bin), v)
			}
			for q, v := range want.Profile {
				var order float64
				fmt.Sscan(q, &order)
				b, _ := json.Marshal(order)
				check("Hill "+q, m("hill@"+string(b)), v)
			}
			// immunarch's functions.
			im := want.Immunarch
			check("immunarch Hill 1", m("hill1"), im["hill1"])
			check("immunarch Hill 2", m("hill2"), im["hill2"])
			check("immunarch Gini", m("gini"), im["gini"])
			check("immunarch Gini–Simpson", m("giniSimpson"), im["giniSimpson"])
			check("immunarch inverse Simpson", m("hill2"), im["inverseSimpson"])
			chao := r.Measures["chao1"]
			// Where Chao1 estimates no clonotype unseen, immunarch's interval (none, or another
			// variance) is not iNEXT's; ClonoLoom follows iNEXT.
			if float64(chao.Estimate) > float64(r.Clonotypes) {
				check("immunarch Chao1", float64(chao.Estimate), im["chao1"])
				check("immunarch Chao1 SD", float64(chao.SE), im["chao1SD"])
				check("immunarch Chao1 lower", float64(chao.Lower), im["chao1Lower"])
				check("immunarch Chao1 upper", float64(chao.Upper), im["chao1Upper"])
			}
			// iNEXT, whose tables are rounded to three decimals.
			in := want.INEXT
			checkINEXT := func(what string, got, wantValue float64) {
				if math.Abs(got-wantValue) > 5e-4+1e-9*math.Abs(wantValue) {
					t.Errorf("%s %s %s: %.12g, iNEXT %.12g", rel, name, what, got, wantValue)
				}
			}
			checkINEXT("iNEXT Chao1", float64(chao.Estimate), in["chao1"])
			checkINEXT("iNEXT Chao1 SE", float64(chao.SE), in["chao1SE"])
			checkINEXT("iNEXT Chao1 lower", float64(chao.Lower), in["chao1Lower"])
			checkINEXT("iNEXT Chao1 upper", float64(chao.Upper), in["chao1Upper"])
			checkINEXT("iNEXT asymptotic Hill 1", m("hill1Estimated"), in["hill1"])
			checkINEXT("iNEXT asymptotic Hill 2", m("hill2Estimated"), in["hill2"])
			// The asymptotic estimates' intervals, as iNEXT's: the estimate ± 1.96 SE, the Shannon
			// diversity's no lower than that seen (iNEXT floors the Simpson diversity's too, which
			// ClonoLoom does not: research.md §5.3). Their SEs are each a bootstrap's, so their
			// sizes are compared below.
			for _, q := range []string{"hill1", "hill2"} {
				v := r.Measures[q+"Estimated"]
				lower := float64(v.Estimate) - 1.959963984540054*float64(v.SE)
				if q == "hill1" {
					lower = max(lower, m(q))
				}
				if math.Abs(float64(v.Lower)-max(lower, 1)) > 1e-9 || math.Abs(float64(v.Upper)-(float64(v.Estimate)+1.959963984540054*float64(v.SE))) > 1e-9 {
					t.Errorf("%s %s: %s interval %v–%v", rel, name, q, float64(v.Lower), float64(v.Upper))
				}
			}
			// vegan.
			// vegan gives none (null) where ACE is not estimable.
			wantACE := math.NaN()
			if v := want.Vegan["ace"]; v != nil {
				wantACE = *v
			}
			check("vegan ACE", m("ace"), wantACE)
			// The bootstrap's standard errors, each a random draw of its own: within a quarter of
			// iNEXT's (with 200 replicates each, their own error is about 5%).
			for _, q := range []string{"hill1", "hill2"} {
				ours, theirs := float64(r.Measures[q+"Estimated"].SE), in[q+"SE"]
				if theirs > 0 {
					seRatios = append(seRatios, ours/theirs)
					if ours/theirs < 0.75 || ours/theirs > 1.33 {
						t.Errorf("%s %s: the asymptotic %s's bootstrap SE %.4g, iNEXT's %.4g", rel, name, q, ours, theirs)
					}
				}
			}
			compared++
		}
	}
	sort.Float64s(seRatios)
	if len(seRatios) > 0 {
		t.Logf("%d samples compared (%s); bootstrap SE over iNEXT's: median %.3f, range %.3f–%.3f", compared, ref.GeneratedBy, seRatios[len(seRatios)/2], seRatios[0], seRatios[len(seRatios)-1])
	}
	if compared == 0 {
		t.Skip("no development data: node validation/fetch.mjs --ci")
	}
}

package validation

// Independent adjacency-graph reference from CSV; all memberships and nearest
// distances are checked, not just family totals. No truth enters inference.
import (
	"clonoloom/internal/bcell"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func familyDigest(lines []string) string {
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	return hex.EncodeToString(sum[:])
}
func TestClonalFamiliesAgainstIndependentGraph(t *testing.T) {
	var ref struct {
		Files        []struct{ Name, SHA256 string }
		Observations int
		Cases        []struct {
			Mode                            string
			Threshold                       float64
			Families, Representatives       int
			Comparisons                     int64
			MembershipSHA256, NearestSHA256 string
		}
	}
	raw, e := os.ReadFile("reference/clonal-families.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &ref); e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(t.TempDir(), "study")
	if _, e = simulate.Run(context.Background(), simulate.Presets["bcell-immunization"], dir, "validation", nil); e != nil {
		t.Fatal(e)
	}
	paths := []string{}
	for _, f := range ref.Files {
		p := filepath.Join(dir, f.Name)
		raw, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Fatal("source changed", f.Name)
		}
		paths = append(paths, p)
	}
	paths = append(paths, filepath.Join(dir, "repertoires.airr.json"))
	res, e := reads.Open(context.Background(), paths)
	if e != nil {
		t.Fatal(e)
	}
	sources := []bcell.FamilySource{}
	for _, s := range res.Sources {
		sources = append(sources, bcell.FamilySource{Name: s.Name, SHA256: s.SHA256})
	}
	subjects := map[string]string{}
	for _, r := range res.Repertoires {
		subjects[r.RepertoireID] = r.SubjectID
	}
	for _, c := range ref.Cases {
		t.Run(fmt.Sprintf("%s-%g", c.Mode, c.Threshold), func(t *testing.T) {
			r, e := bcell.Families(context.Background(), res.Table, nil, sources, subjects, bcell.FamilyOptions{Mode: c.Mode, Threshold: &c.Threshold}, nil)
			if e != nil {
				t.Fatal(e)
			}
			if r.Eligible != ref.Observations || len(r.Families) != c.Families || len(r.Representatives) != c.Representatives || r.Comparisons != c.Comparisons {
				t.Fatalf("counts: eligible %d, families %d, receptors %d, comparisons %d", r.Eligible, len(r.Families), len(r.Representatives), r.Comparisons)
			}
			nearest := map[string]string{}
			for _, p := range r.Representatives {
				n := "NA"
				if p.Nearest != nil {
					n = fmt.Sprintf("%016x", math.Float64bits(*p.Nearest))
				}
				nearest[p.ID] = n
			}
			groups := map[string][]string{}
			distances := []string{}
			for _, o := range r.Observations {
				name := o.Sample + ":" + o.Cell
				groups[o.Family] = append(groups[o.Family], name)
				distances = append(distances, name+"\t"+nearest[o.Representative])
			}
			lines := []string{}
			for _, g := range groups {
				sort.Strings(g)
				lines = append(lines, strings.Join(g, "\t"))
			}
			if familyDigest(lines) != c.MembershipSHA256 || familyDigest(distances) != c.NearestSHA256 {
				t.Fatal("membership or nearest distance differs from independent CSV graph")
			}
		})
	}
}

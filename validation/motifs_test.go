package validation

import (
	"clonoloom/internal/motifs"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"testing"
)

func TestMotifsIndependentReference(t *testing.T) {
	raw, err := os.ReadFile("reference/motifs.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Records []struct{ V, Junction, Arm string }
		Groups  []struct {
			Kind, V, Motif, Allowed                         string
			Length, Start, End, QueryTotal, BackgroundTotal int
			Query, Background                               []string
			P, Q                                            *float64
		}
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	q, b := []motifs.Record{}, []motifs.Record{}
	for _, x := range ref.Records {
		r := motifs.Record{ID: motifs.ID(x.V, x.Junction), V: x.V, Junction: x.Junction, Abundance: 1}
		if x.Arm == "query" {
			q = append(q, r)
		} else {
			b = append(b, r)
		}
	}
	r, err := motifs.Compute(context.Background(), q, b, motifs.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Groups) != len(ref.Groups) {
		t.Fatal(len(r.Groups), len(ref.Groups))
	}
	key := func(kind, v, motif, allowed string, length, start, end int) string {
		return fmt.Sprintf("%s/%s/%d/%s/%s/%d/%d", kind, v, length, motif, allowed, start, end)
	}
	got := map[string]motifs.Group{}
	for _, g := range r.Groups {
		got[key(g.Kind, g.V, g.Motif, g.Allowed, g.Length, g.Start, g.End)] = g
	}
	members := func(ids []int, rows []motifs.Record) string {
		ss := []string{}
		for _, i := range ids {
			ss = append(ss, rows[i].Junction)
		}
		sort.Strings(ss)
		return fmt.Sprint(ss)
	}
	for _, w := range ref.Groups {
		g, ok := got[key(w.Kind, w.V, w.Motif, w.Allowed, w.Length, w.Start, w.End)]
		if !ok {
			t.Fatal("signature absent", w)
		}
		if g.QueryTotal != w.QueryTotal || g.BackgroundTotal != w.BackgroundTotal || members(g.Query, q) != fmt.Sprint(w.Query) || members(g.Background, b) != fmt.Sprint(w.Background) {
			t.Fatal("membership mismatch", g, w)
		}
		for _, x := range []struct {
			v float64
			w *float64
		}{{float64(g.P), w.P}, {float64(g.Q), w.Q}} {
			if x.w == nil {
				if !math.IsNaN(x.v) {
					t.Fatal(g)
				}
			} else if math.IsNaN(x.v) || math.Abs(x.v-*x.w) > 1e-11 {
				t.Fatal("score mismatch", g, w)
			}
		}
		p := motifs.Residues(r, g)
		for pos, counts := range p.Counts {
			sum := 0
			frequency := 0.0
			for a, n := range counts {
				sum += n
				frequency += p.Frequencies[pos][a]
			}
			if sum != len(g.Query) || sum > 0 && math.Abs(frequency-1) > 1e-14 {
				t.Fatal("profile denominator")
			}
		}
	}
	data, _ := json.Marshal(r)
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest != "703b07637413bdaa702d939674feef26fed4235001d38e20ed4e77528e218b91" {
		t.Fatalf("motif result digest %s", digest)
	}
	t.Logf("motif result digest %s; %d signatures", digest, r.Family)
}

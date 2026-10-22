package simulate

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/random"
)

// A small series with a response beside an infusion product: two epitopes the subjects respond to
// and one only the database holds.
func smallResponse() Study {
	s := smallSeries()
	s.Name, s.Title = "small-response", "A small simulated series with a response"
	ser := *s.Series
	ser.Response = &Response{
		Epitopes: []Epitope{
			{Peptide: "KLDTEVYNA", Antigen: "test antigen", Weight: 0.7, Founders: 4},
			{Peptide: "RPQGFSWLV", Antigen: "test antigen", Weight: 0.3, Founders: 3},
			{Peptide: "GHSKLWDTF", Antigen: "unrelated antigen", Founders: 2},
		},
		Family:   Family{Members: 4, Substitutions: 1, Similarity: 0.5, Private: 1, Alpha: 1.3, MaxSize: 100},
		Shares:   map[string][]float64{"control": {0.001, 0.05, 0.01}, "expanded": {0.001, 0.001, 0.001}},
		Database: 2,
	}
	s.Series = &ser
	return s
}

// A family's members are its founder's genes and deletions with other insertions: their CDR3s
// differ from the founder's at the residues they count, only where the insertions made a codon.
func TestFamiliesDifferFromTheirFoundersByTheirInsertions(t *testing.T) {
	resp := smallResponse().Series.Response
	g := NewGenerator(&humanTRB, DefaultRecombination())
	fs := founders(g, resp, random.New(1, 8000))
	if len(fs) != 9 {
		t.Fatalf("%d founders", len(fs))
	}
	members := families(g, fs, resp, map[string]bool{}, "S01", random.New(1, 9000))
	keys := map[string]bool{}
	private, changed := 0, 0
	for _, m := range members {
		f := m.founder
		key := rearrangementKey(&m.Rearrangement)
		if keys[key] || !m.Productive || !(resp.Epitopes[f.epitope].Weight > 0) {
			t.Fatalf("%s: a member twice, not productive, or of an epitope no one responds to", key)
		}
		keys[key] = true
		if f.private {
			private++
			if m.substitutions != 0 || m.JunctionAA != f.x.JunctionAA {
				t.Errorf("a private receptor %s differs from itself", key)
			}
			continue
		}
		x := m.Rearrangement
		if x.V != f.x.V || x.D != f.x.D || x.J != f.x.J || x.DelV != f.x.DelV || x.DelD5 != f.x.DelD5 || x.DelD3 != f.x.DelD3 || x.DelJ != f.x.DelJ ||
			len(x.N1) != len(f.x.N1) || len(x.N2) != len(f.x.N2) {
			t.Fatalf("%s is not %s's genes and deletions", key, f.name)
		}
		again := x
		again.assemble()
		if again.Junction != x.Junction || again.Sequence != x.Sequence {
			t.Fatalf("%s: its insertions do not make its junction", key)
		}
		differ := 0
		for c := range x.JunctionAA {
			if x.JunctionAA[c] == f.x.JunctionAA[c] {
				continue
			}
			differ++
			free := false
			for _, k := range f.free {
				free = free || k == c
			}
			if !free {
				t.Errorf("%s: residue %d changed, not one the insertions made", key, c)
			}
		}
		if differ != m.substitutions || len(x.JunctionAA) != len(f.x.JunctionAA) {
			t.Errorf("%s: %d residues differ from %s, %d counted", key, differ, f.x.JunctionAA, m.substitutions)
		}
		changed += differ
	}
	if private == 0 || changed == 0 || len(members) < 10 {
		t.Errorf("%d members, %d private, %d residues changed", len(members), private, changed)
	}
}

// The truth gives each clonotype's epitope, or none: the series file's epitope where the
// specificity file has the clonotype, nowhere else; the database holds every founder, each
// receptor of an epitope once.
func TestTheResponseTruth(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "response")
	s := smallResponse()
	if _, err := Run(context.Background(), s, dir, "test", nil); err != nil {
		t.Fatal(err)
	}
	specific := map[string]string{}
	for _, row := range readTSV(t, filepath.Join(dir, "truth", "specificity.tsv")) {
		key := row["subject"] + "|" + row["v_call"] + "|" + row["j_call"] + "|" + row["junction_aa"]
		if specific[key] != "" {
			t.Fatalf("%s twice", key)
		}
		specific[key] = row["epitope"]
	}
	seen := 0
	for _, subject := range []string{"S01", "S02"} {
		for _, row := range readTSV(t, filepath.Join(dir, "truth", subject+".series.tsv.gz")) {
			key := subject + "|" + row["v_call"] + "|" + row["j_call"] + "|" + row["junction_aa"]
			if row["epitope"] != specific[key] || (row["epitope"] != "") != strings.Contains(row["trajectory"], "response") {
				t.Fatalf("%s: epitope %q, trajectory %q, the specificity file's %q", key, row["epitope"], row["trajectory"], specific[key])
			}
			if row["epitope"] != "" {
				seen++
			}
		}
	}
	if seen != len(specific) || seen == 0 {
		t.Errorf("%d of %d receptors that recognize an epitope in the series files", seen, len(specific))
	}
	founders := readTSV(t, filepath.Join(dir, "truth", "founders.tsv"))
	database := readTSV(t, filepath.Join(dir, "database", "vdjdb.txt"))
	if len(founders) != 9 || len(database) <= len(founders) {
		t.Fatalf("%d founders, %d receptors in the database", len(founders), len(database))
	}
	held := map[string]string{}
	for _, row := range database {
		key := row["antigen.epitope"] + "|" + row["v.segm"] + "|" + row["j.segm"] + "|" + row["cdr3"]
		if held[key] != "" || row["gene"] != "TRB" || row["species"] != "HomoSapiens" {
			t.Fatalf("%+v", row)
		}
		held[key] = row["vdjdb.score"]
	}
	for _, f := range founders {
		if held[f["epitope"]+"|"+f["v_call"]+"|"+f["j_call"]+"|"+f["junction_aa"]] != "3" {
			t.Errorf("founder %s is not in the database", f["founder"])
		}
	}
}

func TestTheSameResponseOnEveryProcessor(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "response")
	if _, err := Run(context.Background(), smallResponse(), dir, "test", nil); err != nil {
		t.Fatal(err)
	}
	if got := digest(t, dir); got != pinnedSmallResponse {
		t.Fatalf("the small response's digest is %s, pinned %s: if the simulator was meant to change, pin the new one, made on both arm64 and amd64", got, pinnedSmallResponse)
	}
}

const pinnedSmallResponse = "8dda26bd5a46686c8493e264c491a04b977020ad0a21c486ef4e9f0951d0731b"

func TestResponseParametersAreChecked(t *testing.T) {
	for name, change := range map[string]func(r *Response){
		"no epitopes":         func(r *Response) { r.Epitopes = nil },
		"a peptide twice":     func(r *Response) { r.Epitopes[1].Peptide = r.Epitopes[0].Peptide },
		"not a peptide":       func(r *Response) { r.Epitopes[0].Peptide = "KLD1EVYNA" },
		"no founders":         func(r *Response) { r.Epitopes[0].Founders = 0 },
		"no antigen":          func(r *Response) { r.Epitopes[0].Antigen = "" },
		"no weight":           func(r *Response) { r.Epitopes[0].Weight, r.Epitopes[1].Weight = 0, 0 },
		"no members":          func(r *Response) { r.Family.Members = 0 },
		"negative private":    func(r *Response) { r.Family.Private = -1 },
		"no largest size":     func(r *Response) { r.Family.MaxSize = 0 },
		"a share missing":     func(r *Response) { delete(r.Shares, "control") },
		"too few shares":      func(r *Response) { r.Shares["control"] = []float64{0.001} },
		"above the reference": func(r *Response) { r.Shares["control"][1] = 0.8 },
		"a negative database": func(r *Response) { r.Database = -1 },
	} {
		s := smallResponse()
		resp := *s.Series.Response
		resp.Epitopes = append([]Epitope(nil), resp.Epitopes...)
		resp.Shares = map[string][]float64{"control": {0.001, 0.05, 0.01}, "expanded": {0.001, 0.001, 0.001}}
		change(&resp)
		s.Series.Response = &resp
		if err := s.Check(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := smallResponse().Check(); err != nil {
		t.Errorf("the small response: %v", err)
	}
	if err := Presets["vaccine"].Check(); err != nil {
		t.Errorf("the vaccine study: %v", err)
	}
}

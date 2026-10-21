package simulate

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"clonoloom/internal/random"
)

// The germline segments are what the model needs: every V's anchor a cysteine codon, every J's a
// phenylalanine's or tryptophan's in an F/W-G-X-G motif, and the usage of each kind summing to 1.
func TestTheGermlineSegments(t *testing.T) {
	if len(humanTRB.V) < 40 || len(humanTRB.J) != 13 || len(humanTRB.D) != 2 {
		t.Fatalf("%d V, %d D, %d J", len(humanTRB.V), len(humanTRB.D), len(humanTRB.J))
	}
	for _, kind := range [][]Segment{humanTRB.V, humanTRB.J} {
		sum := 0.0
		for _, s := range kind {
			sum += s.Usage
		}
		if math.Abs(sum-1) > 1e-4 {
			t.Errorf("usage sums to %v", sum)
		}
	}
	for _, v := range humanTRB.V {
		if c := v.Seq[v.Anchor : v.Anchor+3]; c != "TGT" && c != "TGC" {
			t.Errorf("%s: anchor %s", v.Name, c)
		}
	}
	for _, j := range humanTRB.J {
		if aa := Translate(j.Seq[j.Anchor:]); !(aa[0] == 'F' || aa[0] == 'W') || aa[1] != 'G' || aa[3] != 'G' {
			t.Errorf("%s: %s", j.Name, aa)
		}
	}
}

func TestPalindromesAndDeletions(t *testing.T) {
	for _, c := range []struct {
		got, want string
	}{
		{trim3("ACGTTG", 2), "ACGT"}, {trim3("ACGTTG", -2), "ACGTTGCA"}, {trim5("ACGTTG", 2), "GTTG"},
		{trim5("ACGTTG", -2), "GTACGTTG"}, {trim3("A", -3), "AT"}, {trim5("", -2), ""},
		{reverseComplement("AACG"), "CGTT"}, {Translate("TGTGCCAGCAGTTTT"), "CASSF"},
	} {
		if c.got != c.want {
			t.Errorf("got %s, want %s", c.got, c.want)
		}
	}
}

// Every rearrangement is what it says: its junction is in its sequence, from its V's cysteine to
// its J's phenylalanine (or tryptophan), and a productive one is in frame, without a stop codon.
func TestRearrangementsAreWellFormed(t *testing.T) {
	g := NewGenerator(&humanTRB, DefaultRecombination())
	r := random.New(3, 1)
	productive := 0
	for range 20000 {
		x := g.Draw(r)
		if x.Sequence[x.JunctionStart:x.JunctionStart+len(x.Junction)] != x.Junction {
			t.Fatal("the junction is not in the sequence")
		}
		if !strings.HasPrefix(x.Junction, x.V.Seq[x.V.Anchor:x.V.Anchor+3]) || !strings.HasSuffix(x.Junction, x.J.Seq[x.J.Anchor:x.J.Anchor+3]) {
			t.Fatalf("anchors: %s %s %s", x.V.Name, x.J.Name, x.Junction)
		}
		if !strings.HasPrefix(x.Sequence, x.V.Seq[max(0, x.V.Anchor-30):x.V.Anchor]) || !strings.HasSuffix(x.Sequence, x.J.Seq[x.J.Anchor:]) {
			t.Fatalf("flanks of %s", x.Sequence)
		}
		if strings.HasPrefix(x.J.Name, "TRBJ1") && x.D.Name != "TRBD1" {
			t.Fatalf("%s with %s", x.J.Name, x.D.Name)
		}
		if x.Productive {
			productive++
			aa := x.JunctionAA
			if len(x.Junction)%3 != 0 || aa != Translate(x.Junction) || strings.Contains(aa, "*") || aa[0] != 'C' || (aa[len(aa)-1] != 'F' && aa[len(aa)-1] != 'W') {
				t.Fatalf("productive %s", aa)
			}
		} else if x.InFrame && !x.Stop {
			t.Fatal("nonproductive without a reason")
		}
	}
	if productive < 1000 {
		t.Errorf("%d productive", productive)
	}
}

// The productive junctions' lengths are the open data's (selection makes them so), and the genes
// are used as the model says.
func TestJunctionLengthsAndUsageMatchTheirTargets(t *testing.T) {
	g := NewGenerator(&humanTRB, DefaultRecombination())
	r := random.New(11, 2)
	var n, sum, sum2 float64
	vCount := map[string]float64{}
	all := 0.0
	for range 150000 {
		x := g.Draw(r)
		vCount[x.V.Name]++
		all++
		if x.Productive {
			l := float64(len(x.JunctionAA))
			n, sum, sum2 = n+1, sum+l, sum2+l*l
		}
	}
	var tn, tsum, tsum2 float64
	for l, c := range humanTRB.JunctionLengths {
		tn, tsum, tsum2 = tn+float64(c), tsum+float64(l*c), tsum2+float64(l*l*c)
	}
	mean, tmean := sum/n, tsum/tn
	sd, tsd := math.Sqrt(sum2/n-mean*mean), math.Sqrt(tsum2/tn-tmean*tmean)
	if math.Abs(mean-tmean) > 0.1 || math.Abs(sd-tsd) > 0.15 {
		t.Errorf("junction lengths: mean %.2f sd %.2f; the data's %.2f and %.2f", mean, sd, tmean, tsd)
	}
	// V usage over every rearrangement drawn: within 0.03 of the model in total variation (the
	// selection of productive ones on length moves it a little).
	tv := 0.0
	for _, v := range humanTRB.V {
		tv += math.Abs(vCount[v.Name]/all - v.Usage)
	}
	if tv/2 > 0.03 {
		t.Errorf("V usage: total variation %.3f", tv/2)
	}
}

// A sample shows, on average, the clonotypes the truth expects of its depth.
func TestSamplingMatchesItsExpectation(t *testing.T) {
	g := NewGenerator(&humanTRB, DefaultRecombination())
	study := Presets["small"]
	subject := population(g, study.Groups[0], "S01", random.New(5, 5))
	pop := populationTruth(subject)
	var observed, expected []float64
	for rep := range 60 {
		seen, st := sampled(subject, study, 1500, random.New(9, uint64(rep)))
		clonotypes := map[string]bool{}
		for i := range seen {
			if c := &subject.Clones[i]; c.Productive {
				clonotypes[clonotypeKey(c)] = true
			}
		}
		e, _ := expectedAt(pop.frequencies, st.ProductiveTemplates)
		observed = append(observed, float64(len(clonotypes)))
		expected = append(expected, e)
	}
	var diff, diff2 float64
	for i := range observed {
		d := observed[i] - expected[i]
		diff, diff2 = diff+d, diff2+d*d
	}
	n := float64(len(observed))
	mean := diff / n
	se := math.Sqrt((diff2/n - mean*mean) / (n - 1))
	if math.Abs(mean) > 4*se+0.5 {
		t.Errorf("observed − expected clonotypes: %.2f (SE %.2f)", mean, se)
	}
}

// The truth is the data's: every row of a sample is a clone of its subject's population with the
// same rearrangement, its clonotypes are the truth's count, and the truth's numbers are those of
// its populations' tables. The same seed makes the same files.
func TestTheTruthIsTheData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "study")
	study := Presets["small"]
	summary, err := Run(context.Background(), study, dir, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Subjects != 2 || summary.Samples != 2 {
		t.Fatalf("%+v", summary)
	}
	var truth studyTruth
	raw, _ := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatal(err)
	}
	clones := map[string][]string{}
	for _, s := range truth.Subjects {
		rows := readTSV(t, filepath.Join(dir, "truth", s.ID+".tsv.gz"))
		cells := map[string]float64{}
		total := 0.0
		for _, row := range rows {
			clones[row["clone_id"]] = []string{row["v_call"], row["j_call"], row["junction"], row["productive"]}
			if row["productive"] == "T" {
				n, _ := strconv.ParseFloat(row["cells"], 64)
				cells[row["v_call"]+"|"+row["j_call"]+"|"+row["junction_aa"]] += n
				total += n
			}
		}
		simpson := 0.0
		for _, c := range cells {
			simpson += (c / total) * (c / total)
		}
		if len(cells) != s.Clonotypes || math.Abs(simpson-s.Simpson) > 1e-12 || math.Abs(s.Hill2-1/simpson) > 1e-6 {
			t.Errorf("%s: %d clonotypes (truth %d), Simpson %v (truth %v)", s.ID, len(cells), s.Clonotypes, simpson, s.Simpson)
		}
	}
	for _, st := range truth.Samples {
		rows := readTSV(t, filepath.Join(dir, st.ID+".tsv"))
		clonotypes := map[string]bool{}
		templates := 0
		for _, row := range rows {
			clone := clones[st.Subject+"-"+strings.TrimPrefix(row["sequence_id"], st.ID+"-")]
			if clone == nil || clone[0] != row["v_call"] || clone[1] != row["j_call"] || clone[2] != row["junction"] || clone[3] != row["productive"] {
				t.Fatalf("%s is not its clone %v", row["sequence_id"], clone)
			}
			n, _ := strconv.Atoi(row["umi_count"])
			templates += n
			if row["productive"] == "T" {
				clonotypes[row["v_call"]+"|"+row["j_call"]+"|"+row["junction_aa"]] = true
			}
		}
		if len(rows) != st.Rows || templates != st.TemplatesSeen || len(clonotypes) != st.Clonotypes {
			t.Errorf("%s: %d rows, %d templates, %d clonotypes; truth %d, %d, %d", st.ID, len(rows), templates, len(clonotypes), st.Rows, st.TemplatesSeen, st.Clonotypes)
		}
	}
	// The same seed, the same bytes; another seed, another study.
	again := filepath.Join(t.TempDir(), "again")
	if _, err := Run(context.Background(), study, again, "test", nil); err != nil {
		t.Fatal(err)
	}
	if a, b := digest(t, dir), digest(t, again); a != b {
		t.Error("the same seed made different files")
	}
	other := study
	other.Seed++
	third := filepath.Join(t.TempDir(), "other")
	if _, err := Run(context.Background(), other, third, "test", nil); err != nil {
		t.Fatal(err)
	}
	if digest(t, dir) == digest(t, third) {
		t.Error("another seed made the same files")
	}
	if _, err := Run(context.Background(), study, dir, "test", nil); err == nil {
		t.Error("a folder that is not empty was written into")
	}
}

func TestStudiesAreChecked(t *testing.T) {
	for name, s := range Presets {
		if err := s.Check(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad := Presets["small"]
	bad.Groups = append([]Group(nil), bad.Groups...)
	bad.Groups[1].Population.Expanded = []float64{0.6, 0.5}
	if bad.Check() == nil {
		t.Error("expanded shares of more than 1 accepted")
	}
}

func readTSV(t *testing.T, path string) []map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var scanner *bufio.Scanner
	if strings.HasSuffix(path, ".gz") {
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		scanner = bufio.NewScanner(z)
	} else {
		scanner = bufio.NewScanner(f)
	}
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	var header []string
	var rows []map[string]string
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if header == nil {
			header = fields
			continue
		}
		row := map[string]string{}
		for i, h := range header {
			row[h] = fields[i]
		}
		rows = append(rows, row)
	}
	return rows
}

// digest is a folder's files and their contents, as one SHA-256.
func digest(t *testing.T, dir string) string {
	t.Helper()
	var names []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		data, _ := os.ReadFile(name)
		rel, _ := filepath.Rel(dir, name)
		h.Write([]byte(rel + "\n"))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

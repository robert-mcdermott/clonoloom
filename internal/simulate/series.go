package simulate

// Simulated studies over time (clonoloom-spec/roadmap.md, wave 3, slice 4): each subject's
// repertoire sampled at several time points from its reference event (an infusion, a transplant),
// its clones' frequencies following trajectories whose every fold is known, and, where a study has
// one, a reference population mixed into the blood in a known share at each time: an infusion
// product, made from the subject's own T cells and expanded ex vivo, or a donor's repertoire. Every
// sample, replicates at one time included, draws each clone's frequency around its expectation
// with the biological variance between samples, as Rytlewski et al. 2019 measured between blood
// samples (PLoS One 14:e0213684): two samples of one subject differ by more than sampling. A
// series may have an antigen-specific response too (response.go): families of receptors that
// recognize an epitope, a known share of the blood at each time point.

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/dmath"
	"clonoloom/internal/random"
)

// Series is a study over time: each subject sampled at Times (with their replicates), the event
// day 0 is (Event), the variance of each clone's frequency between samples around its expectation
// (Variance: of a gamma of mean 1 that multiplies it), the trajectories of classes of the
// subject's own clones, a reference population, if any, and an antigen-specific response, if any.
type Series struct {
	Event        string       `json:"event"`
	Times        []TimePoint  `json:"times"`
	Variance     float64      `json:"variance"`
	Trajectories []Trajectory `json:"trajectories,omitempty"`
	Reference    *Reference   `json:"reference,omitempty"`
	Response     *Response    `json:"response,omitempty"`
}

// TimePoint is a day from the event and how many samples are taken then (replicates).
type TimePoint struct {
	Day        float64 `json:"day"`
	Replicates int     `json:"replicates"`
}

// Trajectory is a class of clones: Share of the productive clones, chosen at random (of the
// subjects of Conditions only, when given), each of whose cells is multiplied by Folds at the time
// points (one each).
type Trajectory struct {
	Name       string    `json:"name"`
	Share      float64   `json:"share"`
	Folds      []float64 `json:"folds"`
	Conditions []string  `json:"conditions,omitempty"`
}

// Reference is a population mixed into the subject's blood: a product (Cells of the subject's
// blood collected, each clone then expanded ex vivo by a log-normal fold of spread Expansion) or a
// donor (a repertoire of its own, Population). Its share of the blood's cells at each time point is
// Shares' by the subject's condition; Trajectories are classes of its clones. It is sampled once
// (Templates), as the subject's reference sample, labeled Label (its tissue in the metadata).
type Reference struct {
	Kind         string               `json:"kind"`
	Label        string               `json:"label"`
	Cells        int                  `json:"cells,omitempty"`
	Expansion    float64              `json:"expansion,omitempty"`
	Population   *Population          `json:"population,omitempty"`
	Templates    int                  `json:"templates"`
	Shares       map[string][]float64 `json:"shares"`
	Trajectories []Trajectory         `json:"trajectories,omitempty"`
}

func (ser *Series) check(s Study) error {
	if len(ser.Times) == 0 {
		return fmt.Errorf("a series needs time points")
	}
	for i, t := range ser.Times {
		if t.Replicates < 1 || (i > 0 && t.Day <= ser.Times[i-1].Day) {
			return fmt.Errorf("a series' time points must be in order, each with a replicate or more")
		}
	}
	if !(ser.Variance >= 0) {
		return fmt.Errorf("a series' variance must be zero or more")
	}
	trajectories := func(ts []Trajectory) error {
		total := 0.0
		for _, tr := range ts {
			if len(tr.Folds) != len(ser.Times) || tr.Share < 0 {
				return fmt.Errorf("trajectory %q needs a fold for each time point and a share of zero or more", tr.Name)
			}
			for _, f := range tr.Folds {
				if !(f >= 0) || math.IsInf(f, 0) {
					return fmt.Errorf("trajectory %q: folds must be zero or more", tr.Name)
				}
			}
			total += tr.Share
		}
		if total > 1 {
			return fmt.Errorf("the trajectories' shares must sum to at most 1")
		}
		return nil
	}
	if err := trajectories(ser.Trajectories); err != nil {
		return err
	}
	if ref := ser.Reference; ref != nil {
		switch {
		case ref.Kind == "product" && (ref.Cells < 1 || ref.Expansion < 0):
			return fmt.Errorf("a product needs the cells collected and an expansion of zero or more")
		case ref.Kind == "donor" && (ref.Population == nil || ref.Population.Clonotypes < 1 || ref.Population.Alpha <= 0 || ref.Population.MaxSize < 1):
			return fmt.Errorf("a donor needs a population")
		case ref.Kind != "product" && ref.Kind != "donor":
			return fmt.Errorf("a reference is a product or a donor")
		case ref.Templates < 1:
			return fmt.Errorf("the reference sample needs templates")
		}
		for _, g := range s.Groups {
			shares := ref.Shares[g.Condition]
			if len(shares) != len(ser.Times) {
				return fmt.Errorf("the reference needs a share at each time point for condition %q", g.Condition)
			}
			for _, x := range shares {
				if !(x >= 0 && x <= 1) {
					return fmt.Errorf("the reference's shares must be from 0 to 1")
				}
			}
		}
		if err := trajectories(ref.Trajectories); err != nil {
			return err
		}
	}
	if ser.Response != nil {
		return ser.Response.check(s, ser)
	}
	return nil
}

// part is a population as mixed into a sample: entries, each a row of the sample's clones (an
// index into them: a product's clones are the subject's own) with its cells and its trajectory's
// index (-1: none).
type part struct {
	row   []int
	cells []float64
	class []int
}

func ownPart(clones []Clone) part {
	p := part{row: make([]int, len(clones)), cells: make([]float64, len(clones)), class: make([]int, len(clones))}
	for i, c := range clones {
		p.row[i], p.cells[i], p.class[i] = i, float64(c.Size), -1
	}
	return p
}

// assign gives each trajectory its share of the part's productive entries, chosen at random among
// those not yet given one.
func (p *part) assign(trajectories []Trajectory, condition string, productive func(row int) bool, r *random.Random) map[string]int {
	counts := map[string]int{}
	var free []int
	for i, row := range p.row {
		if productive(row) {
			free = append(free, i)
		}
	}
	n := len(free)
	for t, tr := range trajectories {
		if len(tr.Conditions) > 0 && !contains(tr.Conditions, condition) {
			continue
		}
		want := int(math.Round(float64(tr.Share * float64(n))))
		for k := 0; k < want && len(free) > 0; k++ {
			j := r.IntN(len(free))
			p.class[free[j]] = t
			free[j] = free[len(free)-1]
			free = free[:len(free)-1]
		}
		counts[tr.Name] = want
	}
	return counts
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// weights are the part's cells at time point t (its trajectories' folds applied), and their sum.
func (p part) weights(trajectories []Trajectory, t int) ([]float64, float64) {
	w := make([]float64, len(p.cells))
	total := 0.0
	for i, c := range p.cells {
		if k := p.class[i]; k >= 0 {
			c = float64(c * trajectories[k].Folds[t])
		}
		w[i] = c
		total += c
	}
	return w, total
}

// SeriesTruth is a subject's series: its reference population, how many of its clones (and the
// reference's) each trajectory took, its receptors that recognize the response's epitopes, and the
// file that holds every clonotype's expected frequency at each time point.
type SeriesTruth struct {
	Subject   string          `json:"subject"`
	Reference *ReferenceTruth `json:"reference,omitempty"`
	Classes   map[string]int  `json:"classes,omitempty"`
	Response  *ResponseTruth  `json:"response,omitempty"`
	File      string          `json:"file"`
}

// ReferenceTruth is a subject's reference population: its clones and cells, and its productive
// clonotypes.
type ReferenceTruth struct {
	Kind       string         `json:"kind"`
	Label      string         `json:"label"`
	Clones     int            `json:"clones"`
	Cells      float64        `json:"cells"`
	Clonotypes int            `json:"clonotypes"`
	Classes    map[string]int `json:"classes,omitempty"`
}

// runSeries makes a study over time (Run, with s.Series): each subject's own population as Run
// makes it, its reference population, its families of receptors that recognize the response's
// epitopes, and its samples at each time point and its reference sample, with the truth.
func runSeries(ctx context.Context, s Study, dir, version string, progress Progress) (Summary, error) {
	ser := s.Series
	g := NewGenerator(&humanTRB, s.Recombination)
	var subjects []Subject
	n := 0
	for _, group := range s.Groups {
		n += group.Subjects
	}
	steps := float64(2 * n)
	for _, group := range s.Groups {
		for range group.Subjects {
			if err := ctx.Err(); err != nil {
				return Summary{}, err
			}
			id := fmt.Sprintf("S%02d", len(subjects)+1)
			progress(float64(len(subjects))/steps, "Recombining "+id+"'s repertoire")
			subjects = append(subjects, population(g, group, id, random.New(s.Seed, uint64(1000+len(subjects)))))
		}
	}
	summary := Summary{Dir: dir, Subjects: len(subjects)}
	truth := studyTruth{Simulator: "ClonoLoom simulate " + version, Study: s, Germline: humanTRB.Source}
	// The response's founders, the same in every subject.
	var fs []founder
	var specific []specificRow
	if ser.Response != nil {
		fs = founders(g, ser.Response, random.New(s.Seed, 8000))
	}
	var repertoires []map[string]any
	k := 0
	for si, subject := range subjects {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		progress(float64(n+si)/steps, "Sampling "+subject.ID+" over time")
		r := random.New(s.Seed, uint64(3000+si))
		pop := populationTruth(subject)
		truth.Subjects = append(truth.Subjects, pop.subject)
		st := SeriesTruth{Subject: subject.ID, File: filepath.Join("truth", subject.ID+".series.tsv.gz")}
		// The sample's clones: the subject's own, then a donor's.
		clones := append([]Clone(nil), subject.Clones...)
		productive := func(row int) bool { return clones[row].Productive }
		own := ownPart(subject.Clones)
		st.Classes = own.assign(ser.Trajectories, subject.Condition, productive, r)
		var ref *part
		var shares []float64
		if rs := ser.Reference; rs != nil {
			shares = rs.Shares[subject.Condition]
			p := part{}
			switch rs.Kind {
			case "product":
				// The cells collected from the subject's blood, each clone then expanded ex vivo.
				collected := r.Multinomial(rs.Cells, own.cells, nil)
				for row, c := range collected {
					if c > 0 {
						p.row = append(p.row, row)
						p.cells = append(p.cells, float64(float64(c)*dmath.Exp(float64(rs.Expansion*r.Normal()))))
						p.class = append(p.class, -1)
					}
				}
			case "donor":
				donor := population(g, Group{Condition: "donor", Subjects: 1, Population: *rs.Population}, subject.ID+"D", random.New(s.Seed, uint64(5000+si)))
				for i, c := range donor.Clones {
					p.row = append(p.row, len(clones)+i)
					p.cells = append(p.cells, float64(c.Size))
					p.class = append(p.class, -1)
				}
				clones = append(clones, donor.Clones...)
			}
			rt := &ReferenceTruth{Kind: rs.Kind, Label: rs.Label, Clones: len(p.row)}
			rt.Classes = p.assign(rs.Trajectories, subject.Condition, productive, r)
			keys := map[string]bool{}
			for i, row := range p.row {
				rt.Cells += p.cells[i]
				if clones[row].Productive {
					keys[clonotypeKey(&clones[row])] = true
				}
			}
			rt.Clonotypes = len(keys)
			st.Reference = rt
			ref = &p
		}
		// The subject's families, each member a clonotype its blood does not already hold.
		var resp *part
		var respShares []float64
		epitopeOf := map[string]string{}
		if rp := ser.Response; rp != nil {
			respShares = rp.Shares[subject.Condition]
			taken := map[string]bool{}
			for i := range clones {
				if clones[i].Productive {
					taken[clonotypeKey(&clones[i])] = true
				}
			}
			members := families(g, fs, rp, taken, subject.ID, random.New(s.Seed, uint64(9000+si)))
			p := responsePart(members, rp, len(clones))
			for _, m := range members {
				clones = append(clones, m.Clone)
				epitopeOf[rearrangementKey(&m.Rearrangement)] = rp.Epitopes[m.founder.epitope].Peptide
				specific = append(specific, specificRow{subject: subject.ID, m: m})
			}
			st.Response = responseTruth(members, rp)
			resp = &p
		}
		respShare := func(t int) float64 {
			if resp == nil {
				return 0
			}
			return respShares[t]
		}
		whole := Subject{ID: subject.ID, Condition: subject.Condition, Clones: clones}
		// Each time point's expected productive frequencies by clonotype, for the truth.
		expected := make([]map[string]float64, len(ser.Times))
		for t := range ser.Times {
			share := 0.0
			if ref != nil {
				share = shares[t]
			}
			expected[t] = expectedFrequencies(clones, own, ser.Trajectories, ref, refTrajectories(ser), share, resp, respShare(t), t)
		}
		refKeys := map[string]bool{}
		if ref != nil {
			for _, row := range ref.row {
				if clones[row].Productive {
					refKeys[clonotypeKey(&clones[row])] = true
				}
			}
		}
		// The samples over time.
		for t, tp := range ser.Times {
			share := 0.0
			if ref != nil {
				share = shares[t]
			}
			for rep := 1; rep <= tp.Replicates; rep++ {
				id := subject.ID + "-" + dayName(tp.Day)
				if tp.Replicates > 1 {
					id += "-" + strconv.Itoa(rep)
				}
				weights := sampleWeights(len(clones), own, ser.Trajectories, ref, refTrajectories(ser), share, resp, respShare(t), t, ser.Variance, random.New(s.Seed, uint64(6000+k)))
				seen, sample := drawSample(clones, s, s.Templates[k%len(s.Templates)], weights, random.New(s.Seed, uint64(7000+k)))
				k++
				sample.ID, sample.Subject = id, subject.ID
				day := tp.Day
				sample.Day, sample.Replicate = &day, rep
				if ref != nil {
					x := share
					sample.ReferenceShare = &x
					a := 0.0
					for _, key := range sortedKeys(expected[t]) {
						if refKeys[key] {
							a += expected[t][key]
						}
					}
					sample.Attribution = &a
				}
				sample.ExpectedClonotypes, sample.ExpectedCoverage = expectedAt(sorted(expected[t]), sample.ProductiveTemplates)
				truth.Samples = append(truth.Samples, sample)
				name := id + ".tsv"
				if err := writeSample(filepath.Join(dir, name), id, whole, seen); err != nil {
					return Summary{}, err
				}
				summary.Files = append(summary.Files, name)
				summary.Rows += sample.Rows
				summary.Samples++
				repertoires = append(repertoires, seriesRepertoire(s, subject, id, name, version, &day, "blood"))
			}
		}
		// The reference sample, as it was before it was infused (or the donor, as given).
		if ref != nil {
			rs := ser.Reference
			id := subject.ID + "-" + rs.Kind
			weights := make([]float64, len(clones))
			r2 := random.New(s.Seed, uint64(6000+k))
			for i, row := range ref.row {
				weights[row] += noisy(ref.cells[i], ser.Variance, r2)
			}
			seen, sample := drawSample(clones, s, rs.Templates, weights, random.New(s.Seed, uint64(7000+k)))
			k++
			sample.ID, sample.Subject, sample.Reference = id, subject.ID, true
			truth.Samples = append(truth.Samples, sample)
			name := id + ".tsv"
			if err := writeSample(filepath.Join(dir, name), id, whole, seen); err != nil {
				return Summary{}, err
			}
			summary.Files = append(summary.Files, name)
			summary.Rows += sample.Rows
			summary.Samples++
			repertoires = append(repertoires, seriesRepertoire(s, subject, id, name, version, nil, rs.Label))
		}
		if err := writePopulation(filepath.Join(dir, "truth", subject.ID+".tsv.gz"), subject); err != nil {
			return Summary{}, err
		}
		if err := writeSeries(filepath.Join(dir, st.File), ser, clones, own, ref, refKeys, epitopeOf, expected); err != nil {
			return Summary{}, err
		}
		summary.Files = append(summary.Files, filepath.Join("truth", subject.ID+".tsv.gz"), st.File)
		truth.Series = append(truth.Series, st)
	}
	if rp := ser.Response; rp != nil {
		sp := &SpecificityTruth{Founders: filepath.Join("truth", "founders.tsv"), Clonotypes: filepath.Join("truth", "specificity.tsv"), Database: filepath.Join("database", "vdjdb.txt")}
		if err := writeFounders(filepath.Join(dir, sp.Founders), fs, rp); err != nil {
			return Summary{}, err
		}
		if err := writeSpecificity(filepath.Join(dir, sp.Clonotypes), specific, rp); err != nil {
			return Summary{}, err
		}
		rows, err := writeDatabase(filepath.Join(dir, sp.Database), s.Name, fs, rp, random.New(s.Seed, 8500))
		if err != nil {
			return Summary{}, err
		}
		sp.DatabaseRows = rows
		truth.Specificity = sp
		summary.Files = append(summary.Files, sp.Founders, sp.Clonotypes, sp.Database)
	}
	truth.Sharing = sharing(subjects)
	if err := writeJSON(filepath.Join(dir, "repertoires.airr.json"), map[string]any{
		"Info":       map[string]any{"title": "AIRR Data File", "description": s.Title + " (simulated by ClonoLoom " + version + ")"},
		"Repertoire": repertoires,
	}); err != nil {
		return Summary{}, err
	}
	if err := writeJSON(filepath.Join(dir, "truth", "truth.json"), truth); err != nil {
		return Summary{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme(s, summary, version)), 0o644); err != nil {
		return Summary{}, err
	}
	summary.Files = append(summary.Files, "repertoires.airr.json", filepath.Join("truth", "truth.json"), "README.md")
	sort.Strings(summary.Files)
	progress(1, "Done")
	return summary, nil
}

func refTrajectories(ser *Series) []Trajectory {
	if ser.Reference == nil {
		return nil
	}
	return ser.Reference.Trajectories
}

// dayName names a sample by its day: pre (before the event), d0, d7, d28.
func dayName(day float64) string {
	if day < 0 {
		return "pre"
	}
	return "d" + strconv.FormatFloat(day, 'f', -1, 64)
}

// noisy is cells times a gamma of mean 1 and the variance given (none when it is 0): the clone's
// cells in one sample's blood, around their expectation.
func noisy(cells, variance float64, r *random.Random) float64 {
	if variance <= 0 || cells <= 0 {
		return cells
	}
	shape := 1 / variance
	return float64(cells * float64(r.Gamma(shape)*variance))
}

// sampleWeights are each clone's cells in one sample's blood at time point t: the subject's own
// population, in the share 1 − share − respShare, the reference's, in share, and the response's,
// in respShare, each part's cells relative to its total, its trajectories' folds applied, and each
// entry varying around that by variance.
func sampleWeights(rows int, own part, ownTr []Trajectory, ref *part, refTr []Trajectory, share float64, resp *part, respShare float64, t int, variance float64, r *random.Random) []float64 {
	out := make([]float64, rows)
	w, total := own.weights(ownTr, t)
	for i, c := range w {
		if total > 0 {
			out[own.row[i]] += noisy(float64((1-share-respShare)*c)/total, variance, r)
		}
	}
	if ref != nil && share > 0 {
		w, total := ref.weights(refTr, t)
		for i, c := range w {
			if total > 0 {
				out[ref.row[i]] += noisy(float64(share*c)/total, variance, r)
			}
		}
	}
	if resp != nil && respShare > 0 {
		w, total := resp.weights(nil, t)
		for i, c := range w {
			if total > 0 {
				out[resp.row[i]] += noisy(float64(respShare*c)/total, variance, r)
			}
		}
	}
	return out
}

// drawSample draws a sample's templates from its blood (each clone in proportion to its weight)
// and each template's reads: the clones seen, with their templates and reads.
func drawSample(clones []Clone, s Study, templates int, weights []float64, r *random.Random) (map[int][2]int, SampleTruth) {
	counts := r.Multinomial(templates, weights, nil)
	seen := map[int][2]int{}
	truth := SampleTruth{Templates: templates}
	clonotypes := map[string]bool{}
	for i, t := range counts {
		for range t {
			reads := r.NegBinomial(s.ReadsPerTemplate, s.ReadsSize)
			if reads == 0 {
				continue
			}
			x := seen[i]
			seen[i] = [2]int{x[0] + 1, x[1] + reads}
			truth.TemplatesSeen++
			truth.Reads += reads
			if clones[i].Productive {
				truth.ProductiveTemplates++
				clonotypes[clonotypeKey(&clones[i])] = true
			}
		}
	}
	truth.Rows = len(seen)
	truth.Clonotypes = len(clonotypes)
	return seen, truth
}

// expectedFrequencies are the productive clonotypes' expected shares of a sample's productive
// cells at time point t (its blood without the variance between samples).
func expectedFrequencies(clones []Clone, own part, ownTr []Trajectory, ref *part, refTr []Trajectory, share float64, resp *part, respShare float64, t int) map[string]float64 {
	cells := map[string]float64{}
	add := func(p part, tr []Trajectory, weight float64) {
		w, total := p.weights(tr, t)
		if total <= 0 {
			return
		}
		for i, c := range w {
			if row := p.row[i]; clones[row].Productive {
				cells[clonotypeKey(&clones[row])] += float64(weight*c) / total
			}
		}
	}
	add(own, ownTr, 1-share-respShare)
	if ref != nil && share > 0 {
		add(*ref, refTr, share)
	}
	if resp != nil && respShare > 0 {
		add(*resp, nil, respShare)
	}
	total := 0.0
	for _, key := range sortedKeys(cells) {
		total += cells[key]
	}
	for key, c := range cells {
		cells[key] = c / total
	}
	return cells
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sorted is a map's values in its keys' order (sums over them the same every run).
func sorted(m map[string]float64) []float64 {
	out := make([]float64, 0, len(m))
	for _, k := range sortedKeys(m) {
		out = append(out, m[k])
	}
	return out
}

// writeSeries writes a subject's clonotypes over time: each productive clonotype, the trajectory
// its clones follow (own, and the reference's), whether the reference holds it and its share of
// the reference's productive cells, the epitope it recognizes (in a series with a response), and
// its expected share of the productive cells at each time.
func writeSeries(path string, ser *Series, clones []Clone, own part, ref *part, refKeys map[string]bool, epitopeOf map[string]string, expected []map[string]float64) error {
	classOf := map[string][]string{}
	note := func(p part, tr []Trajectory, prefix string) {
		for i, row := range p.row {
			if k := p.class[i]; k >= 0 && clones[row].Productive {
				key := clonotypeKey(&clones[row])
				name := prefix + tr[k].Name
				if !contains(classOf[key], name) {
					classOf[key] = append(classOf[key], name)
				}
			}
		}
	}
	note(own, ser.Trajectories, "")
	// The response's receptors follow its shares.
	for key := range epitopeOf {
		classOf[key] = append(classOf[key], "response")
	}
	refShare := map[string]float64{}
	if ref != nil {
		note(*ref, ser.Reference.Trajectories, ser.Reference.Kind+" ")
		total := 0.0
		for i, row := range ref.row {
			if clones[row].Productive {
				refShare[clonotypeKey(&clones[row])] += ref.cells[i]
				total += ref.cells[i]
			}
		}
		for key, c := range refShare {
			refShare[key] = c / total
		}
	}
	keys := map[string]bool{}
	for _, e := range expected {
		for key := range e {
			keys[key] = true
		}
	}
	for key := range refShare {
		keys[key] = true
	}
	all := make([]string, 0, len(keys))
	for key := range keys {
		all = append(all, key)
	}
	sort.Strings(all)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	z := gzip.NewWriter(f)
	w := bufio.NewWriter(z)
	header := []string{"v_call", "j_call", "junction_aa", "trajectory", "in_reference", "reference_frequency"}
	if ser.Response != nil {
		header = append(header, "epitope")
	}
	for _, tp := range ser.Times {
		header = append(header, "frequency_"+dayName(tp.Day))
	}
	w.WriteString(strings.Join(header, "\t") + "\n")
	num := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	for _, key := range all {
		parts := strings.SplitN(key, "|", 3)
		row := []string{parts[0], parts[1], parts[2], strings.Join(classOf[key], ","), flag(refKeys[key]), num(refShare[key])}
		if ser.Response != nil {
			row = append(row, epitopeOf[key])
		}
		for _, e := range expected {
			row = append(row, num(e[key]))
		}
		w.WriteString(strings.Join(row, "\t") + "\n")
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := z.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// seriesRepertoire is a sample's Repertoire metadata with its time point (AIRR's
// collection_time_point_relative, in days from the event) or its tissue (a reference sample's:
// infusion product, donor blood).
func seriesRepertoire(s Study, subject Subject, id, file, version string, day *float64, tissue string) map[string]any {
	r := repertoire(s, subject, id, file, version)
	sample := r["sample"].([]any)[0].(map[string]any)
	if tissue != "blood" {
		sample["tissue"] = map[string]any{"id": nil, "label": tissue}
	}
	if day != nil {
		sample["collection_time_point_relative"] = *day
		sample["collection_time_point_relative_unit"] = map[string]any{"id": "UO:0000033", "label": "day"}
		sample["collection_time_point_reference"] = s.Series.Event
	}
	return r
}

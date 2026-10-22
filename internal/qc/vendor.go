package qc

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/rep"
)

// Check is a number a vendor reported about a sample beside the same number ClonoLoom computed
// from the sample's rows: agreement says the files were read as the vendor wrote them.
type Check struct {
	Metric     string `json:"metric"`
	Reported   string `json:"reported"`
	Recomputed string `json:"recomputed"`
	// Agrees: true or false where the two can be compared; null where ClonoLoom can only report its
	// own number (it lacks what the vendor counted from, such as cells without contigs).
	Agrees *bool  `json:"agrees"`
	Note   string `json:"note,omitempty"`
}

func agree(b bool) *bool { return &b }

// CellRangerMetrics reads a Cell Ranger metrics_summary.csv: the vdj pipeline's one row under its
// header, or a multi run's long table (Category or Library or Sample, Library Type, Grouped By,
// Group Name, Metric Name, Metric Value), whose sample-level (not library-level) V(D)J rows are
// taken, keyed "VDJ T: name". Names are compared without regard to case.
func CellRangerMetrics(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > 16<<20 {
		return nil, fmt.Errorf("%s is larger than a metrics summary", path)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields, ok := splitCSVLine(line)
		if !ok {
			return nil, fmt.Errorf("%s: unbalanced quotes", path)
		}
		rows = append(rows, fields)
	}
	out := map[string]string{}
	if len(rows) == 0 {
		return out, nil
	}
	header := rows[0]
	name, value, kind, category := -1, -1, -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "metric name":
			name = i
		case "metric value":
			value = i
		case "library type":
			kind = i
		case "category", "library or sample":
			category = i
		}
	}
	if name >= 0 && value >= 0 {
		for _, r := range rows[1:] {
			if len(r) <= value || len(r) <= name {
				continue
			}
			if category >= 0 && strings.EqualFold(r[category], "Library") {
				continue
			}
			prefix := ""
			if kind >= 0 {
				prefix = r[kind] + ": "
			}
			out[strings.ToLower(prefix+r[name])] = r[value]
		}
		return out, nil
	}
	if len(rows) >= 2 {
		for i, h := range header {
			if i < len(rows[1]) {
				out[strings.ToLower(h)] = rows[1][i]
			}
		}
	}
	return out, nil
}

// lookup finds a metric by any of its names, with or without a library-type prefix.
func lookup(metrics map[string]string, library string, names ...string) (string, bool) {
	for _, n := range names {
		for _, key := range []string{strings.ToLower(library + ": " + n), strings.ToLower(n)} {
			if v, ok := metrics[key]; ok {
				return v, true
			}
		}
	}
	return "", false
}

// parseMetric reads "4,562", "87.51%" or "3.0": its value, and whether it was a percent.
func parseMetric(s string) (float64, bool, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	percent := strings.HasSuffix(s, "%")
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	return v, percent, err == nil
}

// decimals: how many digits after the point a reported number has (its rounding).
func decimals(s string) int {
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

// CompareCellRanger compares Cell Ranger's metrics of a sample with ClonoLoom's count of its cells
// (from the contig annotations, before any filter): cells with a productive chain of each locus and
// cells with a productive pair, as counts and as shares of the cells Cell Ranger estimated.
func CompareCellRanger(metrics map[string]string, ch *Chains, loci []string) []Check {
	if ch == nil {
		return nil
	}
	var checks []Check
	library := "VDJ T"
	if len(loci) > 0 && strings.HasPrefix(loci[0], "IG") {
		library = "VDJ B"
	}
	estimated := math.NaN()
	if v, ok := lookup(metrics, library, "Estimated Number of Cells", "VDJ cells", "VDJ Cells"); ok {
		if n, _, ok := parseMetric(v); ok {
			estimated = n
			checks = append(checks, Check{Metric: "Cells", Reported: v, Recomputed: strconv.Itoa(ch.Cells), Agrees: nil,
				Note: "Cell Ranger also counts cells none of whose contigs passed its filters, which the contig annotations do not list"})
		}
	}
	paired := ch.Paired
	if v, ok := lookup(metrics, library, "Number of Cells With Productive V-J Spanning Pair"); ok {
		if n, _, ok := parseMetric(v); ok {
			checks = append(checks, Check{Metric: "Cells with a productive pair", Reported: v, Recomputed: strconv.Itoa(paired), Agrees: agree(n == float64(paired))})
		}
	}
	share := func(metric, name string, count int) {
		v, ok := lookup(metrics, library, name)
		if !ok || math.IsNaN(estimated) || estimated <= 0 {
			return
		}
		reported, percent, ok := parseMetric(v)
		if !ok || !percent {
			return
		}
		d := decimals(v)
		mine := 100 * float64(count) / estimated
		scale := math.Pow(10, float64(d))
		rounded := math.Round(mine*scale) / scale
		checks = append(checks, Check{Metric: metric, Reported: v, Recomputed: strconv.FormatFloat(rounded, 'f', d, 64) + "%",
			Agrees: agree(math.Abs(rounded-reported) < 0.5/scale+1e-9), Note: fmt.Sprintf("%d cells of the %s Cell Ranger estimated", count, strconv.FormatFloat(estimated, 'f', -1, 64))})
	}
	share("Cells with a productive pair (share)", "Cells With Productive V-J Spanning Pair", paired)
	sorted := append([]string(nil), loci...)
	sort.Strings(sorted)
	for _, l := range sorted {
		share("Cells with a productive "+l+" chain", "Cells With Productive "+l+" Contig", ch.ByLocus[l])
	}
	return checks
}

// CompareImmunoSEQ compares the totals immunoSEQ reports with a sample (its sample columns: total
// and productive templates and rearrangements, out-of-frame and stop templates) with ClonoLoom's
// sums of the sample's rows. Template totals are compared where the dataset is counted in
// templates.
func CompareImmunoSEQ(t *rep.Table, sample int) []Check {
	meta := t.Samples[sample].Metadata
	if len(meta) == 0 {
		return nil
	}
	var rows, productiveRows int
	var total, productive, outOfFrame, stop float64
	for row := 0; row < t.Len(); row++ {
		if int(t.Sample[row]) != sample {
			continue
		}
		rows++
		w := t.Count[row]
		if math.IsNaN(w) {
			w = 0
		}
		total += w
		switch {
		case t.Productive[row] == rep.True:
			productiveRows++
			productive += w
		case t.StopCodon[row] == rep.True:
			stop += w
		case t.VJInFrame[row] == rep.False:
			outOfFrame += w
		}
	}
	var checks []Check
	count := func(metric, key string, mine float64, note string) {
		v, ok := meta[key]
		if !ok {
			return
		}
		reported, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return
		}
		c := Check{Metric: metric, Reported: v, Recomputed: strconv.FormatFloat(mine, 'f', -1, 64), Agrees: agree(reported == mine)}
		if reported != mine {
			c.Note = note
		}
		checks = append(checks, c)
	}
	// Seen in published exports: the rows' templates by frame type do not always sum to the sample
	// columns (pandas sums the rows as ClonoLoom does), which come from another estimate.
	const earlier = "immunoSEQ's summary column differs from the sum of the export's own rows; ClonoLoom uses the rows"
	count("Rearrangements", "total_rearrangements", float64(rows), "")
	count("Productive rearrangements", "productive_rearrangements", float64(productiveRows), "")
	if t.Unit == rep.Templates {
		count("Templates", "total_templates", total, earlier)
		count("Productive templates", "productive_templates", productive, earlier)
		count("Out-of-frame templates", "outofframe_templates", outOfFrame, earlier)
		count("Stop-codon templates", "stop_templates", stop, earlier)
	}
	checks = append(checks, adaptiveClonality(t, sample, meta)...)
	return checks
}

// adaptiveClonality recomputes immunoSEQ's clonality columns as Adaptive defines them (its tech
// notes MRK-00342 and MRK-00355): over the productive rearrangements (rows, not clonotypes), each
// one's frequency of the productive reads. Adaptive computes these from reads, not templates (its
// reported values are reproduced from reads alone, on every published export checked): the
// entropy (log base 2), the clonality 1 − entropy/log2(rearrangements), the Simpson clonality
// √Σp² and the largest frequency.
func adaptiveClonality(t *rep.Table, sample int, meta map[string]string) []Check {
	readsOf := func(row int) float64 {
		switch {
		case t.Reads != nil:
			return t.Reads[row]
		case t.Unit == rep.Reads:
			return t.Count[row]
		}
		return math.NaN()
	}
	var reads []float64
	total := 0.0
	for row := 0; row < t.Len(); row++ {
		if int(t.Sample[row]) != sample || t.Productive[row] != rep.True {
			continue
		}
		if r := readsOf(row); r > 0 {
			reads = append(reads, r)
			total += r
		}
	}
	if len(reads) < 2 {
		return nil
	}
	var h, simpson, top float64
	for _, r := range reads {
		p := r / total
		h -= p * math.Log2(p)
		simpson += p * p
		top = max(top, p)
	}
	mine := map[string]float64{
		"productive_entropy": h, "productive_clonality": 1 - h/math.Log2(float64(len(reads))),
		"productive_simpson_clonality": math.Sqrt(simpson), "max_productive_frequency": top,
	}
	var checks []Check
	for _, c := range []struct{ metric, key string }{
		{"Productive clonality (from reads)", "productive_clonality"}, {"Productive Simpson clonality (from reads)", "productive_simpson_clonality"},
		{"Productive entropy, log2 (from reads)", "productive_entropy"}, {"Largest productive frequency (from reads)", "max_productive_frequency"},
	} {
		v, ok := meta[c.key]
		if !ok {
			continue
		}
		reported, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			continue
		}
		// Agreeing to seven significant digits: Adaptive's entropy differs from the exact one by
		// about 2·10⁻⁸ of itself (computed in lower precision), its other columns by less.
		same := math.Abs(reported-mine[c.key]) <= 1e-7*math.Max(1e-3, math.Abs(reported))
		check := Check{Metric: c.metric, Reported: v, Recomputed: strconv.FormatFloat(mine[c.key], 'g', 10, 64), Agrees: agree(same)}
		if !same {
			check.Note = "immunoSEQ's value differs from its rows' reads"
		}
		checks = append(checks, check)
	}
	return checks
}

// splitCSVLine splits one line of a CSV (RFC 4180 quoting).
func splitCSVLine(line string) ([]string, bool) {
	var fields []string
	var b strings.Builder
	quoted, inQuotes := false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQuotes && c == '"' && i+1 < len(line) && line[i+1] == '"':
			b.WriteByte('"')
			i++
		case c == '"' && (b.Len() == 0 || inQuotes):
			inQuotes = !inQuotes
			quoted = true
		case c == ',' && !inQuotes:
			fields = append(fields, b.String())
			b.Reset()
			quoted = false
		default:
			b.WriteByte(c)
		}
	}
	_ = quoted
	if inQuotes {
		return nil, false
	}
	return append(fields, b.String()), true
}

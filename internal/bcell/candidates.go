package bcell

// Descriptive review policy, not an affinity or efficacy estimator. AIRR 2.0
// observations and the existing family/SHM/isotype contracts supply the evidence.
// Ordered criteria, strict missingness and competition ranks are ClonoLoom choices.
import (
	"context"
	"errors"
	"math/big"
	"sort"
)

const CandidateVersion = 1
const CandidateLimits = "Descriptive candidate review, not binding, affinity, efficacy, selection or synthesis readiness. One selected sample; no pooled subjects or inferred independent replicates. Size counts assigned observations (sample-scoped cells or bulk heavy-chain rows), not reads or molecules. Mutation pools observed heavy-row differences/evaluable V/J bases, not historical events or mutation rates; incomplete rows make ranking unavailable. Preferred-class share requires every assigned observation's class to be resolved. Share change is current minus explicitly named baseline observation share, not a statistical expansion test; zero observed counts do not establish biological absence. All metrics condition on the saved family definition, QC and reference choices. Missing values sort last in either direction; exact rational ties retain the same competition rank."

type CandidateCriterion struct {
	Metric    string `json:"metric"`
	Direction string `json:"direction"`
}
type CandidateOptions struct {
	Sample         string               `json:"sample"`
	Baseline       string               `json:"baseline"`
	PreferredClass string               `json:"preferredClass"`
	Criteria       []CandidateCriterion `json:"criteria"`
}
type CandidateMetric struct {
	Numerator   int64    `json:"numerator"`
	Denominator int64    `json:"denominator"`
	Value       *float64 `json:"value"`
	Missing     string   `json:"missing,omitempty"`
}

func candidateMetric(n, d int64, missing string) CandidateMetric {
	m := CandidateMetric{Numerator: n, Denominator: d, Missing: missing}
	if missing == "" && d > 0 {
		v := float64(n) / float64(d)
		m.Value = &v
	}
	return m
}

type Candidate struct {
	Family             string                     `json:"family"`
	Rank               int                        `json:"rank"`
	Tied               int                        `json:"tied"`
	Current            int                        `json:"current"`
	Baseline           int                        `json:"baseline"`
	HeavyRows          int                        `json:"heavyRows"`
	MissingHeavyRows   int                        `json:"missingHeavyRows"`
	UnresolvedIsotypes int                        `json:"unresolvedIsotypes"`
	Metrics            map[string]CandidateMetric `json:"metrics"`
	Observations       []FamilyObservation        `json:"observations"`
}
type CandidateResult struct {
	Options          CandidateOptions `json:"options"`
	Scope            string           `json:"scope"`
	Unit             string           `json:"unit"`
	CurrentTotal     int              `json:"currentTotal"`
	BaselineTotal    int              `json:"baselineTotal"`
	CurrentExcluded  int              `json:"currentExcluded"`
	BaselineExcluded int              `json:"baselineExcluded"`
	Rows             []Candidate      `json:"rows"`
	Limits           string           `json:"limits"`
}

func CandidateReview(ctx context.Context, families FamilyResult, shm *SHMResult, isotypes *IsotypeResult, options CandidateOptions) (CandidateResult, error) {
	out := CandidateResult{Options: options, Rows: []Candidate{}, Limits: CandidateLimits, Unit: "heavy-chain rows"}
	if families.SingleCell {
		out.Unit = "sample-scoped cells"
	}
	if families.Options.Threshold == nil || len(families.Observations) > 50000 || len(families.Families) > 5000 {
		return out, errors.New("Choose thresholded families within 50,000 observations and 5,000 families")
	}
	if options.Sample == "" || len(options.Criteria) < 1 || len(options.Criteria) > 4 {
		return out, errors.New("Select one sample and one to four explicit ranking criteria")
	}
	classes := map[string]bool{}
	for _, c := range isotypeClasses {
		classes[c] = true
	}
	seen := map[string]bool{}
	for _, c := range options.Criteria {
		if seen[c.Metric] || (c.Direction != "ascending" && c.Direction != "descending") {
			return out, errors.New("Criteria must be unique with an explicit direction")
		}
		seen[c.Metric] = true
		switch c.Metric {
		case "size":
		case "mutation":
			if shm == nil {
				return out, errors.New("Mutation ranking requires a matching saved SHM result")
			}
		case "isotype":
			if isotypes == nil {
				return out, errors.New("Isotype ranking requires a matching saved isotype result")
			}
		case "change":
			if options.Baseline == "" {
				return out, errors.New("Share change needs an explicit baseline sample")
			}
		default:
			return out, errors.New("Unsupported candidate criterion")
		}
	}
	if options.PreferredClass != "" {
		ok := false
		for _, v := range isotypeClasses {
			if v == options.PreferredClass {
				ok = true
			}
		}
		if !ok {
			return out, errors.New("Select a supported preferred isotype class")
		}
	}
	if seen["isotype"] && options.PreferredClass == "" {
		return out, errors.New("Select a preferred isotype class explicitly")
	}
	scope := ""
	subject := ""
	baselineSubject := ""
	baselineScope := ""
	foundSample, foundBase := false, false
	for _, o := range families.Observations {
		if o.Sample == options.Sample {
			foundSample = true
			if o.Excluded != "" || o.Family == "" {
				out.CurrentExcluded++
				continue
			}
			if scope != "" && (scope != o.Scope || subject != o.Subject) {
				return out, errors.New("Selected sample has ambiguous family scopes")
			}
			scope = o.Scope
			subject = o.Subject
			out.CurrentTotal++
		}
		if options.Baseline != "" && o.Sample == options.Baseline {
			foundBase = true
			if o.Excluded != "" || o.Family == "" {
				out.BaselineExcluded++
				continue
			}
			if o.Subject == "" || baselineSubject != "" && (baselineSubject != o.Subject || baselineScope != o.Scope) {
				return out, errors.New("Ambiguous baseline subject")
			}
			baselineSubject = o.Subject
			baselineScope = o.Scope
			out.BaselineTotal++
		}
	}
	if !foundSample || scope == "" {
		return out, errors.New("Selected sample has no assigned family observations")
	}
	if options.Baseline != "" && (!foundBase || options.Baseline == options.Sample || families.Options.Scope != "subject" || subject == "" || subject != baselineSubject || scope != baselineScope) {
		return out, errors.New("Baseline must be a different evaluable sample from the same explicit subject under subject-scoped families")
	}
	out.Scope = scope
	iso := map[string]IsotypeObservation{}
	if isotypes != nil {
		for _, o := range isotypes.Observations {
			if _, ok := iso[o.ID]; ok {
				return out, errors.New("Repeated isotype observation")
			}
			iso[o.ID] = o
		}
	}
	mutations := map[string]SHMRow{}
	if shm != nil {
		for _, r := range shm.Rows {
			if _, ok := mutations[r.ID]; ok {
				return out, errors.New("Repeated SHM source row")
			}
			mutations[r.ID] = r
		}
	}
	by := map[string]*Candidate{}
	for _, f := range families.Families {
		if f.Scope == scope {
			if by[f.ID] != nil {
				return out, errors.New("Repeated family identity")
			}
			by[f.ID] = &Candidate{Family: f.ID, Metrics: map[string]CandidateMetric{}, Observations: []FamilyObservation{}}
		}
	}
	ids := map[string]bool{}
	sourceRows := map[string]bool{}
	for _, o := range families.Observations {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if o.ID == "" || ids[o.ID] {
			return out, errors.New("Repeated family observation")
		}
		ids[o.ID] = true
		for _, row := range o.Rows {
			if row.ID == "" || sourceRows[row.ID] {
				return out, errors.New("Repeated family source row")
			}
			sourceRows[row.ID] = true
		}
		if len(sourceRows) > 250000 {
			return out, errors.New("Candidate review exceeds 250,000 source rows")
		}
		if o.Excluded != "" || o.Family == "" || o.Scope != scope {
			continue
		}
		c := by[o.Family]
		if c == nil {
			return out, errors.New("Observation references an unknown family")
		}
		if o.Sample == options.Baseline {
			c.Baseline++
		}
		if o.Sample != options.Sample {
			continue
		}
		c.Current++
		c.Observations = append(c.Observations, o)
		im := c.Metrics["isotype"]
		v, ok := iso[o.ID]
		if ok && (v.Family != o.Family || v.Sample != o.Sample || v.Scope != o.Scope || v.Subject != o.Subject || v.Cell != o.Cell) {
			return out, errors.New("Isotype observation identity differs from family evidence")
		}
		if !ok || v.Excluded != "" || !classes[v.Class] {
			c.UnresolvedIsotypes++
		} else {
			if v.Family != o.Family || v.Sample != o.Sample || v.Scope != o.Scope {
				return out, errors.New("Isotype observation identity differs from family evidence")
			}
			if v.Class == options.PreferredClass {
				im.Numerator++
			}
		}
		im.Denominator++
		c.Metrics["isotype"] = im
		mm := c.Metrics["mutation"]
		heavySeen := false
		for _, row := range o.Rows {
			if row.Locus != "IGH" {
				continue
			}
			heavySeen = true
			c.HeavyRows++
			r, ok := mutations[row.ID]
			available := false
			if ok {
				if r.Family != o.Family || r.Observation != o.ID || r.Sample != o.Sample || r.FamilyRow != row {
					return out, errors.New("SHM row identity differs from family evidence")
				}
				for _, region := range r.Regions {
					if region.Region == "all" && r.Excluded == "" && region.Unavailable == "" && region.EvaluableBases > 0 {
						if region.Mutations < 0 || region.Mutations > region.EvaluableBases || region.EvaluableBases > 10000 {
							return out, errors.New("Invalid SHM counts")
						}
						mm.Numerator += int64(region.Mutations)
						mm.Denominator += int64(region.EvaluableBases)
						available = true
						break
					}
				}
			}
			if !available {
				c.MissingHeavyRows++
			}
		}
		if !heavySeen {
			return out, errors.New("Assigned observation has no heavy-chain source row")
		}
		c.Metrics["mutation"] = mm
	}
	for _, c := range by {
		c.Metrics["size"] = candidateMetric(int64(c.Current), 1, "")
		m := c.Metrics["mutation"]
		why := ""
		if shm == nil {
			why = "No SHM parent selected"
		} else if c.HeavyRows == 0 || c.MissingHeavyRows > 0 {
			why = "Incomplete heavy-row mutation evidence"
		}
		c.Metrics["mutation"] = candidateMetric(m.Numerator, m.Denominator, why)
		m = c.Metrics["isotype"]
		why = ""
		if isotypes == nil || options.PreferredClass == "" {
			why = "No isotype parent or preferred class selected"
		} else if c.Current == 0 || c.UnresolvedIsotypes > 0 {
			why = "Incomplete observed isotype classes"
		}
		c.Metrics["isotype"] = candidateMetric(m.Numerator, m.Denominator, why)
		why = ""
		if options.Baseline == "" || out.BaselineTotal == 0 {
			why = "No evaluable baseline sample"
		}
		c.Metrics["change"] = candidateMetric(int64(c.Current)*int64(out.BaselineTotal)-int64(c.Baseline)*int64(out.CurrentTotal), int64(out.CurrentTotal)*int64(out.BaselineTotal), why)
		sort.Slice(c.Observations, func(i, j int) bool { return c.Observations[i].ID < c.Observations[j].ID })
		out.Rows = append(out.Rows, *c)
	}
	compare := func(a, b Candidate) int {
		for _, criterion := range options.Criteria {
			x, y := a.Metrics[criterion.Metric], b.Metrics[criterion.Metric]
			if x.Value == nil && y.Value != nil {
				return 1
			}
			if x.Value != nil && y.Value == nil {
				return -1
			}
			if x.Value == nil {
				continue
			}
			cmp := new(big.Rat).SetFrac64(x.Numerator, x.Denominator).Cmp(new(big.Rat).SetFrac64(y.Numerator, y.Denominator))
			if criterion.Direction == "descending" {
				cmp = -cmp
			}
			if cmp != 0 {
				return cmp
			}
		}
		return 0
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		cmp := compare(out.Rows[i], out.Rows[j])
		if cmp == 0 {
			return out.Rows[i].Family < out.Rows[j].Family
		}
		return cmp < 0
	})
	for i := 0; i < len(out.Rows); {
		j := i + 1
		for j < len(out.Rows) && compare(out.Rows[i], out.Rows[j]) == 0 {
			j++
		}
		for k := i; k < j; k++ {
			out.Rows[k].Rank = i + 1
			out.Rows[k].Tied = j - i
		}
		i = j
	}
	return out, nil
}

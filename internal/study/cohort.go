package study

// Subject-level cohort design checks. Lazic SE (2010), BMC Neurosci 11:5,
// doi:10.1186/1471-2202-11-5, explains why repeated observations are not independent
// experimental units. Squair et al. (2021), Nat Commun 12:5692,
// doi:10.1038/s41467-021-25960-2, demonstrates this problem for single-cell expression.
// ClonoLoom's choices: two explicit levels, subjects only, no automatic pooling or
// complete-case deletion, and at least two independent subjects per group/complete pairs.
// These are structural checks, not proof of independence, power or causal identification.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const MaxCohortSamples = 10_000

type CohortSample struct {
	Name       string `json:"name"`
	Subject    string `json:"subject"`
	Condition  string `json:"condition"`
	Timepoint  string `json:"timepoint"`
	Tissue     string `json:"tissue"`
	CellSubset string `json:"cellSubset"`
	Batch      string `json:"batch"`
}

type CohortSpec struct {
	Unit       string            `json:"unit"`
	Layout     string            `json:"layout"`
	GroupField string            `json:"groupField"`
	Levels     []string          `json:"levels"` // reference, then comparison
	Exclude    map[string]string `json:"exclude"`
}

type CohortInput struct {
	Dataset string         `json:"dataset"`
	Samples []CohortSample `json:"samples"`
	Spec    CohortSpec     `json:"spec"`
}

type CohortRow struct {
	CohortSample
	Group    string `json:"group"`
	Included bool   `json:"included"`
	Reason   string `json:"reason"`
}

type CohortGroup struct {
	Level    string `json:"level"`
	Samples  int    `json:"samples"`
	Subjects int    `json:"subjects"`
}

type CohortDesign struct {
	Format        string        `json:"format"`
	Version       int           `json:"version"`
	ID            string        `json:"id"`
	Input         CohortInput   `json:"input"`
	OK            bool          `json:"ok"`
	Rows          []CohortRow   `json:"rows"`
	Groups        []CohortGroup `json:"groups"`
	Subjects      int           `json:"subjects"`
	CompletePairs int           `json:"completePairs"`
	Included      int           `json:"included"`
	Excluded      int           `json:"excluded"`
	Findings      []Finding     `json:"findings"`
	Words         []string      `json:"words"`
	Citations     []string      `json:"citations"`
}

func cohortField(s CohortSample, field string) string {
	switch field {
	case "condition":
		return s.Condition
	case "timepoint":
		return s.Timepoint
	case "tissue":
		return s.Tissue
	case "cellSubset":
		return s.CellSubset
	}
	return ""
}

// CheckCohort returns every input sample's disposition. No sample-level metric or
// receptor evidence is consumed: a later analysis must pin this design and its sources.
func CheckCohort(input CohortInput) (CohortDesign, error) {
	d := CohortDesign{Format: "clonoloom-cohort-design", Version: 1, OK: true,
		Rows: []CohortRow{}, Groups: []CohortGroup{}, Findings: []Finding{},
		Citations: []string{"Lazic SE. BMC Neurosci. 2010;11:5. doi:10.1186/1471-2202-11-5", "Squair JW et al. Nat Commun. 2021;12:5692. doi:10.1038/s41467-021-25960-2"}}
	spec := input.Spec
	if input.Dataset == "" || len(input.Dataset) > 4096 {
		return d, fmt.Errorf("give a source-bound dataset identity of at most 4096 bytes")
	}
	if spec.Unit != "subject" || (spec.Layout != "independent" && spec.Layout != "paired") {
		return d, fmt.Errorf("choose unit subject and layout independent or paired; cells, sequences and repeated samples are not independent subjects")
	}
	if spec.GroupField != "condition" && spec.GroupField != "timepoint" && spec.GroupField != "tissue" && spec.GroupField != "cellSubset" {
		return d, fmt.Errorf("groupField must be condition, timepoint, tissue or cellSubset")
	}
	if len(spec.Levels) != 2 || strings.TrimSpace(spec.Levels[0]) == "" || strings.TrimSpace(spec.Levels[1]) == "" || strings.TrimSpace(spec.Levels[0]) == strings.TrimSpace(spec.Levels[1]) {
		return d, fmt.Errorf("choose two different, nonempty levels: reference then comparison")
	}
	if len(input.Samples) == 0 || len(input.Samples) > MaxCohortSamples {
		return d, fmt.Errorf("a cohort design needs 1–%d samples", MaxCohortSamples)
	}
	spec.Levels = []string{strings.TrimSpace(spec.Levels[0]), strings.TrimSpace(spec.Levels[1])}
	if len(spec.Levels[0]) > 4096 || len(spec.Levels[1]) > 4096 {
		return d, fmt.Errorf("group labels must be at most 4096 bytes")
	}
	exclude := make(map[string]string, len(spec.Exclude))
	for name, reason := range spec.Exclude {
		if strings.TrimSpace(reason) == "" || len(reason) > 4096 {
			return d, fmt.Errorf("give a reason for excluding sample %q", name)
		}
		exclude[name] = strings.TrimSpace(reason)
	}
	spec.Exclude = exclude
	input.Spec = spec
	input.Samples = append([]CohortSample(nil), input.Samples...)
	seen := map[string]bool{}
	for i := range input.Samples {
		s := &input.Samples[i]
		if s.Name == "" || len(s.Name) > 4096 || seen[s.Name] {
			return d, fmt.Errorf("sample names must be nonempty and unique: %q", s.Name)
		}
		seen[s.Name] = true
		for _, value := range []*string{&s.Subject, &s.Condition, &s.Timepoint, &s.Tissue, &s.CellSubset, &s.Batch} {
			*value = strings.TrimSpace(*value)
			if len(*value) > 4096 {
				return d, fmt.Errorf("sample %q has an annotation longer than 4096 bytes", s.Name)
			}
		}
	}
	for name := range exclude {
		if !seen[name] {
			return d, fmt.Errorf("excluded sample %q is not in this dataset", name)
		}
	}
	sort.Slice(input.Samples, func(i, j int) bool { return input.Samples[i].Name < input.Samples[j].Name })
	d.Input = input
	encoded, err := json.Marshal(struct {
		Version int         `json:"version"`
		Input   CohortInput `json:"input"`
	}{d.Version, input})
	if err != nil {
		return d, err
	}
	sum := sha256.Sum256(encoded)
	d.ID = "cd-" + hex.EncodeToString(sum[:])
	add := func(code, severity, title, detail string, examples []string) {
		if len(examples) == 0 {
			return
		}
		n := len(examples)
		if n > 5 {
			examples = examples[:5]
		}
		d.Findings = append(d.Findings, Finding{Code: code, Severity: severity, Title: title, Detail: detail, Count: n, Examples: examples})
		if severity == "error" {
			d.OK = false
		}
	}
	bySubject := map[string][2][]string{}
	groupSubjects := [2]map[string]bool{{}, {}}
	batchGroups := map[string][2]bool{}
	var missingSubject, missingGroup, missingBatch []string
	contexts := map[string]map[string]bool{"timepoint": {}, "tissue": {}, "cellSubset": {}}
	for _, level := range spec.Levels {
		d.Groups = append(d.Groups, CohortGroup{Level: level})
	}
	for _, s := range input.Samples {
		r := CohortRow{CohortSample: s, Group: cohortField(s, spec.GroupField)}
		if reason, ok := exclude[s.Name]; ok {
			r.Reason = "Excluded: " + reason
		} else if r.Group == "" {
			r.Reason = "Missing group annotation"
			missingGroup = append(missingGroup, s.Name)
		} else if r.Group != spec.Levels[0] && r.Group != spec.Levels[1] {
			r.Reason = "Outside the selected contrast"
		} else if s.Subject == "" {
			r.Reason = "Missing subject annotation"
			missingSubject = append(missingSubject, s.Name)
		} else {
			r.Included = true
			g := 0
			if r.Group == spec.Levels[1] {
				g = 1
			}
			d.Groups[g].Samples++
			groupSubjects[g][s.Subject] = true
			groups := bySubject[s.Subject]
			groups[g] = append(groups[g], s.Name)
			bySubject[s.Subject] = groups
			if s.Batch == "" {
				missingBatch = append(missingBatch, s.Name)
			} else {
				bg := batchGroups[s.Batch]
				bg[g] = true
				batchGroups[s.Batch] = bg
			}
			for field, values := range contexts {
				values[cohortField(s, field)] = true
			}
		}
		if r.Included {
			d.Included++
		} else {
			d.Excluded++
		}
		d.Rows = append(d.Rows, r)
	}
	add("cohort-no-subject", "error", "Subjects are missing", "Annotate these samples or explicitly exclude them with a reason.", missingSubject)
	add("cohort-no-group", "error", "Groups are missing", "Annotate these samples or explicitly exclude them with a reason.", missingGroup)
	ids := make([]string, 0, len(bySubject))
	for id := range bySubject {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var repeated, shared, incomplete []string
	for _, id := range ids {
		gs := bySubject[id]
		for i, names := range gs {
			if len(names) > 1 {
				repeated = append(repeated, id+" / "+spec.Levels[i]+": "+strings.Join(names, ", "))
			}
		}
		if len(gs[0]) > 0 && len(gs[1]) > 0 {
			shared = append(shared, id)
		} else {
			incomplete = append(incomplete, id)
		}
		if len(gs[0]) == 1 && len(gs[1]) == 1 {
			d.CompletePairs++
		}
	}
	d.Subjects = len(ids)
	for i := range d.Groups {
		d.Groups[i].Subjects = len(groupSubjects[i])
	}
	add("cohort-repeated-samples", "error", "More than one sample per subject and group", "No samples are pooled automatically. Select one scientifically justified sample with explicit exclusions; repeated observations do not add independent subjects.", repeated)
	if spec.Layout == "independent" {
		add("cohort-shared-subject", "error", "Subjects occur in both independent groups", "Use a paired design for repeated observations of the same subject.", shared)
		for _, g := range d.Groups {
			if g.Subjects < 2 {
				add("cohort-too-few-subjects", "error", "Too few independent subjects", "Each group requires at least two subjects. This minimum does not establish adequate power.", []string{g.Level})
			}
		}
	} else {
		add("cohort-incomplete-pairs", "error", "Subjects lack a selected level", "Complete pairs are required. Missing observations are not zero; exclude incomplete subjects explicitly, stating why.", incomplete)
		if d.CompletePairs < 2 {
			add("cohort-too-few-pairs", "error", "Too few complete subject pairs", "At least two complete pairs are required. This minimum does not establish adequate power.", []string{fmt.Sprintf("%d complete pairs", d.CompletePairs)})
		}
	}
	add("cohort-missing-batch", "warning", "Batch information is incomplete", "Batch confounding cannot be fully checked; absence of a finding is not evidence of no batch effect.", missingBatch)
	if len(missingBatch) == 0 && len(batchGroups) > 1 {
		mixed := false
		for _, gs := range batchGroups {
			mixed = mixed || (gs[0] && gs[1])
		}
		if !mixed && d.Groups[0].Samples > 0 && d.Groups[1].Samples > 0 {
			add("cohort-batch-confounded", "error", "Group is inseparable from batch", "Every recorded batch contains only one group. Group and batch effects cannot be separated by this comparison.", spec.Levels)
		}
	}
	for _, field := range []string{"timepoint", "tissue", "cellSubset"} {
		if field != spec.GroupField && len(contexts[field]) > 1 {
			add("cohort-mixed-"+field, "warning", "Review mixed "+field+" annotations", "Different or missing annotations occur among included samples. No adjustment is performed; review the sampling context before comparing groups.", []string{field})
		}
	}
	d.Words = []string{
		fmt.Sprintf("%s design: %s minus %s, grouped by %s; the independent unit is the subject.", spec.Layout, spec.Levels[1], spec.Levels[0], spec.GroupField),
		fmt.Sprintf("%d included samples from %d subjects; %d excluded samples; %d complete subject pairs.", d.Included, d.Subjects, d.Excluded, d.CompletePairs),
		"This checks the recorded design only. It performs no statistical test, pooling, covariate adjustment or power calculation and cannot verify biological independence. Cells and sequences are not replicates.",
		"Group labels match exactly after trimming whitespace; time labels are not converted or combined. Review warnings and sampling context even when the structural checks pass.",
	}
	return d, nil
}

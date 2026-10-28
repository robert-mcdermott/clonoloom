package main

// The allowlist is intentionally smaller than the durable job registry. A resumable
// job is not automatically a portable replay contract or a headless source workflow.
type certificateSupport struct {
	Kind      string `json:"kind"`
	Scope     string `json:"scope"`
	SourceRun bool   `json:"sourceRun"`
	Limits    string `json:"limits"`
}

var certificateSupportMatrix = []certificateSupport{
	{cohortFeaturesKind, replayScope, false, replayLimits},
	{cohortCompareKind, replayScope, false, replayLimits},
	{cohortCountsKind, replayScope, false, replayLimits},
	{batchqcKind, "captured-batch-qc-evidence", true, "Exact replay from captured sample QC measurements only. Source files and the extraction of sample metrics are not independently verified. Batch medians, MADs and descriptive flags are recomputed; flags do not establish contamination or justify automatic correction. Matching bytes do not establish authorship or scientific validity. Captured evidence is not de-identified."},
	{motifsKind, "captured-motif-evidence", true, "Exact replay from captured query/background sequence units only. Source files and upstream clonotype capture are not independently verified. The existing GLIPH2-style ClonoLoom adaptation is rerun with its recorded testing family and options; this is not upstream GLIPH2 equivalence, population-calibrated inference or proof of antigen specificity. Matching bytes do not establish authorship or scientific validity. Captured evidence is not de-identified."},
}

func certificateSupportFor(kind string) (certificateSupport, bool) {
	for _, s := range certificateSupportMatrix {
		if s.Kind == kind {
			return s, true
		}
	}
	return certificateSupport{}, false
}
func (a *app) handleReplayJobs() {
	a.handleCohortFeaturesJobs()
	a.handleCohortCompareJobs()
	a.handleCohortCountsJobs()
	a.handleBatchQCJobs()
	a.handleMotifsJobs()
}

type workflowSupport struct {
	Kind        string `json:"kind"`
	Certificate bool   `json:"certificate"`
	Replay      bool   `json:"replay"`
	SourceRun   bool   `json:"sourceRun"`
	Scope       string `json:"scope,omitempty"`
	Limits      string `json:"limits"`
}

func (a *app) workflowSupport() []workflowSupport {
	out := []workflowSupport{}
	for _, kind := range a.jobs.Kinds() {
		s, ok := certificateSupportFor(kind)
		limits := s.Limits
		if !ok {
			limits = "Unavailable: source, parent, notice and portable replay contracts are not mapped for this kind. Durable resume is not certificate replay."
		}
		out = append(out, workflowSupport{kind, ok, ok, s.SourceRun, s.Scope, limits})
	}
	return out
}

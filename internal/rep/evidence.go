package rep

// EvidenceFields are the retained input annotations, in stable export/cache order.
// AIRR fields plus the native IMGT annotation extension. AIRR Rearrangement schema 2.0; Vander Heiden et al. 2018, Front Immunol 9:2206,
// doi:10.3389/fimmu.2018.02206. Raw text preserves invalid and missing evidence;
// checked interpretations never replace what the annotation tool supplied.
var EvidenceFields = [...]string{
	"sequence",
	"rev_comp",
	"sequence_alignment",
	"germline_alignment",
	"v_cigar",
	"d_cigar",
	"j_cigar",
	"germline_database",
	"v_sequence_start",
	"v_sequence_end",
	"v_germline_start",
	"v_germline_end",
	"v_alignment_start",
	"v_alignment_end",
	"d_sequence_start",
	"d_sequence_end",
	"d_germline_start",
	"d_germline_end",
	"d_alignment_start",
	"d_alignment_end",
	"j_sequence_start",
	"j_sequence_end",
	"j_germline_start",
	"j_germline_end",
	"j_alignment_start",
	"j_alignment_end",
	"fwr1_start",
	"fwr1_end",
	"cdr1_start",
	"cdr1_end",
	"fwr2_start",
	"fwr2_end",
	"cdr2_start",
	"cdr2_end",
	"fwr3_start",
	"fwr3_end",
	"cdr3_start",
	"cdr3_end",
	"fwr4_start",
	"fwr4_end",
	"np1_length",
	"np2_length",
	// Native IMGT annotations, including coordinate provenance, as a JSON object.
	"imgt_annotations",
}

const EvidenceCount = len(EvidenceFields)

// EvidenceValue returns a supplied field; an absent column or empty value is missing.
func (t *Table) EvidenceValue(row, field int) string {
	if t.EvidenceText == nil || t.Evidence[field] == nil {
		return ""
	}
	return t.EvidenceText.Get(t.Evidence[field][row])
}

// EvidenceRow is a bounded copy of one rearrangement's annotations.
func (t *Table) EvidenceRow(row int) [EvidenceCount]string {
	var out [EvidenceCount]string
	for i := range out {
		out[i] = t.EvidenceValue(row, i)
	}
	return out
}

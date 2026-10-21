package rep

import (
	"fmt"
	"math"

	"clonoloom/internal/genes"
)

// Row is one rearrangement as a reader hands it to a Builder. Strings are as read (trimmed);
// "" is missing, as are NaN counts and Missing flags.
type Row struct {
	Sample               string
	RepertoireID         string
	Locus                string
	V, D, J, C           string
	Junction, JunctionAA string
	Productive           Flag
	VJInFrame, StopCodon Flag
	Count, Reads, UMIs   float64
	Cell, Clone          string
	SequenceID           string
	IsCell               Flag
	HighConfidence       Flag
	FullLength           Flag
	Source               int
	Line                 uint32
}

// NewRow is a Row with every value missing.
func NewRow() Row {
	return Row{Productive: Missing, VJInFrame: Missing, StopCodon: Missing, Count: math.NaN(), Reads: math.NaN(), UMIs: math.NaN(), IsCell: Missing, HighConfidence: Missing, FullLength: Missing}
}

// Builder makes a Table row by row. Optional columns are made the first time a row has a value
// in them, and earlier rows get a missing value.
type Builder struct {
	t       *Table
	samples map[string]uint32
	// loci caches the locus of a gene call, for rows whose source names no locus.
	loci map[string]string
}

func NewBuilder(unit Unit) *Builder {
	return &Builder{
		t: &Table{
			Dict: NewStrings(true), Junctions: NewStrings(true), JunctionsAA: NewStrings(true), IDs: NewStrings(false),
			Unit: unit,
		},
		samples: map[string]uint32{},
		loci:    map[string]string{},
	}
}

// callLocus is the locus a row's genes name when its source names none (AIRR files without a
// locus column, Cell Ranger's AIRR file): the J call's, else the V call's, else the C call's.
func (b *Builder) callLocus(r *Row) string {
	for _, call := range []string{r.J, r.V, r.C} {
		if call == "" {
			continue
		}
		locus, ok := b.loci[call]
		if !ok {
			locus = genes.ParseCall(call).Locus()
			b.loci[call] = locus
		}
		if locus != "" {
			return locus
		}
	}
	return ""
}

// AddSample declares a sample, so that samples keep the order a source gives them even before
// their first row; it answers the sample's index.
func (b *Builder) AddSample(s Sample) uint32 {
	if i, ok := b.samples[s.Name]; ok {
		return i
	}
	i := uint32(len(b.t.Samples))
	b.t.Samples = append(b.t.Samples, s)
	b.samples[s.Name] = i
	return i
}

// Append adds a row. A row's sample is declared with the row's source when it was not before.
func (b *Builder) Append(r *Row) error {
	t := b.t
	sample, ok := b.samples[r.Sample]
	if !ok {
		sample = b.AddSample(Sample{Name: r.Sample, RepertoireID: r.RepertoireID, Source: r.Source})
	}
	if r.Source < 0 || r.Source > math.MaxUint16 {
		return fmt.Errorf("source index %d out of range", r.Source)
	}
	var codes [9]uint32
	var err error
	for i, v := range []string{r.V, r.D, r.J, r.C, r.Cell, r.Clone} {
		if codes[i], err = t.Dict.Add(v); err != nil {
			return err
		}
	}
	if codes[6], err = t.Junctions.Add(r.Junction); err != nil {
		return err
	}
	if codes[7], err = t.JunctionsAA.Add(r.JunctionAA); err != nil {
		return err
	}
	if codes[8], err = t.IDs.Add(r.SequenceID); err != nil {
		return err
	}
	n := len(t.Sample)
	locus := r.Locus
	if locus == "" {
		locus = b.callLocus(r)
	}
	t.Sample = append(t.Sample, sample)
	t.Locus = append(t.Locus, LocusCode(locus))
	t.V = append(t.V, codes[0])
	t.D = append(t.D, codes[1])
	t.J = append(t.J, codes[2])
	t.C = append(t.C, codes[3])
	t.Junction = append(t.Junction, codes[6])
	t.JunctionAA = append(t.JunctionAA, codes[7])
	t.SequenceID = append(t.SequenceID, codes[8])
	t.Productive = append(t.Productive, r.Productive)
	t.VJInFrame = append(t.VJInFrame, r.VJInFrame)
	t.StopCodon = append(t.StopCodon, r.StopCodon)
	t.Count = append(t.Count, r.Count)
	t.Source = append(t.Source, uint16(r.Source))
	t.Line = append(t.Line, r.Line)
	t.Reads = appendOptionalFloat(t.Reads, n, r.Reads)
	t.UMIs = appendOptionalFloat(t.UMIs, n, r.UMIs)
	t.Cell = appendOptionalCode(t.Cell, n, codes[4])
	t.Clone = appendOptionalCode(t.Clone, n, codes[5])
	t.IsCell = appendOptionalFlag(t.IsCell, n, r.IsCell)
	t.HighConfidence = appendOptionalFlag(t.HighConfidence, n, r.HighConfidence)
	t.FullLength = appendOptionalFlag(t.FullLength, n, r.FullLength)
	return nil
}

func appendOptionalFloat(column []float64, n int, v float64) []float64 {
	if column == nil {
		if math.IsNaN(v) {
			return nil
		}
		column = make([]float64, n, max(2*n, 16))
		for i := range column {
			column[i] = math.NaN()
		}
	}
	return append(column, v)
}

func appendOptionalCode(column []uint32, n int, v uint32) []uint32 {
	if column == nil {
		if v == 0 {
			return nil
		}
		column = make([]uint32, n, max(2*n, 16))
	}
	return append(column, v)
}

func appendOptionalFlag(column []Flag, n int, v Flag) []Flag {
	if column == nil {
		if v == Missing {
			return nil
		}
		column = make([]Flag, n, max(2*n, 16))
		for i := range column {
			column[i] = Missing
		}
	}
	return append(column, v)
}

// Table finishes the table: its string stores are frozen and its columns trimmed to their length.
func (b *Builder) Table() *Table {
	t := b.t
	t.Dict.Freeze()
	t.Junctions.Freeze()
	t.JunctionsAA.Freeze()
	t.IDs.Freeze()
	t.Sample = t.Sample[:len(t.Sample):len(t.Sample)]
	return t
}

// CountReads makes the reads of the rows of a source their counts: for a source whose counts
// cannot all be given in its first unit (immunoSEQ templates missing for some rows), so that its
// rows are counted alike.
func (b *Builder) CountReads(source int) {
	t := b.t
	for i := range t.Count {
		if int(t.Source[i]) != source {
			continue
		}
		if t.Reads == nil {
			t.Count[i] = math.NaN()
			continue
		}
		t.Count[i] = t.Reads[i]
	}
}

// Len is the number of rows so far.
func (b *Builder) Len() int { return len(b.t.Sample) }

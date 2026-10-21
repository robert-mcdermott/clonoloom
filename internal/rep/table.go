// Package rep holds the rearrangement table: every reader's output, whatever the source format
// (clonoloom-spec/design.md, "The rearrangement table"). It is columnar, with repeated strings
// interned, so that a cohort of tens of millions of rearrangements fits a laptop. Its fields are
// those of the AIRR Rearrangement schema ClonoLoom uses (Vander Heiden et al. 2018, Front Immunol
// 9:2206, doi:10.3389/fimmu.2018.02206); a source's other columns stay in the source, which is
// read again when an export needs them.
package rep

import (
	"math"
	"sync"

	"clonoloom/internal/genes"
)

// Unit is what a rearrangement's count counts.
type Unit string

const (
	// UnitUnknown: the source does not say (an AIRR file with only duplicate_count); the
	// validator asks the user to confirm one.
	UnitUnknown Unit = ""
	// Reads: sequencing reads (MiXCR's readCount, immunoSEQ's reads, an AIRR duplicate_count
	// confirmed as reads).
	Reads Unit = "reads"
	// UMIs: unique molecular identifiers (an AIRR umi_count, MiXCR's uniqueMoleculeCount).
	UMIs Unit = "umis"
	// Templates: immunoSEQ's estimate of input DNA molecules (about one per cell).
	Templates Unit = "templates"
	// Cells: single-cell data, where a clonotype's abundance is its number of cells.
	Cells Unit = "cells"
	// Sequences: each row counts once (no count given; a table of unique sequences).
	Sequences Unit = "sequences"
)

// Flag is a boolean that may be missing.
type Flag int8

const (
	Missing Flag = -1
	False   Flag = 0
	True    Flag = 1
)

// FlagOf is the flag of a boolean.
func FlagOf(b bool) Flag {
	if b {
		return True
	}
	return False
}

// Sample is a repertoire: one immunoSEQ file, one repertoire_id of an AIRR file, one Cell Ranger
// run.
type Sample struct {
	Name string `json:"name"`
	// RepertoireID: the AIRR repertoire_id (or the vendor's sample name) as given.
	RepertoireID string `json:"repertoireId,omitempty"`
	// Source: the index of the source file in the dataset.
	Source int `json:"source"`
	// Metadata: what the source says of the sample beside its rows (immunoSEQ's sample columns),
	// as given.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Table is a dataset's rearrangements, one row per rearrangement (bulk) or contig (single cell).
// Optional columns are nil when no source gave them.
type Table struct {
	// Dict interns the short strings that repeat: gene calls, samples' barcodes, clone ids.
	Dict *Strings
	// Junctions (nucleotides) and JunctionsAA (amino acids) intern the IMGT junctions.
	Junctions, JunctionsAA *Strings
	// IDs holds the sequence (contig) ids as given, not interned: each is a row's own.
	IDs *Strings

	Sample      []uint32 // index into Samples
	Locus       []uint8  // 1 + index into genes.Loci; 0 when not known
	V, D, J, C  []uint32 // Dict codes of the calls as given
	Junction    []uint32 // Junctions codes
	JunctionAA  []uint32 // JunctionsAA codes
	Productive  []Flag
	VJInFrame   []Flag
	StopCodon   []Flag
	Count       []float64 // the abundance in the dataset's unit; NaN when not given
	Reads, UMIs []float64 // optional: reads and UMIs where the unit is something else
	Cell        []uint32  // optional: Dict code of the cell (barcode, cell_id)
	Clone       []uint32  // optional: Dict code of the source's own clonotype or clone id
	SequenceID  []uint32  // IDs codes
	// Single-cell flags (Cell Ranger's is_cell, high_confidence, full_length), optional.
	IsCell, HighConfidence, FullLength []Flag
	// Where each row came from: the source's index in the dataset, and its line (1 is the
	// header).
	Source []uint16
	Line   []uint32

	Samples []Sample
	// Unit of Count.
	Unit Unit

	callsOnce sync.Once
	calls     []genes.Call // parsed Dict entries, by code
}

// Len is the number of rows.
func (t *Table) Len() int { return len(t.Sample) }

// LocusName is a row's locus ("TRB"), or "".
func (t *Table) LocusName(row int) string {
	if l := t.Locus[row]; l > 0 && int(l) <= len(genes.Loci) {
		return genes.Loci[l-1]
	}
	return ""
}

// LocusCode is the Locus column's code of a locus name (0 when not a locus).
func LocusCode(name string) uint8 {
	for i, l := range genes.Loci {
		if l == name {
			return uint8(i + 1)
		}
	}
	return 0
}

// Call is a Dict entry read as a gene call, parsed once per distinct entry.
func (t *Table) Call(code uint32) genes.Call {
	t.callsOnce.Do(func() {
		t.calls = make([]genes.Call, t.Dict.Len()+1)
		for i := 1; i <= t.Dict.Len(); i++ {
			t.calls[i] = genes.ParseCall(t.Dict.Get(uint32(i)))
		}
	})
	if int(code) >= len(t.calls) {
		return genes.Call{}
	}
	return t.calls[code]
}

// Size is the memory the table takes, in bytes, without its interning indexes.
func (t *Table) Size() int64 {
	size := t.Dict.Size() + t.Junctions.Size() + t.JunctionsAA.Size() + t.IDs.Size()
	n := int64(len(t.Sample))
	size += n * (4 + 1 + 16 + 8 + 3 + 8 + 4 + 2 + 4)
	if t.Reads != nil {
		size += 8 * n
	}
	if t.UMIs != nil {
		size += 8 * n
	}
	if t.Cell != nil {
		size += 4 * n
	}
	if t.Clone != nil {
		size += 4 * n
	}
	for _, f := range [][]Flag{t.IsCell, t.HighConfidence, t.FullLength} {
		if f != nil {
			size += n
		}
	}
	return size
}

// NaN is a missing count.
var NaN = math.NaN()

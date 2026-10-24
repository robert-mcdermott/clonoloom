// Artificial annotation cases, not biological receptors. Run with Node from any directory.
import { writeFileSync } from 'node:fs';
const base = { sequence_id: 'complete', sequence: 'ACGTACGTACGT', rev_comp: 'F', productive: 'T', locus: 'IGH', v_call: 'IGHV1-2*01', d_call: '', j_call: 'IGHJ4*01', c_call: 'IGHM', sequence_alignment: 'ACGTACGTACGT', germline_alignment: 'ACGTACGTACGT', junction: 'TGTGCTTTT', junction_aa: 'CAF', v_cigar: '12M', d_cigar: '', j_cigar: '', umi_count: '3', repertoire_id: 'Synthetic evidence', v_sequence_start: '1', v_sequence_end: '12', v_germline_start: '1', v_germline_end: '12', v_alignment_start: '1', v_alignment_end: '12', fwr1_start: '1', fwr1_end: '6', np1_length: '0', germline_database: 'synthetic: no biological reference' };
const rows = [
  {},
  { sequence_id: 'partial', sequence: '', sequence_alignment: '', germline_alignment: '', v_cigar: '', v_sequence_start: '', v_sequence_end: '', fwr1_start: '', fwr1_end: '' },
  { sequence_id: 'ambiguous', sequence: 'ACGTNCGTACGT', sequence_alignment: 'ACGTNCGTACGT', locus: 'IGK', v_call: 'IGKV1-5*01', j_call: 'IGKJ1*01', c_call: 'IGKC' },
  { sequence_id: 'unequal', germline_alignment: 'ACGT', locus: 'IGL', v_call: 'IGLV1-40*01', j_call: 'IGLJ1*01', c_call: 'IGLC1' },
  { sequence_id: 'bad-coordinate', fwr1_end: '999999999999999999999999' },
  { sequence_id: 'bad-cigar', v_cigar: '12H' },
  { sequence_id: 'reverse', rev_comp: 'T' },
];
const fields = Object.keys(base);
writeFileSync(new URL('./bcell-inputs.tsv', import.meta.url), `${fields.join('\t')}\n${rows.map((r) => { const row = { ...base, ...r }; return fields.map((f) => row[f]).join('\t'); }).join('\n')}\n`);

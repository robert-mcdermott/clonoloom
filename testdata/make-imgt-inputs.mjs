// Artificial IMGT tables following the published column contract; no reference sequences.
// Run: node testdata/make-imgt-inputs.mjs
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
const dir = fileURLToPath(new URL('./imgt-inputs/', import.meta.url));
mkdirSync(dir, { recursive: true });
const common = ['Sequence number', 'Sequence ID', 'V-DOMAIN Functionality', 'V-GENE and allele', 'J-GENE and allele', 'D-GENE and allele'];
const rows = [
  ['1', 'heavy', 'productive', 'Homsap IGHV1-2*02 F, or Homsap IGHV1-2*01 F', 'Homsap IGHJ4*02 F', 'Homsap IGHD3-10*01 F'],
  ['2', 'light', 'productive (see comment)', 'Homsap IGKV1-39*01 F', 'Homsap IGKJ1*01 F', ''],
  ['3', 'heavy', 'unproductive', 'Homsap IGHV3-23*01 F', 'Homsap IGHJ4*02 F', ''],
  ['4', 'failed', 'No results', '', '', ''],
];
const sequences = ['aaatgtgcttggccc', 'gggtgtcaatttaaa', 'aaatgtAgcttggccc', 'NNNN'];
const junctions = ['tgtgcttgg', 'tgtcaattt', 'tgtgcttgg', ''];
function table(name, extra, values, order) {
  const lines = [[...common, ...extra], ...order.map((i) => [...rows[i], ...values[i]])];
  writeFileSync(`${dir}${name}.txt`, lines.map((r) => r.join('\t')).join('\n') + '\n');
}
table('1_Summary', ['AA JUNCTION', 'JUNCTION frame', 'Orientation', 'V-DOMAIN Functionality comment', 'V-REGION insertions', 'V-REGION deletions', 'Sequence'], [
  ['CAW', 'in-frame', '+', '', '', '', sequences[0]],
  ['CQF', 'in-frame', '-', 'artificial reverse submission', '', '', sequences[1]],
  ['CA#W', 'out-of-frame', '+', 'artificial corrected insertion', '7, 1 nt', '', sequences[2]],
  ['', '', '', 'No rearrangement identified', '', '', sequences[3]],
], [0, 1, 2, 3]);
table('2_IMGT-gapped-nt-sequences', ['V-D-J-REGION', 'V-J-REGION', 'V-REGION'], [
  ['aaa...tgtgcttggccc', '', 'aaa...tgt'],
  ['', 'ggg...tgtcaatttaaa', 'ggg...tgt'],
  ['aaa...tgtgcttggccc', '', 'aaa...tgt'],
  ['', '', ''],
], [2, 0, 3, 1]);
table('3_Nt-sequences', ['V-REGION', 'V-REGION start', 'V-REGION end', 'J-REGION', 'J-REGION start', 'J-REGION end', 'CDR3-IMGT', 'CDR3-IMGT start', 'CDR3-IMGT end'], [
  ['aaatgt', '1', '6', 'tggccc', '10', '15', 'gct', '7', '9'],
  ['gggtgt', '1', '6', 'tttaaa', '10', '15', 'caa', '7', '9'],
  ['aaatgt', '1', '6', 'tggccc', '10', '15', 'gct', '7', '9'],
  ['', '', '', '', '', '', '', '', ''],
], [1, 3, 0, 2]);
table('6_Junction', ['JUNCTION frame', 'JUNCTION', 'JUNCTION (AA)', 'JUNCTION (AA) (with frameshift)'], [
  ['in-frame', junctions[0], 'CAW', ''],
  ['in-frame', junctions[1], 'CQF', ''],
  ['out-of-frame', junctions[2], '', 'CA#W'],
  ['', '', '', ''],
], [3, 2, 1, 0]);
writeFileSync(`${dir}11_Parameters.txt`, 'Synthetic format fixture\nIMGT/V-QUEST program version\tNot run\nIMGT/V-QUEST reference directory release\tNone\n');

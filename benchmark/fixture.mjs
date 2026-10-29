// Artificial AIRR contract fixture, not a population simulation or biological finding.
// AIRR Rearrangement standard: Vander Heiden et al. 2018, doi:10.3389/fimmu.2018.02206.
// Counts and group identities are deliberately enumerated, independent of ClonoLoom's engine.
import {createHash} from 'node:crypto';

export const FIXTURE_VERSION = 1;
export const SAMPLES = ['P01-baseline', 'P01-followup', 'P02-baseline', 'P02-followup'];
const codons = {C:'TGT', A:'GCT', S:'TCT', L:'CTG', G:'GGT', Q:'CAG', E:'GAA', T:'ACT', Y:'TAT', F:'TTT', N:'AAT', I:'ATT', R:'CGT'};
const nt = aa => [...aa].map(a => codons[a]).join('');
export const digest = value => createHash('sha256').update(value).digest('hex');

export function fixture(variant = 1) {
  if (!Number.isInteger(variant) || variant < 1 || variant > 100) throw new Error('variant must be 1–100');
  const k = variant;
  const groups = [
    ['CASSLGQETQYF','TRBV5-1*01','TRBJ2-5*01',[40*k,60*k,10*k,20*k]],
    ['CASSLGQETQYF','TRBV5-1*01','TRBJ2-5*01',[5*k,0,3*k,0]],
    ['CASSLGQETQYF','TRBV6-5*01','TRBJ2-5*01',[10*k,10*k,20*k,10*k]],
    ['CASSQGANTEAFF','TRBV7-9*01','TRBJ1-1*01',[20*k,10*k,10*k,10*k]],
    ['CASSYSGNEQFF','TRBV5-1*01','TRBJ2-1*01',[0,5*k,0,5*k]],
    ['CASSIRSSYEQYF','TRBV9*01','TRBJ2-7*01',[null,null,null,null]],
    ['CASSLAGGTEAFF','TRBV10-1*01','TRBJ1-1*01',[0,0,0,0]],
    ['CASSFGQETQYF','TRBV5-1*01','TRBJ2-5*01',[7*k,7*k,7*k,7*k]],
  ];
  // Required AIRR columns with unknown full-sequence/alignment fields left null.
  const header = 'sequence_id\tlocus\tproductive\tv_call\tj_call\tjunction\tjunction_aa\tumi_count\trepertoire_id\tsequence\trev_comp\td_call\tsequence_alignment\tgermline_alignment\tv_cigar\td_cigar\tj_cigar\n';
  const files = Object.fromEntries(SAMPLES.map((sample, s) => [sample + '.tsv', header + groups.map(([aa,v,j,counts], g) => {
    // A synonymous nucleotide alternative shares its amino-acid/V/J group.
    const junction = g === 1 ? nt(aa).replace('GCT','GCC') : nt(aa);
    return [`row-${s+1}-${g+1}`,'TRB',g===7?'F':'T',v,j,junction,aa,counts[s]??'',sample,'','','','','','','',''].join('\t');
  }).join('\n') + '\n']));
  const annotations = Object.fromEntries(SAMPLES.map((s,i) => [s,{subject:i<2?'P01':'P02',timepoint:i%2?'day 14':'baseline',condition:'artificial control'}]));
  const rows = [
    {junction:'CASSLGQETQYF',v:'TRBV5-1',j:'TRBJ2-5',counts:[45,60,13,20]},
    {junction:'CASSLGQETQYF',v:'TRBV6-5',j:'TRBJ2-5',counts:[10,10,20,10]},
    {junction:'CASSQGANTEAFF',v:'TRBV7-9',j:'TRBJ1-1',counts:[20,10,10,10]},
    {junction:'CASSYSGNEQFF',v:'TRBV5-1',j:'TRBJ2-1',counts:[0,5,0,5]},
    {junction:'CASSLAGGTEAFF',v:'TRBV10-1',j:'TRBJ1-1',counts:[0,0,0,0]},
  ].map(r=>({...r,counts:r.counts.map(n=>n*k)}));
  const mergedRows = [{junction:rows[0].junction,counts:[55,70,33,30].map(n=>n*k)},...rows.slice(2)].map(({junction,counts})=>({junction,v:'',j:'',counts}));
  const truth = {count:5, total:248*k, top:138*k, mergedTop:188*k, mergedCount:4, matched:2, missingRows:4, zeroClonotypes:1, unit:'umis', subjects:2, observations:4,rows,mergedRows,samples:SAMPLES};
  return {version:FIXTURE_VERSION,variant,files,annotations,truth,sha256:digest(JSON.stringify({files,annotations}))};
}

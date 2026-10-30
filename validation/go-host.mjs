// Run every top-level host test and fuzz seed once across independent race shards.
// node validation/go-host.mjs SHARD (1..8); --list only prints the complete partition.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';

export const SHARDS=8;
// Relative scheduling costs from the full 0.9.0 race run, not performance budgets.
// New tests are discovered automatically; these hints only keep the jobs balanced.
const costs={
  TestCertificateReplayReleaseCohortControls:331,TestPortableArchivesReopenReleaseProjects:279,
  TestPseudobulkDETruthReplayExports:262,TestCertificateReplayDiffersAndUnavailable:191,
  TestLineageLikelihoodReview:114,TestEveryReleasesProjectStillOpens:101,
  TestHTMLReportsExactEvidenceAndReplay:79,TestMotifArtifactTruthAndReplay:69,
  TestCertificateReplayExactWorkers:68,TestHeadlessSourceWorkersExactAndPortable:65,
  TestCertificateAPIAndSourceFreeIntegrity:53,TestSuiteReleaseEvidence:44,
  TestPseudobulkTruthReplayExports:25,TestCandidatesNativeEvidenceAndPermissions:21,
  TestConvergenceTruthReplayAndExports:14,TestWorkbookOversizedEvidence:14,TestWorkbookSavedEvidence:13,
};
export function partition(names,count=SHARDS){
  assert(Number.isInteger(count)&&count>0);
  assert(names.length>0&&new Set(names).size===names.length,'Missing or duplicate discovered tests');
  assert(names.every(n=>/^(Test|Fuzz|Example)[\p{L}\p{Nd}_]*$/u.test(n)),'Unexpected Go test name');
  const bins=Array.from({length:count},()=>({names:[],cost:0}));
  for(const name of [...names].sort((a,b)=>(costs[b]??1)-(costs[a]??1)||a.localeCompare(b))){
    const bin=bins.reduce((a,b)=>a.cost<=b.cost?a:b);bin.names.push(name);bin.cost+=costs[name]??1;
  }
  const result=bins.map(b=>b.names.sort());
  assert.deepEqual(result.flat().sort(),[...names].sort(),'Incomplete test partition');
  return result;
}
export const selection=names=>'^('+names.join('|')+')$';

if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  const arg=process.argv[2];assert(process.argv.length===3&&(arg==='--list'||/^[1-8]$/.test(arg)),'Use a shard number 1..8 or --list');
  const root=resolve(import.meta.dirname,'..');
  const listed=spawnSync('go',['test','-race','-list','.','.'],{cwd:root,encoding:'utf8',maxBuffer:4*1024*1024});
  if(listed.status!==0){process.stderr.write(listed.stderr??'');throw listed.error??new Error('Go test discovery failed');}
  const names=listed.stdout.split(/\r?\n/).filter(n=>/^(Test|Fuzz|Example)\S*$/.test(n));
  const bins=partition(names);
  if(arg==='--list')console.log(JSON.stringify({total:names.length,shards:bins},null,2));
  else{
    const selected=bins[Number(arg)-1];assert(selected.length,'Empty test shard');
    console.log(`Host race shard ${arg}/${SHARDS}: ${selected.length}/${names.length} discovered tests and fuzz targets; all subtests retained.`);
    const run=spawnSync('go',['test','-race','-count=1','-v','-timeout','15m','-run',selection(selected),'.'],{cwd:root,stdio:'inherit'});
    if(run.error)throw run.error;process.exitCode=run.status??1;
  }
}

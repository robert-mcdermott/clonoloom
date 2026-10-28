import test from 'node:test';
import assert from 'node:assert/strict';
import { certificateResults, integrityLabel } from './certificates.js';

test('only supported saved result kinds are offered for certificates', () => {
  const p = { artifacts: [{id:'f',kind:'cohort-features'}, {id:'c',kind:'cohort-compare'}, {id:'n',kind:'cohort-counts'}, {id:'b',kind:'batch-qc'}, {id:'m',kind:'motifs'}, {id:'d',kind:'diversity'}] };
  assert.deepEqual(certificateResults(p).map((r) => r.id), ['f','c','n','b','m']);
  assert.equal(p.artifacts.length, 6);
});
test('integrity display never calls bytes reproduced or scientific evidence verified', () => {
  const r = {format:'clonoloom-verification',version:1,scope:'recorded-artifact-bytes',recomputed:false,verdict:'integrity-match'};
  assert.equal(integrityLabel(r), 'Integrity match — not recomputed');
  assert.equal(integrityLabel({...r, verdict:'differs'}), 'Integrity check failed');
  for (const patch of [{version:2},{recomputed:true},{scope:'recomputed'},{format:'other'}]) assert.equal(integrityLabel({...r,...patch}), 'Unsupported verification report');
});

test('replay labels require exact output and metadata matches and state the captured boundary', async () => {
  const { verificationLabel } = await import('./certificates.js');
  const r = { format:'clonoloom-replay', version:1, scope:'captured-cohort-evidence', verdict:'replay-match', recomputed:true, integrity:{verdict:'integrity-match'}, steps:[{verdict:'replay-match',metadataMatched:true,expectedSHA256:'a',actualSHA256:'a'}] };
  assert.equal(verificationLabel(r), 'Exact replay match — captured evidence');
  for (const scope of ['captured-batch-qc-evidence', 'captured-motif-evidence']) assert.equal(verificationLabel({...r,scope}), 'Exact replay match — captured evidence');
  assert.equal(verificationLabel({...r,scope:'original-sources'}), 'Unsupported verification report');
  for (const patch of [{version:2},{recomputed:false},{steps:[]},{integrity:{verdict:'differs'}},{steps:[{...r.steps[0],actualSHA256:'b'}]},{steps:[{...r.steps[0],metadataMatched:false}]}]) assert.equal(verificationLabel({...r,...patch}), 'Unsupported verification report');
  assert.equal(verificationLabel({...r,verdict:'unavailable',recomputed:false}), 'Replay unavailable');
  assert.equal(verificationLabel({...r,verdict:'differs'}), 'Replay differs');
});

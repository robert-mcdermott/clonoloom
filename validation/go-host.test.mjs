import assert from 'node:assert/strict';
import {test} from 'node:test';
import {readFileSync} from 'node:fs';
import {partition,selection,SHARDS} from './go-host.mjs';

test('new host tests and fuzz seeds are selected exactly once without prefix collisions',()=>{
  const names=['TestCertificateReplayReleaseCohortControls','TestPortableArchivesReopenReleaseProjects','TestPseudobulkDETruthReplayExports',...Array.from({length:100},(_,i)=>'TestNew'+i),'TestΔ','FuzzSource','ExampleHost'];
  const bins=partition(names);
  assert.equal(bins.length,SHARDS);
  for(const name of names)assert.equal(bins.filter(bin=>new RegExp(selection(bin)).test(name)).length,1,name);
  for(const bin of bins)assert(!new RegExp(selection(bin)).test('TestNew1000'));
  assert.deepEqual(partition([...names].reverse()),bins,'discovery order changes coverage');
  assert.equal(bins.filter(b=>b.some(n=>['TestCertificateReplayReleaseCohortControls','TestPortableArchivesReopenReleaseProjects','TestPseudobulkDETruthReplayExports'].includes(n))).length,3);
});

test('invalid discovery and duplicate names fail instead of silently omitting tests',()=>{
  for(const names of [[],['TestA','TestA'],['TestA|TestB'],['not a test']])assert.throws(()=>partition(names));
});

test('CI schedules every implemented shard',()=>{
  const workflow=readFileSync(new URL('../.github/workflows/ci.yml',import.meta.url),'utf8');
  const matrix=workflow.match(/^\s+shard: \[([\d, ]+)\]$/m);assert(matrix,'Missing host shard matrix');
  assert.deepEqual(matrix[1].split(',').map(Number),Array.from({length:SHARDS},(_,i)=>i+1));
});

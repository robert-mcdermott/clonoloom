import assert from 'node:assert/strict';
import {test} from 'node:test';
import {processResult} from './harness.mjs';

test('commands needing no input start with stdin already closed',async()=>{
  const script="const s=require('node:fs').fstatSync(0);process.stdout.write(s.isFIFO()||s.isSocket()?'pipe':'closed-input')";
  assert.equal(await processResult(process.execPath,['-e',script]),'closed-input');
  for(let i=0;i<20;i++)assert.equal(await processResult(process.execPath,['-e',"process.stdout.write('exact')"]),'exact');
});

test('input is delivered unchanged and output remains exact',async()=>{
  const input='{"integer":9007199254740993,"decimal":0.123456789012345678901}\n';
  assert.equal(await processResult(process.execPath,['-e','process.stdin.pipe(process.stdout)'],{input}),input);
});

test('early child failure retains exit status and stderr instead of EPIPE',async()=>{
  await assert.rejects(processResult(process.execPath,['-e',"require('node:fs').writeSync(2,'deliberate permission refusal');process.exit(7)"],{input:'x'.repeat(1024*1024)}),/exited 7: deliberate permission refusal/);
});

test('a broken input pipe does not become successful evidence',async()=>{
  await assert.rejects(processResult(process.execPath,['-e','process.exit(0)'],{input:'x'.repeat(1024*1024)}),/stdin.*EPIPE/);
});

test('missing commands and hung children still fail',async()=>{
  await assert.rejects(processResult('/nonexistent-clonoloom-benchmark-command',[]),/ENOENT/);
  await assert.rejects(processResult(process.execPath,['-e','setInterval(()=>{},1000)'],{timeout:1000}),/deadline exceeded/);
});

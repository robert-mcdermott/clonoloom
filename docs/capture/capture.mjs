// Captures the documentation's screenshots, in the light and dark themes, as
// docs/images/<scene>-<theme>.webp: the README shows the one matching the reader's theme. Each
// scene starts a ClonoLoom built from source on a fresh, empty library, with the development data
// as its examples (node validation/fetch.mjs --ci downloads them), and drives it through its
// remote control (remote.go, web/ui/remote.js), as any script can, so every run gives the same
// pictures. Only openly licensed data are shown. Follows SituCraft 0.9.0's
// docs/capture/capture.mjs.
//
//   node docs/capture/capture.mjs [scene …] [--theme light|dark|both]
//
// Needs Go (to build ClonoLoom) and Chrome, Chromium, Edge or Brave (CHROME=path).

import { spawn } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { launch, sleep } from '../../validation/cdp.mjs';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const IMAGES = join(ROOT, 'docs/images');
// A port of ClonoLoom's range the development server (8878) does not use.
const PORT = 8877;
const DATA = process.env.CLONOLOOM_DATA || join(ROOT, 'validation/cache');
// What the pictures show as the places of the library and the examples (not this run's temporary
// ones): macOS's defaults.
const SHOWN = {
  library: '~/Library/Application Support/ClonoLoom',
  examples: '~/Library/Application Support/ClonoLoom/examples',
};

const args = process.argv.slice(2);
const option = (name) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : null;
};
const themes = { light: ['light'], dark: ['dark'], both: ['light', 'dark'] }[option('theme') ?? 'both'];
if (!themes) throw new Error('--theme is light, dark or both');
const wanted = args.filter((a, i) => !a.startsWith('--') && args[i - 1] !== '--theme');
for (const set of ['10x-pbmc-1k', '10x-pbmc-10k-v2']) {
  if (!existsSync(join(DATA, set))) throw new Error(`The scenes need the development data: run node validation/fetch.mjs --ci (${set} is missing)`);
}

// --- ClonoLoom -----------------------------------------------------------------------------------

const temp = mkdtempSync(join(tmpdir(), 'clonoloom-capture-'));
const binary = join(temp, process.platform === 'win32' ? 'clonoloom.exe' : 'clonoloom');
let URL = '';
let token = '';
let places = {};

// Started per scene on an empty library, so that no scene sees another's projects. examples: the
// examples folder (the development data by default; an empty one shows them to download).
async function startClonoLoom({ examples = DATA } = {}) {
  const library = mkdtempSync(join(temp, 'library-'));
  places = { library, examples };
  const server = spawn(binary, ['--remote-control', '--window', 'none', '--port', String(PORT), '--data-dir', library, '--examples-dir', examples, '--cache-dir', join(temp, 'cache')], { cwd: temp, stdio: ['ignore', 'pipe', 'inherit'] });
  let output = '';
  server.stdout.on('data', (chunk) => { output += chunk; });
  const stop = async () => {
    const exited = new Promise((done) => server.once('exit', done));
    server.kill();
    await Promise.race([exited, sleep(3000)]);
  };
  for (let i = 0; i < 150; i += 1) {
    const address = /running at (http:\/\/[\d.]+:\d+)/.exec(output)?.[1];
    if (address && existsSync(join(library, 'remote.json'))) {
      URL = `${address}/`;
      token = JSON.parse(readFileSync(join(library, 'remote.json'), 'utf8')).token;
      return { stop };
    }
    await sleep(100);
  }
  await stop();
  throw new Error(`ClonoLoom did not start:\n${output}`);
}

// An action through remote control, as any script sends it.
async function act(action, actionArgs = {}) {
  const response = await fetch(`${URL}api/remote/action`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-ClonoLoom-Token': token }, body: JSON.stringify({ action, args: actionArgs, client: 'docs/capture' }) });
  const result = await response.json();
  if (!result.ok) throw new Error(`${action}: ${result.message ?? result.error}`);
  return result;
}

// --- Page helpers --------------------------------------------------------------------------------

let b;
const js = (expression) => b.eval(expression);

async function waitFor(expression, timeout = 60000) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    if (await js(expression)) return true;
    await sleep(200);
  }
  throw new Error(`Timed out waiting for ${expression}`);
}

const clearToasts = () => js(`document.querySelectorAll('.toast').forEach((t) => t.remove())`);

// The places shown in the window, as SHOWN names them: every text node and title that holds this
// run's temporary folders.
async function showPlaces() {
  const pairs = Object.entries(places).map(([k, v]) => [v, SHOWN[k]]).sort((a, c) => c[0].length - a[0].length);
  await js(`(() => {
    const pairs = ${JSON.stringify(pairs)};
    const fix = (text) => pairs.reduce((t, [from, to]) => t.split(from).join(to), text);
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) if (pairs.some(([from]) => node.nodeValue.includes(from))) node.nodeValue = fix(node.nodeValue);
    for (const el of document.querySelectorAll('[title]')) el.title = fix(el.title);
    return true;
  })()`);
}

async function checkAccessibility() {
  // Modal/toast entrance opacity is not the resting contrast of the UI.
  await sleep(250);
  const axe = join(DATA, 'axe-core/axe.min.js');
  if (!existsSync(axe)) throw new Error('The study accessibility check needs axe-core (node validation/fetch.mjs axe-core)');
  await js(`(0, eval)(${JSON.stringify(readFileSync(axe, 'utf8'))}); true`);
  const violations = await js(`(async () => {
    const result = await axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] }, resultTypes: ['violations'] });
    return result.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }));
  })()`);
  if (violations.length) throw new Error(`Study accessibility: ${JSON.stringify(violations)}`);
}

// Cohort comparisons use only the saved subject table, with the raw observations
// and mean-effect interval captured separately from the probability effect table.
async function cohortComparisonScene(paired) {
  await act('new_project', { name: paired ? 'Artificial paired subject comparison' : 'Artificial independent subject comparison' });
  await act('open_example', { example: 'simulated-standard', download: true });
  await act('measure_diversity');
  const spec = { unit: 'subject', layout: paired ? 'paired' : 'independent', groupField: paired ? 'timepoint' : 'condition', levels: paired ? ['day 0','day 7'] : ['control','expanded'], exclude: {} };
  if (paired) {
    const samples = {};
    for (let i=1;i<=6;i++) for (const j of [1,2]) samples[`S${String(i).padStart(2,'0')}-${j}`] = {timepoint:j===1?'day 0':'day 7'};
    await act('annotate_samples', {samples});
  } else for (let i=1;i<=6;i++) spec.exclude[`S${String(i).padStart(2,'0')}-2`]='Teaching selection: first observation per subject';
  await act('check_cohort_design', {spec,name:paired?'Paired simulated observations':'First simulated observations',save:true});
  const state=(await act('get_state')).data;
  const feature=(await act('build_cohort_features',{design:state.cohortDesigns[0].id,diversity:state.artifacts.find((r)=>r.kind==='diversity').id})).data.artifact;
  const options={features:feature,options:{features:['shannon','hill1'],confidence:.95,permutations:9999,seed:20261025}};
  const result=(await act('compare_cohort',options)).data;
  if(result.family!==4||result.tested!==4)throw new Error('Wrong comparison family');
  await js(`(async()=>{
    const app=window.clonoloom,v=app.cohortCompareView,answer=v.state().answer;
    const c=answer.result.analysis.comparisons[0];
    if(c.points.length!==${paired?12:6}||c.pairs!==${paired?6:0})throw new Error('Wrong subject units');
    if(document.querySelectorAll('.cohort-compare [data-sample]').length!==${paired?12:6}||document.querySelectorAll('.cohort-compare [data-subject]').length!==${paired?6:0})throw new Error('Point or pair geometry changed');
    const id=v.state().artifact,snapshot=JSON.stringify(answer.result);
    app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===id))throw new Error('Comparison undo failed');
    app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id});
    window.comparisonCheck={id,snapshot};return true;
  })()`);
  await waitFor(`window.clonoloom.cohortCompareView?.state().status==='ready'`);
  await js(`(async()=>{
    const app=window.clonoloom,v=app.cohortCompareView;
    if(JSON.stringify(v.state().answer.result)!==window.comparisonCheck.snapshot)throw new Error('Comparison changed on reopen');
    await v.choose(${JSON.stringify(feature)});
    const checkbox=document.querySelector('[aria-label="Compare Hill diversity q=1"]');checkbox.click();
    [...document.querySelectorAll('.cohort-compare button')].find((b)=>b.textContent==='Compare selected features').click();return true;
  })()`);
  await waitFor(`window.clonoloom.cohortCompareView.state().status==='ready'`);
  if(await js(`window.clonoloom.cohortCompareView.state().artifact`)!==result.artifact)throw new Error('Visible settings differ from remote settings');
  await js(`(async()=>{
    const app=window.clonoloom,v=app.cohortCompareView,ensure=app.ensureDataset,create=URL.createObjectURL;let blob;
    app.ensureDataset=async()=>{throw new Error('Sources unavailable')};URL.createObjectURL=(x)=>{blob=x;return create(x)};
    try{await v.show(${JSON.stringify(result.artifact)});await v.download();const bytes=new Uint8Array(await blob.arrayBuffer());if(bytes[0]!==80||bytes[1]!==75)throw new Error('Comparison ZIP unavailable without sources');}
    finally{app.ensureDataset=ensure;URL.createObjectURL=create;}
    const {figureBlob}=await import('./ui/figures.js');
    for(const which of ['observations','effect']){const text=await (await figureBlob(v.plotSpec(which),'svg')).text();if(!text.includes('definition')||!text.includes('BH does not adjust intervals')||!text.includes('Contrast:')||!text.includes('Selected observations:'))throw new Error('Figure lost provenance');}
    return true;
  })()`);
  await act('show_methods');
  if(!await js(`document.querySelector('.methods-text').textContent.includes('BH family')`))throw new Error('Comparison methods absent');
  await act('close_dialogs');
  await js(`(()=>{const root=document.querySelector('.cohort-compare'),plot=root.querySelector('figure');root.closest('.workbench-scroll').scrollTop=plot.offsetTop-65;return true})()`);
  await clearToasts();await checkAccessibility();
}

async function countModelScene(paired) {
  await act('new_project',{name:paired?'Paired repertoire count study':'Independent repertoire count study'});
  await act('open_example',{example:paired?'simulated-counts-paired':'simulated-counts-independent',download:true});
  const annotations={};
  for(let i=0;i<(paired?28:32);i++)for(let j=0;j<(paired?2:1);j++)annotations[`S${String(i+1).padStart(2,'0')}-${j+1}`]={batch:`B${(i+j)%2+1}`,...(paired?{timepoint:j?'day 14':'day 0'}:{})};
  await act('annotate_samples',{samples:annotations});
  await act('measure_diversity');
  await act('check_cohort_design',{name:paired?'Before and after stimulation':'Independent control and stimulated subjects',save:true,spec:{unit:'subject',layout:paired?'paired':'independent',groupField:paired?'timepoint':'condition',levels:paired?['day 0','day 14']:['control','stimulated'],exclude:{}}});
  const state=(await act('get_state')).data;
  const features=(await act('build_cohort_features',{design:state.cohortDesigns[0].id,diversity:state.artifacts.find((r)=>r.kind==='diversity').id})).data.artifact;
  const options={minCount:10,minSamples:12,adjustBatch:true,confidence:.95};
  const answer=(await act('fit_cohort_counts',{features,options})).data;
  if(answer.tested<25||answer.family>48)throw new Error(`Unexpected model coverage ${JSON.stringify(answer)}`);
  await js(`(async()=>{
    const app=window.clonoloom,v=app.cohortCountsView,doc=v.state().answer.result,r=doc.analysis;
    if(r.design.rows.length!==${paired?56:32}||r.design.residualDF<24)throw new Error('Count design wrong');
    if(getComputedStyle(document.querySelector('.cohort-counts .plot-axis')).fill!=='none')throw new Error('Count axes unexpectedly filled');
    if(document.querySelectorAll('.cohort-counts [data-clone]').length!==r.tested||document.querySelectorAll('.cohort-counts [data-sample]').length!==${paired?56:32}||document.querySelectorAll('.cohort-counts [data-subject]').length!==${paired?28:0})throw new Error('Count plot geometry wrong');
    window.countCheck={id:v.state().artifact,snapshot:JSON.stringify(doc)};
    app.store.undo();if(app.store.project.artifacts.some((a)=>a.id===window.countCheck.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:window.countCheck.id});return true;
  })()`);
  await waitFor(`window.clonoloom.cohortCountsView?.state().status==='ready'`);
  await js(`(()=>{const v=window.clonoloom.cohortCountsView;if(JSON.stringify(v.state().answer.result)!==window.countCheck.snapshot)throw new Error('Saved count model changed');v.choose(${JSON.stringify(features)});document.querySelector('[aria-label="Adjust count model for batch"]').checked=true;[...document.querySelectorAll('.cohort-counts button')].find((b)=>b.textContent==='Fit replicated counts').click();return true})()`);
  await waitFor(`window.clonoloom.cohortCountsView.state().status==='ready'`);
  await js(`(async()=>{
    const app=window.clonoloom,v=app.cohortCountsView;if(v.state().artifact!==window.countCheck.id)throw new Error('Visible settings differ');
    const ensure=app.ensureDataset,create=URL.createObjectURL;let blob;app.ensureDataset=async()=>{throw new Error('Source unavailable')};URL.createObjectURL=(b)=>{blob=b;return create(b)};
    try{await v.show(window.countCheck.id);await v.download();const b=new Uint8Array(await blob.arrayBuffer());if(b[0]!==80||b[1]!==75)throw new Error('ZIP failed');}finally{app.ensureDataset=ensure;URL.createObjectURL=create;}
    const {figureBlob}=await import('./ui/figures.js');for(const which of ['ma','observations','effect']){const text=await(await figureBlob(v.plotSpec(which),'svg')).text();if(!text.includes('Contrast:')||!text.includes('conditional on fitted dispersion'))throw new Error('Count figure lost method/provenance');}
    return true;
  })()`);
  await act('show_methods');if(!await js(`document.querySelector('.methods-text').textContent.includes('Cox–Reid')`))throw new Error('Count methods absent');await act('close_dialogs');
  await js(`(()=>{const root=document.querySelector('.cohort-counts');root.closest('.workbench-scroll').scrollTop=root.querySelector('.count-result-title').offsetTop-65;return true})()`);
  await clearToasts();await checkAccessibility();
}

async function definitionAuditScene() {
  await act('new_project',{name:'Definition-sensitive repertoire study'});
  await act('open_example',{example:'simulated-definition-study',download:true});
  const samples={};for(let i=0;i<32;i++)samples[`S${String(i+1).padStart(2,'0')}-1`]={batch:`B${i%2+1}`};
  await act('annotate_samples',{samples});await act('measure_diversity');
  await act('check_cohort_design',{name:'Control and stimulated subjects',save:true,spec:{unit:'subject',layout:'independent',groupField:'condition',levels:['control','stimulated'],exclude:{}}});
  const state=(await act('get_state')).data;
  const features=(await act('build_cohort_features',{design:state.cohortDesigns[0].id,diversity:state.artifacts.find((r)=>r.kind==='diversity').id})).data.artifact;
  const comparison=(await act('compare_cohort',{features,options:{features:['hill1','richness','shannon'],confidence:.95,permutations:999,seed:20261025}})).data.artifact;
  await js(`window.clonoloom.robustnessView.choose(${JSON.stringify(comparison)})`);
  const definitions=await js(`(()=>{const p=window.clonoloom.cohortCompareView.state().answer.result.features.parent.definition;return [['nt','gene','NT · V/J genes'],['aa','allele','AA · V/J alleles'],['aa','none','AA · junction only']].map(([junction,r,name])=>({...p,junction,v:r,j:r,name}))})()`);
  const answer=(await act('audit_definitions',{comparison,definitions})).data;
  if(answer.family!==24)throw new Error('Wrong audit family');
  await js(`(async()=>{
    const app=window.clonoloom,v=app.robustnessView;v.select('richness');const doc=v.state().answer.result,r=doc.analysis;
    if(r.variants.length!==4||r.variants.some((x)=>x.table.rows.length!==32)||r.summaries.find((x)=>x.feature.id==='richness').direction!=='Effect direction changes across definitions')throw new Error('Audit lost definition-dependent richness');
    if(document.querySelectorAll('.definition-robustness [data-estimate]').length!==4||document.querySelectorAll('.definition-robustness [data-lower]').length!==4)throw new Error('Forest geometry wrong');
    if(JSON.stringify(r.variants[0].analysis)!==JSON.stringify(doc.parent.analysis))throw new Error('Baseline changed');
    window.auditCheck={id:v.state().artifact,snapshot:JSON.stringify(doc)};
    app.store.undo();if(app.store.project.artifacts.some((a)=>a.id===window.auditCheck.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:window.auditCheck.id});return true;
  })()`);
  await waitFor(`window.clonoloom.robustnessView?.state().status==='ready'`);
  await js(`(async()=>{const v=window.clonoloom.robustnessView;if(JSON.stringify(v.state().answer.result)!==window.auditCheck.snapshot)throw new Error('Saved audit changed');await v.choose(${JSON.stringify(comparison)});[...document.querySelectorAll('.definition-robustness button')].find((b)=>b.textContent==='Audit definitions').click();return true})()`);
  await waitFor(`window.clonoloom.robustnessView.state().status==='ready'`);
  await js(`(async()=>{
    const app=window.clonoloom,v=app.robustnessView;if(v.state().artifact!==window.auditCheck.id)throw new Error('Visible/remote settings differ');
    const ensure=app.ensureDataset,create=URL.createObjectURL;let blob;app.ensureDataset=async()=>{throw new Error('Source unavailable')};URL.createObjectURL=(b)=>{blob=b;return create(b)};
    try{await v.show(window.auditCheck.id);await v.download();const b=new Uint8Array(await blob.arrayBuffer());if(b[0]!==80||b[1]!==75)throw new Error('ZIP failed');}finally{app.ensureDataset=ensure;URL.createObjectURL=create;}
    v.select('richness');const {figureBlob}=await import('./ui/figures.js');for(const which of ['effects','observations']){const text=await(await figureBlob(v.plotSpec(which),'svg')).text();if(!text.includes('Contrast:')||!text.includes('simultaneous')||!text.includes('declared order'))throw new Error('Figure lost provenance');}return true;
  })()`);
  await act('show_methods');if(!await js(`(()=>{const s=document.querySelector('.methods-text').textContent;return s.includes('Descriptive sensitivity')&&s.includes('synonymous')&&document.querySelector('.methods-references').textContent.includes('Specification curve')})()`))throw new Error('Audit/example methods absent');await act('close_dialogs');
  await js(`(()=>{const root=document.querySelector('.definition-robustness');root.closest('.workbench-scroll').scrollTop=root.querySelector('.robustness-title').offsetTop-65;return true})()`);
  await clearToasts();await checkAccessibility();
}

async function motifScene() {
  await act('new_project',{name:'Artificial sequence-signature study'});
  await act('open_example',{example:'simulated-motif-study',download:true});
  const query=Array.from({length:4},(_,i)=>`S${String(i+1).padStart(2,'0')}`),background=Array.from({length:8},(_,i)=>`S${String(i+5).padStart(2,'0')}`);
  const out=(await act('screen_motifs',{query,background})).data;
  if(!out.reported||out.family<1000)throw new Error('Motif positive control missing');
  await js(`(async()=>{
    const app=window.clonoloom,v=app.motifsView,doc=v.state().answer.result,r=doc.analysis;
    if(r.query.length!==108||r.background.length!==1000||!r.groups.some((g)=>g.kind==='global'&&g.reported))throw new Error('Wrong source units/global control');
    const group=r.groups.filter((g)=>g.motif==='QGHT'&&g.reported).sort((a,b)=>b.query.length-a.query.length||a.id.localeCompare(b.id))[0];if(!group)throw new Error('Local control missing');await v.select(group.id);
    const p=v.state().detail.profile;if(p.n!==group.query.length||p.counts.some((c)=>c.reduce((a,b)=>a+b,0)!==p.n))throw new Error('Profile denominator wrong');
    if(document.querySelectorAll('.motifs-view [data-position]').length!==20*group.length)throw new Error('Profile geometry wrong');
    const empty=r.groups.find((g)=>!g.query.length);if(!empty)throw new Error('Background-only control absent');await v.select(empty.id);if(v.plotSpec()!==null||!document.querySelector('.motifs-view').textContent.includes('No query members; residue frequencies are unavailable.'))throw new Error('Empty-query frequencies misrepresented');await v.select(group.id);app.store.notify(['jobs']);
    window.motifCheck={id:v.state().answer.artifact.id,group:group.id,snapshot:JSON.stringify(doc)};
    app.store.undo();if(app.store.project.artifacts.some((a)=>a.id===window.motifCheck.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:window.motifCheck.id});return true;
  })()`);
  await waitFor(`window.clonoloom.motifsView?.state().status==='ready'&&window.clonoloom.motifsView.state().detail`);
  await js(`(async()=>{const v=window.clonoloom.motifsView;if(JSON.stringify(v.state().answer.result)!==window.motifCheck.snapshot)throw new Error('Saved motif result changed');v.configure(${JSON.stringify(query)},${JSON.stringify(background)});[...document.querySelectorAll('.motifs-view button')].find((b)=>b.textContent==='Screen motifs').click();return true})()`);
  await waitFor(`window.clonoloom.motifsView.state().status==='ready'&&window.clonoloom.motifsView.state().detail`);
  await js(`(async()=>{const app=window.clonoloom;let v=app.motifsView;if(v.state().answer.artifact.id!==window.motifCheck.id)throw new Error('Visible/remote settings differ');const ensure=app.ensureDataset,create=URL.createObjectURL;let blob;app.ensureDataset=async()=>{throw new Error('Source unavailable')};URL.createObjectURL=(b)=>{blob=b;return create(b)};try{await app.setMode('data');app.focusItem({kind:'result',id:window.motifCheck.id});for(let i=0;i<200;i++){if(app.motifsView?.state().status==='ready'&&app.motifsView.state().detail)break;await new Promise((r)=>setTimeout(r,50));}v=app.motifsView;if(v?.state().status!=='ready'||!v.state().detail)throw new Error('Source-free remount failed');await v.download();const b=new Uint8Array(await blob.arrayBuffer());if(b[0]!==80||b[1]!==75)throw new Error('ZIP unavailable');await v.select(window.motifCheck.group);}finally{app.ensureDataset=ensure;URL.createObjectURL=create;}const {figureBlob}=await import('./ui/figures.js');const text=await(await figureBlob(v.plotSpec(),'svg')).text();for(const phrase of ['frequency','pooled','ClonoLoom adaptation','Query samples:','not proven'])if(!text.includes(phrase))throw new Error('Figure provenance missing '+phrase);return true})()`);
  await act('show_methods');if(!await js(`(()=>{const s=document.querySelector('.methods-text').textContent;return s.includes('Planted local QGHT')&&s.includes('No N/P boost')})()`))throw new Error('Motif methods absent');await act('close_dialogs');
  await js(`(()=>{const root=document.querySelector('.motifs-view');root.closest('.workbench-scroll').scrollTop=root.querySelector('.motif-title').offsetTop-65;return true})()`);
  await clearToasts();await checkAccessibility();
}

async function convergenceScene() {
  await act('new_project', { name: 'Artificial subject recurrence and HLA study' });
  await act('open_example', { example: 'simulated-convergence-study', download: true });
  const folder = join(places.examples, 'simulated-convergence-study');
  const arms = JSON.parse(readFileSync(join(folder, 'truth/arms.json'), 'utf8'));
  const subjects = JSON.parse(readFileSync(join(folder, 'truth/subjects.json'), 'utf8'));
  const parent = (await act('screen_motifs', { ...arms, options: { minK: 4, maxK: 4 } })).data.artifact;
  const groups = await js(`window.clonoloom.motifsView.state().answer.result.analysis.groups.filter((g)=>(g.kind==='local'&&g.motif==='QGHT'&&g.length===17&&g.start===5)||(g.kind==='global'&&g.allowed==='ILMV')).map((g)=>g.id)`);
  if (groups.length !== 2) throw new Error('Planted signatures absent');
  const selection = { species: 'human', samples: [...arms.query, ...arms.background], subjects, groups, alleles: ['HLA-A*02:01', 'HLA-B*07:02', 'HLA-DRB1*15:01'] };
  const out = (await act('analyze_convergence', { motifs: parent, selection })).data;
  if (out.subjects !== 32 || out.family !== 6) throw new Error('Wrong subject/family counts');
  await js(`(async()=>{
    const app=window.clonoloom,v=app.convergenceView,r=v.state().answer.result.analysis;
    const group=r.recurrence.find((g)=>g.label.includes('QGHT')),index=r.associations.findIndex((x)=>x.group===group.id&&x.allele==='HLA-A*02:01'),x=r.associations[index];
    if(group.subjects.length!==18||x.n!==28||x.carriers!==20||x.shared!==17||x.excluded!==4||x.noncarrier.matches!==1||x.noncarrier.total!==8)throw new Error('Wrong subject truth');
    if(r.associations.some((x)=>x.locus!=='A'&&x.n!==32))throw new Error('Wrong locus universe');
    v.select(index);const points=[...document.querySelectorAll('.convergence-view [data-estimate]')];
    if(points.length!==2||points[0].dataset.matches!=='17'||points[0].dataset.total!=='20'||points[1].dataset.matches!=='1'||points[1].dataset.total!=='8')throw new Error('Frequency geometry wrong');
    await v.loadParent(${JSON.stringify(parent)});v.configure(${JSON.stringify(selection)});
    if(document.querySelector('[aria-label="Typing H01"]').value!==${JSON.stringify(Object.values(subjects[0].typing).flat().join(', '))})throw new Error('Raw typing control lost');
    window.convergenceCheck={id:v.state().answer.artifact.id,index,snapshot:JSON.stringify(v.state().answer.result)};
    [...document.querySelectorAll('.convergence-view button')].find((b)=>b.textContent==='Analyze recurrence and HLA').click();return true;
  })()`);
  await waitFor(`window.clonoloom.convergenceView.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,v=app.convergenceView,c=window.convergenceCheck;if(v.state().answer.artifact.id!==c.id)throw new Error('Visible/remote declaration differs');app.store.undo();if(app.store.project.artifacts.some((a)=>a.id===c.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:c.id});return true})()`);
  await waitFor(`window.clonoloom.convergenceView?.state().status==='ready'`);
  await js(`(async()=>{
    const app=window.clonoloom,c=window.convergenceCheck;if(JSON.stringify(app.convergenceView.state().answer.result)!==c.snapshot)throw new Error('Saved result changed');
    const ensure=app.ensureDataset,originalFetch=window.fetch,create=URL.createObjectURL;let blob;
    app.ensureDataset=async()=>{throw new Error('Source unavailable')};window.fetch=(url,...args)=>{if(String(url).includes('api/motifs'))throw new Error('Parent unavailable');return originalFetch(url,...args)};URL.createObjectURL=(b)=>{blob=b;return create(b)};
    try{await app.setMode('data');app.focusItem({kind:'result',id:c.id});for(let i=0;i<200;i++){if(app.convergenceView?.state().status==='ready')break;await new Promise((r)=>setTimeout(r,50));}const v=app.convergenceView;if(v?.state().status!=='ready')throw new Error('Source-free remount failed');await v.download();const bytes=new Uint8Array(await blob.arrayBuffer());if(bytes[0]!==80||bytes[1]!==75)throw new Error('ZIP unavailable');v.select(c.index);const {figureBlob}=await import('./ui/figures.js');for(const which of ['hla','recurrence']){const text=await(await figureBlob(v.plotSpec(which),'svg')).text();for(const phrase of ['subjects','BY','Wilson','Selected samples','restriction','Glanville J','10.1080/01621459.1927.10502953'])if(!text.includes(phrase))throw new Error('Figure provenance missing '+phrase);}}
    finally{app.ensureDataset=ensure;window.fetch=originalFetch;URL.createObjectURL=create;}return true;
  })()`);
  await act('show_methods');
  if (!await js(`(()=>{const s=document.querySelector('.methods-text').textContent;return s.includes('32 fictional subjects')&&s.includes('typed-subject universe')&&document.querySelector('.methods-references').textContent.includes('Glanville')})()`)) throw new Error('Convergence/example methods absent');
  await act('close_dialogs');
  await js(`(()=>{const root=document.querySelector('.convergence-view');const scroller=root.closest('.workbench-scroll');scroller.scrollTop+=root.querySelector('.convergence-title').getBoundingClientRect().top-scroller.getBoundingClientRect().top-16;return true})()`);
  await clearToasts(); await checkAccessibility();
}

async function batchQCScene() {
 await act('new_project',{name:'Artificial sample quality and batch study'});
 await act('open_example',{example:'simulated-batch-qc-study',download:true});
 const samples=JSON.parse(readFileSync(join(places.examples,'simulated-batch-qc-study','truth/samples.json'),'utf8'));
 const declaration={samples,options:{threshold:3.5,minSamples:5}};
 const out=(await act('screen_batch_qc',declaration)).data;
 if(out.samples!==24||out.subjects!==24)throw new Error('Batch subject/sample accounting differs');
 await js(`(()=>{const app=window.clonoloom;app.focusItem({kind:'result',id:${JSON.stringify(out.artifact)}});app.qcView.show('contamination');return true})()`);
 await waitFor(`window.clonoloom.contaminationView?.state().status==='idle'`);
 await js(`(()=>{if(document.querySelectorAll('.contamination-view input[aria-label^="subject "]').length!==26)throw new Error('Switching QC tabs did not load source assignments');window.clonoloom.qcView.show('batch-qc');return true})()`);
 await js(`(()=>{const v=window.clonoloom.batchQCView,r=v.state().answer.result.analysis,groups=r.groups.filter((g)=>g.metric==='productive'),points=[...document.querySelectorAll('.batch-qc-view [data-sample]')],values=groups.flatMap((g)=>g.observations).filter((p)=>p.transformed!=null);if(points.length!==values.length||points.length!==23)throw new Error('Plot omits sample values');for(const node of points){const p=values.find((p)=>r.evidence.samples[p.sample].assignment.name===node.dataset.sample);if(!p||String(p.value)!==node.dataset.value||String(p.transformed)!==node.dataset.transformed)throw new Error('Plot differs from evidence');}if(groups.find((g)=>g.batch==='run-C').scored!==0)throw new Error('Small group scored');if(r.groups.find((g)=>g.batch==='run-D'&&g.metric==='rows').mad!==0)throw new Error('Constant group control differs');window.batchQCCheck={id:v.state().answer.artifact.id,snapshot:JSON.stringify(v.state().answer.result)};v.configure(${JSON.stringify(declaration)});[...document.querySelectorAll('.batch-qc-view button')].find((b)=>b.textContent==='Screen batch QC').click();return true})()`);
 await waitFor(`window.clonoloom.batchQCView?.state().status==='ready'`);
 await js(`(async()=>{const app=window.clonoloom,v=app.batchQCView,c=window.batchQCCheck;if(v.state().answer.artifact.id!==c.id)throw new Error('Visible/remote identity differs');app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===c.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:c.id});return true})()`);
 await waitFor(`window.clonoloom.batchQCView?.state().status==='ready'`);
 await js(`(async()=>{const app=window.clonoloom,c=window.batchQCCheck;if(JSON.stringify(app.batchQCView.state().answer.result)!==c.snapshot)throw new Error('Saved result changed');const ensure=app.ensureDataset,create=URL.createObjectURL;let blob;app.ensureDataset=async()=>{throw new Error('Source unavailable')};URL.createObjectURL=(b)=>{blob=b;return create(b)};try{await app.setMode('data');app.focusItem({kind:'result',id:c.id});for(let i=0;i<200;i++){if(app.batchQCView?.state().status==='ready')break;await new Promise((r)=>setTimeout(r,50))}const v=app.batchQCView;if(v?.state().status!=='ready')throw new Error('Source-free remount failed');await v.download();const bytes=new Uint8Array(await blob.arrayBuffer());if(bytes[0]!==80||bytes[1]!==75)throw new Error('ZIP failed');const {figureBlob}=await import('./ui/figures.js');const text=await(await figureBlob(v.plotSpec(),'svg')).text();for(const phrase of ['no quantitative meaning','Threshold:','QC:','0.6745','Iglewicz','no batch correction'])if(!text.includes(phrase))throw new Error('Figure provenance missing '+phrase);}finally{app.ensureDataset=ensure;URL.createObjectURL=create;}return true})()`);
 await act('show_methods');if(!await js(`(()=>{const text=document.querySelector('.methods-text').textContent;return text.includes('26 samples from 25 fictional subjects')&&text.includes('zero MAD')&&document.querySelector('.methods-references').textContent.includes('Iglewicz')})()`))throw new Error('Methods missing');await act('close_dialogs');
 await js(`(()=>{const root=document.querySelector('.batch-qc-view'),scroller=root.parentElement;scroller.scrollTop+=root.querySelector('.batch-qc-title').getBoundingClientRect().top-scroller.getBoundingClientRect().top-12;return true})()`);
 await clearToasts();await checkAccessibility();
}

async function contaminationScene() {
  await act('new_project',{name:'Artificial nucleotide sharing and carryover study'});
  await act('open_example',{example:'simulated-contamination-study',download:true});
  const samples=JSON.parse(readFileSync(join(places.examples,'simulated-contamination-study','truth/samples.json'),'utf8'));
  await act('set_definition',{definition:{chains:['TRB'],junction:'nt',v:'gene',j:'gene',productiveOnly:true}});
  const pgen=(await act('measure_generation',{models:'clonoloom',organism:'human',genes:false})).data.result;
  const declaration={species:'human',unrelated:true,samples,pgen,options:{minCount:2,highFraction:.01,ratio:20,highPgen:1e-8}};
  const out=(await act('screen_contamination',declaration)).data;
  if(out.pairs!==35)throw new Error('Same-subject pair was not excluded');
  // Switching from another dataset to a saved result must restore its toolbar context.
  await js(`(async()=>{const app=window.clonoloom,store=app.store,id=${JSON.stringify(out.artifact)},original=store.project.datasets[0],ensure=app.ensureDataset;app.ensureDataset=async()=>{throw new Error('Navigation control has no source')};try{store.commit({...store.project,datasets:[...store.project.datasets,{...original,id:'qc-navigation-control',name:'Navigation control'}]},'Navigation control',['project']);app.focusItem({kind:'dataset',id:'qc-navigation-control'});await new Promise((r)=>setTimeout(r,50));if(document.querySelector('.workbench-head select[aria-label="Dataset"]').value!=='qc-navigation-control')throw new Error('Control dataset was not selected');app.focusItem({kind:'result',id});await new Promise((r)=>setTimeout(r,50));if(document.querySelector('.workbench-head select[aria-label="Dataset"]').value!==original.id)throw new Error('Saved result did not restore its dataset');store.undo();}finally{app.ensureDataset=ensure;}return true})()`);
  await waitFor(`window.clonoloom.contaminationView?.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,v=app.contaminationView,r=v.state().answer.result.analysis,index=r.pairs.findIndex((p)=>r.evidence.samples[p.a].sample.name==='D01-1'&&r.evidence.samples[p.b].sample.name==='D02-1');v.select(index);const p=r.pairs[index],rows=r.matches.filter((m)=>m.a===p.a&&m.b===p.b);if(p.shared!==6||rows.filter((m)=>m.pattern==='Asymmetric observed fractions').length!==3)throw new Error('Planted sharing controls differ');const points=[...document.querySelectorAll('.contamination-view [data-receptor]')];if(points.length!==6||points.some((node)=>!rows.some((m)=>m.receptor===node.dataset.receptor&&String(m.fractionA)===node.dataset.fractionA&&String(m.fractionB)===node.dataset.fractionB)))throw new Error('Scatter fractions differ');window.contaminationCheck={id:v.state().answer.artifact.id,index,snapshot:JSON.stringify(v.state().answer.result)};v.configure(${JSON.stringify(declaration)});[...document.querySelectorAll('.contamination-view button')].find((b)=>b.textContent==='Screen contamination evidence').click();return true})()`);
  await waitFor(`window.clonoloom.contaminationView?.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,v=app.contaminationView,c=window.contaminationCheck;if(v.state().answer.artifact.id!==c.id)throw new Error('Visible/remote settings differ');app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===c.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:c.id});return true})()`);
  await waitFor(`window.clonoloom.contaminationView?.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,c=window.contaminationCheck;if(JSON.stringify(app.contaminationView.state().answer.result)!==c.snapshot)throw new Error('Saved evidence changed');const ensure=app.ensureDataset,originalFetch=window.fetch,create=URL.createObjectURL;let blob;app.ensureDataset=async()=>{throw new Error('Source unavailable')};window.fetch=(url,...args)=>{if(String(url).includes('api/pgen'))throw new Error('Pgen parent unavailable');return originalFetch(url,...args)};URL.createObjectURL=(b)=>{blob=b;return create(b)};try{await app.setMode('data');app.focusItem({kind:'result',id:c.id});for(let i=0;i<200;i++){if(app.contaminationView?.state().status==='ready')break;await new Promise((r)=>setTimeout(r,50))}const v=app.contaminationView;if(v?.state().status!=='ready')throw new Error('Source-free remount failed');v.select(c.index);await v.download();const bytes=new Uint8Array(await blob.arrayBuffer());if(bytes[0]!==80||bytes[1]!==75)throw new Error('ZIP failed');const {figureBlob}=await import('./ui/figures.js');const text=await(await figureBlob(v.plotSpec(),'svg')).text();for(const phrase of ['no pseudocount','Thresholds:','QC','bbad230','checksum','unrelated'])if(!text.includes(phrase))throw new Error('Figure provenance missing '+phrase);}finally{app.ensureDataset=ensure;window.fetch=originalFetch;URL.createObjectURL=create;}return true})()`);
  await act('show_methods');if(!await js(`(()=>{const s=document.querySelector('.methods-text').textContent;return s.includes('eight fictional subjects')&&s.includes('No sequence is removed')&&document.querySelector('.methods-references').textContent.includes('bbad230')})()`))throw new Error('Scientific methods absent');await act('close_dialogs');
  await js(`(()=>{const root=document.querySelector('.contamination-view'),scroller=root.parentElement;scroller.scrollTop+=root.querySelector('.contamination-title').getBoundingClientRect().top-scroller.getBoundingClientRect().top-12;return true})()`);
  await clearToasts();await checkAccessibility();
}

// --- Scenes --------------------------------------------------------------------------------------

// Each prescribed cohort step goes through the public action and its normal view.
async function guidedCohortScene(confounded) {
  await act('new_project',{name:confounded?'Confounded artificial cohort':'Complete artificial cohort study'});
  for (const preset of confounded ? ['counts-confounded'] : ['counts-independent','counts-null','counts-paired','counts-unbalanced']) {
    const opened=(await act('open_example',{example:`simulated-${preset}`,download:true})).data;
    await act('show',{view:'cohort',dataset:opened.dataset});
    await waitFor(`Boolean(document.querySelector('.cohort-view .tracking-guide'))`);
    const catalog=await(await fetch(`${URL}api/examples`)).json();
    const example=catalog.examples.find((e)=>e.name===`simulated-${preset}`);
    // Exercise the actual visible design button; later steps use the same action directly.
    const before=await js('(window.clonoloom.store.project.cohortDesigns??[]).length');
    await js(`document.querySelector('.cohort-view .tracking-guide button').click()`);
    await waitFor(`(window.clonoloom.store.project.cohortDesigns??[]).length===${before+1}`);
    for(const step of example.opens[0].steps.slice(1))await act(step.action,{dataset:opened.dataset,...step.args});
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods=await js(`document.querySelector('.methods-text').textContent`);
    if(!methods.includes(confounded?'FAILED CHECK':'Saved Replicated')||!methods.includes('subject'))throw new Error('Cohort methods incomplete');
    await act('close_dialogs');
    if(!confounded)await js(`(()=>{const v=window.clonoloom.cohortCountsView,r=v.state().answer.result.analysis;if(document.querySelectorAll('.cohort-counts [data-sample]').length!==${preset==='counts-paired'?56:32})throw new Error('Count observations do not match study');if(document.querySelectorAll('.cohort-counts [data-clone]').length!==r.tested)throw new Error('MA plot omitted tested clones');return true})()`);
  }
  await js(`(async()=>{const a=window.clonoloom;await a.saveNow();await a.openProject(a.store.project.id);await a.setMode('cohort');const p=a.store.project,failed=p.cohortDesigns.find((d)=>!d.result.ok);if(${confounded}){if(!failed||p.artifacts.length)throw new Error('Confounded study inferred a result');await a.cohortView.choose(failed.dataset);await a.cohortView.configure({spec:failed.result.input.spec});if(a.cohortView.state().result.ok)throw new Error('Confounding lost on reopening');if(!document.querySelector('.cohort-saved').textContent.includes('Group is inseparable from batch'))throw new Error('Saved finding hidden');if([...document.querySelector('[aria-label="Feature design version"]').options].some((o)=>o.value===failed.id))throw new Error('Failed check offered for analysis');}else{const views={'cohort-features':'cohortFeaturesView','cohort-compare':'cohortCompareView','cohort-counts':'cohortCountsView'};for(const r of p.artifacts.filter((r)=>views[r.kind])){await a[views[r.kind]].show(r.id);if(a[views[r.kind]].state().status!=='ready')throw new Error('Saved evidence unavailable');}const last=p.cohortDesigns.at(-1);if(last.result.groups[0].subjects!==12||last.result.groups[1].subjects!==20)throw new Error('Unbalanced membership changed');await a.cohortView.choose(last.dataset);const v=a.cohortCountsView;await v.download();const root=document.querySelector('.cohort-counts');root.closest('.workbench-scroll').scrollTop=root.querySelector('.count-result-title').offsetTop-65;}return true})()`);
  await clearToasts();await checkAccessibility();
}

async function capturedCertificateScene(kind) {
  if (kind === 'batch-qc') await batchQCScene(); else await motifScene();
  const artifact = await js(`window.clonoloom.store.project.artifacts.find((a)=>a.kind===${JSON.stringify(kind)}).id`);
  const record = (await act('inspect_certificate', {artifact})).data.certificate;
  const scope = kind === 'batch-qc' ? 'captured-batch-qc-evidence' : 'captured-motif-evidence';
  await js(`(async()=>{
    const v=window.clonoloom.certificatesView,response=await fetch('api/certificates/${artifact}/download'),blob=await response.blob();
    const integrity=await v.verify(blob,'${record.fingerprint}');if(integrity.verdict!=='integrity-match'||integrity.recomputed)throw new Error('Integrity contract changed');
    const r=await v.verify(blob,'${record.fingerprint}','replay');
    if(r.verdict!=='replay-match'||r.scope!=='${scope}'||!r.recomputed||r.steps.length!==1||!r.steps[0].metadataMatched||r.steps[0].expectedSHA256!==r.steps[0].actualSHA256)throw new Error('Captured replay is not exact');
    if(!r.limits.includes(${JSON.stringify(kind === 'batch-qc' ? 'descriptive' : 'GLIPH2 equivalence')}))throw new Error('Method limit absent');
    if(!document.querySelector('.certificate-verdict').textContent.includes('Exact replay match — captured evidence'))throw new Error('Wrong visible verdict');
    const card=document.querySelector('.certificate-check');card.closest('.workbench-scroll').scrollTop=card.offsetTop-65;return true;
  })()`);
  await clearToasts(); await checkAccessibility();
}

const scenes = {
  async checkpoints() {
    await batchQCScene();
    const artifact = await js('window.batchQCCheck.id');
    await act('set_definition', {definition:{chains:[],junction:'aa',v:'gene',j:'gene'}});
    const before = (await act('create_checkpoint', {name:'Declared batch QC study'})).data;
    const certificate = (await act('inspect_certificate', {artifact})).data.certificate;
    await js(`(()=>{const app=window.clonoloom,p=app.store.project;window.checkpointOriginal=JSON.stringify({datasets:p.datasets,artifacts:p.artifacts,samples:p.samples});app.store.commit({...p,datasets:p.datasets.map((d)=>({...d,definition:{...(d.definition??{}),junction:'nt'},filter:'f-review'})),filters:[...(p.filters??[]),{id:'f-review',name:'Proposed productive-only review',dataset:p.datasets[0].id,version:1,rules:{productiveOnly:true,minCount:3},reason:'Sensitivity review; no results recomputed'}],samples:(p.samples?.length?p.samples:[{dataset:p.datasets[0].id,sample:p.datasets[0].samples[0]}]).map((s,i)=>i===0?{...s,condition:'Review group'}:s)},'Prepare sensitivity review');return true})()`);
    const after = (await act('create_checkpoint', {name:'Proposed sensitivity settings'})).data;
    const diff = (await act('compare_checkpoint', {checkpoint:before.id, against:after.id})).data;
    if (!diff.changes.some((c)=>c.category==='QC filters') || !diff.changes.some((c)=>c.category==='Clonotype definitions') || !diff.changes.some((c)=>c.category==='Sample populations and annotations')) throw new Error('Checkpoint categories missing');
    // Delete only this disposable capture library's result, then recover its exact bytes.
    rmSync(join(places.library,'artifacts',artifact),{recursive:true});
    await act('recover_checkpoint', {checkpoint:before.id});
    await js(`(()=>{const app=window.clonoloom,p=app.store.project;if(JSON.stringify({datasets:p.datasets,artifacts:p.artifacts,samples:p.samples})!==window.checkpointOriginal)throw new Error('Recovery altered analysis');app.store.undo();if(!app.store.project.filters.some((f)=>f.id==='f-review'))throw new Error('Recovery undo failed');app.store.redo();return true})()`);
    await js(`(async()=>{const app=window.clonoloom;await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('reports');return true})()`);
    const restored = (await act('inspect_certificate', {artifact})).data.certificate;
    if (restored.fingerprint !== certificate.fingerprint) throw new Error('Recovery changed saved evidence');
    await js(`(async()=>{const response=await fetch('api/certificates/${artifact}/download'),r=await window.clonoloom.certificatesView.verify(await response.blob(),'${certificate.fingerprint}','replay');if(r.verdict!=='replay-match')throw new Error('Recovered evidence does not replay');return true})()`);
    await js(`(async()=>{
      const app=window.clonoloom,v=app.checkpointsView,originalFetch=window.fetch;let release,entered;
      const gate=new Promise((r)=>{release=r}),started=new Promise((r)=>{entered=r});
      window.fetch=async(...args)=>{const response=await originalFetch(...args);if(String(args[0]).includes('/recover')){entered();await gate;}return response};
      try{const pending=v.recover('${before.id}');await started;app.store.commit({...app.store.project,recoverySentinel:'keep newer edit'},'Concurrent edit');release();let refused=false;try{await pending}catch(e){refused=e.message.includes('analysis changed')}if(!refused||app.store.project.recoverySentinel!=='keep newer edit')throw new Error('Stale recovery replaced a newer edit');}
      finally{release();window.fetch=originalFetch;}
      app.store.undo();await new Promise((r)=>setTimeout(r,0));
      const pending=app.checkpointsView.compare('${before.id}');[...document.querySelectorAll('.checkpoints-view button')].find((b)=>b.textContent==='Cancel checkpoint operation').click();let cancelled=false;try{await pending}catch(e){cancelled=e.name==='AbortError'}if(!cancelled||app.checkpointsView.state().busy||app.checkpointsView.state().difference)throw new Error('Checkpoint cancellation left a result');return true;
    })()`);
    await act('compare_checkpoint', {checkpoint:before.id, against:after.id});
    await js(`(()=>{const root=document.querySelector('.checkpoints-view');root.querySelector('details')?.setAttribute('open','');root.closest('.workbench-scroll').scrollTop=0;return true})()`);
    await clearToasts(); await checkAccessibility();
  },
  async ['certificates-batch-qc']() { await capturedCertificateScene('batch-qc'); },
  async ['certificates-motifs']() { await capturedCertificateScene('motifs'); },
  async certificates() {
    await act('new_project', {name:'Artificial cohort evidence'});
    const opened=(await act('open_example',{example:'simulated-counts-independent',download:true})).data;
    for(const step of ['design','features','compare']) await act('cohort_example_step',{dataset:opened.dataset,preset:'counts-independent',step});
    const state=(await act('get_state')).data;
    const artifact=state.artifacts.find((r)=>r.kind==='cohort-compare').id;
    const record=(await act('inspect_certificate',{artifact})).data.certificate;
    await waitFor(`Boolean(document.querySelector('.certificate-record'))`);
    await js(`[...document.querySelectorAll('button')].find((b)=>b.textContent==='Export integrity certificate').click()`);
    await waitFor(`!window.clonoloom.certificatesView.state().busy`);
    const exported=(await act('export_certificate',{artifact})).data;
    if(exported.fingerprint!==record.fingerprint)throw new Error('Visible and remote certificates differ');
    await js(`(async()=>{const a=window.clonoloom,v=a.certificatesView,r=await fetch('api/certificates/${artifact}/download'),blob=await r.blob();const pass=await v.verify(blob,'${record.fingerprint}');if(pass.verdict!=='integrity-match'||pass.recomputed||!document.querySelector('.certificate-verdict').textContent.includes('not recomputed'))throw new Error('Integrity mislabeled');const wrong=await v.verify(blob,'${'0'.repeat(64)}');if(wrong.verdict!=='differs')throw new Error('External fingerprint ignored');const broken=await v.verify(blob.slice(0,blob.size-15));if(broken.verdict!=='differs')throw new Error('Truncated ZIP accepted');await v.verify(blob);await a.saveNow();await a.openProject(a.store.project.id);await a.setMode('reports');const reopened=await a.certificatesView.inspect('${artifact}');if(reopened.certificate.fingerprint!=='${record.fingerprint}')throw new Error('Certificate changed on project reopening');await a.certificatesView.verify(blob,'${record.fingerprint}');return true})()`);
    await js(`(async()=>{const v=window.clonoloom.certificatesView,r=await fetch('api/certificates/${artifact}/download'),blob=await r.blob();const replay=await v.verify(blob,'${record.fingerprint}','replay');if(replay.verdict!=='replay-match'||!replay.recomputed||replay.steps.length!==2||replay.steps.some((s)=>s.expectedSHA256!==s.actualSHA256||!s.metadataMatched))throw new Error('Replay is not exact');if(!document.querySelector('.certificate-verdict').textContent.includes('Exact replay match — captured evidence'))throw new Error('Replay scope hidden');const pending=v.verify(blob,'','replay');[...document.querySelectorAll('button')].find((b)=>b.textContent==='Cancel verification').click();let cancelled=false;try{await pending}catch(e){cancelled=e.name==='AbortError'}if(!cancelled||v.state().busy||v.state().report)throw new Error('Replay cancellation left a verdict');const method=document.querySelector('[aria-label="Certificate check"]');method.value='integrity';method.dispatchEvent(new Event('change',{bubbles:true}));if(!document.querySelector('.certificate-check').textContent.includes('--integrity'))throw new Error('Mode command stale');method.value='replay';method.dispatchEvent(new Event('change',{bubbles:true}));const transfer=new DataTransfer();transfer.items.add(new File([blob],'cohort.certificate.clz',{type:'application/zip'}));const upload=document.querySelector('[aria-label="Certificate file"]');upload.files=transfer.files;upload.dispatchEvent(new Event('change',{bubbles:true}));for(let i=0;i<200&&v.state().busy;i++)await new Promise((resolve)=>setTimeout(resolve,50));if(v.state().report?.verdict!=='replay-match')throw new Error('Visible upload did not replay');const rawReport=v.state().reportText,check=await fetch('api/certificates/verify?mode=replay&expect=${record.fingerprint}',{method:'POST',body:blob});if(await check.text()!==rawReport)throw new Error('Verification JSON bytes changed in browser');const originalFetch=window.fetch,originalURL=URL.createObjectURL,precise=rawReport.replace('"original":{','"original":{"numericTokenControl":9007199254740993,');let savedBlob;try{window.fetch=async()=>new Response(precise,{status:200});await v.verify(blob,'','replay');URL.createObjectURL=(b)=>{savedBlob=b;return originalURL(b)};[...document.querySelectorAll('button')].find((b)=>b.textContent==='Export verification JSON').click();if(!savedBlob||await savedBlob.text()!==precise)throw new Error('Report download rounded numeric tokens');}finally{window.fetch=originalFetch;URL.createObjectURL=originalURL;}const corrupt=await v.verify(blob.slice(0,blob.size-15),'','replay');if(corrupt.verdict!=='differs'||corrupt.recomputed)throw new Error('Corrupt evidence replayed');await v.verify(blob,'${record.fingerprint}','replay');const card=document.querySelector('.certificate-check');card.closest('.workbench-scroll').scrollTop=card.offsetTop-65;return true})()`);
    await clearToasts(); await checkAccessibility();
  },
  async 'cohort-study'(){await guidedCohortScene(false);},
  async 'cohort-confounded'(){await guidedCohortScene(true);},
 async ['pseudobulk-de'](){
  await act('new_project',{name:'Artificial paired expression'});
  await act('open_example',{example:'simulated-pseudobulk-de-study',download:true});
  const truth=JSON.parse(readFileSync(join(places.examples,'simulated-pseudobulk-de-study','truth/truth.json'),'utf8'));
  if(truth.model!=='clonoloom-artificial-pseudobulk-de-v1')throw new Error('Stale paired expression example');
  await act('set_definition',{definition:{chains:['TRA','TRB'],junction:'aa',v:'gene',j:'gene',productiveOnly:true}});
  const dataset=await js(`window.clonoloom.store.project.datasets[0].id`),names=await js(`window.clonoloom.store.project.datasets[0].samples`),parents=[];
  if(names.length!==28)throw new Error('Expected 28 subject samples');
  for(let i=0;i<names.length;i++){
   const path=join(places.examples,'simulated-pseudobulk-de-study','expression',names[i]);
   await act('inspect_expression',{path,cellsPath:join(path,'cells.tsv'),dataset,sample:i});
   const p=await act('compute_cell_phenotypes',{population:'receptor',annotation:'cell_type',breakpoints:[1]});parents.push(p.data.artifact);
  }
  const counts=(await act('aggregate_pseudobulk',{phenotypes:parents,options:{annotation:'cell_type',minCells:10}})).data;
  const timepoint=await js(`window.clonoloom.cellPseudobulkView.state().answer.result.evidence.input.samples[0].annotation.timepoint`);
  const declaration={pseudobulk:counts.artifact,options:{type:'Artificial T cells',timepoint,reference:'1',comparison:'>1',minCount:10,minSamples:25,confidence:.95}};
  const result=(await act('compare_pseudobulk',declaration)).data;
  if(result.subjects!==28||result.family!==38||result.tested!==37)throw new Error('Paired expression accounting differs');
  await js(`(async()=>{const v=window.clonoloom.cellPseudobulkDEView,r=v.state().answer.result.analysis,points=[...document.querySelectorAll('.pseudobulk-de-view circle[data-gene]')];if(points.length!==37||points.some((p)=>!r.rows.some((g)=>g.feature.id===p.dataset.gene&&g.log2Fold!=null)))throw new Error('Gene effect points differ');if(document.querySelectorAll('.pseudobulk-de-view circle[data-sample]').length!==56||document.querySelectorAll('.pseudobulk-de-view path[data-subject]').length!==28)throw new Error('Wrong biological units');window.pseudobulkDECheck={id:v.state().answer.artifact.id,snapshot:JSON.stringify(v.state().answer.result)};await v.configure(${JSON.stringify(declaration)});[...document.querySelectorAll('.pseudobulk-de-view button')].find((b)=>b.textContent==='Compare pseudobulk expression').click();return true})()`);
  await waitFor(`window.clonoloom.cellPseudobulkDEView.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,v=app.cellPseudobulkDEView,c=window.pseudobulkDECheck;if(v.state().answer.artifact.id!==c.id)throw new Error('Visible/remote identity differs');app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===c.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:c.id});return true})()`);
  await waitFor(`window.clonoloom.cellPseudobulkDEView?.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,c=window.pseudobulkDECheck;if(JSON.stringify(app.cellPseudobulkDEView.state().answer.result)!==c.snapshot)throw new Error('Reopening changed expression');const original=window.fetch,create=URL.createObjectURL;let blob;window.fetch=(url,...args)=>{if(['api/expression/','api/phenotypes/','api/pseudobulk/'].some((p)=>String(url).includes(p))&&!String(url).includes('pseudobulk-de'))throw new Error('Parent unavailable');return original(url,...args)};URL.createObjectURL=(b)=>{blob=b;return create(b)};try{const v=app.cellPseudobulkDEView;await v.show(c.id);await v.download();const bytes=new Uint8Array(await blob.arrayBuffer());if(bytes[0]!==80||bytes[1]!==75)throw new Error('ZIP failed');const {figureBlob}=await import('./ui/figures.js');for(const which of ['ma','observations','effect']){const text=await(await figureBlob(v.plotSpec(which),'svg')).text();for(const phrase of ['subject','conditional','size','gks042','ClonoLoom'])if(!text.includes(phrase))throw new Error('Figure provenance missing '+phrase)}v.select('SIM-G38');if(!document.querySelector('.pseudobulk-de-view').textContent.includes('All-zero subject block'))throw new Error('Failed gene hidden');v.select('SIM-G01');}finally{window.fetch=original;URL.createObjectURL=create}return true})()`);
  await act('show_methods');if(!await js(`document.querySelector('.methods-text').textContent.includes('Gamma(16')&&document.querySelector('.methods-references').textContent.includes('gks042')`))throw new Error('Paired methods absent');await act('close_dialogs');
  await clearToasts();await checkAccessibility();
  await js(`(()=>{const root=document.querySelector('.pseudobulk-de-view'),scroller=root.parentElement;scroller.scrollTop+=root.querySelectorAll('figure')[1].getBoundingClientRect().top-scroller.getBoundingClientRect().top-12;return true})()`);
  await showPlaces();
  const theme=await js(`document.documentElement.dataset.theme`);
  await b.capture(join(IMAGES,`pseudobulk-de-pairs-${theme}.webp`),{format:'webp',quality:86});
  await js(`(()=>{const root=document.querySelector('.pseudobulk-de-view'),scroller=root.parentElement;scroller.scrollTop+=root.querySelector('.pseudobulk-de-title').getBoundingClientRect().top-scroller.getBoundingClientRect().top-12;return true})()`);
 },

 async pseudobulk(){
  await act('open_example',{example:'simulated-pseudobulk-study',download:true});
  const truth=JSON.parse(readFileSync(join(places.examples,'simulated-pseudobulk-study','truth/truth.json'),'utf8'));
  if(truth.model!=='clonoloom-artificial-pseudobulk-v1'||!truth.cloneSampling.includes('1/k'))throw new Error('Stale pseudobulk example');
  const dataset=await js(`window.clonoloom.store.project.datasets[0].id`);
  const names=await js(`window.clonoloom.store.project.datasets[0].samples`);
  const parents=[];
  for(let i=0;i<names.length;i++){
   const path=join(places.examples,'simulated-pseudobulk-study','expression',names[i]);
   await act('inspect_expression',{path,cellsPath:join(path,'cells.tsv'),dataset,sample:i});
   const p=await act('compute_cell_phenotypes',{population:'receptor',annotation:'state',breakpoints:[1,5,10]});parents.push(p.data.artifact);
  }
  const declaration={phenotypes:parents,options:{annotation:'state',minCells:10}};
  const out=(await act('aggregate_pseudobulk',declaration)).data;
  if(out.subjects!==4)throw new Error('Pseudobulk subjects differ');
  await js(`(()=>{const v=window.clonoloom.cellPseudobulkView,r=v.state().answer.result;if(r.evidence.input.samples.length!==8||r.analysis.groups.length!==96||r.analysis.coverage.some((x)=>x.cells!==420||x.included!==417))throw new Error('Pseudobulk accounting differs');const shown=r.analysis.groups.filter((g)=>g.type==='state A'&&g.cells>0),bars=[...document.querySelectorAll('.pseudobulk-view rect[data-cells]')];if(bars.length!==shown.length||bars.some((n)=>!shown.some((g)=>r.evidence.input.samples[g.sample].annotation.name===n.dataset.sample&&String(g.cells)===n.dataset.cells)))throw new Error('Coverage bars differ');window.pseudobulkCheck={id:v.state().answer.artifact.id,snapshot:JSON.stringify(v.state().answer.result)};v.configure(${JSON.stringify(declaration)});[...document.querySelectorAll('.pseudobulk-view button')].find((b)=>b.textContent==='Aggregate pseudobulk counts').click();return true})()`);
  await waitFor(`window.clonoloom.cellPseudobulkView.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,v=app.cellPseudobulkView,c=window.pseudobulkCheck;if(v.state().answer.artifact.id!==c.id)throw new Error('Visible/remote identity differs');app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===c.id))throw new Error('Undo failed');app.store.redo();await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('data');app.focusItem({kind:'result',id:c.id});return true})()`);
  await waitFor(`window.clonoloom.cellPseudobulkView?.state().status==='ready'`);
  await js(`(async()=>{const app=window.clonoloom,c=window.pseudobulkCheck;if(JSON.stringify(app.cellPseudobulkView.state().answer.result)!==c.snapshot)throw new Error('Reopening changed evidence');const original=window.fetch,ensure=app.ensureDataset,create=URL.createObjectURL;let blob;app.ensureDataset=async()=>{throw new Error('Source unavailable')};window.fetch=(url,...args)=>{if(String(url).includes('api/expression')||String(url).includes('api/phenotypes'))throw new Error('Source unavailable');return original(url,...args)};URL.createObjectURL=(b)=>{blob=b;return create(b)};try{await app.setMode('data');app.focusItem({kind:'result',id:c.id});for(let i=0;i<200;i++){if(app.cellPseudobulkView?.state().status==='ready')break;await new Promise((r)=>setTimeout(r,50))}const v=app.cellPseudobulkView;if(v?.state().status!=='ready')throw new Error('Source-free remount failed');await v.download();const bytes=new Uint8Array(await blob.arrayBuffer());if(bytes[0]!==80||bytes[1]!==75)throw new Error('ZIP failed');const {figureBlob}=await import('./ui/figures.js');const text=await(await figureBlob(v.plotSpec(),'svg')).text();for(const phrase of ['raw','sample','not statistical significance','s41467','ClonoLoom'])if(!text.includes(phrase))throw new Error('Figure provenance missing '+phrase);}finally{window.fetch=original;app.ensureDataset=ensure;URL.createObjectURL=create;}return true})()`);
  await act('show_methods');if(!await js(`document.querySelector('.methods-text').textContent.includes('negative-binomial')&&document.querySelector('.methods-references').textContent.includes('s41467')`))throw new Error('Methods absent');await act('close_dialogs');
  await js(`(()=>{const root=document.querySelector('.pseudobulk-view'),scroller=root.parentElement;scroller.scrollTop+=root.querySelector('.pseudobulk-title').getBoundingClientRect().top-scroller.getBoundingClientRect().top-12;return true})()`);
  await clearToasts();await checkAccessibility();
 },

 async ['batch-qc'](){await batchQCScene();},
 async contamination() {await contaminationScene();},
  async convergence() { await convergenceScene(); },
  async motifs() { await motifScene(); },
  async 'definition-robustness'() { await definitionAuditScene(); },
  async 'cohort-counts'() { await countModelScene(false); },
  async 'cohort-counts-paired'() { await countModelScene(true); },
  async 'cohort-compare'() { await cohortComparisonScene(false); },
  async 'cohort-paired'() { await cohortComparisonScene(true); },
  async 'cohort-features'() {
    await act('new_project', { name: 'Artificial subject feature study' });
    await act('open_example', { example: 'simulated-standard', download: true });
    await act('measure_diversity', { replicates: 20 });
    const spec = { unit: 'subject', layout: 'independent', groupField: 'condition', levels: ['control', 'expanded'], exclude: {} };
    for (let i = 1; i <= 6; i++) spec.exclude[`S${String(i).padStart(2, '0')}-2`] = 'Teaching selection: first simulated observation per subject';
    await act('check_cohort_design', { spec, name: 'First simulated observations', save: true });
    const state = (await act('get_state')).data;
    const options = { design: state.cohortDesigns[0].id, diversity: state.artifacts.find((r) => r.kind === 'diversity').id };
    const result = (await act('build_cohort_features', options)).data;
    if (result.subjects !== 6 || result.observations !== 6) throw new Error('Subject features changed the replication unit');
    await js(`(async () => {
      const app = window.clonoloom, id = app.cohortFeaturesView.state().artifact;
      const saved = JSON.stringify(app.cohortFeaturesView.state().answer.result);
      app.store.undo(); if (app.store.project.artifacts.some((r) => r.id === id)) throw new Error('Feature undo failed');
      app.store.redo(); if (!app.store.project.artifacts.some((r) => r.id === id)) throw new Error('Feature redo failed');
      await app.saveNow(); await app.openProject(app.store.project.id); await app.setMode('data');
      app.focusItem({kind:'result',id});
      return { id, saved };
    })()`).then((v) => { globalThis.featureSnapshot = v; });
    await waitFor(`window.clonoloom.cohortFeaturesView?.state().status === 'ready'`);
    if (await js(`JSON.stringify(window.clonoloom.cohortFeaturesView.state().answer.result)`) !== globalThis.featureSnapshot.saved) throw new Error('Feature reopening changed evidence');
    // Current annotation edits must not rewrite the selected saved design.
    await act('annotate_samples', { samples: { 'S01-1': { subject: 'changed-subject' } } });
    const repeated = (await act('build_cohort_features', options)).data;
    if (repeated.artifact !== result.artifact) throw new Error('Current annotations silently changed saved features');
    await act('annotate_samples', { samples: { 'S01-1': { subject: 'S01' } } });
    await js(`(async () => {
      const app = window.clonoloom, original = app.ensureDataset, create = URL.createObjectURL;
      let blob; app.ensureDataset = async () => { throw new Error('Sources unavailable'); };
      URL.createObjectURL = (value) => { blob = value; return create(value); };
      try {
        await app.cohortFeaturesView.show(${JSON.stringify(result.artifact)});
        await app.cohortFeaturesView.download();
        const bytes = new Uint8Array(await blob.arrayBuffer());
        if (bytes[0] !== 80 || bytes[1] !== 75) throw new Error('Feature ZIP download failed');
      } finally { app.ensureDataset = original; URL.createObjectURL = create; }
      const select = document.querySelector('[aria-label="Displayed feature"]');
      select.value = 'hill1'; select.dispatchEvent(new Event('change'));
      if (!document.querySelector('.cohort-features table caption').textContent.includes('Hill diversity q=1')) throw new Error('Feature selector failed');
      return true;
    })()`);
    await js(`(() => {
      const choose = (label, value) => { const input = document.querySelector('[aria-label="' + label + '"]'); input.value = value; input.dispatchEvent(new Event('change')); };
      choose('Feature design version', ${JSON.stringify(options.design)});
      choose('Diversity parent', ${JSON.stringify(options.diversity)});
      [...document.querySelectorAll('.cohort-features button')].find((b) => b.textContent === 'Build subject feature table').click();
      return true;
    })()`);
    await waitFor(`window.clonoloom.cohortFeaturesView.state().status === 'ready'`);
    if (await js(`window.clonoloom.cohortFeaturesView.state().artifact`) !== result.artifact) throw new Error('Visible feature choices differ from remote result');
    await js(`(() => {
      const input = document.querySelector('[aria-label="Find feature observations"]'); input.value = 'S01'; input.dispatchEvent(new Event('change'));
      if (document.querySelectorAll('.cohort-features table')[0].querySelectorAll('tbody tr').length !== 1) throw new Error('Observation filter failed');
      const reset = document.querySelector('[aria-label="Find feature observations"]'); reset.value = ''; reset.dispatchEvent(new Event('change')); return true;
    })()`);
    await act('show_methods');
    if (!await js(`document.querySelector('.methods-text').textContent.includes('Subject feature table')`)) throw new Error('Methods omitted subject features');
    await clearToasts();
    await checkAccessibility();
    await act('close_dialogs');
    await act('show', { view: 'cohort' });
    await js(`(async () => { await window.clonoloom.cohortFeaturesView.show(${JSON.stringify(result.artifact)}); const root = document.querySelector('.cohort-features'); root.closest('.workbench-scroll').scrollTop = root.querySelector('.feature-result-title').offsetTop - 65; return true; })()`);
    await clearToasts();
    await checkAccessibility();
  },
  async 'cohort-design'() {
    await act('new_project', { name: 'Artificial cohort design' });
    await act('show', { view: 'cohort' });
    await clearToasts();
    await checkAccessibility();
    await act('open_example', { example: 'simulated-standard', download: true });
    const spec = { unit: 'subject', layout: 'independent', groupField: 'condition', levels: ['control', 'expanded'], exclude: {} };
    const repeated = (await act('check_cohort_design', { spec })).data;
    if (repeated.ok || repeated.subjects !== 6 || repeated.included !== 12 || !repeated.findings.some((f) => f.code === 'cohort-repeated-samples')) throw new Error('Repeated observations treated as independent subjects');
    for (let i = 1; i <= 6; i++) spec.exclude[`S${String(i).padStart(2, '0')}-2`] = 'Teaching selection: first simulated observation per subject';
    const checked = (await act('check_cohort_design', { spec, name: 'First simulated observations', save: true })).data;
    if (!checked.ok || checked.subjects !== 6 || checked.included !== 6 || checked.excluded !== 6 || checked.groups.some((g) => g.subjects !== 3)) throw new Error('Incorrect subject-level cohort counts');
    await js(`(async () => {
      const app = window.clonoloom, id = app.store.project.cohortDesigns[0].id;
      app.store.undo(); if (app.store.project.cohortDesigns?.length) throw new Error('Design undo failed');
      app.store.redo(); if (app.store.project.cohortDesigns[0].id !== id) throw new Error('Design redo failed');
      await app.saveNow(); await app.openProject(app.store.project.id); await app.setMode('cohort');
      return true;
    })()`);
    await waitFor('window.clonoloom.cohortView && !window.clonoloom.cohortView.state().busy');
    if (!await js(`document.querySelector('.cohort-saved summary')?.textContent.includes('annotations as saved')`)) throw new Error('Saved design did not reopen');
    const times = Object.fromEntries(checked.input.samples.map((s) => [s.name, { timepoint: s.name.endsWith('-1') ? 'day 0' : 'day 7' }]));
    await act('annotate_samples', { samples: times });
    const pairedSpec = { unit: 'subject', layout: 'paired', groupField: 'timepoint', levels: ['day 0', 'day 7'], exclude: {} };
    const paired = (await act('check_cohort_design', { spec: pairedSpec })).data;
    if (!paired.ok || paired.completePairs !== 6 || paired.subjects !== 6) throw new Error('Paired subject counts are wrong');
    const unpaired = (await act('check_cohort_design', { spec: { ...pairedSpec, layout: 'independent' } })).data;
    if (unpaired.ok || !unpaired.findings.some((f) => f.code === 'cohort-shared-subject')) throw new Error('Repeated subjects accepted as independent');
    await act('annotate_samples', { samples: Object.fromEntries(checked.input.samples.map((s) => [s.name, { timepoint: s.timepoint }])) });
    // An unavailable source must not hide the saved snapshot or prevent its export.
    await js(`(async () => {
      const app = window.clonoloom, original = app.ensureDataset;
      app.ensureDataset = async () => ({ status: 'failed', error: 'Source unavailable (validation control)' });
      try { await app.cohortView.choose(app.store.project.datasets[0].id); }
      finally { app.ensureDataset = original; }
      if (!document.querySelector('.cohort-saved summary').textContent.includes('current annotations unavailable')) throw new Error('Unavailable annotations mislabeled');
      const create = URL.createObjectURL; let blob;
      URL.createObjectURL = (value) => { blob = value; return create(value); };
      try { [...document.querySelectorAll('.cohort-saved button')].find((b) => b.textContent === 'Export saved JSON').click(); }
      finally { URL.createObjectURL = create; }
      const exported = JSON.parse(await blob.text());
      if (JSON.stringify(exported) !== JSON.stringify(app.store.project.cohortDesigns[0])) throw new Error('Export changed the saved record');
      await app.cohortView.choose(app.store.project.datasets[0].id);
      return true;
    })()`);
    await act('annotate_samples', { samples: { 'S01-1': { subject: 'changed-subject' } } });
    await waitFor(`document.querySelector('.cohort-saved summary')?.textContent.includes('annotations changed')`);
    await act('annotate_samples', { samples: { 'S01-1': { subject: 'S01' } } });
    await act('check_cohort_design', { spec });
    // Exercise the visible controls, including clearing and restoring an exclusion.
    await js(`(() => { const input = document.querySelector('[aria-label="Exclusion reason for S01-2"]'); input.value = ''; input.dispatchEvent(new Event('change')); [...document.querySelectorAll('button')].find((b) => b.textContent === 'Check design').click(); return true; })()`);
    await waitFor('window.clonoloom.cohortView.state().result !== null');
    if (await js('window.clonoloom.cohortView.state().result.ok')) throw new Error('Visible exclusion edit did not affect design');
    await act('check_cohort_design', { spec });
    await clearToasts();
    await checkAccessibility();
  },
  async 'cell-study'() {
    await act('new_project', { name: 'Artificial single-cell study' });
    await act('open_example', { example: 'simulated-cell-states-clouds', download: true });
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    const example = catalog.examples.find((e) => e.name === 'simulated-cell-states-clouds');
    for (const step of example.opens[0].steps) await act(step.action, step.args);
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.methods-text').textContent`);
    const references = await js(`document.querySelector('.methods-references').textContent`);
    for (const phrase of ['cell-states-clouds', 'negative-binomial', 'PCA / UMAP', 'Clone sizes', 'Clonotype modularity', 'Receptor / expression neighborhoods', 'AnnData export']) {
      if (!methods.includes(phrase)) throw new Error(`Study methods omitted ${phrase}`);
    }
    if (/GRCh38|Ensembl|power-law|sequenced into reads/.test(methods)) throw new Error('Unrelated simulation model in study methods');
    for (const ref of ['1802.03426', 's41587-021-00989-2', 'fileformat-prose']) if (!references.includes(ref)) throw new Error(`Missing study reference ${ref}`);
    await act('close_dialogs');
    await js(`(async () => {
      const app = window.clonoloom;
      await app.saveNow(); await app.openProject(app.store.project.id); await app.setMode('cells');
      const views = { 'expression-map': 'cellAnalysisView', phenotypes: 'cellPhenotypesView', modularity: 'cellModularityView', conga: 'cellCongaView', 'anndata-export': 'cellAnnDataView' };
      for (const r of app.store.project.artifacts) {
        await app[views[r.kind]].show(r.id);
        if (app[views[r.kind]].state().status !== 'ready') throw new Error('Saved result did not reopen: ' + r.kind);
      }
      return true;
    })()`);
    await js(`document.querySelector('#computed-cell-map').scrollIntoView({ block: 'start' })`);
    await checkAccessibility();
  },
  async 'cell-anndata-export'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`window.clonoloom.cellsView.example(true)`);
    await act('compute_expression_map');
    const overlay = await act('compute_cell_phenotypes', {map:'latest', annotation:'state'});
    await js(`(() => {const p=document.querySelector('[aria-label="Export phenotype parent"]');p.value=${JSON.stringify(overlay.data.artifact)};p.dispatchEvent(new Event('change'));[...document.querySelectorAll('button')].find((b)=>b.textContent==='Prepare AnnData export').click();return true;})()`);
    await waitFor(`['ready','failed'].includes(window.clonoloom.cellAnnDataView?.state().status)`,120000);
    const state = await js('window.clonoloom.cellAnnDataView.state()');
    if(state.status!=='ready'||state.answer.result.cells!==360||state.answer.result.embeddings.length!==3)throw new Error(`AnnData export failed: ${JSON.stringify(state)}`);
    const remote = await act('export_anndata');
    if(remote.data.artifact!==state.artifact)throw new Error('Visible and remote export differ');
    await js(`(async()=>{const app=window.clonoloom,id=app.cellAnnDataView.state().artifact;app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===id))throw new Error('Export undo failed');app.store.redo();const response=await fetch('api/anndata-export/'+id+'/download');if(!response.ok||!response.headers.get('Content-Type').includes('zip'))throw new Error('Export download failed');const zip=new Uint8Array(await response.arrayBuffer());if(zip[0]!==80||zip[1]!==75)throw new Error('Invalid download');await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('cells');await app.cellAnnDataView.show(id);if(app.cellsView.state().record)throw new Error('Saved export required expression source');return true;})()`);
    await js(`document.querySelector('#anndata-export').scrollIntoView({block:'start'})`);
  },
  async 'cell-conga'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`window.clonoloom.cellsView.example(true)`);
    await act('compute_expression_map');
    const overlay=await act('compute_cell_phenotypes',{map:'latest',annotation:'state'});
    await js(`(() => {const p=document.querySelector('[aria-label="Comparison phenotype parent"]');p.value=${JSON.stringify(overlay.data.artifact)};p.dispatchEvent(new Event('change'));document.querySelector('[aria-label="Comparison neighborhood sizes"]').value='2,4';document.querySelector('[aria-label="Comparison receptor distance"]').value='cdr3';[...document.querySelectorAll('button')].find((b)=>b.textContent==='Compare receptor and expression neighborhoods').click();return true;})()`);
    await waitFor(`['ready','failed'].includes(window.clonoloom.cellCongaView?.state().status)`,120000);
    const state=await js('window.clonoloom.cellCongaView.state()');
    if(state.status!=='ready'||state.answer.result.analysis.eligible!==358||state.answer.result.analysis.nodes.length!==10||state.answer.result.analysis.tested!==20)throw new Error(`Comparison failed: ${JSON.stringify({status:state.status,error:state.error,result:state.answer?.result.analysis})}`);
    const remote=await act('compare_receptor_expression',{options:{neighbors:[2,4],cdr3Only:true}});
    if(remote.data.artifact!==state.artifact)throw new Error('Visible and remote comparison differ');
    await js(`(()=>{const app=window.clonoloom,id=app.cellCongaView.state().artifact;app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===id))throw new Error('Comparison undo failed');app.store.redo();const r=app.cellCongaView.state().answer.result.analysis,t=r.tests.find((t)=>t.logP!=null);app.cellCongaView.selectClone(r.nodes[t.node].id);const plot=document.querySelector('.conga-plot');if(!plot||Number(plot.querySelector('[data-observed]').dataset.observed)!==t.overlap||Number(plot.querySelector('[data-expected]').dataset.expected)!==t.expected)throw new Error('Plot differs from Go result');const f=document.querySelector('[aria-label="Filter comparison clones"]');f.value=r.nodes[0].id;f.dispatchEvent(new Event('change'));if(app.cellCongaView.state().answer.result.analysis.tested!==20)throw new Error('Filtering changed family');return true;})()`);
    await js(`(()=>{[...document.querySelectorAll('button')].find((b)=>b.textContent==='Open comparison phenotype overlay').click();return true;})()`);
    await waitFor('window.clonoloom.cellPhenotypesView.state().selected.length>1');
    await js(`(async()=>{const app=window.clonoloom,id=app.cellCongaView.state().artifact;await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('cells');await app.cellCongaView.show(id);if(app.cellsView.state().record)throw new Error('Saved comparison required expression source');return true;})()`);
    await js(`document.querySelector('#receptor-expression-neighborhoods').scrollIntoView({block:'start'})`);
  },
  async 'cell-modularity'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`window.clonoloom.cellsView.example(true)`);
    await act('compute_expression_map');
    const overlay = await act('compute_cell_phenotypes', { map:'latest', annotation:'state' });
    await js(`(() => { const c=document.querySelector('[aria-label="Modularity phenotype parent"]'); c.value=${JSON.stringify(overlay.data.artifact)}; c.dispatchEvent(new Event('change')); [...document.querySelectorAll('button')].find((b)=>b.textContent==='Compute clonotype modularity').click(); return true; })()`);
    await waitFor(`['ready','failed'].includes(window.clonoloom.cellModularityView?.state().status)`,120000);
    const state=await js('window.clonoloom.cellModularityView.state()');
    if(state.status!=='ready'||state.answer.result.analysis.eligible!==358||state.answer.result.analysis.tested!==10) throw new Error(`Modularity failed: ${JSON.stringify({status:state.status,error:state.error,eligible:state.answer?.result.analysis.eligible,tested:state.answer?.result.analysis.tested})}`);
    const remote=await act('compute_clonotype_modularity');
    if(remote.data.artifact!==state.artifact) throw new Error('Visible and remote modularity differ');
    await js(`(() => {const app=window.clonoloom,id=app.cellModularityView.state().artifact;app.store.undo();if(app.store.project.artifacts.some((r)=>r.id===id))throw new Error('Modularity undo failed');app.store.redo();const r=app.cellModularityView.state().answer.result.analysis;const c=r.clones.filter((c)=>c.p!=null).sort((a,b)=>b.score-a.score)[0];app.cellModularityView.selectClone(c.id);const plot=document.querySelector('.modularity-plot');if(!plot||Number(plot.querySelector('[data-observed]').dataset.observed)!==c.observed)throw new Error('Observed marker differs from result');const f=document.querySelector('[aria-label="Filter modularity clones"]');f.value=c.id;f.dispatchEvent(new Event('change'));if(app.cellModularityView.state().answer.result.analysis.tested!==10)throw new Error('Filtering changed test family');return true;})()`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b)=>b.textContent==='Open parent phenotype overlay').click();return true;})()`);
    await waitFor('window.clonoloom.cellPhenotypesView.state().selected.length > 1');
    await js(`(async()=>{const app=window.clonoloom,id=app.cellModularityView.state().artifact;await app.saveNow();await app.openProject(app.store.project.id);await app.setMode('cells');await app.cellModularityView.show(id);if(app.cellsView.state().record)throw new Error('Saved modularity required open expression source');const r=app.cellModularityView.state().answer.result.analysis;app.cellModularityView.selectClone(r.clones.filter((c)=>c.p!=null).sort((a,b)=>b.score-a.score)[0].id);return true;})()`);
    await js(`document.querySelector('#clonotype-modularity').scrollIntoView({block:'start'})`);
  },
  async 'cell-phenotypes'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`window.clonoloom.cellsView.example(true)`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    await js(`(() => { const c = document.querySelector('[aria-label="Phenotype annotation"]'); c.value = 'state'; c.dispatchEvent(new Event('change')); [...document.querySelectorAll('button')].find((b) => b.textContent === 'Compute clonotype phenotypes').click(); return true; })()`);
    await waitFor(`['ready','failed'].includes(window.clonoloom.cellPhenotypesView?.state().status)`, 120000);
    const state = await js('window.clonoloom.cellPhenotypesView.state()');
    if (state.status !== 'ready' || state.answer.result.analysis.coverage.mapped !== 358 || state.answer.result.analysis.coverage.matched !== 359 || state.answer.result.analysis.cells.length !== 360 || state.answer.result.analysis.options.annotation !== 'state') throw new Error(`Phenotype result: ${state.error || JSON.stringify(state.answer?.result.analysis.coverage)}`);
    const remote = await act('compute_cell_phenotypes', { annotation: 'state' });
    if (remote.data.artifact !== state.artifact) throw new Error('Visible and remote phenotype controls differ');
    await js(`(() => {
      const app = window.clonoloom, id = app.cellPhenotypesView.state().artifact;
      app.store.undo(); if (app.store.project.artifacts.some((r) => r.id === id)) throw new Error('Phenotype undo failed'); app.store.redo();
      const filter = document.querySelector('[aria-label="Filter phenotype table"]'); filter.value = 'CELL00001-1'; filter.dispatchEvent(new Event('change'));
      [...document.querySelectorAll('button')].find((b) => b.textContent === 'Select matching cells').click(); return true;
    })()`);
    await waitFor('window.clonoloom.cellPhenotypesView.state().selected.length === 1');
    await js(`(() => {
      if (!document.querySelector('#inspector').textContent.includes('CELL00001-1')) throw new Error('Selected cells absent from inspector');
      const plot = document.querySelector('.cell-overlay-plot'); if (!plot.querySelector('[opacity="0.14"]')) throw new Error('Other cells not dimmed');
      [...document.querySelectorAll('button')].find((b) => b.textContent === 'Reset cell selection').click(); return true;
    })()`);
    await waitFor('window.clonoloom.cellPhenotypesView.state().selected.length === 0');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Undo cell selection').click(); return true; })()`);
    await waitFor('window.clonoloom.cellPhenotypesView.state().selected.length === 1');
    await js(`document.querySelector('#clonotype-phenotypes').scrollIntoView({ block: 'start' })`);
    const box = await js(`(() => { const r = document.querySelector('.cell-overlay-plot svg').getBoundingClientRect(); return { x:r.x+35, y:r.y+25, endX:r.right-35, endY:r.bottom-30 }; })()`);
    await b.send('Input.dispatchMouseEvent', { type:'mousePressed', x:box.x, y:box.y, button:'left', clickCount:1 });
    await b.send('Input.dispatchMouseEvent', { type:'mouseMoved', x:box.endX, y:box.endY, button:'left', buttons:1 });
    await b.send('Input.dispatchMouseEvent', { type:'mouseReleased', x:box.endX, y:box.endY, button:'left', clickCount:1 });
    await waitFor('window.clonoloom.cellPhenotypesView.state().selected.length > 1');
    await js(`(() => { const app=window.clonoloom, r=app.cellPhenotypesView.state().answer.result; app.pendingCellClone={dataset:r.dataset,definition:r.analysis.definition,filter:r.analysis.filter,id:r.analysis.cells.find((c)=>c.clone).clone}; app.setMode('clonotypes'); return true; })()`);
    await waitFor('Boolean(window.clonoloom.clonotypesView)');
    await js(`window.clonoloom.clonotypesView.select(window.clonoloom.pendingCellClone.id)`);
    await waitFor(`[...document.querySelectorAll('button')].some((b)=>b.textContent==='Show on a cell map')`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b)=>b.textContent==='Show on a cell map').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellPhenotypesView?.state().status === 'ready' && window.clonoloom.cellPhenotypesView.state().selected.length > 0`);
    await js(`(async () => { const app=window.clonoloom,id=app.cellPhenotypesView.state().artifact; await app.saveNow(); await app.openProject(app.store.project.id); await app.setMode('cells'); await app.cellPhenotypesView.show(id); if(app.cellsView.state().record)throw new Error('Saved overlay required expression input'); return true; })()`);
    // Reopen the source and attach a computed map; its population differs from the imported one.
    await js(`window.clonoloom.cellsView.example(true)`);
    const computed = await act('compute_expression_map');
    const overlay = await act('compute_cell_phenotypes', { map: computed.data.artifact, population:'mapped', annotation:'state' });
    if (overlay.data.coverage.mapped !== 359) throw new Error('Computed overlay reused imported positions');
    await js(`(async () => { const app=window.clonoloom; await app.cellPhenotypesView.show(${JSON.stringify(state.artifact)}); const choice=document.querySelector('[aria-label="Overlay coloring"]');choice.value='annotation:0';choice.dispatchEvent(new Event('change'));const missing=document.querySelector('.cell-overlay-plot circle.cell-map-missing');if(!missing||getComputedStyle(missing).fill!=='rgb(163, 163, 163)')throw new Error('Overlay missing label not gray');choice.value='size';choice.dispatchEvent(new Event('change'));return true; })()`);
    await js(`document.querySelector('#clonotype-phenotypes').scrollIntoView({ block: 'start' })`);
  },
  async 'cell-analysis'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Use cell-state study').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Compute expression map').click(); return true; })()`);
    await waitFor(`['ready','failed'].includes(window.clonoloom.cellAnalysisView?.state().status)`, 120000);
    let computed = await js('window.clonoloom.cellAnalysisView.state()');
    if (computed.status !== 'ready' || computed.answer.result.analysis.cells.length !== 359 || computed.answer.result.analysis.excluded.length !== 1 || !computed.answer.result.analysis.graph.weights.length) throw new Error(`Computed map failed: ${JSON.stringify({status:computed.status,error:computed.error,cells:computed.answer?.result.analysis.cells.length,excluded:computed.answer?.result.analysis.excluded.map((c)=>({barcode:c.barcode,reason:c.reason}))})}`);
    const remote = await act('compute_expression_map');
    if (remote.data.artifact !== computed.artifact || remote.data.cells !== 359) throw new Error('Remote computed map differs from the visible controls');
    await js(`(() => {
      const app = window.clonoloom, id = app.cellAnalysisView.state().artifact;
      if (!app.store.project.artifacts.some((r) => r.id === id)) throw new Error('Computed map not kept');
      app.store.undo();
      if (app.store.project.artifacts.some((r) => r.id === id)) throw new Error('Map undo failed');
      app.store.redo(); return true;
    })()`);
    await js(`(async () => {
      const app = window.clonoloom, id = app.cellAnalysisView.state().artifact;
      await app.saveNow(); await app.openProject(app.store.project.id); await app.setMode('cells');
      await app.cellAnalysisView.show(id);
      if (app.cellsView.state().record !== null) throw new Error('Saved map required opening its input');
      return true;
    })()`);
    await js(`(async () => {
      const choice = document.querySelector('[aria-label="Computed map annotation"]'); choice.value = '0'; choice.dispatchEvent(new Event('change'));
      const plot = document.querySelector('[aria-label="Computed expression map"]');
      if (!plot.textContent.includes('UMAP 1 (computed)') || plot.textContent.includes('not computed')) throw new Error('Computed provenance mislabeled');
      const missing = plot.querySelector('circle.cell-map-missing');
      if (!missing || getComputedStyle(missing).fill !== 'rgb(163, 163, 163)') throw new Error('Computed missing annotation not gray');
      missing.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      if (!document.querySelector('[aria-label="Selected computed cell"]').textContent.includes('CELL')) throw new Error('Computed selection failed');
      [...document.querySelectorAll('button')].find((b) => b.textContent === 'Selected features').click(); await Promise.resolve();
      if (!document.body.textContent.includes('Log variance')) throw new Error('Feature table missing');
      [...document.querySelectorAll('button')].find((b) => b.textContent === 'Mapped cells').click(); await Promise.resolve();
      const link = [...document.querySelectorAll('a')].find((a) => a.textContent === 'Download result and provenance');
      if (!link?.download.endsWith('.json')) throw new Error('Result export missing');
      document.querySelector('#computed-cell-map').scrollIntoView({ block: 'start' }); return true;
    })()`);
    computed = await js('window.clonoloom.cellAnalysisView.state()');
    if (computed.answer.result.analysis.cells[0].barcode !== 'CELL00001-1') throw new Error('Saved map barcode order changed');
  },
  async 'cells-anndata'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Use AnnData example').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    const state = await js('window.clonoloom.cellsView.state()');
    if (state.page.summary.anndata.selection.layer !== 'layers/counts' || state.page.cells[0].geneUMIs !== 5 || state.page.summary.anndata.airr.chains !== 3) throw new Error('AnnData evidence differs');
    await js(`window.clonoloom.cellsView.page('features')`);
    if ((await js('window.clonoloom.cellsView.state().page.variableValues[1][0]')) !== 'Shared') throw new Error('Variable annotations lost');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'AIRR evidence').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.airr?.length === 3`);
    if ((await js('window.clonoloom.cellsView.state().page.airr[1].values[2]')) !== null) throw new Error('Missing AIRR field changed');
    await js(`window.clonoloom.cellsView.page('airr-cells')`);
    if ((await js('window.clonoloom.cellsView.state().page.airrCells[1].chains')) !== 0) throw new Error('Empty AIRR list changed');
    await js(`(async () => {
      const app = window.clonoloom, id = app.store.project.id;
      await app.saveNow(); await app.openProject(id); await app.setMode('cells');
      const record = app.store.project.expression[0];
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      if (app.cellsView.state().record.id !== ${JSON.stringify(state.record.id)}) throw new Error('AnnData identity changed on reopen');
      const before = JSON.stringify(app.store.project.expression);
      await app.cellsView.inspect({ ...record, anndata: { ...record.anndata, layer: 'layers/normalized' } }).catch(() => {});
      if (app.cellsView.state().status !== 'error' || JSON.stringify(app.store.project.expression) !== before) throw new Error('Normalized layer replaced saved count evidence');
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      return true;
    })()`);
    // Submit the visible fields after reopening: saved modality/layer/embedding selections
    // must reach the server, not just the remote action.
    await js(`document.querySelector('.view-body form').requestSubmit()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    await act('inspect_expression', { anndataExample: true, sample: 0 });
    await js(`window.clonoloom.cellsView.page('map')`);
    await js(`(() => { const c = document.querySelector('[aria-label="Map annotation"]'); c.value='0'; c.dispatchEvent(new Event('change')); return true; })()`);
    if (!(await js(`document.querySelector('[aria-label="Imported cell map"]').textContent.includes('UMAP 1 (supplied)')`))) throw new Error('Embedding axes missing');
    await js(`document.querySelector('[aria-label="MEX directory, 10x HDF5, AnnData or MuData file"]').value = ${JSON.stringify(`${SHOWN.examples}/synthetic-anndata/cells.h5ad`)}`);
    await js(`document.querySelector('#imported-cell-map').scrollIntoView({ block: 'start' })`);
  },
  async 'cells-hdf5'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Use HDF5 example').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    const state = await js('window.clonoloom.cellsView.state()');
    if (state.page.summary.sources.length !== 1 || state.page.summary.sources[0].role !== 'matrix-hdf5' || state.page.links.matched !== 2 || state.page.cells[0].geneUMIs !== 5 || state.page.cells[3].geneUMIs !== 0) throw new Error('HDF5 evidence differs');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Features').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.features.length === 3`);
    if ((await js('window.clonoloom.cellsView.state().page.features[2].type')) !== 'Antibody Capture') throw new Error('HDF5 modality missing');
    await js(`(async () => {
      const app = window.clonoloom, id = app.store.project.id;
      await app.saveNow(); await app.openProject(id); await app.setMode('cells');
      const record = app.store.project.expression[0];
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      if (app.cellsView.state().record.id !== ${JSON.stringify(state.record.id)}) throw new Error('HDF5 identity changed on project reopen');
      const before = JSON.stringify(app.store.project.expression);
      await app.cellsView.inspect({ path: ${JSON.stringify(join(ROOT, 'testdata/expression-hdf5/unsupported-filter.h5'))} }).catch(() => {});
      if (app.cellsView.state().status !== 'error' || JSON.stringify(app.store.project.expression) !== before) throw new Error('Unsupported HDF5 replaced saved evidence');
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      return true;
    })()`);
    await js(`(() => {
      const sample = document.querySelector('[aria-label="Receptor sample"]'); sample.value = '';
      document.querySelector('.view-body form').requestSubmit();
      if (!document.querySelector('[role="status"]').textContent.includes('explicitly')) throw new Error('Missing HDF5 sample accepted');
      sample.value = '0'; document.querySelector('.view-body form').requestSubmit(); return true;
    })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    await act('inspect_expression', { hdf5Example: true, sample: 0 });
    await js(`document.querySelector('[aria-label="MEX directory, 10x HDF5, AnnData or MuData file"]').value = ${JSON.stringify(`${SHOWN.examples}/synthetic-expression-hdf5/matrix.h5`)}`);
  },
  async 'cell-states'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Use cell-state study').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    const state = await js('window.clonoloom.cellsView.state()');
    if (state.page.links.matched !== 359 || state.page.summary.annotations.mapped !== 358 || state.page.cells[1].annotation || state.page.cells[2].annotation.position !== null || state.page.cells[3].annotation.values[0] !== '') throw new Error('Known missing evidence differs');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Annotations').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.annotations?.length === 50`);
    for (let i = 0; i < 7; i++) {
      await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Next').click(); return true; })()`);
      await waitFor(`window.clonoloom.cellsView.state().page.offset === ${(i + 1) * 50}`);
    }
    if (!(await js(`document.querySelector('.view-body').textContent.includes('ANNOTATION-ONLY')`))) throw new Error('Unmatched annotations hidden');
    await js(`(async () => {
      const app = window.clonoloom, record = app.cellsView.state().record;
      const before = JSON.stringify(app.store.project.expression);
      await app.cellsView.inspect({ ...record, cellsPath: record.path }).catch(() => {});
      if (app.cellsView.state().status !== 'error' || before !== JSON.stringify(app.store.project.expression)) throw new Error('Invalid annotation file changed project');
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      await app.saveNow(); await app.openProject(app.store.project.id); await app.setMode('cells');
      const saved = app.store.project.expression[0];
      if (!saved.cellsPath || saved.sources.length !== 4) throw new Error('Annotation provenance lost');
      await app.cellsView.inspect({ ...saved, expectedId: saved.id });
      return true;
    })()`);
    // Exercise the visible optional-file form, then the map and keyboard-accessible table.
    await js(`(() => { document.querySelector('.view-body form').requestSubmit(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().status === 'ready'`);
    await act('inspect_expression', { studyExample: true, studyPreset: 'cell-states-clouds', sample: 0 });
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Imported map').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.map?.length === 358`);
    await js(`(() => {
      const choice = document.querySelector('[aria-label="Map annotation"]'); choice.value = '0'; choice.dispatchEvent(new Event('change'));
      const missing = document.querySelector('circle.cell-map-missing');
      if (!missing || getComputedStyle(missing).fill !== 'rgb(163, 163, 163)' || missing !== [...document.querySelectorAll('circle[data-cell]')].at(-1)) throw new Error('Missing annotation must be gray and drawn last');
      missing.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      if (!document.querySelector('[aria-label="Selected cell"]').textContent.includes('CELL00004-1')) throw new Error('Missing cell selection lost original index');
      document.querySelector('circle[data-cell]').dispatchEvent(new MouseEvent('click', { bubbles: true }));
      if (!document.querySelector('[aria-label="Selected cell"]').textContent.includes('Coordinates:')) throw new Error('Map cell inspection failed');
      document.querySelector('table.data tbody button').click();
      if (!document.querySelector('[aria-label="Selected cell"]').textContent.includes('CELL00001-1')) throw new Error('Table inspection failed');
      return true;
    })()`);
    await js(`document.querySelector('[aria-label="MEX directory, 10x HDF5, AnnData or MuData file"]').value = ${JSON.stringify(`${SHOWN.examples}/simulated-cell-states-clouds/expression/S01-1`)}`);
    await js(`document.querySelector('[aria-label="Cell annotations"]').value = ${JSON.stringify(`${SHOWN.examples}/simulated-cell-states-clouds/expression/S01-1/cells.tsv`)}`);
    await js(`document.querySelector('#imported-cell-map').scrollIntoView({ block: 'start' })`);
  },
  async cells() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Use artificial example').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    const state = await js('window.clonoloom.cellsView.state()');
    if (state.page.links.matched !== 2 || state.page.links.receptorOnly !== 1 || state.page.cells[0].geneUMIs !== 5 || state.page.cells[3].geneUMIs !== 0) throw new Error('Expression evidence differs');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Features').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.features.length === 3`);
    if (!(await js(`document.querySelector('.view-body').textContent.includes('Antibody Capture')`))) throw new Error('Feature modality missing');
    await js(`window.clonoloom.setMode('data')`);
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Reopen').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.cellsView.state().record.id')) !== state.record.id) throw new Error('Expression identity changed on reopen');
    await js(`(async () => {
      const app = window.clonoloom;
      const before = JSON.stringify(app.store.project.expression);
      await app.cellsView.inspect({ path: '/does-not-exist' }).catch(() => {});
      if (app.cellsView.state().status !== 'error' || JSON.stringify(app.store.project.expression) !== before) throw new Error('Invalid input changed saved evidence');
      const record = app.store.project.expression[0];
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      return true;
    })()`);
    // Exercise the visible directory form and its explicit sample selector.
    await js(`(() => {
      const sample = document.querySelector('[aria-label="Receptor sample"]');
      sample.value = ''; document.querySelector('.view-body form').requestSubmit();
      if (!document.querySelector('[role="status"]').textContent.includes('explicitly')) throw new Error('Missing sample accepted');
      sample.value = '0'; document.querySelector('.view-body form').requestSubmit(); return true;
    })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    await js(`(async () => {
      const app = window.clonoloom;
      const id = app.store.project.id;
      await app.saveNow();
      await app.openProject(id);
      await app.setMode('cells');
      const record = app.store.project.expression[0];
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      if (app.cellsView.state().page.links.matched !== 2) throw new Error('Saved project expression did not reopen');
      return true;
    })()`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Remove').click(); return true; })()`);
    if ((await js('window.clonoloom.store.project.expression.length')) !== 0) throw new Error('Expression removal failed');
    await js(`(() => { window.clonoloom.store.undo(); return true; })()`);
    await waitFor('window.clonoloom.store.project.expression.length === 1');
    await act('inspect_expression', { example: true, sample: 0 });
    await js(`document.querySelector('[aria-label="MEX directory, 10x HDF5, AnnData or MuData file"]').value = ${JSON.stringify(`${SHOWN.examples}/synthetic-expression`)}`);
  },

  async isotypes() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await js(`(() => { window.clonoloom.bcellsMode.show('isotypes'); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Summarize isotypes').click(); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.isotypesView.state()');
    if (result.page.result.assigned !== 2160 || result.page.result.resolvedGene !== 2160 || result.page.familyCount !== 36 || result.page.count !== 18) throw new Error('Wrong isotype totals');
    if (!result.page.rows.every((r) => r.total === 240 && r.timepoint && r.tissue)) throw new Error('Missing denominator or metadata');
    await js(`window.clonoloom.isotypesView.showPage('pairs')`);
    const pairs = await js('window.clonoloom.isotypesView.state().page.rows');
    if (pairs.length !== 3 || pairs.some((p) => p.families !== 36)) throw new Error('Wrong co-occurrence');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'IGHA1 + IGHG1').click(); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView.state().page?.kind === 'families'`);
    if ((await js('window.clonoloom.isotypesView.state().page.count')) !== 36) throw new Error('Pair filter lost families');
    const family = await js('window.clonoloom.isotypesView.state().page.rows[0].id');
    await js(`window.clonoloom.isotypesView.showPage('usage', {family:${JSON.stringify(family)},level:'gene'})`);
    if (!(await js('window.clonoloom.isotypesView.state().page.rows.every((r) => r.family && r.level === "gene")'))) throw new Error('Missing family usage');
    await js(`window.clonoloom.isotypesView.showPage('observations')`);
    const first = await js('window.clonoloom.isotypesView.state().page.rows[0].id');
    await js(`window.clonoloom.isotypesView.showPage('observations', {offset:50})`);
    if ((await js('window.clonoloom.isotypesView.state().page.rows[0].id')) === first) throw new Error('Observation page did not advance');
    await js(`(() => { const v=window.clonoloom.isotypesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('Raw C call:')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('eLife.16578') || !methods.includes(result.page.result.isotypeOptions.familySha256) || !methods.includes('unordered')) throw new Error('Missing isotype methods');
    await act('close_dialogs');
    await js(`(() => { window.clonoloom.isotypesView.showPage('usage'); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.isotypesView.state().artifact')) !== result.artifact) throw new Error('Saved isotypes did not reopen');
  },
  async lineages() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await act('reconstruct_bcell_germlines', { reference: 'simulated-bcell-immunization' });
    await js(`(() => { window.clonoloom.bcellsMode.show('lineages'); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Build lineage trees').click(); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.lineagesView.state()');
    if (result.page.result.built !== 72 || result.page.rowCount !== 4320 || result.page.rows.length !== 50) throw new Error('Wrong lineage totals');
    await js(`window.clonoloom.lineagesView.openTree(${JSON.stringify(result.page.rows[0].id)})`);
    const tree = await js('window.clonoloom.lineagesView.state().tree');
    if ((await js(`getComputedStyle(document.querySelector('.lineage-plot path')).stroke`)) === 'none') throw new Error('Invisible tree edges');
    if (tree.sites !== 327 || !tree.optimalTrees || !tree.nodes.some((n) => n.kind === 'inferred')) throw new Error('Wrong lineage detail');
    const id = tree.nodes.find((n) => n.kind === 'inferred').id;
    await js(`(() => { document.querySelector('[data-node="${id}"]').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView.state().node?.id === ${JSON.stringify(id)}`);
    if ((await js('window.clonoloom.lineagesView.state().node.possible.length')) !== 327) throw new Error('Missing conditional states');
    if (!(await js(`document.activeElement.hasAttribute('data-lineage-detail')`))) throw new Error('Node details lost keyboard focus');
    await js(`(() => { const s=document.querySelector('[aria-label="Tree annotations"]'); s.value='timepoint'; s.dispatchEvent(new Event('change')); return true; })()`);
    await js(`window.clonoloom.lineagesView.showPage('rows')`);
    const row = await js('window.clonoloom.lineagesView.state().page.rows[0]');
    if (!row.isotype || !row.timepoint || !row.tissue || !row.node) throw new Error('Missing observed annotations');
    await js(`window.clonoloom.lineagesView.showPage('rows', '', '', 50)`);
    if ((await js('window.clonoloom.lineagesView.state().page.rows[0].id')) === row.id) throw new Error('Lineage page did not advance');
    await js(`(() => { const v=window.clonoloom.lineagesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('Lineage source row')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('0128004') || !methods.includes('not bootstrap support') || !methods.includes(result.page.result.lineageOptions.germlineSha256)) throw new Error('Missing lineage methods');
    await act('close_dialogs');
    await js(`(() => { window.clonoloom.lineagesView.openTree(${JSON.stringify(tree.id)}); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.lineagesView.state().artifact')) !== result.artifact) throw new Error('Saved lineage did not reopen');
    await js(`window.clonoloom.lineagesView.openTree(${JSON.stringify(tree.id)})`);
    await js(`(() => { document.querySelector('.lineage-plot').scrollIntoView({block:'center'}); return true; })()`);
  },
  async shm() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await act('reconstruct_bcell_germlines', { reference: 'simulated-bcell-immunization' });
    await js(`(() => { window.clonoloom.bcellsMode.show('shm'); return true; })()`);
    await waitFor(`window.clonoloom.shmView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Measure SHM').click(); return true; })()`);
    await waitFor(`window.clonoloom.shmView.state().status === 'ready'`);
    const result = await js('window.clonoloom.shmView.state()');
    if (result.page.result.evaluated !== 4320 || result.page.summaryCount !== 270 || result.page.rows.length !== 50 || Object.keys(result.page.result.codonExclusions).length) throw new Error('Wrong SHM summary');
    await js(`window.clonoloom.shmView.showPage('rows')`);
    const row = await js('window.clonoloom.shmView.state().page.rows[0]');
    if (row.regions[0].evaluableBases !== 327 || row.regions[0].evaluableCodons !== 109 || row.regions[8].frequency !== null) throw new Error('Wrong SHM denominator');
    await js(`window.clonoloom.shmView.showPage('rows', '', '', 50)`);
    if ((await js('window.clonoloom.shmView.state().page.rows[0].id')) === row.id) throw new Error('SHM page did not advance');
    await js(`(() => { const v=window.clonoloom.shmView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('Endpoint codons')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('single-hit') || !methods.includes('No selection statistic') || !methods.includes(result.page.result.shmOptions.germlineSha256)) throw new Error('Missing SHM methods');
    await act('close_dialogs');
    await js(`(() => { window.clonoloom.shmView.showPage('rows'); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.shmView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.shmView.state().artifact')) !== result.artifact) throw new Error('Saved SHM did not reopen');
    await js(`window.clonoloom.shmView.showPage('rows')`);
  },
  async germlines() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await js(`(() => { window.clonoloom.bcellsMode.show('germlines'); return true; })()`);
    await waitFor(`Boolean(document.querySelector('[aria-label="Reference"]'))`);
    await js(`(() => { const select = document.querySelector('[aria-label="Reference"]'); select.value = 'simulated-bcell-immunization'; select.dispatchEvent(new Event('change')); return true; })()`);
    await waitFor(`window.clonoloom.germlinesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Reconstruct germlines').click(); return true; })()`);
    await waitFor(`window.clonoloom.germlinesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.germlinesView.state()');
    if (result.page.result.reconstructed !== 4320 || result.page.familyCount !== 72 || result.page.rows.length !== 50) throw new Error('Wrong germline reconstruction');
    await js(`window.clonoloom.germlinesView.showPage('rows')`);
    const row = await js('window.clonoloom.germlinesView.state().page.rows[0]');
    if (row.germline.slice(285,318) !== 'N'.repeat(33) || row.queryPositions.length !== 360) throw new Error('Wrong masking or coordinates');
    await js(`(() => { const v=window.clonoloom.germlinesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('not IMGT numbered')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('D-masked germline reconstruction') || !methods.includes('btv359') || !methods.includes(result.page.result.germlinesOptions.files[0].sha256)) throw new Error('Missing germline methods or provenance');
    await act('close_dialogs');
    // A delayed row load cannot repaint a destroyed view.
    await js(`(() => { window.clonoloom.germlinesView.showPage('rows'); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.germlinesView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.germlinesView.state().artifact')) !== result.artifact) throw new Error('Saved germline result did not reopen');
  },
  async families() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families');
    const diagnostic = await js('window.clonoloom.familiesView.state()');
    if (diagnostic.page.familyCount !== 0 || diagnostic.page.representativeCount !== 133 || diagnostic.page.result.eligible !== 2160) throw new Error('Wrong distance diagnostics: ' + JSON.stringify({ counts: [diagnostic.page.familyCount, diagnostic.page.representativeCount, diagnostic.page.result.eligible], result: diagnostic.page.result }));
    // Exercise the visible form, not just the remote handler.
    await js(`(() => {
      const input = document.querySelector('[aria-label="Family threshold"]'); input.value = '0.1';
      [...document.querySelectorAll('button')].find((b) => b.textContent === 'Apply threshold').click(); return true;
    })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Find clonal families').click(); return true; })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.familiesView.state()');
    if (result.page.familyCount !== 36 || result.page.result.eligible !== 2160) throw new Error('Wrong family recovery');
    if (await js(`document.querySelector('.plot').outerHTML.includes('NaN')`)) throw new Error('Invalid histogram');
    await js(`window.clonoloom.familiesView.showPage('members', ${JSON.stringify(result.page.rows[0].id)})`);
    await js(`(() => { const v = window.clonoloom.familiesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('SHA-256')`);
    await js(`window.clonoloom.familiesView.showPage('nearest')`);
    if ((await js('window.clonoloom.familiesView.state().page.rows.length')) !== 50) throw new Error('Unbounded nearest page');
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('1601850') || !methods.includes('Paired maximum distance')) throw new Error('Family methods missing');
    await act('close_dialogs');
    // An older diagnostic is inspected by its identity, independent of current options.
    await js(`(() => { window.clonoloom.focusItem({kind:'result', id:${JSON.stringify(diagnostic.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'ready' && window.clonoloom.familiesView.state().page.familyCount === 0`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result', id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'ready' && window.clonoloom.familiesView.state().page.familyCount === 36`);
  },
  async immunization() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    const example = catalog.examples.find((e) => e.name === 'simulated-bcell-immunization');
    for (const step of example.opens[0].steps) await act(step.action, step.args);
    await act('close_dialogs');
    const saved = await js('window.clonoloom.store.project.artifacts');
    for (const kind of ['families', 'germlines', 'shm', 'lineages', 'isotypes']) {
      if (!saved.some((a) => a.kind === kind)) throw new Error(`Guided study did not save ${kind}`);
    }
    await act('inspect_bcell_inputs', { select: 1 });
    const page = await js('window.clonoloom.bcellsView.page()');
    if (page.count !== 4320 || page.samples.length !== 27 || page.samples.some((s) => s.rows !== s.valid || s.rows !== s.regions)) throw new Error('Artificial B-cell evidence differs');
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.methods-text').textContent`);
    if (!methods.includes('artificial paired B-cell') || methods.includes('GRCh38') || methods.includes('varying between samples beyond sampling')) throw new Error('Wrong simulation methods');
    const references = await js(`document.querySelector('.methods-references').textContent`);
    for (const citation of ['10.4049/jimmunol.1601850', '10.1093/bioinformatics/btv359', '10.1093/sysbio/20.4.406', '10.1137/0128004', '10.1371/journal.pcbi.1009885']) {
      if (!references.includes(citation)) throw new Error(`Guided methods omitted ${citation}`);
    }
    await act('close_dialogs');
    await act('inspect_bcell_inputs', { select: 1 });
    await waitFor(`document.querySelector('.inspector').textContent.includes('clonoloom-artificial-bcell-v2')`);
  },
  async imgt() {
    await act('open_example', { example: 'synthetic-imgt-inputs', download: true });
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    const example = catalog.examples.find((e) => e.name === 'synthetic-imgt-inputs');
    for (const step of example.opens[0].steps) await act(step.action, step.args);
    const page = await js('window.clonoloom.bcellsView.page()');
    if (page.count !== 3 || page.rows.some((r) => r.evidence.paired) || !page.rows[2].evidence.issues.length) throw new Error('IMGT evidence differs');
    await js(`(async () => {
      const app = window.clonoloom;
      const record = app.store.project.datasets[0];
      if (record.sources.length !== 5 || record.sources.some((s) => !s.imgtGroup)) throw new Error('IMGT source provenance missing');
      const response = await fetch('api/datasets/' + record.id, { method: 'DELETE' });
      if (!response.ok) throw new Error('Closing IMGT dataset failed');
      app.datasetStates.delete(record.id);
      const reopened = await app.ensureDataset(record);
      if (reopened.status !== 'open') throw new Error('IMGT project reopen failed');
      return true;
    })()`);
    await act('inspect_bcell_inputs', { select: 3 });
    await waitFor(`document.querySelector('.inspector').textContent.includes('withheld')`);
  },
  async bcells() {
    await act('open_example', { example: 'synthetic-bcell-inputs', download: true });
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    const example = catalog.examples.find((e) => e.name === 'synthetic-bcell-inputs');
    for (const step of example.opens[0].steps) await act(step.action, step.args);
    const page = await js('window.clonoloom.bcellsView.page()');
    if (page.count !== 7 || page.rows[3].evidence.valid || !page.rows[0].evidence.valid) throw new Error('B-cell evidence summary differs');
    // A selection that finishes after reloading must not restore stale inspector evidence.
    await js(`(async () => {
      const app = window.clonoloom, view = app.bcellsView;
      const id = app.store.project.datasets[0].id;
      const oldFetch = window.fetch;
      let release;
      const gate = new Promise((resolve) => { release = resolve; });
      window.fetch = async (...args) => {
        const response = oldFetch(...args);
        if (String(args[0]).includes('bcells/inputs') && JSON.parse(args[1].body).id) await gate;
        return response;
      };
      try {
        const pending = view.inspect(view.page().rows[0].id);
        await view.show(id);
        release();
        if (await pending !== undefined) throw new Error('Stale inspector response applied');
      } finally { release(); window.fetch = oldFetch; }
      const { saveFilter } = await import('./lib/filters.js');
      app.store.commit(saveFilter(app.store.project, { dataset: id, name: 'Only light kappa', rules: { loci: ['IGK'] }, reason: 'Check evidence filtering' }), 'Filter evidence');
      return true;
    })()`);
    await waitFor(`window.clonoloom.bcellsView.page()?.count === 1`);
    await js(`(async () => {
      const app = window.clonoloom;
      const { saveFilter } = await import('./lib/filters.js');
      app.store.commit(saveFilter(app.store.project, { dataset: app.store.project.datasets[0].id, name: 'Only T cells', rules: { loci: ['TRB'] }, reason: 'Check empty B-cell inputs' }), 'Filter evidence');
      return true;
    })()`);
    await waitFor(`window.clonoloom.bcellsView.page()?.count === 0`);
    await js(`(async () => {
      const app = window.clonoloom;
      const { useFilter } = await import('./lib/filters.js');
      app.store.commit(useFilter(app.store.project, app.store.project.datasets[0].id, null), 'Restore inputs');
      return true;
    })()`);
    await waitFor(`window.clonoloom.bcellsView.page()?.count === 7`);
    await act('inspect_bcell_inputs', { select: 4 });
    await waitFor(`document.querySelector('.inspector').textContent.includes('different lengths')`);
  },
  // The start page of a first run.
  async start() {
    await waitFor(`Boolean(document.querySelector('.welcome'))`);
    await checkAccessibility();
  },
  // The Example data dialog of a first run: nothing downloaded yet.
  async examples() {
    await act('show_examples');
    await waitFor(`document.querySelectorAll('.example-card').length >= 6`);
  },
  // The Data view of the PBMC 1k T cells: the dataset, its validation and samples; its files and
  // license in the inspector.
  async data() {
    await act('open_example', { example: '10x-pbmc-1k', opening: 'T cells' });
    await waitFor(`Boolean(document.querySelector('.dataset-card.focused'))`);
  },
  // QC of the PBMC 10k T cells: Cell Ranger's metrics recomputed, the chains of each cell.
  async qc() {
    await act('open_example', { example: '10x-pbmc-10k-v2', opening: 'T cells' });
    await act('show', { view: 'qc' });
    await waitFor(`document.querySelectorAll('.plot-card svg').length > 0`);
  },
  // The clonotypes of the PBMC 10k T cells under the paired αβ definition, the largest in the
  // inspector.
  async clonotypes() {
    await act('open_example', { example: '10x-pbmc-10k-v2', opening: 'T cells' });
    await act('select_clonotype', { rank: 1 });
    await waitFor(`document.querySelectorAll('.ctable-row:not(.loading)').length > 5 && Boolean(document.querySelector('.members'))`);
  },
  // The methods paragraph of the PBMC 1k T cells, with its references.
  async vaccine() {
    // Execute the catalog's actual guided actions, without downloaded reference dependencies.
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    for (const name of ['dash-2017-tcrdist3', 'simulated-vaccine']) {
      const e = catalog.examples.find((e) => e.name === name);
      for (const opening of e.opens) {
        await act('new_project', { name: opening.name });
        await act('open_example', { example: name, opening: opening.title, download: true });
        for (const step of opening.steps ?? []) {
          await act(step.action, step.args ?? {});
          console.log(`Guided step: ${opening.title} / ${step.title}`);
        }
      }
    }
    await waitFor(`window.clonoloom.trackingView?.followed().status === 'ready'`);
    const tracked = await js('window.clonoloom.trackingView?.followed().result');
    if (!tracked?.choice.evidence?.artifact || !tracked.subjects.length) throw new Error('Guided specificity tracking failed');
    await act('match_specificity', {});
    await waitFor(`Boolean(document.querySelector('.specificity-view table tbody button'))`);
  },
  async invariant() {
    await act('open_example', { example: '10x-pbmc-10k-v2', opening: 'T cells' });
    const result = await act('flag_invariant', { species: 'human', population: 'MAIT' });
    if (result.data.invariant.samples[0].matched[0] !== 80) throw new Error('Unexpected MAIT cell count');
    await js('window.clonoloom.store.undo()');
    await waitFor(`window.clonoloom.clonotypeContext?.request.species === '' && window.clonoloom.clonotypeContext?.request.population === ''`);
    await act('flag_invariant', { species: 'human', population: 'MAIT' });
    await js(`document.querySelector('.clonotype-info details').open = true`);
    await waitFor(`document.querySelectorAll('.ctable-row:not(.loading)').length > 5`);
  },
  async methods() {
    await act('open_example', { example: '10x-pbmc-1k', opening: 'T cells' });
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
  },
  // The diversity of the simulated study's twelve samples, each with its 95% interval.
  async diversity() {
    await act('open_example', { example: 'simulated-standard' });
    await act('measure_diversity');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 3`);
  },
  // Its samples compared at equal coverage, with their rarefaction and extrapolation curves.
  async rarefaction() {
    await act('open_example', { example: 'simulated-standard' });
    await act('rarefy');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 3`);
  },
  // Its samples' V genes, and one gene across the samples.
  async genes() {
    await act('open_example', { example: 'simulated-standard' });
    await act('gene_usage');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 2`);
  },
  // Its samples' junction lengths and amino acids.
  async junctions() {
    await act('open_example', { example: 'simulated-standard' });
    await act('gene_usage', { part: 'junctions' });
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 3`);
  },
  // Every pair of its samples by Morisita's index; the pairs of one subject and of two.
  async overlap() {
    await act('open_example', { example: 'simulated-standard' });
    await act('measure_overlap');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 2`);
  },
  // The simulated CAR-T study's subjects: each patient's samples in days from the infusion, its
  // infusion product as its reference sample, and the two samples taken before it (replicates).
  async subjects() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('describe_subjects');
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 1`);
  },
  // Its second patient's clonotypes followed through time: each sample's five largest, the one
  // that rises most after the infusion chosen and drawn with its intervals.
  async tracking() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('track_clonotypes', { by: 'largest', n: 5, subject: 'S02' });
    await js(`(() => {
      const view = window.clonoloom.trackingView;
      const s = view.followed().result.subjects.find((x) => x.id === 'S02');
      const rise = (c) => Math.max(...c.points.slice(2).map((v) => v.frequency ?? 0)) / Math.max(c.points[1].frequency ?? 0, 1e-6);
      view.select([...s.clonotypes].sort((a, b) => rise(b) - rise(a))[0].id);
      return true;
    })()`);
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 2 && Boolean(document.querySelector('.plot-chosen'))`);
  },
  // Its clonotypes tested between the week before the infusion and two weeks after by the
  // beta-binomial test: its variance, fitted on the samples taken twice before the infusion, at the
  // top; below, the second patient's calls, the most significant increase chosen.
  async tests() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('test_expansion', { from: 'day -7', to: 'day 14', subject: 'S02' });
    await js(`(() => {
      const view = window.clonoloom.trackingView;
      const s = view.tested().result.subjects.find((x) => x.subject === 'S02');
      view.showTested('S02', s.clonotypes.find((c) => c.call === 'increase').id);
      return true;
    })()`);
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 1 && Boolean(document.querySelector('.plot-ring'))`);
  },
  // Its clonotypes classed against the week before the infusion: the guided steps folded away, the
  // classes by subject, then the second patient's transient clonotypes rising to day 14.
  async classes() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('class_clonotypes', { base: 'day -7', class: 'transient', subject: 'S02' });
    await js(`(() => { const g = document.querySelector('.tracking-guide'); if (g) g.open = false; return true; })()`);
    await waitFor(`Boolean(document.querySelector('.tracking-class-head'))`);
    await js(`(() => { const el = document.querySelector('.tracking-class-head'); el.closest('.workbench-scroll').scrollTop = el.offsetTop - 160; return true; })()`);
  },
  // The transplant study's samples attributed to the donors: the share derived from each donor.
  async attribution() {
    await act('open_example', { example: 'simulated-transplant', download: true });
    await act('attribute_samples');
    await js(`(() => { const g = document.querySelector('.tracking-guide'); if (g) g.open = false; return true; })()`);
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 2`);
    await js(`(() => { const el = document.querySelectorAll('.tracking-view .plot-card')[1]; el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
  // Dash et al.'s mouse TCRs compared by TCRdist with their V genes' loops (the reference data put
  // in the library, as a download leaves them): each clonotype's distance to its nearest, and the
  // clonotype with the most neighbors with its nearest, loop by loop.
  async tcrdist() {
    cpSync(join(DATA, 'tcrdist3-loops'), join(places.library, 'references', 'tcrdist3-loops'), { recursive: true });
    await act('open_example', { example: 'dash-2017-tcrdist3', download: true });
    await act('find_neighbors', { select: 1 });
    await waitFor(`Boolean(document.querySelector('.specificity-view .nearest table'))`);
    await js(`(() => { const el = document.querySelector('.specificity-view .plot-card'); el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
  // A simulated sample's network, with the complete graph and bounded drawing.
  async database() {
    await act('open_example', { example: 'simulated-vaccine', download: true });
    cpSync(join(DATA, 'tcrdist3-loops'), join(places.library, 'references', 'tcrdist3-loops'), { recursive: true });
    const near = await act('match_specificity', { database: 'simulated-vaccine', metric: 'tcrdist', radius: 24, genes: true, cdr3Only: false });
    if (near.data?.matches <= 62) throw new Error('Near matching failed to find additional evidence');
    const result = await act('match_specificity', { metric: 'exact' });
    if (result.data?.epitopes.length !== 4 || result.data?.matches !== 62) throw new Error('Specificity remote action returned incorrect counts: ' + JSON.stringify(result));
    await act('match_specificity', { follow: 2 });
    await waitFor(`window.clonoloom.trackingView?.followed().status === 'ready'`);
    const tracked = await js(`window.clonoloom.trackingView.followed().result`);
    if (!tracked.choice.evidence?.artifact || !tracked.subjects.length) throw new Error('Specificity tracking lost its provenance or subjects');
    await act('match_specificity', {});
    await waitFor(`Boolean(document.querySelector('.specificity-view table tbody button'))`);
  },
  async networks() {
    await act('open_data', { paths: [join(DATA, 'simulated-vaccine/S01-d14.tsv')], name: 'Simulated vaccination · S01, day 14' });
    const result = await act('find_networks', { metric: 'levenshtein', cutoff: 2, sameV: true, sameJ: true, select: 1 });
    if (result.data?.nodes !== 7658 || result.data?.edges !== 1176) throw new Error('Network remote action returned incorrect counts');
    await waitFor(`Boolean(document.querySelector('.specificity-view canvas'))`);
    // Check bounded cluster navigation and a member's link before capturing.
    await js(`(() => { const input = document.querySelector('input[aria-label="Cluster number"]'); input.value = '201'; input.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(`document.querySelector('select[aria-label="Cluster"]').value === '201'`);
    await js(`document.querySelector('.specificity-view table tbody button').click()`);
    await waitFor(`window.clonoloom.store.ui.mode === 'clonotypes'`);
    await act('find_networks', { select: 1 });
    await waitFor(`Boolean(document.querySelector('.specificity-view canvas'))`);
  },
  // Neighborhoods against a background drawn from OLGA's human TRB model.
  async neighborhoods() {
    cpSync(join(DATA, 'tcrdist3-loops'), join(places.library, 'references', 'tcrdist3-loops'), { recursive: true });
    cpSync(join(DATA, 'olga-models'), join(places.library, 'references', 'olga-models'), { recursive: true });
    await act('open_example', { example: 'simulated-vaccine', download: true });
    await act('find_neighborhoods', {});
    await waitFor(`document.querySelectorAll('.specificity-view table.selectable tbody tr').length > 10`);
    await js(`(() => { const el = document.querySelector('.specificity-view .summary-card'); el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
  // Dash et al.'s mouse TCRs' generation probabilities under OLGA's mouse models (the reference
  // data put in the library, as a download leaves them): their distribution, and the clonotypes,
  // the most abundant first.
  async generation() {
    cpSync(join(DATA, 'olga-models'), join(places.library, 'references', 'olga-models'), { recursive: true });
    await act('open_example', { example: 'dash-2017-tcrdist3', download: true });
    await act('measure_generation', {});
    await waitFor(`document.querySelectorAll('.specificity-view .plot-card svg').length >= 2`);
    await js(`(() => { const el = document.querySelectorAll('.specificity-view .plot-card')[1]; el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
};

const names = wanted.length ? wanted : Object.keys(scenes);
for (const name of names) if (!scenes[name]) throw new Error(`Unknown scene ${name}. Scenes: ${Object.keys(scenes).join(', ')}`);
mkdirSync(IMAGES, { recursive: true });
await new Promise((done, fail) => spawn('go', ['build', '-o', binary, '.'], { cwd: ROOT, stdio: 'inherit' }).on('exit', (code) => (code ? fail(new Error('go build failed (is Go installed?)')) : done())));
try {
  for (const theme of themes) {
    for (const name of names) {
      let clonoloom = null;
      b = null;
      try {
        clonoloom = await startClonoLoom((name === 'examples' || name === 'checkpoints' || name.startsWith('certificates') || name.startsWith('cohort-') || name === 'definition-robustness' || name === 'motifs' || name === 'convergence' || name === 'pseudobulk-de' || name === 'pseudobulk' || name === 'batch-qc' || name === 'contamination') ? { examples: mkdtempSync(join(temp, 'examples-')) } : {});
        // A Chrome that does not start is started again once; a scene that cannot start fails
        // alone, not the scenes after it.
        b = await launch({ width: 1600, height: 1000, scale: 1.25 }).catch(async () => {
          await sleep(2000);
          return launch({ width: 1600, height: 1000, scale: 1.25 });
        });
        await b.theme(theme === 'dark');
        await b.goto(URL, 800);
        await waitFor('Boolean(window.clonoloom?.remote)', 30000);
        // No commit in the status bar: the pictures are of the release, not of a build.
        await js(`(() => { const app = window.clonoloom; app.commit = ''; app.store.notify(['library']); return true; })()`);
        await scenes[name]();
        await clearToasts();
        await sleep(800);
        await showPlaces();
        await b.capture(join(IMAGES, `${name}-${theme}.webp`), { format: 'webp', quality: 86 });
        console.log(`${name} (${theme})`);
      } catch (error) {
        console.error(`${name} (${theme}) failed: ${error.message}`);
        process.exitCode = 1;
      } finally {
        await b?.close();
        await clonoloom?.stop();
      }
    }
  }
} finally {
  rmSync(temp, { recursive: true, force: true });
}

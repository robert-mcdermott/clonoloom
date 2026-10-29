// Pin a freshly generated ClonoLoom artificial study, not a real-data study design.
// node docs/examples/headless-recipe.mjs batch-qc|motifs STUDY_DIR NEW_RECIPE NEW_OUTPUT
// Prints an argument-safe shell command for review; never executes it.
import { createHash } from 'node:crypto';
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
const [kind, folder, target, output, ...extra] = process.argv.slice(2);
if (!['batch-qc', 'motifs'].includes(kind) || !folder || !target || !output || extra.length) throw new Error('Usage: node docs/examples/headless-recipe.mjs batch-qc|motifs STUDY_DIR NEW_RECIPE NEW_OUTPUT');
const source = resolve(folder);
const files = readdirSync(source, {withFileTypes:true}).filter((f) => f.isFile() && (f.name.endsWith('.tsv') || f.name.endsWith('.airr.json'))).map((f) => f.name).sort();
if (!files.length) throw new Error('No artificial AIRR files');
const inputs = files.map((file, i) => {
  const raw = readFileSync(join(source, file));
  return {name:`source${i}`, file, size:raw.length, sha256:createHash('sha256').update(raw).digest('hex')};
});
const samples = Array.from({length:12}, (_, i) => `S${String(i + 1).padStart(2, '0')}`);
const parameters = kind === 'batch-qc'
  ? {samples:JSON.parse(readFileSync(join(source, 'truth/samples.json'), 'utf8')), options:{threshold:3.5, minSamples:5}}
  : {definition:{junction:'aa', v:'gene', j:'gene'}, query:samples.slice(0, 4), background:samples.slice(4), subjects:Object.fromEntries(samples.map((s) => [s, s])), options:{}};
const recipe = {format:'clonoloom-run-recipe', version:1, time:'2026-10-27T00:00:00Z', tasks:[{id:'study', operation:'compute', kind, study:`Artificial ${kind} study`, inputs, parameters}]};
writeFileSync(target, `${JSON.stringify(recipe, null, 2)}\n`, {flag:'wx', mode:0o600});
const quote = (s) => `'${s.replaceAll("'", "'\\''")}'`;
const args = ['clonoloom', 'run', '--recipe', resolve(target), '--out', resolve(output), ...inputs.flatMap((i) => ['--input', `${i.name}=${join(source, i.file)}`])];
process.stdout.write(`${args.map(quote).join(' ')}\n`);

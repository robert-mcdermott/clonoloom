// Writes germline.go: the human TRB V, D and J gene segments the simulator recombines, and how
// often each V and J is used.
//
// - The segments are GRCh38's sequence at Ensembl's annotation of the TRB locus (chr7:142,299,011–
//   142,813,287): each functional gene's (biotype TR_V_gene, TR_D_gene, TR_J_gene) canonical
//   transcript, spliced (cDNA). Ensembl "imposes no restrictions on access to, or use of, the
//   data provided" (https://www.ensembl.org/info/about/legal/disclaimer.html); the genome
//   sequence is the Genome Reference Consortium's. A V keeps its last 60 nucleotides before the
//   conserved cysteine (2nd-CYS 104) and the rest; a J, the whole segment, with its conserved
//   phenylalanine (J-PHE 118, in its F-G-X-G motif).
// - V and J usage are those of the productive TRB chains of the cells of 10x Genomics' 10k PBMC
//   dataset (CC BY 4.0; validation/sources.json, 10x-pbmc-10k-v2), each gene counted once per
//   chain, half a chain for a gene the data never use; and their junctions' lengths, which the
//   simulator's selection gives its productive junctions.
//
//   node validation/fetch.mjs 10x-pbmc-10k-v2
//   node internal/simulate/generate-germline.mjs && gofmt -w internal/simulate/germline.go

import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '../..');
const REST = 'https://rest.ensembl.org';
const REGION = '7:142299011-142813287';

async function rest(path, body) {
  for (let attempt = 1; ; attempt++) {
    const response = await fetch(`${REST}${path}`, body
      ? { method: 'POST', headers: { 'Content-Type': 'application/json', Accept: 'application/json' }, body: JSON.stringify(body) }
      : { headers: { Accept: 'application/json' } });
    if (response.ok) return response.json();
    if (attempt === 4) throw new Error(`${path}: HTTP ${response.status}`);
    await new Promise((resolve) => setTimeout(resolve, 1500 * attempt));
  }
}

const release = (await rest('/info/data')).releases[0];
const assembly = (await rest('/info/assembly/homo_sapiens')).assembly_name;
const genes = (await rest(`/overlap/region/human/${REGION}?feature=gene`))
  .filter((g) => ['TR_V_gene', 'TR_D_gene', 'TR_J_gene'].includes(g.biotype));
const lookups = await rest('/lookup/id', { ids: genes.map((g) => g.id), expand: 1 });
// A gene's name: its symbol, else its lookup's, else its description's ("T cell receptor beta
// diversity 2" is TRBD2, which release 116 gives no symbol).
for (const g of genes) {
  const described = /^T cell receptor beta (variable|diversity|joining) ([0-9-]+)/.exec(g.description ?? '');
  g.external_name ??= lookups[g.id]?.display_name ?? (described ? `TRB${described[1][0].toUpperCase()}${described[2]}` : g.id);
}
genes.sort((a, b) => a.external_name.localeCompare(b.external_name, 'en', { numeric: true }));
const transcripts = genes.map((g) => {
  const record = lookups[g.id];
  const canonical = record.Transcript.find((t) => t.is_canonical) ?? record.Transcript[0];
  return { gene: g, transcript: canonical.id };
});
// The sequence endpoint takes at most 50 ids a request.
const sequences = [];
for (let i = 0; i < transcripts.length; i += 50) sequences.push(...await rest('/sequence/id?type=cdna', { ids: transcripts.slice(i, i + 50).map((t) => t.transcript) }));
const seqOf = new Map(sequences.map((s) => [s.query, s.seq.toUpperCase()]));

const CODONS = {};
{
  const bases = 'TCAG';
  const aas = 'FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG';
  let i = 0;
  for (const a of bases) for (const b of bases) for (const c of bases) CODONS[a + b + c] = aas[i++];
}
const translate = (s) => s.match(/.{3}/g)?.map((c) => CODONS[c] ?? 'X').join('') ?? '';

const V = [];
const D = [];
const J = [];
for (const { gene, transcript } of transcripts) {
  const seq = seqOf.get(transcript);
  const name = gene.external_name;
  if (gene.biotype === 'TR_V_gene') {
    // The conserved cysteine (2nd-CYS 104) is the C of a Y-X-C motif, in any frame, whose codon
    // starts 9 to 24 nucleotides before the V's end (C-A-S-S… follows it). An ORF whose cysteine
    // is mutated (TRBV7-3, TRBV17) has none and is left out, as a gene that cannot make a
    // productive chain.
    let anchor = -1;
    for (let frame = 0; frame < 3; frame++) {
      const protein = translate(seq.slice(frame, frame + 3 * Math.floor((seq.length - frame) / 3)));
      for (const m of protein.matchAll(/Y.C/g)) {
        const at = frame + 3 * (m.index + 2);
        if (seq.length - at >= 9 && seq.length - at <= 24 && at > anchor) anchor = at;
      }
    }
    if (anchor < 0) {
      console.error(`${name}: no conserved cysteine found; left out`);
      continue;
    }
    const from = Math.max(0, anchor - 60);
    V.push({ name, seq: seq.slice(from), anchor: anchor - from, transcript });
  } else if (gene.biotype === 'TR_D_gene') {
    D.push({ name, seq, transcript });
  } else {
    // The frame whose translation holds F-G-X-G (or W-G-X-G); the anchor is that F's codon.
    let anchor = -1;
    for (let frame = 0; frame < 3 && anchor < 0; frame++) {
      const m = /[FW]G.G/.exec(translate(seq.slice(frame)));
      if (m) anchor = frame + 3 * m.index;
    }
    if (anchor < 0) {
      console.error(`${name}: no F-G-X-G motif found; left out`);
      continue;
    }
    J.push({ name, seq, anchor, transcript });
  }
}

// Usage in the productive TRB chains of the 10k PBMC cells.
const dataDir = process.env.CLONOLOOM_DATA || join(root, 'validation/cache');
const folder = join(dataDir, '10x-pbmc-10k-v2');
const file = readdirSync(folder).find((f) => f.endsWith('vdj_t_airr_rearrangement.tsv'));
const lines = readFileSync(join(folder, file), 'utf8').split('\n');
const header = lines[0].split('\t');
const at = (name) => header.indexOf(name);
const counts = { V: new Map(), J: new Map() };
const lengths = new Map();
let chains = 0;
for (const line of lines.slice(1)) {
  const f = line.split('\t');
  if (f.length < header.length || f[at('productive')] !== 'T' || f[at('is_cell')] !== 'T' || !f[at('v_call')].startsWith('TRBV')) continue;
  chains++;
  const aa = f[at('junction_aa')].length;
  lengths.set(aa, (lengths.get(aa) ?? 0) + 1);
  for (const [kind, column] of [['V', 'v_call'], ['J', 'j_call']]) {
    const g = f[at(column)].split(',')[0].split('*')[0];
    counts[kind].set(g, (counts[kind].get(g) ?? 0) + 1);
  }
}
const usage = (kind, list) => {
  const raw = list.map((s) => counts[kind].get(s.name) ?? 0.5);
  const sum = raw.reduce((a, b) => a + b, 0);
  list.forEach((s, i) => { s.usage = raw[i] / sum; });
};
usage('V', V);
usage('J', J);

const q = (s) => JSON.stringify(s);
const num = (x) => Number(x.toPrecision(6));
const out = `// Code generated by internal/simulate/generate-germline.mjs; DO NOT EDIT.

package simulate

// humanTRB: GRCh38's human TRB gene segments at Ensembl release ${release}'s annotation (${assembly}),
// and V and J usage in the ${chains} productive TRB chains of the cells of 10x Genomics' 10k PBMC
// dataset (CC BY 4.0).
var humanTRB = Locus{
	Locus:  "TRB",
	Source: ${q(`GRCh38 (${assembly}) at Ensembl release ${release}'s annotation of the TRB locus; usage from 10x Genomics' 10k PBMC dataset (CC BY 4.0), ${chains} productive TRB chains`)},
	V: []Segment{
${V.map((s) => `\t\t{Name: ${q(s.name)}, Seq: ${q(s.seq)}, Anchor: ${s.anchor}, Usage: ${num(s.usage)}, Transcript: ${q(s.transcript)}},`).join('\n')}
	},
	D: []Segment{
${D.map((s) => `\t\t{Name: ${q(s.name)}, Seq: ${q(s.seq)}, Transcript: ${q(s.transcript)}},`).join('\n')}
	},
	J: []Segment{
${J.map((s) => `\t\t{Name: ${q(s.name)}, Seq: ${q(s.seq)}, Anchor: ${s.anchor}, Usage: ${num(s.usage)}, Transcript: ${q(s.transcript)}},`).join('\n')}
	},
	// Productive junctions by their length in amino acids: chains.
	JunctionLengths: map[int]int{${[...lengths].sort((a, b) => a[0] - b[0]).map(([k, v]) => `${k}: ${v}`).join(', ')}},
}
`;
writeFileSync(join(here, 'germline.go'), out);
console.log(`germline.go: ${V.length} V, ${D.length} D, ${J.length} J (Ensembl ${release}, ${assembly}); usage from ${chains} chains`);
for (const s of V) console.log(`  ${s.name.padEnd(10)} ${translate(s.seq.slice(s.anchor)).padEnd(8)} ${(100 * s.usage).toFixed(2)}%`);
for (const s of J) console.log(`  ${s.name.padEnd(10)} ${translate(s.seq.slice(s.anchor % 3)).padEnd(18)} ${(100 * s.usage).toFixed(2)}%`);
for (const s of D) console.log(`  ${s.name.padEnd(10)} ${s.seq}`);

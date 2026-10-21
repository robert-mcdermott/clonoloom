// A methods paragraph for a dataset (clonoloom-spec/requirements.md, O2), written from what the
// project records of it (its files, format, unit, validation, filter and clonotype definition),
// what the program computed (the rows its filter kept, its clonotypes) and ClonoLoom's version,
// with the references it cites, numbered in the order they are first cited. Only what was done is
// said: nothing is written for a step that did not happen. Deterministic: the same inputs give
// the same text. Adapted from SituCraft 0.9.0 web/lib/methods.js.

import { UNITS, datasetUnit } from './datasets.js';
import { describeRules } from './filters.js';

const count = (n) => Number(n).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;

export function joinList(parts) {
  if (parts.length <= 1) return parts.join('');
  return `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`;
}

// The works a paragraph cites.
export const REFERENCES = {
  airr: 'Vander Heiden JA, Marquez S, Marthandan N, et al. AIRR Community standardized representations for annotated immune repertoires. Front Immunol. 2018;9:2206. doi:10.3389/fimmu.2018.02206',
  miairr: 'Rubelt F, Busse CE, Bukhari SAC, et al. Adaptive Immune Receptor Repertoire Community recommendations for sharing immune-repertoire sequencing data. Nat Immunol. 2017;18:1274–1278. doi:10.1038/ni.3873',
  imgtNames: 'Lefranc MP. Immunoglobulin and T cell receptor genes: IMGT and the birth and rise of immunoinformatics. Front Immunol. 2014;5:22. doi:10.3389/fimmu.2014.00022',
  imgtJunction: 'Lefranc MP, Pommié C, Ruiz M, et al. IMGT unique numbering for immunoglobulin and T cell receptor variable domains and Ig superfamily V-like domains. Dev Comp Immunol. 2003;27:55–77. doi:10.1016/S0145-305X(02)00039-3',
  hgnc: 'Seal RL, Braschi B, Gray K, et al. Genenames.org: the HGNC resources in 2023. Nucleic Acids Res. 2023;51:D1003–D1009. doi:10.1093/nar/gkac888',
};

// The formats' names, as a methods section says them.
const FORMATS = {
  airr: 'AIRR Rearrangement TSV',
  'cellranger-contigs': '10x Genomics Cell Ranger contig annotations',
  'immunoseq-v3': 'Adaptive Biotechnologies immunoSEQ exports (v3 columns)',
  'immunoseq-v2': 'Adaptive Biotechnologies immunoSEQ exports (v2 columns)',
};

const day = (iso) => String(iso ?? '').slice(0, 10);

// The paragraph and its references. inputs: { dataset (the project's record), summary (the
// program's summary of the open dataset, or null), filter (the filter version its analyses use,
// or null) and filterResult ({ rows, kept } the program counted, or null), definition (its
// clonotype definition) and clonotypes ({ id, describe, count, cells } from the program, or
// null), program: { version, commit } }.
export function methodsParagraph({ dataset, summary = null, filter = null, filterResult = null, definition = null, clonotypes = null, program }) {
  const refs = [];
  const cite = (key, text = REFERENCES[key]) => {
    let i = refs.findIndex((r) => r.key === key);
    if (i < 0) {
      refs.push({ key, text });
      i = refs.length - 1;
    }
    return `[${i + 1}]`;
  };
  const sentences = [];
  const sources = dataset.sources ?? [];
  const files = sources.length === 1 ? 'The file' : `The ${count(sources.length)} files`;

  // Where the data come from: public data with their license and citation.
  for (const c of dataset.credits ?? []) {
    const terms = c.licenseClass === 'open' ? `published under ${c.license}` : `used under ${c.license}`;
    sentences.push(`The data are ${c.title}, ${terms} ${cite(`data:${c.example}`, c.citation)}${dataset.opened ? `, downloaded and first read on ${day(dataset.opened)}` : ''}.`);
  }

  // How they were read: the formats by the readers' codes, in the order of the files.
  const codes = [...new Set(sources.map((s) => s.format))];
  const formats = codes.length ? codes.map((f) => FORMATS[f] ?? f) : (dataset.formats ?? []);
  const program_ = `ClonoLoom ${program.version}${program.commit ? ` (commit ${program.commit.slice(0, 7)})` : ''} ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}`;
  const airr = codes.includes('airr') ? `, following the AIRR Community's Rearrangement schema ${cite('airr')}` : '';
  sentences.push(`${files} (${joinList(formats)}) ${sources.length === 1 ? 'was' : 'were'} read with ${program_} by its own readers (reader version ${dataset.readerVersion})${airr}, which recorded each file's SHA-256 checksum as it was read.`);
  if (summary?.samples?.some((s) => s.repertoire)) {
    const matched = summary.samples.filter((s) => s.repertoire).length;
    sentences.push(`AIRR Repertoire metadata ${cite('miairr')} were matched to ${matched === summary.samples.length ? 'every sample' : `${plural(matched, 'sample')} of ${count(summary.samples.length)}`} by repertoire_id.`);
  }

  // What they hold, and in which unit.
  const unit = datasetUnit(dataset);
  const unitText = unit
    ? `counted in ${UNITS[unit] ?? unit}${dataset.unitConfirmed ? ' (the unit confirmed by the user, which the files did not state)' : ''}`
    : 'whose counts the files do not say are of reads or of molecules';
  sentences.push(`The dataset holds ${plural(dataset.samples.length, 'sample')} and ${plural(dataset.rows, dataset.singleCell ? 'contig' : 'rearrangement')}${dataset.singleCell ? ' of single cells' : ''}, ${unitText}.`);

  // Gene names.
  const mapped = (summary?.sources ?? []).reduce((n, s) => n + Object.keys(s.geneNames ?? {}).length, 0);
  sentences.push(`Gene calls were read in IMGT nomenclature ${cite('imgtNames')}, ambiguous calls kept as sets${mapped ? `; ${plural(mapped, 'Adaptive Biotechnologies gene name')} ${mapped === 1 ? 'was' : 'were'} mapped to IMGT's by ClonoLoom's rules and the HGNC list of human TR and IG genes ${cite('hgnc')}, each mapping recorded` : ''}.`);

  // Validation.
  const f = dataset.findings ?? { fatal: 0, warnings: 0, notes: 0 };
  const warnings = (summary?.report?.findings ?? []).filter((x) => x.severity === 'warning' || x.severity === 'fatal')
    .map((x) => `${x.title.charAt(0).toLowerCase()}${x.title.slice(1)} (${plural(x.count, 'row')})`);
  sentences.push(`ClonoLoom's validator found ${plural(f.fatal, 'fatal problem')}, ${plural(f.warnings, 'warning')} and ${plural(f.notes, 'note')}${warnings.length ? `: ${joinList(warnings)}` : ''}.`);

  // The filter its analyses use.
  if (filter) {
    const kept = filterResult ? `, which kept ${count(filterResult.kept)} of ${count(filterResult.rows)} ${dataset.singleCell ? 'contigs' : 'rearrangements'}${filterResult.rows ? ` (${((100 * filterResult.kept) / filterResult.rows).toFixed(1)}%)` : ''}` : '';
    sentences.push(`${dataset.singleCell ? 'Contigs' : 'Rearrangements'} were filtered with "${filter.name}" (version ${filter.version}: ${describeRules(filter.rules)}; made because "${filter.reason.replace(/[.\s]+$/, '')}")${kept}.`);
  }

  // Clonotypes.
  if (definition && clonotypes) {
    sentences.push(`Clonotypes were defined as ${clonotypes.describe} ("${definition.name}", ClonoLoom definition ${clonotypes.id}).`);
    if (/junction/.test(clonotypes.describe)) sentences.push(`A junction is the IMGT junction, from the conserved 2nd-CYS 104 to J-PHE or J-TRP 118 ${cite('imgtJunction')}.`);
    if (clonotypes.count !== undefined) sentences.push(`${clonotypes.cells ? `The ${plural(clonotypes.cells, 'cell')} formed` : 'They formed'} ${plural(clonotypes.count, 'clonotype')}.`);
  }

  return { text: sentences.join(' '), references: refs.map((r, i) => `[${i + 1}] ${r.text}`) };
}

// A methods paragraph for a dataset (clonoloom-spec/requirements.md, O2), written from what the
// project records of it (its files, format, unit, validation, filter and clonotype definition),
// what the program computed (the rows its filter kept, its clonotypes) and ClonoLoom's version,
// with the references it cites, numbered in the order they are first cited. Only what was done is
// said: nothing is written for a step that did not happen. Deterministic: the same inputs give
// the same text. Adapted from SituCraft 0.9.0 web/lib/methods.js.

import { UNITS, datasetUnit } from './datasets.js';
import { describeRules } from './filters.js';
import { choiceWords } from './tracking.js';

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
  ensembl: 'Dyer SC, Austine-Orimoloye O, Azov AG, et al. Ensembl 2025. Nucleic Acids Res. 2025;53:D948–D957. doi:10.1093/nar/gkae1071',
  selection: 'Elhanati Y, Murugan A, Callan CG Jr, Mora T, Walczak AM. Quantifying selection in immune receptor repertoires. Proc Natl Acad Sci USA. 2014;111:9875–9880. doi:10.1073/pnas.1409572111',
  hill: 'Hill MO. Diversity and evenness: a unifying notation and its consequences. Ecology. 1973;54:427–432. doi:10.2307/1934352',
  chao1984: 'Chao A. Nonparametric estimation of the number of classes in a population. Scand J Stat. 1984;11:265–270.',
  chaoLee: 'Chao A, Lee SM. Estimating the number of classes via sample coverage. J Am Stat Assoc. 1992;87:210–217. doi:10.1080/01621459.1992.10475194',
  chaoWangJost: 'Chao A, Wang YT, Jost L. Entropy and the species accumulation curve: a novel entropy estimator via discovery rates of new species. Methods Ecol Evol. 2013;4:1091–1100. doi:10.1111/2041-210X.12108',
  chao2014: 'Chao A, Gotelli NJ, Hsieh TC, Sander EL, Ma KH, Colwell RK, Ellison AM. Rarefaction and extrapolation with Hill numbers: a framework for sampling and estimation in species diversity studies. Ecol Monogr. 2014;84:45–67. doi:10.1890/13-0133.1',
  jaccard: 'Jaccard P. The distribution of the flora in the alpine zone. New Phytol. 1912;11:37–50. doi:10.1111/j.1469-8137.1912.tb05611.x',
  dice: 'Dice LR. Measures of the amount of ecologic association between species. Ecology. 1945;26:297–302. doi:10.2307/1932409',
  morisita: 'Morisita M. Measuring of interspecific association and similarity between communities. Mem Fac Sci Kyushu Univ Ser E Biol. 1959;3:65–80',
  horn: 'Horn HS. Measurement of "overlap" in comparative ecological studies. Am Nat. 1966;100:419–424. doi:10.1086/282436',
  horvitzThompson: 'Horvitz DG, Thompson DJ. A generalization of sampling without replacement from a finite universe. J Am Stat Assoc. 1952;47:663–685. doi:10.1080/01621459.1952.10483446',
  chaoJost: 'Chao A, Jost L. Coverage-based rarefaction and extrapolation: standardizing samples by completeness rather than size. Ecology. 2012;93:2533–2547. doi:10.1890/11-1952.1',
  rytlewski: 'Rytlewski J, Deng S, Xie T, et al. Model to improve specificity for identification of clinically-relevant expanded T cells in peripheral blood. PLoS One. 2019;14:e0213684. doi:10.1371/journal.pone.0213684',
  wilson: 'Wilson EB. Probable inference, the law of succession, and statistical inference. J Am Stat Assoc. 1927;22:209–212. doi:10.1080/01621459.1927.10502953',
};

const coverageWords = (v) => (v > 0.99 && v < 1 ? v.toFixed(4) : v.toFixed(3));

const UNIT_WORDS = { cells: 'cells', templates: 'templates', reads: 'reads', umis: 'UMIs', sequences: 'sequences' };

// The formats' names, as a methods section says them.
const FORMATS = {
  airr: 'AIRR Rearrangement TSV',
  'cellranger-contigs': '10x Genomics Cell Ranger contig annotations',
  'immunoseq-v3': 'Adaptive Biotechnologies immunoSEQ exports (v3 columns)',
  'immunoseq-v2': 'Adaptive Biotechnologies immunoSEQ exports (v2 columns)',
  table: 'a table with mapped columns',
};

// The fields a mapping names, as a methods section says them (internal/reads/mapped.go).
const MAPPED_FIELDS = {
  junction_aa: 'the junction\'s amino acids', junction: 'the junction\'s nucleotides', v_call: 'the V gene', d_call: 'the D gene', j_call: 'the J gene', c_call: 'the C gene',
  count: 'the count', sample: 'the sample', locus: 'the locus', productive: 'productivity', cell_id: 'the cell', clone_id: 'the clone', sequence_id: 'the sequence\'s ID',
};
const order = (field) => {
  const i = Object.keys(MAPPED_FIELDS).indexOf(field);
  return i < 0 ? Infinity : i;
};

const day = (iso) => String(iso ?? '').slice(0, 10);

// The paragraph and its references. inputs: { dataset (the project's record), summary (the
// program's summary of the open dataset, or null), filter (the filter version its analyses use,
// or null) and filterResult ({ rows, kept } the program counted, or null), definition (its
// clonotype definition) and clonotypes ({ id, describe, count, cells } from the program, or
// null), results (the project's records of the results kept of it: lib/results.js), program:
// { version, commit } }.
export function methodsParagraph({ dataset, summary = null, filter = null, filterResult = null, definition = null, clonotypes = null, results = [], program }) {
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

  // Simulated data (clonoloom simulate): how they were made.
  const simulated = (summary?.samples ?? []).map((s) => s.repertoire).find((r) => r?.studyId?.startsWith('clonoloom-simulated-'));
  if (simulated) {
    sentences.push(`The data are simulated (${simulated.studyTitle || simulated.studyId}): T-cell receptor β rearrangements made by ClonoLoom's simulator ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)} from GRCh38's gene segments at Ensembl's annotation ${cite('ensembl')}, with the V and J usage of 10x Genomics' 10k PBMC data, deletions and non-templated insertions at the junctions, and selection on junction length to that data's ${cite('selection')}; each subject's clones of power-law sizes, each sample templates drawn from them and sequenced into reads. Their truth is recorded beside them.`);
    // A study over time: its samples' time points and reference samples say so.
    const repertoires = (summary?.samples ?? []).map((x) => x.repertoire).filter(Boolean);
    const event = repertoires.map((r) => /\(([^()]+)\)$/.exec(r.timePoint ?? '')?.[1]).find(Boolean);
    if (event) {
      const tissues = repertoires.map((r) => r.tissue ?? '');
      const reference = tissues.includes('infusion product')
        ? ' An infusion product, made from each subject\'s own T cells and expanded ex vivo, was mixed into its blood in known shares after the infusion, and sampled as its reference sample.'
        : tissues.includes('donor blood')
          ? ' A donor\'s repertoire was mixed into each recipient\'s blood in known shares after the transplant, and sampled as its reference sample.'
          : '';
      sentences.push(`Each subject was sampled at several time points, in days from the ${event}, its clones' frequencies following known trajectories and varying between samples beyond sampling, as between blood samples ${cite('rytlewski')}.${reference}`);
    }
  }

  // How they were read: the formats by the readers' codes, in the order of the files.
  const codes = [...new Set(sources.map((s) => s.format))];
  const formats = codes.length ? codes.map((f) => FORMATS[f] ?? f) : (dataset.formats ?? []);
  const program_ = `ClonoLoom ${program.version}${program.commit ? ` (commit ${program.commit.slice(0, 7)})` : ''} ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}`;
  sentences.push(`${files} (${joinList(formats)}) ${sources.length === 1 ? 'was' : 'were'} read with ${program_} by its own readers (reader version ${dataset.readerVersion}), which recorded ${sources.length === 1 ? 'its' : 'each file\'s'} SHA-256 checksum as ${sources.length === 1 ? 'it' : 'each'} was read.`);
  if (codes.includes('airr')) sentences.push(`AIRR files were read as the AIRR Community's Rearrangement schema defines them ${cite('airr')}.`);
  // Tables of other tools, read through the columns the user mapped.
  for (const s of sources.filter((x) => x.mapping)) {
    const m = s.mapping;
    const fields = Object.keys(m.columns ?? {}).sort((a, b) => order(a) - order(b));
    const columns = fields.map((field) => `${MAPPED_FIELDS[field] ?? field} from "${m.columns[field]}"`);
    const counts = m.columns?.count ? `, the count being of ${UNIT_WORDS[m.unit] ?? m.unit}` : ', each row one sequence (no count was mapped)';
    const inferred = m.columns?.productive ? '' : '; a rearrangement was taken as productive when its junction is whole codons with no stop codon';
    sentences.push(`${s.name}'s columns were mapped by the user: ${joinList(columns)}${counts}${m.sample ? `, every row of sample ${m.sample}` : ''}${inferred}.`);
  }
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

  // Diversity, as each result kept says it was measured (the latest of each definition and filter).
  const diversities = results.filter((r) => r.kind === 'diversity');
  for (const r of diversities) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    sentences.push(`Each sample's diversity was measured over its clonotypes' abundances in ${unit} (${under}) by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: the Hill numbers of order 0 to 4 ${cite('hill')}, Shannon's entropy (natural log), Simpson's index, Pielou's evenness, clonality (1 − evenness) and Simpson clonality (√Σp²), Gini's coefficient, the largest clonotypes' shares and clonal space homeostasis; the richness of the repertoire each was drawn from by Chao1 (bias-corrected) ${cite('chao1984')} ${cite('chao2014')} and ACE ${cite('chaoLee')}, its Shannon diversity by Chao, Wang and Jost's estimator ${cite('chaoWangJost')} and its Simpson diversity by n(n−1)/ΣX(X−1) ${cite('chao2014')}; and the sample coverage ${cite('chaoJost')}. Intervals are 95%: Chao1's from its variance, log-transformed; the others' from ${opts.replicates ?? 200} bootstrap replicates of each sample drawn from its estimated repertoire ${cite('chao2014')} (seed ${opts.seed ?? '—'}), the estimate ± 1.96 standard errors.`);
  }

  // Rarefaction and extrapolation, likewise, with the levels the samples were compared at.
  for (const r of results.filter((x) => x.kind === 'rarefaction')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const l = r.levels;
    const levels = l ? ` The samples were compared at a sample coverage of ${coverageWords(l.coverage)}, the lowest among them, and at ${Math.round(l.size).toLocaleString('en-US')} ${unit}, the smallest sample's size, each sample rarefied to them.` : '';
    sentences.push(`Each sample's clonotypes, Shannon diversity and Simpson diversity (the Hill numbers of order 0, 1 and 2 ${cite('hill')}) were rarefied and extrapolated, over its clonotypes' abundances in ${unit} (${under}), by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: at sample sizes up to twice its own ${cite('chao2014')}, and at equal sample coverage ${cite('chaoJost')}, with 95% intervals from ${opts.replicates ?? 200} bootstrap replicates of each sample drawn from its estimated repertoire (seed ${opts.seed ?? '—'}), the estimate ± 1.96 standard errors.${levels}`);
  }

  // Gene usage, junction lengths and composition, likewise.
  for (const r of results.filter((x) => x.kind === 'usage')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const groups = r.groups ? ` Each condition was summarized by the mean of its samples' shares, each sample counted once.` : '';
    sentences.push(`What each sample's clonotypes (${under}) are made of was counted by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: their V, D, J and C genes and V–J pairs, at the gene and the IMGT subgroup level ${cite('imgtNames')}, a call tied between several genes sharing its part equally among them; their junctions' lengths in amino acids; and their junctions' amino acids, overall and at each IMGT position ${cite('imgtJunction')}. Each is a share of the sample's clonotypes, each counted once, and of its abundance in ${unit}, with a 95% interval from ${opts.replicates ?? 200} bootstrap replicates of the sample's clonotypes (seed ${opts.seed ?? '—'}), the estimate ± 1.96 standard errors.${groups}`);
  }

  // Overlap, likewise.
  for (const r of results.filter((x) => x.kind === 'overlap')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const opts = r.options ?? {};
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const depth = r.depth > 0 ? `, as sampled and with both samples rarefied to ${Math.round(r.depth).toLocaleString('en-US')} ${unit}, the smallest sample's size (the exact expectation)` : '';
    const by = r.publicBy === 'subject' ? 'subjects' : 'samples';
    sentences.push(`The overlap of every pair of samples (clonotypes ${under}) was measured by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: the clonotypes they share, Jaccard's index ${cite('jaccard')}, Sørensen–Dice's ${cite('dice')} and the overlap coefficient${depth}; and over their abundances in ${unit}, Morisita's index ${cite('morisita')} and the Morisita–Horn index ${cite('horn')}. Intervals are 95%: from ${opts.replicates ?? 200} bootstrap replicates of each sample's cells (seed ${opts.seed ?? '—'}), widened for the chance that decides which clonotypes two samples share ${cite('horvitzThompson')}, on the logarithmic scale. Clonotypes found in more than one of the ${by} were counted as public.`);
  }

  // Clonotypes followed over time, likewise.
  for (const r of results.filter((x) => x.kind === 'tracking')) {
    const unit = UNIT_WORDS[r.unit] ?? 'counts';
    const under = `${r.definition ? `"${r.definition}"` : 'its clonotype definition'}${r.filter ? `, filter ${r.filter}` : ''}`;
    const choice = r.choice ?? { by: 'reference', n: 10, list: [] };
    const how = r.intervals
      ? `each one's frequency at each time point is its count of the ${unit} there, the samples taken at one time pooled, with Wilson's ${Math.round(100 * (r.options?.confidence ?? 0.95))}% score interval ${cite('wilson')}, which accounts for sampling alone; a clonotype not found at a time point is reported as not found, the interval's upper end being what that depth could have missed`
      : `each one's frequency at each time point is its share of the ${unit} there, the samples taken at one time pooled, without an interval (${r.unit === 'reads' ? 'reads count each molecule many times over' : 'the abundances are not whole counts'})`;
    sentences.push(`Clonotypes (${under}) were followed through each subject's samples in time order by ClonoLoom ${cite('clonoloom', `McDermott R. ClonoLoom, version ${program.version}. https://github.com/robert-mcdermott/clonoloom`)}: ${choiceWords(choice)}, ${plural(r.followed ?? 0, 'clonotype')} in ${plural(r.subjects ?? 0, 'subject')}; ${how}.`);
  }

  return { text: sentences.join(' '), references: refs.map((r, i) => `[${i + 1}] ${r.text}`) };
}

// The project's datasets and their samples' annotation (clonoloom-spec/requirements.md, D10,
// D11). A dataset is recorded by its files' paths and fingerprints (the program reads it again
// from them and says when one changed or moved); its samples are annotated with the fields studies
// compare by. Every edit returns a new project; nothing here mutates its argument.

import { touch } from './project.js';

// The fields of a sample's annotation, in the order the Data view shows them.
export const SAMPLE_FIELDS = [
  { id: 'subject', label: 'Subject', hint: 'The individual the sample comes from: the unit of every comparison between groups.' },
  { id: 'timepoint', label: 'Time point', hint: 'When the sample was taken (day 28, week 4, baseline).' },
  { id: 'tissue', label: 'Tissue', hint: 'Blood, tumor, lymph node, an infusion product…' },
  { id: 'reference', label: 'Reference', hint: 'Marks the subject\'s reference sample and says what it is: infusion product, donor, baseline. Tracking follows its clonotypes; attribution measures each sample against it. A tissue such as infusion product or donor marks it too.' },
  { id: 'cellSubset', label: 'Cell subset', hint: 'CD4, CD8, naive, memory, sorted antigen-specific cells…' },
  { id: 'condition', label: 'Condition', hint: 'The group compared: responder, CMV+, vaccinated…' },
  { id: 'batch', label: 'Batch', hint: 'Library, run or plate, to look for batch effects.' },
  { id: 'hla', label: 'HLA', hint: 'HLA alleles, separated by commas (sensitive: left out of exports on request).' },
];

const FIELD_IDS = new Set(SAMPLE_FIELDS.map((f) => f.id));

// The units a dataset's abundance can be in, as the Data view names them.
export const UNITS = {
  cells: 'cells',
  templates: 'templates',
  reads: 'reads',
  umis: 'UMIs',
  sequences: 'sequences (no counts)',
  '': 'unknown',
};

// A dataset's record in the project, from the program's summary (api_datasets.go).
export function datasetRecord(summary, now = new Date().toISOString()) {
  return {
    id: summary.id,
    name: summary.name,
    // The files, as the program read them: opening the dataset again reads these paths and checks
    // their fingerprints.
    sources: summary.sources.map((s) => ({ path: s.path, name: s.name, size: s.size, sha256: s.sha256, modified: s.modified ?? '', format: s.format, ...(s.mapping ? { mapping: s.mapping } : {}) })),
    // The AIRR Repertoire metadata read with them, part of the dataset in the same way.
    metadata: (summary.metadata ?? []).map((m) => ({ path: m.path, name: m.name, size: m.size, sha256: m.sha256, modified: m.modified ?? '' })),
    formats: summary.formats,
    // The unit the formats give ('' when they leave it open); unitConfirmed is the user's choice.
    unit: summary.unit,
    unitConfirmed: null,
    singleCell: summary.singleCell,
    rows: summary.rows,
    samples: summary.samples.map((s) => s.name),
    findings: { fatal: summary.report.fatal, warnings: summary.report.warnings, notes: summary.report.notes },
    readerVersion: summary.readerVersion,
    // The example data its files are: their source, license and citation (for the methods).
    credits: (summary.credits ?? []).map((c) => ({ example: c.example, title: c.title, source: c.source, page: c.page, license: c.license, licenseUrl: c.licenseUrl ?? '', licenseClass: c.licenseClass, citation: c.citation })),
    opened: now,
  };
}

// The unit a dataset's results are in: the user's confirmed one, else the format's.
export function datasetUnit(dataset) {
  return dataset.unitConfirmed || dataset.unit || '';
}

function withHistory(project, action, detail, now) {
  return touch({ ...project, history: [...project.history, { time: now, action, detail }] }, now);
}

// Adds a dataset (or updates one already there, opened again).
export function addDataset(project, record, now = new Date().toISOString()) {
  const at = project.datasets.findIndex((d) => d.id === record.id);
  if (at >= 0) {
    const datasets = project.datasets.slice();
    datasets[at] = { ...record, name: datasets[at].name, unitConfirmed: datasets[at].unitConfirmed };
    return touch({ ...project, datasets }, now);
  }
  const files = record.sources.length === 1 ? record.sources[0].name : `${record.sources.length} files`;
  return withHistory({ ...project, datasets: [...project.datasets, record] }, 'Opened a dataset', `${record.name} (${files}): ${record.samples.length} sample${record.samples.length === 1 ? '' : 's'}, ${record.rows.toLocaleString('en-US')} rows, ${record.findings.fatal} fatal, ${record.findings.warnings} warnings`, now);
}

export function removeDataset(project, id, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === id);
  if (!dataset) return project;
  return withHistory({
    ...project,
    datasets: project.datasets.filter((d) => d.id !== id),
    samples: (project.samples ?? []).filter((s) => s.dataset !== id),
  }, 'Removed a dataset', dataset.name, now);
}

export function renameDataset(project, id, name, now = new Date().toISOString()) {
  const trimmed = String(name).trim();
  if (!trimmed) return project;
  return touch({ ...project, datasets: project.datasets.map((d) => (d.id === id ? { ...d, name: trimmed } : d)) }, now);
}

// Confirms a dataset's unit (or clears the confirmation with null).
export function confirmUnit(project, id, unit, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === id);
  if (!dataset) return project;
  return withHistory({ ...project, datasets: project.datasets.map((d) => (d.id === id ? { ...d, unitConfirmed: unit || null } : d)) },
    'Confirmed a unit', `${dataset.name}: ${unit ? UNITS[unit] ?? unit : 'as the format gives it'}`, now);
}

// A dataset's files moved: the paths the program found them at (same fingerprints), in the order
// of datasetFiles (its tables, then its metadata).
export function relocate(project, id, paths, now = new Date().toISOString()) {
  return touch({
    ...project,
    datasets: project.datasets.map((d) => {
      if (d.id !== id) return d;
      const n = d.sources.length;
      return { ...d, sources: d.sources.map((s, i) => ({ ...s, path: paths[i] ?? s.path })), metadata: (d.metadata ?? []).map((m, i) => ({ ...m, path: paths[n + i] ?? m.path })) };
    }),
  }, now);
}

// The files a dataset is read from: its tables, then its metadata.
export function datasetFiles(dataset) {
  return [...dataset.sources, ...(dataset.metadata ?? [])];
}

// A sample's annotation: { dataset, sample, fields: { subject: …, … }, origin: { field: 'manual' |
// 'metadata' | 'file name' } }, or an empty one.
export function annotation(project, datasetId, sample) {
  return (project.samples ?? []).find((s) => s.dataset === datasetId && s.sample === sample) ?? { dataset: datasetId, sample, fields: {}, origin: {} };
}

// Sets fields of samples' annotations: edits = [{ dataset, sample, field, value, origin }]. An
// empty value removes the field (missing, not an empty string).
export function annotate(project, edits, label = 'Annotated samples', now = new Date().toISOString()) {
  const samples = (project.samples ?? []).map((s) => ({ ...s, fields: { ...s.fields }, origin: { ...s.origin } }));
  let changed = 0;
  for (const edit of edits) {
    if (!FIELD_IDS.has(edit.field)) throw new Error(`Unknown sample field ${edit.field}.`);
    let entry = samples.find((s) => s.dataset === edit.dataset && s.sample === edit.sample);
    if (!entry) {
      entry = { dataset: edit.dataset, sample: edit.sample, fields: {}, origin: {} };
      samples.push(entry);
    }
    const value = String(edit.value ?? '').trim();
    if ((entry.fields[edit.field] ?? '') === value) continue;
    changed += 1;
    if (value) {
      entry.fields[edit.field] = value;
      entry.origin[edit.field] = edit.origin ?? 'manual';
    } else {
      delete entry.fields[edit.field];
      delete entry.origin[edit.field];
    }
  }
  if (!changed) return project;
  return withHistory({ ...project, samples: samples.filter((s) => Object.keys(s.fields).length) }, label, `${changed} value${changed === 1 ? '' : 's'}`, now);
}

// Fields suggested from file names. Adapted from CytoWeave 0.8.1 web/lib/workspace.js
// (suggestFieldsFromNames): names are split on _ - . and spaces, and each position whose parts
// differ between samples becomes a field, named by what its values look like. Time-like parts (d7,
// day28, wk4, 24h, T0, baseline, pre, post) are time points; batch-like parts (B1, batch2, run3,
// plate1) batches; tissue words (PBMC, blood, tumor, TIL, LN, BM, product) tissues; cell subsets
// (CD4, CD8, Treg, naive, memory, tetramer+) cell subsets; of the rest, ID-like parts with many
// values (P01, S12, Patient3, HIP00110) are subjects, and the word-like part with the fewest
// distinct values is the condition. Returns [{ field, values }] with one value per name, in the
// names' order of parts.
export function suggestFromNames(names) {
  const tokens = names.map((name) => String(name)
    .replace(/(\.(tsv|csv|txt|tab|gz|airr))+$/i, '')
    .replace(/_(filtered|all)_contig_annotations$|_airr_rearrangement$/i, '')
    .split(/[_\-\s.]+/).filter(Boolean));
  const width = Math.max(0, ...tokens.map((t) => t.length));
  const columns = [];
  for (let i = 0; i < width; i += 1) {
    const values = tokens.map((t) => t[i] ?? '');
    const distinct = new Set(values);
    if (distinct.size > 1) columns.push({ position: i, values, distinct: distinct.size });
  }
  const every = (column, pattern) => column.values.every((v) => pattern.test(v));
  const TIME = /^((day|d|wk|week|w|month|m|tp|t|h|hr)\d+|\d+(\.\d+)?(h|hr|hrs|min|d|w|wk|mo)|baseline|pre|post|screening|infusion)$/i;
  const BATCH = /^(b|batch|plate|run|lane|lib|library|seq)\d+$/i;
  const TISSUE = /^(blood|pbmc|wb|plasma|spleen|spl|ln|lymphnode|bm|marrow|thymus|liver|lung|skin|gut|colon|tumou?r|til|tils|csf|bal|kidney|brain|product|infusion|apheresis|normal|adjacent)$/i;
  const SUBSET = /^(cd4|cd8|cd4\+|cd8\+|treg|tconv|naive|memory|tem|tcm|temra|tscm|mait|nkt|gd|tet\+?|tetramer\+?|dex\+?|sorted|bcell|bcells|plasmablast|pb|gc|mbc)$/i;
  const replicateLike = (column) => every(column, /^(rep|r|replicate|well)\d+$/i);
  const idLike = (column) => every(column, /^[a-z]{0,10}\d+[a-z]?$/i) && !replicateLike(column);
  const out = [];
  const take = (column, field) => {
    out.push({ field, position: column.position, values: column.values });
    columns.splice(columns.indexOf(column), 1);
  };
  for (const column of columns.filter((c) => every(c, BATCH))) if (!out.some((o) => o.field === 'batch')) take(column, 'batch');
  for (const column of columns.filter((c) => every(c, TIME))) if (!out.some((o) => o.field === 'timepoint')) take(column, 'timepoint');
  for (const column of columns.filter((c) => every(c, TISSUE))) if (!out.some((o) => o.field === 'tissue')) take(column, 'tissue');
  for (const column of columns.filter((c) => every(c, SUBSET))) if (!out.some((o) => o.field === 'cellSubset')) take(column, 'cellSubset');
  const subjects = columns.filter(idLike).sort((a, b) => b.distinct - a.distinct);
  if (subjects.length) take(subjects[0], 'subject');
  const conditions = columns.filter((c) => !idLike(c) && !replicateLike(c)).sort((a, b) => a.distinct - b.distinct);
  if (conditions.length) take(conditions[0], 'condition');
  return out.sort((a, b) => a.position - b.position).map(({ field, values }) => ({ field, values }));
}

// A data unit's value that says nothing ("no", "none"), as OAS and OTS write a field they lack.
const NOTHING = /^(no|none|null|unknown|unclear|na|n\/a|)$/i;

// Fields a sample's own metadata gives: AIRR Repertoire metadata (subject, tissue, cell subset,
// time point, diagnosis or study group as the condition, HLA); immunoSEQ's sample tags (HLA
// alleles, and a CMV serostatus as the condition); an OAS or OTS data unit's metadata (Subject,
// BSource or TSource as the tissue, BType or TType with TSubtype as the cell subset, Disease with
// CancerType as the condition); and a tcrdist3 clone table's subject and epitope (the condition).
export function fieldsFromMetadata(sample) {
  const out = {};
  const m = sample.metadata ?? {};
  const given = (key) => (m[key] != null && !NOTHING.test(String(m[key]).trim()) ? String(m[key]).trim() : '');
  if (given('Subject')) out.subject = given('Subject');
  if (given('BSource') || given('TSource')) out.tissue = given('BSource') || given('TSource');
  const type = given('BType') || given('TType');
  if (type) out.cellSubset = given('TSubtype') ? `${type} (${given('TSubtype')})` : type;
  if (given('Disease')) out.condition = given('CancerType') ? `${given('Disease')} (${given('CancerType')})` : given('Disease');
  if (given('subject')) out.subject = given('subject');
  if (given('epitope')) out.condition = given('epitope');
  const r = sample.repertoire;
  if (r) {
    if (r.subjectId) out.subject = r.subjectId;
    if (r.tissue) out.tissue = r.tissue;
    if (r.cellSubset) out.cellSubset = r.cellSubset;
    if (r.timePoint) out.timepoint = r.timePoint;
    if (r.diagnosis) out.condition = r.diagnosis;
    else if (r.studyGroup) out.condition = r.studyGroup;
    const hla = (r.hla ?? []).filter((a) => /^HLA-/i.test(a));
    if (hla.length) out.hla = hla.join(', ');
  }
  const tags = sample.metadata?.sample_tags;
  if (tags) {
    const parts = tags.split(',').map((t) => t.trim()).filter(Boolean);
    const hla = [];
    for (const part of parts) {
      const value = part.includes(':') ? part.slice(part.indexOf(':') + 1).trim() : part;
      if (/^HLA-[A-Z0-9]+\*/i.test(value) && !/inferred/i.test(part)) hla.push(value);
      const cmv = /cytomegalovirus\s*([+-])/i.exec(part);
      if (cmv && !out.condition) out.condition = `CMV${cmv[1]}`;
    }
    if (hla.length && !out.hla) out.hla = [...new Set(hla)].join(', ');
  }
  return out;
}

// A sample's field as the analyses use it: its annotation's, else its metadata's (sample: the
// program's summary of the sample, with its repertoire and metadata).
export function fieldValue(project, datasetId, sample, field) {
  return annotation(project, datasetId, sample.name).fields[field] ?? fieldsFromMetadata(sample)[field];
}

// The samples of a dataset as the study's design is read from them (api_study.go): each one's
// subject, time point, tissue and reference.
export function studySamples(project, datasetId, summary) {
  return (summary?.samples ?? []).map((s) => {
    const out = { name: s.name };
    for (const field of ['subject', 'timepoint', 'tissue', 'reference']) {
      const v = fieldValue(project, datasetId, s, field);
      if (v) out[field] = v;
    }
    return out;
  });
}

// Whether a recorded file is the one the program found: same size and SHA-256.
export function sameFile(recorded, found) {
  return recorded.size === found.size && recorded.sha256 === found.sha256;
}

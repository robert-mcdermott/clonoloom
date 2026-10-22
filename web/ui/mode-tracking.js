// The Tracking view (clonoloom-spec/requirements.md, T1–T6; roadmap.md, wave 3). Its first part,
// Subjects (slice 3: D10, G3): each subject's samples in time order, from the samples' annotation
// (their own fields, else their metadata's), read and checked by the program (api_study.go,
// internal/study): the time points as days from each subject's reference event, the reference
// samples (an infusion product, a donor), the samples taken at one time (replicates), and what is
// left out of tracking and why. The parts that follow clonotypes and test them come with their
// slices.

import { h, icon, clear } from './dom.js';
import { timeline } from '../lib/plot.js';
import { fieldValue, studySamples } from '../lib/datasets.js';
import { exportable, madeBy } from './figures.js';

const SEVERITY = {
  warning: { label: 'Warning', badge: 'warn', icon: 'warning' },
  note: { label: 'Note', badge: 'accent', icon: 'info' },
};

const count = (n) => Number(n ?? 0).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;

async function postJSON(url, body) {
  const response = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const answer = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(answer.error ?? `${url}: HTTP ${response.status}`);
  return answer;
}

// The words a time point is shown in, with its event where the annotation names one.
const when = (t) => (t ? `${t.label}${t.event ? ` (${t.event})` : ''}` : 'no time point');

export function mountTracking(app, container) {
  const { store } = app;
  const datasetSelect = h('select.input.small', { 'aria-label': 'Dataset', onchange: (e) => { chosen = e.target.value; load(); } });
  const annotateButton = h('button.btn.small', { type: 'button', title: 'Give the samples their subjects, time points and reference in the Data view\'s samples table', onclick: () => app.setMode('data') }, icon('edit'), 'Annotate samples');
  const head = h('div.workbench-head', h('h1', icon('threads'), 'Tracking'),
    h('div.segmented', { role: 'tablist', 'aria-label': 'Part' }, h('button.active', { type: 'button', role: 'tab', 'aria-selected': 'true' }, 'Subjects')),
    h('span.spacer'), h('label.kv-inline', h('span.muted', 'Dataset'), datasetSelect), annotateButton);
  const body = h('div.view-body.tracking-view');
  const root = h('div.view', head, h('div.workbench-scroll', body));
  container.append(root);
  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id ?? null;
  // idle | loading | ready | failed, with the design and the samples it was read from.
  let state = { status: 'idle' };
  let generation = 0;

  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0] ?? null;

  async function load() {
    const mine = ++generation;
    const d = dataset();
    renderHead();
    if (!d) {
      state = { status: 'idle' };
      render();
      return;
    }
    if (state.status !== 'ready' || state.dataset !== d.id) {
      state = { status: 'loading' };
      render();
    }
    try {
      const open = await app.ensureDataset(d);
      if (open.status !== 'open') throw new Error(open.error ?? `${d.name} could not be opened.`);
      const samples = studySamples(store.project, d.id, open.summary);
      const design = await postJSON('api/study', { samples });
      if (mine !== generation) return;
      state = { status: 'ready', dataset: d.id, design, samples, summary: open.summary };
    } catch (error) {
      if (mine !== generation) return;
      state = { status: 'failed', error: error.message };
    }
    render();
  }

  function renderHead() {
    clear(datasetSelect);
    for (const d of store.project.datasets) datasetSelect.append(h('option', { value: d.id, selected: d.id === dataset()?.id }, d.name));
    datasetSelect.disabled = !store.project.datasets.length;
  }

  function render() {
    clear(body);
    const d = dataset();
    annotateButton.hidden = !d;
    if (!d) {
      body.append(h('div.empty', icon('threads'), h('h3', 'No data yet'),
        h('p', 'Open a study\'s samples, give each its subject and time point (and mark each subject\'s reference sample, such as an infusion product), and follow its clonotypes through time here.'),
        h('div.btn-row', { style: { justifyContent: 'center', marginTop: '10px' } },
          h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data'),
          h('button.btn', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data'))));
      return;
    }
    if (state.status === 'loading' || state.status === 'idle') {
      body.append(h('p.muted', 'Reading the samples…'));
      return;
    }
    if (state.status === 'failed') {
      body.append(h('div.callout.danger', h('p', state.error)));
      return;
    }
    const { design } = state;
    const subjects = design.subjects;
    const placed = subjects.reduce((n, s) => n + s.samples.length, 0);
    const references = subjects.reduce((n, s) => n + (s.references?.length ?? 0), 0);
    const replicates = subjects.reduce((n, s) => n + (s.replicates?.length ?? 0), 0);
    body.append(h('p.tracking-summary',
      `${plural(subjects.length, 'subject')}, ${plural(placed, 'sample')} placed in time`,
      references ? `, ${plural(references, 'reference sample')}` : '',
      replicates ? `, ${plural(replicates, 'group')} of samples taken at one time` : '', '.'));
    if (!subjects.length) {
      body.append(h('div.callout', h('p', 'No sample has a subject yet. Tracking follows clonotypes within a subject: give each sample its subject and time point in the Data view\'s samples table, or fill them from the files\' metadata or names.')));
    }
    for (const f of design.findings) body.append(finding(f));
    if (subjects.length) {
      body.append(timelineCard(d, design));
      body.append(subjectTable(design));
    }
  }

  function finding(f) {
    const s = SEVERITY[f.severity] ?? SEVERITY.note;
    const of = f.of ? ` of ${count(f.of)}` : '';
    return h(`div.finding.${f.severity}`,
      h('div.finding-head',
        h(`span.badge.${s.badge}`, icon(s.icon), s.label),
        h('b', f.title),
        h('span.muted', `${count(f.count)}${of}`),
        h('span.spacer'),
        h('code.finding-code', { title: 'The finding\'s code, the same in every version' }, f.code)),
      h('p', f.detail),
      f.examples?.length ? h('p.muted.small', `For example: ${f.examples.join('; ')}${f.count > f.examples.length ? '; …' : ''}.`) : null,
      f.remedy ? h('p.finding-remedy', h('b', 'What to do: '), f.remedy) : null);
  }

  // The subjects' samples on one axis of days (or visits, where no sample has a day), colored by
  // the subject's condition.
  function timelineCard(d, design) {
    const days = design.subjects.some((s) => s.samples.some((p) => p.time?.kind === 'day'));
    const summaryOf = new Map((state.summary?.samples ?? []).map((s) => [s.name, s]));
    const conditionOf = (subject) => {
      for (const p of subject.samples) {
        const s = summaryOf.get(p.name);
        const c = s && fieldValue(store.project, d.id, s, 'condition');
        if (c) return c;
      }
      return null;
    };
    const conditions = [...new Set(design.subjects.map(conditionOf).filter(Boolean))];
    const classOf = (subject) => {
      const c = conditionOf(subject);
      return c ? `plot-s${(conditions.indexOf(c) % 6) + 1}` : 'plot-accent';
    };
    const rows = design.subjects.map((s) => ({
      label: s.id, className: classOf(s),
      marks: s.samples.flatMap((p) => {
        const label = `${p.name}: ${p.reference ? `${p.reference}, ` : ''}${when(p.time)}`;
        if (!p.time) return [{ column: 'reference', label, reference: true }];
        switch (p.time.kind) {
          case 'before': return [{ column: 'baseline', label, reference: !!p.reference }];
          case 'after': return [{ column: 'after', label, reference: !!p.reference }];
          case 'visit': return days ? [] : [{ x: p.time.value, label, reference: !!p.reference }];
          default: return [{ x: p.time.days, label, reference: !!p.reference }];
        }
      }),
    }));
    // The axis says from what event, where every subject's time points name the same one.
    const events = [...new Set(design.subjects.flatMap((s) => s.samples.map((p) => p.time?.event).filter(Boolean)))];
    const xLabel = !days ? 'Visit' : events.length === 1 ? `Days from ${events[0]}` : 'Days from each subject\'s reference event (day 0)';
    const plot = timeline(rows, { label: `Subjects of ${d.name}`, xLabel, width: 680, labelWidth: 96 });
    const legend = [
      ...conditions.map((c, i) => ({ label: c, className: `plot-s${(i % 6) + 1}` })),
      ...(design.subjects.some((s) => s.references?.length) ? [{ label: 'reference sample', className: 'plot-reference' }] : []),
    ];
    const visitsLeftOut = days && design.subjects.some((s) => s.samples.some((p) => p.time?.kind === 'visit'));
    const note = `Each subject's samples by their time point${days ? ', in days' : ', by visit'}; reference samples as diamonds, samples taken at one time one above the other.${visitsLeftOut ? ' Samples given as visits are listed below, not placed among days.' : ''}`;
    const card = h('figure.plot-card',
      h('figcaption', 'Each subject\'s samples in time'),
      h('div.plot-holder', { html: plot.svg }),
      legend.length ? h('div.plot-legend', ...legend.map((l) => h('span', h(`span.swatch.${l.className}`), l.label))) : null,
      h('p.muted.small', note));
    return exportable(card, () => ({
      name: `${d.name} subjects`, title: `Subjects of ${d.name}`, plot: plot.svg, legend,
      caption: [note, `${d.name}: ${plural(design.subjects.length, 'subject')}. Time points as annotated, read by ClonoLoom.`, madeBy(app.version)],
    }));
  }

  // Each subject's samples in order: what each is, when, and what is left out.
  function subjectTable(design) {
    const rows = design.subjects.map((s) => {
      const replicateOf = new Map();
      (s.replicates ?? []).forEach((group, i) => group.forEach((n) => replicateOf.set(n, i + 1)));
      return h('tr',
        h('td', h('b', s.id)),
        h('td', h('div.subject-samples', ...s.samples.map((p) => h('span.subject-sample', { title: p.referenceFrom === 'tissue' ? 'A reference sample by its tissue' : null },
          p.reference ? h('span.badge.accent', p.reference) : null,
          h('span.mono', p.name), h('span.muted', { title: when(p.time) }, p.time ? p.time.label : 'no time point'),
          replicateOf.has(p.name) ? h('span.badge', { title: 'Taken at one time with other samples: replicates' }, `replicate ${replicateOf.get(p.name)}`) : null)))),
        h('td.muted', (s.untimed ?? []).join(', ') || '—'));
    });
    const table = h('table.data.subjects-table',
      h('thead', h('tr', h('th', 'Subject'), h('th', 'Samples, in time order'), h('th', { title: 'Samples without a time point, or with one not understood' }, 'Left out of tracking'))),
      h('tbody', ...rows));
    const section = h('section', { 'aria-label': 'Subjects' }, h('div.section-title', `Subjects · ${design.subjects.length}`), h('div.table-scroll', { tabIndex: 0 }, table));
    if (design.unassigned?.length) section.append(h('p.muted.small', `Without a subject: ${design.unassigned.join(', ')}.`));
    return section;
  }

  // The annotation edited (in the Data view, or by a script) or datasets opened: read again.
  let shownChoices = '';
  const choices = () => {
    const d = dataset();
    const summary = d ? app.datasetStates.get(d.id)?.summary : null;
    return JSON.stringify([d?.id ?? null, d && summary ? studySamples(store.project, d.id, summary) : null, d && summary ? summary.samples.map((s) => fieldValue(store.project, d.id, s, 'condition') ?? null) : null]);
  };
  function update(topics) {
    if (topics.has('project') || topics.has('project-loaded') || topics.has('datasets')) {
      if (!store.project.datasets.some((d) => d.id === chosen)) chosen = store.project.datasets[0]?.id ?? null;
      const now = choices();
      if (topics.has('project-loaded') || now !== shownChoices) {
        shownChoices = now;
        load();
      } else renderHead();
    }
  }

  app.trackingView = {
    load: () => load(),
    state: () => state,
    choose: (id) => {
      chosen = id;
      return load();
    },
  };
  shownChoices = choices();
  load();
  return {
    update,
    destroy() {
      app.trackingView = null;
      root.remove();
    },
  };
}

// The project tree: what the project holds, by kind (datasets, results, saved views), and what was
// handed to ClonoLoom on this computer but is not in the project yet. Selecting an item focuses it
// (store.ui.focus): the inspector describes it, and the views that use it open on it. Adapted from
// SituCraft 0.9.0 web/ui/sidebar.js.

import { h, icon, clear, formatBytes } from './dom.js';

// The icon of an item handed to the program, by its kind (local.go, fileKind and dirKind).
export const KIND_ICONS = {
  cellranger: 'chip', table: 'table', metadata: 'braces', expression: 'grid', matrix: 'grid',
  fasta: 'dna', archive: 'archive', project: 'library',
};

export const KIND_LABELS = {
  cellranger: 'Cell Ranger output', table: 'Table', metadata: 'Metadata', expression: 'Gene expression',
  matrix: 'Matrix', fasta: 'FASTA', archive: 'Archive', project: 'Project archive',
};

const SECTIONS = [
  {
    kind: 'local', title: 'On this computer', icon: 'folder',
    items: (project, app) => app.localItems,
    label: (item) => item.name,
    itemIcon: (item) => KIND_ICONS[item.kind] ?? 'file',
    meta: (item) => (item.dir ? KIND_LABELS[item.kind] ?? item.kind : formatBytes(item.size)),
    hideWhenEmpty: true,
  },
  {
    kind: 'dataset', title: 'Datasets', icon: 'database',
    items: (project) => project.datasets,
    label: (d) => d.name ?? 'Dataset',
    meta: (d) => d.format ?? '',
    empty: 'Repertoire files you open appear here, each checked before it is used.',
    action: { label: 'Open data', run: (app) => app.openPathDialog() },
  },
  {
    kind: 'result', title: 'Results', icon: 'report',
    items: (project) => project.artifacts,
    label: (a) => a.name ?? 'Result',
    meta: (a) => a.method ?? '',
    empty: 'Filters, analyses and tests, each kept with its clonotype definition and how it was made.',
  },
  {
    kind: 'view', title: 'Saved views', icon: 'eye',
    items: (project) => project.views,
    label: (v) => v.name ?? 'View',
    meta: () => '',
    empty: 'A view with its samples, definition and selection, reopened exactly.',
  },
];

export function mountSidebar(app) {
  const root = document.getElementById('sidebar');

  function render() {
    const { project, ui } = app.store;
    clear(root);
    const body = h('div.panel-body', { style: { paddingTop: '4px' } });
    for (const section of SECTIONS) {
      const items = section.items(project, app);
      if (!items.length && section.hideWhenEmpty) continue;
      const list = h('div', { role: 'list' });
      for (const item of items) {
        const id = item.id ?? item.url;
        const selected = ui.focus?.kind === section.kind && ui.focus?.id === id;
        list.append(h(`button.dataset-row${selected ? '.selected' : ''}`, {
          type: 'button',
          role: 'listitem',
          'aria-current': selected ? 'true' : null,
          title: section.label(item),
          onclick: () => app.focusItem({ kind: section.kind, id }),
        }, icon(section.itemIcon?.(item) ?? section.icon), h('span.label', section.label(item)), h('span.meta', section.meta(item))));
      }
      if (!items.length) {
        list.append(h('p.tree-empty', section.empty));
        if (section.action) list.append(h('button.drop-hint', { type: 'button', style: { width: 'calc(100% - 12px)' }, onclick: () => section.action.run(app) }, section.action.label));
      }
      body.append(h('section.tree-section', { 'aria-label': section.title },
        h('div.panel-head', h('h2', section.title, items.length ? h('span.muted', `· ${items.length}`) : null)),
        list));
    }
    root.append(h('div.panel', { style: { flex: '1' } }, body));
  }

  return {
    render,
    update(topics) {
      if (topics.has('project') || topics.has('focus') || topics.has('project-loaded') || topics.has('local')) render();
    },
  };
}

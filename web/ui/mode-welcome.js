// The start page: open data, or a saved project. Adapted from SituCraft 0.9.0
// web/ui/mode-welcome.js.

import { h, icon, clear, relativeTime, formatBytes } from './dom.js';
import { prefs } from './storage.js';

// What a researcher brings (clonoloom-spec/requirements.md, D1–D6).
const INPUTS = [
  ['table', 'AIRR rearrangement tables', 'The AIRR Community\'s standard TSV, written by IgBLAST and Change-O, MiXCR, TRUST4, Cell Ranger and the AIRR Data Commons, with its Repertoire metadata.'],
  ['chip', 'Cell Ranger V(D)J outputs', 'A vdj or multi run\'s folder: contig annotations, clonotypes and metrics of single-cell paired TCR or BCR, read where it is.'],
  ['database', 'immunoSEQ exports', 'Bulk TCR or BCR exports from the immunoSEQ Analyzer and immuneACCESS, with templates and reads kept apart.'],
  ['dna', 'More, as ClonoLoom grows', 'MiXCR clone tables, OAS and OTS data units, specificity databases, and gene expression (AnnData) to link to receptors.'],
];

export function mountWelcome(app, container) {
  const { library } = app;
  const recent = h('div.welcome-grid');
  const root = h('div.workbench-scroll', h('div.welcome',
    h('div.welcome-hero',
      h('div', { style: { flex: 1 } },
        h('h1', 'Immune repertoires, from receptor tables ', h('span.brand-text', 'to results others can check.')),
        h('p', 'ClonoLoom opens T- and B-cell receptor data from bulk and single-cell sequencing, checks them, and defines clonotypes the way you choose and can see. Measure diversity and overlap, follow clonotypes over time, group receptors by likely specificity and trace B-cell families, on your own computer, with every step recorded.'),
        h('div.btn-row',
          h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data'),
          h('button.btn', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data'),
          h('button.btn', { type: 'button', onclick: () => app.openLibrary() }, icon('library'), 'Open a saved project'))),
      h('img', { src: 'favicon.svg', width: 132, height: 132, alt: '', style: { filter: 'drop-shadow(0 18px 40px rgba(200, 64, 111, 0.35))', borderRadius: '28px' } })),
    h('div.section-title', 'Recent projects'),
    recent,
    h('div.section-title', { style: { marginTop: '22px' } }, 'What to bring'),
    h('div.feature-list', ...INPUTS.map(([glyph, title, text]) => h('div.feature', h('span.glyph', icon(glyph)), h('div', h('b', title), h('span', text))))),
    h('p.muted', { style: { marginTop: '22px', fontSize: '12px' } },
      'Data are read on this computer and never uploaded. ClonoLoom is for research use only; it is not a medical device and makes no diagnostic claims.')));
  container.append(root);

  async function loadRecent() {
    clear(recent);
    let list = [];
    try {
      list = await library.listProjects();
    } catch { /* none */ }
    if (!list.length) {
      recent.append(h('div.card', h('p.muted', `No saved projects yet. Projects save automatically to the library folder (${library.location}).`)));
      return;
    }
    const last = prefs.get('lastProject', null);
    for (const item of list.slice(0, 8)) {
      recent.append(h('div.card.clickable', { role: 'button', tabIndex: 0, onclick: () => app.openProject(item.id), onkeydown: (event) => { if (event.key === 'Enter') app.openProject(item.id); } },
        h('h4', icon('library'), item.name || 'Untitled project', item.id === last ? h('span.badge.accent', { style: { marginLeft: '6px' } }, 'Last opened') : null),
        h('p', `${item.datasets ?? 0} dataset${item.datasets === 1 ? '' : 's'} · ${relativeTime(item.modified)}${item.size ? ` · ${formatBytes(item.size)}` : ''}`)));
    }
  }

  loadRecent();
  return {
    update(topics) {
      if (topics.has('library')) loadRecent();
    },
    destroy() {
      root.remove();
    },
  };
}

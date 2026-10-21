// The inspector: what is in focus (a dataset, a sample, a clonotype, a cell), described in full.
// Views add their own sections through app.inspector.setSection(id, builder) as they are built;
// until something is in focus it describes the project and says how to begin. Adapted from
// SituCraft 0.9.0 web/ui/inspector.js.

import { h, icon, relativeTime, formatBytes } from './dom.js';
import { KIND_ICONS, KIND_LABELS } from './sidebar.js';

export function mountInspector(app) {
  const root = document.getElementById('inspector');
  // Sections contributed by views: id → (app) => Node | null.
  const contributed = new Map();

  function projectSection() {
    const { project } = app.store;
    const kv = h('dl.kv',
      h('dt', 'Name'), h('dd', project.name),
      h('dt', 'Datasets'), h('dd', String(project.datasets.length)),
      h('dt', 'Results'), h('dd', String(project.artifacts.length)),
      h('dt', 'Modified'), h('dd', relativeTime(project.modified)));
    return h('section.inspector-section', h('h3', icon('library'), 'Project'), kv);
  }

  // An item handed to the program on this computer: what ClonoLoom takes it to be.
  function localSection(item) {
    return h('section.inspector-section',
      h('h3', icon(KIND_ICONS[item.kind] ?? 'file'), item.name),
      h('dl.kv',
        h('dt', 'Kind'), h('dd', KIND_LABELS[item.kind] ?? item.kind),
        item.folder ? [h('dt', 'Found in'), h('dd', item.folder)] : null,
        item.dir ? null : [h('dt', 'Size'), h('dd', formatBytes(item.size))]),
      h('p.muted', { style: { margin: '8px 0 0', fontSize: '12px' } }, item.dir
        ? 'Read where it is: ClonoLoom never copies, changes or moves its files.'
        : 'Read on this computer; nothing is uploaded.'));
  }

  function focusSection() {
    const focus = app.store.ui.focus;
    if (focus?.kind === 'local') {
      const item = app.localItems.find((f) => f.url === focus.id);
      if (item) return localSection(item);
    }
    if (!focus) {
      return h('section.inspector-section',
        h('h3', icon('info'), 'Nothing selected'),
        h('p.muted', { style: { margin: 0, fontSize: '12.5px' } }, 'Select an item in the project tree, a sample or a clonotype to see everything known about it here: identifiers, sequences, genes, counts, and where each number came from.'));
    }
    return null;
  }

  function render() {
    root.replaceChildren();
    root.append(focusSection());
    for (const build of contributed.values()) {
      try {
        root.append(build(app));
      } catch (error) {
        root.append(h('section.inspector-section', h('p.muted', `This section could not be shown: ${error.message}`)));
      }
    }
    root.append(projectSection());
    root.append(h('section.inspector-section', h('p.muted', { style: { margin: 0, fontSize: '11.5px' } },
      'ClonoLoom is for research use only. It is not a medical device.')));
  }

  return {
    render,
    setSection(id, build) {
      if (build) contributed.set(id, build);
      else contributed.delete(id);
      render();
    },
    update(topics) {
      if (topics.has('project') || topics.has('focus') || topics.has('selection') || topics.has('project-loaded') || topics.has('local') || topics.has('datasets')) render();
    },
  };
}

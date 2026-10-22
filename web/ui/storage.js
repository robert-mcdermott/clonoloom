// The project library, kept by the ClonoLoom program in its data folder (served at /api/library):
// projects as JSON documents, small files stored once each by SHA-256, and records kept across
// projects by kind (clonotype definitions, presets): listRecords(kind), getRecord(kind, id),
// putRecord(kind, id, doc), deleteRecord(kind, id). As SituCraft's, it has no browser-storage
// fallback: projects refer to data by their place on this computer, so ClonoLoom's page needs its
// program. Adapted from SituCraft 0.9.0 web/ui/storage.js.

// The program's description of itself (/api/info), or null when the page is not served by it.
export async function detectBackend() {
  try {
    const response = await fetch('api/info', { cache: 'no-store' });
    if (!response.ok) return null;
    const info = await response.json();
    return info?.name === 'ClonoLoom' ? info : null;
  } catch {
    return null;
  }
}

export async function sha256Hex(bytes) {
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

async function failure(response, fallback) {
  return new Error((await response.json().catch(() => ({}))).error ?? fallback);
}

export function createLibrary(info) {
  const path = (...parts) => ['api', 'library', ...parts.map(encodeURIComponent)].join('/');
  return {
    location: info.dataDir,

    async listProjects() {
      const response = await fetch(path('projects'), { cache: 'no-store' });
      if (!response.ok) throw await failure(response, 'Could not list the projects.');
      return (await response.json()).projects;
    },

    async loadProject(id) {
      const response = await fetch(path('projects', id), { cache: 'no-store' });
      if (!response.ok) throw await failure(response, 'The project could not be opened.');
      return response.text();
    },

    async saveProject(id, text) {
      const response = await fetch(path('projects', id), { method: 'PUT', body: text, headers: { 'Content-Type': 'application/json' } });
      if (!response.ok) throw await failure(response, 'The project could not be saved.');
    },

    async deleteProject(id) {
      const response = await fetch(path('projects', id), { method: 'DELETE' });
      if (!response.ok) throw await failure(response, 'The project could not be deleted.');
    },

    async hasFile(sha) {
      const response = await fetch(path('has', sha), { cache: 'no-store' });
      return response.ok && (await response.json()).exists === true;
    },

    async getFile(sha) {
      const response = await fetch(path('files', sha));
      if (!response.ok) return null;
      return new Uint8Array(await response.arrayBuffer());
    },

    // Stores bytes and answers with the SHA-256 the program computed while writing them.
    async addFile(bytes) {
      const response = await fetch(path('files'), { method: 'POST', body: bytes });
      if (!response.ok) throw await failure(response, 'The file could not be stored.');
      return response.json();
    },

    async listRecords(kind) {
      const response = await fetch(path('records', kind), { cache: 'no-store' });
      if (!response.ok) throw await failure(response, 'Could not list the library\'s records.');
      return (await response.json()).records;
    },

    async getRecord(kind, id) {
      const response = await fetch(path('records', kind, id), { cache: 'no-store' });
      if (response.status === 404) return null;
      if (!response.ok) throw await failure(response, 'The record could not be read.');
      return response.json();
    },

    async putRecord(kind, id, doc) {
      const response = await fetch(path('records', kind, id), { method: 'PUT', body: JSON.stringify(doc), headers: { 'Content-Type': 'application/json' } });
      if (!response.ok) throw await failure(response, 'The record could not be saved.');
    },

    async deleteRecord(kind, id) {
      const response = await fetch(path('records', kind, id), { method: 'DELETE' });
      if (!response.ok) throw await failure(response, 'The record could not be deleted.');
    },
  };
}

// Hands paths on this computer (tables, output folders, folders of them) to the program, as a second
// "clonoloom <paths>" launch does. Answers with every item the program now holds and what it
// skipped.
export async function openPaths(paths) {
  const response = await fetch('api/open', { method: 'POST', body: JSON.stringify({ paths }), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) throw await failure(response, 'The program could not open those paths.');
  return response.json();
}

// Small per-browser preferences (theme, drawer, last project): never analysis state.
export const prefs = {
  get(key, fallback) {
    try {
      const raw = localStorage.getItem(`clonoloom:${key}`);
      return raw === null ? fallback : JSON.parse(raw);
    } catch {
      return fallback;
    }
  },
  set(key, value) {
    try {
      localStorage.setItem(`clonoloom:${key}`, JSON.stringify(value));
    } catch { /* storage unavailable */ }
  },
};

// An error the program answered with, with its code (missing, format, not-open) when it gave one.
async function programError(response, fallback) {
  const body = await response.json().catch(() => ({}));
  const error = new Error(body.error ?? fallback);
  error.code = body.code ?? '';
  error.status = response.status;
  return error;
}

// Opens files as one dataset (api_datasets.go): { paths: [...] } on this computer, or { library:
// [{ sha256, name }] } added to the library; answers the dataset's summary.
export async function openDataset(body) {
  const response = await fetch('api/datasets/open', { method: 'POST', body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) throw await programError(response, 'The files could not be opened.');
  return response.json();
}

// The summary of a dataset the program has open, or null when it is not open.
export async function getDataset(id) {
  const response = await fetch(`api/datasets/${encodeURIComponent(id)}`, { cache: 'no-store' });
  if (response.status === 404) return null;
  if (!response.ok) throw await programError(response, 'The dataset could not be described.');
  return response.json();
}

// Finds a file that moved by its fingerprint, in folders near where it was: its path, or null.
export async function locateFile({ name, size, sha256, near }) {
  const response = await fetch('api/datasets/locate', { method: 'POST', body: JSON.stringify({ name, size, sha256, near }), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) return null;
  return (await response.json()).path;
}

// A dataset's QC under a filter's rules (api_qc.go).
export async function datasetQC(dataset, filter) {
  const response = await fetch('api/qc', { method: 'POST', body: JSON.stringify({ dataset, filter: filter ?? {} }), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) throw await programError(response, 'QC could not be computed.');
  return response.json();
}

// A page of a dataset's clonotypes (api_clonotypes.go): { dataset, definition, filter, sort, desc,
// query, offset, limit }.
export async function clonotypePage(request) {
  const response = await fetch('api/clonotypes', { method: 'POST', body: JSON.stringify(request), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) throw await programError(response, 'The clonotypes could not be listed.');
  return response.json();
}

// One clonotype in full: the table's request with its id.
export async function clonotypeDetail(request) {
  const response = await fetch('api/clonotypes/one', { method: 'POST', body: JSON.stringify(request), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) throw await programError(response, 'The clonotype could not be described.');
  return response.json();
}

// A clonotype's chains' generation probabilities (api_pgen.go): { chains, junction, pgen } answers
// each chain's, their product, and the models.
export async function chainsPgen(body) {
  const response = await fetch('api/pgen/chains', { method: 'POST', body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) throw await programError(response, 'The generation probability could not be computed.');
  return response.json();
}

// The clonotype definitions most studies use.
export async function clonotypePresets() {
  const response = await fetch('api/clonotypes/presets', { cache: 'no-store' });
  if (!response.ok) throw await programError(response, 'The definitions could not be listed.');
  return (await response.json()).presets;
}

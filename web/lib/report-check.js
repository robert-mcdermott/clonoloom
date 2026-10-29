// Hash exact bytes, never decoded/re-encoded scientific JSON. This checker is also
// embedded in offline reports; it does not perform native certificate replay.
export async function checkReport(recordBytes, files, embedded, expected, digest) {
  const hex = (s) => typeof s === 'string' && /^[a-f0-9]{64}$/.test(s);
  if (!hex(embedded) || (expected && !hex(expected))) throw new Error('Expected report fingerprint must be 64 lowercase hexadecimal characters.');
  if (!(recordBytes instanceof Uint8Array) || recordBytes.length > 1048576) throw new Error('Unsupported report inventory size.');
  const fingerprint = await digest(recordBytes);
  if (fingerprint !== embedded || (expected && fingerprint !== expected)) throw new Error('Report fingerprint differs.');
  const record = JSON.parse(new TextDecoder('utf-8', {fatal:true}).decode(recordBytes));
  if (record.format !== 'clonoloom-html-report' || record.version !== 1 || record.scope !== 'embedded-report-bytes' || !Array.isArray(record.files) || !Array.isArray(record.results) || !record.results.length || record.results.length > 8 || record.files.length !== 3 + 2*record.results.length || files.length !== record.files.length) throw new Error('Unsupported report inventory or incomplete member set.');
  const needed = new Set(['report.html','geometry.js','report.css']), results = new Set();
  for (const r of record.results) {
    if (typeof r.artifact !== 'string' || !/^[a-zA-Z0-9_-]+$/.test(r.artifact) || results.has(r.artifact) || !['cohort-features','cohort-compare','cohort-counts','batch-qc','motifs'].includes(r.kind) || !hex(r.fingerprint) || r.certificate !== `${r.artifact}.certificate.clz` || r.result !== `${r.artifact}.result.json`) throw new Error('Unsupported report result contract.');
    results.add(r.artifact); needed.add(r.certificate); needed.add(r.result);
  }
  let total = 0, previous = '';
  for (let i=0;i<record.files.length;i++) {
    const f=record.files[i], bytes=files[i];
    if (!needed.delete(f.path) || f.path <= previous || !Number.isSafeInteger(f.size) || f.size < 0 || !hex(f.sha256) || !(bytes instanceof Uint8Array) || bytes.length !== f.size) throw new Error('Missing, duplicate, unordered or changed report file.');
    total += f.size; previous=f.path;
    if (total > 64*1024*1024 || await digest(bytes) !== f.sha256) throw new Error(`Report bytes differ or exceed bounds: ${f.path}`);
  }
  if (needed.size) throw new Error('Report inventory is incomplete.');
  return {record, fingerprint, verdict:'integrity-match', recomputed:false, filesChecked:files.length, bytesChecked:total};
}

// Display contracts only. Go owns certificate bytes, fingerprints and verification.
export const CERTIFICATE_KINDS = ['cohort-features', 'cohort-compare', 'cohort-counts'];
export function certificateResults(project) {
  return (project.artifacts ?? []).filter((r) => CERTIFICATE_KINDS.includes(r.kind));
}
export function integrityLabel(report) {
  if (report?.format !== 'clonoloom-verification' || report.version !== 1 || report.scope !== 'recorded-artifact-bytes' || report.recomputed !== false) return 'Unsupported verification report';
  return report.verdict === 'integrity-match' ? 'Integrity match — not recomputed' : 'Integrity check failed';
}

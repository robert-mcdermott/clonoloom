// Display contracts only. Go owns certificate bytes, fingerprints and verification.
export const CERTIFICATE_KINDS = ['cohort-features', 'cohort-compare', 'cohort-counts'];
export function certificateResults(project) {
  return (project.artifacts ?? []).filter((r) => CERTIFICATE_KINDS.includes(r.kind));
}
export function integrityLabel(report) {
  if (report?.format !== 'clonoloom-verification' || report.version !== 1 || report.scope !== 'recorded-artifact-bytes' || report.recomputed !== false) return 'Unsupported verification report';
  return report.verdict === 'integrity-match' ? 'Integrity match — not recomputed' : 'Integrity check failed';
}
export function verificationLabel(report) {
  if (report?.format !== 'clonoloom-replay') return integrityLabel(report);
  if (report.version !== 1 || report.scope !== 'captured-cohort-evidence') return 'Unsupported verification report';
  if (report.verdict === 'replay-match' && report.recomputed === true && report.integrity?.verdict === 'integrity-match' && report.steps?.length && report.steps.every((s) => s.verdict === 'replay-match' && s.metadataMatched && s.expectedSHA256 === s.actualSHA256)) return 'Exact replay match — captured evidence';
  if (report.verdict === 'differs') return 'Replay differs';
  if (report.verdict === 'unavailable') return 'Replay unavailable';
  return 'Unsupported verification report';
}

// Only the documented effective annotation fields enter a de-identification request.
import { SAMPLE_FIELDS, fieldValue } from './datasets.js';
export function deidentificationAnnotations(project, dataset, samples) {
  return samples.map(sample => ({sample:sample.name,fields:Object.fromEntries(SAMPLE_FIELDS.map(({id})=>[id,fieldValue(project,dataset,sample,id)??'']))}));
}
export function validDeidentificationReceipt(record) {
  return record?.record?.format==='clonoloom-deidentified-tables' && record.record.version===1 && /^[a-f0-9]{64}$/.test(record.token??'') && /^[a-f0-9]{64}$/.test(record.fingerprint??'');
}

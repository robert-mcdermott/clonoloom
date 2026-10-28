import test from 'node:test';
import assert from 'node:assert/strict';
import {deidentificationAnnotations,validDeidentificationReceipt} from './deidentification.js';

test('only effective allowlisted annotations enter preparation; explicit blanks override imported values',()=>{
 const samples=[{name:'sample-A',repertoire:{subjectId:'imported',tissue:'blood',hla:['HLA-A*02:01']} },{name:'sample-B',repertoire:{subjectId:'other-subject'}}];
 const project={samples:[{dataset:'d',sample:'sample-A',fields:{subject:'',condition:'vaccinated',unknown:'secret'}}]};
 const before=JSON.stringify({project,samples});
 const rows=deidentificationAnnotations(project,'d',samples);
 assert.equal(rows[0].fields.subject,'');assert.equal(rows[0].fields.condition,'vaccinated');
 assert.equal(rows[0].fields.unknown,undefined);assert.equal(rows[0].fields.sex,undefined);
 assert.equal(rows[1].fields.subject,'other-subject');assert.equal(rows[0].fields.tissue,'blood');assert.equal(rows[0].fields.hla,'HLA-A*02:01');assert.equal(Object.keys(rows[0].fields).length,8);
 assert.equal(JSON.stringify({project,samples}),before);
});
test('receipts require the supported contract and complete lowercase fingerprints',()=>{
 const receipt={record:{format:'clonoloom-deidentified-tables',version:1},token:'a'.repeat(64),fingerprint:'f'.repeat(64)};
 assert(validDeidentificationReceipt(receipt));
 for(const bad of [{...receipt,token:''},{...receipt,fingerprint:'F'.repeat(64)},{...receipt,record:{...receipt.record,version:2}},null])assert(!validDeidentificationReceipt(bad));
});

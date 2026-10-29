// Prepare a new private replay bundle. Preserve certificate bytes; never submit jobs.
// node docs/examples/cluster-replay.mjs NEW_BUNDLE KIND CERTIFICATE [KIND CERTIFICATE ...]
import {createHash} from 'node:crypto';
import {constants,openSync,closeSync,fstatSync,lstatSync,mkdirSync,readSync,rmSync,writeFileSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
const kinds=new Set(['batch-qc','motifs','cohort-features','cohort-compare','cohort-counts']);
export function prepareBundle(target,pairs) {
  if(!target||!pairs.length||pairs.length%2||pairs.length>128)throw new Error('Provide a new bundle and 1-64 KIND CERTIFICATE pairs');
  const inputs=[];let total=0;
  for(let i=0;i<pairs.length;i+=2){
    if(!kinds.has(pairs[i]))throw new Error('Unsupported replay kind: '+pairs[i]);
    const path=resolve(pairs[i+1]),stat=lstatSync(path);
    if(!stat.isFile()||stat.isSymbolicLink()||stat.size>256*1024*1024)throw new Error('Certificate must be a regular file at most 256 MiB');
    total+=stat.size;if(total>512*1024*1024)throw new Error('Bundle exceeds 512 MiB');
    inputs.push({kind:pairs[i],path,size:stat.size,stat});
  }
  const dir=resolve(target);mkdirSync(dir,{mode:0o700});
  try{
    const tasks=inputs.map(({kind,path,size,stat},i)=>{
      const id='task-'+String(i).padStart(3,'0'),file=id+'.certificate.clz';
      const fd=openSync(path,constants.O_RDONLY|(constants.O_NOFOLLOW??0));
      let raw;
      try{
        const opened=fstatSync(fd);
        if(!opened.isFile()||opened.ino!==stat.ino||opened.dev!==stat.dev||opened.size!==size)throw new Error('Certificate changed while preparing bundle');
        raw=Buffer.alloc(size);let offset=0;
        while(offset<size){const n=readSync(fd,raw,offset,size-offset,null);if(!n)throw new Error('Certificate shortened while preparing bundle');offset+=n;}
        const after=fstatSync(fd);
        if(readSync(fd,Buffer.alloc(1),0,1,null)||after.size!==size||after.mtimeMs!==opened.mtimeMs)throw new Error('Certificate changed while preparing bundle');
      }finally{closeSync(fd);}
      const sha256=createHash('sha256').update(raw).digest('hex');
      writeFileSync(join(dir,file),raw,{flag:'wx',mode:0o600});
      return {id,operation:'replay',kind,inputs:[{name:'evidence',file,size,sha256}]};
    });
    const recipe={format:'clonoloom-run-recipe',version:1,tasks};
    writeFileSync(join(dir,'replay.json'),JSON.stringify(recipe,null,2)+'\n',{flag:'wx',mode:0o600});
    writeFileSync(join(dir,'tasks.txt'),tasks.map(t=>t.id).join('\n')+'\n',{flag:'wx',mode:0o600});
    return {directory:dir,tasks:tasks.length,array:`0-${tasks.length-1}`};
  }catch(error){rmSync(dir,{recursive:true,force:true});throw error;}
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  const [target,...pairs]=process.argv.slice(2);
  const result=prepareBundle(target,pairs);
  console.log(`Prepared ${result.tasks} individually pinned replay tasks in ${result.directory}.\nReview kinds and provider terms, then explicitly select array ${result.array}. No job submitted.\nCertificate integrity and scientific replay are checked by the native worker, not this copy helper.`);
}

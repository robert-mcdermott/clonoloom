// Bounded, local JSONL candidate adapter. A trusted process, not an OS sandbox.
import {spawn} from 'node:child_process';

export const MAX_CALLS = 16;
// Node 22 supplies the original primitive token. Refuse rounded numbers before forwarding.
// 2^53 itself is exactly representable and is used only as the host-refusal probe.
export function parseMessage(line) {
  return JSON.parse(line,(_key,value,context)=>{
    if(typeof value==='number' && (!context?.source || !Number.isFinite(value) || JSON.stringify(value)!==context.source || (Number.isInteger(value)&&!Number.isSafeInteger(value)&&value!==2**53)))throw new Error('Noncanonical or unsafe JSON number; use exact bounded arguments');
    return value;
  });
}
export function commandAgent(argv, task, call, {timeout=120000, maxBytes=1024*1024}={}) {
  if (!Array.isArray(argv) || !argv.length || argv.some(s=>typeof s!=='string'||s.includes('\0')) || !argv[0]) throw new Error('command must be a nonempty JSON argv array');
  return new Promise((resolve,reject)=>{
    const child=spawn(argv[0],argv.slice(1),{stdio:['pipe','pipe','pipe'],shell:false});
    let buffer='',bytes=0,stderrBytes=0,busy=false,answer,answered=false,ended=false,calls=0;
    const ids=new Set();
    const finish=(error)=>{
      if(ended)return; ended=true; clearTimeout(timer);
      child.stdin.destroy(); if(child.exitCode===null)child.kill('SIGKILL');
      if(error)reject(error);else resolve(answer);
    };
    const timer=setTimeout(()=>finish(new Error('Candidate deadline exceeded; no automatic retry')),timeout);
    const send=value=>{if(!ended)child.stdin.write(JSON.stringify(value)+'\n');};
    child.on('error',finish); child.stdin.on('error',error=>finish(error));
    child.stderr.on('data',chunk=>{stderrBytes+=chunk.length;if(stderrBytes>maxBytes)finish(new Error('Candidate stderr limit exceeded'));});
    child.stdout.setEncoding('utf8');
    child.stdout.on('data',chunk=>{
      bytes+=Buffer.byteLength(chunk);if(bytes>maxBytes)return finish(new Error('Candidate output limit exceeded'));
      buffer+=chunk.toString('utf8');
      while(buffer.includes('\n')&&!ended){
        const end=buffer.indexOf('\n'),line=buffer.slice(0,end);buffer=buffer.slice(end+1);
        try {
          const message=parseMessage(line);
          if(!message||Array.isArray(message)||typeof message!=='object')throw new Error('Expected a JSON object');
          if(answered||busy)throw new Error('Extra output or concurrent calls are not allowed');
          if(Object.keys(message).length===1&&Object.hasOwn(message,'answer')){
            answer=message.answer;answered=true;child.stdin.end();continue;
          }
          if(Object.keys(message).sort().join(',')!=='action,args,id'||!Number.isSafeInteger(message.id)||message.id<1||ids.has(message.id)||typeof message.action!=='string'||!message.args||Array.isArray(message.args)||typeof message.args!=='object')throw new Error('Invalid action message or duplicate ID');
          if(++calls>MAX_CALLS)throw new Error('Candidate call limit exceeded');
          ids.add(message.id);busy=true;
          Promise.resolve().then(()=>call(message.action,message.args)).then(response=>{busy=false;send({id:message.id,response});},finish);
        }catch(error){finish(error);}
      }
    });
    child.on('close',code=>{
      if(ended)return;
      if(code!==0||!answered||busy||buffer.trim())finish(new Error('Candidate ended without one complete answer and a clean exit'));
      else finish();
    });
    send(task);
  });
}

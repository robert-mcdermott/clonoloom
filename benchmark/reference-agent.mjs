// Test-only scripted candidate for the JSONL adapter. This is not a model agent.
import {createInterface} from 'node:readline';
import {TASKS,reference} from './tasks.mjs';
const lines=createInterface({input:process.stdin});
let next=0,pending;
lines.on('line',async line=>{
  try{
    const message=JSON.parse(line);
    if(message.type==='task'){
      const task=TASKS.find(t=>t.id===message.id);
      const answer=await reference(task,(action,args)=>new Promise((resolve,reject)=>{
        const id=++next;pending={id,resolve,reject};process.stdout.write(JSON.stringify({id,action,args})+'\n');
      }));
      process.stdout.write(JSON.stringify({answer})+'\n');
    }else{
      if(!pending||message.id!==pending.id)throw new Error('Unexpected response');
      const previous=pending;pending=null;previous.resolve(message.response);
    }
  }catch(e){process.stderr.write(e.message+'\n');process.exitCode=1;lines.close();}
});

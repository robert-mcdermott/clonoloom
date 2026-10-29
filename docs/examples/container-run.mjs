// Print an explicit, pinned local OCI command. Never execute it or widen permissions.
// node docs/examples/container-run.mjs docker|podman IMAGE_ID INPUT_DIR RECIPE TASK OUTPUT_PARENT NEW_NAME
import {lstatSync,readFileSync,realpathSync,existsSync} from 'node:fs';
import {join,resolve,sep} from 'node:path';
import {pathToFileURL} from 'node:url';
const logical=/^[a-z][a-z0-9_-]{0,63}$/;
const named=s=>typeof s==='string'&&logical.test(s);
const portable=/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const filename=s=>typeof s==='string'&&portable.test(s)&&!s.endsWith('.');
export function containerCommand(engine,image,input,recipeName,taskID,output,name,{uid=process.getuid(),gid=process.getgid()}={}) {
  if(!['docker','podman'].includes(engine)||! /^(sha256:)?[a-f0-9]{64}$/.test(image))throw new Error('Select docker or podman and an already inspected local image ID');
  if(!Number.isInteger(uid)||uid<=0||!Number.isInteger(gid)||gid<0)throw new Error('Use a non-root numeric user with access to the mounts');
  if(!filename(recipeName)||!named(taskID)||!named(name))throw new Error('Use portable recipe filename and logical task/output names');
  const inputs=realpathSync(input),outputs=realpathSync(output);
  if(!lstatSync(inputs).isDirectory()||!lstatSync(outputs).isDirectory()||/[\r\n,]/.test(inputs+outputs))throw new Error('Use existing mount directories without commas or newlines');
  if(inputs===outputs||inputs.startsWith(outputs+sep)||outputs.startsWith(inputs+sep))throw new Error('Input and writable output mounts must be disjoint');
  if(existsSync(join(outputs,name)))throw new Error('Output must not exist');
  const recipe=join(inputs,recipeName),stat=lstatSync(recipe);
  if(!stat.isFile()||stat.isSymbolicLink()||stat.size>8*1024*1024)throw new Error('Recipe must be a regular file at most 8 MiB');
  const document=JSON.parse(readFileSync(recipe,'utf8'));
  if(document.format!=='clonoloom-run-recipe'||document.version!==1||!Array.isArray(document.tasks)||document.tasks.length>64)throw new Error('Unsupported recipe');
  const selected=document.tasks.filter(t=>t.id===taskID);
  if(selected.length!==1)throw new Error('Select exactly one declared task');
  const task=selected[0];
  if(!Array.isArray(task.inputs)||!task.inputs.length||task.inputs.length>256)throw new Error('Invalid input list');
  const names=new Set();
  for(const i of task.inputs){
    if(!named(i.name)||names.has(i.name)||!filename(i.file))throw new Error('Invalid or duplicate input binding');
    names.add(i.name);const file=lstatSync(join(inputs,i.file));
    if(!file.isFile()||file.isSymbolicLink())throw new Error('Bind regular input files only');
  }
  return [engine,'run','--rm','--pull=never','--network=none','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',
    '--user',`${uid}:${gid}`,'--pids-limit=128','--memory=1g','--cpus=2',...(engine==='podman'?['--read-only-tmpfs=false']:[]),
    '--mount',`type=bind,src=${inputs},dst=/inputs,ro`,'--mount',`type=bind,src=${outputs},dst=/output,rw`,
    '--tmpfs','/scratch:rw,noexec,nosuid,nodev,size=1g,mode=1777',image,'run','--recipe',`/inputs/${recipeName}`,'--task',taskID,
    ...task.inputs.flatMap(i=>['--input',`${i.name}=/inputs/${i.file}`]),'--out',`/output/${name}`];
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  const args=process.argv.slice(2);if(args.length!==7)throw new Error('Usage: container-run.mjs docker|podman IMAGE_ID INPUT_DIR RECIPE TASK OUTPUT_PARENT NEW_NAME');
  const quote=s=>`'${s.replaceAll("'","'\\''")}'`;
  console.log(containerCommand(...args).map(quote).join(' '));
}

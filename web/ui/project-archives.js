import { h, clear, downloadBlob, formatBytes } from './dom.js';
import { parseExactProject, archiveCounts } from '../lib/project-archives.js';

export function createProjectArchives(app) {
  const {store} = app;
  let busy=false, error='', record=null, controller=null, input=null, inspected=false;
  const notify=()=>store.notify(['archives']);
  const fail=(message)=>{error=message;notify();throw new Error(message);};
  async function json(response) { const value=await response.json();if(!response.ok)throw new Error(value.error ?? 'Archive operation failed.');return value; }
  async function perform(work) {
    if(busy)throw new Error('Wait for the archive operation.');
    if(app.jobs?.running().length || store.state.busy?.size)throw new Error('Wait for current work before transferring a project.');
    const project=store.project,same=store.sameProject();const valid=()=>same()&&project===store.project;
    busy=true;error='';controller=new AbortController();notify();
    try{return await work(project,valid,controller.signal);}
    catch(e){error=e.name==='AbortError'?'Archive operation cancelled. Inspect the library before retrying an import; verified unreferenced files may remain.':e.message;throw e;}
    finally{busy=false;controller=null;notify();}
  }
  const upload=(source,extra,signal)=> typeof source==='string'
    ? {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path:source,...extra}),signal}
    : {method:'POST',headers:{'Content-Type':'application/zip'},body:source,signal};
  const api={
    state:()=>({busy,error,record,inspected}), cancel:()=>controller?.abort(),
    async exportArchive({includeSources=false,ownSources=false,accept=false}={}) {
      return perform(async(project,valid,signal)=>{
        const body={project,includeSources,ownSources,accept};
        record=await json(await fetch('api/project-archives/export?preview=1',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body),signal}));inspected=false;notify();
        if(!valid())throw new Error('The project changed during archive preparation.');
        const response=await fetch('api/project-archives/export',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...body,expected:record.fingerprint}),signal});
        if(!response.ok)await json(response);
        const blob=await response.blob();if(!valid())throw new Error('The project changed; export again.');
        downloadBlob(blob,`${project.id}.clz`);return record;
      });
    },
    async inspect(source,{expected=''}={}) {
      if(!source || (typeof source!=='string'&&source.size>520*1024*1024))fail('Choose a project archive no larger than 520 MiB.');
      return perform(async(_project,valid,signal)=>{
        const value=await json(await fetch(`api/project-archives/inspect?expect=${encodeURIComponent(expected)}`,upload(source,{expected,accept:false},signal)));
        if(!valid())throw new Error('The project changed during inspection.');
        record=value.archive;input=source;inspected=true;notify();return value;
      });
    },
    async importArchive(source=input,{expected=record?.fingerprint ?? '',accept=false}={}) {
      if(!source||!expected)fail('Inspect an archive and supply its fingerprint first.');
      return perform(async(_project,valid,signal)=>{
        const checked=await json(await fetch(`api/project-archives/inspect?expect=${encodeURIComponent(expected)}`,upload(source,{expected,accept},signal)));
        parseExactProject(checked.projectJSON);
        if(!valid())throw new Error('The project changed before import.');
        await app.saveNow();if(!valid())throw new Error('The project changed before import.');
        const value=await json(await fetch(`api/project-archives/import?expect=${encodeURIComponent(expected)}&accept=${accept?'1':'0'}`,upload(source,{expected,accept},signal)));
        if(!valid())throw new Error('The project changed; the imported project is saved in the library but was not opened.');
        const doc=parseExactProject(value.projectJSON);record=value.archive;inspected=false;
        await app.loadProject(doc);return {archive:value.archive,project:{id:doc.id,name:doc.name},recomputed:false};
      });
    },
  };
  return api;
}
export function mountProjectArchives(app,container) {
  const root=h('section.card.project-archives-view');container.append(root);
  let includeSources=false,ownSources=false,accept=false,expected='';
  const run=(work)=>work().catch(()=>{});
  function render(){
    const {busy,error,record,inspected}=app.archives.state();clear(root);
    const check=(label,value,set)=>h('label',h('input',{type:'checkbox',checked:value,disabled:busy,onchange:e=>set(e.target.checked)}),' ',label);
    root.append(h('h2','Portable project archives'),h('p','Keep the original project, history, result files and checkpoints in one checked .clz archive. Source files are optional. Import verifies every member and never overwrites an existing project or conflicting evidence.'),
      check('Include available source files where permitted',includeSources,v=>{includeSources=v;}),
      check('I have authority to include sources without recorded provider licenses',ownSources,v=>{ownSources=v;}),
      check('I acknowledge private-use restrictions on captured provider evidence',accept,v=>{accept=v;}),
      h('p.muted','Restricted provider sources are always omitted. Captured results can still contain identifiers, sequences and counts. Archives are not de-identified or a redistribution license.'),
      h('div.btn-row',h('button.btn.small',{type:'button',disabled:busy,onclick:()=>run(()=>app.archives.exportArchive({includeSources,ownSources,accept}))},'Export project archive'),busy?h('button.btn.small',{type:'button',onclick:()=>app.archives.cancel()},'Cancel archive operation'):null),
      h('label.field',h('span','Expected archive fingerprint (optional for inspection)'),h('input.input',{value:expected,disabled:busy,oninput:e=>{expected=e.target.value.trim();}})),
      h('label.field',h('span','Inspect project archive'),h('input',{type:'file',accept:'.clz,.zip',disabled:busy,onchange:e=>{const file=e.target.files[0];if(file)run(()=>app.archives.inspect(file,{expected}));}})),
      h('button.btn.small',{type:'button',disabled:busy||!inspected,onclick:()=>run(()=>app.archives.importArchive(undefined,{expected:record.fingerprint,accept}))},'Import inspected project'));
    if(error)root.append(h('p.callout.danger',{role:'alert'},error));
    if(record){
      root.append(h('h3','Verified archive inventory'),h('p',`${record.files.length} files · ${formatBytes(record.files.reduce((sum,f)=>sum+f.size,0))}. Scientific recomputation: not performed.`),
        h('p',Object.entries(archiveCounts(record)).map(([state,n])=>`${n} ${state}`).join(' · ')||'No original source bindings.'),
        h('p.mono.certificate-fingerprint',`SHA-256: ${record.fingerprint}`),h('p',record.limits));
      const rows=h('tbody');for(const s of record.sources)rows.append(h('tr',h('td',s.name),h('td',s.status),h('td',s.reason)));
      if(record.sources.length)root.append(h('details',h('summary',`All ${record.sources.length} source bindings`),h('div',{style:{overflowX:'auto'}},h('table.data',h('thead',h('tr',h('th','Source'),h('th','Status'),h('th','Reason'))),rows))));
    }
    if(app.store.project.portableArchive)root.append(h('p',h('a',{href:`api/project-archives/${app.store.project.portableArchive}/original`,download:'original-project.json'},'Download original project evidence')));
  }
  render();return{update(topics){if(topics.has('archives')||topics.has('project'))render();},destroy(){root.remove();}};
}

// Named, local review of host-authorized calls. The host's grants are never widened here.
import { h } from './dom.js';
import { showDialog } from './overlays.js';

export function automationWindow(app) {
  let active = null;
  let audit = [];
  app.drawer.addTab('automation', {label:'Automation', render:() => h('div',
    h('p', 'Automation audit — this host session, last 500 events. Names are supplied by callers, not authenticated identities. Arguments are represented by SHA-256, not copied here.'),
    h('ol.history-list', ...audit.slice().reverse().map((r) => h('li',h('time',{title:r.time},new Date(r.time).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'})),h('span',`${r.client}: ${r.action} — ${r.state}`))))) });
  async function refresh() {
    const response = await fetch(`api/remote/audit?session=${encodeURIComponent(app.info.session)}`);
    if (response.ok) {audit = (await response.json()).events;app.drawer.render();}
  }
  void refresh().catch(()=>{});
  function cancel(id) { if (active && (id === undefined || active.id === id)) active.cancel(); }
  async function run(event, action) {
    if (active) throw Object.assign(new Error('Another automation call is still settling; no new action was run.'), {code:'busy'});
    const hasEffects = event.effects.some((effect) => effect !== 'read');
    const busy = () => hasEffects && (app.jobs.running().length > 0 || app.store.state.busy.size > 0);
    if (busy()) throw Object.assign(new Error('Wait for current jobs and data imports before submitting this automation call.'), {code:'busy'});
    const project = app.store.project, current = app.store.sameProject();
    const previousJobs=new Set(app.jobs.running().map((job)=>job.id));
    let cancelled = false, close = null, finish = null, release = null;
    const stop = () => { cancelled = true; finish?.(); for(const job of app.jobs.running())if(!previousJobs.has(job.id))void app.jobs.cancel(job.id).catch(()=>{}); if(!finish)close?.(); };
    active = {id:event.id,cancel:stop};
    try {
      if(event.effects.some((effect)=>effect!=='read')) {
        const elements=[...document.body.children].filter((el)=>el.id!=='overlay-root'&&!el.inert);
        for(const el of elements)el.inert=true;
        const keys=(event)=>{if((event.metaKey||event.ctrlKey)&&event.key!=='Enter'){event.preventDefault();event.stopImmediatePropagation();}};
        document.addEventListener('keydown',keys,true);
        release=()=>{document.removeEventListener('keydown',keys,true);for(const el of elements)el.inert=false;};
      }
      if (event.review) {
        const accepted = await new Promise((resolve) => {
          const dialog = showDialog({title:'Review automation proposal',width:'wide',
            content:h('div',h('p',`${event.client} proposes “${event.title ?? event.action}” in ${project.name}.`),
              h('p',`Declared effects: ${event.effects.join(', ')}.`),
              h('p','Accept runs this call once against this project state. Undo restores project edits; downloaded files, cache entries and completed worker outputs are not erased.'),
              h('pre',{tabIndex:0,style:'max-height:35vh;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere'},JSON.stringify(event.args??{},null,2))),
            buttons:[{label:'Reject',value:false},{label:'Accept proposal',primary:true,value:true}],onClose:resolve});
          dialog.dialog.dataset.automationProposal=event.id;
          close=()=>dialog.close(false);
        });
        close=null;
        if(cancelled)throw Object.assign(new Error('Automation call cancelled before acceptance.'),{code:'cancelled'});
        if(!accepted)throw Object.assign(new Error('Proposal rejected; no action was run.'),{code:'rejected'});
        if(!current() || app.store.project!==project)throw Object.assign(new Error('The project changed while this proposal was open. Submit a fresh proposal.'),{code:'stale-project'});
      }
      if(cancelled)throw Object.assign(new Error('Automation cancelled.'),{code:'cancelled'});
      if (busy()) throw Object.assign(new Error('A job or data import started during review; submit again when it finishes.'), {code:'busy'});
      const confirmation = await fetch(`api/remote/accepted/${encodeURIComponent(event.id)}`,{method:'POST',headers:{'X-ClonoLoom-Session':app.info.session}});
      if(!confirmation.ok || cancelled)throw Object.assign(new Error('The caller is no longer waiting; no action was run.'),{code:'cancelled'});
      if(event.review && (!current()||app.store.project!==project))throw Object.assign(new Error('The project changed before execution.'),{code:'stale-project'});
      // A modal plus inert application surface prevents incidental user edits from inheriting
      // the caller's history label. Earlier asynchronous attachments are invalidated by the store.
      if(event.effects.some((effect)=>effect!=='read')) {
        finish=app.store.beginAutomation(event.review ? event.client : null, {readOnly:!event.review,invalidate:event.review && event.effects.some((effect)=>effect==='compute'||effect==='files')});
        await Promise.resolve();
        const dialog=showDialog({title:'Automation running',dismissable:false,content:h('div',h('p',`${event.client}: ${event.action}`),h('p','Cancellation prevents later project attachments. Work already written or downloaded may remain. The window stays busy until this call settles.')),buttons:[{label:'Cancel action',onClick:()=>{stop();return false;}}]});
        close=()=>dialog.close();
      }
      const result=await action(event.args??{});
      if(cancelled)throw Object.assign(new Error('Automation cancelled. Earlier side effects may remain; review History and use Undo for project edits.'),{code:'cancelled'});
      return result;
    } catch(error) {
      if(cancelled)throw Object.assign(new Error('Automation cancelled; earlier side effects may remain.'),{code:'cancelled'});
      throw error;
    } finally {
      close?.(); release?.(); finish?.(true); active=null; void refresh().catch(()=>{});
    }
  }
  return {run,cancel,refresh};
}

// Standalone runtime, appended after the checked-in hash/checker/geometry modules.
// No imports, network calls, external fonts or application/library dependency.
(() => {
  const executingSource=document.currentScript.textContent;
  const $=(id)=>document.getElementById(id), status=$('status');
  const decode=(text)=>Uint8Array.from(atob(text.trim()),(c)=>c.charCodeAt(0));
  const text=(bytes)=>new TextDecoder('utf-8',{fatal:true}).decode(bytes);
  const digest=async(bytes)=>{const h=new hash.SHA256();h.update(bytes);return h.hex();};
  const message=(node,s)=>{const p=document.createElement('p');p.textContent=s;node.append(p);};
  function figures(node,raw,kind) {
    // Scientific values are already stored. JS only lays out the existing plots.
    const doc=JSON.parse(text(raw)), analysis=doc.analysis;
    const unsafe=(v)=>typeof v==='number' ? !Number.isFinite(v)||(Number.isInteger(v)&&!Number.isSafeInteger(v)) : v&&typeof v==='object' ? Object.values(v).some(unsafe) : false;
    if (unsafe(analysis)) {message(node,'Figures unavailable: unsafe browser numeric geometry. Exact tables and downloads are retained.');return;}
    const plots=[];
    if(kind==='batch-qc') {
      if(analysis.metrics.length>64 || analysis.groups.reduce((n,g)=>n+g.observations.length,0)>20000) throw new Error('Figure geometry exceeds report limits.');
      for(const metric of analysis.metrics) plots.push(batch.batchQCPlot(analysis,metric.id));
    } else if(kind==='cohort-compare') {
      if(analysis.comparisons.length>32) throw new Error('Figure geometry exceeds report limits.');
      for(const comparison of analysis.comparisons) plots.push(cohort.cohortPoints(comparison,analysis),cohort.cohortEffect(comparison,analysis));
    } else if(kind==='cohort-counts') {
      if(analysis.rows.length>20000) throw new Error('Figure geometry exceeds report limits.');
      plots.push(counts.countMA(analysis));
    } else {message(node,'This result is presented as complete recorded tables. No additional figure or new calculation is generated.');return;}
    for(const plot of plots) {
      const figure=document.createElement('figure'), caption=document.createElement('figcaption');
      if(plot.svg) figure.innerHTML=plot.svg;
      caption.textContent=plot.description;figure.append(caption);node.append(figure);
    }
    message(node,'Figures use the application’s existing geometry; axis ticks may be rounded for readability. Exact values and all unavailable observations remain in the tables and original JSON. No new statistics are computed.');
  }
  async function check() {
    $('check').disabled=true;status.textContent='Checking embedded bytes…';status.dataset.verdict='checking';$('report').replaceChildren();$('downloads').replaceChildren();
    try {
      const record=$('report-record'), members=[...document.querySelectorAll('script[id^="report-file-"]')];
      if(members.length>19 || members.some((n,i)=>n.id!==`report-file-${i}`)) throw new Error('Unexpected report members.');
      const files=members.map((n)=>decode(n.textContent));
      const checked=await checkReport(decode(record.textContent),files,record.dataset.sha256,$('expected').value.trim(),digest);
      $('fingerprint').textContent=checked.fingerprint;
      const byPath=new Map(checked.record.files.map((f,i)=>[f.path,files[i]]));
      const geometry=executingSource.split('/* report geometry start */\n')[1]?.split('/* report geometry end */')[0];
      if(geometry!==text(byPath.get('geometry.js')) || document.querySelectorAll('style').length!==1 || document.querySelector('style').textContent!==text(byPath.get('report.css'))) throw new Error('Report geometry or styles differ from the inventory.');
      const template=document.createElement('template');template.innerHTML=text(byPath.get('report.html'));
      // Generated content has only text/tables. Refuse executable or navigational
      // markup even if someone replaced both content and internal hashes.
      const allowed=new Set(['H1','H2','H3','P','SECTION','DETAILS','SUMMARY','PRE','DIV','TABLE','CAPTION','THEAD','TBODY','TR','TH','TD']);
      for(const el of template.content.querySelectorAll('*')) {
        if(!allowed.has(el.tagName) || [...el.attributes].some((a)=>!['class','tabindex','scope','data-result','data-kind'].includes(a.name))) throw new Error('Unsupported report content markup.');
      }
      $('report').append(template.content);
      for(const node of $('report').querySelectorAll('.figures')) {
        const entry=checked.record.results.find((r)=>r.result===node.dataset.result&&r.kind===node.dataset.kind);
        if(!entry) throw new Error('Figure is not bound to a selected result.');
        try {figures(node,byPath.get(entry.result),entry.kind);} catch {message(node,'Figures unavailable for this saved geometry. Exact tables and downloads are retained; this is not a scientific validation verdict.');}
      }
      const heading=document.createElement('h2');heading.textContent='Verified embedded downloads';$('downloads').append(heading);
      for(const f of checked.record.files) {
        if(['report.html','geometry.js','report.css'].includes(f.path)) continue;
        const button=document.createElement('button');button.type='button';button.textContent=`Download ${f.path}`;
        button.onclick=()=>{const url=URL.createObjectURL(new Blob([byPath.get(f.path)],{type:f.path.endsWith('.clz')?'application/zip':'application/json'})),a=document.createElement('a');a.href=url;a.download=f.path;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);};$('downloads').append(button);
      }
      for(const r of checked.record.results) message($('downloads'),`Native scientific replay: clonoloom verify --replay --expect ${r.fingerprint} ${r.certificate}`);
      status.textContent=`Integrity match — ${checked.filesChecked} embedded files (${checked.bytesChecked} bytes). Scientific recomputation: not performed. ${$('expected').value.trim()?'Independently supplied fingerprint matched.':'No independent fingerprint supplied.'}`;status.dataset.verdict='integrity-match';
    } catch(e) { $('report').replaceChildren();$('downloads').replaceChildren();status.textContent=`Check failed or unavailable: ${e.message}`;status.dataset.verdict='differs'; }
    finally {$('check').disabled=false;}
  }
  $('check').onclick=check;
  $('theme').onclick=()=>{const dark=document.documentElement.dataset.theme==='dark'||(!document.documentElement.dataset.theme&&matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.dataset.theme=dark?'light':'dark';};
  check();
})();

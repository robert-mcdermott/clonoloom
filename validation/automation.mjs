// Test-only UI driver: explicit grants still come from the host command line, and each
// proposed edit is accepted by clicking the same visible button as a researcher.
export async function acceptAutomation(fetchResult, browser) {
  let done=false;
  const result=Promise.resolve(fetchResult).finally(()=>{done=true;});
  // Attach rejection immediately while the browser waits for a proposal.
  result.catch(()=>{});
  while(!done){
    await browser.eval(`(()=>{const proposal=document.querySelector('[data-automation-proposal]');const button=proposal?.querySelector('button.primary');if(button)button.click();return Boolean(button)})()`);
    await new Promise((resolve)=>setTimeout(resolve,40));
  }
  return result;
}

// Imports can finish before the jobs SSE snapshot catches up. Await observed idle
// before submitting another non-read call; never automatically retry a failed edit.
export async function waitForAutomationIdle(browser) {
  const deadline = Date.now() + 10 * 60 * 1000;
  while (Date.now() < deadline) {
    if (await browser.eval(`Boolean(window.clonoloom && window.clonoloom.jobs.running().length === 0 && window.clonoloom.store.state.busy.size === 0)`)) return;
    await new Promise((resolve) => setTimeout(resolve, 80));
  }
  throw new Error('Existing work did not settle before automation submission.');
}

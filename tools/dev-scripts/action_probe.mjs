import { chromium } from 'playwright';
const b = await chromium.launch();
const p = await (await b.newContext()).newPage();
await p.goto('http://127.0.0.1:5173',{waitUntil:'domcontentloaded'});
await p.waitForTimeout(4000);
const seen = {};
for (let i=0;i<200;i++){
  const acts = await p.evaluate(()=> (window.__pixiHandle?.getEntities()||[]).map(e=>e.current_action).filter(Boolean));
  for(const a of acts) seen[a]=(seen[a]||0)+1;
  await p.waitForTimeout(100);
}
console.log('ACTIONS:', JSON.stringify(seen));
await b.close();

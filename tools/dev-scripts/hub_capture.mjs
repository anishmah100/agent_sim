// Capture the busiest cluster of agents (the spawn hub) for a demo shot.
import { chromium } from 'playwright';
const F='http://127.0.0.1:5173', E='http://127.0.0.1:8080';
// Find the centroid of living agents to frame the action.
async function centroid(){
  const a=await fetch(E+'/api/v1/agents').then(r=>r.json()).catch(()=>({agents:[]}));
  const ps=(a.agents||[]).filter(x=>x.pos).map(x=>x.pos);
  if(!ps.length) return [760,860];
  const sx=ps.reduce((s,p)=>s+p[0],0)/ps.length, sy=ps.reduce((s,p)=>s+p[1],0)/ps.length;
  return [Math.round(sx),Math.round(sy)];
}
const b=await chromium.launch();
const p=await (await b.newContext({viewport:{width:1400,height:880},deviceScaleFactor:2})).newPage();
await p.goto(F,{waitUntil:'domcontentloaded'});
await p.waitForTimeout(2500);
const skip=p.getByTestId('onboarding-skip');
if(await skip.count()) await skip.first().click().catch(()=>{});
let ready=false; p.on('console',m=>{if(m.text().includes('character atlas loaded'))ready=true;});
for(let i=0;i<30&&!ready;i++) await p.waitForTimeout(400);
for(let i=0;i<3;i++){
  const c=await centroid();
  await p.evaluate(({x,y})=>{const h=window.__pixiHandle;h?.centerOn(x,y);h?.viewport?.setZoom?.(3.2,true);},{x:c[0],y:c[1]});
  await p.waitForTimeout(1200);
  await p.screenshot({path:`/tmp/capstone_${i}.png`}); console.log('saved /tmp/capstone_'+i+'.png @'+c);
  await p.waitForTimeout(1500);
}
await b.close();

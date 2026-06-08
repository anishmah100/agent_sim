import { chromium } from 'playwright';
const F='http://127.0.0.1:5173', E='http://127.0.0.1:8080';
async function centroid(){
  const a=await fetch(E+'/api/v1/agents').then(r=>r.json()).catch(()=>({agents:[]}));
  const ps=(a.agents||[]).filter(x=>x.pos&&!(x.pos[0]===0&&x.pos[1]===0)).map(x=>x.pos);
  if(!ps.length) return [764,864];
  ps.sort((u,v)=>u[0]-v[0]); const mx=ps[Math.floor(ps.length/2)][0];
  ps.sort((u,v)=>u[1]-v[1]); const my=ps[Math.floor(ps.length/2)][1];
  return [mx,my]; // median = where the crowd actually is
}
const b=await chromium.launch();
const p=await (await b.newContext({viewport:{width:1280,height:820},deviceScaleFactor:2})).newPage();
await p.goto(F,{waitUntil:'domcontentloaded'}); await p.waitForTimeout(2500);
const skip=p.getByTestId('onboarding-skip'); if(await skip.count()) await skip.first().click().catch(()=>{});
let ready=false; p.on('console',m=>{if(m.text().includes('character atlas loaded'))ready=true;});
for(let i=0;i<30&&!ready;i++) await p.waitForTimeout(400);
for(let i=0;i<3;i++){ const c=await centroid();
  await p.evaluate(({x,y})=>{const h=window.__pixiHandle;h?.centerOn(x,y);h?.viewport?.setZoom?.(3.4,true);},{x:c[0],y:c[1]});
  await p.waitForTimeout(1000); await p.screenshot({path:`/tmp/live_${i}.png`}); console.log('saved /tmp/live_'+i+'.png @'+c);
  await p.waitForTimeout(1200);
}
await b.close();

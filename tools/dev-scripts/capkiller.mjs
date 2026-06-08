import { chromium } from 'playwright';
const F='http://127.0.0.1:5173', E='http://127.0.0.1:8080';
async function pos(name){
  const a=await fetch(E+'/api/v1/agents').then(r=>r.json()).catch(()=>({agents:[]}));
  const k=(a.agents||[]).find(x=>x.persona_name===name);
  return k&&k.pos?k.pos:null;
}
const b=await chromium.launch();
const p=await (await b.newContext({viewport:{width:1100,height:760},deviceScaleFactor:2})).newPage();
await p.goto(F,{waitUntil:'domcontentloaded'}); await p.waitForTimeout(2500);
const skip=p.getByTestId('onboarding-skip'); if(await skip.count()) await skip.first().click().catch(()=>{});
let ready=false; p.on('console',m=>{if(m.text().includes('character atlas loaded'))ready=true;});
for(let i=0;i<30&&!ready;i++) await p.waitForTimeout(400);
for(let i=0;i<4;i++){
  const c=await pos('Hunter')||await pos('Forager0'); if(!c){console.log('no target');break;}
  await p.evaluate(({x,y})=>{const h=window.__pixiHandle;h?.centerOn(x,y);h?.viewport?.setZoom?.(5.5,true);},{x:c[0],y:c[1]});
  await p.waitForTimeout(700);
  await p.screenshot({path:`/tmp/kill_${i}.png`}); console.log('saved /tmp/kill_'+i+'.png @'+c);
  await p.waitForTimeout(800);
}
await b.close();

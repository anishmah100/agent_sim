import { chromium } from 'playwright';
const E='http://127.0.0.1:8080', F='http://127.0.0.1:5173';
async function cat(){
  const a=await fetch(E+'/api/v1/agents').then(r=>r.json());
  const c=(a.agents||[]).find(x=>x.persona_name==='Cat');
  return c&&c.pos?c.pos:null;
}
const b=await chromium.launch();
const p=await (await b.newContext({viewport:{width:1280,height:800},deviceScaleFactor:2})).newPage();
await p.goto(F,{waitUntil:'domcontentloaded'});
const skip=p.getByTestId('onboarding-skip'); await p.waitForTimeout(2500);
if(await skip.count()) await skip.first().click().catch(()=>{});
// wait for atlas
let ready=false; p.on('console',m=>{if(m.text().includes('character atlas loaded'))ready=true;});
for(let i=0;i<30&&!ready;i++) await p.waitForTimeout(400);
for(let i=0;i<5;i++){
  const c=await cat(); if(!c){console.log('no cat (mouse dead/ended)');break;}
  await p.evaluate(({x,y})=>{const h=window.__pixiHandle;h?.centerOn(x,y);h?.viewport?.setZoom?.(4.5,true);},{x:c[0],y:c[1]});
  await p.waitForTimeout(900);
  await p.screenshot({path:`/tmp/catmouse_${i}.png`}); console.log('saved /tmp/catmouse_'+i+'.png cat@'+c);
  await p.waitForTimeout(700);
}
await b.close();

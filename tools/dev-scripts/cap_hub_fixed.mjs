import { chromium } from 'playwright';
const F='http://127.0.0.1:5173';
const b=await chromium.launch();
const p=await (await b.newContext({viewport:{width:1280,height:820},deviceScaleFactor:2})).newPage();
await p.goto(F,{waitUntil:'domcontentloaded'}); await p.waitForTimeout(2500);
const skip=p.getByTestId('onboarding-skip'); if(await skip.count()) await skip.first().click().catch(()=>{});
let ready=false; p.on('console',m=>{if(m.text().includes('character atlas loaded'))ready=true;});
for(let i=0;i<30&&!ready;i++) await p.waitForTimeout(400);
for(let i=0;i<3;i++){
  await p.evaluate(()=>{const h=window.__pixiHandle;h?.centerOn(762,862);h?.viewport?.setZoom?.(3.0,true);});
  await p.waitForTimeout(900);
  await p.screenshot({path:`/tmp/big_${i}.png`}); console.log('saved /tmp/big_'+i+'.png');
  await p.waitForTimeout(1200);
}
await b.close();

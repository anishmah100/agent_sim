import { chromium } from 'playwright';
const b = await chromium.launch();
const p = await (await b.newContext({viewport:{width:1280,height:800},deviceScaleFactor:2})).newPage();
await p.goto('http://127.0.0.1:5173',{waitUntil:'domcontentloaded'});
await p.waitForTimeout(4500);
// poll for a wounded agent over ~30s; center + shoot when found.
for (let i=0;i<60;i++){
  const w = await p.evaluate(()=>{
    const es = window.__pixiHandle?.getEntities()||[];
    const hurt = es.filter(e=>{const h=e.extras?.hp,m=e.extras?.max_hp; return typeof h==='number'&&typeof m==='number'&&h<m&&h>0;})
      .sort((a,b)=>(a.extras.hp/a.extras.max_hp)-(b.extras.hp/b.extras.max_hp));
    return hurt.length?{id:hurt[0].entity_id,pos:hurt[0].pos,hp:hurt[0].extras.hp,max:hurt[0].extras.max_hp,n:hurt.length}:null;
  });
  if (w){
    console.log('wounded found:', JSON.stringify(w));
    await p.evaluate(({x,y})=>{const h=window.__pixiHandle;h?.centerOn(x,y);h?.viewport?.setZoom?.(4.5,true);}, {x:w.pos[0],y:w.pos[1]});
    await p.waitForTimeout(700);
    await p.screenshot({path:'/tmp/wounded.png'});
    console.log('saved /tmp/wounded.png');
    b.close(); process.exit(0);
  }
  await p.waitForTimeout(500);
}
console.log('no wounded agent seen in 30s');
await b.close();

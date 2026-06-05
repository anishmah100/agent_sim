import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
page.on('console', m => console.log(`[browser ${m.type()}]`, m.text()));
page.on('pageerror', e => console.log(`[browser PAGEERROR]`, e.message));

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 8000));
try { await page.getByTestId('onboarding-skip').click({ timeout: 1500 }); } catch {}
await new Promise(r => setTimeout(r, 1500));

const dump = await page.evaluate(() => {
  const vp = window.__viewport;
  if (!vp) return {err: 'no viewport'};
  const b = vp.getVisibleBounds();
  return {
    worldWidth: vp.worldWidth, worldHeight: vp.worldHeight,
    centerX: vp.center.x, centerY: vp.center.y,
    bx: b.x, by: b.y, bw: b.width, bh: b.height,
    scale: vp.scale.x,
  };
});
console.log('default state:', JSON.stringify(dump, null, 2));

await page.evaluate(({wx, wy}) => {
  window.__viewport.moveCenter(wx, wy);
}, {wx: 800*16, wy: 900*16});
await new Promise(r => setTimeout(r, 1500));

const dump2 = await page.evaluate(() => {
  const vp = window.__viewport;
  const b = vp.getVisibleBounds();
  return {
    centerX: vp.center.x, centerY: vp.center.y,
    bx: b.x, by: b.y, bw: b.width, bh: b.height,
    scale: vp.scale.x,
    tilemapChildren: document.querySelector('canvas')?.parentNode?.childNodes?.length || -1,
  };
});
console.log('after move to crossroads:', JSON.stringify(dump2, null, 2));

await page.screenshot({ path: '/tmp/eldoria_dbg_crossroads.png' });
console.log('debug shot saved');
await browser.close();

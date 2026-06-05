import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
const errs = [];
page.on('console', m => { if (m.type() === 'error') errs.push(m.text()); });
page.on('pageerror', e => errs.push(`PAGEERROR: ${e.message}`));

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 12000));

await page.evaluate(() => {
  localStorage.setItem('agent_sim:onboarding_seen_v1', '1');
});
await page.reload();
await new Promise(r => setTimeout(r, 10000));

// Wide zoom — show the whole continent.
await page.evaluate(() => {
  const vp = window.__viewport;
  if (!vp) return;
  vp.moveCenter(vp.worldWidth / 2, vp.worldHeight / 2);
  vp.setZoom(0.5, true);
});
await new Promise(r => setTimeout(r, 4000)); // let sprite budget catch up
await page.screenshot({ path: '/tmp/eldoria_wide.png' });
console.log('wide shot');

// Default close zoom at world center
await page.evaluate(() => {
  const vp = window.__viewport;
  vp.moveCenter(vp.worldWidth / 2, vp.worldHeight / 2);
  vp.setZoom(4.0, true);
});
await new Promise(r => setTimeout(r, 1500));
await page.screenshot({ path: '/tmp/eldoria_default.png' });
console.log('default shot');

const REGIONS = [
  { name: 'pinewood', tx: 320, ty: 450 },
  { name: 'crossroads', tx: 800, ty: 900 },
  { name: 'saltport', tx: 1180, ty: 700 },
  { name: 'dunehallow', tx: 700, ty: 1320 },
  { name: 'frostvale', tx: 200, ty: 280 },
  { name: 'greenfield', tx: 600, ty: 760 },
  { name: 'lake', tx: 1000, ty: 1000 },
  { name: 'coast_north', tx: 1230, ty: 400 },  // sample real coastline
  { name: 'forest_interior', tx: 380, ty: 500 }, // sample real forest
];

for (const r of REGIONS) {
  await page.evaluate(({tx, ty}) => {
    window.__viewport.moveCenter(tx * 16, ty * 16);
    window.__viewport.setZoom(2.5, true);
  }, r);
  await new Promise(r => setTimeout(r, 1500));
  await page.screenshot({ path: `/tmp/eldoria_${r.name}.png` });
  console.log(`${r.name} shot`);
}

console.log('errors:', errs.slice(0, 8));
await browser.close();

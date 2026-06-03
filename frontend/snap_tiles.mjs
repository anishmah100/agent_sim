import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
const errs = [];
p.on('pageerror', e => errs.push(`PAGE: ${e.message}`));
p.on('console', m => { const t = m.text(); if (t.includes('atlas') || t.includes('tile')) console.log('PAGE:', t); if (m.type() === 'error') errs.push(`[err] ${m.text()}`); });
await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 6500));
await p.screenshot({ path: '/tmp/agent_sim_real_tiles.png' });
await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  h.viewport.setZoom(3.0, true);
  h.viewport.moveCenter(15*16, 9*16);
});
await new Promise(r => setTimeout(r, 1500));
await p.screenshot({ path: '/tmp/agent_sim_real_tiles_zoom.png' });
console.log('errs:', errs.length ? errs.join('\n') : '(none)');
await b.close();

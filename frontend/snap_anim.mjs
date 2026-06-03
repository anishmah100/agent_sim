import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
const errs = [];
p.on('pageerror', e => errs.push(`PAGE ERR: ${e.message}`));
p.on('console', m => { if (m.type() === 'error') errs.push(`[err] ${m.text()}`); });
await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 4500));

// Zoom WAY in on test_npc_2 and snap 4 frames as it walks.
const frames = [];
for (let i = 0; i < 8; i++) {
  await p.evaluate(() => {
    const h = globalThis.__pixiHandle;
    const e = h.getEntities().find(x => x.entity_id === 'test_npc_2');
    h.viewport.setZoom(8.0, true);
    h.viewport.moveCenter(e.pos[0] * 16 + 8, e.pos[1] * 16 + 8);
  });
  await new Promise(r => setTimeout(r, 200));
  await p.screenshot({ path: `/tmp/agent_sim_anim_${i}.png` });
}
console.log('errs:', errs.length ? errs.join('\n') : '(none)');
await b.close();

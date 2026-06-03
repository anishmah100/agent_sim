import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
const errs = [];
p.on('pageerror', e => errs.push(`PAGE: ${e.message}`));
p.on('console', m => { if (m.type() === 'error') errs.push(`[err] ${m.text()}`); });
await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 5000));

const state = await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  const layer = h.viewport.children.find(c => c.label === 'entities');
  const ents = layer?.children.filter(c => c.label?.startsWith('entity:')) ?? [];
  const result = ents.map(c => {
    const sprite = c.children.find(ch => ch.constructor.name === '_AnimatedSprite');
    return {
      id: c.label,
      tex: sprite?.texture ? [sprite.texture.width, sprite.texture.height] : null,
    };
  });
  return result;
});
console.log('entity textures (each should be ~328 wide):', JSON.stringify(state));

await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  const e = h.getEntities().find(x => x.entity_id === 'test_npc_2');
  h.viewport.setZoom(8.0, true);
  h.viewport.moveCenter(e.pos[0] * 16 + 8, e.pos[1] * 16 + 8);
});
await new Promise(r => setTimeout(r, 600));
await p.screenshot({ path: '/tmp/agent_sim_final_anim.png' });
console.log('errs:', errs.length ? errs.join('\n') : '(none)');
await b.close();

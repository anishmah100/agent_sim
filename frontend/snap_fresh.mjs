import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 4500));

const state = await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  const layer = h.viewport.children.find(c => c.label === 'entities');
  const entityContainers = layer?.children.filter(c => c.label?.startsWith('entity:'));
  return {
    layerChildCount: layer?.children.length,
    entityCount: entityContainers?.length,
    entityIds: entityContainers?.map(c => c.label),
  };
});
console.log('state:', JSON.stringify(state));

await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  const e = h.getEntities().find(x => x.entity_id === 'test_npc_2');
  h.viewport.setZoom(6.0, true);
  h.viewport.moveCenter(e.pos[0] * 16 + 8, e.pos[1] * 16 + 8);
});
await new Promise(r => setTimeout(r, 800));
await p.screenshot({ path: '/tmp/agent_sim_fresh.png' });
await b.close();

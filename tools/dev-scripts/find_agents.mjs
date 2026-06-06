// find_agents.mjs — visual locator for live Qwen agents.
//
// Given a list of entity IDs (from the engine's connect log), opens
// the UI, centers on each one, zooms in, and screenshots. Sends the
// user 3 framed views so they don't have to pan around 1500×1500.
//
// Usage:
//   node find_agents.mjs npc_lakeshore_3 npc_aspendell_3 npc_saltport_5

import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';
const TILE_SIZE_PX = 16;

const ids = process.argv.slice(2);
if (!ids.length) {
  console.error('usage: node find_agents.mjs <entity_id> [entity_id ...]');
  process.exit(2);
}

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
await page.goto(FRONTEND);
await page.waitForFunction(() => window.__viewport, { timeout: 20000 });
await page.evaluate(() => localStorage.setItem('agent_sim:onboarding_seen_v1', '1'));
await page.reload();
await page.waitForFunction(() => window.__viewport, { timeout: 20000 });
// Give the viewer WS a moment to deliver an entity snapshot.
await new Promise(r => setTimeout(r, 3000));

for (const id of ids) {
  // Read the entity's current position from the live render.
  const pos = await page.evaluate((eid) => {
    // The Pixi entities layer keeps a getEntities() through the handle
    // but isn't exposed on window. Read from __pixi via heuristic.
    const vp = window.__viewport;
    if (!vp) return null;
    // Climb the scene graph looking for entity sprites with `.entity`
    // metadata. Fallback: hit the engine API.
    return null;
  }, id);

  // Engine fallback: hit /api/v1/world/info won't give us per-entity pos.
  // Use the inspector handler which DOES expose pos via mental_state if
  // we've inspected it, or scrape the WS snapshot via a viewer probe.
  // Simplest: use the viewer broadcast via fetch on a snapshot endpoint
  // — but there isn't one. So we read it from the agent's stdout log we
  // tail externally. For this script, accept positions from CLI as
  // fallback if --pos arg is given, otherwise just click-pan via
  // window.__viewport using a known coordinate set the caller supplied
  // via environment.
  //
  // Pragmatic path: use Playwright's evaluation of the entity layer
  // children. Pixi exposes `entities` indirectly via the live
  // viewport scene graph; sprites carry their entity id in `name`.
  const live = await page.evaluate((eid) => {
    const vp = window.__viewport;
    const wantLabel = `entity:${eid}`;
    // Each entity wrap stores its world coords directly in wrap.x/y
    // (tile * 16, see Entity.applyPos). No parent-walk math needed.
    function find(node) {
      if (node.label === wantLabel) return { x: node.x, y: node.y };
      for (const c of node.children ?? []) {
        const hit = find(c);
        if (hit) return hit;
      }
      return null;
    }
    return find(vp);
  }, id);

  if (!live) {
    console.warn(`[${id}] not found in scene graph — skipping`);
    continue;
  }

  await page.evaluate(({ x, y }) => {
    const vp = window.__viewport;
    vp.moveCenter(x, y);
    vp.setZoom(3.5, true);
  }, live);
  await new Promise(r => setTimeout(r, 800));
  const tileX = Math.floor(live.x / TILE_SIZE_PX);
  const tileY = Math.floor(live.y / TILE_SIZE_PX);
  const out = `/tmp/agent_${id}.png`;
  await page.screenshot({ path: out });
  console.log(`[${id}] @ (${tileX},${tileY}) → ${out}`);
}

await browser.close();
console.log('done');

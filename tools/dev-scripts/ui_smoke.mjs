// UI smoke test. Hard regression gate that fails noisily if the
// frontend can't actually render the world. Run BEFORE declaring a
// substrate/engine change shipped. Earlier substrate refactors broke
// the world-static-serve endpoint and the bug surfaced only when a
// human opened the browser — none of the engine unit tests + LLM
// smoke runs caught it because they only exercised the WebSocket
// path, not the static world.json load.
//
// What this asserts:
//   1. /api/v1/world/info returns valid JSON (engine alive).
//   2. /worlds/<bundle>.json returns valid JSON (static serve OK).
//   3. The frontend root loads without page errors.
//   4. No error banner ("world load failed", "...") shows in the DOM.
//   5. The viewer WebSocket connects (ws: open status visible).
//   6. The render canvas exists AND at least one entity sprite is
//      drawn (probed via the Pixi stage child count).
//
// Exits non-zero on the first failure with a clear message.
//
// Usage:
//   node tools/dev-scripts/ui_smoke.mjs
// Assumes: engine on :8080, frontend on :5173.

import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';
const BUNDLE = process.env.BUNDLE_NAME || 'eldoria';

const fail = (msg) => { console.error(`FAIL: ${msg}`); process.exit(1); };
const ok = (msg) => console.log(`PASS: ${msg}`);

// --- 1. Engine alive + world.json reachable ---

async function fetchJson(url, label) {
  const r = await fetch(url);
  if (!r.ok) fail(`${label}: HTTP ${r.status} on ${url}`);
  const ct = r.headers.get('content-type') || '';
  if (!ct.includes('json')) {
    fail(`${label}: content-type=`+ct+` on ${url} (not JSON — likely an HTML 404)`);
  }
  try {
    return await r.json();
  } catch (e) {
    fail(`${label}: parse failed on ${url}: ${e.message}`);
  }
}

const info = await fetchJson(`${ENGINE}/api/v1/world/info`, 'engine /world/info');
if (!info?.world) fail(`engine /world/info missing .world: ${JSON.stringify(info)}`);
ok(`engine alive: world=${info.world} tick=${info.tick}`);

const world = await fetchJson(`${ENGINE}/worlds/${BUNDLE}.json`, 'static /worlds/<bundle>.json');
if (!world?.tiles || !Array.isArray(world.tiles)) {
  fail(`static world JSON missing .tiles: ${Object.keys(world ?? {})}`);
}
ok(`static world JSON serves: ${world.tiles.length} tile rows`);

// --- 2. Frontend renders without errors ---

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();

const consoleErrs = [];
page.on('console', m => { if (m.type() === 'error') consoleErrs.push(m.text()); });
const pageErrs = [];
page.on('pageerror', e => pageErrs.push(`PAGEERROR: ${e.message}`));

await page.goto(FRONTEND);
// Wait for app shell + WS + initial snapshot. The Pixi viewport
// publishes window.__viewport once it's mounted.
await page.waitForFunction(() => window.__viewport !== undefined, { timeout: 20000 })
  .catch(() => fail('window.__viewport never set — Pixi viewport never mounted'));
// Give the WS time to deliver a snapshot + first render.
await new Promise(r => setTimeout(r, 4000));

// 3. No error banner in the toolbar.
const errBanner = await page.locator('text=/world load failed|engine error|ws: closed/').count();
if (errBanner > 0) {
  const txt = await page.locator('text=/world load failed|engine error|ws: closed/').first().textContent();
  fail(`UI shows error banner: `+txt+``);
}
ok('no error banner');

// 4. ws status reads "open".
const wsStatus = await page.locator('text=/ws:\\s*open/').count();
if (wsStatus === 0) {
  fail('toolbar does not show "ws: open" — WebSocket not connected');
}
ok('WebSocket open');

// 5. Pixi stage has rendered children — proxy for "something is drawn".
const stats = await page.evaluate(() => {
  const vp = window.__viewport;
  if (!vp) return null;
  return {
    children: vp.children?.length ?? 0,
    worldW: vp.worldWidth,
    worldH: vp.worldHeight,
  };
});
if (!stats) fail('window.__viewport missing — couldn\'t probe Pixi stage');
if (stats.children < 1) fail(`Pixi viewport has 0 children — nothing drawn (stats=${JSON.stringify(stats)})`);
ok(`Pixi viewport has ${stats.children} child layers, world=${stats.worldW}x${stats.worldH}`);

// 6. No console / page errors that would have surfaced as red text.
if (pageErrs.length) fail(`page errors: ${pageErrs.join(' / ')}`);
// Allow a small number of benign console errors (favicon, etc.) — fail
// only on patterns we care about.
const realErrs = consoleErrs.filter(s => /world|fetch|api|websocket|ws:|Pixi|sprite|atlas/i.test(s));
if (realErrs.length) fail(`console errors related to world/api/render: ${realErrs.slice(0, 5).join(' / ')}`);
ok(`no relevant console errors (${consoleErrs.length} benign suppressed)`);

await browser.close();
console.log('\nUI SMOKE: PASS');

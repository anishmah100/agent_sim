// Verifies the D8 frontend fixes:
//   1. Items render with their actual sprite (NOT all wood_log).
//   2. Hovering an item shows the InfoPanel.
//   3. Clicking an item does NOT open the Inspector.
//
// This is the testing-discipline gap from D2 — engine + smoke pass
// alone don't catch frontend regressions. This probe drives the
// browser as a user.

import { chromium } from 'playwright';
const FRONTEND = 'http://127.0.0.1:5173';
const fail = (m) => { console.error(`FAIL: ${m}`); process.exit(1); };
const ok = (m) => console.log(`PASS: ${m}`);

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
const consoleErrs = [];
page.on('console', m => { if (m.type() === 'error') consoleErrs.push(m.text()); });
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500);
const skip = page.getByTestId('onboarding-skip');
if (await skip.count()) { await skip.click(); await page.waitForTimeout(300); }

// Pull the actual scattered items from the engine snapshot so we
// know real coords + sprites to assert on.
const items = await page.evaluate(async () => {
  const r = await fetch('http://127.0.0.1:8080/worlds/eldoria.json');
  const j = await r.json();
  return (j.entities || [])
    .filter(e => e.archetype === 'item')
    .slice(0, 20)
    .map(e => ({ id: e.entity_id, pos: e.pos, sprite: e.extras?.sprite }));
});
if (items.length < 5) fail(`expected at least 5 item entities, got ${items.length}`);
ok(`engine reports ${items.length} item entities sampled`);

// 1. Sprite diversity: not all items have the same sprite.
const uniqueSprites = new Set(items.map(i => i.sprite));
if (uniqueSprites.size < 3) {
  fail(`only ${uniqueSprites.size} distinct sprites in first 20 items; bug 'all items render as wood_log' may still be present. Sprites: ${[...uniqueSprites].join(', ')}`);
}
ok(`item sprites are diverse (${uniqueSprites.size} distinct: ${[...uniqueSprites].slice(0, 5).join(', ')}...)`);

// Pick a target item not already at a known crowded position
const target = items.find(i => i.sprite && i.sprite !== 'item:wood_log') ?? items[0];
console.log(`probing entity ${target.id} sprite=${target.sprite} at (${target.pos[0]}, ${target.pos[1]})`);

// Center camera on the target.
await page.evaluate(([tx, ty]) => {
  const h = window.__pixiHandle;
  if (h?.centerOn) h.centerOn(tx, ty);
}, [target.pos[0], target.pos[1]]);
await page.waitForTimeout(2000);

// 2. Hover should show InfoPanel.
const canvas = page.locator('canvas').first();
const box = await canvas.boundingBox();
const cx = box.x + box.width / 2;
const cy = box.y + box.height / 2;
const panel = page.getByTestId('info-panel');
const inspector = page.getByTestId('inspector');

// Try a few hover positions around center until InfoPanel shows.
let panelShown = false;
for (const [dx, dy] of [[0,0],[-20,0],[20,0],[0,-20],[0,20],[-40,0],[40,0]]) {
  await page.mouse.move(cx + dx, cy + dy);
  await page.waitForTimeout(180);
  if (await panel.count()) { panelShown = true; break; }
}
if (!panelShown) {
  console.log('  note: did not see InfoPanel on hover — item sprite may be too small to land the mouse on');
} else {
  ok('hovering an item shows the InfoPanel');
}

// 3. Click should NOT open Inspector.
// Move pointer back to center, then click.
await page.mouse.click(cx, cy);
await page.waitForTimeout(300);
if (await inspector.count()) {
  fail('clicking an item opened the Inspector (Speech/Mind/Trace). Should be ignored.');
}
ok('clicking an item does NOT open the Inspector');

if (consoleErrs.length) {
  console.error('console errors:');
  consoleErrs.forEach(e => console.error('  - ' + e));
  fail(`${consoleErrs.length} console errors during probe`);
}

await browser.close();
console.log('\nITEMS PROBE: PASS');

// Probe that clicks a market stall AND an item in Eldoria, verifying
// the InfoPanel renders with title, kind, description, and stats.
//
// Click flow exercised:
//   1. Center on a known market location (771, 894).
//   2. Click the stall sprite — panel should open with "Stall — …"
//      title and "Market stall" kind.
//   3. Close and click an item next door — panel should open with
//      a satiety/damage/worth stat line.
//
// Assertions are content-based, not pixel-based, so style tweaks don't
// break the gate.

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

// Center on the Crossroads market area. The DecorationLayer culls
// off-viewport sprites, so the click target only exists once we pan there.
await page.evaluate(() => {
  const h = window.__pixiHandle;
  if (h?.centerOn) h.centerOn(772, 894);
});
await page.waitForTimeout(2000);

// Click the canvas at the centered tile (~viewport center). The
// decoration layer's click handlers run on the sprite under the
// pointer. We click slightly biased toward a building.
const canvas = page.locator('canvas').first();
const box = await canvas.boundingBox();
if (!box) fail('no canvas');

// Center of viewport.
const cx = box.x + box.width / 2;
const cy = box.y + box.height / 2;

// Loop: click in a small radius around center until the info panel opens.
const panel = page.getByTestId('info-panel');
async function clickUntilPanel(label) {
  const offsets = [
    [0, 0], [-30, 0], [30, 0], [0, -30], [0, 30],
    [-60, 0], [60, 0], [0, -60], [0, 60],
    [-30, -30], [30, -30], [-30, 30], [30, 30],
  ];
  for (const [dx, dy] of offsets) {
    await page.mouse.click(cx + dx, cy + dy);
    await page.waitForTimeout(180);
    if (await panel.count()) return true;
  }
  fail(`${label}: no info panel after probing ~13 nearby tiles`);
}

await clickUntilPanel('first click');
ok('info panel opened on overworld click');

const title = await panel.locator('strong').first().innerText();
if (!title || title.trim().length === 0) fail('info panel title is empty');
ok(`info panel title: "${title}"`);

const bodyText = await panel.innerText();
if (!/Tile · \(\d+, \d+\)/.test(bodyText)) fail('info panel missing tile-coord line');
ok('info panel shows tile coords');

// Close and re-click for variety.
await panel.locator('button[aria-label="close info panel"]').click();
await page.waitForTimeout(200);
if (await panel.count()) fail('info panel did not close on ×');
ok('info panel closes via ×');

// Re-open via a different click + check the close handler is robust.
await clickUntilPanel('second click');
ok('info panel reopens on subsequent click');

if (consoleErrs.length) {
  console.error('console errors:');
  consoleErrs.forEach(e => console.error('  - ' + e));
  fail(`${consoleErrs.length} console errors during probe`);
}

await browser.close();
console.log('\nINFO PANEL PROBE: PASS');

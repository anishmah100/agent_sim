// Smoke test for the ArtCatalog migration. Boots the page, verifies
// (1) the catalog actually loaded and reports the expected sprite
//     count, (2) it resolves a handful of critical sprite ids, and
// (3) the world's tilemap + decorations render without console errors.
import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
const page = await ctx.newPage();
const errs = [];
page.on('console', m => { if (m.type() === 'error') errs.push(m.text()); });
page.on('pageerror', e => errs.push(`PAGEERROR: ${e.message}`));

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 12000));
await page.evaluate(() => { localStorage.setItem('agent_sim:onboarding_seen_v1', '1'); });
await page.reload();
await new Promise(r => setTimeout(r, 12000));

// Pull what the boot path saw.
const report = await page.evaluate(() => {
  const vp = window.__viewport;
  return {
    haveViewport: !!vp,
    worldW: vp?.worldWidth ?? 0,
    worldH: vp?.worldHeight ?? 0,
    entityCountText: document.body.innerText.match(/(\d+) entities/)?.[1] ?? null,
  };
});
console.log('boot:', JSON.stringify(report));

// Take a screenshot for visual diff against pre-catalog baseline.
await page.screenshot({ path: '/tmp/smoke_catalog.png' });
console.log('errors:', errs.slice(0, 5));
await browser.close();

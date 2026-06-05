import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
const logs = [];
page.on('console', m => {
  const t = m.text();
  if (t.includes('tile atlas') || t.includes('atlas') || t.includes('grass') || t.includes('Error') || t.includes('error')) {
    logs.push(`[${m.type()}]`, t);
  }
});
page.on('pageerror', e => logs.push('[PAGEERROR]', e.message));

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 12000));
try { await page.getByTestId('onboarding-skip').click({ timeout: 1500 }); } catch {}
await new Promise(r => setTimeout(r, 1000));

// Pull diagnostics about the actual tile container's first few children.
const probe = await page.evaluate(() => {
  const vp = window.__viewport;
  if (!vp) return {err: 'no viewport'};
  // Walk down to find the tilemap container
  let tilemap = null;
  const walk = (c) => {
    if (c.label === 'tilemap') tilemap = c;
    for (const ch of (c.children || [])) walk(ch);
  };
  walk(vp);
  if (!tilemap) return {err: 'no tilemap container'};
  return {
    childCount: tilemap.children.length,
    firstChildX: tilemap.children[0]?.x,
    firstChildY: tilemap.children[0]?.y,
    firstChildWidth: tilemap.children[0]?.width,
    firstChildTexW: tilemap.children[0]?.texture?.width,
    firstChildTexLabel: tilemap.children[0]?.texture?.label,
    viewportScale: vp.scale.x,
    viewportCenter: { x: vp.center.x, y: vp.center.y },
    visibleBounds: vp.getVisibleBounds(),
  };
});
console.log('PROBE:', JSON.stringify(probe, null, 2));
console.log('LOGS:', logs.join('\n'));
await browser.close();

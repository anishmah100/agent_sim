import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
page.on('console', m => console.log(`[${m.type()}]`, m.text()));
page.on('pageerror', e => console.log('[PAGEERROR]', e.message));

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 14000));

const r = await page.evaluate(() => {
  const a = window.__tileAtlas;
  if (!a) return { err: 'no __tileAtlas on window' };
  // Check what kinds the atlas has registered
  const kinds = ['grass', 'dirt', 'path', 'water', 'stone', 'sand', 'wall', 'floor_wood', 'void'];
  const status = {};
  for (const k of kinds) {
    const tex = a.defaultFor(k);
    status[k] = { has: a.has(k), tex: tex ? { w: tex.width, h: tex.height } : null };
  }
  return { kinds: status, byNameCount: a.byName?.size ?? -1, defaultCount: a.defaultsByKind?.size ?? -1 };
});
console.log('ATLAS:', JSON.stringify(r, null, 2));
await browser.close();

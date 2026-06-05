import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
page.on('console', m => {
  const t = m.type();
  if (t === 'log' || t === 'error' || t === 'warning') console.log(`[${t}]`, m.text());
});
page.on('pageerror', e => console.log(`[PAGEERROR]`, e.message));

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 12000));
try { await page.getByTestId('onboarding-skip').click({ timeout: 1500 }); } catch {}
await new Promise(r => setTimeout(r, 1500));

const r = await page.evaluate(async () => {
  const r1 = await fetch('/art/manifests/overworld_tileset.json');
  const ok1 = r1.ok;
  const r2 = await fetch('/api/v1/world/info');
  const j2 = await r2.json();
  return { manifestOk: ok1, info: j2 };
});
console.log('atlas manifest fetched:', r);

await browser.close();

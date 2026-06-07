import { chromium } from 'playwright';
const FRONTEND = 'http://127.0.0.1:5173';
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1280, height: 800 } })).newPage();
const t0 = Date.now();
let loadedAt = null;
page.on('console', m => {
  const t = m.text();
  if (t.includes('character atlas loaded')) { loadedAt = Date.now() - t0; console.log(`[${loadedAt}ms] ${t}`); }
});
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
// wait up to 30s for the atlas-loaded log
for (let i = 0; i < 60 && loadedAt === null; i++) await page.waitForTimeout(500);
console.log('ATLAS_LOAD_MS=', loadedAt ?? 'NEVER (>30s)');
await browser.close();

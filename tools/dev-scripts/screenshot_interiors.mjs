import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
const page = await ctx.newPage();
const errs = [];
page.on('console', m => { if (m.type() === 'error') errs.push(m.text()); });
await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 5000));
try { await page.getByTestId('onboarding-skip').click({ timeout: 1500 }); } catch {}
await new Promise(r => setTimeout(r, 800));

const BUILDINGS = ['bld:000', 'bld:004', 'bld:blacksmith', 'bld:town_hall', 'bld:granary'];
const NAMES = {
  'bld:000': 'cottage', 'bld:004': 'tavern', 'bld:blacksmith': 'blacksmith',
  'bld:town_hall': 'town_hall', 'bld:granary': 'granary',
};

// Verify Esc closes
await page.evaluate(() => window.__interior?.show('bld:000'));
await new Promise(r => setTimeout(r, 600));
const visibleBefore = await page.evaluate(() => window.__interior?.container.visible);
await page.keyboard.press('Escape');
await new Promise(r => setTimeout(r, 350));
const visibleAfter = await page.evaluate(() => window.__interior?.container.visible);
console.log(`esc test: visible before=${visibleBefore} after=${visibleAfter}`);

for (const id of BUILDINGS) {
  await page.evaluate((sprite) => window.__interior?.show(sprite), id);
  await new Promise(r => setTimeout(r, 700));
  await page.screenshot({ path: `/tmp/interior_${NAMES[id]}.png` });
  await page.evaluate(() => window.__interior?.hide());
  await new Promise(r => setTimeout(r, 350));
}
console.log('done. console errors:', errs.slice(0, 8));
await browser.close();

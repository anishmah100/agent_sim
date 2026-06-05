// Comprehensive audit pass — sample many regions across the world at
// game zoom + wide zoom, plus interior shots for each enterable
// building type. Saves to /tmp/audit_*.png so I can scan for visual
// issues.
import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
page.on('console', m => {
  if (m.type() === 'error') console.log('[ERR]', m.text());
});
page.on('pageerror', e => console.log('[PAGEERROR]', e.message));

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 12000));
await page.evaluate(() => {
  localStorage.setItem('agent_sim:onboarding_seen_v1', '1');
});
await page.reload();
await new Promise(r => setTimeout(r, 12000));

// Wide overview.
await page.evaluate(() => {
  const vp = window.__viewport;
  vp.moveCenter(vp.worldWidth / 2, vp.worldHeight / 2);
  vp.setZoom(0.5, true);
});
await new Promise(r => setTimeout(r, 4000));
await page.screenshot({ path: '/tmp/audit_wide.png' });
console.log('audit_wide');

// Region grid — sample 4×3 = 12 regions across the continent.
const REGIONS = [];
for (let row = 0; row < 3; row++) {
  for (let col = 0; col < 4; col++) {
    const tx = 200 + col * 350;
    const ty = 250 + row * 450;
    REGIONS.push({ name: `grid_${row}_${col}`, tx, ty });
  }
}

// Plus all 25 named towns.
const TOWNS = [
  ['crossroads', 800, 900], ['pinewood', 320, 450], ['saltport', 1180, 700],
  ['greenfield', 600, 760], ['lakeshore', 870, 1090], ['ironkeep', 250, 200],
  ['sunmarsh', 750, 1230], ['frostvale', 180, 280], ['stonemoor', 350, 220],
  ['coldbrook', 130, 350], ['mountainfoot', 450, 350], ['cliffhaven', 1150, 380],
  ['aspendell', 240, 530], ['mossglen', 410, 530], ['riverbend', 680, 580],
  ['oakshade', 480, 680], ['greenrun', 720, 820], ['westgate', 380, 870],
  ['eastfall', 1000, 940], ['highmarsh', 880, 1010], ['marshton', 950, 1180],
  ['reedwater', 620, 1180], ['dunehallow', 700, 1330], ['sandholme', 920, 1380],
  ['driftwell', 450, 1320], ['saltwatch', 1230, 850], ['hawkspire', 1100, 580],
];

for (const r of REGIONS) {
  await page.evaluate(({tx, ty}) => {
    const vp = window.__viewport;
    vp.moveCenter(tx * 16, ty * 16);
    vp.setZoom(3.5, true);
  }, r);
  await new Promise(r => setTimeout(r, 1200));
  await page.screenshot({ path: `/tmp/audit_${r.name}.png` });
  console.log(`audit_${r.name}`);
}

for (const [name, tx, ty] of TOWNS) {
  await page.evaluate(({tx, ty}) => {
    const vp = window.__viewport;
    vp.moveCenter(tx * 16, ty * 16);
    vp.setZoom(3.0, true);
  }, {tx, ty});
  await new Promise(r => setTimeout(r, 1200));
  await page.screenshot({ path: `/tmp/audit_town_${name}.png` });
  console.log(`audit_town_${name}`);
}

// Interiors — open each enterable building once
const INTERIORS = ['bld:000', 'bld:001', 'bld:blacksmith', 'bld:town_hall', 'bld:granary'];
for (const sprite of INTERIORS) {
  await page.evaluate((s) => window.__interior?.show(s), sprite);
  await new Promise(r => setTimeout(r, 900));
  const safe = sprite.replace(/[:]/g, '_');
  await page.screenshot({ path: `/tmp/audit_interior_${safe}.png` });
  await page.evaluate(() => window.__interior?.hide());
  await new Promise(r => setTimeout(r, 400));
  console.log(`interior_${safe}`);
}

await browser.close();
console.log('done');

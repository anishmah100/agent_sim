// Capture composed hero + gallery images for the README.
// Saves PNGs into docs/images/. Run while a live experiment is going so
// agents + Society-Pulse relationship lines are on screen.
import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';
const OUT = process.argv[2] || 'docs/images';

async function frame() {
  // Center on the strongest live social edge; fall back to median.
  const [ar, sr] = await Promise.all([
    fetch(`${ENGINE}/api/v1/agents`).then(r => r.json()),
    fetch(`${ENGINE}/api/v1/social`).then(r => r.json()),
  ]);
  const alive = new Map();
  for (const a of (ar.agents ?? [])) {
    if (Array.isArray(a.pos) && (a.pos[0] > 0 || a.pos[1] > 0)) alive.set(a.entity_id, a.pos);
  }
  const live = (sr.edges ?? []).filter(e => alive.has(e.a) && alive.has(e.b))
    .map(e => ({ ...e, t: e.trade + e.whisper + e.pay + e.attack + e.contract }))
    .sort((a, b) => b.t - a.t);
  if (live.length) {
    const pa = alive.get(live[0].a), pb = alive.get(live[0].b);
    return { x: Math.round((pa[0] + pb[0]) / 2), y: Math.round((pa[1] + pb[1]) / 2) };
  }
  const xs = [...alive.values()].map(p => p[0]).sort((a, b) => a - b);
  const ys = [...alive.values()].map(p => p[1]).sort((a, b) => a - b);
  return { x: xs[xs.length >> 1] ?? 760, y: ys[ys.length >> 1] ?? 860 };
}

const c = await frame();
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 900 }, deviceScaleFactor: 2 });
const page = await ctx.newPage();
// Watch for the atlas-loaded log so we never screenshot the placeholder
// (rectangle) window before character sprites are ready.
let atlasReady = false;
page.on('console', (m) => { if (m.text().includes('character atlas loaded')) atlasReady = true; });
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
const skip = page.getByTestId('onboarding-skip');
await page.waitForTimeout(1500);
if (await skip.count()) await skip.first().click().catch(() => {});
// Block until the character atlas has loaded (up to 20s).
for (let i = 0; i < 40 && !atlasReady; i++) await page.waitForTimeout(500);
console.log('atlas ready:', atlasReady);
await page.waitForTimeout(800);

async function shot(name, x, y, zoom, settleMs = 1800) {
  await page.evaluate(({ x, y, zoom }) => {
    const h = window.__pixiHandle; h?.centerOn(x, y); h?.viewport?.setZoom?.(zoom, true);
  }, { x, y, zoom });
  await page.waitForTimeout(settleMs);
  // re-center in case agents drifted
  const cc = await frame().catch(() => ({ x, y }));
  await page.evaluate(({ x, y }) => window.__pixiHandle?.centerOn(x, y), cc);
  await page.waitForTimeout(400);
  const path = `${OUT}/${name}.png`;
  await page.screenshot({ path });
  console.log('saved', path);
}

// Hero: medium-wide, town + agents + relationship lines.
await shot('hero', c.x, c.y, 2.4);
// Gallery 1: tight Society-Pulse closeup on the strongest edge.
await shot('society_pulse', c.x, c.y, 3.6);
// Gallery 2: wide world vista (zoom out for the Eldoria landscape).
await shot('eldoria_vista', c.x, c.y, 1.4);

await browser.close();

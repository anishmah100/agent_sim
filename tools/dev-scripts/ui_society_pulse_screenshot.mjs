// Capture the Society-Pulse relationship overlay live over the world.
// Centers the viewport on the agent cluster, ensures the pulse toggle is
// on, and grabs a few frames so transient combat/economy FX are caught
// alongside the persistent relationship lines.
// Usage: node ui_society_pulse_screenshot.mjs [out_prefix]
import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';

async function clusterCenter() {
  const [ar, sr] = await Promise.all([
    fetch(`${ENGINE}/api/v1/agents`).then(r => r.json()),
    fetch(`${ENGINE}/api/v1/social`).then(r => r.json()),
  ]);
  // Alive agents with a real position (exclude origin = dead/respawning).
  const alive = new Map();
  for (const a of (ar.agents ?? [])) {
    if (Array.isArray(a.pos) && (a.pos[0] > 0 || a.pos[1] > 0)) alive.set(a.entity_id, a.pos);
  }
  if (!alive.size) throw new Error('no alive agents');
  // Prefer framing the strongest social edge whose BOTH endpoints are
  // alive — that guarantees a relationship line is in-frame.
  const liveEdges = (sr.edges ?? [])
    .filter(e => alive.has(e.a) && alive.has(e.b))
    .map(e => ({ ...e, total: e.trade + e.whisper + e.pay + e.attack + e.contract }))
    .sort((x, y) => y.total - x.total);
  if (liveEdges.length) {
    const pa = alive.get(liveEdges[0].a), pb = alive.get(liveEdges[0].b);
    return { x: Math.round((pa[0] + pb[0]) / 2), y: Math.round((pa[1] + pb[1]) / 2), n: alive.size, liveEdges: liveEdges.length };
  }
  // Fallback: median of alive positions (robust to outliers).
  const xs = [...alive.values()].map(p => p[0]).sort((a, b) => a - b);
  const ys = [...alive.values()].map(p => p[1]).sort((a, b) => a - b);
  const mid = a => a[Math.floor(a.length / 2)];
  return { x: mid(xs), y: mid(ys), n: alive.size, liveEdges: 0 };
}

const c = await clusterCenter();
console.log(`cluster center ≈ (${c.x}, ${c.y}) over ${c.n} agents`);

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();
page.on('console', (m) => { if (m.type() === 'error') console.log('PAGE-ERR:', m.text()); });
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(3000);

const skip = page.getByTestId('onboarding-skip');
if (await skip.count()) await skip.first().click().catch(() => {});

// Make sure the Society-Pulse overlay is enabled (it is by default, but
// click only if the button reports it off).
await page.waitForTimeout(500);

// Drive the camera through the dev-exposed pixi handle: center on the
// cluster and zoom in enough that relationship lines + sprites read.
await page.evaluate(({ x, y }) => {
  const h = window.__pixiHandle;
  if (h) { h.centerOn(x, y); h.viewport?.setZoom?.(3.4, true); }
}, c);

const prefix = process.argv[2] || '/tmp/society_pulse';
// A handful of frames spaced out to catch one-shot FX + line motion.
for (let i = 0; i < 4; i++) {
  await page.waitForTimeout(1500);
  // keep re-centering: agents move, we want the cluster framed.
  const cc = await clusterCenter().catch(() => c);
  await page.evaluate(({ x, y }) => { window.__pixiHandle?.centerOn(x, y); }, cc);
  await page.waitForTimeout(300);
  const out = `${prefix}_${i}.png`;
  await page.screenshot({ path: out, fullPage: false });
  console.log(`saved ${out}`);
}
await browser.close();

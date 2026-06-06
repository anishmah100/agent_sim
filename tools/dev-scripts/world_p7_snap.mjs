// Capture a screenshot of the current Eldoria world via the dev frontend.
import { chromium } from 'playwright';

const FRONTEND = 'http://127.0.0.1:5173';
const out = process.argv[2] || '/tmp/world_after_p7.png';

const b = await chromium.launch();
const c = await b.newContext({ viewport: { width: 1400, height: 900 } });
const p = await c.newPage();
await p.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await p.waitForTimeout(3500);
try {
  const skip = p.getByTestId('onboarding-skip');
  if (await skip.count()) await skip.first().click();
} catch {}
await p.waitForTimeout(800);
await p.screenshot({ path: out });
await b.close();
console.log(`saved ${out}`);

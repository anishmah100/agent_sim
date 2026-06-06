// Capture a screenshot of the inspector showing the new
// Relationships tab. Output: /tmp/relationships_tab.png
import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';

async function pickEntity() {
  const ar = await fetch(`${ENGINE}/api/v1/agents`).then(r => r.json());
  const a = ar.agents?.find(a => a.entity_id) ?? ar.agents?.[0];
  if (!a?.entity_id) throw new Error('no agents');
  return a;
}

const target = await pickEntity();
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500);

const skip = page.getByTestId('onboarding-skip');
if (await skip.count()) await skip.first().click();

await page.getByRole('button', { name: /^agents$/i }).first().click();
await page.waitForTimeout(500);
await page.locator('[data-testid^="agent-row-"]').first().click();
await page.getByTestId('inspector').first().waitFor({ state: 'visible' });
await page.getByTestId('tab-relationships').click();
await page.waitForTimeout(400);

const out = process.argv[2] || '/tmp/relationships_tab.png';
await page.screenshot({ path: out, fullPage: false });
console.log(`saved ${out}`);
await browser.close();

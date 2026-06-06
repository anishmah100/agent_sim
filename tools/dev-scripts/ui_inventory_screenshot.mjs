// Capture the Inspector with the new Inventory tab visible.
import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';

async function pickEntity() {
  const ar = await fetch(`${ENGINE}/api/v1/agents`).then((r) => r.json());
  const a = ar.agents?.find((x) => x.entity_id) ?? ar.agents?.[0];
  if (!a?.entity_id) throw new Error('no agents');
  return a;
}

const target = await pickEntity();
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500);

try {
  const skip = page.getByTestId('onboarding-skip');
  if (await skip.count()) await skip.first().click();
} catch {}

await page.getByRole('button', { name: /^agents$/i }).first().click();
await page.waitForTimeout(500);
await page.locator('[data-testid^="agent-row-"]').first().click();
await page.getByTestId('inspector').first().waitFor({ state: 'visible' });
await page.getByTestId('tab-inventory').click();
await page.waitForTimeout(400);

const out = process.argv[2] || '/tmp/inventory_tab.png';
await page.screenshot({ path: out, fullPage: false });
console.log(`saved ${out}`);
await browser.close();

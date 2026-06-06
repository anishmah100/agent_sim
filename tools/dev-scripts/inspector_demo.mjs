import { chromium } from 'playwright';
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
await page.goto('http://127.0.0.1:5173');
await page.waitForFunction(() => window.__viewport, { timeout: 20000 });
await page.evaluate(() => localStorage.setItem('agent_sim:onboarding_seen_v1', '1'));
await page.reload();
await page.waitForFunction(() => window.__viewport, { timeout: 20000 });
await new Promise(r => setTimeout(r, 3000));
// Open agents picker + click the guard row.
await page.locator('button:has-text("agents")').click();
await page.waitForSelector('[data-testid="agents-picker"]');
await new Promise(r => setTimeout(r, 2500));
const guardRow = page.locator('[data-testid^="agent-row-"]').filter({
  has: page.locator('strong:has-text("guard")'),
});
await guardRow.first().click();
await new Promise(r => setTimeout(r, 2500));
await page.screenshot({ path: '/tmp/inspector_speech.png' });
// Switch to Trace tab
const traceTab = page.locator('button:has-text("trace")');
if (await traceTab.count() > 0) {
  await traceTab.first().click();
  await new Promise(r => setTimeout(r, 1000));
  await page.screenshot({ path: '/tmp/inspector_trace.png' });
}
const mindTab = page.locator('button:has-text("mind")');
if (await mindTab.count() > 0) {
  await mindTab.first().click();
  await new Promise(r => setTimeout(r, 1000));
  await page.screenshot({ path: '/tmp/inspector_mind.png' });
}
await browser.close();

// Capture the agent-picker UI in action. Opens the page, clicks the
// "agents" toolbar button, and screenshots. Then clicks one of the
// agent rows and screenshots the focused view + inspector.
import { chromium } from 'playwright';
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
await page.goto('http://127.0.0.1:5173');
await page.waitForFunction(() => window.__viewport, { timeout: 20000 });
await page.evaluate(() => localStorage.setItem('agent_sim:onboarding_seen_v1', '1'));
await page.reload();
await page.waitForFunction(() => window.__viewport, { timeout: 20000 });
await new Promise(r => setTimeout(r, 3000));
await page.locator('button:has-text("agents")').click();
await page.waitForSelector('[data-testid="agents-picker"]', { timeout: 5000 });
await new Promise(r => setTimeout(r, 2500)); // let the first poll arrive
await page.screenshot({ path: '/tmp/picker_open.png' });
console.log('picker open shot');

// Click the "guard" row.
const guardRow = page.locator('[data-testid^="agent-row-"]').filter({
  has: page.locator('strong:has-text("guard")'),
});
const guardExists = await guardRow.count();
if (guardExists > 0) {
  await guardRow.first().click();
  await new Promise(r => setTimeout(r, 2000));
  await page.screenshot({ path: '/tmp/picker_focused.png' });
  console.log('focused shot');
}
await browser.close();

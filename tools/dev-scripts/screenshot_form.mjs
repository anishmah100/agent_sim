import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
const page = await ctx.newPage();
await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 3000));
// Dismiss onboarding if present
try {
  await page.getByTestId('onboarding-skip').click({ timeout: 1500 });
  await new Promise(r => setTimeout(r, 300));
} catch {}
// Open the join modal
await page.getByTestId('join-agent-button').click();
await page.waitForSelector('[role="dialog"]', { timeout: 4000 });
await new Promise(r => setTimeout(r, 600));
await page.screenshot({ path: '/tmp/persona_form.png', fullPage: false });
await browser.close();
console.log('saved /tmp/persona_form.png');

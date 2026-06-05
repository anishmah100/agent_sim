import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
const page = await ctx.newPage();
const consoleErrors = [];
page.on('console', m => { if (m.type() === 'error') consoleErrors.push(m.text()); });
await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 4500));   // wait for atlases
try { await page.getByTestId('onboarding-skip').click({ timeout: 1500 }); } catch {}
await new Promise(r => setTimeout(r, 800));
await page.screenshot({ path: '/tmp/world_loaded.png', fullPage: false });

await page.getByTestId('join-agent-button').click();
await page.waitForSelector('[role="dialog"]', { timeout: 4000 });
await new Promise(r => setTimeout(r, 600));
await page.screenshot({ path: '/tmp/persona_form.png', fullPage: false });

console.log('done. console errors:', consoleErrors.slice(0, 8));
await browser.close();

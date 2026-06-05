// Pan with EVERYTHING disabled — measure baseline cost.
import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 12000));
await page.evaluate(() => {
  localStorage.setItem('agent_sim:onboarding_seen_v1', '1');
});
await page.reload();
await new Promise(r => setTimeout(r, 10000));

const stats = await page.evaluate(async () => {
  const vp = window.__viewport;
  for (const c of vp.children) {
    if (c.label === 'tilemap' || c.label === 'decorations' || c.label === 'entities') {
      c.visible = false;
    }
  }
  await new Promise(r => setTimeout(r, 500));
  const start = { x: vp.center.x, y: vp.center.y };
  const frames = [];
  let last = performance.now();
  const onFrame = () => {
    const now = performance.now();
    frames.push(now - last);
    last = now;
    requestAnimationFrame(onFrame);
  };
  requestAnimationFrame(onFrame);
  await new Promise(r => setTimeout(r, 800));
  frames.length = 0;
  const startTime = performance.now();
  await new Promise(resolve => {
    const stepper = () => {
      const t = (performance.now() - startTime) / 3000;
      if (t >= 1) { resolve(); return; }
      vp.moveCenter(start.x + 600 * 16 * t, start.y + 100 * 16 * t);
      requestAnimationFrame(stepper);
    };
    requestAnimationFrame(stepper);
  });
  await new Promise(r => setTimeout(r, 300));
  const f = frames.slice().sort((a, b) => a - b);
  const sum = f.reduce((a, b) => a + b, 0);
  return {
    count: f.length, mean: +(sum / f.length).toFixed(2),
    p99: +f[Math.floor(f.length * 0.99)].toFixed(2),
    max: +f[f.length - 1].toFixed(2),
  };
});
console.log('BLANK PAN FRAME STATS (ms):', JSON.stringify(stats, null, 2));
await browser.close();

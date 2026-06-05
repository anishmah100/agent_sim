// Drag benchmark: programmatically simulate a long mouse drag across
// the camera and measure per-frame duration. Useful to verify pan
// smoothness without a human jiggling the mouse.
import { chromium } from 'playwright';
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
page.on('console', m => {
  const t = m.text();
  if (t.startsWith('[tilemap]') || t.startsWith('[deco]') || t.startsWith('[bench]')) {
    console.log(`[browser]`, t);
  }
});

await page.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 12000));
await page.evaluate(() => {
  localStorage.setItem('agent_sim:onboarding_seen_v1', '1');
});
await page.reload();
await new Promise(r => setTimeout(r, 10000));

// Run the drag + frame timing ENTIRELY inside the browser to avoid
// page.evaluate round-trip latency dominating the measurement.
const stats = await page.evaluate(async () => {
  const vp = window.__viewport;
  const start = { x: vp.center.x, y: vp.center.y };
  const frames = [];
  let lastFrame = performance.now();

  const onFrame = () => {
    const now = performance.now();
    frames.push(now - lastFrame);
    lastFrame = now;
    requestAnimationFrame(onFrame);
  };
  requestAnimationFrame(onFrame);

  // Warmup
  await new Promise(r => setTimeout(r, 1000));
  frames.length = 0;

  // Sweep camera over 4 seconds — covers ~800 tiles diagonally.
  const startTime = performance.now();
  const totalMs = 4000;
  const totalDx = 800 * 16;
  const totalDy = 200 * 16;
  await new Promise(resolve => {
    const stepper = () => {
      const t = (performance.now() - startTime) / totalMs;
      if (t >= 1) { resolve(); return; }
      vp.moveCenter(start.x + totalDx * t, start.y + totalDy * t);
      requestAnimationFrame(stepper);
    };
    requestAnimationFrame(stepper);
  });
  // Cool down
  await new Promise(r => setTimeout(r, 500));

  const f = frames.slice().sort((a, b) => a - b);
  const sum = f.reduce((a, b) => a + b, 0);
  return {
    count: f.length,
    mean: +(sum / f.length).toFixed(2),
    p50: +f[Math.floor(f.length * 0.50)].toFixed(2),
    p90: +f[Math.floor(f.length * 0.90)].toFixed(2),
    p99: +f[Math.floor(f.length * 0.99)].toFixed(2),
    max: +f[f.length - 1].toFixed(2),
    over33ms: f.filter(x => x > 33).length,
  };
});
console.log('FRAME STATS (ms):', JSON.stringify(stats, null, 2));
await browser.close();

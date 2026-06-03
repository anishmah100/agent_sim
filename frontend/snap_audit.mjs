// Snap the world at multiple pan positions for visual audit.
import { chromium } from "playwright";

const URL = "http://127.0.0.1:5173/";
const PANS = [
  { name: "plaza",    dx:   0, dy:  0 },   // default fit-to-world center
  { name: "bridge",   dx: -50, dy:-100 },
  { name: "pond",     dx: 200, dy:-150 },
  { name: "dirt",     dx:-200, dy:-200 },
  { name: "ne_corner",dx:-300, dy: 100 },
  { name: "nw_corner",dx: 300, dy: 100 },
];

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1100, height: 700 } });
const page = await ctx.newPage();
page.on("console", (msg) => msg.type() === "error" && console.log("[browser error]", msg.text()));
await page.goto(URL, { waitUntil: "domcontentloaded", timeout: 15_000 });
await page.waitForSelector("canvas", { timeout: 15_000 });
await page.waitForTimeout(3000);

const canvas = await page.$("canvas");
const box = await canvas.boundingBox();
const cx = box.x + box.width / 2;
const cy = box.y + box.height / 2;

for (const p of PANS) {
  // Always reset to fit-to-world before each pan.
  await page.click("text=fit to world").catch(() => {});
  await page.waitForTimeout(400);
  // Drag-pan in the requested direction.
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  await page.mouse.move(cx + p.dx, cy + p.dy, { steps: 10 });
  await page.mouse.up();
  await page.waitForTimeout(500);
  const out = `/tmp/audit_${p.name}.png`;
  await page.screenshot({ path: out, fullPage: false });
  console.log("snap →", out);
}
await browser.close();

// Snap focused on the dirt clearing (SE corner of Oak Hollow) at a
// medium zoom so the new dirt↔grass autotile edges read clearly.
import { chromium } from "playwright";

const URL = "http://127.0.0.1:5173/";
const OUT = "/tmp/agent_sim_snap_dirt.png";

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1100, height: 700 } });
const page = await ctx.newPage();
page.on("console", (msg) => console.log("[browser]", msg.type(), msg.text()));
await page.goto(URL, { waitUntil: "domcontentloaded", timeout: 15_000 });
await page.waitForSelector("canvas", { timeout: 15_000 });
await page.waitForTimeout(2500);

const canvas = await page.$("canvas");
const box = await canvas.boundingBox();
const cx = box.x + box.width / 2;
const cy = box.y + box.height / 2;
// Two outward zooms to back off from default 4x.
for (let i = 0; i < 4; i++) {
  await page.mouse.move(cx, cy);
  await page.mouse.wheel(0, 200);
  await page.waitForTimeout(50);
}
// Drag-pan SE toward the dirt clearing (~45, 33). The map fit-to-world
// center is (30, 20), so we pan right + down.
await page.mouse.move(cx, cy);
await page.mouse.down();
await page.mouse.move(cx - 200, cy - 150, { steps: 12 });
await page.mouse.up();
await page.waitForTimeout(400);
await page.screenshot({ path: OUT, fullPage: false });
await browser.close();
console.log("snap →", OUT);

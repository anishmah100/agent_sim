// Focus snap on the west cottage to verify entrance is clear of trees.
import { chromium } from "playwright";
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 900, height: 700 } });
const page = await ctx.newPage();
await page.goto("http://127.0.0.1:5173/", { waitUntil: "domcontentloaded" });
await page.waitForSelector("canvas");
await page.waitForTimeout(2500);
const canvas = await page.$("canvas");
const box = await canvas.boundingBox();
// Pan SW toward the west cottage at (12, 18).
await page.mouse.move(box.x + box.width/2, box.y + box.height/2);
await page.mouse.down();
await page.mouse.move(box.x + box.width/2 + 500, box.y + box.height/2 - 50, { steps: 10 });
await page.mouse.up();
await page.waitForTimeout(400);
await page.screenshot({ path: "/tmp/cottage_west.png" });
console.log("snap → /tmp/cottage_west.png");
await browser.close();

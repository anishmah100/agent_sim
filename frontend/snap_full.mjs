import { chromium } from "playwright";
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();
await page.goto("http://127.0.0.1:5173/", { waitUntil: "domcontentloaded" });
await page.waitForSelector("canvas");
await page.waitForTimeout(3000);
const canvas = await page.$("canvas");
const box = await canvas.boundingBox();
for (let i = 0; i < 14; i++) {
  await page.mouse.move(box.x + box.width/2, box.y + box.height/2);
  await page.mouse.wheel(0, 200);
  await page.waitForTimeout(50);
}
await page.waitForTimeout(800);
await page.screenshot({ path: "/tmp/agent_sim_full.png" });
await browser.close();
console.log("snap → /tmp/agent_sim_full.png");

// Demo the hover → click → enter interior flow.
import { chromium } from "playwright";

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1100, height: 700 } });
const page = await ctx.newPage();
page.on("console", (msg) => msg.type() !== "log" && console.log("[browser]", msg.type(), msg.text()));
await page.goto("http://127.0.0.1:5173/", { waitUntil: "domcontentloaded" });
await page.waitForSelector("canvas");
await page.waitForTimeout(3000);

const canvas = await page.$("canvas");
const box = await canvas.boundingBox();

// Click somewhere near the left cottage (the building at x=13-16, y=17-18).
// At default zoom 4 the viewport centers (30,20). Tile (15, 18) is
// 15-30 = -15 tiles left = -240 world-px = -960 device-px / 4 = -240
// scaled... well, approximate: drag-pan left first, then click.
await page.mouse.move(box.x + box.width/2, box.y + box.height/2);
await page.mouse.down();
await page.mouse.move(box.x + box.width/2 + 400, box.y + box.height/2 - 60, { steps: 10 });
await page.mouse.up();
await page.waitForTimeout(400);

await page.screenshot({ path: "/tmp/interior_before.png" });
console.log("snap → /tmp/interior_before.png");

// Hover roughly where a cottage should be; outline filter glows.
await page.mouse.move(box.x + box.width/2, box.y + box.height/2 - 30);
await page.waitForTimeout(300);
await page.screenshot({ path: "/tmp/interior_hover.png" });
console.log("snap → /tmp/interior_hover.png");

// Click to enter.
await page.mouse.click(box.x + box.width/2, box.y + box.height/2 - 30);
await page.waitForTimeout(500);
await page.screenshot({ path: "/tmp/interior_inside.png" });
console.log("snap → /tmp/interior_inside.png");

await browser.close();

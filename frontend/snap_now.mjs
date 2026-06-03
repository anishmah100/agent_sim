// Snap the live world to /tmp/agent_sim_snap.png.
import { chromium } from "playwright";

const URL = "http://127.0.0.1:5173/";
const OUT = "/tmp/agent_sim_snap.png";

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1100, height: 700 } });
const page = await ctx.newPage();
page.on("console", (msg) => console.log("[browser]", msg.type(), msg.text()));
await page.goto(URL, { waitUntil: "domcontentloaded", timeout: 15_000 });
await page.waitForSelector("canvas", { timeout: 15_000 });
await page.waitForTimeout(2500); // let tile + character atlases load
await page.screenshot({ path: OUT, fullPage: false });
await browser.close();
console.log("snap →", OUT);

// Visual regression runner.
//
// For each scenario in scenes.json, opens the dev server at the right
// view (pan + zoom), screenshots, diffs against the committed golden
// PNG. Exits non-zero on any pixel-level regression beyond the
// per-scene threshold.
//
// New scenes: edit scenes.json. New goldens: pass --update to write
// the current snap as the new golden.

import { chromium } from "playwright";
import { readFileSync, writeFileSync, existsSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";

const ROOT = resolve(import.meta.dirname);
const SCENES = JSON.parse(readFileSync(`${ROOT}/scenes.json`, "utf8"));
const URL = process.env.AGENT_SIM_URL ?? "http://127.0.0.1:5173/";
const UPDATE = process.argv.includes("--update");

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1100, height: 700 } });
const page = await ctx.newPage();

let failures = 0;
for (const scene of SCENES) {
  await page.goto(URL, { waitUntil: "domcontentloaded" });
  await page.waitForSelector("canvas");
  await page.waitForTimeout(2500);

  // Apply zoom + pan from the scene config.
  const canvas = await page.$("canvas");
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  for (let i = 0; i < (scene.zoom_out ?? 0); i++) {
    await page.mouse.move(cx, cy);
    await page.mouse.wheel(0, 200);
    await page.waitForTimeout(40);
  }
  if (scene.pan) {
    await page.mouse.move(cx, cy);
    await page.mouse.down();
    await page.mouse.move(cx + scene.pan[0], cy + scene.pan[1], { steps: 10 });
    await page.mouse.up();
    await page.waitForTimeout(400);
  }
  const snapPath = `${ROOT}/snaps/${scene.name}.png`;
  mkdirSync(dirname(snapPath), { recursive: true });
  await page.screenshot({ path: snapPath });

  const goldenPath = `${ROOT}/golden/${scene.name}.png`;
  if (UPDATE || !existsSync(goldenPath)) {
    mkdirSync(dirname(goldenPath), { recursive: true });
    writeFileSync(goldenPath, readFileSync(snapPath));
    console.log(`UPDATED golden: ${scene.name}`);
    continue;
  }
  const a = PNG.sync.read(readFileSync(goldenPath));
  const b = PNG.sync.read(readFileSync(snapPath));
  if (a.width !== b.width || a.height !== b.height) {
    console.error(`SIZE MISMATCH ${scene.name}: golden ${a.width}x${a.height} vs snap ${b.width}x${b.height}`);
    failures++;
    continue;
  }
  const diff = new PNG({ width: a.width, height: a.height });
  const px = pixelmatch(a.data, b.data, diff.data, a.width, a.height,
                        { threshold: 0.15 });
  const ratio = px / (a.width * a.height);
  const tolerance = scene.tolerance ?? 0.005;
  if (ratio > tolerance) {
    const diffPath = `${ROOT}/snaps/${scene.name}.diff.png`;
    writeFileSync(diffPath, PNG.sync.write(diff));
    console.error(`REGRESSION ${scene.name}: ${(ratio*100).toFixed(2)}% diff (tol ${(tolerance*100).toFixed(2)}%)`);
    failures++;
  } else {
    console.log(`ok ${scene.name}: ${(ratio*100).toFixed(3)}% diff`);
  }
}

await browser.close();
process.exit(failures > 0 ? 1 : 0);

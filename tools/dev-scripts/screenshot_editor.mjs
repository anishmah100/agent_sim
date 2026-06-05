// screenshot_editor.mjs — capture the Phase WORLD-3 visual editor panel.
//
// Three shots:
//   editor_baseline.png      — viewport with editor closed
//   editor_open.png          — editor panel open, palette + tools visible
//   editor_with_tile.png     — a palette glyph selected (highlighted)
//
// Requires the engine + vite dev server running. Run from repo root:
//   node tools/dev-scripts/screenshot_editor.mjs

import { chromium } from "playwright";
import { mkdir } from "fs/promises";
import { join } from "path";

const OUT_DIR = ".runlog/screenshots/wave4";
const FRONT = "http://127.0.0.1:5173";

async function main() {
  await mkdir(OUT_DIR, { recursive: true });
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  page.on("pageerror", (e) => console.warn("[pageerror]", e.message));
  page.on("console", (msg) => {
    if (msg.type() === "error") console.warn("[console.error]", msg.text());
  });

  await page.goto(FRONT, { waitUntil: "networkidle" });
  // Wait for the world to finish loading the tile legend.
  await page.waitForFunction(
    () => {
      const btn = document.querySelector('[data-testid="editor-toggle-button"]');
      return !!btn;
    },
    { timeout: 30000 },
  );
  await page.waitForTimeout(1500);

  // Shot 1 — baseline.
  await page.screenshot({ path: join(OUT_DIR, "editor_baseline.png"), fullPage: false });
  console.log("wrote", join(OUT_DIR, "editor_baseline.png"));

  // Shot 2 — open editor via the button.
  await page.click('[data-testid="editor-toggle-button"]');
  await page.waitForSelector('[data-testid="editor-panel"]');
  await page.waitForTimeout(500);
  await page.screenshot({ path: join(OUT_DIR, "editor_open.png"), fullPage: false });
  console.log("wrote", join(OUT_DIR, "editor_open.png"));

  // Shot 3 — select a palette glyph (the first one).
  const firstGlyph = await page.$('[data-testid^="palette-"]');
  if (firstGlyph) {
    await firstGlyph.click();
    await page.waitForTimeout(300);
    await page.screenshot({ path: join(OUT_DIR, "editor_with_tile.png"), fullPage: false });
    console.log("wrote", join(OUT_DIR, "editor_with_tile.png"));
  } else {
    console.warn("no palette glyph found; world legend may have failed to load");
  }

  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

// Visual proof for building enter/exit hide rule.
//
// Spawns a long-running python sidecar that registers an agent and
// holds its WS open. The probe then:
//   1. Pans the frontend viewport to the agent.
//   2. Captures /tmp/building_visual/before.png and asserts the
//      sprite is visible.
//   3. Tells the sidecar to send interact(enter).
//   4. Waits for the snapshot to propagate. Captures during_inside.png
//      and asserts the sprite is now hidden (Entity.ts:295 rule).
//   5. Tells the sidecar to send interact(exit). Captures after.png
//      and asserts the sprite reappeared.
//
// Run: ./agent_sim start (engine + frontend) then
//      node tools/dev-scripts/ui_building_visual_proof.mjs

import { chromium } from 'playwright';
import { spawn } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { once } from 'node:events';

const FRONTEND = 'http://127.0.0.1:5173';
const OUT_DIR = '/tmp/building_visual';
mkdirSync(OUT_DIR, { recursive: true });

const fail = (m) => { console.error(`FAIL: ${m}`); process.exit(1); };
const ok = (m) => console.log(`PASS: ${m}`);

// ---- Spawn the sidecar and parse its META line ---------------------

const py = spawn('python3', ['tools/dev-scripts/_building_sidecar.py'], {
  cwd: process.cwd(),
  env: { ...process.env, PYTHONPATH: 'sdk/python' },
  stdio: ['pipe', 'pipe', 'inherit'],
});
py.stdout.setEncoding('utf-8');

let outBuf = '';
function nextLine() {
  return new Promise((resolve) => {
    function check() {
      const i = outBuf.indexOf('\n');
      if (i >= 0) {
        const line = outBuf.slice(0, i);
        outBuf = outBuf.slice(i + 1);
        resolve(line);
      }
    }
    check();
    if (outBuf.indexOf('\n') < 0) {
      const handler = (chunk) => {
        outBuf += chunk;
        if (outBuf.indexOf('\n') >= 0) {
          py.stdout.off('data', handler);
          const i = outBuf.indexOf('\n');
          const line = outBuf.slice(0, i);
          outBuf = outBuf.slice(i + 1);
          resolve(line);
        }
      };
      py.stdout.on('data', handler);
    }
  });
}
function sendCmd(c) { py.stdin.write(c + '\n'); }
async function waitOK(verb) {
  const line = await nextLine();
  if (!line.startsWith('OK')) fail(`sidecar replied: ${line}`);
  if (verb && !line.includes(verb)) fail(`sidecar replied: ${line}; expected verb=${verb}`);
}

const meta = await nextLine();
if (!meta.startsWith('META ')) fail(`expected META first line, got: ${meta}`);
const { entity_id, pos: [agx, agy], building } = JSON.parse(meta.slice(5));
ok(`registered ${entity_id} at (${agx}, ${agy}); building=${building}`);

// ---- Drive the frontend ----------------------------------------------

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
const consoleErrs = [];
page.on('console', (m) => { if (m.type() === 'error') consoleErrs.push(m.text()); });
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500);
try {
  const skip = page.getByTestId('onboarding-skip');
  if (await skip.count()) await skip.first().click();
} catch {}

// Center the viewport on the probe agent.
await page.evaluate(({ x, y }) => {
  const v = window.__viewport;
  if (v?.moveCenter) v.moveCenter(x * 32, y * 32);
}, { x: agx, y: agy });
await page.waitForTimeout(2500);

async function spriteVisible() {
  return await page.evaluate((eid) => {
    const h = window.__pixiHandle;
    if (!h?.entitySpriteVisible) return null;
    return h.entitySpriteVisible(eid);
  }, entity_id);
}

let v = null;
for (let i = 0; i < 20; i++) {
  v = await spriteVisible();
  if (v === true) break;
  await page.waitForTimeout(500);
}
if (v !== true) fail(`probe agent sprite did not render; visible=${v}`);
await page.screenshot({ path: `${OUT_DIR}/before.png` });
ok(`before.png saved — sprite visible=${v}`);

// Enter the building.
sendCmd('enter');
await waitOK('enter');

let after = true;
for (let i = 0; i < 20; i++) {
  await page.waitForTimeout(500);
  after = await spriteVisible();
  if (after === false) break;
}
await page.screenshot({ path: `${OUT_DIR}/during_inside.png` });
if (after !== false) fail(`sprite did NOT hide after enter (visible=${after})`);
ok(`during_inside.png saved — sprite hidden`);

// Exit.
sendCmd('exit');
await waitOK('exit');

let exited = false;
for (let i = 0; i < 20; i++) {
  await page.waitForTimeout(500);
  exited = await spriteVisible();
  if (exited === true) break;
}
await page.screenshot({ path: `${OUT_DIR}/after.png` });
if (exited !== true) fail(`sprite did NOT reappear after exit (visible=${exited})`);
ok(`after.png saved — sprite reappeared`);

// Clean up.
sendCmd('quit');
await once(py, 'close');
await browser.close();

if (consoleErrs.length) {
  console.error('console errors:');
  consoleErrs.forEach((e) => console.error('  - ' + e));
  fail(`${consoleErrs.length} console error(s)`);
}
console.log('\nBUILDING VISUAL PROOF: PASS');
console.log(`screenshots in ${OUT_DIR}/`);

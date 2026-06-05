// End-to-end persona → SDK → connect flow.
//
// Steps:
//   1. Open the frontend (assumes Vite dev server on :5173 and engine
//      on :8080 — start.sh handles both).
//   2. Click the "join as agent" button.
//   3. Fill the PersonaForm.
//   4. Submit; verify the credentials panel appears with agent_id +
//      ws_url.
//   5. Open a WS connection to ws_url using the agent_secret; verify
//      the engine sends at least one observation.
//
// Run after `./start.sh` from the repo root.

import { chromium } from 'playwright';
import WebSocket from 'ws';

const FRONTEND = process.env.FRONTEND ?? 'http://127.0.0.1:5173/';
const ENGINE = process.env.ENGINE ?? 'http://127.0.0.1:8080';

const results = [];
const check = (name, ok, detail = '') => results.push({ name, ok, detail });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
const pageErrors = [];
page.on('pageerror', (e) => pageErrors.push(e.message));

await page.goto(FRONTEND);
await page.waitForLoadState('networkidle');
await new Promise((r) => setTimeout(r, 2000));   // let pixi + ws settle

// 1. join-button visible
const joinBtn = page.getByTestId('join-agent-button');
check('1. join-as-agent button rendered', await joinBtn.isVisible());

// 2. open the modal
await joinBtn.click();
await page.waitForSelector('[role="dialog"][aria-label="Join as agent"]', { timeout: 4000 });
check('2. modal opened', true);

// 3. fill the form
await page.locator('input').first().fill('Smoketest Agent');
await page.locator('textarea').nth(0).fill('A test bot — should register cleanly.');
await page.locator('input[placeholder*="terse"]').fill('terse');
await page.locator('textarea').nth(1).fill('survive');
await page.locator('textarea').nth(2).fill('avoid threats');
await page.locator('textarea').nth(3).fill('{}');

// 4. submit
const submitBtn = page.getByRole('button', { name: /attach/i });
await submitBtn.click();

// 5. wait for credentials panel
await page.waitForSelector('[data-testid="join-credentials"]', { timeout: 8000 });
const credsJson = await page.getByTestId('join-creds-json').textContent();
check('5. credentials panel rendered', !!credsJson, credsJson?.slice(0, 80));

const creds = JSON.parse(credsJson);
check('5a. agent_id present', !!creds.agent_id, creds.agent_id);
check('5b. agent_secret present', !!creds.agent_secret);
check('5c. ws_url present', !!creds.ws_url, creds.ws_url);
check('5d. entity_id present', !!creds.entity_id, creds.entity_id);

// 6. validate WS handshake + first observation
const obsPromise = new Promise((resolve, reject) => {
  const t = setTimeout(() => reject(new Error('ws timeout — no observation in 8s')), 8000);
  const ws = new WebSocket(creds.ws_url);
  ws.on('open', () => ws.send(JSON.stringify({ auth: creds.agent_secret })));
  ws.on('message', (raw) => {
    try {
      const m = JSON.parse(raw.toString());
      if (m.type === 'observation') {
        clearTimeout(t);
        ws.close();
        resolve(m);
      }
    } catch (_) {}
  });
  ws.on('error', (e) => {
    clearTimeout(t);
    reject(e);
  });
});

try {
  const obs = await obsPromise;
  check(
    '6. ws auth + first observation received',
    true,
    `world_tick=${obs.world_tick} self.entity_id=${obs.self?.entity_id}`,
  );
} catch (e) {
  check('6. ws auth + first observation received', false, e.message);
}

// final report
console.log('\n========= E2E REPORT =========');
for (const r of results) {
  console.log(`  ${r.ok ? '✓' : '✗'} ${r.name}${r.detail ? ' :: ' + r.detail : ''}`);
}
const passed = results.filter((r) => r.ok).length;
console.log(`\n${passed}/${results.length} passed`);
if (pageErrors.length) {
  console.log('\n--- page errors ---');
  console.log(pageErrors.join('\n'));
}

await browser.close();
process.exit(passed === results.length ? 0 : 1);

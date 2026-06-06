// Headless browser probe for the Inspector tab fixes.
//
// What this verifies:
//   1. Inspector opens on entity click.
//   2. Clicking Speech/Mind/Trace tabs moves the yellow active highlight.
//      (Was broken: `const active = p.current === p.value` evaluated once at
//      mount in Solid, so Speech stayed yellow forever.)
//   3. Mind tab renders helpful empty-state text instead of nothing when
//      the engine returns "" for top_goal / last_reflection.
//   4. Trace tab renders helpful fallback text instead of nothing when
//      no reasoning traces exist (heuristic agents).
//
// Mind/Trace visibility is gated on capture_reasoning being on; the
// engine is started with -capture-reasoning via ./agent_sim, so tabs
// should be enabled regardless of whether a real trace exists.

import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';

const fail = (m) => { console.error(`FAIL: ${m}`); process.exit(1); };
const ok = (m) => console.log(`PASS: ${m}`);

// Pick any entity from /api/v1/agents (registered agents) — they're
// guaranteed to have an entity_id and pos. Fall back to picking one
// from the world snapshot if no agents registered yet.
async function pickEntity() {
  const ar = await fetch(`${ENGINE}/api/v1/agents`).then(r => r.json());
  if (Array.isArray(ar?.agents) && ar.agents.length) {
    const a = ar.agents.find(a => a.entity_id) ?? ar.agents[0];
    if (a?.entity_id) return { id: a.entity_id, pos: a.pos };
  }
  // Fall back: grab an entity from the snapshot via WS would be heavy;
  // try the NPCs from the bundle.
  const info = await fetch(`${ENGINE}/api/v1/world/info`).then(r => r.json());
  fail(`no agents registered to inspect (world=${info?.world}); spawn one first with ./agent_sim agents 1`);
}

const target = await pickEntity();
console.log(`probing entity ${target.id} at (${target.pos?.[0]},${target.pos?.[1]})`);

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
const consoleErrs = [];
page.on('console', m => { if (m.type() === 'error') consoleErrs.push(m.text()); });
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500); // give the viewer WS time to populate

// Dismiss the first-visit onboarding overlay if it appears — it
// intercepts pointer events on top of the toolbar.
const skipBtn = page.getByTestId('onboarding-skip');
if (await skipBtn.count()) {
  await skipBtn.first().click();
  await page.waitForTimeout(200);
}

// Open inspector by simulating an entity click via the same agents
// picker the UI uses. The picker exposes a button per agent; clicking
// it centers the camera and opens the inspector — exactly the flow we
// want to test.
const agentsBtn = page.getByRole('button', { name: /^agents$/i });
if (!await agentsBtn.count()) fail('agents toolbar button not found');
await agentsBtn.first().click();
await page.waitForTimeout(800);

// Agent row buttons carry data-testid="agent-row-<entity_id>". The
// first generic button in the picker is the close (×).
const agentRow = page.locator('[data-testid^="agent-row-"]').first();
if (!await agentRow.count()) fail('no agent-row buttons rendered in picker');
await agentRow.click();

// The inspector opens via setSelectedSnapshot, which is populated by
// the *next* viewport snapshot after the camera centers on the agent.
// Wait for it instead of a fixed timeout.
const inspector = page.getByTestId('inspector');
try {
  await inspector.first().waitFor({ state: 'visible', timeout: 8000 });
} catch (e) {
  fail('inspector did not open after agent pick (timed out)');
}
ok('inspector opened');

const tabSpeech = page.getByTestId('tab-speech');
const tabMind = page.getByTestId('tab-mind');
const tabTrace = page.getByTestId('tab-trace');

for (const [name, t] of [['speech', tabSpeech], ['mind', tabMind], ['trace', tabTrace]]) {
  if (!await t.count()) fail(`tab "${name}" missing in inspector`);
}
ok('all 3 tabs render');

// --- Active highlight follows the click ---
// "Active" = background near the orange #feae34 OR rgb(254,174,52).
async function bgOf(loc) {
  return await loc.evaluate(el => getComputedStyle(el).backgroundColor);
}
const ACTIVE_RGB = 'rgb(254, 174, 52)';

const bgSpeech0 = await bgOf(tabSpeech);
if (bgSpeech0 !== ACTIVE_RGB) fail(`Speech should be active on open, got bg=${bgSpeech0}`);
ok('Speech is active on open');

await tabMind.click();
await page.waitForTimeout(150);
const bgMind1 = await bgOf(tabMind);
const bgSpeech1 = await bgOf(tabSpeech);
if (bgMind1 !== ACTIVE_RGB) fail(`Mind active highlight didn't apply: bg=${bgMind1}`);
if (bgSpeech1 === ACTIVE_RGB) fail(`Speech still active after clicking Mind: bg=${bgSpeech1}`);
ok('clicking Mind moves the highlight to Mind');

await tabTrace.click();
await page.waitForTimeout(150);
const bgTrace2 = await bgOf(tabTrace);
const bgMind2 = await bgOf(tabMind);
if (bgTrace2 !== ACTIVE_RGB) fail(`Trace active highlight didn't apply: bg=${bgTrace2}`);
if (bgMind2 === ACTIVE_RGB) fail(`Mind still active after clicking Trace: bg=${bgMind2}`);
ok('clicking Trace moves the highlight to Trace');

await tabSpeech.click();
await page.waitForTimeout(150);
const bgSpeech3 = await bgOf(tabSpeech);
if (bgSpeech3 !== ACTIVE_RGB) fail(`Speech didn't reactivate on click: bg=${bgSpeech3}`);
ok('clicking Speech reactivates the highlight');

// --- Empty-state body content ---
// Mind tab: should render top_goal block + reflection block, never blank.
await tabMind.click();
await page.waitForTimeout(150);
const mindBody = page.getByTestId('mind-tab');
if (!await mindBody.count()) fail('mind tab body did not render');
const mindText = (await mindBody.innerText()).trim();
if (mindText.length < 10) fail(`mind tab body suspiciously empty: "${mindText}"`);
ok(`mind tab renders content (${mindText.length} chars)`);

// Trace tab: either renders trace rows OR the heuristic-agent fallback.
await tabTrace.click();
await page.waitForTimeout(150);
const traceBody = page.locator('[data-testid="trace-tab"], [data-testid="inspector"] >> text=/No reasoning traces/i');
if (!await traceBody.count()) fail('trace tab showed neither content nor fallback');
ok('trace tab renders content or fallback');

if (consoleErrs.length) {
  console.error('console errors detected:');
  consoleErrs.forEach(e => console.error('  - ' + e));
  fail(`${consoleErrs.length} console error(s)`);
}

await browser.close();
console.log('\nINSPECTOR PROBE: PASS');

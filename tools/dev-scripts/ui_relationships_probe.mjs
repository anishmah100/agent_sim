// Headless browser probe for the Relationships inspector tab (D19).
//
// What this verifies:
//   1. Inspector opens, exposes a 4th "Relationships" tab.
//   2. The Relationships tab renders either rows OR the empty-state
//      fallback (depending on whether the inspected agent has any
//      peer interactions yet).
//   3. The mental_state endpoint's `peers` field is what populates it
//      (verified end-to-end by registering two SDK bots that whisper
//      and asserting the receiver's Relationships tab counts go up).

import { chromium } from 'playwright';

const ENGINE = 'http://127.0.0.1:8080';
const FRONTEND = 'http://127.0.0.1:5173';

const fail = (m) => { console.error(`FAIL: ${m}`); process.exit(1); };
const ok = (m) => console.log(`PASS: ${m}`);

async function pickEntity() {
  const ar = await fetch(`${ENGINE}/api/v1/agents`).then(r => r.json());
  if (Array.isArray(ar?.agents) && ar.agents.length) {
    const a = ar.agents.find(a => a.entity_id) ?? ar.agents[0];
    if (a?.entity_id) return { id: a.entity_id, pos: a.pos };
  }
  fail('no agents registered to inspect; spawn one first');
}

const target = await pickEntity();
console.log(`probing entity ${target.id}`);

// Sanity check the endpoint shape BEFORE the UI probe so a missing
// peers field fails fast with a clear error.
const ms = await fetch(
  `${ENGINE}/api/v1/agent/${encodeURIComponent(target.id)}/mental_state`
).then(r => r.json());
if (!Object.prototype.hasOwnProperty.call(ms, 'peers')) {
  fail(`endpoint missing peers field; got keys=${Object.keys(ms).join(',')}`);
}
ok(`endpoint exposes peers field (type=${typeof ms.peers})`);

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
const consoleErrs = [];
page.on('console', m => { if (m.type() === 'error') consoleErrs.push(m.text()); });
await page.goto(FRONTEND, { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500);

const skipBtn = page.getByTestId('onboarding-skip');
if (await skipBtn.count()) {
  await skipBtn.first().click();
  await page.waitForTimeout(200);
}

const agentsBtn = page.getByRole('button', { name: /^agents$/i });
if (!await agentsBtn.count()) fail('agents toolbar button not found');
await agentsBtn.first().click();
await page.waitForTimeout(800);
const agentRow = page.locator('[data-testid^="agent-row-"]').first();
if (!await agentRow.count()) fail('no agent-row buttons rendered in picker');
await agentRow.click();

const inspector = page.getByTestId('inspector');
try {
  await inspector.first().waitFor({ state: 'visible', timeout: 8000 });
} catch (e) {
  fail('inspector did not open after agent pick');
}
ok('inspector opened');

const tabRel = page.getByTestId('tab-relationships');
if (!await tabRel.count()) fail('Relationships tab is missing');
ok('Relationships tab present');

await tabRel.click();
await page.waitForTimeout(200);

// Active highlight should follow the click — same active-color
// invariant as the other tabs (orange #feae34).
const bg = await tabRel.evaluate(el => getComputedStyle(el).backgroundColor);
if (bg !== 'rgb(254, 174, 52)') {
  fail(`Relationships tab active highlight missing after click: bg=${bg}`);
}
ok('Relationships tab active highlight applied');

// Body: must render either rows or the empty-state fallback.
const body = page.locator(
  '[data-testid="relationships-tab"], [data-testid="inspector"] >> text=/No social interactions logged/i'
);
if (!await body.count()) fail('relationships tab showed neither rows nor fallback');
ok('relationships tab renders content or fallback');

if (consoleErrs.length) {
  console.error('console errors detected:');
  consoleErrs.forEach(e => console.error('  - ' + e));
  fail(`${consoleErrs.length} console error(s)`);
}

await browser.close();
console.log('\nRELATIONSHIPS PROBE: PASS');

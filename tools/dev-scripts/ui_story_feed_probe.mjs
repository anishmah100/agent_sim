// Headless probe for the D17 Story Feed component.
//
// What this verifies:
//   1. The story-feed container renders (data-testid="story-feed").
//   2. All four filter pills are present (All / L1 / L2 / L3L4).
//   3. Clicking a filter pill flips its active style (no console
//      errors during the interaction).
//   4. A screenshot of the panel area is saved to /tmp/story_feed.png.
//
// Notes:
//   - We do NOT assert that story rows exist. The narrator may not
//     have emitted anything yet (especially on a fresh boot), and
//     the endpoint may not be wired yet — both are valid states.
//     The component MUST still render and the filter pills MUST
//     still respond.
//   - This is a UI-only probe. Engine running is preferred but not
//     required; the empty-state placeholder is acceptable.

import { chromium } from 'playwright'

const FRONTEND = 'http://127.0.0.1:5173'
const SHOT = '/tmp/story_feed.png'

const fail = (m) => { console.error(`FAIL: ${m}`); process.exit(1) }
const ok = (m) => console.log(`PASS: ${m}`)

const browser = await chromium.launch()
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } })
const page = await ctx.newPage()

// We intentionally tolerate 404s from /api/v1/narrator/recent: the
// endpoint is being wired in parallel and may not exist yet. Any
// OTHER console error is a real failure.
const NARRATOR_404_RE = /narrator\/recent.*\b404\b|\b404\b.*narrator\/recent/i
const RESOURCE_404_RE = /Failed to load resource.*404/i
const consoleErrs = []
const networkFails = []
page.on('console', (m) => {
  if (m.type() === 'error') consoleErrs.push(m.text())
})
page.on('response', (resp) => {
  if (resp.status() === 404 && /\/api\/v1\/narrator\/recent/.test(resp.url())) {
    networkFails.push(`narrator/recent 404 (expected — endpoint in parallel)`)
  }
})

try {
  await page.goto(FRONTEND, { waitUntil: 'domcontentloaded', timeout: 10000 })
} catch (e) {
  fail(`could not reach ${FRONTEND}: ${e.message} — is the dev server running?`)
}
await page.waitForTimeout(1500)

// Dismiss the onboarding overlay if it's up; it covers the bottom-right.
const skip = page.getByTestId('onboarding-skip')
if (await skip.count()) {
  await skip.first().click()
  await page.waitForTimeout(200)
}

// 1. Story Feed container.
const feed = page.getByTestId('story-feed')
try {
  await feed.first().waitFor({ state: 'visible', timeout: 5000 })
} catch (e) {
  fail(`story-feed did not render: ${e.message}`)
}
ok('story-feed container renders')

// 2. Filter pills.
const filters = ['all', 'L1', 'L2', 'L3L4']
for (const f of filters) {
  const pill = page.getByTestId(`story-feed-filter-${f}`)
  if (!(await pill.count())) fail(`filter pill missing: story-feed-filter-${f}`)
}
ok('all 4 filter pills present')

// 3. Clicking a filter pill toggles its style.
const ACTIVE_BG = 'rgb(254, 174, 52)' // #feae34 — same active color as the inspector tabs.
const bgOf = (loc) => loc.evaluate((el) => getComputedStyle(el).backgroundColor)

const pillAll = page.getByTestId('story-feed-filter-all')
const pillL1 = page.getByTestId('story-feed-filter-L1')
const pillL2 = page.getByTestId('story-feed-filter-L2')
const pillL3L4 = page.getByTestId('story-feed-filter-L3L4')

const allBg0 = await bgOf(pillAll)
if (allBg0 !== ACTIVE_BG) fail(`"All" should be active on open, got bg=${allBg0}`)
ok('"All" is active by default')

await pillL1.click()
await page.waitForTimeout(120)
const l1Bg = await bgOf(pillL1)
const allBg1 = await bgOf(pillAll)
if (l1Bg !== ACTIVE_BG) fail(`L1 didn't activate on click: bg=${l1Bg}`)
if (allBg1 === ACTIVE_BG) fail(`"All" still active after clicking L1: bg=${allBg1}`)
ok('clicking L1 moves the active highlight')

await pillL2.click()
await page.waitForTimeout(120)
const l2Bg = await bgOf(pillL2)
if (l2Bg !== ACTIVE_BG) fail(`L2 didn't activate on click: bg=${l2Bg}`)
ok('clicking L2 activates L2')

await pillL3L4.click()
await page.waitForTimeout(120)
const l3l4Bg = await bgOf(pillL3L4)
if (l3l4Bg !== ACTIVE_BG) fail(`L3+L4 didn't activate on click: bg=${l3l4Bg}`)
ok('clicking L3+L4 activates L3+L4')

await pillAll.click()
await page.waitForTimeout(120)

// 4. Screenshot — clip to the feed's bounding box plus a small margin.
const box = await feed.first().boundingBox()
if (!box) fail('story-feed has no bounding box (not laid out)')
const pad = 12
const clip = {
  x: Math.max(0, box.x - pad),
  y: Math.max(0, box.y - pad),
  width:  box.width  + pad * 2,
  height: box.height + pad * 2,
}
await page.screenshot({ path: SHOT, clip })
ok(`screenshot saved to ${SHOT}`)

// Drop the generic "Failed to load resource ... 404" lines that
// correspond to the narrator/recent 404 we saw on the network. Each
// such response produces one console line; we already accounted for
// it via the response listener.
const realErrs = consoleErrs.filter((e) => {
  if (NARRATOR_404_RE.test(e)) return false
  // Plain "Failed to load resource: 404" lines without a URL hint:
  // we can only be sure they're ours if the count matches. Be
  // conservative — match resource-404 to network 404s by count.
  return !RESOURCE_404_RE.test(e)
})
if (networkFails.length) {
  console.log(`note: ${networkFails.length} expected narrator/recent 404 — endpoint not wired yet`)
}
// If there are MORE resource-404 console lines than narrator 404
// responses we observed, the extras are real and we fail.
const resource404Count = consoleErrs.filter((e) => RESOURCE_404_RE.test(e)).length
if (resource404Count > networkFails.length) {
  console.error(`unexpected 404s: ${resource404Count - networkFails.length} beyond the narrator/recent count`)
  consoleErrs.forEach((e) => console.error('  - ' + e))
  fail('unexpected 404(s)')
}
if (realErrs.length) {
  console.error('console errors detected:')
  realErrs.forEach((e) => console.error('  - ' + e))
  fail(`${realErrs.length} console error(s)`)
}

await browser.close()
console.log('\nSTORY FEED PROBE: PASS')

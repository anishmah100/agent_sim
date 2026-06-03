import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
const errors = [];
const consoleErrors = [];
p.on('pageerror', e => errors.push(`PAGE ERR: ${e.message}`));
p.on('console', m => { if (m.type() === 'error') consoleErrors.push(`[console err] ${m.text()}`); });
const results = [];
const check = (name, ok, detail) => results.push({ name, ok, detail });

await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 4000));

// 1. Page loads, world rendered.
const init = await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  return {
    handle: !!h, entityCount: h?.getEntities().length,
    hasViewport: !!h?.viewport,
    hasTilemap: !!h?.viewport.children.find(c => c.label === 'tilemap'),
    canvasW: document.querySelector('canvas')?.width,
    canvasH: document.querySelector('canvas')?.height,
  };
});
check('1. page loads', !!init.handle, JSON.stringify(init));
check('1a. tilemap rendered', !!init.hasTilemap, '');
check('1b. entities loaded (3 NPCs)', init.entityCount === 3, `count=${init.entityCount}`);

// 2. WS state pill = open.
const wsTxt = await p.evaluate(() => {
  return Array.from(document.querySelectorAll('span'))
    .map(s => s.textContent).find(t => t?.startsWith('ws: '));
});
check('2. ws status pill shows open', wsTxt?.includes('open'), wsTxt);

// 3. fit-to-world button works.
await p.evaluate(() => globalThis.__pixiHandle.viewport.setZoom(0.5, false));
await p.click('button:has-text("fit to world")');
await new Promise(r => setTimeout(r, 600));
const zoom = await p.evaluate(() => globalThis.__pixiHandle.viewport.scaled);
check('3. fit-to-world button repositions viewport', Math.abs(zoom - 2.0) < 0.5, `zoom=${zoom}`);

// 4. Pan with mouse drag.
const beforePan = await p.evaluate(() => [globalThis.__pixiHandle.viewport.x, globalThis.__pixiHandle.viewport.y]);
await p.mouse.move(640, 400);
await p.mouse.down();
await p.mouse.move(400, 250, { steps: 8 });
await p.mouse.up();
await new Promise(r => setTimeout(r, 300));
const afterPan = await p.evaluate(() => [globalThis.__pixiHandle.viewport.x, globalThis.__pixiHandle.viewport.y]);
check('4. mouse-drag pans viewport',
  Math.abs(afterPan[0] - beforePan[0]) > 50 || Math.abs(afterPan[1] - beforePan[1]) > 50,
  `before=${beforePan} after=${afterPan}`);

// 5. Wheel zoom.
const beforeZoom = await p.evaluate(() => globalThis.__pixiHandle.viewport.scaled);
await p.mouse.move(640, 400);
for (let i = 0; i < 5; i++) { await p.mouse.wheel(0, -120); await new Promise(r => setTimeout(r, 80)); }
await new Promise(r => setTimeout(r, 300));
const afterZoom = await p.evaluate(() => globalThis.__pixiHandle.viewport.scaled);
check('5. wheel-zoom changes zoom level', Math.abs(afterZoom - beforeZoom) > 0.3,
  `before=${beforeZoom} after=${afterZoom}`);

// Reset view.
await p.click('button:has-text("fit to world")');
await new Promise(r => setTimeout(r, 600));

// 6. Click an entity opens inspector.
const click1 = await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  const e = h.getEntities().find(x => x.entity_id === 'test_npc_2');
  const s = h.viewport.toScreen(e.pos[0]*16 + 8, e.pos[1]*16 + 8);
  return { id: e.entity_id, sx: Math.round(s.x), sy: Math.round(s.y) };
});
await p.mouse.move(click1.sx, click1.sy);
await p.mouse.down(); await new Promise(r => setTimeout(r, 50)); await p.mouse.up();
await new Promise(r => setTimeout(r, 600));
const inspector1 = await p.evaluate(() => {
  const d = document.querySelector('[role="dialog"]');
  return d ? { open: true, text: d.innerText, hasEntityId: d.innerText.includes('test_npc_2') } : { open: false };
});
check('6. click entity opens inspector with correct id',
  inspector1.open && inspector1.hasEntityId,
  `open=${inspector1.open} match=${inspector1.hasEntityId}`);

// 7. Selection ring visible.
const ring = await p.evaluate(() => {
  const layer = globalThis.__pixiHandle.viewport.children.find(c => c.label === 'entities');
  const ring = layer?.children.find(c => c.constructor.name === '_Graphics' || c.constructor.name === 'Graphics');
  return { visible: ring?.visible, hasGraphics: !!ring };
});
check('7. gold selection ring visible after click', ring.visible === true, JSON.stringify(ring));

// 8. Inspector ESC closes.
await p.keyboard.press('Escape');
await new Promise(r => setTimeout(r, 400));
const afterEsc = await p.evaluate(() => !!document.querySelector('[role="dialog"]'));
check('8. ESC closes inspector', !afterEsc, `dialog still present=${afterEsc}`);

// 9. Background click also closes inspector — re-open and close via background.
await p.mouse.move(click1.sx, click1.sy);
await p.mouse.down(); await new Promise(r => setTimeout(r, 50)); await p.mouse.up();
await new Promise(r => setTimeout(r, 400));
// Click on water (somewhere in the middle of the canvas where no NPC is — pick a corner of canvas).
await p.mouse.move(200, 200);
await p.mouse.down(); await new Promise(r => setTimeout(r, 50)); await p.mouse.up();
await new Promise(r => setTimeout(r, 400));
const afterBg = await p.evaluate(() => !!document.querySelector('[role="dialog"]'));
check('9. background click closes inspector', !afterBg, `dialog still present=${afterBg}`);

// 10. Inspector × button — re-open and close via X.
await p.mouse.move(click1.sx, click1.sy);
await p.mouse.down(); await new Promise(r => setTimeout(r, 50)); await p.mouse.up();
await new Promise(r => setTimeout(r, 400));
const hasDialog = await p.evaluate(() => !!document.querySelector('[role="dialog"]'));
if (hasDialog) {
  await p.click('[role="dialog"] button:has-text("×")');
  await new Promise(r => setTimeout(r, 400));
}
const afterX = await p.evaluate(() => !!document.querySelector('[role="dialog"]'));
check('10. × button closes inspector', !afterX, `dialog after X=${afterX}`);

// 11. Live tick increments.
const t1 = await p.evaluate(() => {
  const spans = Array.from(document.querySelectorAll('span'));
  const m = spans.map(s => s.textContent).find(t => t?.includes('live tick'));
  return parseInt(m?.match(/live tick (\d+)/)?.[1] || '0');
});
await new Promise(r => setTimeout(r, 1500));
const t2 = await p.evaluate(() => {
  const spans = Array.from(document.querySelectorAll('span'));
  const m = spans.map(s => s.textContent).find(t => t?.includes('live tick'));
  return parseInt(m?.match(/live tick (\d+)/)?.[1] || '0');
});
check('11. live tick increments over time', t2 > t1 + 50, `t1=${t1} t2=${t2}`);

// 12. Character atlas loaded (visible sprites).
const sprites = await p.evaluate(() => {
  const layer = globalThis.__pixiHandle.viewport.children.find(c => c.label === 'entities');
  const sprites = layer?.children.flatMap(e =>
    e.children.filter(c => c.constructor.name === '_AnimatedSprite' || c.constructor.name === 'AnimatedSprite')
  );
  return sprites?.length ?? 0;
});
check('12. all entities use AnimatedSprite (not placeholder)', sprites === 3, `sprite count=${sprites}`);

await p.screenshot({ path: '/tmp/agent_sim_m5_final.png' });

console.log("\n========= UI TEST REPORT =========");
for (const r of results) {
  console.log(`  ${r.ok ? '✓' : '✗'} ${r.name}${r.ok ? '' : ' :: ' + r.detail}`);
}
const passed = results.filter(r => r.ok).length;
console.log(`\n${passed}/${results.length} passed`);
console.log("\n--- console errors ---");
console.log(consoleErrors.length ? consoleErrors.join('\n') : '(none)');
console.log("--- page errors ---");
console.log(errors.length ? errors.join('\n') : '(none)');

await b.close();
process.exit(passed === results.length ? 0 : 1);

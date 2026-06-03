import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 6000));
const dump = await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  const layer = h.viewport.children.find(c => c.label === 'tilemap');
  return {
    layerVisible: layer?.visible,
    layerAlpha: layer?.alpha,
    layerChildCount: layer?.children.length,
    layerChildren: layer?.children.map(c => ({ name: c.constructor.name, visible: c.visible, w: c.width, h: c.height })),
    viewportZoom: h.viewport.scaled,
    viewportPos: [h.viewport.x, h.viewport.y],
    viewportWorld: [h.viewport.worldWidth, h.viewport.worldHeight],
  };
});
console.log(JSON.stringify(dump, null, 2));
await b.close();

import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 6500));
const dump = await p.evaluate(() => {
  // Hook into the atlas via internal modules
  const h = globalThis.__pixiHandle;
  const layer = h.viewport.children.find(c => c.label === 'tilemap');
  const composite = layer.children[0];
  return {
    compositeChildCount: composite.children?.length,
    compositeBoundsW: composite.width,
    compositeBoundsH: composite.height,
    compositeVisible: composite.visible,
    compositeRenderable: composite.renderable,
    compositeTilesetCount: composite.tilesets?.length,
    tilemapsLength: composite.tilemaps?.length,
    firstTilemapTilesCount: composite.tilemaps?.[0]?.pointsBuf?.length,
  };
});
console.log(JSON.stringify(dump, null, 2));
await b.close();

import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
const p = await ctx.newPage();
p.on('console', m => console.log(`PAGE [${m.type()}]:`, m.text()));
await p.goto('http://127.0.0.1:5173/');
await new Promise(r => setTimeout(r, 5000));

const dump = await p.evaluate(() => {
  const h = globalThis.__pixiHandle;
  const layer = h.viewport.children.find(c => c.label === 'entities');
  const entities = layer?.children.filter(c => c.label?.startsWith('entity:'));
  return entities.map(c => ({
    label: c.label,
    pos: [c.x, c.y],
    visible: c.visible,
    children: c.children.map(ch => ({
      name: ch.constructor.name,
      visible: ch.visible,
      width: Math.round(ch.width),
      height: Math.round(ch.height),
      scaleX: ch.scale?.x,
      hasTextureSize: ch.texture ? [ch.texture.width, ch.texture.height] : null,
    })),
  }));
});
console.log('entities:', JSON.stringify(dump, null, 2));
await b.close();

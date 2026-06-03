// PixiJS world canvas host.
//
// Responsibilities:
// - Create the PixiJS Application
// - Mount it into the container element passed by App.tsx
// - Set up the viewport (pan/zoom via pixi-viewport)
// - Add the tilemap and entity layers under the viewport
// - Expose a small handle so the Solid layer can drive camera + inspect
//
// HARD RULE: nothing Solid-reactive lives here. The Solid layer
// communicates with PixiJS via the returned handle's methods, never by
// shared signals. Keeps reactivity isolated and the canvas immune to
// VDOM-style re-renders.
//
// Round 1 lesson baked in: don't mix HMR with PixiJS scenes — the
// listeners leak. The Pixi Application is created once per page load
// (full-reload on source change). Vite HMR for this file is opted out
// at the bottom.

import { Application, Container } from "pixi.js";
import { Viewport } from "pixi-viewport";
import { TilemapLayer, type TileMapData } from "./Tilemap";
import { EntityLayer, type EntityState } from "./Entity";
import { DecorationLayer } from "./Decoration";
import { InteriorLayer } from "./Interior";
import { TILE_SIZE_PX, resetTileCache } from "./tiles";
import { installClickToInspect, type ClickEvent } from "./input";
import { CharacterAtlas } from "./CharacterAtlas";
import { TileAtlas } from "./TileAtlas";
import { setTileAtlas } from "./tiles";

export interface PixiHandle {
  app: Application;
  viewport: Viewport;
  loadWorld(data: TileMapData): void;
  setEntities(entities: EntityState[]): void;
  getEntities(): EntityState[];
  centerOn(tileX: number, tileY: number): void;
  fitToWorld(): void;
  setSelectedEntity(id: string | null): void;
  onClick(handler: (ev: ClickEvent) => void): () => void;
  destroy(): void;
}

export async function mountPixiApp(host: HTMLElement): Promise<PixiHandle> {
  const app = new Application();
  await app.init({
    background: "#181425",                  // dark void — palette-aligned, makes off-map regions visible
    resizeTo: host,
    antialias: false,                       // pixel art — never AA
    roundPixels: true,                      // crisp pixel rendering
    resolution: window.devicePixelRatio,
    autoDensity: true,
    powerPreference: "high-performance",
  });
  host.appendChild(app.canvas);

  // Viewport handles pan + zoom + (eventually) follow-camera. It's our
  // scene's root container; anything we want to be world-positioned
  // goes inside it. UI overlay (HUD) lives outside, in the DOM.
  const viewport = new Viewport({
    screenWidth: app.screen.width,
    screenHeight: app.screen.height,
    worldWidth: 4096,                       // resized once we know world dims
    worldHeight: 4096,
    events: app.renderer.events,
  });
  viewport
    .drag({ mouseButtons: "all" })          // left + middle + right all drag
    .wheel({ smooth: 6, percent: 0.12 })    // scroll-wheel zoom, smoothed
    .clampZoom({ minScale: 0.5, maxScale: 4 })
    .pinch();
  app.stage.addChild(viewport);

  // Layers under the viewport. Order = render order.
  const tilemap = new TilemapLayer(app);
  const decorations = new DecorationLayer();
  const entities = new EntityLayer();
  const fxAbove = new Container();          // particles, selection rings, day/night tint top
  fxAbove.label = "fx_above";
  viewport.addChild(tilemap.container);
  viewport.addChild(decorations.container);
  viewport.addChild(entities.container);
  viewport.addChild(fxAbove);

  // Interior overlay — fixed-position container on the stage (NOT in
  // the viewport) so it doesn't pan/zoom with the world.
  const interior = new InteriorLayer(app);
  app.stage.addChild(interior.container);
  decorations.onBuildingClick(async (ev) => {
    await interior.show(ev.sprite);
  });
  interior.onExit(() => interior.hide());

  // Click handler — installed once. App-level listeners register
  // through the returned onClick().
  const clickHandlers: Array<(ev: ClickEvent) => void> = [];
  installClickToInspect({
    viewport,
    getEntities: () => entities.getAll(),
    onClick: (ev) => {
      for (const h of clickHandlers) h(ev);
    },
  });

  // Per-frame tick for entity layer effects (selection ring pulse).
  app.ticker.add((delta) => {
    entities.tick(delta.deltaMS);
  });

  // Kick off the character atlas load in the background.
  void CharacterAtlas.load().then(
    (atlas) => {
      entities.setAtlas(atlas);
      console.log(`character atlas loaded: ${atlas.list().length} characters`);
    },
    (err) => {
      console.warn("character atlas load failed; using placeholders:", err);
    },
  );

  // Kick off the tile atlas load too. When it resolves, swap the
  // placeholder colored quads for real Endesga-palette tile sprites
  // and re-render the tilemap.
  void TileAtlas.load().then(
    (atlas) => {
      setTileAtlas(atlas);
      if (currentWorld) tilemap.loadTileMap(currentWorld);
      console.log("tile atlas loaded — real overworld tiles now rendered");
    },
    (err) => {
      console.warn("tile atlas load failed; using placeholder colors:", err);
    },
  );

  // Resize the viewport's "screen size" when the host element resizes
  // — pixi-viewport needs explicit notification.
  app.renderer.on("resize", (w: number, h: number) => {
    viewport.resize(w, h);
  });

  let currentWorld: TileMapData | null = null;

  const doFitToWorld = (): void => {
    if (!currentWorld) return;
    viewport.moveCenter(viewport.worldWidth / 2, viewport.worldHeight / 2);
    // HG-style close camera — tiles render at ~64 device-px each so
    // detail (tree texture, character pixel art) reads at a glance.
    viewport.setZoom(4.0, true);
  };

  return {
    app,
    viewport,

    loadWorld(data: TileMapData) {
      currentWorld = data;
      tilemap.loadTileMap(data);
      viewport.worldWidth = data.width_tiles * TILE_SIZE_PX;
      viewport.worldHeight = data.height_tiles * TILE_SIZE_PX;
      entities.setAll(
        data.entities.map((e) => ({
          entity_id: e.entity_id,
          archetype: e.archetype,
          pos: e.pos,
          facing: e.facing,
          display_name: e.display_name,
        })),
      );
      void decorations.load(data.decorations ?? []);
      doFitToWorld();
    },

    setEntities(list: EntityState[]) {
      entities.setAll(list);
    },

    getEntities(): EntityState[] {
      return entities.getAll();
    },

    centerOn(tileX: number, tileY: number) {
      viewport.moveCenter(
        tileX * TILE_SIZE_PX + TILE_SIZE_PX / 2,
        tileY * TILE_SIZE_PX + TILE_SIZE_PX / 2,
      );
    },

    fitToWorld: doFitToWorld,

    setSelectedEntity(id: string | null) {
      entities.setSelected(id);
    },

    onClick(handler) {
      clickHandlers.push(handler);
      return () => {
        const i = clickHandlers.indexOf(handler);
        if (i >= 0) clickHandlers.splice(i, 1);
      };
    },

    destroy() {
      tilemap.destroy();
      entities.destroy();
      resetTileCache();
      app.destroy(true, { children: true, texture: true });
    },
  };
}

// Opt this file's owners out of HMR — see header comment.
if (import.meta.hot) {
  import.meta.hot.invalidate();
}

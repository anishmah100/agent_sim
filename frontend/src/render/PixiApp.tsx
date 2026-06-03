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
import { TILE_SIZE_PX, resetTileCache } from "./tiles";

export interface PixiHandle {
  app: Application;
  viewport: Viewport;
  loadWorld(data: TileMapData): void;
  setEntities(entities: EntityState[]): void;
  centerOn(tileX: number, tileY: number): void;
  fitToWorld(): void;
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
  const entities = new EntityLayer();
  const fxAbove = new Container();          // particles, selection rings, day/night tint top
  fxAbove.label = "fx_above";
  viewport.addChild(tilemap.container);
  viewport.addChild(entities.container);
  viewport.addChild(fxAbove);

  // Resize the viewport's "screen size" when the host element resizes
  // — pixi-viewport needs explicit notification.
  app.renderer.on("resize", (w: number, h: number) => {
    viewport.resize(w, h);
  });

  let currentWorld: TileMapData | null = null;

  const doFitToWorld = (): void => {
    if (!currentWorld) return;
    viewport.fit(true, viewport.worldWidth, viewport.worldHeight);
    viewport.moveCenter(viewport.worldWidth / 2, viewport.worldHeight / 2);
    // Comfortable default zoom — tiles render at ~32 device-px each.
    viewport.setZoom(2.0, true);
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
      doFitToWorld();
    },

    setEntities(list: EntityState[]) {
      entities.setAll(list);
    },

    centerOn(tileX: number, tileY: number) {
      viewport.moveCenter(
        tileX * TILE_SIZE_PX + TILE_SIZE_PX / 2,
        tileY * TILE_SIZE_PX + TILE_SIZE_PX / 2,
      );
    },

    fitToWorld: doFitToWorld,

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

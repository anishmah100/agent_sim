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
import { DecorationLayer, type DecorationInfoEvent } from "./Decoration";
import { InteriorLayer } from "./Interior";
import { SpeechBubbleLayer } from "./SpeechBubble";
import { DayNight } from "./DayNight";
import { HD2DStack } from "./HD2D";
import type { AudibleEvent } from "../net/ws";
import { TILE_SIZE_PX, resetTileCache } from "./tiles";
import { installClickToInspect, type ClickEvent } from "./input";
import { CharacterAtlas } from "./CharacterAtlas";
import { TileAtlas } from "./TileAtlas";
import { setTileAtlas } from "./tiles";
import { ArtCatalog, setArtCatalog } from "./ArtCatalog";

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
  /** Pointer-enter on a non-vegetation decoration. The InfoPanel
   *  appears for as long as the pointer stays on the sprite. */
  onDecorationHoverEnter(handler: (ev: DecorationInfoEvent) => void): () => void;
  /** Pointer-exit on a non-vegetation decoration. The panel hides. */
  onDecorationHoverExit(handler: (ev: DecorationInfoEvent) => void): () => void;
  /** Same hover-enter signal for interior props (inside a building view). */
  onInteriorPropHoverEnter(handler: (ev: DecorationInfoEvent) => void): () => void;
  /** Same hover-exit signal for interior props. */
  onInteriorPropHoverExit(handler: (ev: DecorationInfoEvent) => void): () => void;
  /** Editor — drop a decoration at (tileX, tileY). The sprite id +
   *  footprint come from the editor's selected palette entry. The
   *  caller is responsible for POSTing to /api/v1/world/edit_deco
   *  for persistence; this method only updates the local view. */
  addDecoration(spec: import("./Decoration").DecorationSpec): Promise<void>;
  /** Editor — remove the topmost decoration whose footprint contains
   *  (tileX, tileY) from the local view. Returns true if something
   *  was removed. The caller persists via POST. */
  removeDecorationAt(tileX: number, tileY: number): boolean;
  /** While the editor panel is open, building clicks should NOT auto-
   *  enter the interior — the click belongs to the editor (place /
   *  remove). Solid layer flips this whenever the editor toggles. */
  setEditorActive(active: boolean): void;
  ingestAudible(events: AudibleEvent[]): void;
  /** Editor — repaint one tile to a new glyph. Returns previous glyph
   *  for optimistic-update revert, or undefined if out of bounds. */
  setTileGlyph(tileX: number, tileY: number, glyph: string): string | undefined;
  /** Editor — read the current glyph at (x,y). undefined if unloaded. */
  getTileGlyph(tileX: number, tileY: number): string | undefined;
  /** Returns the current viewport rectangle in TILE coords, clipped
   *  to world bounds. Used by the minimap to draw a "what's visible
   *  now" indicator. Null if no world has loaded yet. */
  getViewportTileRect(): { x: number; y: number; w: number; h: number } | null;
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
  const speechBubbles = new SpeechBubbleLayer();
  const fxAbove = new Container();          // particles, selection rings, day/night tint top
  fxAbove.label = "fx_above";
  viewport.addChild(tilemap.container);
  viewport.addChild(decorations.container);
  viewport.addChild(entities.container);
  viewport.addChild(speechBubbles.container);
  viewport.addChild(fxAbove);

  // Interior overlay — fixed-position container on the stage (NOT in
  // the viewport) so it doesn't pan/zoom with the world.
  //
  // Click on an enterable building → enter the interior directly. The
  // hover-driven InfoPanel preview tells the user what they're about
  // to enter; no Enter button required.
  //
  // While the editor panel is open, the click belongs to the editor
  // (place / remove). editorActive guards the entry path so a user
  // clicking Remove on a cottage gets the cottage removed instead of
  // bounced into its interior view.
  let editorActive = false;
  const interior = new InteriorLayer(app);
  app.stage.addChild(interior.container);
  decorations.onBuildingClick(async (ev) => {
    if (editorActive) return;
    await interior.show(ev.sprite);
  });
  interior.onExit(() => interior.hide());
  if (import.meta.env.DEV) {
    (window as unknown as { __interior?: InteriorLayer }).__interior = interior;
    (window as unknown as { __viewport?: typeof viewport }).__viewport = viewport;
  }

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

  // Day/night ambient tint over the WORLD viewport (overworld layers
  // only — the interior overlay sits outside the viewport so its
  // visuals stay neutral).
  const dayNight = new DayNight(viewport);

  // HD-2D filter stack — bloom + saturation boost. Disabled by default
  // because the bloom pass runs on the entire viewport every frame and
  // dominates pan/zoom latency on the 1500×1500 Eldoria world. Re-enable
  // by setting VITE_ENABLE_HD2D=1 in a small map.
  if (import.meta.env.VITE_ENABLE_HD2D === "1") {
    const hd2d = new HD2DStack(viewport);
    void hd2d;
  }

  // Per-frame tick for entity layer effects (selection ring pulse).
  // Also refreshes viewport-culled tile + decoration sprites when the
  // camera has moved — refreshVisible() is a no-op when the visible
  // tile rect hasn't changed since the last call, so this is cheap.
  app.ticker.add((delta) => {
    entities.tick(delta.deltaMS);
    const byId = new Map<string, EntityState>();
    for (const e of entities.getAll()) byId.set(e.entity_id, e);
    speechBubbles.tick(byId);
    dayNight.tick();
    if (currentWorld) {
      const view = viewport.getVisibleBounds();
      tilemap.refreshVisible(view);
      decorations.refreshVisible(view);
    }
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

  // Art catalog — single source of truth for sprite ids. Loaded once;
  // every resolver (Decoration / Entity / Interior / …) delegates here.
  // Until the migration is finished, resolvers also keep their legacy
  // fallbacks so a missing entry doesn't break rendering.
  void ArtCatalog.load().then(
    (cat) => {
      setArtCatalog(cat);
      console.log(`art catalog loaded: ${cat.size()} sprites`);
    },
    (err) => {
      console.warn("art catalog load failed; resolvers will use legacy paths:", err);
    },
  );

  // Kick off the tile atlas load too. When it resolves, swap the
  // placeholder colored quads for real Endesga-palette tile sprites
  // and re-render the tilemap.
  void TileAtlas.load().then(
    (atlas) => {
      setTileAtlas(atlas);
      if (import.meta.env.DEV) {
        (window as unknown as { __tileAtlas?: TileAtlas }).__tileAtlas = atlas;
      }
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

    onDecorationHoverEnter(handler) {
      return decorations.onDecorationHoverEnter(handler);
    },
    onDecorationHoverExit(handler) {
      return decorations.onDecorationHoverExit(handler);
    },
    onInteriorPropHoverEnter(handler) {
      return interior.onPropHoverEnter(handler);
    },
    onInteriorPropHoverExit(handler) {
      return interior.onPropHoverExit(handler);
    },

    async addDecoration(spec) {
      await decorations.addOne(spec);
    },
    removeDecorationAt(tileX, tileY) {
      return decorations.removeAt(tileX, tileY);
    },
    setEditorActive(active) {
      editorActive = active;
    },

    setTileGlyph(tileX, tileY, glyph) {
      if (!currentWorld) return undefined;
      const legend = currentWorld.tiles_legend;
      const kind = legend[glyph];
      if (kind === undefined) return undefined;
      // Find the previous glyph by scanning the legend for the
      // current kind (kinds aren't 1-1 with glyphs, but the
      // closest-match prev is enough for revert UX).
      const prevKind = tilemap.getTileKind(tileX, tileY);
      const prevGlyph = prevKind
        ? Object.keys(legend).find((g) => legend[g] === prevKind)
        : undefined;
      const before = tilemap.setTileKind(tileX, tileY, kind);
      if (before === null) return undefined;
      // Trigger an immediate refresh so the new chunk is visible
      // without waiting for the next pan/zoom.
      tilemap.refreshVisible(viewport.getVisibleBounds());
      return prevGlyph;
    },

    getTileGlyph(tileX, tileY) {
      if (!currentWorld) return undefined;
      const kind = tilemap.getTileKind(tileX, tileY);
      if (kind === null) return undefined;
      const legend = currentWorld.tiles_legend;
      return Object.keys(legend).find((g) => legend[g] === kind);
    },

    ingestAudible(events) {
      speechBubbles.ingest(events);
    },

    getViewportTileRect() {
      if (!currentWorld) return null;
      // viewport.left/top/right/bottom are in WORLD (pixel) coords.
      const wTiles = currentWorld.width_tiles;
      const hTiles = currentWorld.height_tiles;
      const x = Math.max(0, viewport.left / TILE_SIZE_PX);
      const y = Math.max(0, viewport.top / TILE_SIZE_PX);
      const right = Math.min(wTiles, viewport.right / TILE_SIZE_PX);
      const bottom = Math.min(hTiles, viewport.bottom / TILE_SIZE_PX);
      return { x, y, w: Math.max(0, right - x), h: Math.max(0, bottom - y) };
    },

    destroy() {
      tilemap.destroy();
      entities.destroy();
      speechBubbles.destroy();
      resetTileCache();
      app.destroy(true, { children: true, texture: true });
    },
  };
}

// Opt this file's owners out of HMR — see header comment.
if (import.meta.hot) {
  import.meta.hot.invalidate();
}

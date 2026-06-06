// Decoration layer: static, walkable-or-blocking sprites placed above
// tiles but below characters. Trees, bushes, mushrooms, logs.
//
// Sprites are anchored at bottom-center of their footprint tile so they
// "sit" on the ground cleanly. Heights vary — a tree might span 3 tiles
// vertically with its trunk on the footprint tile and canopy extending
// upward. The footprint tile is the one that goes into the engine's
// collision occupant map.

import { Assets, Container, Graphics, Rectangle, Sprite, Texture } from "pixi.js";
import { OutlineFilter } from "pixi-filters";
import { TILE_SIZE_PX } from "./tiles";
import { artCatalog } from "./ArtCatalog";

// Padding (in tiles) around the viewport for decoration culling. Tall
// decorations (trees, buildings) can poke up several tiles above their
// footprint, so we keep a generous vertical padding ring to avoid
// pop-in at the top edge.
const DECO_PAD_TILES = 24;

// Per-frame creation budget. Each decoration is a Container + Sprite +
// optional shadow + optional outline filter — heavier than a tile.
const DECOS_PER_FRAME = 80;
// Throttle: defers refresh in tight ticker loops.
const DECO_REFRESH_INTERVAL_MS = 20;

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";
const OBJ_URL = (cat: string, file: string) =>
  `${ENGINE_URL}/art/processed/objects/${cat}/${file}`;

export interface DecorationSpec {
  /** SOUTH-WEST footprint corner. For 1×1 sprites this is just "the
   *  tile they sit on"; for multi-tile buildings, this is the
   *  west-most and south-most cell of the rectangular footprint. */
  x: number;
  y: number;
  sprite: string;
  /** Render size in TILES tall. Trees: ~2. Buildings: 3-4. */
  height_tiles?: number;
  /** Render width in TILES. If omitted, width derives from sprite
   *  aspect ratio scaled to height_tiles. Set explicitly for multi-
   *  tile buildings so footprint and render width agree. */
  footprint_w?: number;
  /** Render footprint height in tiles (south rows blocked at ground
   *  level). Defaults to 1 for grounded sprites (trees), can be
   *  bigger for buildings that block a 2- or 3-tile-deep slab. */
  footprint_h?: number;
  walkable?: boolean;
}

export interface BuildingClickEvent {
  /** "bld:000" etc */
  sprite: string;
  /** SW footprint corner in tile coords */
  x: number;
  y: number;
}

/** Emitted on hover-enter of any non-vegetation decoration. The Solid
 *  layer uses this to *show* the InfoPanel ephemerally — the matching
 *  hover-exit event hides it. Clicks on enterable buildings still fire
 *  BuildingClickEvent (the InfoPanel itself no longer has an Enter
 *  button — click = enter directly). */
export interface DecorationInfoEvent {
  sprite: string;
  x: number;
  y: number;
}

export class DecorationLayer {
  readonly container: Container;
  private cache = new Map<string, Texture>();
  private clickHandlers: Array<(ev: BuildingClickEvent) => void> = [];
  private hoverEnterHandlers: Array<(ev: DecorationInfoEvent) => void> = [];
  private hoverExitHandlers: Array<(ev: DecorationInfoEvent) => void> = [];

  // Viewport culling state. We keep the spec list and a spatial bucket
  // so refreshVisible() can rapidly find decorations in the viewport.
  // Bucket size = 64 tiles (covers most large worlds in <1000 buckets).
  private static readonly BUCKET = 64;
  private specs: DecorationSpec[] = [];
  private bucketIndex = new Map<string, number[]>(); // "bx,by" → spec indices
  // Materialized sprite per spec index, present iff currently in view.
  private liveSprites = new Map<number, Container>();
  // Track last viewport rect to skip no-op refreshes.
  private vx0 = 0; private vy0 = 0; private vx1 = 0; private vy1 = 0;
  private hasVis = false;
  private lastRefreshMs = 0;

  constructor() {
    this.container = new Container();
    this.container.label = "decorations";
    this.container.sortableChildren = true;
  }

  onBuildingClick(handler: (ev: BuildingClickEvent) => void): () => void {
    this.clickHandlers.push(handler);
    return () => {
      const i = this.clickHandlers.indexOf(handler);
      if (i >= 0) this.clickHandlers.splice(i, 1);
    };
  }

  /** Fires when the pointer ENTERS a non-vegetation decoration. The
   *  InfoPanel uses this to appear. */
  onDecorationHoverEnter(handler: (ev: DecorationInfoEvent) => void): () => void {
    this.hoverEnterHandlers.push(handler);
    return () => {
      const i = this.hoverEnterHandlers.indexOf(handler);
      if (i >= 0) this.hoverEnterHandlers.splice(i, 1);
    };
  }

  /** Fires when the pointer LEAVES a non-vegetation decoration. */
  onDecorationHoverExit(handler: (ev: DecorationInfoEvent) => void): () => void {
    this.hoverExitHandlers.push(handler);
    return () => {
      const i = this.hoverExitHandlers.indexOf(handler);
      if (i >= 0) this.hoverExitHandlers.splice(i, 1);
    };
  }

  /** Add a single decoration after the initial load — used by the
   *  editor to optimistically render a placement before the engine
   *  round-trip completes. Loads the texture if needed, then registers
   *  in the spatial bucket so it appears on the next refreshVisible(). */
  async addOne(spec: DecorationSpec): Promise<void> {
    const i = this.specs.length;
    this.specs.push(spec);
    const BUCKET = DecorationLayer.BUCKET;
    const bx = Math.floor(spec.x / BUCKET);
    const by = Math.floor(spec.y / BUCKET);
    const key = `${bx},${by}`;
    let arr = this.bucketIndex.get(key);
    if (!arr) { arr = []; this.bucketIndex.set(key, arr); }
    arr.push(i);
    if (!this.cache.has(spec.sprite)) {
      const url = spriteUrl(spec.sprite);
      if (url) {
        try {
          const tex = await Assets.load<Texture>(url);
          tex.source.scaleMode = "nearest";
          this.cache.set(spec.sprite, tex);
        } catch (e) {
          console.warn(`addOne: texture load failed for ${spec.sprite}`, e);
        }
      }
    }
    // Force a refresh against the current viewport so the new sprite
    // materialises immediately if it's in view.
    this.hasVis = false;
  }

  async load(specs: DecorationSpec[]): Promise<void> {
    this.clear();
    this.specs = specs.slice();
    this.bucketIndex.clear();

    // Build a spatial bucket so refreshVisible can find specs by region.
    const BUCKET = DecorationLayer.BUCKET;
    for (let i = 0; i < this.specs.length; i++) {
      const s = this.specs[i];
      const bx = Math.floor(s.x / BUCKET);
      const by = Math.floor(s.y / BUCKET);
      const key = `${bx},${by}`;
      let arr = this.bucketIndex.get(key);
      if (!arr) { arr = []; this.bucketIndex.set(key, arr); }
      arr.push(i);
    }

    // Preload unique textures in parallel.
    const uniq = [...new Set(specs.map((s) => s.sprite))];
    await Promise.all(
      uniq.map(async (id) => {
        if (this.cache.has(id)) return;
        const url = spriteUrl(id);
        if (!url) {
          console.warn(`unknown decoration sprite id: ${id}`);
          return;
        }
        try {
          const tex = await Assets.load<Texture>(url);
          tex.source.scaleMode = "nearest";
          this.cache.set(id, tex);
        } catch (e) {
          console.warn(`decoration load failed: ${id}`, e);
        }
      }),
    );
    // Sprites are materialized on first refreshVisible() call.
    this.hasVis = false;
  }

  /** Add/remove decoration sprites so only the ones intersecting the
   *  given viewport rect (in world pixel coords) are live. */
  refreshVisible(viewRect: Rectangle): void {
    if (this.specs.length === 0) return;
    // Throttle to avoid recomputing the wanted set every frame during
    // continuous drag.
    const now = performance.now();
    if (now - this.lastRefreshMs < DECO_REFRESH_INTERVAL_MS) return;
    this.lastRefreshMs = now;
    const pad = DECO_PAD_TILES;
    const x0 = Math.floor(viewRect.x / TILE_SIZE_PX) - pad;
    const y0 = Math.floor(viewRect.y / TILE_SIZE_PX) - pad;
    const x1 = Math.ceil((viewRect.x + viewRect.width) / TILE_SIZE_PX) + pad;
    const y1 = Math.ceil((viewRect.y + viewRect.height) / TILE_SIZE_PX) + pad;
    if (this.hasVis && x0 === this.vx0 && y0 === this.vy0 && x1 === this.vx1 && y1 === this.vy1) {
      return;
    }
    this.vx0 = x0; this.vy0 = y0; this.vx1 = x1; this.vy1 = y1;
    this.hasVis = true;

    // Find candidate spec indices via spatial buckets.
    const BUCKET = DecorationLayer.BUCKET;
    const bx0 = Math.floor(x0 / BUCKET);
    const by0 = Math.floor(y0 / BUCKET);
    const bx1 = Math.floor((x1 - 1) / BUCKET);
    const by1 = Math.floor((y1 - 1) / BUCKET);
    const wanted = new Set<number>();
    for (let by = by0; by <= by1; by++) {
      for (let bx = bx0; bx <= bx1; bx++) {
        const arr = this.bucketIndex.get(`${bx},${by}`);
        if (!arr) continue;
        for (const i of arr) {
          const s = this.specs[i];
          if (s.x >= x0 && s.x < x1 && s.y >= y0 && s.y < y1) wanted.add(i);
        }
      }
    }

    // Remove sprites no longer wanted.
    for (const [i, wrap] of this.liveSprites) {
      if (!wanted.has(i)) {
        wrap.destroy();
        this.liveSprites.delete(i);
      }
    }
    // Add new wanted sprites, capped to keep per-frame cost bounded.
    let budget = DECOS_PER_FRAME;
    for (const i of wanted) {
      if (this.liveSprites.has(i)) continue;
      const spec = this.specs[i];
      const tex = this.cache.get(spec.sprite);
      if (!tex) continue;
      const wrap = this.addSprite(spec, tex);
      if (wrap) this.liveSprites.set(i, wrap);
      if (--budget <= 0) {
        // Force a re-pass next frame so the rest get materialised.
        this.hasVis = false;
        break;
      }
    }
  }

  private addSprite(spec: DecorationSpec, tex: Texture): Container {
    const wrap = new Container();
    const heightTiles = spec.height_tiles ?? 2.0;
    const footprintW = spec.footprint_w ?? 1;
    const targetH = heightTiles * TILE_SIZE_PX;
    // If footprint_w is given, force render width to match the
    // footprint exactly (buildings); otherwise use sprite aspect.
    const targetW = spec.footprint_w
      ? footprintW * TILE_SIZE_PX
      : tex.width * (targetH / tex.height);

    // Footprint center in local coords.
    const footprintCenterX = (footprintW * TILE_SIZE_PX) / 2;
    const footprintBottom = TILE_SIZE_PX - 1;

    // Shadows are expensive (per-deco Graphics + ellipse fill). Only
    // buildings (large footprint, sit "on" the ground) need them; small
    // vegetation looks fine without and renders ~2× faster.
    const wantsShadow = (spec.footprint_w ?? 1) >= 2;
    if (wantsShadow) {
      const shadow = new Graphics();
      shadow.ellipse(
        footprintCenterX,
        footprintBottom,
        Math.max(5, targetW * 0.35),
        Math.max(2, targetW * 0.10),
      ).fill({ color: 0x000000, alpha: 0.28 });
      wrap.addChild(shadow);
    }

    const sp = new Sprite(tex);
    sp.anchor.set(0.5, 1.0); // bottom-center
    sp.x = footprintCenterX;
    sp.y = footprintBottom;
    sp.width = targetW;
    sp.height = targetH;
    wrap.addChild(sp);

    wrap.x = spec.x * TILE_SIZE_PX;
    wrap.y = spec.y * TILE_SIZE_PX;
    wrap.zIndex = spec.y;
    this.container.addChild(wrap);

    // Hover + click handling — every decoration except vegetation
    // (trees, rocks, mushrooms, boulders, stalagmites) is interactive.
    // Hover-enter/exit drives the InfoPanel ephemerally; a click on
    // an enterable building triggers entry directly (no "Enter" button
    // in the panel — hover shows info, click commits).
    const cat = artCatalog();
    const category = spec.sprite.split(":")[0];
    const isInteractive = category !== "veg";
    const isEnterable = cat ? cat.enterable(spec.sprite) : LEGACY_ENTERABLE.has(spec.sprite);
    if (isInteractive) {
      sp.eventMode = "static";
      sp.cursor = isEnterable ? "pointer" : "help";
      const hoverFilter = new OutlineFilter({
        thickness: 2,
        color: 0xfff2a8,
        alpha: 0.85,
        knockout: false,
      });
      const evShape: DecorationInfoEvent = {
        sprite: spec.sprite,
        x: spec.x,
        y: spec.y,
      };
      sp.on("pointerover", () => {
        sp.filters = [hoverFilter];
        for (const h of this.hoverEnterHandlers) h(evShape);
      });
      sp.on("pointerout", () => {
        sp.filters = [];
        for (const h of this.hoverExitHandlers) h(evShape);
      });
      if (isEnterable) {
        sp.on("pointertap", () => {
          for (const h of this.clickHandlers) h({ ...evShape });
        });
      }
    }
    return wrap;
  }

  clear(): void {
    for (const child of [...this.container.children]) child.destroy();
    this.liveSprites.clear();
    this.specs = [];
    this.bucketIndex.clear();
    this.hasVis = false;
  }

  destroy(): void {
    this.clear();
    this.container.destroy();
    for (const tex of this.cache.values()) tex.destroy(true);
    this.cache.clear();
  }
}

// Sprite IDs come in two flavors:
//   - v1 numeric: "bld:000", "veg:003", etc. — point at art/processed/objects/<cat>/obj_NNN.png
//   - v2 named:   "bld:blacksmith", "veg:tree_oak", "fx:window_glow", "props:bed_red" etc.
//     — point at the processed v2 sliced sheets.
// Building IDs that historically opened an interior — used only as a
// fallback when the art catalog hasn't loaded yet.
const LEGACY_ENTERABLE = new Set([
  "bld:000", "bld:001", "bld:blacksmith", "bld:town_hall",
  "bld:granary", "bld:watchtower",
]);

// Legacy fallback for spriteUrl — kept ONLY for ids the catalog doesn't
// yet cover (e.g. bld:stall_red_bread_open is still emitted by the
// world generator under the bld: namespace; the catalog has it as
// stall:red_bread_open). Once the generator is updated to use the
// canonical category prefixes, this whole function can be deleted.
const V2_BUILDING_NAMES = new Set([
  "blacksmith", "town_hall", "granary", "watchtower", "well",
]);
function spriteUrl(id: string): string | null {
  // Catalog wins when it has the id.
  const cat = artCatalog();
  if (cat) {
    const u = cat.url(id);
    if (u) return u;
    // Also try the canonical-prefix alias for legacy ids the world
    // generator still emits (bld:stall_*, bld:cottage_stage_*, etc.).
    const aliasUrl = cat.url(legacyAlias(id));
    if (aliasUrl) return aliasUrl;
  }

  // Legacy path templates — guarantee no regression while migration
  // is in flight.
  const [c, num] = id.split(":");
  if (!c || !num) return null;
  if (c === "bld" && V2_BUILDING_NAMES.has(num)) {
    return `${ENGINE_URL}/art/processed/v2_${num}.png`;
  }
  if (c === "veg" && /^[a-z]/.test(num)) {
    return `${ENGINE_URL}/art/processed/v2_resources_world_master/${num}.png`;
  }
  if (c === "bld" && num.startsWith("stall_")) {
    return `${ENGINE_URL}/art/processed/v2_market_stall/${num}.png`;
  }
  if (c === "bld" && (num.startsWith("cottage_stage_") || num.startsWith("wreckage_") || num.startsWith("scaffolding_"))) {
    return `${ENGINE_URL}/art/processed/v2_construction_stages/${num}.png`;
  }
  const dir =
    c === "veg" ? "vegetation" :
    c === "bld" ? "buildings" :
    c === "int" ? "interior" :
    c === "item" ? "items" : null;
  if (!dir) return null;
  return OBJ_URL(dir, `obj_${num.padStart(3, "0")}.png`);
}

/** legacyAlias rewrites old-namespace ids to the catalog's canonical
 *  prefix. bld:stall_red_bread_open → stall:red_bread_open;
 *  bld:cottage_stage_0_blueprint → stage:cottage_stage_0_blueprint. */
function legacyAlias(id: string): string {
  const [c, num] = id.split(":");
  if (!c || !num) return id;
  if (c === "bld" && num.startsWith("stall_")) {
    return "stall:" + num.slice("stall_".length);
  }
  if (c === "bld" && (num.startsWith("cottage_stage_") || num.startsWith("wreckage_") || num.startsWith("scaffolding_"))) {
    return "stage:" + num;
  }
  return id;
}

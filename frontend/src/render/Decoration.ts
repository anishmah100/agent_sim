// Decoration layer: static, walkable-or-blocking sprites placed above
// tiles but below characters. Trees, bushes, mushrooms, logs.
//
// Sprites are anchored at bottom-center of their footprint tile so they
// "sit" on the ground cleanly. Heights vary — a tree might span 3 tiles
// vertically with its trunk on the footprint tile and canopy extending
// upward. The footprint tile is the one that goes into the engine's
// collision occupant map.

import { Assets, Container, Graphics, Sprite, Texture } from "pixi.js";
import { OutlineFilter } from "pixi-filters";
import { TILE_SIZE_PX } from "./tiles";

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

export class DecorationLayer {
  readonly container: Container;
  private cache = new Map<string, Texture>();
  private clickHandlers: Array<(ev: BuildingClickEvent) => void> = [];

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

  async load(specs: DecorationSpec[]): Promise<void> {
    this.clear();
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
    for (const spec of specs) {
      const tex = this.cache.get(spec.sprite);
      if (!tex) continue;
      this.addSprite(spec, tex);
    }
  }

  private addSprite(spec: DecorationSpec, tex: Texture): void {
    const wrap = new Container();
    const heightTiles = spec.height_tiles ?? 2.0;
    const footprintW = spec.footprint_w ?? 1;
    const targetH = heightTiles * TILE_SIZE_PX;
    // If footprint_w is given, force render width to match the
    // footprint exactly (buildings); otherwise use sprite aspect.
    const targetW = spec.footprint_w
      ? footprintW * TILE_SIZE_PX
      : tex.width * (targetH / tex.height);

    // Footprint center in local coords. (x, y) is the SW corner of the
    // footprint, so the centre is half-a-footprint east of x.
    const footprintCenterX = (footprintW * TILE_SIZE_PX) / 2;
    const footprintBottom = TILE_SIZE_PX - 1;

    const shadow = new Graphics();
    shadow.ellipse(
      footprintCenterX,
      footprintBottom,
      Math.max(5, targetW * 0.35),
      Math.max(2, targetW * 0.10),
    ).fill({ color: 0x000000, alpha: 0.28 });
    wrap.addChild(shadow);

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

    // Buildings get hover + click. Pokemon-style enter UX: mouse-over
    // glows an outline; click emits a building-click event that the
    // top-level UI handles (opens the interior view).
    const isBuilding = spec.sprite.startsWith("bld:") &&
                       (spec.footprint_w ?? 1) >= 2;  // small props (well, signpost) excluded
    if (isBuilding) {
      sp.eventMode = "static";
      sp.cursor = "pointer";
      const hoverFilter = new OutlineFilter({
        thickness: 2,
        color: 0xfff2a8,
        alpha: 0.85,
        knockout: false,
      });
      sp.on("pointerover", () => {
        sp.filters = [hoverFilter];
      });
      sp.on("pointerout", () => {
        sp.filters = [];
      });
      sp.on("pointertap", () => {
        const ev: BuildingClickEvent = {
          sprite: spec.sprite,
          x: spec.x,
          y: spec.y,
        };
        for (const h of this.clickHandlers) h(ev);
      });
    }
  }

  clear(): void {
    for (const child of [...this.container.children]) child.destroy();
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
const V2_BUILDING_NAMES = new Set([
  "blacksmith", "town_hall", "granary", "watchtower", "well",
]);
function spriteUrl(id: string): string | null {
  const [cat, num] = id.split(":");
  if (!cat || !num) return null;

  // v2 named buildings — standalone PNGs at processed/v2_<name>.png
  if (cat === "bld" && V2_BUILDING_NAMES.has(num)) {
    return `${ENGINE_URL}/art/processed/v2_${num}.png`;
  }
  // v2 vegetation/resources entities — sliced master sheet
  if (cat === "veg" && /^[a-z]/.test(num)) {
    return `${ENGINE_URL}/art/processed/v2_resources_world_master/${num}.png`;
  }
  // v2 market stalls — sliced sheet
  if (cat === "bld" && num.startsWith("stall_")) {
    return `${ENGINE_URL}/art/processed/v2_market_stall/${num}.png`;
  }
  // v2 construction stages — sliced sheet
  if (cat === "bld" && (num.startsWith("cottage_stage_") || num.startsWith("wreckage_") || num.startsWith("scaffolding_"))) {
    return `${ENGINE_URL}/art/processed/v2_construction_stages/${num}.png`;
  }

  // Legacy v1: numeric IDs → obj_NNN.png under category folder.
  const dir =
    cat === "veg" ? "vegetation" :
    cat === "bld" ? "buildings" :
    cat === "int" ? "interior" :
    cat === "item" ? "items" : null;
  if (!dir) return null;
  return OBJ_URL(dir, `obj_${num.padStart(3, "0")}.png`);
}

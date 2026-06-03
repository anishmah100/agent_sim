// Decoration layer: static, walkable-or-blocking sprites placed above
// tiles but below characters. Trees, bushes, mushrooms, logs.
//
// Sprites are anchored at bottom-center of their footprint tile so they
// "sit" on the ground cleanly. Heights vary — a tree might span 3 tiles
// vertically with its trunk on the footprint tile and canopy extending
// upward. The footprint tile is the one that goes into the engine's
// collision occupant map.

import { Assets, Container, Graphics, Sprite, Texture } from "pixi.js";
import { TILE_SIZE_PX } from "./tiles";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";
const OBJ_URL = (cat: string, file: string) =>
  `${ENGINE_URL}/art/processed/objects/${cat}/${file}`;

export interface DecorationSpec {
  /** Footprint tile (engine's collision cell). */
  x: number;
  y: number;
  /** Object library identifier — e.g. "veg:000" → vegetation/obj_000.png. */
  sprite: string;
  /** Render size in TILES tall. Default 2. Width is computed from
   *  the source aspect ratio. Trees: 2.5–3. Bushes: 1–1.5. */
  height_tiles?: number;
  /** Whether the engine should treat the footprint tile as walkable.
   *  Trees: false. Mushrooms/flowers: true. v1 client doesn't enforce
   *  — we just record the intent in the world data. */
  walkable?: boolean;
}

export class DecorationLayer {
  readonly container: Container;
  private cache = new Map<string, Texture>();

  constructor() {
    this.container = new Container();
    this.container.label = "decorations";
    // Sort by zIndex so taller trees draw behind characters that walk
    // in front of them. We assign zIndex = footprint_y so trees south
    // of a character render after (= on top of) the character. v1 keeps
    // this layer entirely below the entity layer — fine for now.
    this.container.sortableChildren = true;
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
    const targetH = heightTiles * TILE_SIZE_PX;
    const scale = targetH / tex.height;
    const targetW = tex.width * scale;

    // Drop shadow under the footprint — same trick we use for chars.
    const shadow = new Graphics();
    shadow.ellipse(
      TILE_SIZE_PX / 2,
      TILE_SIZE_PX - 2,
      Math.max(5, targetW * 0.35),
      Math.max(2, targetW * 0.12),
    ).fill({ color: 0x000000, alpha: 0.28 });
    wrap.addChild(shadow);

    const sp = new Sprite(tex);
    sp.anchor.set(0.5, 1.0); // bottom-center
    sp.x = TILE_SIZE_PX / 2;
    sp.y = TILE_SIZE_PX - 1;
    sp.width = targetW;
    sp.height = targetH;
    wrap.addChild(sp);

    wrap.x = spec.x * TILE_SIZE_PX;
    wrap.y = spec.y * TILE_SIZE_PX;
    // Y-sort key — south = drawn later = on top.
    wrap.zIndex = spec.y;
    this.container.addChild(wrap);
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

function spriteUrl(id: string): string | null {
  const [cat, num] = id.split(":");
  if (!cat || !num) return null;
  const dir =
    cat === "veg" ? "vegetation" :
    cat === "bld" ? "buildings" :
    cat === "int" ? "interior" :
    cat === "item" ? "items" : null;
  if (!dir) return null;
  return OBJ_URL(dir, `obj_${num.padStart(3, "0")}.png`);
}

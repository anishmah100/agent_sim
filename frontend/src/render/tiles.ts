// Tile textures + variant selection.
//
// Two paths:
//   getTileTexture(...)          — solid default tile for a kind. Used as
//                                  fallback before the atlas loads, and
//                                  by code that doesn't care about (x,y).
//   getTileTextureAt(kind, x, y) — deterministic per-tile variant pick.
//                                  Same (x,y) always gets the same tile
//                                  so the world doesn't shuffle on reload.
//
// Variant philosophy: most tiles render the plain default. A small share
// (~10%) render a SUBTLE variant (grass_tuft, dirt_cracked, water_ripple)
// — those still read as the same ground kind, just with tiny texture
// variation. A much smaller share (~1.5%) render a RARE FEATURE (a
// mushroom on grass, a lily on water). Together this avoids two failure
// modes: pure-default looks stamped; uniform-30%-random looks like noise.

import { Application, Graphics, RenderTexture, Texture, type Renderer } from "pixi.js";
import { TileAtlas } from "./TileAtlas";

export type TileKind =
  | "grass"
  | "dirt"
  | "path"
  | "water"
  | "stone"
  | "wall"
  | "floor_wood"
  | "void";

const TILE_COLORS: Record<TileKind, number> = {
  grass:      0x63c74d,
  dirt:       0x733e39,
  path:       0xb86f50,
  water:      0x0099db,
  stone:      0x8b9bb4,
  wall:       0x3a4466,
  floor_wood: 0xc28569,
  void:       0x181425,
};

export const TILE_SIZE_PX = 16;

// Hand-curated. Only tiles that read as "still the same ground" go here
// — alternates with subtle differences from the default. NOT autotile
// edges, NOT structures, NOT distinct objects.
const SUBTLE_VARIANTS: Record<string, string[]> = {
  grass: ["grass_tuft", "grass_pebble"],
  water: ["water_ripple"],
  // dirt: variants disabled — the dirt clearing was breaking up into
  // off-tone squares from the few legacy variants that didn't match
  // the new painterly dirt base. Until we install matching variants
  // we render dirt as solid.
  // stone/path: defaults already vary enough; no variant pool.
};

// Hand-curated. Distinct features that should appear at most once every
// ~70 tiles — a single mushroom in a patch of grass, a lily on a pond.
const RARE_FEATURES: Record<string, string[]> = {
  grass: ["grass_mushroom"],
  water: ["water_lily", "water_rock"],
};

const SUBTLE_RATE = 0.10;
const FEATURE_RATE = 0.015;

let placeholderCache: Map<TileKind, Texture> | null = null;
let atlas: TileAtlas | null = null;

export function setTileAtlas(a: TileAtlas | null): void {
  atlas = a;
}

/** Deterministic 32-bit hash of (x, y, salt). xorshift-style; cheap and
 *  produces visibly uncorrelated values for adjacent tiles. */
function hash2(x: number, y: number, salt: number): number {
  let h = (x | 0) * 374761393 + (y | 0) * 668265263 + (salt | 0) * 2147483647;
  h = (h ^ (h >>> 13)) | 0;
  h = Math.imul(h, 1274126177);
  h = (h ^ (h >>> 16)) | 0;
  return h >>> 0;
}

/** Pick a tile texture for the given world position. Variant choice is
 *  pure-function of (x, y) so the world layout is stable across reloads
 *  and across loadTileMap calls. */
export function getTileTextureAt(
  app: Application,
  kind: TileKind,
  x: number,
  y: number,
): Texture {
  if (atlas?.has(kind)) {
    const def = atlas.defaultFor(kind);
    const features = (RARE_FEATURES[kind] ?? [])
      .map((n) => atlas!.byNameLookup(n))
      .filter((t): t is Texture => t !== null);
    const variants = (SUBTLE_VARIANTS[kind] ?? [])
      .map((n) => atlas!.byNameLookup(n))
      .filter((t): t is Texture => t !== null);

    // Roll for rare feature first (tightest band). Use salt=1 so feature
    // placement is independent of variant placement.
    if (features.length > 0) {
      const fRoll = hash2(x, y, 1) / 0xffffffff;
      if (fRoll < FEATURE_RATE) {
        const pickIdx = hash2(x, y, 2) % features.length;
        return features[pickIdx];
      }
    }
    if (variants.length > 0) {
      const vRoll = hash2(x, y, 3) / 0xffffffff;
      if (vRoll < SUBTLE_RATE) {
        const pickIdx = hash2(x, y, 4) % variants.length;
        return variants[pickIdx];
      }
    }
    if (def) return def;
  }
  return placeholderTexture(app, kind);
}

/** Autotile lookup: at boundary cells where the tile's kind differs
 *  from one or more neighbors, return a hand-crafted edge/corner tile
 *  showing the transition. Returns null if (a) no boundary, or (b) the
 *  needed edge tile isn't in the atlas — caller falls back to the plain
 *  variant picker.
 *
 *  Conventions:
 *    - <kind>_edge_top    : north neighbor is different
 *    - <kind>_edge_bottom : south neighbor is different
 *    - <kind>_edge_left   : west neighbor is different
 *    - <kind>_edge_right  : east neighbor is different
 *    - <kind>_corner_<dir>_outer : convex corner (two adjacent neighbors differ)
 *    - <kind>_corner_<dir>_inner : concave (only diagonal differs)
 *
 *  We don't autotile water against itself or grass against itself.
 *  Edge tile choice prioritizes corners over single edges. */
// Which neighbor kinds a given tile-kind's edge variants depict
// transitions to. DALL-E only painted ONE transition per kind: grass→dirt,
// dirt→grass, water→grass-shore. Using grass_edge_* against water (which
// shows a dirt strip) looks worse than no edge at all. So we only invoke
// edge tiles when the neighbor's kind matches what the edge tile draws.
const EDGE_PARTNERS: Partial<Record<TileKind, Set<TileKind>>> = {
  // Each kind's edge tiles depict ONE transition. Pair lookup is
  // asymmetric — the transition lives on whichever kind painted it.
  grass: new Set<TileKind>(["dirt"]),                  // grass_edge_* = grass→dirt
  dirt:  new Set<TileKind>(["grass"]),                 // dirt_edge_* = dirt→grass
  // water_edge_* tiles depict a sandy GRASS shore — meaningless against
  // stone or dirt (a stone bridge meeting water shouldn't show grass).
  // Restrict water transitions to grass-adjacent only.
  water: new Set<TileKind>(["grass"]),
  stone: new Set<TileKind>(["grass"]),                 // stone_edge_* = stone→grass
  path:  new Set<TileKind>(["grass"]),                 // path uses stone_edge_*
  // path/wall/floor_wood/void: no usable edge variants.
};

function partnersFor(kind: TileKind): Set<TileKind> | null {
  return EDGE_PARTNERS[kind] ?? null;
}

// Some kinds render as the same texture as another kind (path looks like
// stone). When looking up edge variants, fall back to the alias's tile
// names if the kind's own ones don't exist.
const TILE_ALIAS: Partial<Record<TileKind, TileKind>> = {
  path: "stone",
};

function lookupVariant(kind: TileKind, suffix: string): Texture | null {
  if (!atlas) return null;
  return atlas.byNameLookup(`${kind}_${suffix}`)
      ?? (TILE_ALIAS[kind]
          ? atlas.byNameLookup(`${TILE_ALIAS[kind]}_${suffix}`)
          : null);
}

export function pickEdgeTexture(
  kind: TileKind,
  x: number,
  y: number,
  kindAt: (x: number, y: number) => TileKind | null,
): Texture | null {
  if (!atlas) return null;
  const partners = partnersFor(kind);
  if (!partners) return null;
  const n = kindAt(x, y - 1);
  const s = kindAt(x, y + 1);
  const w = kindAt(x - 1, y);
  const e = kindAt(x + 1, y);
  const diffN = n !== null && n !== kind && partners.has(n);
  const diffS = s !== null && s !== kind && partners.has(s);
  const diffW = w !== null && w !== kind && partners.has(w);
  const diffE = e !== null && e !== kind && partners.has(e);
  const diffCount = (diffN ? 1 : 0) + (diffS ? 1 : 0) + (diffW ? 1 : 0) + (diffE ? 1 : 0);

  if (diffCount === 0) return null;

  // Try outer corners first (two adjacent neighbors differ). Two
  // naming conventions are tried per direction:
  //   <kind>_corner_<dir>_outer  — grass uses this (has both inner+outer)
  //   <kind>_corner_<dir>        — water uses this (no inner/outer split)
  const tryCorner = (dir: string): Texture | null => {
    return lookupVariant(kind, `corner_${dir}_outer`)
        ?? lookupVariant(kind, `corner_${dir}`);
  };
  if (diffN && diffE) {
    const t = tryCorner("ne");
    if (t) return t;
  }
  if (diffN && diffW) {
    const t = tryCorner("nw");
    if (t) return t;
  }
  if (diffS && diffE) {
    const t = tryCorner("se");
    if (t) return t;
  }
  if (diffS && diffW) {
    const t = tryCorner("sw");
    if (t) return t;
  }

  if (diffN) { const t = lookupVariant(kind, "edge_top");    if (t) return t; }
  if (diffS) { const t = lookupVariant(kind, "edge_bottom"); if (t) return t; }
  if (diffW) { const t = lookupVariant(kind, "edge_left");   if (t) return t; }
  if (diffE) { const t = lookupVariant(kind, "edge_right");  if (t) return t; }
  return null;
}

/** Legacy entry point: returns the plain default. Kept for callers that
 *  don't know world coords (mini-map swatches, palette UI). */
export function getTileTexture(app: Application, kind: TileKind): Texture {
  if (atlas?.has(kind)) {
    const def = atlas.defaultFor(kind);
    if (def) return def;
  }
  return placeholderTexture(app, kind);
}

function placeholderTexture(app: Application, kind: TileKind): Texture {
  if (placeholderCache === null) placeholderCache = new Map();
  const hit = placeholderCache.get(kind);
  if (hit !== undefined) return hit;

  const g = new Graphics()
    .rect(0, 0, TILE_SIZE_PX, TILE_SIZE_PX)
    .fill(TILE_COLORS[kind]);
  const darker = ((TILE_COLORS[kind] & 0xfefefe) >> 1);
  g.rect(0, 0, TILE_SIZE_PX, 1).fill(darker);
  g.rect(0, 0, 1, TILE_SIZE_PX).fill(darker);
  const tex = renderToTexture(app.renderer as Renderer, g);
  g.destroy();
  placeholderCache.set(kind, tex);
  return tex;
}

function renderToTexture(renderer: Renderer, g: Graphics): Texture {
  const rt = RenderTexture.create({
    width: TILE_SIZE_PX,
    height: TILE_SIZE_PX,
    resolution: 1,
  });
  renderer.render({ container: g, target: rt });
  return rt;
}

export function resetTileCache(): void {
  if (placeholderCache !== null) {
    for (const tex of placeholderCache.values()) tex.destroy(true);
    placeholderCache = null;
  }
  atlas = null;
}

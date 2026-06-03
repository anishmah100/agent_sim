// Placeholder tile textures.
//
// Until real art lands, we render tiles as palette-aligned solid-color
// quads. Generated once via Pixi.Graphics → Texture, then reused on the
// @pixi/tilemap layer.
//
// When real spritesheets arrive (via art/intake.py + the build_atlas
// step), this file becomes a 5-line `loadAtlas()` call instead. The
// rest of the renderer stays the same.

import { Application, Graphics, RenderTexture, Texture, type Renderer } from "pixi.js";

/** Logical tile categories the engine can place. Maps 1:1 with the
 *  in-house tile JSON format (see worlds/dev_test.json). Mirrors the
 *  scenario's terrain enum. */
export type TileKind =
  | "grass"
  | "dirt"
  | "path"
  | "water"
  | "stone"
  | "wall"
  | "floor_wood"
  | "void";

/** Palette-aligned color for each tile kind. From art/style.json's
 *  Endesga 32 palette. Used both as placeholder fill and for a future
 *  minimap pixel color. */
const TILE_COLORS: Record<TileKind, number> = {
  grass:      0x63c74d,    // Endesga light green
  dirt:       0x733e39,    // Endesga warm brown
  path:       0xb86f50,    // Endesga tan
  water:      0x0099db,    // Endesga blue
  stone:      0x8b9bb4,    // Endesga gray
  wall:       0x3a4466,    // Endesga dark blue-gray
  floor_wood: 0xc28569,    // Endesga warm tan
  void:       0x181425,    // Endesga near-black
};

export const TILE_SIZE_PX = 16;

/** Cache so we don't re-render the same tile texture per tile. Keyed
 *  by TileKind. Built lazily on first request. */
let cache: Map<TileKind, Texture> | null = null;

/** Get or build the placeholder texture for a tile kind. Each texture
 *  is exactly TILE_SIZE_PX square and uses one Endesga 32 color. The
 *  textures live as long as the Pixi Application — they're freed when
 *  the app is destroyed (so the Map is also reset). */
export function getTileTexture(app: Application, kind: TileKind): Texture {
  if (cache === null) cache = new Map();
  const hit = cache.get(kind);
  if (hit !== undefined) return hit;

  const g = new Graphics()
    .rect(0, 0, TILE_SIZE_PX, TILE_SIZE_PX)
    .fill(TILE_COLORS[kind]);

  // A small 1px-darker border per tile reads as a grid at full zoom,
  // and disappears (subpixel) at low zoom — exactly what we want for a
  // placeholder so adjacent tiles of the same kind are visually
  // separable while we iterate.
  const darker = ((TILE_COLORS[kind] & 0xfefefe) >> 1);
  g.rect(0, 0, TILE_SIZE_PX, 1).fill(darker);
  g.rect(0, 0, 1, TILE_SIZE_PX).fill(darker);

  const tex = renderToTexture(app.renderer as Renderer, g);
  g.destroy();
  cache.set(kind, tex);
  return tex;
}

/** Render a Graphics into a RenderTexture, returning the readable
 *  Texture handle. Helper isolated so we can swap to a different
 *  pipeline (e.g. SpriteSheet frames) without changing call sites. */
function renderToTexture(renderer: Renderer, g: Graphics): Texture {
  const rt = RenderTexture.create({
    width: TILE_SIZE_PX,
    height: TILE_SIZE_PX,
    resolution: 1,                       // tiles are 1:1 pixel — camera handles scale
  });
  renderer.render({ container: g, target: rt });
  return rt;
}

/** Reset the cache. Call when the Pixi Application is destroyed —
 *  RenderTextures become invalid against a destroyed renderer. */
export function resetTileCache(): void {
  if (cache === null) return;
  for (const tex of cache.values()) tex.destroy(true);
  cache = null;
}

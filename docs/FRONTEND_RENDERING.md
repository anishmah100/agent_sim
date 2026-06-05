# Frontend Rendering — Architecture

How the PixiJS frontend renders the 1500×1500 Eldoria world at 60 fps
on commodity hardware. The naïve "one Pixi Sprite per tile" approach
needs 2.25 M sprites — would freeze any GPU. We don't do that.

## Layer pipeline

```
Stage
└── pixi-viewport (camera, drag/zoom)
    ├── tilemap.container        ← terrain (chunked render-textures)
    ├── decorations.container    ← trees, buildings, props (viewport-culled)
    ├── entities.container       ← NPCs + world-objects (snapshot-driven)
    ├── speechBubbles.container  ← speech / shout overlays
    ├── fxAbove                  ← particle effects + day/night tint
    └── (HD2D bloom stack — gated behind VITE_ENABLE_HD2D=1)
        InteriorLayer            ← fixed-position; sits OUTSIDE the viewport
```

Filters live on the viewport so they tint the entire world layer in
one pass, never per-sprite.

## Tilemap — chunked render-textures (`render/Tilemap.ts`)

The tilemap grid is parsed into a `TileKind[][]` array on load, but no
Pixi sprites are created up front. Instead:

1. The world is divided into 32×32-tile **chunks** (= 512 px in world
   coords).
2. Every frame, the ticker computes which chunks intersect the
   camera's visible rect + a 2-chunk padding ring.
3. Chunks not yet baked are rendered into a 512×512 `RenderTexture`
   exactly once: 1024 tile-sized Sprites composited into the RT, the
   RT then displayed as a single Sprite at the chunk's world position.
4. Up to `CHUNK_BAKES_PER_FRAME = 16` chunks bake per frame so a fast
   zoom-out catches up in a frame or two.
5. Cached chunks live in an LRU map capped at `CHUNK_CACHE_MAX = 128`
   entries (≈128 MB GPU memory at 4 bytes/pixel). Distance-from-camera
   ordering means the closest chunks bake first (no left-to-right
   "snake" front).
6. A **LOD backdrop** — a 1500×1500 canvas painted with one
   biome-coloured pixel per tile — sits behind the detail chunks at
   z=0, so any not-yet-baked area still reads as grass/sand/water
   rather than black.

Pan latency: 21 ms / frame mean (vs 480 ms with the per-tile-sprite
approach). Verified in `frontend/tests/_bench_drag.mjs`.

## Decorations — viewport culling + spatial bucket (`render/Decoration.ts`)

19 k+ decoration specs live in memory but only the ~30-100 currently
inside the viewport are materialised as Pixi sprites.

- Specs are bucketed into 64-tile cells at load time.
- Every refresh: compute the visible rect, look up the buckets that
  touch it, filter the candidates by exact intersection, diff against
  the currently-materialised set, add/remove sprites accordingly.
- A per-frame creation budget caps the cost of zooming into a dense
  forest at `DECOS_PER_FRAME = 80` new sprites. Excess defer to the
  next frame.
- Shadow `Graphics` is only added for buildings (footprint ≥ 2);
  vegetation skips the shadow to halve sprite construction cost.

## Entities — snapshot-driven (`render/Entity.ts`)

The viewer WS stream broadcasts `WorldSnapshot.Entities` 30 times per
second. Each frame the entity layer reconciles:

- Incoming IDs not in `items` → create + add to container.
- Existing IDs → update position / facing / sprite frame.
- IDs no longer in incoming → destroy.

Two render paths:

- **Agent archetypes** — character-frame sprites from `CharacterAtlas`
  with a walk-cycle animation driven by `WalkProgress`.
- **World-object archetypes** (tree, rock, blueprint, item) —
  static sprites picked by `worldObjectSpriteUrl(state)`.

The animation tick is in JS (CPU) — Pixi just draws the chosen frame.
Each entity layer tick maintains a stable Map (not a re-create), so
1000 entities ≈ 1000 cheap Map operations per frame.

## Minimap — pre-baked (`ui/Minimap.tsx`)

The minimap canvas is 200×150 px. Painting 2.25 M `fillRect` calls
into it five times per second froze the main thread (480 ms / frame).
Instead:

- On first draw, the tile grid is rendered once into an offscreen
  canvas, with run-length packing of adjacent same-colour spans so a
  forest row becomes one fillRect, not 600.
- The cache is keyed by `${worldW}x${worldH}-${tiles.length}-${minimapW}x${minimapH}`
  so a world swap rebuilds it lazily.
- The live canvas only blits the baked layer + draws ~30 entity dots
  + the viewport rectangle outline. Per-frame cost: < 1 ms.

## Tile atlas (`render/TileAtlas.ts`)

A small JSON manifest (`art/manifests/overworld_tileset.json`) lists
every tile texture + the per-kind defaults (grass → grass tile, path →
stone tile, etc.). Loaded once at boot in parallel via
`Promise.allSettled`. Until the catalog refactor lands, the variant
+ autotile-edge logic stays in `render/tiles.ts`.

## Filter stack

- `DayNight` — a single `ColorMatrixFilter` driven by a 15-minute
  cycle. Cheap (matrix multiply per pixel).
- `HD2DStack` — saturation bump + AdvancedBloom. Disabled by default
  because the bloom is multi-pass and dominated pan-frame budget at
  1500×1500. Re-enable with `VITE_ENABLE_HD2D=1` for screenshots /
  small worlds.

## Verified performance

- Pan benchmark (`frontend/tests/_bench_drag.mjs`): mean **21 ms** /
  frame across a 4 s diagonal sweep of 800 tiles.
- 1000-bot soak: engine holds 60 Hz, viewer WS delivers snapshots at
  30 Hz, no GPU stalls observed on Intel iGPU + integrated WSL display.
- Memory: ~200 MB JS heap, ~150 MB GPU at fully-explored cache cap.

## Future work

- **`ArtCatalog`** — single manifest replacing the scattered sprite-URL
  resolvers (see `docs/CLEANUP_PLAN.md`).
- **Chunk pre-bake on world load** — background-bake all chunks within
  N of spawn during the splash screen. Currently lazy-bake only.
- **WebGPU** — Pixi v8 will support it; would compress the filter
  stack significantly and remove the SwiftShader software-fallback risk
  on headless Linux.

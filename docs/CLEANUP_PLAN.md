# Cleanup Plan — June 2026

The productionization pass kicked off after we got Eldoria + Phase A
scaling working end-to-end. Scope: docs, code cleanliness, test
coverage, and a real refactor of the art-loading pipeline so new
sprite sheets stop requiring scattered code edits.

## What this pass fixes

| Area | Pain today | Target state |
|---|---|---|
| **Art directory** | 82 loose PNGs at `art/processed/` root, master sheets + sliced versions duplicated, parallel `raw/`+`processed/` trees | Same on-disk layout (no destructive moves), but every sprite reachable through a single declarative catalog |
| **Sprite URL resolution** | 7 separate resolvers in frontend (Decoration.spriteUrl, Entity.worldObjectSpriteUrl, Interior.TILE/LEGACY_TILE/PROP/PROP2W, TileAtlas, tiles.ts, CharacterAtlas) — each with hardcoded path templates + allowlists | Single `ArtCatalog.url(id)` / `meta(id)`; resolvers delegate; allowlists move to manifest data |
| **Hardcoded allowlists** | `V2_BUILDING_NAMES`, `enterableSprites`, `SHEET_PROPS`, `isProp2W`, `CHARACTER_ROTATION`, `SUBTLE_VARIANTS`, `RARE_FEATURES`, `EDGE_PARTNERS` scattered across 4 files | Properties on catalog entries (e.g. `enterable: true`, `interior_template: cottage`, `tile_variants: [...]`) |
| **Adding a new sprite** | Drop PNG → edit 4–7 code locations to wire it up | Drop PNG → add one entry to `sprites.json` → automatically rendered + (optionally) clickable + (optionally) used as a tile variant |
| **Phase A tests** | `queue.go`, `snapshot.go`, `wire/agent.go` have zero coverage | Focused unit tests for each |
| **Stale docs** | LAUNCH_CHECKLIST.md flags Phase A as pending, CLAUDE.md/README.md reference dev_test as default, no design doc for Eldoria or chunked rendering | All shipped work documented + checklist current |
| **Ad-hoc scripts** | 11 `_screenshot_*` / `_debug_*` / `_diag_*` / `_bench_*` files in `frontend/tests/` | Real tests stay; ad-hoc diagnostic scripts move to `tools/dev-scripts/` or get deleted |
| **Open bug** | `engine/internal/systems/construction/construction.go:48` has duplicate JSON tag `"footprint_w"` (should be `"footprint_h"`) | Fixed |

## The art-loading refactor — design

### `art/manifests/sprites.json` — single source of truth

```jsonc
{
  "$schema": "agent_sim/art/sprites/v1",
  "categories": {
    "bld": { "label": "Building",       "default_height_tiles": 4   },
    "veg": { "label": "Vegetation",     "default_height_tiles": 2   },
    "fx":  { "label": "Effect",         "default_height_tiles": 1   },
    "int": { "label": "Interior tile",  "default_height_tiles": 1   },
    "prop":{ "label": "Interior prop",  "default_height_tiles": 1.5 },
    "item":{ "label": "Item",           "default_height_tiles": 1   },
    "char":{ "label": "Character",      "default_height_tiles": 1.5 }
  },
  "sprites": {
    "bld:000": {
      "path": "objects/buildings/obj_000.png",
      "label": "Red-roof cottage",
      "kind":  "house",
      "native_size_px": [392, 299],
      "footprint_tiles": [5, 2],
      "render_height_tiles": 4,
      "enterable": true,
      "interior_template": "cottage"
    },
    "bld:blacksmith": {
      "path": "v2_blacksmith.png",
      "label": "The Forge",
      "kind":  "smithy",
      "native_size_px": [1209, 1236],
      "footprint_tiles": [3, 2],
      "render_height_tiles": 3.5,
      "enterable": true,
      "interior_template": "blacksmith"
    },
    "veg:tree_oak": {
      "path": "v2_resources_world_master/tree_oak.png",
      "kind": "tree",
      "native_size_px": [173, 295]
    },
    "stall:red_bread": {
      "path": "v2_market_stall/stall_red_bread_open.png",
      "kind": "market_stall",
      "native_size_px": [301, 304]
    }
    /* … */
  }
}
```

### Frontend `ArtCatalog`

One class loaded at boot (next to `CharacterAtlas` + `TileAtlas`).
Public API:

```ts
class ArtCatalog {
  static load(): Promise<ArtCatalog>;
  url(id: string): string | null;
  meta(id: string): SpriteMeta | null;
  enterable(id: string): boolean;
  interiorTemplate(id: string): string | null;
  nativeAspect(id: string): number | null;
}
```

The resolvers downshift to thin wrappers:
- `Decoration.spriteUrl(id)` → `catalog.url(id)` (falls through to the
  legacy resolver only when the catalog has no entry — temporary safety
  net during migration; deleted once `sprites.json` covers everything).
- `Decoration.enterableSprites` → `catalog.enterable(id)`.
- `Entity.worldObjectSpriteUrl(e)` → derive the sprite id from
  `archetype` + `extras`, then `catalog.url(id)`.
- `Interior.TILE/LEGACY_TILE/PROP/PROP2W` → unified through the catalog
  using a single sprite-id namespace (`int:floor_wood_medium`,
  `prop:fireplace_lit`, etc.).

### Generator

`engine/cmd/genart_manifest/main.go` (Go for consistency with the rest
of the toolchain) walks `art/processed/`, opens each PNG with stdlib
`image/png` to read native size, and emits `sprites.json` with
sensible defaults. Hand-tuning lives in a sibling `sprites.overrides.json`
that the generator merges in — that way a regenerate doesn't blow away
custom `enterable`/`interior_template`/`footprint_tiles` decisions.

Then `art/manifests/sprites.json` is checked in; the generator is
re-run whenever the on-disk file set changes.

### Backwards compat

Until every resolver is migrated and verified:

1. Catalog returns `null` for sprite IDs not yet in it.
2. Old resolvers check `catalog.url(id)` first; if null, fall through
   to current code.
3. Migration order (low-risk first):
   - **Phase 2.1** Decorations — Decoration.spriteUrl + enterableSprites
   - **Phase 2.2** Entities — Entity.worldObjectSpriteUrl
   - **Phase 2.3** Interior — Interior.TILE / PROP / PROP2W / LEGACY_TILE
   - **Phase 2.4** Tile atlas + CharacterAtlas integration (long tail)
4. After each phase: screenshot key regions, diff against pre-migration
   to confirm identical render output.

## Execution order

| # | Task | Risk | Verification |
|---|---|---|---|
| 1 | Fix `construction.go:48` JSON tag bug | low | `go test ./engine/internal/systems/construction/...` |
| 2 | Refresh LAUNCH_CHECKLIST.md, CLAUDE.md, README.md (Phase A done, Eldoria default) | none | re-read each doc |
| 3 | Write `engine/cmd/genart_manifest/` + emit `art/manifests/sprites.json` | low | manifest has every existing sprite ID; lints clean |
| 4 | Implement `ArtCatalog.ts` + load it from `PixiApp.tsx` (no resolvers migrated yet) | low | console shows `art catalog loaded: N sprites`; no errors |
| 5 | Migrate `Decoration.spriteUrl` + `enterableSprites` to catalog | medium | screenshot diff Crossroads, Pinewood, Saltport vs baseline; click each building variant |
| 6 | Migrate `Entity.worldObjectSpriteUrl` | medium | screenshot diff; tree + rock + blueprint entities render |
| 7 | Migrate `Interior.TILE/PROP/PROP2W` | medium | open each enterable building, screenshot interior, compare |
| 8 | Add unit tests for `queue.go` (FIFO ordering, queue_full reject, reply channel) | low | `go test ./engine/internal/world/...` |
| 9 | Add unit tests for `snapshot.go` (lock-free read, immutability, spatial index correctness) | low | tests pass |
| 10 | Add unit tests for `wire/agent.go` SpawnAgentEntity + register flow | medium | tests pass |
| 11 | Move/delete ad-hoc frontend tests into `tools/dev-scripts/` (keep e2e_join_agent + ui_smoke) | low | nothing imports them |
| 12 | Write `docs/ELDORIA_WORLD_DESIGN.md` + extend SYSTEM_ARCHITECTURE_V2.md with frontend-rendering section | none | read-through |
| 13 | 100-bot soak on Eldoria — verify ≥48 Hz tick rate with the new catalog active | high | soak harness reports PASS |

Commit after each numbered step.

## What this pass deliberately does NOT do

- **Does not rename existing PNG files or move them between directories.**
  Risk is high (every URL would change), and the catalog gives us a
  rename-friendly indirection layer for later if we want one.
- **Does not delete the master sheet PNGs at `art/processed/` root.**
  They're sources of truth that the slice scripts read; just no longer
  referenced at render time.
- **Does not refactor the AI-art intake pipeline.** That's a separate
  pass (the art/ directory survey will inform a follow-up).
- **Does not change the Go module path again.** Already done in commit
  `3c8425b` (rebranded to `anishmah100`).

# v2 art prompts — village build-out

Second round, generated 2026-06-03. Adds the assets needed for the
village populate phase: a blacksmith, a town hall, a market-stall
sheet (more than the v1 tileset), a standalone well, and a window-glow
FX sprite for nighttime lit windows.

## Discipline (carried over from `_README.md`, do not forget)

- DALL-E will ignore dim specs. The sheet that comes back will NOT
  match the requested grid exactly. **Assume nothing about the
  structure.** Open each output in an image viewer, identify the
  actual cell boundaries, and feed the per-asset crop coordinates
  into `art/intake.py` manually.
- DALL-E will add unwanted shading. Custom snap palette + custom
  magenta threshold per sheet — `art/intake.py --palette ... --magenta-tol ...`
- Output saves to `art/raw/<name>.png`. Never overwrite the v1 files.
- Per-asset post-processing notes go into `art/processed/<name>/NOTES.md`
  once intake produces sliced output, so future runs of intake
  reproduce the manual fixups.

## Prompts in this set

- `blacksmith.md` — 3×3 building, stone walls, anvil out front, chimney with smoke.
- `town_hall.md` — 5×3 building, prominent door, banner, flag pole.
- `market_stalls.md` — 1×1 stall sheet, 6 variants (different awning colors + wares).
- `well.md` — 1×1 stone well, replaces the v1 well's blocky look.
- `window_glow_fx.md` — 16×16 radial glow for lit windows at night (used as an emissive overlay).

## Save paths

| Prompt | Save raw to |
|---|---|
| blacksmith.md | `art/raw/v2_blacksmith.png` |
| town_hall.md | `art/raw/v2_town_hall.png` |
| market_stalls.md | `art/raw/v2_market_stalls.png` |
| well.md | `art/raw/v2_well.png` |
| window_glow_fx.md | `art/raw/v2_window_glow.png` |

The `v2_` prefix on raw files lets intake distinguish v1 vs v2 source
sheets without naming collisions.

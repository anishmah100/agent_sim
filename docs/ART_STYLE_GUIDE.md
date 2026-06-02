# ART STYLE GUIDE

The single source of truth for every pixel in the world. If an image doesn't conform to this guide, it doesn't ship.

## Reference target

**Pokémon HeartGold / SoulSilver (DS, 2009)** — top-down with slight 3/4 perspective. Buildings show their front face; characters draw with bottom-center anchor. Bright readable palette. Crisp pixels. Clear silhouettes.

Secondary references for specific concerns:
- **Stardew Valley** — interior room layouts, item iconography.
- **Tiny Swords / Tiny Town (Pixel Frog, CC0)** — fantasy tile mood, vegetation variety.
- **CrossCode** — animated tile shimmer for water/grass.

We will pin one specific reference screenshot per asset class (`art/references/`) before generating. Every generation is judged against the relevant reference.

## §1 — Tile system

- **Tile size**: 16×16 px (hard, no other size accepted).
- **Display scale**: rendered at 3× nominal (so each tile is 48 device px at zoom 1.0). The browser nominal zoom range is 0.5×–3× of that.
- **Pixel-perfect rendering**: PixiJS `RESOLUTION = window.devicePixelRatio`, `roundPixels = true`. No anti-aliasing of pixel art.
- **Grid alignment**: every tile must occupy exactly 16×16. No half-tile sprites in the tileset.

### Tile categories
| Category | Examples |
|---|---|
| Ground | grass (4 variants), dirt, sand, stone, wood floor, marble floor |
| Path | dirt path, stone path, brick path (all autotiling) |
| Water | shallow water, deep water (animated, 4-frame shimmer) |
| Cliff | grass-edge, cliff-side, cliff-top |
| Wall | wood wall, stone wall (autotile edges) |
| Vegetation | tree (oak, pine), bush (small, berry), flowers (yellow, red, blue), tall grass |
| Furniture | bed, table, chair (4 facing), barrel, crate, sign |
| Structures | door, fence, gate, well, lamp post |

All edge/corner tiles for autotiling are sliced from a **47-tile blob autotile** layout (standard pixel-art autotile pattern). LDtk handles the autotile rule definition; we just provide the 47 sliced tiles per terrain type.

## §2 — Character system

- **Sprite dims**: 16×24 px (16 wide, 24 tall — head extends 8 px above the tile footprint).
- **Anchor point**: bottom-center (x=8, y=23). Feet stand on tile center.
- **Color budget**: each character uses ≤16 colors from the palette (8 colors per layer × 2 layers).
- **Layer composition (per character)**:
  1. Base body (skin + clothes)
  2. Outfit overlay (armor, robe, apron, etc.) — separately rendered for variation
  3. Hair (separately rendered for variation)
  4. Accessory (hat / weapon / pack) — optional

This layered approach lets us hand-author a small character cast but produce many variations by mixing layers.

### Character animation states

Every character ships with these animation strips:

| State | Frames | Strip dims |
|---|---|---|
| `idle` | 1 frame per direction (N/S/E/W) × subtle 2-frame breathing | 16×24 × 4 directions × 2 = 128×24 |
| `walk` | 4 frames per direction (down/up/left/right) | 64×24 × 4 directions = 64×96 |
| `attack` | 4 frames, one direction at a time (default down) | 64×24 |
| `hit` | 2 frames | 32×24 |
| `death` | 4 frames | 64×24 |
| `interact` | 2 frames | 32×24 |
| `carry` | 4-frame walk variant with item visible | 64×24 |

A standard character sheet is 128×144 px assembled like:

```
+---------------+---------------+
|  walk_down    |   walk_up     |    (64×24 each)
+---------------+---------------+
|  walk_left    |   walk_right  |
+---------------+---------------+
|  attack       |  hit | death  |
+---------------+---------------+
| interact      |  carry        |
+---------------+---------------+
```

We slice this in `art/intake.py`. ChatGPT generates the full sheet; we cut into frame metadata.

## §3 — Palette

We use a **fixed 32-color palette** (candidate: Endesga 32). Every asset's pixels are quantized to this palette at intake. Off-palette colors snap to nearest.

Why 32 fixed: this is what HeartGold-tier pixel art uses. More colors = harder to keep cohesive. Fewer = monotonous. 32 is the sweet spot for AAA pixel art.

Final palette will be committed to `art/style.json` after we lock the visual anchor (Milestone 0).

## §4 — Buildings

- **Façade-visible 3/4 lean**: each building is drawn with its front face showing (not as a flat rooftop). Sprites are taller than they are wide for multi-story buildings.
- **Building sprite dims**: variable, multiples of 16×16. A small hut might be 48×64 (3×4 tiles). A big tavern might be 96×112 (6×7 tiles).
- **Door tile**: a 16×16 tile at the building's entry point, treated as a portal by the engine (walking onto it triggers an interior-map load).
- **Roof variation**: separate sprite from body for night-time lighting changes (lit windows on body, dark roof).

Buildings are placed in LDtk as multi-tile entities (not as floor-tile patterns). The entity carries `portal_target_map=<interior_id>` and `portal_target_spawn=<spawn_point>` fields.

## §5 — Item iconography

Items have two representations:

- **World sprite**: 16×16 px (how an item looks lying on the ground).
- **Icon sprite**: 16×16 px (how it appears in inventory UI).

For most simple items, world and icon are the same sprite. For containers or compound items (bag full of gold), they may differ.

Item categories for the fantasy v1:
- Food (apple, bread, cheese, fish)
- Tools (axe, pickaxe, fishing rod, hammer)
- Weapons (sword, dagger, bow, staff)
- Armor (helmet, chest, boots, shield)
- Resources (wood, stone, ore, herbs)
- Money (gold coin pile, gem)
- Misc (lantern, scroll, potion)

## §6 — Generation pipeline

### Prompt template (concise but specific)

```
Generate a {asset_class} spritesheet, top-down 3/4 perspective, pixel art,
{tile_or_sprite_dims} per cell. Layout: {layout_description}.

Style reference: Pokémon HeartGold / SoulSilver visual style. Crisp pixels,
no anti-aliasing on outlines, clear silhouettes.

Palette: 32 fixed colors — use {palette_summary} (see attached reference).

Background: pure magenta (#FF00FF) for transparency keying.

{asset_specific_instructions}
```

`{asset_specific_instructions}` is per asset class — e.g. for a character walk
sheet: "4 walk frames × 4 cardinal directions, anchored bottom-center,
each frame 16w × 24h, frame 0 = mid-step left foot forward, frame 1 = both
feet planted, frame 2 = mid-step right foot forward, frame 3 = both feet
planted (mirror of frame 1)."

### Intake script (`art/intake.py`)

For every generated image:

1. **Read**: open with Pillow.
2. **Dim check**: dimensions match expected for that asset class.
3. **Magenta key**: convert `#FF00FF` pixels to alpha=0.
4. **Halo cleanup**: detect and remove anti-aliased halo pixels around alpha edges.
5. **Palette quantize**: snap every remaining pixel to the nearest palette color. Log how many pixels changed (>5% = generation outlier, manual review needed).
6. **Frame slice**: cut into individual frames per the layout spec.
7. **Atlas pack**: feed approved frames into `art/build_atlas.py` to produce the final texture atlas.

If any step fails or flags excessive drift, the image is rejected to `art/rejected/<reason>/`. The asset goes back for regeneration or replacement.

### Manual touch-up

For hero assets (the 5–10 named characters; the main town buildings), if AI gen is close-but-not-quite, we may use **Aseprite** for hand correction. We track which assets were touched in `art/manifest.toml` so we can re-source them if we ever need to regenerate.

For mass assets (background NPCs, generic forageables), no manual touch-up — they either pass the gate or we replace them with bought tiles.

## §7 — Building the atlas

Output is **one texture atlas per category** (one for tiles, one for characters, one for items, one for buildings, one for FX). PixiJS loads each as a single `Spritesheet` resource. The JSON metadata lists every named frame.

This lets us:
- Reduce texture binds (faster draw calls).
- Hot-reload art without recompiling the engine.
- Version atlases so cache invalidation is automatic.

## §8 — UI iconography (for the chrome layer)

The DOM overlay uses a separate icon set:
- Buttons, scrollbars, modal chrome: rendered as styled HTML, NOT pixel art (Kobalte + a pixel-art-themed CSS).
- In-UI iconography (HP heart, gold coin, inventory slot border) IS pixel art — same 16×16 tile style, separate atlas.

The UI is themed to feel cohesive with the pixel world without being pixel-rendered itself. Crisp text (anti-aliased modern font for readability) + pixel borders / icons.

## §9 — Locking the anchor

**Before any sprite is generated for the v1 world**, Milestone 0 produces:

- `art/references/grass_tile.png` — the one perfect grass tile
- `art/references/oak_tree.png` — the one perfect tree
- `art/references/character_template.png` — the one perfect base character
- `art/references/building_house.png` — the one perfect small house
- `art/references/heartgold_sample.png` — a captured HeartGold screen for comparison

These are the visual anchors. Every subsequent generation is judged against them.

When you (the maintainer) see these and approve them, the style is locked.

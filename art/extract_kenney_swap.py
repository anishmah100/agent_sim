"""Extract Kenney CC0 tiles + replace our DALL-E defaults.

Kenney's Roguelike-RPG pack ships a 968×526 master sheet with 16×16 tiles
on a 1px grid. The top-left has autotile-ready water/grass terrain; the
right side has trees, houses, and props.

This script:
  1) Picks specific (col, row) tile positions on the Kenney sheet for
     each kind we render (grass/water/dirt/stone, edges, corners).
  2) Extracts them at SOURCE resolution (16×16) and writes to
     processed/tiles/overworld/<name>.png — same filenames our atlas
     already loads. So nothing in the frontend needs changing.
  3) Replaces our DALL-E tree/bush sprites with Kenney equivalents.

Each tile is at sheet pixel (1 + col*17, 1 + row*17), 16×16 wide.

To revert: re-run extract_overworld_tiles.py which extracts from raw/.
"""

from __future__ import annotations

from pathlib import Path
from PIL import Image

ART = Path(__file__).resolve().parent
SHEET = ART / "external" / "kenney_rpg" / "Spritesheet" / "roguelikeSheet_transparent.png"
TILES_OUT = ART / "processed" / "tiles" / "overworld"
VEG_OUT = ART / "processed" / "objects" / "vegetation"


# === Kenney terrain mapping (col, row) on the master sheet ===
# Identified visually from the top-left autotile block. The water block
# at (cols 3-5, rows 0-2) has sand shoreline transitions; we use it
# directly as our water+edge+corner set.
TERRAIN_TILES: dict[str, tuple[int, int]] = {
    # Solid ground (row 0 of sheet)
    "grass":              (5, 0),   # solid green
    "grass_tuft":         (5, 1),   # grass with tuft variation
    "grass_pebble":       (5, 0),
    "grass_mushroom":     (5, 0),
    "dirt":               (6, 0),   # brown
    "dirt_cracked":       (6, 1),
    "dirt_pebbles":       (6, 0),
    "stone":              (6, 2),   # gray cobblestone (used for plaza + bridge)
    "stone_dark_brick":   (6, 2),
    "stone_grate":        (6, 2),

    # Water + shoreline 3×3 autotile block at cols 2-4, rows 0-2.
    # Center = pure water; edges = water with grass shore.
    "water":              (3, 1),   # center water
    "water_ripple":       (3, 1),
    "water_lily":         (3, 1),
    "water_rock":         (3, 1),
    "water_edge_top":     (3, 0),
    "water_edge_bottom":  (3, 2),
    "water_edge_left":    (2, 1),
    "water_edge_right":   (4, 1),
    "water_corner_nw":    (2, 0),
    "water_corner_ne":    (4, 0),
    "water_corner_sw":    (2, 2),
    "water_corner_se":    (4, 2),

    # Kenney handles grass-side edge with its water-side autotile,
    # so for "grass next to water/dirt" we just use plain grass —
    # the transition lives on the OTHER kind's tile.
    "grass_edge_top":     (5, 0),
    "grass_edge_bottom":  (5, 0),
    "grass_edge_left":    (5, 0),
    "grass_edge_right":   (5, 0),
    "grass_corner_ne_inner": (5, 0),
    "grass_corner_ne_outer": (5, 0),
    "grass_corner_nw_inner": (5, 0),
    "grass_corner_nw_outer": (5, 0),
    "grass_corner_se_inner": (5, 0),
    "grass_corner_se_outer": (5, 0),
    "grass_corner_sw_inner": (5, 0),
    "grass_corner_sw_outer": (5, 0),
    "dirt_edge_top":      (6, 0),
    "dirt_edge_bottom":   (6, 0),
    "dirt_edge_left":     (6, 0),
    "dirt_edge_right":    (6, 0),
    "stone_edge_top":     (6, 2),
    "stone_edge_bottom":  (6, 2),
    "stone_edge_left":    (6, 2),
    "stone_edge_right":   (6, 2),
    "stone_edge_fade":    (6, 2),
}


# === Kenney vegetation (replacement for our trees) ===
# Trees are around (cols 12-16, rows 7-9). Each tree is a multi-tile
# sprite — Kenney's pine tree is 2x3 tiles. We extract the full bbox.
VEGETATION_TILES: dict[str, tuple[int, int, int, int]] = {
    # name -> (col, row, w_tiles, h_tiles).
    # Kenney pines are 1-tile wide × 2-tile tall: foliage on top row,
    # trunk on bottom row.
    "veg_000_pine":      (12, 10, 1, 2),   # green pine
    "veg_004_pine_dark": (15, 10, 1, 2),   # dark green pine
    "veg_008_bush":      (12, 9, 1, 1),    # green round bush
    "veg_009_bush2":     (11, 9, 1, 1),    # green bush variant
    "veg_010_small_bush":(15, 9, 1, 1),    # smaller bush
    "veg_036_mushroom":  (3, 7, 1, 1),     # tiny dirt patch / pebble accent
}


def extract_tile(sheet: Image.Image, col: int, row: int, w: int = 1, h: int = 1) -> Image.Image:
    px = 1 + col * 17
    py = 1 + row * 17
    return sheet.crop((px, py, px + 16 * w, py + 16 * h))


def main() -> None:
    sheet = Image.open(SHEET).convert("RGBA")
    TILES_OUT.mkdir(parents=True, exist_ok=True)
    VEG_OUT.mkdir(parents=True, exist_ok=True)

    for name, (col, row) in TERRAIN_TILES.items():
        tile = extract_tile(sheet, col, row)
        out = TILES_OUT / f"{name}.png"
        tile.save(out)

    print(f"wrote {len(TERRAIN_TILES)} terrain tiles to {TILES_OUT}")

    # NOTE: Vegetation is NOT overwritten here. We keep the painterly
    # DALL-E vegetation sprites in processed/objects/vegetation/ since
    # Kenney's 1-tile pines are too small for HG-style canopy. Restore
    # DALL-E veg sprites via extract_vegetation.py.


if __name__ == "__main__":
    main()

"""Make terrain tiles seamlessly tileable.

GPT-generated tiles look right standalone but show visible vertical/
horizontal seams when placed adjacent to copies of themselves — the
left-edge column doesn't match the right-edge column. Same for top/
bottom.

Approach per tile, in two passes:

  Horizontal pass (kills vertical seams between L-R neighbors):
    For an N-pixel-wide band on each side, replace both left and right
    bands with the average — they become identical. When the tile
    repeats, the right edge of tile A perfectly matches the left edge
    of tile B (they're literally the same pixels).

  Vertical pass (kills horizontal seams between top-bottom neighbors):
    Same with top/bottom bands.

Direction-specific tiles (edges, corners) get only the AXIS that doesn't
contain their transition. For example dirt_edge_top has a transition
strip at the TOP — we don't want to blend its top into its bottom (that
would smear the grass into the dirt). We only blend left↔right for
those tiles.

Run from agent_sim/art:
  python make_tiles_seamless.py
"""

from __future__ import annotations

from pathlib import Path

import numpy as np
from PIL import Image

ART = Path(__file__).resolve().parent
TILES = ART / "processed" / "tiles" / "overworld"

# Blend band size in pixels (each side). 6 is enough to hide a hard
# vertical seam for tiles where the texture frequency is high.
BAND = 14   # wider band hides residual seams more reliably


def blend_left_right(arr: np.ndarray, band: int) -> np.ndarray:
    """Make the tile's left edge match its right edge.

    Replace both edges' bands with the AVERAGE of corresponding
    left-column and right-column pixels. Result: column 0 == column W-1,
    column 1 == column W-2, etc. (modulo the rest of the tile)."""
    h, w, _ = arr.shape
    out = arr.copy()
    for i in range(band):
        l = out[:, i, :].astype(np.int32)
        r = out[:, w - 1 - i, :].astype(np.int32)
        avg = ((l + r) // 2).astype(np.uint8)
        out[:, i, :] = avg
        out[:, w - 1 - i, :] = avg
    return out


def blend_top_bottom(arr: np.ndarray, band: int) -> np.ndarray:
    h, w, _ = arr.shape
    out = arr.copy()
    for i in range(band):
        t = out[i, :, :].astype(np.int32)
        b = out[h - 1 - i, :, :].astype(np.int32)
        avg = ((t + b) // 2).astype(np.uint8)
        out[i, :, :] = avg
        out[h - 1 - i, :, :] = avg
    return out


# Per-tile recipe: which axes to blend.
#   "both"   = horizontal + vertical (solid texture tiles)
#   "h"      = horizontal only (tile has a meaningful top/bottom asymmetry)
#   "v"      = vertical only (tile has meaningful left/right asymmetry)
#   "none"   = corner tiles — both axes carry the transition, leave alone
RECIPES: dict[str, str] = {
    # Solid ground tiles
    "grass.png":           "both",
    "grass_tuft.png":      "both",
    "grass_pebble.png":    "both",
    "grass_mushroom.png":  "both",
    "dirt.png":            "both",
    "dirt_cracked.png":    "both",
    "dirt_pebbles.png":    "both",
    "stone.png":           "both",
    "water.png":           "both",
    "water_ripple.png":    "both",
    # Top-edge tiles: transition is on TOP, so make L↔R seamless
    "grass_edge_top.png":    "h",
    "dirt_edge_top.png":     "h",
    "stone_edge_top.png":    "h",
    "water_edge_top.png":    "h",
    "grass_edge_bottom.png": "h",
    "dirt_edge_bottom.png":  "h",
    "stone_edge_bottom.png": "h",
    "water_edge_bottom.png": "h",
    # Side-edge tiles: transition is on LEFT or RIGHT, so make T↔B seamless
    "grass_edge_left.png":  "v",
    "dirt_edge_left.png":   "v",
    "stone_edge_left.png":  "v",
    "water_edge_left.png":  "v",
    "grass_edge_right.png": "v",
    "dirt_edge_right.png":  "v",
    "stone_edge_right.png": "v",
    "water_edge_right.png": "v",
    # Corner tiles touch BOTH axes — can't safely blend either.
}


def main() -> None:
    processed = 0
    for name, axes in RECIPES.items():
        path = TILES / name
        if not path.exists():
            print(f"  SKIP {name}: not found")
            continue
        arr = np.array(Image.open(path).convert("RGBA"))
        if axes == "both":
            arr = blend_left_right(arr, BAND)
            arr = blend_top_bottom(arr, BAND)
        elif axes == "h":
            arr = blend_left_right(arr, BAND)
        elif axes == "v":
            arr = blend_top_bottom(arr, BAND)
        Image.fromarray(arr, "RGBA").save(path)
        processed += 1
        print(f"  {name}  ({axes})")
    print(f"made {processed} tiles seamless.")


if __name__ == "__main__":
    main()

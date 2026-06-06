"""Synthesize stone-bridge tiles that meet water cleanly.

The plain stone.png tile has no concept of "I'm a bridge over water"
— so where the world places stone next to water, you see a hard
rectangular cut. This was the most obvious remaining defect in the
audit.

We don't have GPT-generated stone↔water transition tiles. Instead we
SYNTHESIZE them: take the existing stone.png and paint a darker wooden
"bridge edge plank" band along whichever side meets water. The band
visually reads as the bridge's wooden retaining edge, breaking up the
hard stone↔water boundary.

Outputs:
  stone_water_edge_left.png    — water on W, stone on E (plank on left)
  stone_water_edge_right.png   — water on E, stone on W (plank on right)
  stone_water_edge_top.png     — water on N
  stone_water_edge_bottom.png  — water on S

Not used directly by the autotile picker today; consumed by an alias
added in tiles.ts.
"""

from pathlib import Path

import numpy as np
from PIL import Image

ART = Path(__file__).resolve().parent
TILES = ART / "processed" / "tiles" / "overworld"

# Wooden plank colors — warm brown for the bridge edge.
PLANK_LIGHT = np.array([120, 76, 38], dtype=np.uint8)
PLANK_DARK = np.array([80, 50, 26], dtype=np.uint8)
SHADOW_DARK = np.array([45, 30, 18], dtype=np.uint8)


def paint_plank_band(stone: np.ndarray, side: str, band_frac: float = 0.18) -> np.ndarray:
    """Paint a wood-plank band along the requested side. Beneath the
    planks, a darker shadow strip implies the plank's underside, then
    abrupt transition to water on the next tile."""
    h, w, _ = stone.shape
    out = stone.copy()
    band = max(8, int(min(h, w) * band_frac))
    shadow = max(2, band // 4)

    if side == "top":
        for y in range(band):
            t = y / max(1, band - 1)
            color = (PLANK_LIGHT * (1 - t * 0.4) + PLANK_DARK * (t * 0.4)).astype(np.uint8)
            out[y, :, :3] = color
            out[y, :, 3] = 255
        # shadow strip just inside the band
        for y in range(band, band + shadow):
            t = (y - band) / max(1, shadow)
            color = (SHADOW_DARK * (1 - t) + stone[y, :, :3].mean(axis=0).astype(np.int32) * t).astype(np.uint8)
            out[y, :, :3] = color
    elif side == "bottom":
        for y in range(h - band, h):
            t = (h - 1 - y) / max(1, band - 1)
            color = (PLANK_LIGHT * (1 - t * 0.4) + PLANK_DARK * (t * 0.4)).astype(np.uint8)
            out[y, :, :3] = color
            out[y, :, 3] = 255
        for y in range(h - band - shadow, h - band):
            t = (h - band - 1 - y) / max(1, shadow)
            color = (SHADOW_DARK * (1 - t) + stone[y, :, :3].mean(axis=0).astype(np.int32) * t).astype(np.uint8)
            out[y, :, :3] = color
    elif side == "left":
        for x in range(band):
            t = x / max(1, band - 1)
            color = (PLANK_LIGHT * (1 - t * 0.4) + PLANK_DARK * (t * 0.4)).astype(np.uint8)
            out[:, x, :3] = color
            out[:, x, 3] = 255
        for x in range(band, band + shadow):
            t = (x - band) / max(1, shadow)
            color = (SHADOW_DARK * (1 - t) + stone[:, x, :3].mean(axis=0).astype(np.int32) * t).astype(np.uint8)
            out[:, x, :3] = color
    elif side == "right":
        for x in range(w - band, w):
            t = (w - 1 - x) / max(1, band - 1)
            color = (PLANK_LIGHT * (1 - t * 0.4) + PLANK_DARK * (t * 0.4)).astype(np.uint8)
            out[:, x, :3] = color
            out[:, x, 3] = 255
        for x in range(w - band - shadow, w - band):
            t = (w - band - 1 - x) / max(1, shadow)
            color = (SHADOW_DARK * (1 - t) + stone[:, x, :3].mean(axis=0).astype(np.int32) * t).astype(np.uint8)
            out[:, x, :3] = color
    return out


def main() -> None:
    stone_path = TILES / "stone.png"
    if not stone_path.exists():
        print("stone.png missing")
        return
    stone = np.array(Image.open(stone_path).convert("RGBA"))
    for side in ("top", "bottom", "left", "right"):
        out = paint_plank_band(stone, side)
        name = f"stone_water_edge_{side}.png"
        Image.fromarray(out, "RGBA").save(TILES / name)
        print(f"  wrote {name}")


if __name__ == "__main__":
    main()

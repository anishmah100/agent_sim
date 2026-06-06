"""Color-harmonize variant tiles so they blend with the default.

DALL-E renders the same source image with slightly inconsistent base
colors — e.g. water_rock's blue is (2, 153, 233) while water's is
(2, 183, 238). When the renderer drops water_rock on top of regular
water tiles, the seam shows.

This script:
  1) Loads each kind's default tile (water.png, grass.png, dirt.png).
  2) Computes the default's background median RGB.
  3) For each variant tile of the same kind, finds the pixels that ARE
     background (close to the variant's own background median) and
     shifts them by (default_med - variant_med) so they match.
  4) Pixels far from the variant's background (the rock, the mushroom,
     the lily) are left untouched — we only fix the background.

Run from agent_sim/art:
  python harmonize_variant_colors.py

It rewrites the tiles in place (originals are still in raw/).
"""

from __future__ import annotations

import numpy as np
from PIL import Image
from pathlib import Path

ART = Path(__file__).resolve().parent
TILES = ART / "processed" / "tiles" / "overworld"

# kind -> (default_name, [variant_names...])
GROUPS: dict[str, tuple[str, list[str]]] = {
    # ONLY harmonize variants where the SAME kind dominates the tile.
    # Edge/corner tiles are excluded because they're transitions — they
    # have ~half background and ~half foreground (different kinds), so
    # naive median-shifting corrupts whichever side isn't dominant.
    "water": ("water", ["water_ripple", "water_lily", "water_rock"]),
    "grass": ("grass", ["grass_tuft", "grass_pebble", "grass_mushroom"]),
    "dirt": ("dirt", ["dirt_cracked", "dirt_pebbles"]),
}

# Pixels within BG_TOLERANCE (euclidean RGB distance) of the variant's
# bg median count as background and get shifted. Pixels farther count
# as feature content and are left alone.
BG_TOLERANCE = 60.0


def bg_median(rgba: np.ndarray) -> np.ndarray:
    """Median RGB of the opaque background. We assume the dominant
    opaque color IS the background — true for our ground tiles where
    grass/water/dirt fills >80% of the area."""
    op = rgba[rgba[..., 3] > 200]
    if len(op) == 0:
        return np.array([0, 0, 0])
    return np.median(op[..., :3], axis=0)


def harmonize(default_path: Path, variant_path: Path) -> tuple[np.ndarray, np.ndarray, int]:
    """Shift variant's bg pixels toward default's bg color. Returns
    (variant_med, default_med, n_shifted) for logging.

    SAFETY: if the variant's median is far from the default's median
    (>120 in RGB euclidean), the variant probably doesn't have the
    expected kind as its dominant color — e.g. water_corner_se is mostly
    GRASS with a small water corner, so its median is green. In that
    case we SKIP harmonization to avoid corrupting the tile."""
    dst = np.array(Image.open(variant_path).convert("RGBA"))
    src_default = np.array(Image.open(default_path).convert("RGBA"))

    def_med = bg_median(src_default)
    var_med = bg_median(dst)
    if np.linalg.norm(def_med - var_med) > 120:
        return var_med, def_med, 0  # skip

    shift = def_med - var_med  # (3,)

    # bg mask = opaque pixels close to variant's own bg color
    opaque = dst[..., 3] > 200
    rgb = dst[..., :3].astype(np.int32)
    dist = np.linalg.norm(rgb - var_med, axis=-1)
    bg_mask = opaque & (dist < BG_TOLERANCE)

    new_rgb = rgb.copy()
    new_rgb[bg_mask] = np.clip(new_rgb[bg_mask] + shift, 0, 255).astype(np.int32)

    dst[..., :3] = new_rgb.astype(np.uint8)
    Image.fromarray(dst, "RGBA").save(variant_path)
    return var_med, def_med, int(bg_mask.sum())


def main() -> None:
    for kind, (default_name, variants) in GROUPS.items():
        default_path = TILES / f"{default_name}.png"
        if not default_path.exists():
            print(f"  SKIP {kind}: no default at {default_path}")
            continue
        print(f"[{kind}]")
        for v in variants:
            vp = TILES / f"{v}.png"
            if not vp.exists():
                print(f"  - {v}: MISSING")
                continue
            var_med, def_med, n = harmonize(default_path, vp)
            shift = (def_med - var_med).astype(int)
            print(f"  - {v:22s}  var_med={tuple(var_med.astype(int))}  →  shift={tuple(shift)}  "
                  f"(applied to {n} bg pixels)")
    print("done.")


if __name__ == "__main__":
    main()

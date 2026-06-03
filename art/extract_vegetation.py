"""Re-extract vegetation sprites from raw/tileset_vegetation.png.

Source has magenta padding between sprites. Same pipeline as the
overworld tile extractor: magenta-mask → blob-label → inset crop →
distance-transform fill → save.

We DON'T rewrite all 40 sprites by default — only the ones we use as
decorations (and any explicitly listed on the CLI).

Run:
  python extract_vegetation.py            # rewrite the canonical six
  python extract_vegetation.py obj_022    # rewrite a specific id
"""

from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
from PIL import Image
from scipy import ndimage  # type: ignore

ART = Path(__file__).resolve().parent
RAW = ART / "raw" / "tileset_vegetation.png"
OUT_DIR = ART / "processed" / "objects" / "vegetation"

MAGENTA_TOL = 80   # aggressive — anything pink-ish dies
INSET = 4
FILL_DISTANCE = 6
HALO_TOL = 100     # post-pass cleanup: any pixel whose RGB is closer to
                   # magenta than to its alpha-weighted neighbors gets zeroed.

# Canonical sprites used by the decoration layer.
CANONICAL = {
    0: "obj_000.png",   # round green tree
    1: "obj_001.png",   # orange autumn round tree
    4: "obj_004.png",   # dark green pine
    8: "obj_008.png",   # green bush
    9: "obj_009.png",   # green bush variant
    10: "obj_010.png",  # smaller bush
    36: "obj_036.png",  # mushroom cluster
}


def magenta_mask(rgba: np.ndarray) -> np.ndarray:
    """Pure magenta padding — true magenta only, killed completely."""
    r, g, b = rgba[..., 0], rgba[..., 1], rgba[..., 2]
    return (r > 255 - MAGENTA_TOL) & (g < MAGENTA_TOL) & (b > 255 - MAGENTA_TOL)


def pinkish_mask(rgba: np.ndarray) -> np.ndarray:
    """Halo pixels — opaque but pink-tinged. The padding around DALL-E
    sprites bleeds magenta into adjacent foreground pixels, leaving a
    pink rim. We detect anything where R and B both substantially exceed
    G (the magenta signature) and treat it as halo to be re-colored."""
    r = rgba[..., 0].astype(np.int32)
    g = rgba[..., 1].astype(np.int32)
    b = rgba[..., 2].astype(np.int32)
    a = rgba[..., 3]
    rb_avg = (r + b) // 2
    return (a > 100) & (rb_avg - g > 60) & (r > 120) & (b > 120)


def crop_and_fill(rgba: np.ndarray, bbox: tuple[int, int, int, int]) -> np.ndarray:
    y0, x0, y1, x1 = bbox
    y0i, x0i = y0 + INSET, x0 + INSET
    y1i, x1i = y1 - INSET, x1 - INSET
    if y1i <= y0i or x1i <= x0i:
        return rgba[y0:y1 + 1, x0:x1 + 1].copy()
    tile = rgba[y0i:y1i + 1, x0i:x1i + 1].copy()

    alpha = tile[..., 3]
    opaque = alpha > 200
    if not opaque.all():
        dist, (iy, ix) = ndimage.distance_transform_edt(  # type: ignore[misc]
            ~opaque, return_indices=True,
        )
        mask = (dist <= FILL_DISTANCE) & (~opaque)
        if mask.any():
            tile[mask, :3] = tile[iy[mask], ix[mask], :3]
            tile[mask, 3] = 255

    tile[magenta_mask(tile), 3] = 0

    # Pink halo cleanup: every pinkish pixel gets recolored to the
    # average of its non-pinkish, opaque neighbors. Iterated 3 times so
    # the fix propagates inward where halo bleed is deep.
    for _ in range(3):
        halo = pinkish_mask(tile)
        if not halo.any():
            break
        opaque_clean = (tile[..., 3] > 100) & ~halo & ~magenta_mask(tile)
        if not opaque_clean.any():
            break
        dist, (iy, ix) = ndimage.distance_transform_edt(  # type: ignore[misc]
            ~opaque_clean, return_indices=True,
        )
        tile[halo, :3] = tile[iy[halo], ix[halo], :3]
    return tile


def main() -> None:
    requested = set(sys.argv[1:])  # e.g. "obj_022.png"
    if requested:
        targets = requested
    else:
        targets = set(CANONICAL.values())

    sheet = np.array(Image.open(RAW).convert("RGBA"))
    non_mag = ~magenta_mask(sheet)
    structure = np.ones((3, 3), dtype=int)
    labels, n = ndimage.label(non_mag, structure=structure)
    bboxes: list[tuple[int, int, int, int]] = []
    for i in range(1, n + 1):
        ys, xs = np.where(labels == i)
        bboxes.append((int(ys.min()), int(xs.min()), int(ys.max()), int(xs.max())))

    # Filter to substantial blobs (drop tiny speckles).
    sized: list[tuple[int, tuple[int, int, int, int]]] = []
    for i, bb in enumerate(bboxes):
        w = bb[3] - bb[1]
        h = bb[2] - bb[0]
        if w >= 40 and h >= 40:
            sized.append((i, bb))

    # Sort by reading order (row-major centroid).
    sized.sort(key=lambda ib: ((ib[1][0] + ib[1][2]) // 2 // 250,
                                (ib[1][1] + ib[1][3]) // 2))

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    written = 0
    for idx, (_, bb) in enumerate(sized):
        name = f"obj_{idx:03d}.png"
        if name not in targets:
            continue
        tile = crop_and_fill(sheet, bb)
        Image.fromarray(tile, "RGBA").save(OUT_DIR / name)
        print(f"  wrote {name}  ({tile.shape[1]}×{tile.shape[0]})")
        written += 1
    print(f"done. {written} vegetation sprites written.")


if __name__ == "__main__":
    main()

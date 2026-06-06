"""Install GPT-generated forest sprites over our DALL-E vegetation.

The forest sheet the maintainer had GPT generate has gorgeous painterly trees,
bushes, flowers, and accents in the same style as our characters. We
override the canonical decoration sprites the renderer uses so the
existing oak_hollow.py placements pick them up.
"""

from pathlib import Path

import numpy as np
from PIL import Image
from scipy import ndimage  # type: ignore

ART = Path(__file__).resolve().parent
SRC = ART / "processed" / "forest_sheet_v2"
DST = ART / "processed" / "objects" / "vegetation"


def deep_halo_clean(im: Image.Image) -> Image.Image:
    """Two-stage halo killer for sprites off a magenta-padded sheet.

    Stage 1 — KILL pink pixels: any pixel whose RB-vs-G signature
    looks pink/magenta gets alpha-zeroed entirely. We don't try to
    recolor them; isolated pink dots inside the sprite became visible
    horizontal bands at upscale, which is the artifact the maintainer flagged.

    Stage 2 — Recolor remaining halo: after Stage 1 there may still
    be a thin rim of pixels that are subtly pink-tinged but not
    fully pink. Those get recolored toward the nearest clean neighbor.
    """
    arr = np.array(im.convert("RGBA"))
    r = arr[..., 0].astype(np.int32)
    g = arr[..., 1].astype(np.int32)
    b = arr[..., 2].astype(np.int32)
    rb_avg = (r + b) // 2

    # Stage 1: kill anything clearly pink. Aggressive thresholds
    # (lower than before). The danger is killing legitimate flowers,
    # but we accept that — flowers can be re-added as decorations later.
    strong_pink = ((rb_avg - g) > 20) & (r > 70) & (b > 70) & (arr[..., 3] > 0)
    pure_magenta = (r > 200) & (b > 200) & (g < 130)
    arr[strong_pink | pure_magenta, 3] = 0

    # Stage 2: rim cleanup. Anything still vaguely pink + opaque gets
    # recolored to nearest clean neighbor.
    for _ in range(3):
        r = arr[..., 0].astype(np.int32)
        g = arr[..., 1].astype(np.int32)
        b = arr[..., 2].astype(np.int32)
        a = arr[..., 3]
        rb_avg = (r + b) // 2
        halo = (a > 100) & ((rb_avg - g) > 10) & (r > 80) & (b > 80)
        if not halo.any():
            break
        clean = (a > 100) & ~halo
        if not clean.any():
            break
        dist, (iy, ix) = ndimage.distance_transform_edt(  # type: ignore[misc]
            ~clean, return_indices=True,
        )
        arr[halo, :3] = arr[iy[halo], ix[halo], :3]
    return Image.fromarray(arr, "RGBA")

# blob index → veg:NNN filename. Verified by visual inspection.
# Each ID has ONE consistent semantic + size so the world generator
# can place it without conflicting size hints.
MAPPING: dict[int, str] = {
    # Big trees — 2 tiles tall when placed.
    9:   "obj_000.png",   # round green tree (default canopy)
    11:  "obj_001.png",   # autumn orange tree (rare accent)
    66:  "obj_004.png",   # green Christmas-tree pine (default pine)
    44:  "obj_022.png",   # berry/apple tree (rare accent)
    47:  "obj_032.png",   # snow-capped pine (rare accent)
    8:   "obj_036.png",   # bright chunky oak (default variant)
    46:  "obj_037.png",   # bright green chunky tree (default variant)

    # Bushes — ~1 tile tall.
    100: "obj_008.png",   # blue-berry bush
    101: "obj_009.png",   # pink-flower bush
    152: "obj_002.png",   # berry bush small
    153: "obj_003.png",   # flower bush small

    # Ground accents — < 1 tile tall, walkable.
    125: "obj_010.png",   # pink/magenta flowers
    130: "obj_025.png",   # leafy plant clump
    150: "obj_011.png",   # big-leaf plant
    140: "obj_040.png",   # small gray rock
    142: "obj_041.png",   # tree stump
    124: "obj_042.png",   # yellow + red flowers cluster
    120: "obj_023.png",   # hollow log
}


def main() -> None:
    DST.mkdir(parents=True, exist_ok=True)
    for blob_idx, dst_name in MAPPING.items():
        src = SRC / f"blob_{blob_idx:03d}.png"
        dst = DST / dst_name
        if not src.exists():
            print(f"  MISSING {src}")
            continue
        # Open + halo-clean + save (instead of plain copy) so any
        # remaining purple bleed gets killed at install time.
        im = Image.open(src)
        cleaned = deep_halo_clean(im)
        cleaned.save(dst)
        print(f"  {src.name}  →  {dst.name}")
    print(f"installed {len(MAPPING)} forest sprites.")


if __name__ == "__main__":
    main()

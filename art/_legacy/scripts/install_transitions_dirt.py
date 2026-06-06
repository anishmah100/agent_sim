"""Install dirt↔grass transition tiles into the atlas.

Output of the second GPT-image-gen prompt. 12 tiles in a 4×3 grid:
8 dirt-with-grass edges + 4 dirt corners + center + 2 dirt variants.

Mapping verified by visual inspection.
"""

from pathlib import Path

import numpy as np
from PIL import Image
from scipy import ndimage  # type: ignore

ART = Path(__file__).resolve().parent
SRC = ART / "processed" / "transitions_dirt_v1"
DST = ART / "processed" / "tiles" / "overworld"


def deep_halo_clean(im: Image.Image) -> Image.Image:
    arr = np.array(im.convert("RGBA"))
    for _ in range(5):
        r = arr[..., 0].astype(np.int32)
        g = arr[..., 1].astype(np.int32)
        b = arr[..., 2].astype(np.int32)
        a = arr[..., 3]
        rb_avg = (r + b) // 2
        halo = (a > 100) & (((rb_avg - g) > 50) & (r > 110) & (b > 110)
                            | ((r > 200) & (b > 200) & (g < 130)))
        if not halo.any():
            break
        clean = (a > 100) & ~halo
        if not clean.any():
            break
        dist, (iy, ix) = ndimage.distance_transform_edt(  # type: ignore[misc]
            ~clean, return_indices=True,
        )
        arr[halo, :3] = arr[iy[halo], ix[halo], :3]
        mag = (arr[..., 0] > 230) & (arr[..., 1] < 60) & (arr[..., 2] > 230)
        arr[mag, 3] = 0
    return Image.fromarray(arr, "RGBA")


# blob index → atlas filename. Verified by inspecting
# /tmp/dirt_labeled.png (auto-generated contact sheet).
MAPPING: dict[int, str] = {
    # Verified by visual inspection of /tmp/dirt_labeled.png.
    # blob_2 and blob_11 not installed — they're duplicate top-variants and
    # alt-pebble accents that no atlas slot consumes. Saved on disk as
    # blob_NNN.png in transitions_dirt_v1/ for future reference.
    0:  "dirt_corner_nw_outer.png",   # grass on N + spilling W
    1:  "dirt_edge_top.png",          # grass on N uniformly
    3:  "dirt_corner_ne_outer.png",   # grass on N + spilling E
    4:  "dirt_edge_left.png",         # grass on W
    5:  "dirt.png",                   # pure dirt (default)
    6:  "dirt_edge_right.png",        # grass on E
    7:  "dirt_edge_bottom.png",       # grass on S
    8:  "dirt_corner_sw_outer.png",   # grass on S + spilling W
    9:  "dirt_corner_se_outer.png",   # grass on S + spilling E
    10: "dirt_cracked.png",           # cracked-dirt variant (replaces existing)
}


def main() -> None:
    DST.mkdir(parents=True, exist_ok=True)
    for blob_idx, dst_name in MAPPING.items():
        src = SRC / f"blob_{blob_idx:03d}.png"
        dst = DST / dst_name
        if not src.exists():
            print(f"  MISSING {src}")
            continue
        cleaned = deep_halo_clean(Image.open(src))
        cleaned.save(dst)
        print(f"  blob_{blob_idx:03d}.png  →  {dst_name}")
    print(f"installed {len(MAPPING)} dirt transition tiles.")


if __name__ == "__main__":
    main()

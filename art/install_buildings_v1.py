"""Install building sprites from the building tileset.

Sprites land in processed/objects/buildings/obj_NNN.png — the renderer
loads them via the bld:NNN decoration ID."""

from pathlib import Path
import numpy as np
from PIL import Image
from scipy import ndimage  # type: ignore

ART = Path(__file__).resolve().parent
SRC = ART / "processed" / "buildings_v1"
DST = ART / "processed" / "objects" / "buildings"


def deep_halo_clean(im: Image.Image) -> Image.Image:
    arr = np.array(im.convert("RGBA"))
    r = arr[..., 0].astype(np.int32)
    g = arr[..., 1].astype(np.int32)
    b = arr[..., 2].astype(np.int32)
    rb_avg = (r + b) // 2
    strong_pink = ((rb_avg - g) > 20) & (r > 70) & (b > 70) & (arr[..., 3] > 0)
    pure_magenta = (r > 200) & (b > 200) & (g < 130)
    arr[strong_pink | pure_magenta, 3] = 0
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


# Hand-picked from /tmp/buildings_labeled.png.
MAPPING: dict[int, str] = {
    0:  "obj_000.png",  # small red-roof cottage (3w×3h footprint)
    1:  "obj_001.png",  # small dark-roof cottage
    4:  "obj_004.png",  # yellow tavern (5w×3h)
    5:  "obj_005.png",  # large red-awning market (5w×2h)
    6:  "obj_006.png",  # red-stripe market stall (1w×2h)
    7:  "obj_007.png",  # blue-stripe market stall
    8:  "obj_008.png",  # wishing well (1w×1h)
    9:  "obj_009.png",  # lantern post (1w×2h)
    10: "obj_010.png",  # signpost (1w×1h)
    11: "obj_011.png",  # arrow sign
    12: "obj_012.png",  # bench (2w×1h)
    13: "obj_013.png",  # barrel
    14: "obj_014.png",  # fence segment (horizontal)
    18: "obj_018.png",  # fence gate (small)
    22: "obj_022.png",  # fence gate (big)
}


def main() -> None:
    DST.mkdir(parents=True, exist_ok=True)
    for blob, name in MAPPING.items():
        src = SRC / f"blob_{blob:03d}.png"
        if not src.exists():
            print(f"  MISSING {src}")
            continue
        cleaned = deep_halo_clean(Image.open(src))
        cleaned.save(DST / name)
        print(f"  blob_{blob:03d}.png  →  {name}")
    print(f"installed {len(MAPPING)} building sprites.")


if __name__ == "__main__":
    main()

"""Install GPT-generated forest sprites over our DALL-E vegetation.

The forest sheet the maintainer had GPT generate has gorgeous painterly trees,
bushes, flowers, and accents in the same style as our characters. We
override the canonical decoration sprites the renderer uses so the
existing oak_hollow.py placements pick them up.
"""

import shutil
from pathlib import Path

ART = Path(__file__).resolve().parent
SRC = ART / "processed" / "forest_sheet_v2"
DST = ART / "processed" / "objects" / "vegetation"

# blob index → veg:NNN filename
MAPPING: dict[int, str] = {
    9:   "obj_000.png",   # default big green tree
    11:  "obj_001.png",   # orange autumn accent
    66:  "obj_004.png",   # pine tree (default forest pine)
    100: "obj_008.png",   # blue berry bush
    101: "obj_009.png",   # flower bush
    125: "obj_010.png",   # pink flowers (small)
    44:  "obj_022.png",   # apple/berry tree (large accent)
    120: "obj_023.png",   # wooden log
    130: "obj_025.png",   # leafy plant
    47:  "obj_032.png",   # snow pine (rare accent)
    8:   "obj_036.png",   # round chunky green tree (extra variety)
    70:  "obj_037.png",   # tall pine (extra)
}


def main() -> None:
    DST.mkdir(parents=True, exist_ok=True)
    for blob_idx, dst_name in MAPPING.items():
        src = SRC / f"blob_{blob_idx:03d}.png"
        dst = DST / dst_name
        if not src.exists():
            print(f"  MISSING {src}")
            continue
        shutil.copy(src, dst)
        print(f"  {src.name}  →  {dst.name}")
    print(f"installed {len(MAPPING)} forest sprites.")


if __name__ == "__main__":
    main()

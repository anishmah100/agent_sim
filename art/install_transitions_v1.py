"""Install GPT-generated transition tiles into our atlas.

Blob → semantic name mapping for the 4×3 transition sheet the maintainer had
GPT generate. The sheet has 8 grass↔cobblestone-path transitions and
4 grass↔water+sand transitions. The painterly style matches our
existing DALL-E character + tree art perfectly, so we slot these in
as the canonical edge tiles for the autotile picker.
"""

import shutil
from pathlib import Path

ART = Path(__file__).resolve().parent
SRC = ART / "processed" / "transitions_v1"
DST = ART / "processed" / "tiles" / "overworld"

# blob index → target file name in our tile atlas.
MAPPING: dict[int, str] = {
    # Stone (cobblestone path) — 8 cells from the sheet
    5:  "stone.png",                  # pure cobblestone center
    1:  "stone_edge_top.png",         # grass strip along TOP
    7:  "stone_edge_bottom.png",      # grass strip along BOTTOM
    4:  "stone_edge_left.png",        # grass strip along LEFT
    3:  "stone_edge_right.png",       # grass strip along RIGHT
    0:  "stone_corner_nw_outer.png",  # grass spilling from NW
    2:  "stone_corner_ne_outer.png",  # grass from NE (similar to top, slight angle)
    6:  "stone_corner_sw_outer.png",  # grass spilling from SW
    # Water + sandy shoreline
    8:  "water_edge_top.png",         # water tile, shore on N
    9:  "water_edge_left.png",        # water tile, shore on W
    10: "water_edge_right.png",       # water tile, shore on E
    11: "water_edge_bottom.png",      # water tile, shore on S
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
    print(f"installed {len(MAPPING)} transition tiles.")


if __name__ == "__main__":
    main()

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
    # Verified by visual inspection 2026-06-02. Comments describe what
    # I see in each blob and which autotile slot it fits.
    5:  "stone.png",                  # blob_005: pure cobblestone w/ green tufts. Default.
    1:  "stone_edge_top.png",         # blob_001: grass strip uniformly across TOP. Pure N edge.
    6:  "stone_edge_bottom.png",      # blob_006: grass strip uniformly across BOTTOM. Pure S edge.
    4:  "stone_edge_left.png",        # blob_004: stone on right, grass strip on LEFT.
    3:  "stone_edge_right.png",       # blob_003: stone on left, grass strip on RIGHT.
    # Corner variants — grass on N and slightly spills around the
    # corresponding side. Only 2 corners are actually present in the sheet.
    0:  "stone_corner_nw_outer.png",  # blob_000: grass N + slight LEFT-side spill = NW corner.
    2:  "stone_corner_ne_outer.png",  # blob_002: grass N + slight RIGHT-side spill = NE corner.
    7:  "stone_corner_se_outer.png",  # blob_007: grass S + slight RIGHT-side spill = SE corner.
    # NB: NO stone_corner_sw_outer is available in this sheet — the
    # autotile picker will fall back to stone_edge_bottom for SW corners.

    # Water + sandy shoreline. 4 edges, no corners in this sheet.
    8:  "water_edge_top.png",         # blob_008: water in S 2/3, sand+grass+flowers on N.
    9:  "water_edge_left.png",        # blob_009: water on E 2/3, sand+grass on W.
    10: "water_edge_right.png",       # blob_010: water on W 2/3, sand+grass on E.
    11: "water_edge_bottom.png",      # blob_011: water on N 2/3, sand+grass on S.
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

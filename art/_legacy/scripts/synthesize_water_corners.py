"""Synthesize water_corner_* tiles by compositing the GPT-style edge
tiles together. The painterly water+sand-shore edges installed from
the transition sheet only cover N/S/E/W. The autotile picker also needs
4 outer corners (NE, NW, SE, SW). The OLD DALL-E water_corner_*.png
files mismatch in style, producing the jarring boundary the maintainer flagged.

Trick: take two adjacent edges (e.g. top + right for an NE corner) and
overlay their SAND/SHORE bands onto a pure-water base. Result is a
tile with sand strips along BOTH edges in the same painterly style.

Output overwrites:
  water_corner_ne.png  (sand on N + E)
  water_corner_nw.png  (sand on N + W)
  water_corner_se.png  (sand on S + E)
  water_corner_sw.png  (sand on S + W)
"""

from pathlib import Path

import numpy as np
from PIL import Image

ART = Path(__file__).resolve().parent
TILES = ART / "processed" / "tiles" / "overworld"


def is_sand_or_grass(rgba: np.ndarray) -> np.ndarray:
    """A pixel is shore/grass if it's NOT predominantly blue.
    Water in our painterly tiles is cyan/blue (high B, lower R).
    Sand is warm tan (high R, mid G, low B). Grass is green (low R,
    high G, low B). Both differ from water."""
    r = rgba[..., 0].astype(np.int32)
    g = rgba[..., 1].astype(np.int32)
    b = rgba[..., 2].astype(np.int32)
    a = rgba[..., 3]
    # water signature: blue dominates, b > r + 30 AND b > 100
    is_water = (b > r + 30) & (b > 100) & (a > 0)
    return (a > 200) & ~is_water


def composite_corner(edge_a_path: Path, edge_b_path: Path, out_path: Path) -> None:
    """Take two edge tiles. For each pixel, prefer the non-water pixel
    from either edge. Otherwise keep the water pixel."""
    a = np.array(Image.open(edge_a_path).convert("RGBA"))
    b_path_img = Image.open(edge_b_path).convert("RGBA").resize(
        (a.shape[1], a.shape[0]), Image.LANCZOS,
    )
    b = np.array(b_path_img)
    sand_a = is_sand_or_grass(a)
    sand_b = is_sand_or_grass(b)
    out = a.copy()
    # Where b has shore but a doesn't, copy b's shore color in.
    only_b = sand_b & ~sand_a
    out[only_b] = b[only_b]
    # Where both have shore, prefer the brighter one (more grass/sand).
    both = sand_a & sand_b
    # Sum RGB; whichever is higher = brighter; pick that pixel
    a_bright = a[both, :3].astype(np.int32).sum(axis=-1)
    b_bright = b[both, :3].astype(np.int32).sum(axis=-1)
    mask = b_bright > a_bright
    idx = np.where(both)
    pick_b = (idx[0][mask], idx[1][mask])
    out[pick_b] = b[pick_b]
    Image.fromarray(out, "RGBA").save(out_path)


def main() -> None:
    et = TILES / "water_edge_top.png"
    eb = TILES / "water_edge_bottom.png"
    el = TILES / "water_edge_left.png"
    er = TILES / "water_edge_right.png"

    pairs = [
        ("water_corner_ne.png", et, er),  # N + E
        ("water_corner_nw.png", et, el),  # N + W
        ("water_corner_se.png", eb, er),  # S + E
        ("water_corner_sw.png", eb, el),  # S + W
    ]
    for (name, a, b) in pairs:
        composite_corner(a, b, TILES / name)
        print(f"  wrote {name}  (from {a.name} + {b.name})")
    print("done.")


if __name__ == "__main__":
    main()

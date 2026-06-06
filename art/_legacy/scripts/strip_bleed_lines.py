#!/usr/bin/env python3
"""Strip slicer-bleed lines from sliced sprite PNGs.

After strip_white_border.py kills near-white halos there's a secondary
artifact: long horizontal/vertical runs of a single light-blue palette
color — RGB (192, 203, 220), the Endesga "stone" tone. These come from
the cell-divider rows in the master sheet that the content-aware slicer
included in adjacent cells.

Detection: scan every row and every column for the LONGEST contiguous
run of pixels matching the bleed RGB. If that run spans >= the
configured fraction (default 50%) of the image dimension, the whole row
or column is treated as a slicer artifact and its bleed pixels are
zero-alpha'd. Isolated bleed pixels (e.g., a legitimate stone-tone in a
tree base) are preserved because they don't form a long run.

Run on a single PNG, dir, or list. Edits in place.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

try:
    from PIL import Image
    import numpy as np
except ImportError:
    sys.exit("requires Pillow + numpy")


DEFAULT_RGB = (192, 203, 220)
DEFAULT_MIN_FRACTION = 0.50


def longest_run(mask_1d: np.ndarray) -> int:
    """Longest contiguous True run in a 1D boolean array."""
    if not mask_1d.any():
        return 0
    # Numpy trick: diff the indicator and find run lengths.
    diff = np.diff(mask_1d.astype(np.int8))
    starts = np.where(diff == 1)[0] + 1
    ends = np.where(diff == -1)[0] + 1
    if mask_1d[0]:
        starts = np.insert(starts, 0, 0)
    if mask_1d[-1]:
        ends = np.append(ends, len(mask_1d))
    return int((ends - starts).max())


def strip(path: Path, rgb: tuple[int, int, int],
          min_fraction: float,
          dom_min_opaque: int = 30,
          dom_fraction: float = 0.9,
          neighbor_max_fraction: float = 0.10) -> tuple[int, int, int]:
    """Returns (rows_killed, cols_killed, pixels_zeroed).

    Two detectors:
      1. Long-run detector: row/col contains a contiguous run of the
         bleed RGB spanning >= `min_fraction` of the perpendicular dim.
      2. Dominant-color detector: row/col is >= `dom_fraction` bleed
         among its opaque pixels (>= `dom_min_opaque` total), AND its
         immediate neighbors are < `neighbor_max_fraction` bleed. This
         catches single isolated columns where the bleed didn't form
         a continuous full-height run but still dominates the column.
    """
    im = Image.open(path).convert("RGBA")
    arr = np.array(im, dtype=np.uint8)
    h, w = arr.shape[:2]
    is_bleed = (
        (arr[..., 0] == rgb[0]) &
        (arr[..., 1] == rgb[1]) &
        (arr[..., 2] == rgb[2]) &
        (arr[..., 3] > 0)
    )
    is_opaque = arr[..., 3] > 0

    bleed_rows = []
    for y in range(h):
        if longest_run(is_bleed[y]) >= min_fraction * w:
            bleed_rows.append(y)
    bleed_cols = []
    for x in range(w):
        if longest_run(is_bleed[:, x]) >= min_fraction * h:
            bleed_cols.append(x)

    # Dominant-color detector (columns).
    col_opaque = is_opaque.sum(axis=0)
    col_bleed = is_bleed.sum(axis=0)
    for x in range(w):
        if col_opaque[x] < dom_min_opaque:
            continue
        if col_bleed[x] / col_opaque[x] < dom_fraction:
            continue
        # Neighbor check — make sure this is an isolated bleed column,
        # not a slab of legitimate stone content.
        nb_ok = True
        for dx in (-1, 1):
            nx = x + dx
            if 0 <= nx < w and col_opaque[nx] > 0:
                if col_bleed[nx] / col_opaque[nx] > neighbor_max_fraction:
                    nb_ok = False
                    break
        if nb_ok and x not in bleed_cols:
            bleed_cols.append(x)

    # Dominant-color detector (rows) — same logic transposed.
    row_opaque = is_opaque.sum(axis=1)
    row_bleed = is_bleed.sum(axis=1)
    for y in range(h):
        if row_opaque[y] < dom_min_opaque:
            continue
        if row_bleed[y] / row_opaque[y] < dom_fraction:
            continue
        nb_ok = True
        for dy in (-1, 1):
            ny = y + dy
            if 0 <= ny < h and row_opaque[ny] > 0:
                if row_bleed[ny] / row_opaque[ny] > neighbor_max_fraction:
                    nb_ok = False
                    break
        if nb_ok and y not in bleed_rows:
            bleed_rows.append(y)

    if not bleed_rows and not bleed_cols:
        return 0, 0, 0

    # Only zero the bleed-RGB pixels in flagged rows/cols. Don't touch
    # legitimately-different pixels that happen to sit in the same row.
    kill = np.zeros((h, w), dtype=bool)
    for y in bleed_rows:
        kill[y] = is_bleed[y]
    for x in bleed_cols:
        kill[:, x] |= is_bleed[:, x]
    zeroed = int(kill.sum())
    arr[..., 3][kill] = 0
    arr[..., 0][kill] = 0
    arr[..., 1][kill] = 0
    arr[..., 2][kill] = 0
    Image.fromarray(arr, mode="RGBA").save(path)
    return len(bleed_rows), len(bleed_cols), zeroed


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("paths", nargs="+", type=Path)
    p.add_argument("--rgb", default=",".join(str(c) for c in DEFAULT_RGB),
                   help="bleed RGB as 'r,g,b' (default 192,203,220)")
    p.add_argument("--min-fraction", type=float, default=DEFAULT_MIN_FRACTION,
                   help="contiguous-run length / image dim (default 0.50)")
    p.add_argument("--dry-run", action="store_true")
    args = p.parse_args()
    rgb = tuple(int(x) for x in args.rgb.split(","))
    if len(rgb) != 3:
        sys.exit("--rgb must be 'r,g,b'")

    files: list[Path] = []
    for raw in args.paths:
        if raw.is_file():
            files.append(raw)
        else:
            files.extend(sorted(raw.glob("*.png")))

    for path in files:
        if ".thumb." in path.name:
            continue
        if args.dry_run:
            im = Image.open(path).convert("RGBA")
            arr = np.array(im, dtype=np.uint8)
            h, w = arr.shape[:2]
            is_bleed = (
                (arr[..., 0] == rgb[0]) &
                (arr[..., 1] == rgb[1]) &
                (arr[..., 2] == rgb[2]) &
                (arr[..., 3] > 0)
            )
            br = sum(1 for y in range(h)
                     if longest_run(is_bleed[y]) >= args.min_fraction * w)
            bc = sum(1 for x in range(w)
                     if longest_run(is_bleed[:, x]) >= args.min_fraction * h)
            if br or bc:
                print(f"{path.name}: would kill {br} rows + {bc} cols")
            continue
        nr, nc, nz = strip(path, rgb, args.min_fraction)
        if nz > 0:
            print(f"{path.name}: killed {nr} rows + {nc} cols = {nz} px")


if __name__ == "__main__":
    main()

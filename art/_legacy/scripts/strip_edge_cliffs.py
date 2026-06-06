#!/usr/bin/env python3
"""Trim narrow edge "cliffs" from sliced sprite PNGs.

After strip_white_border + strip_bleed_lines, some sprites still show a
thin vertical / horizontal line at the bounding-box edge — usually the
last 1-3 columns/rows of a tree canopy where the slicer's tight crop
clipped right through the silhouette. Visually that's a sliver of
opaque pixels packed into a short range, which reads as a line when
composited.

Detection: walk inward from each side; for each edge column (or row),
if its opaque-pixel count is < `cliff_fraction` of the peak opaque
count seen in the next `lookahead` interior columns, drop it. Stop the
walk as soon as a column passes the threshold (we're now in the body).
Conservative — at most `max_trim` columns/rows per side.

Run on dirs or individual files. Edits in place.
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


def opaque_count_per_col(alpha: np.ndarray) -> np.ndarray:
    return (alpha > 0).sum(axis=0)


def opaque_count_per_row(alpha: np.ndarray) -> np.ndarray:
    return (alpha > 0).sum(axis=1)


def trim_side(counts: np.ndarray, from_left: bool,
              cliff_fraction: float, lookahead: int,
              max_trim: int) -> list[int]:
    """Returns indices (into `counts`) to drop. Walks inward from one end."""
    n = len(counts)
    drop: list[int] = []
    if from_left:
        order = range(0, min(max_trim, n))
    else:
        order = range(n - 1, max(n - 1 - max_trim, -1), -1)
    for i in order:
        # Peak of the next `lookahead` interior columns.
        if from_left:
            window = counts[i + 1 : i + 1 + lookahead]
        else:
            window = counts[max(0, i - lookahead) : i]
        if len(window) == 0:
            break
        peak = int(window.max())
        if peak == 0:
            # Past the body; nothing to compare against. Stop.
            break
        if counts[i] < cliff_fraction * peak:
            drop.append(i)
        else:
            break
    return drop


def strip(path: Path, cliff_fraction: float, lookahead: int,
          max_trim: int) -> tuple[int, int, int]:
    """Returns (cols_dropped, rows_dropped, pixels_zeroed)."""
    im = Image.open(path).convert("RGBA")
    arr = np.array(im, dtype=np.uint8)
    h, w = arr.shape[:2]
    alpha = arr[..., 3]
    col_counts = opaque_count_per_col(alpha)
    row_counts = opaque_count_per_row(alpha)

    drop_cols_left = trim_side(col_counts, True, cliff_fraction, lookahead, max_trim)
    drop_cols_right = trim_side(col_counts, False, cliff_fraction, lookahead, max_trim)
    drop_rows_top = trim_side(row_counts, True, cliff_fraction, lookahead, max_trim)
    drop_rows_bot = trim_side(row_counts, False, cliff_fraction, lookahead, max_trim)

    drop_cols = sorted(set(drop_cols_left + drop_cols_right))
    drop_rows = sorted(set(drop_rows_top + drop_rows_bot))
    if not drop_cols and not drop_rows:
        return 0, 0, 0

    kill = np.zeros((h, w), dtype=bool)
    for x in drop_cols:
        kill[:, x] |= alpha[:, x] > 0
    for y in drop_rows:
        kill[y, :] |= alpha[y, :] > 0
    zeroed = int(kill.sum())
    arr[..., 3][kill] = 0
    arr[..., 0][kill] = 0
    arr[..., 1][kill] = 0
    arr[..., 2][kill] = 0
    Image.fromarray(arr, mode="RGBA").save(path)
    return len(drop_cols), len(drop_rows), zeroed


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("paths", nargs="+", type=Path)
    p.add_argument("--cliff-fraction", type=float, default=0.20,
                   help="edge col/row dropped if opaque-count < this * interior peak")
    p.add_argument("--lookahead", type=int, default=5,
                   help="interior cols/rows used to compute peak")
    p.add_argument("--max-trim", type=int, default=4,
                   help="max edge cols/rows trimmed per side")
    p.add_argument("--dry-run", action="store_true")
    args = p.parse_args()

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
            alpha = arr[..., 3]
            col_counts = opaque_count_per_col(alpha)
            row_counts = opaque_count_per_row(alpha)
            dcl = trim_side(col_counts, True, args.cliff_fraction, args.lookahead, args.max_trim)
            dcr = trim_side(col_counts, False, args.cliff_fraction, args.lookahead, args.max_trim)
            drt = trim_side(row_counts, True, args.cliff_fraction, args.lookahead, args.max_trim)
            drb = trim_side(row_counts, False, args.cliff_fraction, args.lookahead, args.max_trim)
            if dcl or dcr or drt or drb:
                print(f"{path.name}: would drop cols L={dcl} R={dcr}  rows T={drt} B={drb}")
            continue
        nc, nr, nz = strip(path, args.cliff_fraction, args.lookahead, args.max_trim)
        if nz > 0:
            print(f"{path.name}: dropped {nc} cols + {nr} rows = {nz} px")


if __name__ == "__main__":
    main()

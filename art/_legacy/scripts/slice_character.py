#!/usr/bin/env python3
"""Slice a processed character sheet into per-frame PNG files.

Key principle (discipline): the v2 sheets do NOT respect the grid.
DALL-E adds wide margins and shifts characters out of alignment. So
we detect cell positions from content projection, not from naive
division of sheet dims by row/col count.

Input:  art/processed/<name>.png  (RGBA, source resolution, magenta keyed)
Output: art/processed/frames/<name>/walk_down_0.png ... action_3.png
        art/manifests/character_frames/<name>.json

Algorithm:
  1. Compute per-row alpha presence to find the 5 row bands. Cluster
     contiguous "has-content" rows into 5 groups; reject if other.
  2. Within each row band, compute per-column alpha presence and
     cluster into 4 character columns (the per-row column positions
     may differ — DALL-E may shift them).
  3. For each (row, col), the bbox is the tight content rect. Crop
     vertically to a uniform row-band height for baseline consistency.
"""

import argparse
import json
import os
import sys
from pathlib import Path

try:
    from PIL import Image
    import numpy as np
except ImportError:
    sys.stderr.write("requires Pillow + numpy\n")
    sys.exit(1)


ROOT = Path(__file__).resolve().parent
PROCESSED = ROOT / "processed"
FRAMES = PROCESSED / "frames"
MANIFEST_DIR = ROOT / "manifests" / "character_frames"

ROW_NAMES = ["walk_down", "walk_up", "walk_left", "walk_right", "action"]


def cluster_runs(occupied: np.ndarray, n_expected: int,
                 min_gap: int = 6) -> list[tuple[int, int]]:
    """Given a 1D boolean array, find runs of True values separated by gaps
    of at least `min_gap` False values. Returns N tuples (start, end_exclusive).
    Merges short gaps so that thin slivers of magenta inside a character
    silhouette (e.g. between legs) don't split a character in two."""
    # Find contiguous True runs.
    runs: list[tuple[int, int]] = []
    in_run = False
    start = 0
    for i, v in enumerate(occupied):
        if v and not in_run:
            start = i
            in_run = True
        elif not v and in_run:
            runs.append((start, i))
            in_run = False
    if in_run:
        runs.append((start, len(occupied)))

    # Merge runs separated by gaps shorter than min_gap.
    merged: list[tuple[int, int]] = []
    for s, e in runs:
        if merged and s - merged[-1][1] < min_gap:
            merged[-1] = (merged[-1][0], e)
        else:
            merged.append((s, e))

    # If too many groups, keep the N widest. If too few, keep what we have.
    if len(merged) > n_expected:
        merged.sort(key=lambda r: -(r[1] - r[0]))
        merged = merged[:n_expected]
        merged.sort(key=lambda r: r[0])
    return merged


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("name", help="processed character id (e.g. 'blacksmith_npc')")
    p.add_argument("--rows", type=int, default=5)
    p.add_argument("--cols", type=int, default=4)
    p.add_argument(
        "--min-row-gap",
        type=int,
        default=10,
        help="min magenta-row run to consider a row separator",
    )
    p.add_argument(
        "--min-col-gap",
        type=int,
        default=15,
        help="min magenta-col run to consider a column separator within a row",
    )
    p.add_argument(
        "--h-pad",
        type=int,
        default=2,
        help="transparent pad above + below the unified frame height",
    )
    p.add_argument(
        "--w-pad",
        type=int,
        default=2,
        help="transparent pad on left + right of each frame bbox",
    )
    args = p.parse_args()

    src = PROCESSED / f"{args.name}.png"
    if not src.exists():
        sys.exit(f"not found: {src}")

    sheet = Image.open(src).convert("RGBA")
    sw, sh = sheet.size
    arr = np.array(sheet, dtype=np.uint8)
    alpha = arr[..., 3]
    print(f"sheet {sw}x{sh}")

    # PASS A: detect row bands from alpha row-projection.
    row_has = (alpha > 0).any(axis=1)
    row_bands = cluster_runs(row_has, args.rows, min_gap=args.min_row_gap)
    print(f"detected {len(row_bands)} row band(s): {row_bands}")
    # Fallback: if too few bands, DALL-E placed two rows tight (a tiny
    # 1-px gap, or none). Find the tallest band and split it at the
    # local minimum of alpha-density within. Repeat until count matches.
    row_density = (alpha > 0).sum(axis=1)  # alpha mass per row of pixels
    while len(row_bands) < args.rows:
        row_bands.sort(key=lambda r: -(r[1] - r[0]))
        biggest_s, biggest_e = row_bands.pop(0)
        # Search the middle 50% of the band for the row with lowest density.
        margin = (biggest_e - biggest_s) // 4
        ys = range(biggest_s + margin, biggest_e - margin)
        if not ys:
            sys.exit("row band too short to split")
        split_y = min(ys, key=lambda y: row_density[y])
        row_bands.append((biggest_s, split_y))
        row_bands.append((split_y + 1, biggest_e))
        row_bands.sort()
        print(f"  split at y={split_y} (density={row_density[split_y]}) → {len(row_bands)} bands")
    if len(row_bands) > args.rows:
        # Keep the N tallest, then re-sort by y.
        row_bands = sorted(row_bands, key=lambda r: -(r[1] - r[0]))[: args.rows]
        row_bands.sort()
        print(f"  trimmed to {args.rows} tallest: {row_bands}")

    # PASS B: per-row band, detect column positions; compute bbox per cell.
    bboxes: dict[tuple[int, int], tuple[int, int, int, int]] = {}
    band_height_max = max(r[1] - r[0] for r in row_bands)

    for r, (y0, y1) in enumerate(row_bands):
        band = alpha[y0:y1, :]
        col_has = (band > 0).any(axis=0)
        col_bands = cluster_runs(col_has, args.cols, min_gap=args.min_col_gap)
        print(f"row {r} ({ROW_NAMES[r]:11s}): y={y0}..{y1}  "
              f"detected {len(col_bands)} col band(s)")
        if len(col_bands) != args.cols:
            print(f"  WARN: expected {args.cols} cols, got {len(col_bands)}")
        for c, (x0, x1) in enumerate(col_bands):
            # Tight bbox WITHIN that col×row band.
            sub = alpha[y0:y1, x0:x1]
            rows_in = np.any(sub > 0, axis=1)
            cols_in = np.any(sub > 0, axis=0)
            if not rows_in.any():
                continue
            by0_l, by1_l = np.where(rows_in)[0][[0, -1]]
            bx0_l, bx1_l = np.where(cols_in)[0][[0, -1]]
            bboxes[(r, c)] = (
                int(x0 + bx0_l),
                int(y0 + by0_l),
                int(x0 + bx1_l) + 1,
                int(y0 + by1_l) + 1,
            )

    # PASS C: compute unified frame height = max content height.
    max_h = max(b[3] - b[1] for b in bboxes.values()) if bboxes else 0
    if max_h == 0:
        sys.exit("no content detected")
    unified_h = max_h + 2 * args.h_pad
    print(f"unified frame height = {unified_h} (max bbox h={max_h} + 2×{args.h_pad} pad)")

    # PASS D: emit frames + manifest.
    out_dir = FRAMES / args.name
    out_dir.mkdir(parents=True, exist_ok=True)
    for f in list(out_dir.iterdir()):
        f.unlink()

    manifest = {
        "id": args.name,
        "sheet": f"{args.name}.png",
        "sheet_dims": [sw, sh],
        "rows_detected": row_bands,
        "unified_frame_h": unified_h,
        "rows": {n: [] for n in ROW_NAMES},
    }

    for r in range(args.rows):
        row_name = ROW_NAMES[r]
        for c in range(args.cols):
            if (r, c) not in bboxes:
                manifest["rows"][row_name].append(None)
                continue
            sx0, sy0, sx1, sy1 = bboxes[(r, c)]
            bbw = sx1 - sx0
            bbh = sy1 - sy0
            out_w = bbw + 2 * args.w_pad
            out_h = unified_h
            dst_x = args.w_pad
            dst_y = unified_h - bbh - args.h_pad

            cell_img = sheet.crop((sx0, sy0, sx1, sy1))
            frame = Image.new("RGBA", (out_w, out_h), (0, 0, 0, 0))
            frame.paste(cell_img, (dst_x, dst_y), cell_img)
            frame.save(out_dir / f"{row_name}_{c}.png")

            manifest["rows"][row_name].append(
                {"x": sx0, "y": sy0, "w": bbw, "h": bbh}
            )

    n = sum(1 for _ in out_dir.iterdir())
    MANIFEST_DIR.mkdir(parents=True, exist_ok=True)
    (MANIFEST_DIR / f"{args.name}.json").write_text(json.dumps(manifest, indent=2))
    print(f"OK  wrote {n} frames -> processed/frames/{args.name}/")
    print(f"OK  manifest -> manifests/character_frames/{args.name}.json")


if __name__ == "__main__":
    main()

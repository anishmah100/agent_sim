#!/usr/bin/env python3
"""Slice a processed master sprite sheet into per-cell PNG files.

Same content-aware detection as slice_character.py: project alpha,
cluster runs into rows + cols, tight-crop each cell.

Output:
  art/processed/v2_<name>/cell_<row>_<col>.png  — one PNG per cell
  art/manifests/v2_<name>.json                  — row/col → bbox + name map

The names mapping is read from art/manifests/v2_<name>_names.json
(hand-authored per sheet to give semantic names like "table", "chair",
etc.). If that file doesn't exist, names default to "cell_R_C".
"""

import argparse
import json
import sys
from pathlib import Path

try:
    from PIL import Image
    import numpy as np
except ImportError:
    sys.exit("requires Pillow + numpy")


ROOT = Path(__file__).resolve().parent
PROCESSED = ROOT / "processed"
MANIFEST_DIR = ROOT / "manifests"


def cluster_runs(occupied: np.ndarray, n_expected: int,
                 min_gap: int) -> list[tuple[int, int]]:
    runs: list[tuple[int, int]] = []
    in_run, start = False, 0
    for i, v in enumerate(occupied):
        if v and not in_run:
            start, in_run = i, True
        elif not v and in_run:
            runs.append((start, i))
            in_run = False
    if in_run:
        runs.append((start, len(occupied)))
    merged: list[tuple[int, int]] = []
    for s, e in runs:
        if merged and s - merged[-1][1] < min_gap:
            merged[-1] = (merged[-1][0], e)
        else:
            merged.append((s, e))
    return merged


def detect_bands(occupied: np.ndarray, n_expected: int,
                 min_gap: int, density: np.ndarray | None = None
                 ) -> list[tuple[int, int]]:
    bands = cluster_runs(occupied, n_expected, min_gap)
    # Top up: if too few, split the tallest band at min-density y/x.
    while len(bands) < n_expected and density is not None:
        bands.sort(key=lambda r: -(r[1] - r[0]))
        s, e = bands.pop(0)
        margin = (e - s) // 4
        candidates = range(s + margin, e - margin)
        if not candidates:
            break
        split = min(candidates, key=lambda x: density[x])
        bands.append((s, split))
        bands.append((split + 1, e))
        bands.sort()
    # Drop down: if too many, keep the N widest.
    if len(bands) > n_expected:
        bands.sort(key=lambda r: -(r[1] - r[0]))
        bands = bands[:n_expected]
        bands.sort()
    return bands


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("name", help="processed sheet name (without .png)")
    p.add_argument("--rows", type=int, required=True)
    p.add_argument("--cols", type=int, required=True)
    p.add_argument("--min-row-gap", type=int, default=10)
    p.add_argument("--min-col-gap", type=int, default=10)
    args = p.parse_args()

    src = PROCESSED / f"{args.name}.png"
    if not src.exists():
        sys.exit(f"not found: {src}")
    sheet = Image.open(src).convert("RGBA")
    sw, sh = sheet.size
    arr = np.array(sheet, dtype=np.uint8)
    alpha = arr[..., 3]
    print(f"{args.name}: {sw}x{sh}")

    # Names manifest (optional).
    names_path = MANIFEST_DIR / f"{args.name}_names.json"
    names_map: dict[str, str] = {}
    if names_path.exists():
        names_map = json.loads(names_path.read_text())
        print(f"  using names from {names_path.name}")

    # Row bands.
    row_has = (alpha > 0).any(axis=1)
    row_density = (alpha > 0).sum(axis=1)
    rows_b = detect_bands(row_has, args.rows, args.min_row_gap, row_density)
    print(f"  rows detected: {len(rows_b)}: {rows_b}")
    if len(rows_b) != args.rows:
        sys.exit(f"  row count {len(rows_b)} != {args.rows}")

    out_dir = PROCESSED / args.name
    out_dir.mkdir(exist_ok=True)
    for f in list(out_dir.iterdir()):
        f.unlink()

    manifest: dict = {
        "id": args.name,
        "sheet": f"{args.name}.png",
        "sheet_dims": [sw, sh],
        "grid": {"rows": args.rows, "cols": args.cols},
        "cells": {},  # key="row,col" -> bbox + name
    }

    for r, (y0, y1) in enumerate(rows_b):
        band = alpha[y0:y1, :]
        col_has = (band > 0).any(axis=0)
        col_density = (band > 0).sum(axis=0)
        cols_b = detect_bands(col_has, args.cols, args.min_col_gap, col_density)
        if len(cols_b) != args.cols:
            print(f"  WARN row {r}: cols={len(cols_b)} != {args.cols}")
        for c, (x0, x1) in enumerate(cols_b):
            sub = alpha[y0:y1, x0:x1]
            rs = np.any(sub > 0, axis=1)
            cs = np.any(sub > 0, axis=0)
            if not rs.any():
                continue
            by0, by1 = np.where(rs)[0][[0, -1]]
            bx0, bx1 = np.where(cs)[0][[0, -1]]
            sx0 = x0 + bx0
            sy0 = y0 + by0
            sx1 = x0 + bx1 + 1
            sy1 = y0 + by1 + 1
            cell = sheet.crop((sx0, sy0, sx1, sy1))

            key = f"{r},{c}"
            name = names_map.get(key, f"cell_{r:02d}_{c:02d}")
            fname = f"{name}.png"
            cell.save(out_dir / fname)
            manifest["cells"][key] = {
                "name": name,
                "x": int(sx0), "y": int(sy0),
                "w": int(sx1 - sx0), "h": int(sy1 - sy0),
            }

    (MANIFEST_DIR / f"{args.name}.json").write_text(
        json.dumps(manifest, indent=2)
    )
    n = sum(1 for _ in out_dir.iterdir())
    print(f"  OK wrote {n} cells -> processed/{args.name}/")


if __name__ == "__main__":
    main()

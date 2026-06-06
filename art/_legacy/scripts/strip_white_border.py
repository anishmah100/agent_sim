#!/usr/bin/env python3
"""Strip stray near-white border pixels from sliced sprite PNGs.

The v2 master sheets came out of generation on a white background. The
intake pipeline keyed *magenta* to alpha=0 but left near-white margin
pixels with alpha=255. After slicing, every per-sprite cell carries a
1–3 px white halo around the actual content, which reads as a
rectangular outline when composited over green grass tiles.

Fix: flood-fill from every edge pixel inward, alpha-zeroing any pixel
that is (a) near-white AND (b) connected to the image border through
near-white-or-already-transparent pixels. Legitimate white highlights
inside the silhouette are preserved because they're enclosed by darker
pixels and never get reached by the flood.

Run:
    python art/strip_white_border.py <png-or-dir> [...]
"""

from __future__ import annotations

import argparse
import sys
from collections import deque
from pathlib import Path

try:
    from PIL import Image
    import numpy as np
except ImportError:
    sys.exit("requires Pillow + numpy")


WHITE_THRESHOLD = 235  # R,G,B all above this counts as "near-white"


def edge_white_count(path: Path, threshold: int = WHITE_THRESHOLD) -> int:
    im = Image.open(path).convert("RGBA")
    arr = np.array(im, dtype=np.uint8)
    r, g, b, a = arr[..., 0], arr[..., 1], arr[..., 2], arr[..., 3]
    nw = (r >= threshold) & (g >= threshold) & (b >= threshold) & (a > 0)
    return int(nw[0].sum() + nw[-1].sum() + nw[:, 0].sum() + nw[:, -1].sum())


def strip(path: Path, threshold: int = WHITE_THRESHOLD) -> tuple[int, int]:
    """Returns (pixels_zeroed, total_pixels). 0 zeroed = no change."""
    im = Image.open(path).convert("RGBA")
    arr = np.array(im, dtype=np.uint8)
    h, w = arr.shape[:2]
    r, g, b, a = arr[..., 0], arr[..., 1], arr[..., 2], arr[..., 3]
    nearwhite = (r >= threshold) & (g >= threshold) & (b >= threshold)
    transparent = a == 0
    floodable = nearwhite | transparent

    visited = np.zeros((h, w), dtype=bool)
    q: deque[tuple[int, int]] = deque()

    def seed(x: int, y: int) -> None:
        if 0 <= x < w and 0 <= y < h and floodable[y, x] and not visited[y, x]:
            visited[y, x] = True
            q.append((x, y))

    for x in range(w):
        seed(x, 0); seed(x, h - 1)
    for y in range(h):
        seed(0, y); seed(w - 1, y)

    while q:
        x, y = q.popleft()
        for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
            nx, ny = x + dx, y + dy
            if 0 <= nx < w and 0 <= ny < h and floodable[ny, nx] and not visited[ny, nx]:
                visited[ny, nx] = True
                q.append((nx, ny))

    kill = visited & nearwhite
    zeroed = int(kill.sum())
    if zeroed == 0:
        return 0, w * h
    arr[..., 3][kill] = 0
    arr[..., 0][kill] = 0
    arr[..., 1][kill] = 0
    arr[..., 2][kill] = 0
    Image.fromarray(arr, mode="RGBA").save(path)
    return zeroed, w * h


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("paths", nargs="+", type=Path)
    p.add_argument("--threshold", type=int, default=WHITE_THRESHOLD)
    p.add_argument("--dry-run", action="store_true")
    args = p.parse_args()

    files: list[Path] = []
    for raw in args.paths:
        if raw.is_file():
            files.append(raw)
        else:
            files.extend(sorted(raw.glob("*.png")))

    total_zeroed = 0
    for path in files:
        # Skip the master sheet itself and thumbnails — those aren't
        # rendered directly.
        if ".thumb." in path.name or path.name.startswith("preview_"):
            continue
        if args.dry_run:
            n = edge_white_count(path, args.threshold)
            if n > 0:
                print(f"{path.name}: {n} near-white edge px")
            continue
        zeroed, total = strip(path, args.threshold)
        if zeroed > 0:
            print(f"{path.name}: zeroed {zeroed} px ({zeroed * 100 / total:.2f}%)")
        total_zeroed += zeroed
    print(f"total zeroed: {total_zeroed}")


if __name__ == "__main__":
    main()

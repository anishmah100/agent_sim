#!/usr/bin/env python3
"""Zero the RGB channels of every fully-transparent (alpha=0) pixel.

Doesn't change rendering — alpha=0 pixels are invisible regardless of
RGB. But it does:
  - make PIL / image previewers show transparency cleanly instead of
    flashing the old background color through transparent regions
  - guard against bugs in any downstream tool that uses premultiplied
    alpha or that reads RGB without checking alpha

Run on dirs or files. Edits in place.
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


def clean(path: Path) -> int:
    im = Image.open(path).convert("RGBA")
    arr = np.array(im, dtype=np.uint8)
    mask = arr[..., 3] == 0
    nonzero_rgb = (arr[..., 0] != 0) | (arr[..., 1] != 0) | (arr[..., 2] != 0)
    target = mask & nonzero_rgb
    n = int(target.sum())
    if n == 0:
        return 0
    arr[..., 0][target] = 0
    arr[..., 1][target] = 0
    arr[..., 2][target] = 0
    Image.fromarray(arr, mode="RGBA").save(path)
    return n


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("paths", nargs="+", type=Path)
    args = p.parse_args()
    files: list[Path] = []
    for raw in args.paths:
        if raw.is_file():
            files.append(raw)
        else:
            files.extend(sorted(raw.glob("*.png")))
    total = 0
    for path in files:
        if ".thumb." in path.name:
            continue
        n = clean(path)
        if n > 0:
            print(f"{path.name}: {n} px cleaned")
        total += n
    print(f"total: {total}")


if __name__ == "__main__":
    main()

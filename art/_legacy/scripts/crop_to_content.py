#!/usr/bin/env python3
"""Tight-crop a processed RGBA sprite to its non-transparent bbox.

Pass one or more processed/<name>.png stems. Output overwrites in place
(safe because the trimmed sprite + the alpha-keyed sheet contain
identical content, just less magenta padding around the edges).
"""

import argparse
import sys
from pathlib import Path

try:
    from PIL import Image
    import numpy as np
except ImportError:
    sys.exit("requires Pillow + numpy")

ROOT = Path(__file__).resolve().parent
PROC = ROOT / "processed"


def crop(name: str) -> None:
    p = PROC / f"{name}.png"
    if not p.exists():
        print(f"missing: {p}")
        return
    img = Image.open(p).convert("RGBA")
    arr = np.array(img, dtype=np.uint8)
    alpha = arr[..., 3]
    rows = np.any(alpha > 0, axis=1)
    cols = np.any(alpha > 0, axis=0)
    if not rows.any():
        print(f"empty: {name}")
        return
    y0, y1 = np.where(rows)[0][[0, -1]]
    x0, x1 = np.where(cols)[0][[0, -1]]
    cropped = img.crop((int(x0), int(y0), int(x1) + 1, int(y1) + 1))
    before = img.size
    after = cropped.size
    cropped.save(p)
    print(f"{name}: {before[0]}x{before[1]} → {after[0]}x{after[1]}")


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("names", nargs="+", help="processed names without .png")
    args = p.parse_args()
    for n in args.names:
        crop(n)


if __name__ == "__main__":
    main()

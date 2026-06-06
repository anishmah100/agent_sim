"""Extract individual sprites from a GPT-generated forest sheet.

Input format expected: a single PNG with magenta (#FF00FF) padding
between sprites. Sprite cells can be any size (the sheet has mixed
tile sizes — small flowers vs 2-tile pines). We detect each sprite
via connected-component blob analysis on the non-magenta mask,
then crop with an inset + distance-transform fill, same pipeline
as the overworld extractor.

Run:
  python extract_forest_sheet.py [sheet.png]
    # default input path: art/raw/forest_sheet_v2.png

Output:
  Raw blob crops at art/processed/forest_sheet_v2/blob_NNN.png
  + a contact sheet at art/processed/forest_sheet_v2/_contact.png
  for visual identification.

After running, you (or I) hand-edit a small JSON config that maps
blob_NNN → semantic name (e.g. blob_042 → "tree_big_green").
That config drives the renaming step which produces final files
into processed/tiles/overworld/ and processed/objects/vegetation/.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage  # type: ignore

ART = Path(__file__).resolve().parent
DEFAULT_INPUT = ART / "raw" / "forest_sheet_v2.png"
RAW_DIR = ART / "processed" / "forest_sheet_v2"

MAGENTA_TOL = 80
HALO_TOL_RB_OVER_G = 60
INSET = 2          # smaller because cell padding is tight on this sheet
FILL_DISTANCE = 6
MIN_BLOB_AREA = 100  # drop tiny speckles


def magenta_mask(rgba: np.ndarray) -> np.ndarray:
    r, g, b = rgba[..., 0], rgba[..., 1], rgba[..., 2]
    return (r > 255 - MAGENTA_TOL) & (g < MAGENTA_TOL) & (b > 255 - MAGENTA_TOL)


def pinkish_mask(rgba: np.ndarray) -> np.ndarray:
    r = rgba[..., 0].astype(np.int32)
    g = rgba[..., 1].astype(np.int32)
    b = rgba[..., 2].astype(np.int32)
    a = rgba[..., 3]
    return (a > 100) & ((r + b) // 2 - g > HALO_TOL_RB_OVER_G) & (r > 120) & (b > 120)


def crop_and_clean(rgba: np.ndarray, bbox: tuple[int, int, int, int]) -> np.ndarray:
    y0, x0, y1, x1 = bbox
    y0i, x0i = max(0, y0 + INSET), max(0, x0 + INSET)
    y1i, x1i = y1 - INSET, x1 - INSET
    if y1i <= y0i or x1i <= x0i:
        return rgba[y0:y1 + 1, x0:x1 + 1].copy()
    tile = rgba[y0i:y1i + 1, x0i:x1i + 1].copy()

    alpha = tile[..., 3]
    opaque = alpha > 200
    if not opaque.all() and opaque.any():
        dist, (iy, ix) = ndimage.distance_transform_edt(  # type: ignore[misc]
            ~opaque, return_indices=True,
        )
        mask = (dist <= FILL_DISTANCE) & (~opaque)
        if mask.any():
            tile[mask, :3] = tile[iy[mask], ix[mask], :3]
            tile[mask, 3] = 255

    tile[magenta_mask(tile), 3] = 0

    for _ in range(3):
        halo = pinkish_mask(tile)
        if not halo.any():
            break
        clean = (tile[..., 3] > 100) & ~halo & ~magenta_mask(tile)
        if not clean.any():
            break
        dist, (iy, ix) = ndimage.distance_transform_edt(  # type: ignore[misc]
            ~clean, return_indices=True,
        )
        tile[halo, :3] = tile[iy[halo], ix[halo], :3]
    return tile


def main() -> None:
    src = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_INPUT
    if not src.exists():
        print(f"ERROR: input not found at {src}")
        sys.exit(1)

    sheet = np.array(Image.open(src).convert("RGBA"))
    print(f"loaded {src} ({sheet.shape[1]}×{sheet.shape[0]})")

    non_mag = ~magenta_mask(sheet)
    structure = np.ones((3, 3), dtype=int)
    labels, n = ndimage.label(non_mag, structure=structure)
    print(f"  found {n} raw blobs")

    bboxes: list[tuple[int, int, int, int]] = []
    for i in range(1, n + 1):
        ys, xs = np.where(labels == i)
        bboxes.append((int(ys.min()), int(xs.min()), int(ys.max()), int(xs.max())))

    # Filter by area.
    keep: list[tuple[int, int, int, int]] = []
    for bb in bboxes:
        area = (bb[2] - bb[0] + 1) * (bb[3] - bb[1] + 1)
        if area >= MIN_BLOB_AREA:
            keep.append(bb)
    keep.sort(key=lambda bb: ((bb[0] + bb[2]) // 2 // 80,
                              (bb[1] + bb[3]) // 2))
    print(f"  kept {len(keep)} blobs after area filter")

    RAW_DIR.mkdir(parents=True, exist_ok=True)
    for old in RAW_DIR.glob("blob_*.png"):
        old.unlink()

    crops: list[np.ndarray] = []
    for idx, bb in enumerate(keep):
        crop = crop_and_clean(sheet, bb)
        Image.fromarray(crop, "RGBA").save(RAW_DIR / f"blob_{idx:03d}.png")
        crops.append(crop)

    # Contact sheet — uniform 96×96 cells with labels.
    cell = 96
    cols = 12
    rows = (len(crops) + cols - 1) // cols
    contact = Image.new("RGBA", (cols * cell, rows * cell), (40, 40, 50, 255))
    draw = ImageDraw.Draw(contact)
    for i, crop in enumerate(crops):
        r, c = divmod(i, cols)
        thumb = Image.fromarray(crop, "RGBA")
        thumb.thumbnail((cell - 8, cell - 16), Image.LANCZOS)
        x = c * cell + (cell - thumb.width) // 2
        y = r * cell + 4
        contact.paste(thumb, (x, y), thumb)
        draw.text((c * cell + 2, r * cell + cell - 14), f"{i:03d}", fill="yellow")
    contact.save(RAW_DIR / "_contact.png")
    print(f"  contact sheet at {RAW_DIR / '_contact.png'}")

    config_path = RAW_DIR / "labels.json"
    if not config_path.exists():
        config = {str(i): "" for i in range(len(crops))}
        config_path.write_text(json.dumps(config, indent=2))
        print(f"  scaffolded labels.json with {len(crops)} entries")


if __name__ == "__main__":
    main()

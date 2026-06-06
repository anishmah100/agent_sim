"""Re-extract overworld tiles from raw/overworld_tileset.png.

Source: raw/overworld_tileset.png (1254×1254, 8×8 grid, magenta padding
between cells, painted dark borders inside each cell).

Pipeline per tile:
  1) Magenta-mask (RGB ≈ (255, 0, 255) within tolerance) → blob = tile.
  2) Inset crop 8 px inside the blob bbox to remove DALL-E's painted
     dark cell border.
  3) Distance-transform fill: pixels within K px of an opaque pixel get
     filled by nearest-neighbor color (kills transparency holes inside).
  4) Save to processed/tiles/overworld/<name>.png at SOURCE RESOLUTION.

Naming comes from the manifest's (row, col) → name map.

Run from agent_sim/art:
  python extract_overworld_tiles.py [tile_names...]

If tile_names given, only those are rewritten (rest left intact).
Otherwise ALL tiles in the manifest are re-extracted.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import numpy as np
from PIL import Image
from scipy import ndimage  # type: ignore

ART = Path(__file__).resolve().parent
RAW = ART / "raw" / "overworld_tileset.png"
MANIFEST = ART / "manifests" / "overworld_tileset.json"
OUT_DIR = ART / "processed" / "tiles" / "overworld"

# Magenta-padding tolerance: pixel is "padding" if it's close to (255,0,255).
MAGENTA_TOL = 40
# How far inside each blob bbox to crop to remove the painted dark border.
INSET = 8
# Pixels within this distance of any opaque pixel get filled with the
# nearest opaque pixel's color. Closes interior transparency holes.
FILL_DISTANCE = 6


def load_grid() -> tuple[np.ndarray, dict[tuple[int, int], dict]]:
    """Return (raw_array RGBA, {(row, col): manifest_entry})."""
    sheet = np.array(Image.open(RAW).convert("RGBA"))
    manifest = json.loads(MANIFEST.read_text())
    grid: dict[tuple[int, int], dict] = {}
    for t in manifest["tiles"]:
        grid[(t["row"], t["col"])] = t
    return sheet, grid


def magenta_mask(rgba: np.ndarray) -> np.ndarray:
    """Boolean mask: True where pixel is the magenta padding."""
    r, g, b = rgba[..., 0], rgba[..., 1], rgba[..., 2]
    return (r > 255 - MAGENTA_TOL) & (g < MAGENTA_TOL) & (b > 255 - MAGENTA_TOL)


def find_blobs(non_magenta: np.ndarray) -> tuple[np.ndarray, int]:
    """Label connected components of non-magenta pixels. Returns
    (labels HxW int, n_components)."""
    structure = np.ones((3, 3), dtype=int)  # 8-connectivity
    labels, n = ndimage.label(non_magenta, structure=structure)
    return labels, n


def blob_bboxes(labels: np.ndarray, n: int) -> list[tuple[int, int, int, int]]:
    """Per-label bbox (y0, x0, y1, x1) inclusive."""
    boxes: list[tuple[int, int, int, int]] = []
    for i in range(1, n + 1):
        ys, xs = np.where(labels == i)
        boxes.append((int(ys.min()), int(xs.min()), int(ys.max()), int(xs.max())))
    return boxes


def grid_index_for_blob(bbox: tuple[int, int, int, int], sheet_size: int) -> tuple[int, int]:
    """Decide which (row, col) grid cell this blob belongs to, based on
    its centroid. We assume an 8×8 grid."""
    y0, x0, y1, x1 = bbox
    cy, cx = (y0 + y1) / 2, (x0 + x1) / 2
    cell = sheet_size / 8
    return (int(cy // cell), int(cx // cell))


def crop_and_fill(rgba: np.ndarray, bbox: tuple[int, int, int, int]) -> np.ndarray:
    """Crop with 8 px inset, then distance-transform fill transparency
    within FILL_DISTANCE of any opaque pixel."""
    y0, x0, y1, x1 = bbox
    y0i, x0i = y0 + INSET, x0 + INSET
    y1i, x1i = y1 - INSET, x1 - INSET
    if y1i <= y0i or x1i <= x0i:
        return rgba[y0:y1 + 1, x0:x1 + 1].copy()
    tile = rgba[y0i:y1i + 1, x0i:x1i + 1].copy()

    # Distance-transform fill: for transparent pixels close to opaque
    # ones, copy the nearest opaque pixel's color.
    alpha = tile[..., 3]
    opaque = alpha > 200
    if opaque.all():
        return tile
    # `distance_transform_edt` with `return_indices=True` gives us the
    # nearest opaque pixel for every position.
    dist, (iy, ix) = ndimage.distance_transform_edt(  # type: ignore[misc]
        ~opaque, return_indices=True,
    )
    mask = (dist <= FILL_DISTANCE) & (~opaque)
    nearest_rgb = tile[iy[mask], ix[mask], :3]
    tile[mask, :3] = nearest_rgb
    tile[mask, 3] = 255

    # Then KILL any remaining magenta pixels by setting alpha to 0.
    remaining_magenta = magenta_mask(tile)
    tile[remaining_magenta, 3] = 0
    return tile


def main() -> None:
    only = set(sys.argv[1:]) if len(sys.argv) > 1 else None
    sheet, grid = load_grid()
    non_mag = ~magenta_mask(sheet)
    labels, n = find_blobs(non_mag)
    bboxes = blob_bboxes(labels, n)

    # Map each blob to its grid cell.
    sheet_size = sheet.shape[0]
    cell_to_blobs: dict[tuple[int, int], list[int]] = {}
    for i, bb in enumerate(bboxes):
        gc = grid_index_for_blob(bb, sheet_size)
        cell_to_blobs.setdefault(gc, []).append(i)

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    written = 0
    skipped = 0
    for (row, col), entry in grid.items():
        name = entry["name"]
        if only is not None and name not in only:
            continue
        candidates = cell_to_blobs.get((row, col), [])
        if not candidates:
            print(f"  NO BLOB @ ({row},{col}) for {name}")
            skipped += 1
            continue
        # Take the largest candidate in the cell (defends against tiny
        # speckles that escaped the magenta mask).
        best = max(candidates, key=lambda i: (
            (bboxes[i][2] - bboxes[i][0]) * (bboxes[i][3] - bboxes[i][1])
        ))
        tile = crop_and_fill(sheet, bboxes[best])
        Image.fromarray(tile, "RGBA").save(OUT_DIR / f"{name}.png")
        written += 1

    print(f"wrote {written}, skipped {skipped}")


if __name__ == "__main__":
    main()

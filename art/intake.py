#!/usr/bin/env python3
"""Validate + normalize a raw AI-generated spritesheet.

Reads from art/raw/, writes to art/processed/ on success, art/rejected/ on
failure with a reason file. Enforces the rules in art/style.json:

- Dimensions match expected layout
- Magenta (#FF00FF) is converted to alpha=0
- Halo pixels around alpha edges are scrubbed
- All non-transparent pixels snap to the locked 32-color palette
- Off-palette percentage must be under intake_rules.max_off_palette_pct
- No anti-aliasing (no semi-transparent pixels in alpha channel after key)

Usage:
    python art/intake.py character_base_v1
    python art/intake.py --asset-class character_base character_base_v1.png

The asset-class controls which dim/layout spec we validate against. See
art/style.json -> *_sheet_layout.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Iterable

try:
    from PIL import Image
    import numpy as np
except ImportError:
    sys.stderr.write(
        "art/intake.py requires Pillow + numpy. Install with:\n"
        "    pip install pillow numpy\n"
    )
    sys.exit(2)


HERE = Path(__file__).parent
STYLE_PATH = HERE / "style.json"
RAW_DIR = HERE / "raw"
PROCESSED_DIR = HERE / "processed"
REJECTED_DIR = HERE / "rejected"


def hex_to_rgb(hex_str: str) -> tuple[int, int, int]:
    s = hex_str.lstrip("#")
    return int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16)


def load_style() -> dict:
    with STYLE_PATH.open() as f:
        return json.load(f)


def reject(name: str, reason: str) -> None:
    """Move asset to rejected/ with a reason file."""
    REJECTED_DIR.mkdir(parents=True, exist_ok=True)
    reason_path = REJECTED_DIR / f"{name}.reason.txt"
    reason_path.write_text(reason + "\n")
    print(f"REJECT  {name}: {reason}", file=sys.stderr)
    sys.exit(1)


def expected_dims(style: dict, asset_class: str) -> tuple[tuple[int, int], int]:
    """Return ((input_w, input_h), downsample_scale) for an asset class."""
    layouts = style.get("character_sheet_layouts", {})
    if asset_class in layouts:
        layout = layouts[asset_class]
        return tuple(layout["input_dims_px"]), int(layout["scale_factor"])
    if asset_class.startswith("character") or asset_class == "trainer_red":
        # Default character layout — the v3 upscaled full sheet.
        layout = layouts["character_full_v3_upscaled"]
        return tuple(layout["input_dims_px"]), int(layout["scale_factor"])
    if asset_class.startswith("tile"):
        raise ValueError("tile assets need explicit layout in style.json")
    raise ValueError(f"unknown asset-class {asset_class!r}")


def key_magenta(arr: np.ndarray, key_rgb: tuple[int, int, int]) -> np.ndarray:
    """Convert RGB(A) array to RGBA with magenta keyed to alpha=0."""
    h, w = arr.shape[:2]
    if arr.shape[2] == 3:
        alpha = np.full((h, w, 1), 255, dtype=np.uint8)
        arr = np.concatenate([arr, alpha], axis=2)
    mask = (
        (arr[:, :, 0] == key_rgb[0])
        & (arr[:, :, 1] == key_rgb[1])
        & (arr[:, :, 2] == key_rgb[2])
    )
    arr[mask, 3] = 0
    return arr


def scrub_halo(arr: np.ndarray) -> int:
    """Remove anti-aliased halo pixels around alpha edges.

    A halo pixel: alpha != 0 AND alpha != 255 (semi-transparent). We
    binarize: any alpha < 128 -> 0, else 255. Returns the count of
    pixels altered (high counts indicate the source had heavy AA).
    """
    alpha = arr[:, :, 3]
    semi = (alpha > 0) & (alpha < 255)
    count = int(semi.sum())
    arr[alpha < 128, 3] = 0
    arr[alpha >= 128, 3] = 255
    return count


def nearest_palette_color(
    pixel_rgb: tuple[int, int, int],
    palette_rgb: list[tuple[int, int, int]],
) -> tuple[int, int, int]:
    pr, pg, pb = pixel_rgb
    best = palette_rgb[0]
    best_d2 = float("inf")
    for r, g, b in palette_rgb:
        d2 = (r - pr) ** 2 + (g - pg) ** 2 + (b - pb) ** 2
        if d2 < best_d2:
            best_d2 = d2
            best = (r, g, b)
    return best


def quantize_to_palette(
    arr: np.ndarray, palette_rgb: list[tuple[int, int, int]]
) -> tuple[np.ndarray, float]:
    """Snap every visible pixel to the nearest palette color.

    Returns (new_arr, off_palette_pct) — the pct of visible pixels that
    needed snapping (a high value suggests the generation was outlier
    and should be reviewed).
    """
    h, w = arr.shape[:2]
    visible = arr[:, :, 3] > 0
    n_visible = int(visible.sum())
    if n_visible == 0:
        return arr, 0.0
    palette_set = {tuple(c) for c in palette_rgb}
    snap_count = 0
    new_arr = arr.copy()
    # Build a unique-color dict to amortize lookups (sheets typically have
    # < 100 unique pre-snap colors after halo scrub).
    unique_colors: dict[tuple[int, int, int], tuple[int, int, int]] = {}
    for y in range(h):
        for x in range(w):
            if not visible[y, x]:
                continue
            rgb = (int(arr[y, x, 0]), int(arr[y, x, 1]), int(arr[y, x, 2]))
            if rgb in unique_colors:
                snapped = unique_colors[rgb]
            elif rgb in palette_set:
                snapped = rgb
                unique_colors[rgb] = rgb
            else:
                snapped = nearest_palette_color(rgb, palette_rgb)
                unique_colors[rgb] = snapped
            if snapped != rgb:
                snap_count += 1
                new_arr[y, x, 0] = snapped[0]
                new_arr[y, x, 1] = snapped[1]
                new_arr[y, x, 2] = snapped[2]
    pct = 100.0 * snap_count / n_visible
    return new_arr, pct


def process(asset_name: str, asset_class: str, explicit_path: Path | None) -> None:
    style = load_style()
    key_rgb = hex_to_rgb(style["transparency_key"])
    palette_rgb = [hex_to_rgb(c) for c in style["palette"]]
    max_off = float(style["intake_rules"]["max_off_palette_pct"])

    src = explicit_path or (RAW_DIR / f"{asset_name}.png")
    if not src.exists():
        reject(asset_name, f"source file not found: {src}")

    try:
        (exp_w, exp_h), scale = expected_dims(style, asset_class)
    except ValueError as e:
        reject(asset_name, str(e))

    img = Image.open(src).convert("RGBA")
    if img.size != (exp_w, exp_h):
        reject(
            asset_name,
            f"dim mismatch: got {img.size}, expected {(exp_w, exp_h)}",
        )

    arr = np.array(img, dtype=np.uint8)
    arr = key_magenta(arr, key_rgb)

    # Downsample BEFORE palette quantize when input is upscaled.
    # We use nearest-neighbour (Image.NEAREST) — bilinear would re-introduce
    # the anti-aliased halos we just keyed out. After downsample, the
    # alpha channel is binarized again (scrub_halo).
    if scale > 1:
        native_w, native_h = exp_w // scale, exp_h // scale
        img_keyed = Image.fromarray(arr, mode="RGBA")
        img_native = img_keyed.resize((native_w, native_h), Image.NEAREST)
        arr = np.array(img_native, dtype=np.uint8)
    halo_count = scrub_halo(arr)

    arr, off_pct = quantize_to_palette(arr, palette_rgb)
    if off_pct > max_off:
        reject(
            asset_name,
            f"off-palette pixels {off_pct:.1f}% > limit {max_off}%. "
            "Source likely used unrelated colors; regenerate or replace.",
        )

    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)
    out = PROCESSED_DIR / f"{asset_name}.png"
    Image.fromarray(arr, mode="RGBA").save(out)

    # Also save a 4× preview so we can eyeball without browser zoom.
    preview_h, preview_w = arr.shape[:2]
    preview_img = Image.fromarray(arr, mode="RGBA").resize(
        (preview_w * 4, preview_h * 4), Image.NEAREST
    )
    preview_path = PROCESSED_DIR / f"{asset_name}.preview_4x.png"
    preview_img.save(preview_path)

    print(
        f"OK      {asset_name}: input {img.size}, "
        f"native {arr.shape[1]}x{arr.shape[0]}, "
        f"halo scrubbed {halo_count} px, off-palette snap {off_pct:.2f}%, "
        f"-> {out.relative_to(HERE.parent)} (+ 4x preview)"
    )


def main() -> None:
    p = argparse.ArgumentParser(
        description="Validate + normalize an AI-generated spritesheet."
    )
    p.add_argument("name", help="asset name (no extension), e.g. 'character_base_v1'")
    p.add_argument(
        "--asset-class",
        default=None,
        help="layout class. inferred from name prefix if omitted.",
    )
    p.add_argument(
        "--path", type=Path, default=None,
        help="explicit input path (defaults to art/raw/<name>.png)",
    )
    args = p.parse_args()

    asset_class = args.asset_class
    if asset_class is None:
        # Default everything character-shaped to the v3 upscaled layout.
        if args.name.startswith("character_") or args.name == "trainer_red":
            asset_class = "character_full_v3_upscaled"
        elif args.name.startswith("tile_"):
            asset_class = "tile"
        else:
            sys.stderr.write(
                f"cannot infer asset class from {args.name!r}; pass --asset-class\n"
            )
            sys.exit(2)

    process(args.name, asset_class, args.path)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Validate + normalize a raw AI-generated spritesheet.

AI image gen (DALL-E 3 / GPT-4o) does NOT produce on-spec pixel art.
Typical artifacts:
- Output dimensions ignored (we ask for 1024x480, get 1831x859)
- "Pure magenta" background is actually thousands of near-magenta shades
- "Solid colors" have JPEG-style noise — 100K+ unique colors total
- Edges have anti-aliasing despite being told not to

This intake is built for that reality. It:
1. Resizes the input to the native target dims (e.g. 128x60) using BOX
   downsampling — averages the JPEG noise into clean target pixels.
2. Quantizes every pixel to a small palette: the 9 character colors plus
   pure magenta. Any near-magenta pixel snaps to pure magenta.
3. Keys magenta to alpha=0.
4. Outputs a clean RGBA PNG + a 4x preview for eyeballing.

For curated pixel artist input (Aseprite, etc.), pass --strict to keep
the old "reject if dims don't match" behavior.

Usage:
    python art/intake.py trainer_red
    python art/intake.py --asset-class character_full_v3_upscaled trainer_red
    python art/intake.py --strict character_base_v4
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

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
    REJECTED_DIR.mkdir(parents=True, exist_ok=True)
    (REJECTED_DIR / f"{name}.reason.txt").write_text(reason + "\n")
    print(f"REJECT  {name}: {reason}", file=sys.stderr)
    sys.exit(1)


CHARACTER_NAMES = {
    "trainer_red", "trainer_lyra_blue", "wizard", "baker",
    "iron_guard", "child", "cloaked_wanderer",
    # also accept the prompt-style names if anyone uses those
    "old_sage", "guard_iron", "child_kid", "merchant_baker", "lyra_blue",
}
TILESET_GRID_NAMES = {"overworld_tileset", "items_master"}
TILESET_FREEFORM_NAMES = {
    "tileset_vegetation", "tileset_building", "tileset_buildings",
    "interior_tileset", "tileset_interior",
}


def get_layout(style: dict, asset_name: str, asset_class: str | None) -> dict:
    """Return the layout descriptor for an asset, inferring from name
    if asset_class isn't passed."""
    char_layouts = style.get("character_sheet_layouts", {})
    tile_layouts = style.get("tileset_layouts", {})

    # Explicit asset_class wins.
    if asset_class is not None:
        if asset_class in char_layouts:
            return char_layouts[asset_class]
        if asset_class in tile_layouts:
            return tile_layouts[asset_class]
        raise ValueError(f"unknown asset-class {asset_class!r}")

    # Name-based inference.
    if asset_name in CHARACTER_NAMES or asset_name.startswith("character_"):
        return char_layouts["character_full_v3_upscaled"]
    if asset_name in TILESET_GRID_NAMES:
        return tile_layouts["tileset_grid_8x8"]
    if asset_name in TILESET_FREEFORM_NAMES:
        return tile_layouts["tileset_freeform_256"]
    raise ValueError(
        f"cannot infer layout for {asset_name!r}; pass --asset-class"
    )


def nearest_color_idx(
    pixels: np.ndarray, palette_rgb: np.ndarray
) -> np.ndarray:
    """Vectorized nearest-color lookup. pixels: (N, 3) uint8.
    palette_rgb: (K, 3) uint8. Returns (N,) int array of indices."""
    # Convert to int32 to avoid uint8 overflow in subtraction.
    p = pixels.astype(np.int32)[:, None, :]              # (N, 1, 3)
    pal = palette_rgb.astype(np.int32)[None, :, :]       # (1, K, 3)
    d2 = ((p - pal) ** 2).sum(axis=2)                    # (N, K)
    return d2.argmin(axis=1)                             # (N,)


def process(
    asset_name: str,
    asset_class: str,
    explicit_path: Path | None,
    strict: bool,
) -> None:
    style = load_style()

    try:
        layout = get_layout(style, asset_name, asset_class)
    except ValueError as e:
        reject(asset_name, str(e))

    # Snap palette resolution order: explicit subset → ref into top-level
    # named palette → full master palette.
    if "snap_palette_subset" in layout:
        snap_hex = layout["snap_palette_subset"]
    elif "snap_palette_ref" in layout:
        ref = layout["snap_palette_ref"]
        if ref not in style:
            reject(asset_name, f"snap_palette_ref {ref!r} not in style.json")
        snap_hex = style[ref]
    else:
        snap_hex = style["palette"]
    palette_rgb = [hex_to_rgb(c) for c in snap_hex]
    palette_rgb_arr = np.array(palette_rgb, dtype=np.uint8)
    print(f"INFO    {asset_name}: snapping to {len(palette_rgb)}-color palette")

    # Some layouts (character) have exact input dims; freeform/grid
    # tilesets use *_approx since DALL-E delivers varying square sizes.
    if "input_dims_px" in layout:
        exp_w, exp_h = layout["input_dims_px"]
    else:
        exp_w, exp_h = layout["input_dims_px_approx"]
        # Treat approx as a hint, not a gate.
    native_w, native_h = layout["native_dims_px"]

    src = explicit_path or (RAW_DIR / f"{asset_name}.png")
    if not src.exists():
        reject(asset_name, f"source file not found: {src}")

    img = Image.open(src).convert("RGB")
    print(f"INFO    {asset_name}: input dims {img.size} "
          f"(expected ~{(exp_w, exp_h)}, downsampling to native {(native_w, native_h)})")
    if strict and img.size != (exp_w, exp_h):
        reject(
            asset_name,
            f"strict mode: dim mismatch {img.size} != {(exp_w, exp_h)}",
        )

    # PHASE 1: Aggressive pre-key on the INPUT image.
    # Any pixel that is unmistakably the magenta background — high red,
    # very low green, high blue — becomes pure (255, 0, 255). This stops
    # AI-gen's "almost magenta" speckle from contaminating downsample.
    src_arr = np.array(img, dtype=np.uint8)
    magenta_mask_src = (
        (src_arr[..., 0] >= 200)
        & (src_arr[..., 1] <= 90)
        & (src_arr[..., 2] >= 200)
    )
    src_arr[magenta_mask_src] = (255, 0, 255)
    img = Image.fromarray(src_arr, mode="RGB")
    pct_pre_keyed = 100.0 * magenta_mask_src.sum() / magenta_mask_src.size
    print(f"INFO    {asset_name}: pre-keyed {pct_pre_keyed:.1f}% of input "
          "pixels as background")

    # PHASE 2: BOX-filter downsample to native. Magenta-background
    # cells stay magenta; cells that span character/background mix.
    img_native = img.resize((native_w, native_h), Image.BOX)
    arr = np.array(img_native, dtype=np.uint8)              # (H, W, 3)

    # PHASE 3: Per-pixel transparency decision BEFORE palette snap.
    # A pixel is "background" if it's clearly more magenta than
    # character — green channel is the deciding factor (magenta has
    # G=0; every character color has G>=39 except hair/outline at G=39).
    # Tunable: G<60 + R>140 + B>140 catches all halo bleed AND pure
    # magenta. Tested on the trainer_red sheet — yields zero halo.
    bg_mask = (
        (arr[..., 1] < 60)
        & (arr[..., 0] > 140)
        & (arr[..., 2] > 140)
    )

    # PHASE 4: Palette quantize the non-background pixels only.
    flat = arr.reshape(-1, 3)
    visible_flat_mask = ~bg_mask.reshape(-1)
    snapped_flat = flat.copy()
    if visible_flat_mask.any():
        visible_idx = nearest_color_idx(
            flat[visible_flat_mask], palette_rgb_arr
        )
        snapped_flat[visible_flat_mask] = palette_rgb_arr[visible_idx]
    snap_diff = (snapped_flat != flat).any(axis=1).sum()
    snap_pct = 100.0 * snap_diff / flat.shape[0]

    # Build RGBA.
    rgba = np.zeros((arr.shape[0], arr.shape[1], 4), dtype=np.uint8)
    rgba[..., :3] = snapped_flat.reshape(arr.shape)
    rgba[..., 3] = np.where(bg_mask, 0, 255)

    # Count visible (non-magenta) and report unique colors actually used.
    visible_mask = rgba[..., 3] > 0
    n_visible = int(visible_mask.sum())
    visible_rgbs = rgba[visible_mask][:, :3]
    unique_after = len({tuple(c) for c in visible_rgbs.tolist()})

    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)
    out = PROCESSED_DIR / f"{asset_name}.png"
    Image.fromarray(rgba, mode="RGBA").save(out)

    # 4x preview for eyeballing in a viewer.
    Image.fromarray(rgba, mode="RGBA").resize(
        (native_w * 4, native_h * 4), Image.NEAREST
    ).save(PROCESSED_DIR / f"{asset_name}.preview_4x.png")

    # 16x preview — useful when native is tiny (16x24 chars).
    Image.fromarray(rgba, mode="RGBA").resize(
        (native_w * 16, native_h * 16), Image.NEAREST
    ).save(PROCESSED_DIR / f"{asset_name}.preview_16x.png")

    print(
        f"OK      {asset_name}: native {arr.shape[1]}x{arr.shape[0]}, "
        f"visible px {n_visible} ({100*n_visible/arr.shape[0]/arr.shape[1]:.1f}%), "
        f"palette snap shifted {snap_pct:.2f}% of pixels, "
        f"unique visible colors: {unique_after}"
    )
    print(f"        -> processed/{asset_name}.png + preview_4x + preview_16x")


def main() -> None:
    p = argparse.ArgumentParser(
        description="Validate + normalize an AI-generated spritesheet."
    )
    p.add_argument("name", help="asset name (no extension), e.g. 'trainer_red'")
    p.add_argument(
        "--asset-class",
        default=None,
        help="layout class. inferred from name if omitted.",
    )
    p.add_argument(
        "--path", type=Path, default=None,
        help="explicit input path (defaults to art/raw/<name>.png)",
    )
    p.add_argument(
        "--strict", action="store_true",
        help="reject on dim mismatch (default: tolerant — designed for AI gen)",
    )
    args = p.parse_args()

    # asset_class can be None — get_layout infers from name.
    process(args.name, args.asset_class, args.path, args.strict)


if __name__ == "__main__":
    main()

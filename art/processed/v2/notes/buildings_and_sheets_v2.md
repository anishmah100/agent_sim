# v2 buildings + master sheets intake notes — 2026-06-03

Five single-asset buildings + market_stall sheet + 6 master sheets +
window_glow processed through the new `v2_building_freeform` and
`v2_master_sheet_freeform` asset classes (preserve source resolution,
tile-palette snap, magenta key).

## Per-asset intake commands

```bash
# Single-asset buildings — preserve source, palette snap + magenta key,
# then tight-crop to content.
for src in blacksmith town_hall granary watchtower well; do
    python3 art/intake.py "v2_$src" --path "art/raw/v2/${src}.png" \
        --asset-class v2_building_freeform
done
python3 art/crop_to_content.py v2_blacksmith v2_town_hall v2_granary v2_watchtower v2_well

# Master sheets — same intake, no tight crop (we slice instead).
for src in market_stall window_glow interior_props_master \
           resources_world_master items_master_v2 fx_particles \
           ui_icons construction_stages; do
    python3 art/intake.py "v2_$src" --path "art/raw/v2/${src}.png" \
        --asset-class v2_master_sheet_freeform
done
python3 art/crop_to_content.py v2_window_glow  # single sprite

# Slice master sheets into per-cell PNGs using the names manifests
# (art/manifests/v2_<name>_names.json) for semantic filenames.
python3 art/slice_master_sheet.py v2_market_stall            --rows 2 --cols 6
python3 art/slice_master_sheet.py v2_interior_props_master   --rows 6 --cols 8
python3 art/slice_master_sheet.py v2_resources_world_master  --rows 5 --cols 8 --min-row-gap 8
python3 art/slice_master_sheet.py v2_items_master_v2         --rows 8 --cols 8
python3 art/slice_master_sheet.py v2_fx_particles            --rows 6 --cols 8
python3 art/slice_master_sheet.py v2_ui_icons                --rows 8 --cols 8
python3 art/slice_master_sheet.py v2_construction_stages     --rows 3 --cols 4
```

## Output

| Asset                       | Source        | Processed    | Slices  |
|-----------------------------|---------------|--------------|---------|
| v2_blacksmith               | 1254×1254     | 1209×1236    | n/a (one sprite) |
| v2_town_hall                | 1619×971      | 1551×949     | n/a |
| v2_granary                  | 1024×1536     | 782×1424     | n/a |
| v2_watchtower               | 887×1774      | 773×1721     | n/a |
| v2_well                     | 1254×1254     | 631×769      | n/a |
| v2_window_glow              | 1254×1254     | 1019×1035    | n/a |
| v2_market_stall             | 2172×724      | 2172×724     | 12 cells |
| v2_interior_props_master    | 1448×1086     | 1448×1086    | 48 cells |
| v2_resources_world_master   | 1448×1086     | 1448×1086    | 40 cells (row 6 empty) |
| v2_items_master_v2          | 1254×1254     | 1254×1254    | 64 cells |
| v2_fx_particles             | 1448×1086     | 1448×1086    | 48 cells |
| v2_ui_icons                 | 1254×1254     | 1254×1254    | 64 cells |
| v2_construction_stages      | 1448×1086     | 1448×1086    | 12 cells |

## Quirks logged

- `resources_world_master` row 0 trees are 2 tiles tall. The cluster
  detector found 5 row bands (one of which is the 2-tile tree row at
  the top). The row 6 in the prompt was deliberately left empty —
  manifest only contains 40 cells (rows 0–4), not 48.
- `ui_icons` icons sit on small white cream cards. When the per-cell
  content bbox is computed, a few cream pixels from the NEIGHBORING
  cell occasionally bleed into the top edge. Minor visual artifact —
  invisible at the rendered icon size (16–32 px in the DOM), so left
  uncorrected for v2. If it shows up at full UI scale, manually crop
  ~4 pixels off the top in Aseprite per icon.
- Similar minor top-edge bleed on a few `items_master_v2` cells
  (especially weapons at the top of their cells).
- `fx_particles` row 0 cells 0-2 came back as crescent moons instead
  of sword-slash arcs. They still read as "swooping crescent" — kept
  in the manifest as `sword_arc_0/1/2` because the renderer's
  intended visual purpose (a swept-arc particle that fades) is
  preserved.
- `construction_stages` returned cleanly — all 12 cells matched the
  spec. The "blueprint ghost" stage uses translucent white pixels,
  not a fade — this is the right pixel-art approach.
- `market_stall` returned both open (top row) and closed (bottom
  row) states cleanly with all 6 awning color variants distinct.

## Naming maps

Each master sheet has an accompanying `art/manifests/v2_<name>_names.json`
that maps "row,col" → semantic name. Hand-authored, not derived from
the prompt's grid coordinates (since DALL-E's grid is approximate).
The slicer reads this file and uses the names as output filenames.

## Per-asset processing principle

NO downsampling. NO singular pipeline. Each sheet passed through
intake with its specific asset class, then sliced/cropped based on
its specific structure. Per-asset quirks logged above — re-runs
reproduce the exact output.

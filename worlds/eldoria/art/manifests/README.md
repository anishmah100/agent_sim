# Sprite manifests

One JSON manifest per processed tileset, declaring every named sprite within. Built AFTER intake.py produces the native-resolution PNG; the manifest is what the renderer + engine consume at runtime.

## Schema (per-sprite entry)

```json
{
  "name": "oak_apple",
  "source_rect": [x, y, w, h],
  "footprint_tiles": [w, h],
  "sprite_size_tiles": [w, h],
  "anchor": "bottom_center",
  "interactable_tile": null,
  "tags": ["tree", "harvestable"]
}
```

- **`source_rect`**: pixel rectangle inside the processed PNG (native coords). Where to slice the texture.
- **`footprint_tiles`**: walkability + collision footprint in tile units. A small bush = `[1, 1]`. An oak tree = `[2, 2]`. A small house = `[3, 3]`. The engine blocks movement through these tiles.
- **`sprite_size_tiles`**: visual rendering size in tile units. Almost always **taller** than the footprint for buildings + trees, because the roof/façade/canopy extends UP above the footprint. The sprite's bottom edge aligns with the footprint's bottom edge.
- **`anchor`**: how to position the sprite. Default `bottom_center`.
- **`interactable_tile`** (optional): for buildings, which tile within the footprint triggers a `interact` (e.g. door tile → portal to interior).
- **`tags`**: scenario-side filters ("tree", "vendor", "decorative", "harvestable").

## Convention: bottom-aligned sprites with extra vertical headroom

```
   sprite_size_tiles = [3, 5]              footprint_tiles = [3, 3]
   ┌─────────────────┐                    
   │                 │     <- roof extends above footprint (visual only)
   │   ┌───────┐     │
   │   │       │     │
   │   │       │     │     <- façade (visual only)
   │   │ DOOR  │     │
   │   └───────┘     │
   │  ▓▓▓▓▓▓▓▓▓▓▓    │     <- footprint (collision)  ← bottom-aligned
   │  ▓▓▓▓▓▓▓▓▓▓▓    │
   │  ▓▓▓▓▓▓▓▓▓▓▓    │
   └─────────────────┘
```

Why this works for the reference-style top-down:
- Player walks AROUND the rectangular footprint
- When the player is south of the footprint, Y-sort draws them ON TOP of the building (looks like standing in front)
- When the player is north of the footprint, they draw BEHIND the building's roof — looks like they walked behind it
- We never need to draw a "side view" of the building. The flat front face works for any approach direction.

## Status

- `overworld_tileset.json` — TBD (grid of 64 tiles)
- `tileset_vegetation.json` — TBD (mixed sizes)
- `tileset_building.json` — TBD (mixed sizes, biggest variety in footprint)
- `interior_tileset.json` — TBD
- `items_master.json` — TBD (all 1×1, just naming the 64 cells)

Manifests will be hand-authored from screenshot inspection of the processed sheets. They're committed alongside the processed PNGs and shipped to the frontend (via the build atlas) + the engine (for the multimodal rasterizer).

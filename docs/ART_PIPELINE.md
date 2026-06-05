# Art Pipeline — adding, replacing, swapping sprite sets

agent_sim's frontend talks to art exclusively through the **art
catalog** at `art/manifests/sprites.json`. One declarative file lists
every sprite id, where its PNG lives, how big it is, whether it's
enterable, etc. Resolvers (`Decoration.spriteUrl`,
`Entity.worldObjectSpriteUrl`, the interior layer, …) just call
`ArtCatalog.url(id)` — they don't know about path templates.

## Adding a single new sprite

1. Drop the PNG somewhere under `art/processed/`.
2. Regenerate the catalog so the new file is auto-detected:

   ```
   cd engine
   go run ./cmd/genart_manifest -art ../art \
     -out ../art/manifests/sprites.json \
     -overrides ../art/manifests/sprites.overrides.json
   ```

3. If the sprite needs hand-tuning the auto-detector won't infer
   (footprint, enterable flag, interior template, custom label),
   add an entry in `art/manifests/sprites.overrides.json` — the
   generator merges it on top of the auto-detected catalog so a
   re-run never blows it away.

   ```jsonc
   {
     "bld:custom_palace": {
       "path": "v2_custom_palace.png",
       "label": "The Imperial Palace",
       "footprint_tiles": [8, 3],
       "render_height_tiles": 6,
       "enterable": true,
       "interior_template": "town_hall"
     }
   }
   ```

4. Reference the sprite id (`bld:custom_palace`) in the world JSON
   / generator / wherever it's needed. No code changes.

That's it. The frontend's `ArtCatalog.url("bld:custom_palace")` now
returns the correct URL; click-to-enter fires the right interior
template; the renderer scales it to the declared footprint.

## Swapping the entire sprite set (themes / packs)

Drop a different pack of PNGs under `art/processed/packs/<name>/`,
write a pack manifest, point the frontend at it.

`art/manifests/sprites.medieval.json`:

```jsonc
{
  "$schema": "agent_sim/art/sprites/v1",
  "extends":   "sprites.json",              // inherit the default catalog
  "base_path": "packs/medieval",            // pack PNGs live under here
  "sprites": {
    // Override the cottage so it points at the medieval pack PNG.
    "bld:000": {
      "path": "cottage_red.png",
      "native_size_px": [256, 192]
    },
    // Brand-new sprite present only in this pack.
    "bld:keep": {
      "path": "keep.png",
      "native_size_px": [512, 400],
      "footprint_tiles": [4, 3],
      "render_height_tiles": 5,
      "enterable": true,
      "interior_template": "town_hall"
    }
  }
}
```

Then build the frontend with the pack manifest selected:

```
VITE_ART_MANIFEST=sprites.medieval.json npm run dev
```

The catalog walks the extends chain bottom-up, so the medieval pack's
entries win over `sprites.json` for any ids it covers, and everything
else still resolves from the default catalog. Chain depth capped at 4
to keep cycles bounded.

## Removing a sprite

1. Delete the PNG.
2. Remove its entry from `sprites.overrides.json` (if present).
3. Rerun the generator. The auto-detected catalog stops emitting the
   id; resolvers return null; world generators referencing the id
   will see warnings until updated.

## How the generator categorises files

`engine/cmd/genart_manifest/main.go:resolveCategory` is the
single mapping from on-disk path to sprite id. Highlights:

| Directory | Catalog category | Example id |
|---|---|---|
| `objects/buildings/obj_*.png` | `bld:` | `bld:000` |
| `objects/vegetation/obj_*.png` | `veg:` | `veg:003` |
| `objects/interior/obj_*.png` | `prop:` | `prop:042` |
| `objects/items/obj_*.png` | `item:` | `item:017` |
| `v2_<named>.png` (root) | `bld:` (audited names only) | `bld:blacksmith` |
| `v2_resources_world_master/*` | `veg:` | `veg:tree_oak` |
| `v2_market_stall/stall_*` | `stall:` | `stall:red_bread_open` |
| `v2_construction_stages/*` | `stage:` | `stage:cottage_stage_0_blueprint` |
| `v2_interior_tiles_master/*` | `int:` | `int:floor_wood_medium` |
| `v2_interior_props_master/*` | `prop:` | `prop:fireplace_lit` |
| `v2_items_master_v2/*` | `item:` | `item:wood_log` |
| `v2_fx_particles/*` | `fx:` | `fx:coin_sparkle_single` |
| `v2_ui_icons/*` | `ui:` | `ui:bag` |
| `frames/<char>/` + a sibling JSON | `char:` | `char:baker` |
| `tiles/interior/*` | `prop:` | `prop:anvil_sheet` |
| `.thumb.png` / `.preview_*x.png` | skipped | (derivatives) |

If you add a new top-level directory under `processed/`, also add a
case here. Otherwise the files exist on disk but the auto-detector
skips them.

## Useful invariants

- **Sprite ids are immutable.** Renaming `bld:000` to `bld:cottage_red`
  breaks every world JSON that referenced it. Add an alias entry that
  points at the same path instead.
- **Auto-detected entries are diff-stable.** The generator outputs JSON
  keys in sorted order, so checking in `sprites.json` after a regen
  produces a sensible diff.
- **Resolvers fall back to legacy paths only during the migration.**
  Once everything in use is in the catalog, the fallback in
  `Decoration.spriteUrl()` can go.
- **The catalog is loaded once at boot.** No way to swap packs mid-run
  today; reload the page to pick up a different `VITE_ART_MANIFEST`.

## Quick troubleshooting

- "sprites manifest 404" — the engine isn't serving `/art/`, or the
  manifest path is wrong. Check `art/manifests/<name>` exists.
- Sprite renders as a coloured placeholder — the catalog didn't have
  the id and the legacy fallback couldn't resolve it either. Check
  `cat.url("bld:thatId")` in the browser console.
- Sprite stretches weirdly — the `native_size_px` in the catalog
  doesn't match the PNG, or `footprint_tiles` is forcing a width that
  doesn't honour the native aspect. Regenerate the catalog (the
  auto-detector reads PNG headers) and check the relevant entry.

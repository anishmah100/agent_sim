# v2 art prompts — comprehensive set

Second-round prompts. This set covers EVERYTHING needed to retire the
placeholder rectangles and pixel-rect Graphics in the renderer, plus
new village content (more NPCs + new building types + interior props
+ FX + UI icons). Generated 2026-06-03.

After this set is in, we should not need more bulk art generation
until a third scenario beyond fantasy_town. Individual asset patches
might still happen as a system finds a gap.

## Discipline (the only rules that matter)

1. **Assume nothing about the output sheet structure.** DALL-E will
   misread the requested grid. Open each result in a viewer, find
   the real cell boundaries by eye, crop manually.
2. **Each sprite is processed individually.** No batch script. Open
   each, identify its kind, apply per-asset palette snap + magenta
   tolerance + pixel-level fixups.
3. **Inspect against the existing world.** A new asset that looks
   fine in isolation can clash with the surrounding tiles. Always
   composite into a test scene before declaring it done.
4. **No naming-scheme assumptions.** You'll likely save files with
   ad-hoc names. The intake pass will inspect `art/raw/` and figure
   out which prompt each file satisfies, not the other way around.
5. **NO DOWNSAMPLING.** Generated sheets are 8× scale; process at
   that scale, slice at that scale, save processed sprites at
   8× output. The renderer does the final scale-down at draw time.
6. **Per-asset NOTES.md.** After processing, drop a
   `art/processed/v2_<asset>/NOTES.md` with the per-asset tweaks
   used so future re-runs reproduce.

## Comprehensive prompt list

### Characters (full 1024×480 walk + action sheets)
| Prompt | NPC role |
|---|---|
| `blacksmith_character.md` | Burly forge-keeper with hammer |
| `woodcutter_character.md` | Wiry lumberjack with axe |
| `mason_character.md` | Stonemason with trowel |
| `mayor_character.md` | Robed civic official with cane |
| `drifter_character.md` | Rogue with sheathed dagger |
| `goblin_character.md` | Hostile small creature with rusty sword |

### Buildings (per-asset sized to footprint)
| Prompt | Size | Role |
|---|---|---|
| `blacksmith.md` | 384×384 (3×3 tiles) | Forge with anvil out front |
| `town_hall.md` | 640×384 (5×3 tiles) | Civic building w/ flag |
| `market_stalls.md` | 768×256 (6 stalls × 2 states) | Stall sheet |
| `well.md` | 128×128 (1×1 tile) | Village square centerpiece |
| `granary.md` | 256×384 (2×3 tiles) | Cylindrical food silo |
| `watchtower.md` | 256×512 (2×4 tiles) | Stone defensive tower |

### Master sprite sheets (large gridded outputs)
| Prompt | Size | Cell count | Purpose |
|---|---|---|---|
| `interior_props_master.md` | 1024×768 | 48 (8×6) | Furniture + decor + lighting |
| `resources_world_master.md` | 1024×768 | 48 (8×6) | Trees + rocks + bushes + flowers as entities |
| `items_master_v2.md` | 1024×1024 | 64 (8×8) | All inventory items |
| `fx_particles_master.md` | 1024×768 | 48 (8×6) | Combat + harvest + smoke + economy FX |
| `construction_stages.md` | 1024×768 | 12 (4×3) | Blueprint → built progression |
| `ui_icons.md` | 512×512 | 64 (8×8) | Toolbar + status + HUD icons |

### Standalone effects
| Prompt | Size | Purpose |
|---|---|---|
| `window_glow_fx.md` | 128×128 | Nighttime additive window overlay |

## Save paths (suggested but not required)

The intake pass inspects `art/raw/` and identifies files by content,
not filename. Use any name that makes sense to you when saving.
Suggested patterns:

| Prompt | Suggested raw filename |
|---|---|
| `blacksmith_character.md` | `blacksmith_character.png` |
| `woodcutter_character.md` | `woodcutter_character.png` |
| `mason_character.md` | `mason_character.png` |
| `mayor_character.md` | `mayor_character.png` |
| `drifter_character.md` | `drifter_character.png` |
| `goblin_character.md` | `goblin_character.png` |
| `blacksmith.md` | `v2_blacksmith.png` |
| `town_hall.md` | `v2_town_hall.png` |
| `market_stalls.md` | `v2_market_stalls.png` |
| `well.md` | `v2_well.png` |
| `granary.md` | `v2_granary.png` |
| `watchtower.md` | `v2_watchtower.png` |
| `interior_props_master.md` | `interior_props_master.png` |
| `resources_world_master.md` | `resources_world_master.png` |
| `items_master_v2.md` | `items_master_v2.png` |
| `fx_particles_master.md` | `fx_particles_master.png` |
| `construction_stages.md` | `construction_stages.png` |
| `ui_icons.md` | `ui_icons.png` |
| `window_glow_fx.md` | `v2_window_glow.png` |

If you save with different names, intake will figure it out. The
master sheets are uniquely recognizable by their distinctive grids.

## Workflow after generation

1. Paste all files into `art/raw/` (any names).
2. Run nothing automatically. The intake for this round is interactive
   in the next session — I'll open each file, identify the prompt,
   crop manually, color-correct, drop processed sprites + NOTES.md.
3. Once all processed, I wire each asset into its consuming subsystem
   one commit per asset family (characters → CharacterAtlas, interior
   props → Interior.ts, etc.).

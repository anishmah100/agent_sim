# v2 character intake notes — 2026-06-03

Six new NPC sheets processed via the existing `character_full_v3_upscaled`
class (preserve source resolution) and the new content-aware
`slice_character.py`.

## Per-asset intake commands

```bash
# Same recipe for each — only `name` differs.
python3 art/intake.py blacksmith_npc --path art/raw/v2/blacksmith_guy.png --asset-class character_full_v3_upscaled
python3 art/intake.py woodcutter     --path art/raw/v2/woodcutter.png    --asset-class character_full_v3_upscaled
python3 art/intake.py mason          --path art/raw/v2/mason.png         --asset-class character_full_v3_upscaled
python3 art/intake.py mayor          --path art/raw/v2/mayor.png         --asset-class character_full_v3_upscaled
python3 art/intake.py drifter        --path art/raw/v2/drifter.png       --asset-class character_full_v3_upscaled
python3 art/intake.py goblin         --path art/raw/v2/goblin.png        --asset-class character_full_v3_upscaled

# Slice each sheet into 20 individual frames (4 walk × 4 dirs + 4 action).
python3 art/slice_character.py blacksmith_npc
python3 art/slice_character.py woodcutter
python3 art/slice_character.py mason
python3 art/slice_character.py mayor      # used split fallback at y=704
python3 art/slice_character.py drifter
python3 art/slice_character.py goblin
```

## Why slice_character.py is content-aware, not grid-uniform

DALL-E does NOT respect the 5×4 grid in the prompt:
- All sheets have a wide pure-magenta margin (~366 px) before the
  first character even appears in row 0.
- Mayor's action row (row 4) is placed with only a 1-pixel magenta
  gap between it and row 3 (walk_right) — naive row clustering
  merged them.

The new slicer:
1. Projects alpha row-presence → clusters into 5 row bands using a
   min-gap threshold.
2. Fallback: if too few bands, splits the tallest band at the y
   with lowest content density (within the middle 50% of the band).
   This is what rescued the mayor.
3. Per row band, projects alpha column-presence → clusters into 4
   column bands. Each row's columns can have different positions
   (DALL-E shifts characters across rows).
4. Per (row, col), tight bbox → variable-width frame, uniform height.

Output frames match the existing trainer_red contract: variable
width per pose (≈100–390 px wide) at a uniform height per character
(170–180 px). CharacterAtlas loads them as separate textures.

## Visual confirmation

Each character's `walk_down_0.png` (idle facing south) was visually
inspected post-slice. All six render as recognizable, on-prompt:

- **blacksmith_npc**: burly bearded man, red sweatband, leather
  apron, hammer in right hand. Brown beard, off-white tunic.
- **woodcutter**: red hair + stubble, red+black flannel, axe on
  shoulder, green trousers, brown boots.
- **mason**: dark hair, gray tunic + leather apron, trowel in
  right hand, slate-blue trousers.
- **mayor**: gray hair + beard, deep blue robe with gold trim,
  cane in right hand, dignified.
- **drifter**: dark hair in a braid, forest-green hooded jerkin
  (hood thrown back), sheathed dagger at belt.
- **goblin**: muddy-green skin, red loincloth, leather scraps,
  rusty jagged sword, snarling.

## Output

| Character | Sheet (out) | Frames (out)                                 |
|---|---|---|
| blacksmith_npc | 1831×859 RGBA, 21 colors | 20 frames, 70–392 px wide × 176 high |
| woodcutter     | 1831×859 RGBA, 20 colors | 20 frames × 172 high                 |
| mason          | 1831×859 RGBA, 22 colors | 20 frames × 155 high                 |
| mayor          | 1831×859 RGBA, 23 colors | 20 frames × 171 high                 |
| drifter        | 1829×860 RGBA, 21 colors | 20 frames × 150 high                 |
| goblin         | 1831×859 RGBA, 21 colors | 20 frames × 138 high                 |

All registered in `art/manifests/characters.json` so CharacterAtlas
loads them on the next frontend boot.

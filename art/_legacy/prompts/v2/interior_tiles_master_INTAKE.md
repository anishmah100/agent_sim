# Intake runbook — `interior_tiles_master.png`

After ChatGPT produces the sheet from `interior_tiles_master.md`,
follow this checklist. **Stop and inspect after every step.**

## Operating principle

There is no shared "cleanup script" for these tiles. Each cell is its
own problem. The strip_* scripts in `art/` are starting points — you
may end up writing a custom one-off for an individual tile because its
particular failure mode (e.g. a magenta pixel embedded INSIDE the
rug's gold border, an asymmetric off-by-one column on just the
arched-window wall) doesn't match what any of the existing scripts
look for.

The right loop is:

```
1. open the tile, look at it at native size AND tiled 4×4
2. pick the ONE specific problem you see (a halo? a missing column?
   a magenta speck inside a window pane?)
3. write the smallest script that fixes only that problem on only
   that file
4. run it, look again
5. either accept or iterate on the script
6. move to the next tile
```

If you find yourself wanting to write a script that operates on the
whole folder, stop. That's the failure mode this rule exists to
prevent (`MEMORY.md` → [Sprite handling](feedback_sprite_handling.md)).

Per-tile inspection is not the slow path — it is the path. The lower-
hygiene "run a script over the folder" approach burned hours during
the v2 resource cleanup and produced edge artifacts we found later in
the screenshot review.

---

## Step 0 — Paste + first look

1. Save the generated PNG to `art/raw/v2/interior_tiles_master.png`.
2. Open it in an image viewer. Confirm at the sheet level:
   - Image is **1024 × 768** (8 × 6 grid).
   - Background between cells is solid magenta `#FF00FF`.
   - All 48 cells are populated except row 6 which is intentionally magenta.
   - No off-magenta speckle near cell edges (would break magenta-keying).

If any of those fail, re-prompt before touching the pipeline.

## Step 1 — Magenta-key + intake (sheet-wide; same for every tile)

This is the one operation that's truly uniform — magenta → alpha=0.
It's the only blanket step in this runbook.

```sh
cd ~/projects/agent_sim
python3 art/intake.py --input art/raw/v2/interior_tiles_master.png \
                      --output art/processed/v2_interior_tiles_master.png
```

Verify on the master output:
- Alpha is set where the input was magenta.
- No magenta pixels remain in the output (check the corners).

## Step 2 — Slice the master into per-cell PNGs

Write the names manifest, then slice. This is also uniform — the
slicer just cuts on a known grid.

```sh
cat > art/manifests/v2_interior_tiles_master_names.json <<'JSON'
{
  "0_0": "floor_wood_light",
  "0_1": "floor_wood_medium",
  "0_2": "floor_wood_dark",
  "0_3": "floor_wood_diagonal",
  "0_4": "floor_stone_large",
  "0_5": "floor_flagstone",
  "0_6": "floor_marble",
  "0_7": "floor_checker",

  "1_0": "wall_wood_light",
  "1_1": "wall_wood_dark",
  "1_2": "wall_wood_window",
  "1_3": "wall_wood_picture",
  "1_4": "wall_stone",
  "1_5": "wall_stone_torch",
  "1_6": "wall_stone_banner",
  "1_7": "wall_stone_arch_window",

  "2_0": "rug_red_l",
  "2_1": "rug_red_m",
  "2_2": "rug_red_r",
  "2_3": "rug_blue_l",
  "2_4": "rug_blue_m",
  "2_5": "rug_blue_r",
  "2_6": "rug_red_solid",
  "2_7": "rug_bearskin",

  "3_0": "door_wood_plain",
  "3_1": "door_wood_paneled",
  "3_2": "door_wood_ajar",
  "3_3": "door_wood_double",
  "3_4": "door_wood_arched",
  "3_5": "door_stone_arch",
  "3_6": "door_iron_bound",
  "3_7": "door_trapdoor",

  "4_0": "decor_doormat_straw",
  "4_1": "decor_doormat_welcome",
  "4_2": "decor_floorboard_knot",
  "4_3": "decor_stone_cracked",
  "4_4": "decor_spill_wine",
  "4_5": "decor_spill_water",
  "4_6": "decor_straw_scatter",
  "4_7": "decor_coins_drop"
}
JSON

python3 art/slice_master_sheet.py v2_interior_tiles_master --rows 6 --cols 8
```

Output: `art/processed/v2_interior_tiles_master/<name>.png`.

## Step 3 — Per-tile inspection loop

**This is the bulk of the work.** Start with one tile and iterate. Do
not pre-batch.

Suggested order (high-impact first so you can decide whether the
sheet is salvageable before processing every cell):

1. `floor_wood_medium` — the most-used tile. If this doesn't tile
   seamlessly the sheet has to be re-prompted; no script can fix a
   genuinely non-tileable image.
2. `wall_wood_light` — second most-used. Same gate.
3. `door_wood_plain` — visual gate for the standalone door variants.
4. `rug_red_l`, `rug_red_m`, `rug_red_r` — verify the three connect.
5. Everything else.

For each tile:

### 3a. Look

Open the tile at 1× (native 16×16) AND at 8× zoom. Also create a 4×4
tiled preview. The defects that matter aren't always visible at
single-tile zoom.

A 4×4 tile preview without scripting:

```sh
python3 -c "
from PIL import Image
src = Image.open('art/processed/v2_interior_tiles_master/floor_wood_medium.png').convert('RGBA')
out = Image.new('RGBA', (src.width*4, src.height*4))
for x in range(4):
    for y in range(4):
        out.paste(src, (x*src.width, y*src.height))
out.resize((out.width*4, out.height*4), Image.NEAREST).save('/tmp/preview.png')
"
xdg-open /tmp/preview.png
```

### 3b. Diagnose

Name the specific defect in one sentence. Examples:
- "One column on the right edge is one shade darker than the matching column on the left → seam."
- "There's a magenta pixel inside the window's mullion that the magenta-key kept."
- "The rug's gold border is broken at x=15, y=8."

If you can't name it in one sentence, look longer.

### 3c. Decide

Pick the smallest fix:

- **Sometimes one of the existing scripts is the right tool, used on
  ONE file.** E.g. a wood floor that has a faint white halo is what
  `strip_white_border.py --dry-run <one file>` is built for.
- **Sometimes none of them are.** A magenta pixel embedded in window
  glass is a 5-line custom script.
- **Sometimes the fix is "manually patch the PNG in an image editor."**
  A single mis-placed pixel doesn't deserve any script at all.

Examples of one-off custom fixes you might write:

```sh
# Fix: stamp the bottom-left corner of door_wood_plain transparent —
# it has a stray brown speck the magenta-key missed
python3 -c "
from PIL import Image
im = Image.open('art/processed/v2_interior_tiles_master/door_wood_plain.png').convert('RGBA')
px = im.load()
px[0, 15] = (0,0,0,0)
im.save('art/processed/v2_interior_tiles_master/door_wood_plain.png')
"

# Fix: copy the LEFT column to the RIGHT column on floor_marble to
# force seamlessness — the right edge was 1 px darker than the left
python3 -c "
from PIL import Image
im = Image.open('art/processed/v2_interior_tiles_master/floor_marble.png').convert('RGBA')
px = im.load()
for y in range(im.height):
    px[im.width-1, y] = px[0, y]
im.save('art/processed/v2_interior_tiles_master/floor_marble.png')
"
```

These ad-hoc one-liners belong inline in the runbook log, not in a
"big strip script that handles 48 cases."

### 3d. Verify

Re-run the 4×4 preview. Confirm the named defect is gone AND no new
defect appeared (the floor's interior is still recognizable, the
rug's pattern is still correct, etc.).

### 3e. Move on

Repeat 3a–3d for the next tile.

## Step 4 — Wire into the renderer

After all per-tile inspections pass:

In `frontend/src/render/Interior.ts`, the `TILE(name)` resolver
currently points at `art/processed/tiles/interior/<name>.png`. Change
it to point at `art/processed/v2_interior_tiles_master/<name>.png`,
and remap the names if needed (e.g. old `floor_wood` → new
`floor_wood_medium`, old `wall_wood` → new `wall_wood_light`).

## Step 5 — Renderer-level acceptance gate

1. `./start.sh`, click into each interior type (cottage / tavern /
   blacksmith / town hall). Confirm no missing-tile fallbacks
   (purple-checker placeholders).
2. Screenshot each interior. Compare to the screenshots that prompted
   this work — the new tiles should look like they belong in the same
   world as the furniture, not below it.
3. Pay special attention to the seams:
   - Floor across the entire room (no visible grid).
   - Wall row across the top of the room (continuous).
   - Rug runner across its three tiles (continuous).
   - Door reads as a door at game zoom (3× nominal).

If any of those fail, the right move is back to step 3, NOT a sweep
across all files.

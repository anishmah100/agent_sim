# `interior_tiles_master` — floors, walls, rugs, doors (REGEN)

The master sheet that replaces the procedurally-drawn floor + wall +
rug + door tiles in the interior renderer. These tiles must SEAMLESSLY
TILE so a 14×10 room of wood floor doesn't show a visible grid; the
walls must run continuously across the top of a room.

This is **REGENERATION v2.** The first attempt failed on two specific
constraints, called out below in the prompt itself.

**Save the result to:**

```
~/projects/agent_sim/art/raw/v2/interior_tiles_master.png
```

---

Generate a single PNG: a top-down pixel-art **interior tile master sheet** in the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS, 2009). Floors, walls, rugs, doors, and floor decorations for indoor rooms. Same visual fidelity as the existing `interior_props_master` and `resources_world_master` sheets — crisp pixel art with 1 px shadows + readable highlights.

**Image size: 1024 × 768 px.** 8 columns × 6 rows = 48 cells. Each cell is **128 × 128 px in the output**, representing one **16 × 16 native game tile** drawn at 8× nominal scale.

---

## THIS IS A REGENERATION — read these failure modes first

The previous attempt failed on two specific constraints. Both must be fixed in this attempt; everything else stayed roughly correct.

### ❌ Constraint 1 — NO CELL BORDERS, FRAMES, OR OUTLINES around individual cells

The previous attempt drew each cell like a trading card with a ~2 px dark border framing the content. **This kills tile-ability.** When two copies of a floor tile sit side-by-side, you can see two dark vertical lines down the join.

Fix:

- Do **NOT** draw a frame, card-border, drop-shadow, or any kind of outline around each cell.
- The cell *content* should run all the way to the edges of the 128×128 cell area.
- The boundary between cell-content and the magenta gutter must be a hard color transition with no intermediate pixels — no fade, no glow, no shadow.
- The cell *content* may have its own internal pixel-art outlines (e.g. the iron strap on a door is outlined; the mullions of a window are outlined). That's correct. What's wrong is an outline around the WHOLE cell.

Self-test you can run on your own output before returning it:

> Crop just one floor cell (say wood floor medium). Paste 4 copies in a 2×2 grid touching each other. Look at the 4 internal seams. **You should NOT be able to see where one tile ends and the next begins.** If you can see any seam, the cell has a baked-in border — fix it.

### ❌ Constraint 2 — Background between cells must be EXACT `#FF00FF` (RGB 255, 0, 255)

The previous attempt used JPEG-soft magenta — pixel values like `(226, 14, 232)` and `(242, 2, 242)` instead of pure `(255, 0, 255)`. The downstream magenta-key requires exact match.

Fix:

- Every gutter pixel between cells must be RGB exactly `(255, 0, 255)`.
- Encode the output as a **PNG with no lossy compression.** No JPEG anywhere in the pipeline.
- The 4 px gutter between cells must be a uniform block of pure magenta. No noise, no gradient, no softening, no anti-aliasing between magenta and any sprite color.
- Inside each cell, NEVER use the color `(255, 0, 255)` — if a cell happens to need bright magenta-pink, shift it to `(238, 24, 200)` or any nearby color that's clearly NOT pure magenta.

### Constraint 3 — Tile-edge matching, where required

For cells in **Row 1 (floors)** and **Row 3 (rugs)**, the cell must seamlessly tile with copies of itself or its neighbors:

- **Row 1, every cell:** the rightmost column of pixels must visually match the leftmost column. The bottom row must match the top row. When 4 copies of the same cell are arranged in a 2×2 grid, the result must look like one continuous floor.
- **Row 2 (walls):** each cell only tiles horizontally — the rightmost column matches the leftmost column. Top and bottom edges do NOT need to match (top edge is where the wall meets the ceiling shadow, bottom edge is the baseboard — those are intentional and asymmetric).
- **Row 3 cells 17–19 (red rug):** rightmost column of cell 17 must match leftmost column of cell 18. Same for 18 → 19.
- **Row 3 cells 20–22 (blue rug):** same chain.
- **Row 3 cells 23–24 (single-tile rugs):** don't need to tile.
- **Row 4 (doors):** don't need to tile. Single use each.
- **Row 5 (decorations):** don't need to tile. Single overlays.

### Constraint 4 — Style (largely unchanged from first attempt)

- Crisp pixel art, **no anti-aliasing**, 1 px hard outline on the edges of any 3D feature (mortar joints, wall trim, rug border).
- Slight 3/4 perspective — for floors, top-down with a faint suggestion of grain direction; for walls, you see the FRONT FACE of the wall as if looking at the back of a room.
- Constrained palette (~6–10 colors per tile) inspired by Endesga 32: warm browns for wood, slate blues for stone, deep reds + cream for rugs, brass/gold for door fittings, soft sky blue for window panes.
- Drop shadows live INSIDE the cell (no shadow protruding past the cell boundary into the magenta gutter).

---

## Sheet layout — 48 cells, top-to-bottom, left-to-right

### Row 1 — FLOORS (cells 1–8) — must seamlessly tile in all 4 directions

1. **Wood floor — light warm honey planks.** Three horizontal planks per tile, subtle grain. Color: warm tan (~#c08560). Faint plank seams ≤1 px wide. The plank seams must wrap around — the seam at the top must continue at the bottom.
2. **Wood floor — medium brown planks.** Same plank layout as #1 but darker (~#9a6045). The most common cottage floor.
3. **Wood floor — dark walnut planks.** Almost-black brown with red undertones (~#5a3520). For studies and grand halls.
4. **Wood floor — diagonal planks.** Same medium tone as #2 but planks run on a 45° diagonal — for a single-cell accent.
5. **Stone floor — large gray slabs.** Two stones per tile (cobblestone style). Cool slate blue-gray (~#7f8696). Mortar lines 1 px wide. The mortar grid must wrap around the edges.
6. **Stone floor — small flagstone.** Four smaller stones per tile, more irregular shapes, mossy hint in one corner.
7. **Marble floor — cream with subtle veining.** Light parchment color with 1–2 px veins in pale gold. Suggest gloss with a single highlight pixel per tile.
8. **Checker floor — blue-gray + cream 2×2 squares.** Town-hall style. Diagonal pairs match each other; checks tile across cells.

### Row 2 — WALLS, north-facing (cells 9–16) — must seamlessly tile horizontally

9. **Wood plank wall — light.** FRONT FACE: vertical planks of light honey wood, the bottom row showing a baseboard/skirting strip. Top edge has a 1-px shadow line where wall meets ceiling.
10. **Wood plank wall — dark.** Same as #9 but in walnut tone.
11. **Wood wall with window — light wood, square window.** Centered window: blue-sky panes split into a 2×2 grid by wooden mullions. Frame in slightly darker wood than the wall.
12. **Wood wall with framed picture — landscape painting.** Centered: rectangular gold frame, inside is a tiny pixel-art landscape (green hills + blue sky + one bird silhouette).
13. **Stone wall — gray block.** Three rows of stone blocks per tile, staggered like real masonry. Slate blue-gray. Mortar in darker gray.
14. **Stone wall with iron sconce + lit torch.** Wall background as #13 but with a wrought-iron bracket holding a small orange flame. Faint warm glow on the surrounding 4 px.
15. **Stone wall with heraldic banner.** Wall background as #13 but with a hanging banner: dark red fabric, gold trim, a small sigil (chevron or sun) in the center.
16. **Stone wall with arched window.** Wall background as #13 but with an arched opening showing blue sky panes with mullions, stone arch above.

### Row 3 — RUGS (cells 17–24) — chained tiling

17. **Red Persian rug — LEFT cap.** End fringe on the LEFT edge (visible tassels). The right edge connects seamlessly to cell 18.
18. **Red Persian rug — middle.** No fringe; both left and right edges connect to other rug tiles. Center has an ornate diamond motif in gold.
19. **Red Persian rug — RIGHT cap.** End fringe on the RIGHT edge. The left edge connects seamlessly to cell 18.
20. **Blue Persian rug — LEFT cap.** Same as #17 but in deep blue with cream/gold accents.
21. **Blue Persian rug — middle.**
22. **Blue Persian rug — RIGHT cap.**
23. **Solid red oriental rug — single tile centerpiece.** A standalone 1-tile rug with ornate gold border + center medallion. For small rooms.
24. **Bear-skin rug — single tile centerpiece.** Brown fur with bear head pointing south (toward the viewer). Standalone, doesn't need to tile.

### Row 4 — DOORS (cells 25–32) — single-tile, do NOT need to tile

25. **Wood door — closed, plain.** Dark walnut planks (3 vertical), brass round doorknob on right side, two iron strap hinges (top and bottom on left side). Bottom of the door rests on a small stone threshold.
26. **Wood door — closed, paneled.** Same dark walnut but with 2 inset rectangular panels (upper + lower). Slightly fancier — for town hall / mayor's office.
27. **Wood door — slightly ajar.** Door #25 but cracked open ~10°, showing a sliver of dark interior beyond.
28. **Wood double door — closed.** Two narrow doors meeting in the middle, brass ring handles on each, suggesting a grand entrance.
29. **Wood door with arched top — closed.** Arched header above a rectangular door, light glow through a small grilled window at the top.
30. **Stone archway — open.** Just the stone arch, no door; the passage shows dark interior beyond (suggest depth). Same stone palette as the stone walls.
31. **Iron-bound door — closed.** Heavy wood door reinforced with horizontal iron straps (3 bands), large iron studs at the corners. For dungeons, forges, jails.
32. **Trapdoor — closed.** Top-down view of a square wooden trapdoor in the floor, brass ring handle, iron hinges. Same wood tone as #25 so it sits naturally on the wood floor tiles.

### Row 5 — FLOOR DECORATIONS (cells 33–40) — single-tile overlays

33. **Doormat — woven straw, rectangular.** Beige/tan, fits within a tile, suggests "wipe your feet here".
34. **Doormat — red, with "welcome" cross-stitch suggestion.**
35. **Floorboard — single loose plank with a knot.** Same color as #2 but with a knot in the wood — for variety, prevents repetition fatigue.
36. **Stone floor — single cracked slab.** Same as #5 but one of the stones has a crack running diagonally.
37. **Spill — red wine puddle.** Small dark red puddle on a wood floor.
38. **Spill — water puddle.** Small bluish puddle.
39. **Loose straw scatter.** Few wisps of straw on stone (for stables / blacksmith).
40. **Coin sparkle on floor.** Two or three gold coins dropped on the floor with a small sparkle effect.

### Row 6 — RESERVED (cells 41–48)

Leave these cells **as solid magenta** `#FF00FF` (the slicer will skip them). They're reserved for future tile additions so the slot indexing of rows 1–5 stays stable.

---

## Acceptance checklist (before returning the sheet)

1. **Cell-border check.** Crop any one floor cell. Paste 4 copies in a 2×2 grid touching each other. Look at the seams. If you see them, fix the cell — there's a baked-in border.
2. **Magenta check.** Sample any pixel in the gutter between two cells. The value must be exactly RGB `(255, 0, 255)`. If it's `(254, 1, 255)` or `(255, 0, 253)` or anything else, fix the encoding.
3. **No anti-aliasing.** Zoom into any sprite/magenta boundary. There should be a hard 1-pixel transition, not a 2–3 pixel fade.
4. **Row 6 = pure magenta.** Should look like a solid magenta band at the bottom of the image.
5. Output is a PNG, not a JPEG. No lossy compression anywhere.

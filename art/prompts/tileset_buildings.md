# `tileset_buildings` — houses, market stalls, fences, props

The structures layer. Houses, shops, fences, lamps, signs. Buildings show their FRONT FACE (3/4 perspective with a visible façade).

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/tileset_buildings.png`.**

---

Generate a single PNG: a top-down pixel-art **building / structure tileset** in the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS, 2009). Buildings, market stalls, fences, lamp posts, signs. Slight 3/4 perspective — buildings show their front face (you can see the door and windows on the front-facing side), not just a flat roof from above.

**Image size: 1024 × 1024 px.** 8× nominal scale. Buildings span multiple "tile slots." Per-slot size at output is **128 × 128 px** (16 × 16 native).

**Background where no structure exists:** solid magenta `#FF00FF`.

**Style:** Crisp pixel art. No AA. 1 px detail at native resolution. Slight 3/4 perspective so you see roof + front wall + door of each building.

**Sheet layout — top-to-bottom regions:**

**Region 1 (y=0–383, takes the top 3 rows): SMALL HOUSE — 3 tiles wide × 3 tiles tall = 384×384 in output**
Centered horizontally. A small medieval-fantasy cottage:
- Brown wooden walls
- Red tile / thatched roof (use the red palette color for tile, golden for thatch — choose one and commit)
- One door (centered, dark wood)
- Two small windows (one each side of the door, with white sashes)
- Slight stone foundation at the base

Place ONE small house at cols 1–3 of rows 1–3, then leave cols 4–8 of rows 1–3 mostly magenta.

In cols 4–6 of rows 1–3 (also 384×384), draw an **ALTERNATE small house** with a DIFFERENT color combo: gray stone walls + dark wood roof.

In cols 7–8 (256 wide × 384 tall), draw an isolated DOOR + WINDOW set as individual tiles (one door tile and one window tile in this column, stacked).

**Region 2 (y=384–639, takes 2 rows): LARGER BUILDING — 4 tiles wide × 2 tiles tall**
- Cols 1–4 of rows 4–5: a TAVERN / inn with a hanging wooden sign over the door (sign shows a stylized beer mug or just "INN" in unreadable pixel-art letters). Larger windows. Two-story hint with a second row of small windows up top. Brown wood with golden trim.
- Cols 5–8 of rows 4–5: a MARKET / shop building with a striped red-and-white awning above the door, more like an open-front market structure. Wood + canvas.

**Region 3 (y=640–767, 1 row): MARKET STALLS + WELL + props**
- Col 1: market stall A — wooden table with a striped canvas roof (red/white), no walls (you can see through it)
- Col 2: market stall B — same shape, different canvas color (blue/white)
- Col 3: well — stone rim, wooden bucket, slight wooden roof
- Col 4: lamp post — vertical wooden post with a small lit lantern at the top (yellow glow)
- Col 5: wooden sign post (blank wooden plank)
- Col 6: sign post with arrow pointing right
- Col 7: bench (wooden, 2-board seat with backrest)
- Col 8: wooden barrel

**Region 4 (y=768–895, 1 row): FENCE pieces (autotile-friendly)**
Wooden picket fence segments — these connect into longer fences:
- Col 1: fence horizontal (single segment, runs east-west)
- Col 2: fence vertical (runs north-south)
- Col 3: fence corner NE (north + east)
- Col 4: fence corner NW
- Col 5: fence corner SE
- Col 6: fence corner SW
- Col 7: fence gate (closed, has hinges)
- Col 8: fence gate (open, swung aside)

**Region 5 (y=896–1023, 1 row): DOORS + WINDOWS + roof variants**
- Col 1: large wooden door (closed)
- Col 2: large wooden door (open, you see darkness inside)
- Col 3: stone arch entrance
- Col 4: stained glass window (small blue/red tint)
- Col 5: roof tile red (single 16×16 slot — for compositing custom roofs)
- Col 6: roof tile gray slate
- Col 7: roof tile thatched golden
- Col 8: chimney (vertical, brick) with a small smoke puff visible at the top

**Color palette — use ONLY these 14 colors:**

| Use | Hex |
|---|---|
| Wood light (planks, signs) | `#b86f50` |
| Wood dark (door, beams, trim) | `#733e39` |
| Wood very dark (deep shadow) | `#3e2731` |
| Stone light | `#c0cbdc` |
| Stone shadow | `#8b9bb4` |
| Roof red tile | `#e43b44` |
| Roof red shadow | `#a22633` |
| Thatch golden | `#feae34` |
| Canvas white | `#ffffff` |
| Awning blue (alt stalls) | `#0099db` |
| Lantern light glow | `#fee761` |
| Window glass | `#2ce8f5` |
| Smoke puff | `#c0cbdc` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 1024 PNG. Magenta between all structures. No labels, no grid lines.

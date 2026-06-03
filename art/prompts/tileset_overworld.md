# `tileset_overworld` — ground, paths, water, cliffs

The overworld terrain tileset. Covers every TERRAIN tile (not trees, not buildings — those are separate sheets). Autotile-friendly so adjacent grass+dirt+path+water all blend cleanly.

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/tileset_overworld.png`.**

---

Generate a single PNG: a top-down pixel-art **terrain tileset** in the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS, 2009). This is the GROUND layer — grass, dirt, path, water, sand, cliffs. No trees, no flowers, no buildings (those are separate sheets).

**Image size: 1024 × 1024 px.** This is an 8× scaled tilesheet — each "tile" is **128 × 128 px in the output** (native = 16 × 16 px). The sheet contains a grid of **8 columns × 8 rows = 64 tiles total**.

**Background where no tile content exists:** solid magenta `#FF00FF`.

**Style:** Crisp pixel art. No anti-aliasing. Slight 3/4 top-down perspective (very slight — this is the ground viewed nearly straight down, so most tiles read flat with subtle shading hints). 1 px detail at native resolution = 8 px at 8× scale.

**Tile grid layout — 8 cols × 8 rows, left-to-right, top-to-bottom:**

**Row 1 (y=0–127): GRASS variants**
- Col 1: grass flat (base, used for most grass tiles)
- Col 2: grass tuft (small grass blades sticking up, slightly darker green)
- Col 3: grass with a small pebble
- Col 4: grass with a small mushroom (red cap with white spots)
- Col 5–8: grass autotile EDGES — top, right, bottom, left edges where grass meets dirt. The grass color fades into a bit of dirt brown along the edge.

**Row 2 (y=128–255): GRASS autotile CORNERS + transitions**
- Cols 1–4: outer corners (top-left, top-right, bottom-right, bottom-left) where grass meets dirt diagonally
- Cols 5–8: inner corners (concave) for the same transitions

**Row 3 (y=256–383): DIRT path variants**
- Col 1: dirt flat (base)
- Col 2: dirt with footprint trail (subtle)
- Col 3: dirt with small rocks
- Col 4: dirt cracked
- Col 5–8: dirt → grass autotile edges (mirror of row 1 cols 5-8)

**Row 4 (y=384–511): STONE / cobblestone path**
- Col 1: cobblestone flat (gray stones in a tight pattern)
- Col 2: cobblestone with a darker stone or two
- Col 3: cobblestone edge fade to dirt
- Col 4: cobblestone with a metal grate / drain
- Col 5–8: stone path autotile edges (top/right/bottom/left where stone meets grass)

**Row 5 (y=512–639): WATER — shallow / lake**
- Col 1–2: water flat with slight ripple variants (2 frames for animation later)
- Col 3: water with floating lily pad
- Col 4: water with a tiny fish jumping
- Col 5–8: water shore edges where water meets grass — top, right, bottom, left

**Row 6 (y=640–767): WATER autotile corners + DEEP water**
- Col 1–4: water-to-grass outer corners + inner corners
- Col 5: deep water (darker, slightly more saturated blue)
- Col 6: water with a single rock sticking out
- Col 7: water + dock plank (gray-brown wood)
- Col 8: water with a small wave foam

**Row 7 (y=768–895): SAND + BEACH**
- Col 1: sand flat (warm tan)
- Col 2: sand with shells / debris
- Col 3: sand wet (slightly darker)
- Col 4: sand → grass edge transition
- Col 5: sand → water edge (the wet shore)
- Col 6–8: sand autotile edges where sand meets grass (top, right, bottom)

**Row 8 (y=896–1023): CLIFF + ROCK**
- Col 1: cliff face front (stone wall facing south, you see the front)
- Col 2: cliff face with crack detail
- Col 3: cliff top (grass on top of a cliff — different from regular grass; slightly elevated visual hint)
- Col 4: cliff top with a small bush
- Col 5: cliff edge left (the corner where cliff face meets ground)
- Col 6: cliff edge right
- Col 7: cliff inner corner
- Col 8: solid stone (gray rock — for caves / mountainous interior)

**Color palette — use ONLY these 12 colors:**

| Use | Hex |
|---|---|
| Grass light | `#63c74d` |
| Grass shadow | `#3e8948` |
| Dirt warm | `#b86f50` |
| Dirt shadow | `#733e39` |
| Cobblestone light | `#c0cbdc` |
| Cobblestone shadow | `#8b9bb4` |
| Sand | `#ead4aa` |
| Sand shadow | `#e4a672` |
| Water light | `#2ce8f5` |
| Water mid | `#0099db` |
| Water deep | `#124e89` |
| Outline / very dark accent | `#181425` |
| Background | `#FF00FF` |

That's 12 character colors + magenta. No other colors allowed.

**Output:** 1024 × 1024 px PNG. Magenta background preserved between/around tiles. No labels, no grid lines, no annotations.

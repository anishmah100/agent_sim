# `tileset_vegetation` — trees, bushes, flowers, tall grass

The plant-life layer for the overworld. Rendered ABOVE the ground tileset.

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/tileset_vegetation.png`.**

---

Generate a single PNG: a top-down pixel-art **vegetation tileset** in the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS, 2009). Trees, bushes, flowers, and tall grass. These render ABOVE the ground tileset — they have a transparent background.

**Image size: 1024 × 1024 px.** 8× nominal scale. Each "tile slot" is **128 × 128 px in the output** (16 × 16 native). Larger objects span multiple slots — that's intentional and noted in the layout.

**Background where no plant exists:** solid magenta `#FF00FF`.

**Style:** Crisp pixel art, no AA, 1 px outline at native resolution (= 8 px at 8× scale). Slight 3/4 perspective so trees show their TRUNK in addition to canopy from above (you see the trunk at the base and the leaves as a roughly circular canopy above). Trees should look like they grow up out of the ground, not flat circles.

**Sheet layout — 8 cols × 8 rows = 64 slots:**

**Rows 1–2 (y=0–255, 8 cols × 2 rows = 16 slots): OAK TREES**
Large oak trees span **2 slots wide × 2 slots tall = 4 slots each (256 × 256 px each in output)**. Fit 4 oak trees in this region:
- Cols 1–2 + rows 1–2: oak summer (full green canopy)
- Cols 3–4 + rows 1–2: oak autumn (orange-tinged canopy)
- Cols 5–6 + rows 1–2: oak with red fruit (apple tree)
- Cols 7–8 + rows 1–2: oak shadow / leafless (dead/winter)

**Rows 3–4 (y=256–511): PINE TREES**
4 pine trees, each 2×2 slots = 256×256 in output:
- Cols 1–2: pine tall (dark green conical)
- Cols 3–4: pine snow-dusted (white snow on top)
- Cols 5–6: pine with cones (small brown cones visible)
- Cols 7–8: pine dead (gray-brown bare branches)

**Row 5 (y=512–639): BUSHES (1 slot each = 128×128 in output)**
- Col 1: small grass bush (round, green)
- Col 2: berry bush (green with red berries)
- Col 3: blueberry bush (green with blue berries)
- Col 4: thorny bush (gray-brown, jagged)
- Col 5: lavender bush (green with purple-ish flowers — wait, NO purple in this palette; use the deep red instead to suggest a flowering bush)
- Col 6: bush trimmed (round, neat — for ornamental gardens)
- Col 7: bush with single flower
- Col 8: very small grass tuft

**Row 6 (y=640–767): FLOWERS (1 slot each = 128×128 in output)**
- Col 1: red flower (5 petals, yellow center)
- Col 2: yellow flower (sunflower-style with brown center)
- Col 3: white flower (daisy-style with yellow center)
- Col 4: orange flower (5 petals)
- Col 5: small red flower cluster (3 small blooms)
- Col 6: small white flower cluster
- Col 7: dandelion (yellow puff)
- Col 8: dead flower (gray-brown wilted)

**Row 7 (y=768–895): TALL GRASS variants**
- Col 1: tall grass dense (waist-high green grass, multi-blade)
- Col 2: tall grass sparse
- Col 3: tall grass with hidden item glint (subtle white sparkle pixel)
- Col 4: cattails / reeds (taller stalks with brown tops, for water edges)
- Col 5–8: tall grass autotile edges where it meets short grass (top, right, bottom, left fade)

**Row 8 (y=896–1023): MISC PLANTS**
- Col 1: mushroom (red cap, white spots — same as in overworld but bigger so it's its own object)
- Col 2: brown mushroom (large)
- Col 3: pumpkin (orange round)
- Col 4: cabbage / lettuce (green leafy ball)
- Col 5: wheat stalk (golden, bundled)
- Col 6: corn stalk (green with yellow ears)
- Col 7: stump (cut tree, brown ring on top)
- Col 8: log (fallen, brown)

**Color palette — use ONLY these 14 colors:**

| Use | Hex |
|---|---|
| Leaf light green | `#63c74d` |
| Leaf shadow green | `#3e8948` |
| Pine dark green | `#265c42` |
| Autumn orange | `#f77622` |
| Berry / flower red | `#e43b44` |
| Flower yellow | `#feae34` |
| Pumpkin orange | `#d77643` |
| Trunk brown light | `#b86f50` |
| Trunk brown dark | `#733e39` |
| Bare wood / stump | `#3e2731` |
| White (mushroom spots, flowers) | `#ffffff` |
| Snow / frost | `#c0cbdc` |
| Wheat golden | `#fee761` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 1024 PNG. Magenta background between/around all objects. No labels, no grid lines.

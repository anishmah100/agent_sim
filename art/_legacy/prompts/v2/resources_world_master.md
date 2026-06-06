# `resources_world_master` — trees + rocks + bushes + flowers as ENTITIES

A master sheet for world-resource entities: things agents can chop,
mine, gather, or just walk past. These are NOT the v1 tileset_vegetation
decorations — these are first-class entities with a hardness state
and a clickable hover. The art needs to read as "you can interact
with me" + show variety so a forest doesn't look like clones.

**Save the result to:**

```
~/projects/agent_sim/art/raw/resources_world_master.png
```

---

Generate a single PNG: a top-down pixel-art **world resource entities
sprite sheet** in the visual style of **Pokémon HeartGold / SoulSilver**
(Nintendo DS, 2009). Trees, bushes, rocks, flowers, and gatherable
plants laid out in a strict grid. Slight 3/4 perspective.

**Image size: 1024 × 768 px.** 8 columns × 6 rows = 48 cells, each
128×128 px output (16×16 native at 8× scale). EXCEPT row 1 trees,
which are 2 cells tall (96×16 → 128×256 output). The grid is
non-uniform — see layout below.

**Background between cells:** solid magenta `#FF00FF`, ≥ 4 px of
magenta between every sprite.

**Style:** Crisp pixel art, no anti-aliasing, 1 px outline per
sprite, slight 3/4 perspective. Each sprite has a clear base / footprint
that grounds it on the tile.

**Sheet layout:**

**Rows 1–2 — Trees (cells 1–8, each 128 wide × 256 tall, full tree at
2 tiles tall, base on bottom):**
1. Oak — broad green canopy, sturdy brown trunk, classic shape.
2. Pine — tall narrow conical dark green, slender trunk.
3. Birch — pale white-bark trunk, thinner canopy, golden-tinged
   leaves.
4. Dead tree — bare gray trunk, no leaves, twisted branches.
5. Apple tree — green canopy with red apple specks visible.
6. Cherry tree — pink-flowered canopy.
7. Willow — drooping branches, lighter green.
8. Sapling — small tree, only 1 tile tall (centered in the 2-tile
   cell with magenta padding above).

**Row 3 — Bushes (cells 9–16, each 128×128 = 1 tile):**
9. Round shrub green
10. Round shrub dark
11. Berry bush (red berries on green leaves)
12. Berry bush (blue/purple berries)
13. Thorn bush (sharper, brown-tinged)
14. Topiary bush (geometric trimmed shape)
15. Tall grass cluster
16. Reeds / cattails (for waterside)

**Row 4 — Rocks + boulders (cells 17–24, each 128×128):**
17. Small rock (single, low, gray)
18. Small rock (cluster of 3 stones)
19. Medium boulder (gray, 1 tile fills cell)
20. Medium boulder (mossy — green patches)
21. Large boulder (with cracks suggesting it's mineable)
22. Iron ore boulder (gray with reddish-orange streaks visible)
23. Gem/crystal cluster (purple crystals jutting up)
24. Stalagmite (pointed, light stone — cave variant)

**Row 5 — Flowers + ground cover (cells 25–32, each 128×128):**
25. Red flower patch (4 small flowers on green stems)
26. Yellow flower patch
27. Blue flower patch
28. Purple flower patch
29. White flower patch
30. Sunflower (single tall yellow flower)
31. Mushroom cluster (red caps with white spots)
32. Lily pads on water (3 pads, 1 with white flower)

**Row 6 — Misc / utility (cells 33–40, each 128×128):**
33. Tree stump (chopped — replaces a chopped-down tree visually)
34. Tree stump with axe stuck in it
35. Pile of logs (cut wood, stacked)
36. Pile of stones (gathered rock)
37. Fallen tree trunk (lying horizontally)
38. Wheelbarrow with hay
39. Stack of hay bales
40. Pile of mined ore chunks

**Cells 41–48 are EMPTY** — reserved for future v3 additions
(monsters, traps, special). Leave them solid magenta.

**Color palette — use ONLY the Endesga 32 colors. Common usages:**

| Use | Hex |
|---|---|
| Tree leaf green dark | `#193c3e` |
| Tree leaf green | `#265c42` |
| Tree leaf green highlight | `#63c74d` |
| Tree leaf golden (birch) | `#feae34` |
| Tree blossom pink | `#f6757a` |
| Bark light brown | `#b86f50` |
| Bark dark brown | `#733e39` |
| Bark deep shadow | `#3e2731` |
| Dead tree gray | `#8b9bb4` |
| Apple red | `#e43b44` |
| Berry red | `#e43b44` |
| Berry blue | `#124e89` |
| Stone gray light | `#c0cbdc` |
| Stone gray | `#8b9bb4` |
| Stone shadow | `#5a6988` |
| Moss green | `#3e8948` |
| Iron ore rust | `#a22633` |
| Crystal purple | `#b55088` |
| Crystal highlight | `#f6757a` |
| Flower red | `#e43b44` |
| Flower yellow | `#fee761` |
| Flower blue | `#0099db` |
| Flower purple | `#68386c` |
| Flower white | `#ffffff` |
| Stem green | `#63c74d` |
| Mushroom red | `#e43b44` |
| Mushroom spot | `#ffffff` |
| Hay golden | `#feae34` |
| Water blue (lily) | `#0099db` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** trees in rows 1-2 are 2-tile-tall sprites with the
TRUNK BASE on the bottom row's bottom edge. The canopy fills the top
row. This base/canopy split is critical for the rendering layer to
know where the footprint tile is. All non-tree sprites are exactly 1
tile tall, centered in their 128×128 cell with ~8 px magenta padding.

**Output:** 1024 × 768 PNG, strong magenta between cells.

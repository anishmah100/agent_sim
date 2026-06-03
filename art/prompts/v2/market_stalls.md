# `v2_market_stalls` — 6 market stall variants

A 6-cell wide × 2-cell tall sheet of single-tile (16×16 native,
128×128 output per cell) market stalls. The v1 buildings tileset
already included 2 stall variants; this round adds 6 more so a market
row of 4–8 stalls doesn't look like clones. Each variant differs in
the awning color AND the goods displayed on the table — agents (and
viewers) should read the wares from the sprite alone.

Sheet output: 6 cols × 2 rows = **768 × 256 px** (12 cells, but only
the top row is used in this prompt; the bottom row carries empty /
collapsed states of the same stalls so a future "closed stall" mode
can reuse the same atlas).

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/v2_market_stalls.png`.**

---

Generate a single PNG: a top-down pixel-art **market stalls
spritesheet** in the visual style of **Pokémon HeartGold / SoulSilver**
(Nintendo DS, 2009). Twelve cells laid out in a 6×2 grid. Each cell
shows one 16×16 native sprite at 128×128 output (8× scale).

**Image size: 768 × 256 px.** No grid lines, no labels.

**Background between cells:** solid magenta `#FF00FF`. Strong magenta
border around every sprite so per-cell crops are unambiguous.

**Style:** Crisp pixel art. No anti-aliasing. 1 px detail at native
resolution. Slight 3/4 perspective on the awning — you can see the
canvas roof angled toward the camera.

**Sheet layout — TOP ROW (open stalls):**

Each open stall is:
- A wooden table base spanning the bottom 2/3 of the cell.
- A canvas awning above, angled at 3/4 perspective.
- Goods stacked on the tabletop, clearly distinct from the next
  stall's goods.

| Cell | Awning color | Goods on table |
|---|---|---|
| Col 1 | Red+white striped | Loaves of bread (golden, rounded), small basket |
| Col 2 | Blue+white striped | Fish on ice (silver, small) — fishmonger |
| Col 3 | Green+white striped | Vegetables: carrots (orange) + cabbages (green) |
| Col 4 | Solid golden | Wheels of cheese (yellow, with darker rind) |
| Col 5 | Deep purple+white | Bolts of cloth, stacked (purple, blue, red colors) |
| Col 6 | Brown leather | Hammers + horseshoes + nails — blacksmith's stall |

**Sheet layout — BOTTOM ROW (closed stalls):**

Same 6 stalls but in their "closed" state — awning rolled down,
goods cleared, table visible but empty. This is the night/no-vendor
state; agents can render it when the stall's owner is sleeping.

Same awning colors as the top row, but the awning hangs vertically
covering the goods area. No items visible on the table.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Wood light (table) | `#b86f50` |
| Wood dark (legs, frame) | `#733e39` |
| Wood very dark (shadow) | `#3e2731` |
| Canvas white | `#ffffff` |
| Awning red | `#e43b44` |
| Awning blue | `#0099db` |
| Awning green | `#3e8948` |
| Awning gold | `#feae34` |
| Awning purple | `#68386c` |
| Awning leather brown | `#5a6988` (use as muted leather; if too dark, swap to `#733e39`) |
| Bread golden | `#e4a672` |
| Fish silver | `#c0cbdc` |
| Vegetable orange | `#f77622` |
| Vegetable green | `#63c74d` |
| Cheese yellow | `#fee761` |
| Cloth bolt purple | `#b55088` |
| Cloth bolt blue | `#124e89` |
| Iron / nails | `#262b44` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** the goods on each stall should be clearly different.
The viewer should be able to tell "bread stall vs fish stall" at
thumbnail size from the colors alone.

**Output:** 768 × 256 PNG. Magenta between all cells, strong magenta
border around every sprite.

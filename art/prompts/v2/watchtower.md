# `v2_watchtower` — village defense tower

A 2×4-tile (32×64 native, 256×512 output) tall watchtower for the
edge of the village. Stone with crenellated top and a banner — reads
as "fortified / lookout" immediately.

**Save the result to:**

```
~/projects/agent_sim/art/raw/v2_watchtower.png
```

---

Generate a single PNG: a top-down pixel-art **stone watchtower** in
the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS,
2009). Slight 3/4 perspective.

**Image size: 256 × 512 px.** 8× nominal scale. 2 tiles wide × 4 tiles
tall = 256×512 output.

**Background where the tower does not cover:** solid magenta `#FF00FF`.

**Style:** Crisp pixel art, no anti-aliasing, 1 px detail at native
resolution, slight 3/4 perspective.

**Subject — a medieval stone watchtower:**

- **Tall square stone walls** — light stone with darker mortar. Walls
  occupy the lower 3 tile rows.
- **Crenellated top** — the upper 1 tile is the battlement with
  alternating stone teeth (merlons), 4 visible across the front
  face. A small banner pole protrudes from the front center merlon.
- **Banner** — small triangular cloth flag in deep red with a thin
  gold border, hanging from the banner pole.
- **Heavy wooden door** at the base, centered, with iron studs.
- **Narrow arrow slits** — 3 vertical narrow slits on the wall, one
  per tile of height (so 3 stacked up the wall), each ~2 px wide and
  4 px tall at native resolution, with a dark interior.
- **Stone buttresses** — slightly projecting stone columns at the
  corners of the front face, giving the tower a structural look.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Stone light | `#c0cbdc` |
| Stone shadow | `#8b9bb4` |
| Stone deep shadow | `#5a6988` |
| Mortar | `#3a4466` |
| Wood door | `#733e39` |
| Wood door shadow | `#3e2731` |
| Iron studs / hinges | `#262b44` |
| Arrow slit dark | `#181425` |
| Banner red | `#a22633` |
| Banner gold border | `#feae34` |
| Banner pole wood | `#b86f50` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** the silhouette should be tall and clearly fortified.
The crenellations and arrow slits are the "this is a defense tower"
signals — keep them legible at thumbnail size.

**Output:** 256 × 512 PNG, magenta only at the sky corners above the
tower roof.

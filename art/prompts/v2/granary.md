# `v2_granary` — food storage building

A 2×3-tile (32×48 native, 256×384 output) building for the village
granary. Cylindrical or tall-square stone silo with a conical wooden
roof — distinct from cottages by shape (tall + narrow) and from the
town hall (no flag, simpler).

**Save the result to:**

```
~/projects/agent_sim/art/raw/v2_granary.png
```

---

Generate a single PNG: a top-down pixel-art **grain silo / granary**
in the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo
DS, 2009). Slight 3/4 perspective — you can see the conical roof and
the cylindrical wall front.

**Image size: 256 × 384 px.** 8× nominal scale. The building fills the
image: 2 tiles wide × 3 tiles tall at 16×16 native = 256×384 output.

**Background where the building does not cover:** solid magenta
`#FF00FF`.

**Style:** Crisp pixel art, no anti-aliasing, 1 px detail at native
resolution, slight 3/4 perspective.

**Subject — a medieval-fantasy grain silo:**

- **Cylindrical stone walls** — light stone with darker mortar lines.
  Occupies the lower 2 tile rows (height-wise). Curved silhouette,
  not square — visible curvature at the top edge of the cylinder.
- **Conical wooden roof** — dark wood shingles, tapering up to a
  point. Upper 1 tile of height. A small wooden finial on top.
- **Small loading door** — about 1 tile high, dark wood, with a
  small wooden ladder leaning against the wall to the right of the
  door.
- **High loading hatch** — small square wooden hatch ~halfway up the
  silo wall (this is where grain gets loaded from a wagon).
- **Wheat bundle leaning against the base** to the left of the door
  — golden, decorative, makes it read as "grain storage".
- No windows (granaries don't have them).

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Stone light (walls) | `#c0cbdc` |
| Stone shadow | `#8b9bb4` |
| Stone curvature shadow (right side) | `#5a6988` |
| Mortar | `#3a4466` |
| Wood roof dark | `#733e39` |
| Wood roof highlight | `#b86f50` |
| Wood roof shingles | `#3e2731` |
| Roof finial gold | `#feae34` |
| Wood door | `#733e39` |
| Hatch wood | `#5a6988` |
| Ladder wood | `#b86f50` |
| Wheat bundle gold | `#feae34` |
| Wheat shadow | `#d77643` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** the cylindrical silhouette must be clear — viewers
should see "round silo", not "square tower". Add an extra column of
darker stone on the right side to suggest curvature shadow.

**Output:** 256 × 384 PNG, magenta only at the corners cut off by the
roof's cone.

# `v2_blacksmith` — forge building

A 3×3-tile (48×48 native, 384×384 output) building for the village
blacksmith. Distinct from cottages: stone walls instead of wood,
chimney with smoke, and an anvil silhouette OUT FRONT so it reads as
"forge" from one glance even at thumbnail size.

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/v2_blacksmith.png`.**

---

Generate a single PNG: a top-down pixel-art **blacksmith / forge
building** in the visual style of **Pokémon HeartGold / SoulSilver**
(Nintendo DS, 2009). Slight 3/4 perspective — you can see the roof
plus the front wall and door.

**Image size: 384 × 384 px.** 8× nominal scale. The building fills the
full image: 3 tiles wide × 3 tiles tall at 16×16 native = 384×384
output. No grid lines, no labels.

**Background where the building does not cover:** solid magenta
`#FF00FF`. (The building footprint is rectangular and fills the image,
so magenta only shows in the corners cut off by the roof's slope, if
any.)

**Style:** Crisp pixel art. No anti-aliasing. 1 px detail at native
resolution. Slight 3/4 perspective.

**Subject — a small-medieval blacksmith forge:**

- **Stone block walls** — light stone with darker mortar lines. Wall
  occupies the lower 2 tile rows.
- **Roof:** dark gray slate tiles with a slight angle (NOT thatch).
  Roof is the upper 1.5 tiles tall.
- **Chimney:** prominent brick chimney on the right side of the roof,
  with a small puff of light-gray smoke at the top.
- **Door:** dark wood, centered on the front face, large and
  rectangular. Iron hinges visible.
- **Windows:** ONE small window on each side of the door, square,
  with a warm orange/yellow glow inside (visible forge fire). The
  warm glow inside is the strongest visual signal — pick the warmest
  yellow in the palette.
- **Out front (in front of the door, lower 1/4 of the image):** a
  small ANVIL sitting on a wooden block. The anvil is iron-gray. This
  is the silhouette that makes the building read as "forge" even at
  thumbnail size.
- **Optional bonus:** a wooden barrel + a pile of horseshoes on the
  ground beside the anvil. Keep these small.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Stone light | `#c0cbdc` |
| Stone shadow | `#8b9bb4` |
| Mortar / very dark stone | `#3a4466` |
| Roof slate | `#5a6988` |
| Roof slate shadow | `#3a4466` |
| Brick (chimney) | `#a22633` |
| Brick highlight | `#d77643` |
| Wood door | `#733e39` |
| Wood door shadow | `#3e2731` |
| Iron (anvil, hinges) | `#262b44` |
| Forge glow (window) | `#feae34` |
| Forge glow bright | `#fee761` |
| Smoke puff | `#c0cbdc` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** the forge-glow windows should be the brightest spot in
the image. The anvil should be clearly silhouetted against the wall
behind it. Do NOT include any text, signs, or grid lines.

**Output:** 384 × 384 PNG. Magenta only where the building footprint
does not reach (typically the four corners of the image).

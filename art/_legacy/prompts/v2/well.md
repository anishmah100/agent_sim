# `v2_well` — village square centerpiece

A single 1×1-tile (16×16 native, 128×128 output) well sprite, designed
to sit at the center of the village square. The v1 well existed inside
the buildings tileset but was small + blocky; this round upgrades it
to a clearly-stone-rimmed well with a wooden roof and a bucket on a
rope.

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/v2_well.png`.**

---

Generate a single PNG: a top-down pixel-art **village well** in the
visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS,
2009). One sprite, centered in the image, with a magenta border for
unambiguous cropping.

**Image size: 128 × 128 px.** 8× nominal scale. The well sprite is
16×16 native (128×128 output) and fills the center of the image. A
generous magenta border surrounds it so cropping is trivial.

**Background:** solid magenta `#FF00FF`.

**Style:** Crisp pixel art. No anti-aliasing. 1 px detail at native
resolution. Slight 3/4 perspective — you see the round stone rim from
above-and-front and the wooden roof angled toward the camera.

**Subject — a medieval-fantasy stone well:**

- **Round stone rim** at the bottom 2/3 of the sprite. Light stone
  with darker mortar between blocks. The rim should be clearly
  CIRCULAR (use the slight 3/4 view to make it an ellipse, not a
  square).
- **Dark opening** in the center of the rim — a black/very-dark
  circle showing the well shaft going down.
- **Wooden roof** above the rim, peaked, dark wood with golden
  thatch or simple wooden shingles. Two small wooden support posts
  holding it up at left and right.
- **Bucket** hanging from a rope that goes from under the roof down
  toward the center of the well opening. Wooden bucket, small, with
  a dark iron band around it.
- **Optional:** a small wooden lever or crank handle on the right
  side of the rim for raising the bucket.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Stone light (rim) | `#c0cbdc` |
| Stone shadow | `#8b9bb4` |
| Mortar | `#3a4466` |
| Well shaft (interior dark) | `#181425` |
| Wood roof | `#733e39` |
| Wood roof highlight | `#b86f50` |
| Wood post | `#b86f50` |
| Rope | `#e4a672` |
| Bucket wood | `#733e39` |
| Bucket iron band | `#262b44` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** the rim must read as CIRCULAR (slight ellipse from
3/4 view), not as a square. The well opening / dark shaft is the
most recognizable feature — keep it prominent.

**Output:** 128 × 128 PNG. Sprite centered. Generous magenta border.

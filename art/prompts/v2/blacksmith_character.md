# `blacksmith_character` — burly forge-keeper NPC

Full character spritesheet for the village blacksmith. Reads as
"strong, working person" at a glance via leather apron + hammer +
muscular silhouette.

**Save the result to:**

```
~/projects/agent_sim/art/raw/blacksmith_character.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of a
**village blacksmith** in the visual style of **Pokémon HeartGold /
SoulSilver** (Nintendo DS, 2009). A burly middle-aged man with a thick
brown beard, broad shoulders, wearing a heavy leather apron over a
rolled-up off-white tunic, brown trousers, work boots, and a small
red cloth tied around his head as a sweatband. He carries a forging
hammer in his right hand at all times.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale. Each
frame cell is 128 × 96 pixels in the output; the character at native
resolution would be 16 × 24 px.

**Background:** solid magenta `#FF00FF` everywhere that isn't part of
the character. No gradients, no halos, no anti-aliased edges.

**Style rules:**
- Crisp pixel art. No anti-aliasing. No painterly shading.
- Hard color transitions only.
- 1 px dark outline around every silhouette (= 8 px at 8× scale).
- Slight 3/4 perspective.
- Each frame's character occupies the centered 128 × 96 region.

**Sheet layout: 5 rows × 4 columns = 20 cells, each 256 × 96 px.**

| Row | y range | What | Frames |
|---|---|---|---|
| 1 | 0–95 | Walk facing SOUTH | 4 walk frames |
| 2 | 96–191 | Walk facing NORTH | 4 walk frames |
| 3 | 192–287 | Walk facing WEST | 4 walk frames |
| 4 | 288–383 | Walk facing EAST | 4 walk frames |
| 5 | 384–479 | Action row (south): attack, attack, hit, interact | 4 action frames |

**Walk pose discipline:** standard 4-frame walk — idle / step-A /
passing / step-B with opposite-arm-opposite-leg swing and a 1-px
body-bob dip on frames 1–3. The hammer stays in his right hand
throughout (no swap to other hand on different directions — it
disappears behind the body on west-facing rows).

**Action row poses (facing south):**
- Frame 0 (windup): hammer pulled back over shoulder, body twisted,
  legs braced wide.
- Frame 1 (strike): hammer brought forward and down, mid-swing,
  weight shifted forward.
- Frame 2 (hit / took damage): body knocked back, head turned aside,
  free hand raised defensively.
- Frame 3 (interact): bent forward at the waist, both hands extended
  forward as if grabbing an item from the anvil.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Beard / hair (dark brown) | `#3e2731` |
| Sweatband cloth (red) | `#e43b44` |
| Tunic (off-white) | `#ead4aa` |
| Tunic shadow | `#e4a672` |
| Leather apron | `#733e39` |
| Apron shadow / strap | `#3e2731` |
| Trousers brown | `#5a6988` |
| Boots dark brown | `#262b44` |
| Hammer head iron | `#5a6988` |
| Hammer handle wood | `#b86f50` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta key preserved, no text or labels.

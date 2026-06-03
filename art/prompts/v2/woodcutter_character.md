# `woodcutter_character` — lumberjack NPC with axe

Full character spritesheet for a village woodcutter. Reads as
"outdoorsman" via plaid shirt + axe + boots + scruff.

**Save the result to:**

```
~/projects/agent_sim/art/raw/woodcutter_character.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of a
**village woodcutter / lumberjack** in the visual style of **Pokémon
HeartGold / SoulSilver** (Nintendo DS, 2009). A wiry, weather-tanned
man with short reddish-brown hair, a stubbly beard, wearing a red-and-
black checkered flannel shirt over a brown undershirt, dark green
trousers tucked into knee-high brown boots, with leather wrist guards.
He carries a felling axe over his right shoulder at all times.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale. Each
frame cell is 128 × 96 pixels in the output; the character at native
resolution would be 16 × 24 px.

**Background:** solid magenta `#FF00FF` everywhere not part of the
character.

**Style rules:** Crisp pixel art, no anti-aliasing, 1 px outline,
slight 3/4 perspective, character centered in each cell.

**Sheet layout: 5 rows × 4 columns = 20 cells, each 256 × 96 px.**

Same layout convention as the trainer_red prompt: rows 1–4 are walk
cycles facing S / N / W / E with 4 frames each (idle / step-A /
passing / step-B, body-bob on frames 1–3, opposite-arm swing). Row 5
is the action row facing south: attack windup, attack strike, hit,
interact.

**Action row poses (facing south):**
- Frame 0 (windup): axe pulled back over head, both hands on the
  haft, knees bent wide.
- Frame 1 (strike): axe brought forward and down at a target on the
  ground in front, body fully extended.
- Frame 2 (hit): body knocked sideways, axe lowered, free hand
  raised.
- Frame 3 (interact): bent forward, one hand reaching down (e.g.
  picking up a log), axe held loose at his side.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair / stubble (reddish brown) | `#a22633` |
| Hair shadow | `#3e2731` |
| Flannel red square | `#e43b44` |
| Flannel black square | `#262b44` |
| Flannel cross-thread (highlight) | `#a22633` |
| Undershirt brown | `#b86f50` |
| Trousers dark green | `#265c42` |
| Trousers shadow | `#193c3e` |
| Boots brown | `#733e39` |
| Leather wrist guards | `#3e2731` |
| Axe blade iron | `#c0cbdc` |
| Axe blade shadow | `#8b9bb4` |
| Axe handle wood | `#b86f50` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta key preserved.

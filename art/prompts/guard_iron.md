# `guard_iron` — town guard spritesheet

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/guard_iron.png`.**

---

Generate a single PNG: a top-down pixel-art character spritesheet of a town guard / soldier in the visual style of **Pokémon HeartGold / SoulSilver overworld sprites** (Nintendo DS, 2009).

Specifically draw: a tall, serious-looking guard wearing **a steel helmet** (medieval kettle-helm style), **gray steel chest plate over chain mail**, **dark navy tunic underneath visible at the arms**, **gray steel shin guards**, **dark boots**. Carrying **a steel sword in his right hand** and **a round wooden shield with iron rim in his left**. Strong upright posture.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale (each frame cell 128 × 96 px in output; native 16 × 24 px). Slight 3/4 perspective.

**Background:** solid magenta `#FF00FF`. No gradients, no halos, no AA. 1 px dark outline (= 8 px at 8× scale).

**Sheet layout — 5 rows × 4 cols:**

| Row | y range | What |
|---|---|---|
| 1 | 0–95 | Walk facing SOUTH — 4 frames (military march cadence) |
| 2 | 96–191 | Walk facing NORTH (helmet visible from above) |
| 3 | 192–287 | Walk facing WEST |
| 4 | 288–383 | Walk facing EAST |
| 5 | 384–479 | Action row (south): sword raised, sword swing, hit reaction with shield raised, interact (saluting) |

**Walk frames:** Crisp march — armored steps, slight body bob. Sword stays in right hand throughout. Shield stays in left. Cape (if you draw one — optional, dark navy) flutters back. Frame 0 = idle stance with feet slightly apart. Frames 1–3 = walk cycle with sword arm slightly swinging.

**Action row F0-F3 (facing south):**
- F0 sword raised: sword arm pulled back over shoulder, ready to swing, shield up at chest level
- F1 sword swing: sword fully extended forward at chest height, body twisted into the strike
- F2 hit/blocking: shield raised in front of face, body half-turned, sword lowered defensively
- F3 salute / interact: standing rigid, sword arm raised across chest in formal salute, shield at side

**Color palette — use ONLY these 9 colors:**
| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Helmet + chest plate (light steel) | `#c0cbdc` |
| Steel shadow | `#8b9bb4` |
| Tunic / undergarments (dark navy) | `#262b44` |
| Sword blade (bright steel) | `#ffffff` |
| Shield wood / sword grip (brown) | `#733e39` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta preserved, no labels.

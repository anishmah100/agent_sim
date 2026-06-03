# `lyra_blue` — female adventurer character spritesheet

**Paste the block below into ChatGPT chat.** Save the result to:
```
~/projects/agent_sim/art/raw/lyra_blue.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of a young woman adventurer in the visual style of **Pokémon HeartGold / SoulSilver overworld sprites** (Nintendo DS, 2009). Think of trainer Lyra/Kris — small chibi proportions (large head ~1/3 of body, short stylized body).

Specifically draw: a young woman with a **white-and-blue cap with a half-circle brim**, **brown twin-tail pigtails coming out from under the cap**, **light blue short-sleeved jacket over a white t-shirt**, **red shorts**, **white-and-blue sneakers**. Pale skin. No weapons or accessories visible.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale (each frame cell is 128 × 96 px in the output; the character at native resolution would be 16 × 24 px).

**Background:** solid magenta `#FF00FF` everywhere that isn't the character. No gradients, no halos, no anti-aliasing.

**Style rules:**
- Crisp pixel art, no anti-aliasing, hard color transitions, 1 px dark outline around the silhouette (8 px at 8× scale).
- Slight 3/4 top-down perspective — top of cap visible when facing south.
- Each frame's character centered in its 256 × 96 cell with 64 px magenta padding each side.

**Sheet layout — 5 rows × 4 columns:**

| Row | y range | What |
|---|---|---|
| 1 | 0–95 | Walk facing SOUTH (face visible) |
| 2 | 96–191 | Walk facing NORTH (back, cap brim from above) |
| 3 | 192–287 | Walk facing WEST (left) |
| 4 | 288–383 | Walk facing EAST (right) |
| 5 | 384–479 | Action row (all facing south): attack windup, attack release, hit reaction, interact (reaching down) |

**Walk frames (rows 1–4):** Frame 0 = idle/contact, Frame 1 = one leg forward + opposite arm swung forward + body bobbing slightly lower, Frame 2 = passing pose (legs near center, body at lowest), Frame 3 = mirror of frame 1 with the OTHER leg lead. Arms must swing opposite to lead leg. Body height must dip on frames 1–3 vs frame 0 (walk bounce).

**Action row poses (row 5, all facing south):**
- F0 attack windup: right arm pulled back, body twisted
- F1 attack release: right arm fully extended forward, mid-strike
- F2 hit/take damage: body knocked back, head turned aside, defensive arm raised
- F3 interact: bent slightly forward, both hands extended forward (like picking up an item)

**Color palette — use ONLY these 9 colors:**
| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair (brown pigtails) | `#733e39` |
| Cap white / shirt / sneaker accents | `#ffffff` |
| Cap blue brim / jacket | `#0099db` |
| Cap blue shadow / jacket shadow | `#124e89` |
| Red shorts | `#e43b44` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 px PNG, magenta background preserved (don't convert to alpha). No watermarks, no labels, no grid lines.

# `child_kid` — village child spritesheet

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/child_kid.png`.**

---

Generate a single PNG: a top-down pixel-art character spritesheet of a small child villager in the visual style of **Pokémon HeartGold / SoulSilver overworld sprites** (Nintendo DS, 2009).

Specifically draw: a small child (~7-9 years old in-fiction). **Shorter and smaller than the adult characters by ~25%** — the silhouette should occupy maybe 12 × 18 of the 16 × 24 native cell, with the smaller proportions clearly visible. **Spiky bright blond hair**, **a green short-sleeved t-shirt**, **brown shorts**, **simple brown sandals**. No accessories. Wide eyes, energetic posture.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale (each frame cell 128 × 96 px in output; native 16 × 24 px). Slight 3/4 perspective.

**Background:** solid magenta `#FF00FF`. No gradients, no halos, no AA. 1 px dark outline (= 8 px at 8× scale).

**Sheet layout — 5 rows × 4 cols:**

| Row | y range | What |
|---|---|---|
| 1 | 0–95 | Walk facing SOUTH — 4 frames (bouncy energetic walk) |
| 2 | 96–191 | Walk facing NORTH |
| 3 | 192–287 | Walk facing WEST |
| 4 | 288–383 | Walk facing EAST |
| 5 | 384–479 | Action row (south): throw stone (windup), throw stone (release), hit + crying, interact (pick up shiny object) |

**Walk frames:** Bouncier than adults — Frame 1 and 3 should show clear vertical bounce, arms swinging energetically. Hair is springy and may shift slightly between frames. Frame 0 = idle standing with hands at sides.

**Action row F0-F3 (facing south):**
- F0 throw windup: small arm pulled back over head, hand clutching an imaginary stone, body twisted
- F1 throw release: arm extended forward in throwing motion, body un-twisted
- F2 hit + crying: knocked back onto bottom, one arm raised, mouth open in cry (small visible "O" or downturned mouth)
- F3 pick-up: bent forward, both hands reaching down to pick up something shiny

**Color palette — use ONLY these 9 colors:**
| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair (bright blond) | `#feae34` |
| Hair shadow | `#d77643` |
| Shirt (green) | `#63c74d` |
| Shirt shadow | `#3e8948` |
| Shorts + sandals (brown) | `#733e39` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta preserved, no labels.

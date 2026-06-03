# `cloaked_wanderer` — mysterious hooded figure spritesheet

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/cloaked_wanderer.png`.**

---

Generate a single PNG: a top-down pixel-art character spritesheet of a mysterious cloaked wanderer in the visual style of **Pokémon HeartGold / SoulSilver overworld sprites** (Nintendo DS, 2009).

Specifically draw: a slim figure entirely wrapped in **a dark forest-green hooded cloak** that covers the head and falls past the knees. Hood is up — **face is mostly hidden in shadow**, only the lower half (chin and mouth) visible with **pale skin**. Underneath the cloak: **dark leather armor** glimpsed on arms and legs, **brown leather boots**. A **silver dagger** is sheathed at the waist (visible on the side views). Posture: lean, alert, cautious.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale (each frame cell 128 × 96 px in output; native 16 × 24 px). Slight 3/4 perspective.

**Background:** solid magenta `#FF00FF`. No gradients, no halos, no AA. 1 px dark outline (= 8 px at 8× scale).

**Sheet layout — 5 rows × 4 cols:**

| Row | y range | What |
|---|---|---|
| 1 | 0–95 | Walk facing SOUTH (hood + lower face visible) — 4 frames |
| 2 | 96–191 | Walk facing NORTH (only the back of the hood + cloak visible) |
| 3 | 192–287 | Walk facing WEST (profile of hood, dagger on far side) |
| 4 | 288–383 | Walk facing EAST (profile of hood, dagger visible at side) |
| 5 | 384–479 | Action row (south): dagger draw, dagger strike, hit + cloak flare, interact (lifting an object — gloved hand visible) |

**Walk frames:** Stealthy gait — quieter than the other characters. Frame 0 = idle with slightly tilted head. Frames 1-3 show a soft walk cycle. The cloak's hem ripples slightly between frames (show 2-3 different bottom positions). When facing east/west, the cloak slightly trails behind.

**Action row F0-F3 (facing south):**
- F0 dagger draw: cloak open at the side, hand reaching for the dagger hilt
- F1 dagger strike: dagger thrust forward, blade extended, body twisted forward
- F2 hit + cloak flare: stumbled back, cloak flaring dramatically, hood half-knocked back showing more face
- F3 interact: bent forward, both gloved hands gently lifting an unseen item

**Color palette — use ONLY these 9 colors:**
| Use | Hex |
|---|---|
| Skin (pale, partial) | `#e8b796` |
| Skin shadow (the hood-shadow on upper face) | `#3e2731` |
| Cloak (dark forest green) | `#265c42` |
| Cloak shadow / fold | `#193c3e` |
| Leather armor (dark brown) | `#3e2731` |
| Boots (brown) | `#733e39` |
| Dagger blade (steel) | `#c0cbdc` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta preserved, no labels.

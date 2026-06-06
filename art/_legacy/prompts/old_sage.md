# `old_sage` — robed elder mentor spritesheet

**Paste below into ChatGPT chat. Save result to `~/projects/agent_sim/art/raw/old_sage.png`.**

---

Generate a single PNG: a top-down pixel-art character spritesheet of an old wizard/sage character in the visual style of **Pokémon HeartGold / SoulSilver overworld sprites** (Nintendo DS, 2009).

Specifically draw: an elderly man with **long gray hair and a flowing white beard**, wearing **long flowing dark blue robes** that go down to his feet, **a brown leather sash across the chest**, carrying **a wooden staff with a small glowing yellow gem at the top** in his right hand. No hat. Slightly hunched silhouette.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale (each frame cell 128 × 96 px in output; character native is 16 × 24 px). Slight 3/4 top-down perspective.

**Background:** solid magenta `#FF00FF`. No gradients, no halos, no AA. 1 px dark outline around silhouette (= 8 px at 8× scale).

**Sheet layout — 5 rows × 4 columns:**

| Row | y range | What |
|---|---|---|
| 1 | 0–95 | Walk facing SOUTH (face + beard visible) — 4 frames |
| 2 | 96–191 | Walk facing NORTH (back of robes + gray hair from above) |
| 3 | 192–287 | Walk facing WEST |
| 4 | 288–383 | Walk facing EAST |
| 5 | 384–479 | Action row south: cast windup (staff raised), cast release (staff thrust forward with yellow glow), hit reaction, interact (slight bow) |

**Walk frames:** Frame 0 = idle, Frame 1 = step forward with one leg + staff planted on opposite side, Frame 2 = passing pose with body bobbing lower (his shoulders dip more than youth — he's old), Frame 3 = mirror of frame 1. The staff swings opposite to the leading leg. Robes have a slight visible flutter — show 2-3 different bottom-hem positions across the frames.

**Action row F0-F3 (facing south):**
- F0 cast windup: staff held vertical above head, both hands grip staff
- F1 cast release: staff thrust forward at 45° angle, yellow gem glowing brighter (use lighter yellow color from palette)
- F2 hit: stumbled back, one arm raised, staff lowered defensively
- F3 interact: slight bow, beard tilted forward, one hand extended in a gesturing motion

**Color palette — use ONLY these 9 colors:**
| Use | Hex |
|---|---|
| Skin (older, ruddy) | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair + beard (gray-white) | `#c0cbdc` |
| Robe body (dark blue) | `#3a4466` |
| Robe shadow / fold | `#262b44` |
| Leather sash (brown) | `#733e39` |
| Staff wood | `#b86f50` |
| Gem/glow (yellow) | `#fee761` |
| Outline | `#181425` |
| Background | `#FF00FF` |

That's 9 character colors + magenta. No other colors allowed.

**Output:** 1024 × 480 px PNG, magenta preserved, no labels.

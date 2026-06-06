# `merchant_baker` — friendly shopkeeper spritesheet

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/merchant_baker.png`.**

---

Generate a single PNG: a top-down pixel-art character spritesheet of a friendly middle-aged shopkeeper / baker in the visual style of **Pokémon HeartGold / SoulSilver overworld sprites** (Nintendo DS, 2009).

Specifically draw: a stocky middle-aged man with **a round friendly face**, **a bushy brown mustache**, **short brown hair**, **a white baker's apron over a brown shirt**, **dark trousers**, **brown work boots**. **Slightly chubby silhouette** — wider than the trainer/adventurer characters. Holding **a single loaf of bread** in one hand on idle frames.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale (each frame cell 128 × 96 px in output; native 16 × 24 px). Slight 3/4 perspective.

**Background:** solid magenta `#FF00FF`. No gradients, no halos, no AA. 1 px dark outline (= 8 px at 8× scale).

**Sheet layout — 5 rows × 4 cols:**

| Row | y range | What |
|---|---|---|
| 1 | 0–95 | Walk facing SOUTH — 4 frames (waddle gait — wider stance) |
| 2 | 96–191 | Walk facing NORTH |
| 3 | 192–287 | Walk facing WEST |
| 4 | 288–383 | Walk facing EAST |
| 5 | 384–479 | Action row (south): hand-out-with-bread (selling), hand-receiving-coin, hit reaction, interact (placing bread on counter) |

**Walk frames:** Same 4-frame pattern as other characters (idle / step / passing / step). His walk is slightly waddling — wider stance, lower bounce than the slim characters. Belly visible. Apron sways slightly.

**Action row poses (row 5, all south):**
- F0 sell pose: arm extended forward, palm up, bread balanced on hand offered to viewer
- F1 receive payment: arm extended forward, palm cupped to receive a coin (no bread, switched hands)
- F2 hit: stumbled back, both arms raised defensively, look of surprise
- F3 place item: bent slightly forward, both hands placing bread down on an unseen counter in front of him

**Color palette — use ONLY these 10 colors:**
| Use | Hex |
|---|---|
| Skin (ruddy) | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair + mustache (brown) | `#733e39` |
| Apron white | `#ffffff` |
| Apron tie (dark brown) | `#3e2731` |
| Shirt (brown) | `#b86f50` |
| Trousers (dark gray-blue) | `#3a4466` |
| Boots (dark brown) | `#3e2731` |
| Bread (golden tan) | `#feae34` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta preserved, no labels/grid.

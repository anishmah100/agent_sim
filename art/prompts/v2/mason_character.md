# `mason_character` — stonemason / builder NPC

Full character spritesheet for a village mason. Reads as "builder"
via leather apron over rougher clothes + trowel/hammer in hand.

**Save the result to:**

```
~/projects/agent_sim/art/raw/mason_character.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of a
**stonemason / builder** in the visual style of **Pokémon HeartGold /
SoulSilver** (Nintendo DS, 2009). A middle-aged woman with short
practical dark hair tied back, wearing a dust-gray short-sleeved tunic
under a stained light-brown leather apron, slate-blue trousers, sturdy
boots, with a cloth wrap around her left forearm. She carries a small
mason's trowel in her right hand and has a chisel tucked into her
apron front pocket.

**Image size: 1024 × 480 px.** 8× nominal scale, character 16×24
native per cell.

**Background:** solid magenta `#FF00FF` everywhere not part of the
character.

**Style rules:** Crisp pixel art, no anti-aliasing, 1 px outline,
slight 3/4 perspective, character centered in each cell.

**Sheet layout: 5 rows × 4 columns = 20 cells, each 256 × 96 px.**

Same layout convention — rows 1–4 walk S/N/W/E (4 frames each, idle /
step-A / passing / step-B, body-bob on frames 1–3, opposite-arm swing).
Row 5 action row facing south: attack windup, attack strike, hit,
interact.

**Action row poses (facing south):**
- Frame 0 (windup): trowel raised, body crouched as if about to lay
  a stone.
- Frame 1 (strike): trowel scraping down across an unseen surface in
  front, arm fully extended.
- Frame 2 (hit): body recoiled, free hand raised defensively.
- Frame 3 (interact): squatting low, both hands placing a stone
  block on the ground in front.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Skin | `#e4a672` |
| Skin shadow | `#c28569` |
| Hair dark brown | `#3e2731` |
| Tunic dust gray | `#c0cbdc` |
| Tunic shadow | `#8b9bb4` |
| Leather apron | `#b86f50` |
| Apron shadow / strap | `#733e39` |
| Trousers slate blue | `#3a4466` |
| Boots dark brown | `#262b44` |
| Wrist wrap cloth | `#ead4aa` |
| Trowel iron | `#8b9bb4` |
| Trowel handle wood | `#733e39` |
| Chisel iron | `#c0cbdc` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta key preserved.

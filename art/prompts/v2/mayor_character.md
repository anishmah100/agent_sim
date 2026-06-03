# `mayor_character` — village mayor / civic official NPC

Full character spritesheet for the mayor. Reads as "important / civic"
via long blue robe with gold trim + small chain of office + dignified
posture.

**Save the result to:**

```
~/projects/agent_sim/art/raw/mayor_character.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of a
**village mayor / civic official** in the visual style of **Pokémon
HeartGold / SoulSilver** (Nintendo DS, 2009). An older man (50s) with
neatly combed gray hair and a closely-trimmed beard, wearing a long
deep-blue robe with gold trim along the hem and cuffs, a small white
collar visible at the neck, a thin gold chain of office across the
chest with a small pendant, and dark leather shoes. He carries a thin
wooden walking cane in his right hand. Dignified upright posture.

**Image size: 1024 × 480 px.** 8× nominal scale, character 16×24
native per cell.

**Background:** solid magenta `#FF00FF` everywhere not part of the
character.

**Style rules:** Crisp pixel art, no anti-aliasing, 1 px outline,
slight 3/4 perspective, character centered in each cell.

**Sheet layout: 5 rows × 4 columns = 20 cells, each 256 × 96 px.**

Same layout convention — rows 1–4 walk S/N/W/E (4 frames each, idle /
step-A / passing / step-B). His walk is SLOWER and more deliberate
than other characters — minimal body-bob (~half-amplitude on frames
1–3), short steps. The robe hem swings gently. Row 5 action row
facing south: attack windup, attack strike, hit, interact.

**Action row poses (facing south):**
- Frame 0 (windup / address): cane raised to point forward, free hand
  on his chest, head slightly tilted up.
- Frame 1 (strike / declaration): cane fully extended forward,
  pointing at something, the gesture of a speech-giver.
- Frame 2 (hit): body bent back, free hand raised to his head,
  startled-old-man pose.
- Frame 3 (interact): leaning forward slightly with both hands
  resting on the cane, like he's examining something on the ground.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair gray | `#c0cbdc` |
| Hair shadow | `#8b9bb4` |
| Robe deep blue | `#124e89` |
| Robe shadow | `#262b44` |
| Robe gold trim | `#feae34` |
| Gold trim highlight | `#fee761` |
| Collar white | `#ffffff` |
| Chain of office gold | `#feae34` |
| Pendant | `#a22633` |
| Shoes dark leather | `#3e2731` |
| Cane wood | `#733e39` |
| Cane tip | `#c0cbdc` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta key preserved.

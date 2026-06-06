# `goblin_character` — hostile creature NPC

Full character spritesheet for a goblin (hostile monster used in
combat scenarios). Smaller and uglier than humanoid NPCs; reads as
"enemy" immediately.

**Save the result to:**

```
~/projects/agent_sim/art/raw/goblin_character.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of a
**goblin** in the visual style of **Pokémon HeartGold / SoulSilver**
(Nintendo DS, 2009). A small (slightly shorter than the human NPCs)
hunched humanoid with mottled muddy-green skin, pointed ears, a wide
fanged mouth, beady yellow eyes, sparse dark hair, wearing scraps of
brown leather armor over a ragged dark-red loincloth, with crude
bandages around its hands and feet. It carries a jagged rusty short
sword in its right hand at all times.

The goblin's posture is HUNCHED — its head juts forward, knees
slightly bent at rest. Shorter than the human NPCs (occupies ~20 px
of the 24-px-tall character region instead of the full 24).

**Image size: 1024 × 480 px.** 8× nominal scale, character ~16×20
native per cell (shorter than humans).

**Background:** solid magenta `#FF00FF` everywhere not part of the
character.

**Style rules:** Crisp pixel art, no anti-aliasing, 1 px outline,
slight 3/4 perspective, character centered in each cell.

**Sheet layout: 5 rows × 4 columns = 20 cells, each 256 × 96 px.**

Same layout convention — rows 1–4 walk S/N/W/E. The goblin's walk is
SCURRYING — quick short steps with a pronounced body-bob, head
bobbing up-down with the steps. Arms swing wide. Row 5 action row
facing south.

**Action row poses (facing south):**
- Frame 0 (windup): sword raised sideways at shoulder height, body
  twisted, lips drawn back snarling.
- Frame 1 (strike): sword slashed across the body in a wide arc,
  weight lunged forward.
- Frame 2 (hit): body knocked backward, head thrown back, sword arm
  flailing.
- Frame 3 (death / pickup): hunched far over, head almost touching
  the ground — readable as either a low scavenge pose OR a "dying"
  pose (the engine will pick which by context).

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Skin muddy green | `#3e8948` |
| Skin shadow | `#265c42` |
| Skin highlight | `#63c74d` |
| Hair dark | `#3e2731` |
| Eyes yellow | `#fee761` |
| Teeth white | `#ead4aa` |
| Loincloth dark red | `#a22633` |
| Leather armor brown | `#733e39` |
| Leather shadow | `#3e2731` |
| Hand/foot bandages cream | `#ead4aa` |
| Bandages dirty shadow | `#c28569` |
| Sword blade rusty iron | `#a22633` (rusted spots) + `#8b9bb4` (uncorroded edge) |
| Sword handle | `#3e2731` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta key preserved.

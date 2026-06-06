# `drifter_character` — rogue / drifter NPC

Full character spritesheet for a drifter — distinct from the existing
cloaked_wanderer. Reads as "rogue / outsider" via darker colors +
hood thrown back + dagger at hip + watchful expression.

**Save the result to:**

```
~/projects/agent_sim/art/raw/drifter_character.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of a
**drifter / rogue** in the visual style of **Pokémon HeartGold /
SoulSilver** (Nintendo DS, 2009). A wiry young woman with sharp eyes
and a long dark braid trailing over one shoulder, wearing a dark
forest-green hooded jerkin (hood thrown back, not up), a black leather
belt across the chest holding a small pouch and a sheathed dagger, a
charcoal-gray under-tunic, dark gray trousers, and well-worn brown
boots. Watchful, ready posture. She carries no weapon in hand — the
dagger stays sheathed at her hip.

**Image size: 1024 × 480 px.** 8× nominal scale, character 16×24
native per cell.

**Background:** solid magenta `#FF00FF` everywhere not part of the
character.

**Style rules:** Crisp pixel art, no anti-aliasing, 1 px outline,
slight 3/4 perspective.

**Sheet layout: 5 rows × 4 columns = 20 cells, each 256 × 96 px.**

Same layout convention — rows 1–4 walk S/N/W/E. Her walk is QUIETER —
short, careful steps, body-bob slightly muted, weight stays low. The
braid swings opposite the body. Row 5 action row facing south.

**Action row poses (facing south):**
- Frame 0 (windup): right hand pulling dagger half-drawn from its
  sheath, body crouched low, left hand extended for balance.
- Frame 1 (strike): dagger fully drawn and slashing forward across
  the body, weight on the front foot.
- Frame 2 (hit): body twisted aside, dagger lowered, one arm raised.
- Frame 3 (interact): crouched low, both hands extended forward to
  examine or pick something up (lockpicking pose).

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair black-brown | `#181425` |
| Hair highlight | `#3e2731` |
| Jerkin forest green | `#265c42` |
| Jerkin shadow | `#193c3e` |
| Hood lining | `#3e2731` |
| Under-tunic charcoal | `#3a4466` |
| Trousers dark gray | `#262b44` |
| Belt leather black | `#181425` |
| Belt buckle gold | `#feae34` |
| Pouch brown | `#733e39` |
| Boots brown | `#5a6988` |
| Dagger sheath dark | `#3e2731` |
| Dagger blade silver | `#c0cbdc` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 480 PNG, magenta key preserved.

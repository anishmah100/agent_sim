# Character base spritesheet v2 — `character_base_v2`

**Why v2 exists:** v1 produced a mushy result. Two changes in v2:

1. **Generate at 8× scale** (1024 × 384 output, character cells of 128 × 192). The AI has more pixels to work with for detail; we'll downsample with proper palette quantization on intake.
2. **Walk only.** Combat, hit, and death animations will be a SEPARATE prompt (`character_actions_v1.md`) once the walk is locked. Don't let scope kill quality.

**Paste everything between the `---` markers into ChatGPT (image gen enabled).**

**Save the result to:**

```
~/projects/agent_sim/art/raw/character_base_v2.png
```

---

Generate a single PNG image that is a top-down pixel-art character spritesheet for an RPG, in the EXACT visual style of **Pokémon HeartGold / SoulSilver (Nintendo DS, 2009)** — the overworld walking sprites in Lyra/Ethan's villages.

# Image specifications

- **Output dimensions: 1024 × 384 pixels.**
  Each character frame is **128 × 96 pixels** in the output. The character is drawn at **8× nominal scale** so you have enough pixels for crisp single-pixel detail at the target native resolution (16 × 24 nominal). Treat it like an Aseprite source file at 8× preview.
- **Background: solid magenta `#FF00FF`** filling all non-character pixels exactly. The magenta will be keyed to transparency later — every transparent area must be exactly this RGB, no halos, no gradients.
- **Pixel art rules:**
  - Crisp single-pixel detail at nominal resolution. When the 1024×384 output is downscaled 8× to its target 128×48 nominal, every pixel must be intentional and clean.
  - **No anti-aliasing.** No semi-transparent edge pixels.
  - **No blur, no gradients, no painterly shading.** Hard color transitions only.
  - **1-pixel dark outline** around each character silhouette (at nominal resolution = 8 px wide at 8× scale).

# Character to draw

A generic young human adult in plain neutral clothing. This is the FOUNDATION character — variations (hair, armor) will be layered in separate sheets.

- Build: average human, slightly stylized "chibi" proportions like HeartGold (large head ≈ 1/3 of total height, simplified body).
- Outfit:
  - Short dark brown hair, cropped to head
  - Pale skin
  - Plain warm-brown short tunic
  - Dark brown pants
  - Black/dark boots
- No hat, no weapon, no shield, no cape, no visible inventory.

# Sheet layout — 4 rows × 4 columns

The sheet is divided into a 4-row × 4-column grid. Each cell is 256 × 96 px in the 1024×384 output. The character occupies the central 128 × 96 of each cell, surrounded by 64 px of magenta padding on each side. (This per-cell padding gives the character "headroom" in the cell without overlap.)

**Wait — re-read the layout:**

Actually, draw each cell as **256 wide × 96 tall** in the output. Center the 128 × 96 character within the cell with 64 px of magenta on left and right. This gives a 4-column × 4-row = 16 cell grid filling 1024 × 384.

**Rows (top to bottom):**

| Row | y range | Direction | Frames |
|-----|---------|-----------|--------|
| 1 | 0–95 | **Facing SOUTH** (viewer sees face) | Walk frames 0–3 |
| 2 | 96–191 | **Facing NORTH** (viewer sees back of head) | Walk frames 0–3 |
| 3 | 192–287 | **Facing WEST** (character faces left) | Walk frames 0–3 |
| 4 | 288–383 | **Facing EAST** (character faces right) | Walk frames 0–3 |

# Per-frame poses (CRITICAL)

For each direction, the 4 frames must be **visibly distinct** so animation reads as walking, not vibrating in place. Specifically:

- **Frame 0 (Idle / contact)**: Both feet planted flat on the ground, slight rest pose. This is the "standing still" frame.
- **Frame 1 (Step forward)**: One leg lifted forward (toe of trailing foot just leaving ground, leading foot fully planted with knee slightly bent). For SOUTH/NORTH facing, the LEFT leg leads forward. For WEST/EAST, the leg closer to the viewer leads.
- **Frame 2 (Passing / mid-step)**: Both legs near each other, weight transferring. This is the moment between left-foot-forward and right-foot-forward. Body should bob slightly LOWER than frames 0 and 3 (this gives the walk a believable bounce).
- **Frame 3 (Step forward, opposite)**: Mirror of Frame 1 — the OTHER leg now lifted forward.

The arms must also swing: when the left leg is forward (frame 1), the RIGHT arm swings forward. When the right leg is forward (frame 3), the LEFT arm swings forward. This is how real walking looks.

**Anti-checklist** (do NOT do these):
- ❌ All 4 frames the same pose (vibration, not walking)
- ❌ Arms not swinging
- ❌ Body height identical in all frames (no walk bounce)
- ❌ Mushy silhouette / blurred outlines
- ❌ Gradient shading
- ❌ Half-transparent halo pixels around the character

# Strict color palette — use ONLY these 8 colors

Every non-magenta pixel MUST be exactly one of these hex values (no other colors):

| Use | Hex |
|---|---|
| Skin highlight | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair (dark brown) | `#3e2731` |
| Tunic base | `#733e39` |
| Tunic highlight (small accents) | `#b86f50` |
| Pants | `#3e2731` |
| Boots / outline | `#181425` |
| Background | `#FF00FF` |

That's 7 character colors + 1 background. **Any other color is a failure.**

# Style references

- Pokémon HeartGold / SoulSilver overworld walking sprites (Lyra, Ethan, NPCs in New Bark Town, Cherrygrove)
- The work of pixel artist Cup Nooble ("Sprout Lands") for character clarity at small scale
- Aseprite-source-level cleanness (this is what a professional pixel artist would output as their working .ase file at 8× preview zoom)

# Output

- One PNG, exactly **1024 × 384** pixels.
- Magenta background `#FF00FF` preserved (do NOT convert to alpha transparency yourself; we handle that downstream).
- No watermark, signature, or text anywhere in the image.

# Character base spritesheet — `character_base_v1`

**Paste everything inside the `---` markers into ChatGPT (with image generation enabled).** Use GPT-4o or DALL-E 3.

**When ChatGPT returns the image, save it to:**

```
~/projects/agent_sim/art/raw/character_base_v1.png
```

If the directory doesn't exist:

```bash
mkdir -p ~/projects/agent_sim/art/raw
```

We'll process it through the intake pipeline (palette quantize, magenta→alpha, frame slicing) once you have it.

---

Generate a single 128×144 pixel PNG image: a top-down pixel-art character spritesheet in the visual style of **Pokémon HeartGold / SoulSilver (Nintendo DS, 2009)**.

**Image specs**
- Exact dimensions: 128 px wide × 144 px tall, 1× pixel scale (do not upscale).
- Background: solid magenta `#FF00FF` (will be keyed to transparency later).
- Pixel art only — crisp single-pixel edges, no anti-aliasing, no blur, no gradients.
- Outline pixels around the character are 1 px wide, in very dark brown/black.

**Character to draw**
A generic young human adult in plain neutral clothing. This is the FOUNDATION character — variations (hair color, armor, weapons) will be layered on later in separate sheets.
- Short dark hair
- Pale skin
- Plain gray-brown tunic
- Dark brown pants
- Black boots
- No weapons or accessories visible
- Sprite size: 16 px wide × 24 px tall per frame, bottom-center anchored (feet pixel at y=23 within each frame)
- Head extends into the top ~8 px above the 16×16 ground footprint

**Sheet layout — 6 rows × variable columns, each row 24 px tall**

```
Row 1 (y=0–23)    WALK DOWN  (facing south, viewer sees face)        4 frames at x=0,16,32,48
Row 2 (y=24–47)   WALK UP    (facing north, viewer sees back)         4 frames at x=0,16,32,48
Row 3 (y=48–71)   WALK LEFT  (facing west)                            4 frames at x=0,16,32,48
Row 4 (y=72–95)   WALK RIGHT (facing east)                            4 frames at x=0,16,32,48
Row 5 (y=96–119)  ACTION ROW (all facing south)
                    ATTACK frames at x=0,16,32,48
                    HIT REACTION frames at x=64,80
                    INTERACT frames at x=96,112
Row 6 (y=120–143) DEATH (facing south)                                4 frames at x=0,16,32,48
```

Pixels outside the listed frame cells are magenta `#FF00FF`.

**Per-frame pose specs**

WALK (each direction, 4 frames):
- Frame 0: standing idle (both feet planted, slight rest pose)
- Frame 1: left foot stepping forward
- Frame 2: passing pose (both feet roughly together, weight transferring)
- Frame 3: right foot stepping forward

ATTACK (4 frames, facing south):
- Frame 0: wind-up (arm pulled back)
- Frame 1: mid-swing
- Frame 2: full extension (arm forward)
- Frame 3: recoil/return

HIT REACTION (2 frames, facing south):
- Frame 0: stumbled back, head turned aside
- Frame 1: recovering to upright

INTERACT (2 frames, facing south):
- Frame 0: arm reaching forward
- Frame 1: hand fully extended

DEATH (4 frames, facing south):
- Frame 0: knees buckling
- Frame 1: falling forward
- Frame 2: collapsed on ground
- Frame 3: collapsed on ground, slightly faded/dimmed

**Strict color palette — use ONLY these colors**

Every non-magenta pixel must be one of:

| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair (dark brown) | `#3e2731` |
| Tunic base | `#733e39` |
| Tunic highlight | `#b86f50` |
| Pants | `#3e2731` |
| Boots | `#181425` |
| Outline | `#181425` |
| Background | `#FF00FF` (transparency key) |

No additional colors. No anti-aliased halos. No gradients.

**Style references**
- Pokémon HeartGold / SoulSilver overworld character sprites (2009 DS).
- Crisp single-pixel silhouettes.
- Slight 3/4 perspective — slight top of head visible.
- Readable silhouette at thumbnail size.
- Step animations show clear weight shift, not just leg position swap.

**Output**
Return the image as a single 128×144 PNG with the magenta background preserved (do not pre-convert to alpha transparency — we handle that downstream).

# `trainer_red` — full character spritesheet prompt for ChatGPT

**Copy-paste the block below into ChatGPT chat. ChatGPT image gen will produce a single PNG.**

**Save the result to:**

```
~/projects/agent_sim/art/raw/trainer_red.png
```

---

Generate a single PNG: a top-down pixel-art character spritesheet of **Trainer Red from Pokémon HeartGold / SoulSilver** (Nintendo DS, 2009). Exact look — the red baseball cap, red short-sleeved jacket, white t-shirt underneath, dark blue denim shorts, red and white sneakers, dark short hair under the cap. Aim for the actual HeartGold overworld sprite style of Red as he appears at Mt. Silver.

**Image size: 1024 × 480 px.** Pixel art at 8× nominal scale (each frame cell is 128 × 96 pixels in the output; the character at native resolution would be 16 × 24 px). Generate at 8× so detail survives.

**Background:** solid magenta `#FF00FF` everywhere that isn't part of the character. No gradients, no halos, no anti-aliased edges. This will be keyed to transparency later.

**Style rules:**
- Crisp pixel art. No anti-aliasing. No painterly shading. No blur.
- Hard color transitions only.
- 1 px dark outline around every silhouette (= 8 px at 8× scale).
- Slight 3/4 perspective — viewer is looking down and forward at the character, so the top of the cap is visible when facing south, etc.
- Each frame's character occupies the centered 128 × 96 region of its cell.

**Sheet layout: 5 rows × 4 columns = 20 cells, each 256 × 96 px.**

Each cell is 256 px wide (column) × 96 px tall (row). The character is centered inside the cell with 64 px of magenta padding on each side. Rows top to bottom:

| Row | y range | What | Frames |
|---|---|---|---|
| 1 | 0–95 | **Walk facing SOUTH** (viewer sees Red's face) | 4 walk frames |
| 2 | 96–191 | **Walk facing NORTH** (viewer sees Red's back, cap brim visible from above) | 4 walk frames |
| 3 | 192–287 | **Walk facing WEST** (Red facing left) | 4 walk frames |
| 4 | 288–383 | **Walk facing EAST** (Red facing right) | 4 walk frames |
| 5 | 384–479 | **Action row**, all facing south: attack, attack, hit, interact | 4 action frames |

**Walk frame poses (every row 1–4):**
- Frame 0 (idle / contact): both feet planted, arms at sides, slight resting pose. This is the "standing still" frame.
- Frame 1 (step): one leg lifted forward, opposite arm swung forward. Body bobbing slightly lower than frame 0.
- Frame 2 (passing): legs near center, weight transferring. Body at lowest point of the bob.
- Frame 3 (step, opposite): the OTHER leg now lifted forward, the OTHER arm swung forward.

Critical: arms must swing in opposite-leg pattern (left leg forward = right arm forward). Body height must dip on frames 1–3 vs frame 0 (this is the walk bounce).

**Action row 5 frame poses (all facing south):**
- Frame 0 (attack windup): right arm pulled back, body slightly twisted, weight shifted onto rear foot. Looks like he's about to throw a punch or swing.
- Frame 1 (attack release): right arm fully extended forward, body twisted toward the target. Mid-strike.
- Frame 2 (hit / take damage): body knocked backward, head turned aside, one arm raised defensively. Reads as "I just got hit."
- Frame 3 (interact / pick up): bent slightly forward, both hands extended forward, like reaching down to grab or examine an item on the ground.

**Color palette — use ONLY these 9 colors:**

| Use | Hex |
|---|---|
| Skin | `#e8b796` |
| Skin shadow | `#c28569` |
| Hair (very dark brown, visible under cap) | `#3e2731` |
| Cap and jacket (Red's signature red) | `#e43b44` |
| Cap brim and jacket trim (dark red shadow) | `#a22633` |
| White t-shirt / cap front patch | `#ffffff` |
| Denim shorts | `#3a4466` |
| Sneakers (white with red accent — use white + the red above) | `#ffffff` and `#e43b44` |
| Outline | `#181425` |
| Background (magenta key) | `#FF00FF` |

No other colors allowed.

**Output:**
- One PNG, exactly **1024 × 480** pixels.
- Magenta `#FF00FF` background preserved as-is — do not convert to alpha transparency.
- No watermark, signature, text, frame labels, grid lines, or annotations anywhere in the image.

Generate the image now.

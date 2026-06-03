# `v2_window_glow` — nighttime emissive overlay

A single 1×1-tile (16×16 native, 128×128 output) radial-glow sprite
used as an emissive overlay for lit windows at night. Composited
ADDITIVELY over building windows when day phase = night/twilight, so
windows appear to glow warm yellow when the sun's down.

Distinct from the Particles layer — this is a STATIC sprite that
sits on the building's render container and only fades in/out with
day phase. No animation, no flicker.

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/v2_window_glow.png`.**

---

Generate a single PNG: a soft radial-gradient **glow sprite** in
pixel-art style. Used as an emissive overlay for lit windows at
night in a top-down RPG.

**Image size: 128 × 128 px.** 8× nominal scale. The glow fills the
center, fading to magenta at the edges so the alpha mask is
unambiguous when processed.

**Background where the glow does not reach:** solid magenta `#FF00FF`.

**Style:** Soft radial glow in pixel art. NOT crisp. NOT anti-aliased
with smooth gradients. Use Bayer-style dithering at the falloff edge
so the gradient steps are visible per-pixel (this is the pixel-art
aesthetic — banded falloff, not smooth blur).

**Subject — a warm window glow:**

- **Bright core (innermost ~3 native pixels):** brightest yellow,
  almost white-hot.
- **Mid band (~5 native pixels around the core):** warm gold.
- **Outer band (~8 native pixels):** dim warm orange, transitioning
  to magenta via dither pattern.
- **Edge:** solid magenta. The dither transition between the warm
  orange and magenta should span ~3 pixels of mixed-color noise so
  the alpha mask is gradual but pixel-art-flat.

The glow should be ROUND (radial), not square. Center it precisely.

**Color palette — use ONLY these colors (Endesga 32 subset):**

| Use | Hex |
|---|---|
| Bright core | `#ffffff` |
| Hot white | `#fee761` |
| Warm gold | `#feae34` |
| Mid warm | `#f77622` |
| Outer warm | `#d77643` |
| Background | `#FF00FF` |

**Important:** no anti-aliasing, no smooth gradients. The falloff is
DITHERED — alternating pixels of two adjacent palette colors at the
transitions. This matches the pixel-art aesthetic of the rest of the
game.

**Output:** 128 × 128 PNG. Round glow centered. Magenta corners.

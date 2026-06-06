# `fx_particles_master` — visual effect sprites

A master sheet for one-off particle / FX sprites the renderer composites
ABOVE entities — combat hits, gold sparkles, dust kicks, healing
hearts, etc. Each sprite is a single frame; the renderer handles fade
+ scale animation procedurally.

For multi-frame animations (sword swing arc, attack flash), provide
the keyframes left-to-right in the same row.

**Save the result to:**

```
~/projects/agent_sim/art/raw/fx_particles_master.png
```

---

Generate a single PNG: a top-down pixel-art **FX / particle sprite
sheet** in the visual style of **Pokémon HeartGold / SoulSilver**
(Nintendo DS, 2009). 48 sprite cells in an 8×6 grid, each 128×128 px
(16×16 native).

**Image size: 1024 × 768 px.** 8 columns × 6 rows.

**Background between cells:** solid magenta `#FF00FF`, ≥ 4 px of
magenta between every sprite.

**Style:** Crisp pixel art, no anti-aliasing, 1 px outline (or NO
outline for soft glow effects — see per-row notes).

**Sheet layout — 48 cells:**

**Row 1 — Combat hit FX (cells 1–8) (3 frames each for 2 effects, plus 2 static):**
1. Sword slash arc — frame 1 (faint curved white slash, just starting)
2. Sword slash arc — frame 2 (full bright slash arc, white core, gold edges)
3. Sword slash arc — frame 3 (faded arc, almost gone)
4. Impact star — frame 1 (small white cross-burst)
5. Impact star — frame 2 (large bright star-burst, white core, yellow rays)
6. Impact star — frame 3 (faint expanded outline)
7. Blood splatter (small red droplet pattern, NOT cartoonish — 4–6
   tiny red dots arranged like a splash)
8. Crit indicator (yellow zigzag lightning bolt — single frame)

**Row 2 — Resource harvest FX (cells 9–16):**
9. Wood chip (single light-brown triangle)
10. Wood chip cluster (3 chips flying outward)
11. Sawdust puff (cream-colored cloud)
12. Stone shard (single gray chip)
13. Stone shard cluster (3 chips)
14. Stone dust puff (gray cloud)
15. Iron spark (small bright yellow speck)
16. Spark burst (5 yellow specks radiating outward)

**Row 3 — Smoke / fire / steam (cells 17–24):**
17. Smoke puff small (1 frame — light gray cloud, soft edges)
18. Smoke puff medium (1 frame — larger, slightly darker)
19. Smoke puff dispersing (1 frame — thin, irregular)
20. Steam wisp (white, more vertical than smoke)
21. Fire small (3-px-tall orange/yellow flame)
22. Fire medium (5-px-tall flame)
23. Fire large (7-px-tall flame, with red base)
24. Ember (single orange dot, slightly glowing)

**Row 4 — Economy / social FX (cells 25–32):**
25. Coin sparkle (single small gold star, 4-pointed)
26. Coin burst (5 gold stars radiating outward)
27. Gold pile growing (1 coin transitioning into 3 coins — single
   frame, suggests "got money")
28. Healing heart (red heart shape, slight halo, NO outline — pixel
   art heart silhouette)
29. Healing heart bright (with white sparkles around it)
30. Friendship icon (two interlocked rings, gold)
31. Question mark (yellow ?, for surprised reactions / unknown)
32. Exclamation mark (yellow !, for alerts)

**Row 5 — Environment FX (cells 33–40):**
33. Footprint dust (small gray puff, low and wide)
34. Water splash small (cyan splash, low + wide)
35. Water splash large (taller plume)
36. Water ripple (concentric cyan circles, NO outline)
37. Leaves falling (3 small green leaves drifting down)
38. Wind line (single horizontal cyan/white streak)
39. Snowflake (single white 6-point flake)
40. Raindrop (small cyan teardrop)

**Row 6 — Status / state icons (cells 41–48):**
41. Sleep "Z" (gray Z floating, single frame)
42. Speech bubble small (white cloud, empty inside — used by
    rendering layer as the bubble background)
43. Speech bubble pointed (with tail pointing down-left)
44. Thought bubble (3 round connected bubbles)
45. Shout starburst (jagged yellow outline, no fill — wraps text)
46. Whisper indicator (small "..." dots, faint)
47. Death cross (small X over a circle, gray, NOT cartoonish skull)
48. Resurrection / spawn glow (vertical white pillar — single frame)

**Color palette — use ONLY the Endesga 32 colors. Common usages:**

| Use | Hex |
|---|---|
| White (bright FX) | `#ffffff` |
| Cream highlight | `#fee761` |
| Gold | `#feae34` |
| Orange (fire mid) | `#f77622` |
| Red (fire base, blood, heart) | `#e43b44` |
| Deep red (blood shadow) | `#a22633` |
| Cyan (water, glass) | `#2ce8f5` |
| Sky blue (water shadow) | `#0099db` |
| Gray (smoke, dust) | `#c0cbdc` |
| Mid gray | `#8b9bb4` |
| Dark gray | `#5a6988` |
| Yellow (sparkle, !/?) | `#fee761` |
| Leaf green | `#63c74d` |
| Outline (where applicable) | `#181425` |
| Background | `#FF00FF` |

**Important:**
- Hit / spark / sparkle FX have NO outline — they're additive bright
  effects, so outlining defeats the look. Pure bright colors only.
- Smoke / fire / water effects have soft DITHERED edges (not smooth
  gradients — alternating pixels of two adjacent palette colors at
  the falloff).
- Icons (Z, !, ?, hearts, speech bubbles) DO have a 1px outline so
  they read clearly against the world.

**Output:** 1024 × 768 PNG, strong magenta between cells.

# `ui_icons` — toolbar + status + HUD icon sheet

Crisp icons for the DOM-side UI overlay — toolbar buttons, status
effect badges, HUD info badges. These do NOT live in the PixiJS
viewport; they're loaded as PNG into Solid components. Render at
exactly 64×64 px so the UI can display them sharply at 16-32 px on a
modern display.

**Save the result to:**

```
~/projects/agent_sim/art/raw/ui_icons.png
```

---

Generate a single PNG: a pixel-art **UI icon sheet** in the visual
style of **Pokémon HeartGold / SoulSilver** menus — clean, simple
shapes, strong outlines, gold/blue/red accent palette. 64 icons
arranged in an 8×8 grid, each cell 64×64 px output (single 32×32
native icon centered, NOT 16×16 like world sprites — these are UI
icons and need more detail at small size).

**Image size: 512 × 512 px.** 8 columns × 8 rows.

**Background between cells:** solid magenta `#FF00FF`, ≥ 4 px of
magenta between every icon.

**Style:** Crisp pixel art, no anti-aliasing, 1 px outline per icon.
Icons are flat (no 3/4 perspective — these are UI, not world).

**Sheet layout — 64 cells:**

**Row 1 — Toolbar actions (cells 1–8):**
1. Open book (rulebook — represents the World Rulebook button)
2. Trophy (leaderboards button)
3. Eye open (HUD show button)
4. Eye closed (HUD hide button)
5. Magnifying glass (inspector / search)
6. Map / compass (minimap toggle)
7. Speech bubble (dialogue / drama feed toggle)
8. Gear / cog (settings)

**Row 2 — Navigation / world (cells 9–16):**
9. Arrow pointing N
10. Arrow pointing S
11. Arrow pointing E
12. Arrow pointing W
13. Crosshair / target reticle
14. Map pin (location marker)
15. Plus / zoom in
16. Minus / zoom out

**Row 3 — Stats / status (cells 17–24):**
17. Heart (HP — red, with light highlight)
18. Heart half (HP partial)
19. Heart empty (HP zero / dead — outlined only)
20. Coin (gold — single yellow circle)
21. Coin stack (multiple)
22. Sword crossed with sword (combat / kills)
23. Shield (defense / defending)
24. Hammer crossed with anvil (crafting / construction)

**Row 4 — Vital signs / states (cells 25–32):**
25. Sleeping Z
26. Skull and crossbones (dead)
27. Question mark in circle (unknown / surprised)
28. Exclamation mark in triangle (alert)
29. Bandage / cross (heal / medic)
30. Flame (on fire status)
31. Snowflake (frozen status)
32. Poison droplet (green poison)

**Row 5 — Actions / activities (cells 33–40):**
33. Walking person (idle / wandering)
34. Running person (moving fast)
35. Pickaxe (mining)
36. Axe (chopping)
37. Hand grabbing (interact / pickup)
38. Hand giving (give item)
39. Open mouth speaking (speak action)
40. Cupped hand to ear (listening / hearing)

**Row 6 — Quest / social (cells 41–48):**
41. Scroll with seal (quest / contract)
42. Open letter (message)
43. Two interlocked hands (friendship)
44. Broken heart (rival / hostility)
45. Heart-eyes face (love)
46. Anger face (angry)
47. Smile face (happy)
48. Sad face (sad)

**Row 7 — Buildings / property (cells 49–56):**
49. House outline (building marker)
50. Padlock closed (locked)
51. Padlock open (unlocked)
52. Key (owns / has key)
53. Crown (ownership / rule)
54. Door (enter / exit)
55. Hammer (building / construction)
56. Trash / X (demolish)

**Row 8 — Misc / utility (cells 57–64):**
57. Clock face (time / tick)
58. Sun (day phase)
59. Moon (night phase)
60. Cloud (weather)
61. Tree (nature / outdoors)
62. Information "i" in circle (info)
63. Question mark (help)
64. Checkmark (done / confirm)

**Color palette — use ONLY the Endesga 32 colors. Common usages:**

| Use | Hex |
|---|---|
| Background base | `#ffffff` (for icons that want light bg) |
| Strong red | `#e43b44` |
| Dark red | `#a22633` |
| Gold | `#feae34` |
| Gold highlight | `#fee761` |
| Sky blue | `#0099db` |
| Deep blue | `#124e89` |
| Green | `#3e8948` |
| Bright green | `#63c74d` |
| Purple | `#68386c` |
| Light gray | `#c0cbdc` |
| Mid gray | `#8b9bb4` |
| Dark gray | `#3a4466` |
| Cream | `#ead4aa` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:**
- Every icon has a 1px dark outline.
- Each icon fits within ~48×48 of the 64×64 cell, leaving ~8 px of
  magenta padding all around.
- Icons should be HIGH-CONTRAST and read clearly at small sizes.

**Output:** 512 × 512 PNG, strong magenta between cells.

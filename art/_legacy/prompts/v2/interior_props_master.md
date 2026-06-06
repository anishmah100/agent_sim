# `interior_props_master` — furniture + interior decoration sheet

A master sheet of every interior prop the rendering layer needs to
replace the placeholder rectangles inside cottages, taverns, and the
town hall. Single sheet, gridded, magenta-keyed, each cell strictly
one prop.

**Save the result to:**

```
~/projects/agent_sim/art/raw/interior_props_master.png
```

---

Generate a single PNG: a top-down pixel-art **interior furniture +
props sprite sheet** in the visual style of **Pokémon HeartGold /
SoulSilver** (Nintendo DS, 2009). Many small props arranged in a grid,
each one occupying one 16×16 native cell shown at 8× scale (128×128 px
per cell). Slight 3/4 perspective — you see the top of each piece of
furniture plus a little of the front face.

**Image size: 1024 × 768 px.** 8 columns × 6 rows = 48 cells, each
128×128 px output.

**Background between cells:** solid magenta `#FF00FF`, with at least
4 px of magenta between every sprite so per-cell cropping is
unambiguous.

**Style:** Crisp pixel art, no anti-aliasing, 1 px outline per
sprite, slight 3/4 perspective.

**Sheet layout — 48 cells, top-to-bottom, left-to-right:**

**Row 1 — Tables + chairs (cells 1–8):**
1. Small wooden table (2 chair seats around it implied)
2. Round dining table with cloth (cream cloth on dark wood)
3. Long bench (wooden, no back)
4. Wooden chair (with backrest)
5. Padded stool (round, red cushion on dark legs)
6. Tavern bar counter (long, wooden, with a small mug on top)
7. Writing desk (small, with a quill+inkpot)
8. Side table with a candlestick (lit candle, small yellow flame)

**Row 2 — Beds + soft furnishings (cells 9–16):**
9. Single bed with red blanket + cream pillow (this REPLACES the
   `b` placeholder in the cottage interior)
10. Single bed with blue blanket
11. Double bed with red blanket (wider, fancier)
12. Cot / simple straw bed (no frame)
13. Wool rug (woven, geometric pattern in red/cream)
14. Bear-skin rug (brown, head pointing forward)
15. Curtains drawn (vertical heavy red fabric strip)
16. Cushion (round, gold-tasseled)

**Row 3 — Storage (cells 17–24):**
17. Wooden barrel (banded iron rings)
18. Tall wooden cabinet (closed doors)
19. Open cabinet (visible shelves with bottles)
20. Treasure chest (closed, gold trim, padlock)
21. Treasure chest (open, gold coins overflowing)
22. Wicker basket
23. Crate (square wooden, with rope handle)
24. Bookshelf (4 shelves, books in mixed colors)

**Row 4 — Kitchen + bar (cells 25–32):**
25. Cooking pot on a tripod over a small fire (orange flame visible)
26. Stack of plates / dishes
27. Mug of ale (wooden, frothing on top)
28. Wine bottle + goblet
29. Hanging cured meat (ham hock on a hook)
30. Sack of flour (cream, with a small spill)
31. Round of cheese on a board
32. Loaf of bread on a board

**Row 5 — Fireplaces + lighting (cells 33–40):**
33. Stone fireplace (lit, orange-red fire inside, smoke hint above)
34. Stone fireplace (unlit, just dark interior + logs)
35. Wall sconce (wooden bracket + lit yellow candle)
36. Wall sconce (unlit, just bracket + dark candle)
37. Floor candelabra (tall iron, 3 lit candles)
38. Lantern hanging from chain (lit yellow)
39. Lantern hanging from chain (unlit)
40. Small fire pit (stones in a circle around an open flame)

**Row 6 — Misc decor (cells 41–48):**
41. Painting on the wall (gold-framed, abstract landscape)
42. Tapestry (vertical, red with golden symbol)
43. Wooden coat rack (with a draped cloak)
44. Wash basin on a stand (water visible)
45. Mirror (oval, gold frame)
46. Small wooden stool with a sewing kit
47. Vase with flowers (red flowers in a blue vase)
48. Open scroll on a stand (visible pixel-art text squiggles)

**Color palette — use ONLY the Endesga 32 colors. Common usages:**

| Use | Hex |
|---|---|
| Wood light | `#b86f50` |
| Wood dark | `#733e39` |
| Wood deep shadow | `#3e2731` |
| Iron / dark metal | `#262b44` |
| Stone light | `#c0cbdc` |
| Stone shadow | `#8b9bb4` |
| Cream cloth / pillow | `#ead4aa` |
| Cream shadow | `#e4a672` |
| Red blanket / fabric | `#a22633` |
| Red highlight | `#e43b44` |
| Blue blanket | `#124e89` |
| Blue highlight | `#0099db` |
| Gold trim / coins / candle flame | `#feae34` |
| Gold highlight | `#fee761` |
| Fire orange | `#f77622` |
| Fire bright | `#fee761` |
| Plant leaf green | `#63c74d` |
| Plant flower red | `#e43b44` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** every sprite is contained within its 128×128 cell with
~8 px of magenta padding on every side. No sprite touches a cell
boundary. No labels or text anywhere.

**Output:** 1024 × 768 PNG, strong magenta between cells.

# `items_master_v2` — full item / inventory sprite sheet

Replaces and extends the v1 items sheet. Every inventory item agents
can pick up, drop, give, equip, or trade. Each in a strict grid so
the atlas can address them by index.

**Save the result to:**

```
~/projects/agent_sim/art/raw/items_master_v2.png
```

---

Generate a single PNG: a top-down pixel-art **items / inventory sprite
sheet** in the visual style of **Pokémon HeartGold / SoulSilver**
(Nintendo DS, 2009). 64 items in an 8×8 grid, each cell 128×128 px
output (16×16 native at 8× scale). Items are shown as if lying flat
on the ground, slight 3/4 perspective.

**Image size: 1024 × 1024 px.** 8 columns × 8 rows = 64 cells.

**Background between cells:** solid magenta `#FF00FF`, ≥ 4 px of
magenta between every sprite.

**Style:** Crisp pixel art, no anti-aliasing, 1 px outline per sprite.

**Sheet layout — 64 cells, top-to-bottom, left-to-right:**

**Row 1 — Resources from gathering (cells 1–8):**
1. Wood log (cut log, bark visible)
2. Plank (sawn wooden plank)
3. Stone block (rough-cut gray)
4. Refined stone (smooth gray block)
5. Iron ore chunk (gray with red streaks)
6. Iron ingot (polished bar)
7. Coal chunk (black, irregular)
8. Wheat sheaf (golden bundle)

**Row 2 — Tools (cells 9–16):**
9. Axe (felling)
10. Pickaxe
11. Hammer
12. Saw (hand saw)
13. Shovel
14. Sickle (curved blade)
15. Fishing rod
16. Bucket (wooden, empty)

**Row 3 — Weapons (cells 17–24):**
17. Short sword (steel blade)
18. Long sword (longer steel blade)
19. Dagger
20. Wooden club
21. Spear
22. Bow (recurve)
23. Quiver of arrows
24. Crossbow

**Row 4 — Armor / wearables (cells 25–32):**
25. Leather helmet
26. Iron helmet
27. Leather chestplate
28. Iron chestplate
29. Wooden shield
30. Iron shield
31. Boots (leather)
32. Cloak (folded)

**Row 5 — Food / consumables (cells 33–40):**
33. Loaf of bread
34. Apple (red)
35. Wheel of cheese
36. Fish (whole, raw)
37. Cooked fish (on a plank)
38. Mug of ale (foam visible)
39. Wine bottle
40. Healing potion (red bottle with cork)

**Row 6 — Quest / interaction items (cells 41–48):**
41. Key (gold, ornate)
42. Key (iron, simple)
43. Sealed scroll
44. Open letter
45. Map (rolled, with red ribbon)
46. Magnifying glass / lens
47. Lockpick set
48. Compass (brass)

**Row 7 — Valuables (cells 49–56):**
49. Single gold coin
50. Small pile of gold coins (~5)
51. Large pile of gold coins (~20, overflowing)
52. Gem — ruby (red)
53. Gem — sapphire (blue)
54. Gem — emerald (green)
55. Gold ring
56. Golden chalice

**Row 8 — Misc / utility (cells 57–64):**
57. Torch (unlit)
58. Torch (lit, orange flame)
59. Wooden bucket of water (water visible)
60. Glass bottle (empty, transparent — cyan tint)
61. Sack / bag (cloth, drawstring)
62. Lantern (lit, yellow glow)
63. Lantern (unlit)
64. Empty / placeholder slot (faint dotted square, indicates "no
   item" — used in inventory UI for empty slots)

**Color palette — use ONLY the Endesga 32 colors. Common usages:**

| Use | Hex |
|---|---|
| Wood light | `#b86f50` |
| Wood dark | `#733e39` |
| Wood shadow | `#3e2731` |
| Iron / steel light | `#c0cbdc` |
| Iron / steel | `#8b9bb4` |
| Iron deep shadow | `#5a6988` |
| Iron rust spots | `#a22633` |
| Stone light | `#c0cbdc` |
| Stone shadow | `#8b9bb4` |
| Coal black | `#181425` |
| Coal highlight | `#3a4466` |
| Wheat / hay gold | `#feae34` |
| Gold | `#feae34` |
| Gold highlight | `#fee761` |
| Leather brown | `#b86f50` |
| Leather dark | `#733e39` |
| Cloth / cloak gray | `#5a6988` |
| Bread golden | `#e4a672` |
| Apple red | `#e43b44` |
| Cheese yellow | `#fee761` |
| Fish silver | `#c0cbdc` |
| Fish blue tint | `#0099db` |
| Ale foam | `#ead4aa` |
| Wine red | `#a22633` |
| Wine bottle dark green | `#193c3e` |
| Potion red | `#ff0044` |
| Glass transparent (use cyan) | `#2ce8f5` |
| Ruby red | `#e43b44` |
| Sapphire blue | `#0099db` |
| Emerald green | `#3e8948` |
| Flame orange | `#f77622` |
| Flame bright | `#fee761` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** each item is contained within its 128×128 cell with
~12 px of magenta padding on every side. No item touches a cell
boundary. No text or labels anywhere.

**Output:** 1024 × 1024 PNG, strong magenta between cells.

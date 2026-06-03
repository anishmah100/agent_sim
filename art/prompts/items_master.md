# `items_master` — 64 item icons on one sheet

All v1 items in a single sprite sheet. Food, tools, weapons, armor, resources, money, misc. Each item is a 16×16 native icon usable for inventory + the world (item lying on the ground).

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/items_master.png`.**

---

Generate a single PNG: a top-down pixel-art **item icon sheet** for an RPG in the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS, 2009). 64 distinct item icons arranged in a grid.

**Image size: 1024 × 1024 px.** 8× nominal scale. Each "item slot" is **128 × 128 px in the output** (16 × 16 native). The icon should fill most of the slot (centered, with 1-2 px of native padding around it).

**Background where no item exists:** solid magenta `#FF00FF`.

**Style:** Crisp pixel art. No anti-aliasing. Hard color transitions. Each item has a 1-px dark outline at native (= 8 px at 8× scale). Items viewed slightly top-down with subtle 3/4 perspective hint — a sword shows its blade with a tiny bit of side; an apple shows its top + a hint of its bulge.

**Sheet layout — 8 cols × 8 rows = 64 items:**

**Row 1 (y=0–127): FOOD**
- Col 1: apple (red round + small stem + leaf)
- Col 2: bread loaf (golden brown, oval)
- Col 3: cheese wedge (yellow, triangle)
- Col 4: fish (silver, side-view, eye visible)
- Col 5: berries (cluster of 3 red berries on a small stem)
- Col 6: mushroom (red cap with white spots — for eating)
- Col 7: pumpkin (orange, ridged)
- Col 8: water flask (round bottle, blue liquid, brown cork)

**Row 2 (y=128–255): TOOLS**
- Col 1: axe (wooden handle + steel head, double-headed)
- Col 2: pickaxe (wooden handle + steel pointed head)
- Col 3: fishing rod (long thin rod, line, small hook)
- Col 4: hammer (wooden handle + steel head, blacksmith style)
- Col 5: shovel (wooden handle + steel scoop)
- Col 6: hoe (wooden handle + steel blade, gardening)
- Col 7: rope (coiled brown rope)
- Col 8: lantern (small metal frame with yellow glow inside)

**Row 3 (y=256–383): WEAPONS**
- Col 1: short sword (steel blade pointing up-right, brown grip)
- Col 2: long sword (longer blade, ornate guard)
- Col 3: dagger (small, pointed, slim)
- Col 4: bow (curved wooden bow with string)
- Col 5: arrow (single arrow, feathered tail)
- Col 6: staff (wooden staff with a small blue gem at the top)
- Col 7: spear (long shaft + steel head)
- Col 8: war hammer (thick metal head, brown handle)

**Row 4 (y=384–511): ARMOR + clothing**
- Col 1: helmet (steel, kettle style)
- Col 2: chest plate (steel breastplate)
- Col 3: shield (round, wooden + iron rim)
- Col 4: boots (brown leather)
- Col 5: gloves (brown leather pair)
- Col 6: cloak (green hooded cloak laid out flat)
- Col 7: leather tunic
- Col 8: chain mail shirt (gray metallic rings)

**Row 5 (y=512–639): RESOURCES (raw materials)**
- Col 1: wood log (brown cylinder, rings on the end)
- Col 2: stone (gray, rounded)
- Col 3: ore — copper (orange-red chunk)
- Col 4: ore — iron (gray chunk)
- Col 5: ore — gold (yellow chunk)
- Col 6: leather (brown folded piece)
- Col 7: cloth (white folded piece)
- Col 8: herbs (small green bundle)

**Row 6 (y=640–767): MONEY + valuables**
- Col 1: gold coin single (round, yellow, with a small mark)
- Col 2: gold coin small pile (3 coins stacked)
- Col 3: gold coin large pile (6-10 coins in a heap)
- Col 4: silver coin single
- Col 5: gem ruby (red, faceted)
- Col 6: gem emerald (green)
- Col 7: gem sapphire (blue)
- Col 8: coin purse (brown leather pouch, drawstring)

**Row 7 (y=768–895): POTIONS + magic**
- Col 1: red potion (round flask, red liquid)
- Col 2: blue potion (round flask, blue liquid)
- Col 3: green potion (round flask, green liquid)
- Col 4: yellow potion (round flask, yellow liquid)
- Col 5: scroll (rolled parchment, brown)
- Col 6: book (closed, brown leather cover)
- Col 7: spell tome (open, with glowing yellow pages)
- Col 8: crystal (clear quartz cluster, pointing up)

**Row 8 (y=896–1023): MISC + utility**
- Col 1: key (brown / brass, decorative bow)
- Col 2: chest closed (small treasure chest, brown wood + gold trim)
- Col 3: chest open (lid up, gold coins visible inside)
- Col 4: sack (brown burlap sack, tied with string)
- Col 5: barrel (small, wooden + iron bands)
- Col 6: crate (wooden cube)
- Col 7: torch (wooden stick + flaming top)
- Col 8: flower picked (a single red flower with stem)

**Color palette — use ONLY these 18 colors:**

| Use | Hex |
|---|---|
| Apple / potion red | `#e43b44` |
| Pumpkin orange | `#f77622` |
| Bread / gold / wood-light | `#feae34` |
| Cheese / coin yellow | `#fee761` |
| Mushroom spot / cloth white | `#ffffff` |
| Berry red dark | `#a22633` |
| Wood light | `#b86f50` |
| Wood dark | `#733e39` |
| Wood very dark | `#3e2731` |
| Steel light (blades, plate) | `#c0cbdc` |
| Steel shadow | `#8b9bb4` |
| Steel dark (helmet) | `#5a6988` |
| Stone gray | `#3a4466` |
| Liquid blue / sapphire | `#0099db` |
| Liquid green / emerald | `#63c74d` |
| Magic glow / candle | `#fee761` |
| Crystal pale | `#2ce8f5` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 1024 PNG. Magenta between/around items. No labels, no grid lines, no annotations.

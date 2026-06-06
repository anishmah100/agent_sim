# `tileset_interior` — floors, walls, furniture

Indoor tiles. Used inside houses/shops when an agent walks through a door portal.

**Paste below into ChatGPT chat. Save to `~/projects/agent_sim/art/raw/tileset_interior.png`.**

---

Generate a single PNG: a top-down pixel-art **interior tileset** in the visual style of **Pokémon HeartGold / SoulSilver** (Nintendo DS, 2009) — the rooms INSIDE houses, taverns, and shops. Floors, walls, furniture, decorations.

**Image size: 1024 × 1024 px.** 8× nominal scale. Each "tile slot" is **128 × 128 px in the output** (16 × 16 native). Some furniture items span multiple slots.

**Background where no tile exists:** solid magenta `#FF00FF`.

**Style:** Crisp pixel art, no AA. Slight 3/4 perspective — interior walls show their FRONT FACE (you see the wallpaper or wood paneling). Furniture has a slight visible front/top.

**Sheet layout — 8 cols × 8 rows = 64 slots:**

**Row 1 (y=0–127): FLOORS — wood plank variants**
- Col 1: wood floor light (warm honey color)
- Col 2: wood floor dark (deeper brown)
- Col 3: wood floor with knot detail
- Col 4: wood floor worn / scuffed
- Col 5: wood floor edge fade (where it meets a wall)
- Col 6: wood floor with welcome mat (red rectangle)
- Col 7: wood floor with rug center (decorative red pattern)
- Col 8: wood floor with rug edge

**Row 2 (y=128–255): FLOORS — stone + tile variants**
- Col 1: stone floor (large gray tiles)
- Col 2: stone floor checkered (gray + dark gray)
- Col 3: marble floor (light with subtle veining — use white + light gray hints)
- Col 4: brick floor (red-brown bricks)
- Col 5: cellar floor (dark gray with cracks)
- Col 6: carpet red (decorative)
- Col 7: carpet blue
- Col 8: dirt floor (cellar / cave)

**Row 3 (y=256–383): WALLS — wood plank**
- Col 1: wood wall flat (vertical planks)
- Col 2: wood wall with horizontal beam
- Col 3: wood wall with framed picture (small picture frame)
- Col 4: wood wall with wall-mounted candle holder (lit candle)
- Col 5: wood wall with window (sees outside daylight — blue tinted)
- Col 6: wood wall corner (interior corner)
- Col 7: wood wall top edge (where wall meets ceiling — slightly darker line at top)
- Col 8: wood wall door frame (an empty door frame opening)

**Row 4 (y=384–511): WALLS — stone / plaster**
- Col 1: stone wall flat
- Col 2: stone wall with banner (red banner hanging)
- Col 3: stone wall with torch (lit, yellow flame)
- Col 4: plaster wall white (smooth painted)
- Col 5: plaster wall with shelf (small wooden shelf with a vase)
- Col 6: stone wall corner
- Col 7: stone wall top edge
- Col 8: stone wall arched doorway

**Row 5 (y=512–639): FURNITURE — beds + sleeping**
- Cols 1–2 (256w × 128h): single bed — wooden frame, white pillow, blue blanket. Spans 2 horizontal slots.
- Cols 3–4 (256w × 128h): double bed — wider, red blanket, two pillows
- Col 5: bed roll (simple sleeping mat on the floor, brown blanket)
- Col 6: pillow (single, white)
- Col 7: nightstand (small wooden table with a candle on top)
- Col 8: wardrobe (tall wooden cabinet, closed doors)

**Row 6 (y=640–767): FURNITURE — tables + chairs**
- Cols 1–2 (256w × 128h): large dining table (wooden, with a plate of bread + a mug)
- Col 3: small round side table (wooden)
- Col 4: chair facing south (wooden, simple)
- Col 5: chair facing north (back of chair)
- Col 6: chair facing west
- Col 7: chair facing east
- Col 8: stool (wooden, no back)

**Row 7 (y=768–895): FURNITURE — storage + crafting**
- Col 1: wooden chest closed (lid down)
- Col 2: wooden chest open (lid up, showing gold coins inside)
- Col 3: barrel (wooden, banded with iron)
- Col 4: barrel open (top off, showing contents — apples or grain)
- Col 5: crate (wooden cube)
- Col 6: bookshelf (wooden shelf with multicolored book spines)
- Col 7: anvil (iron, on wooden block)
- Col 8: forge (small brick furnace with orange glow visible)

**Row 8 (y=896–1023): DECORATIONS + misc**
- Col 1: fireplace (stone, with orange flames visible)
- Col 2: cauldron (iron pot on small fire, green liquid)
- Col 3: clock (grandfather, wooden tall)
- Col 4: painting (framed picture on the wall — abstract landscape colors)
- Col 5: vase with flowers (blue vase, red flowers)
- Col 6: bookshelf (small) with books
- Col 7: candle on a candlestick (lit)
- Col 8: chandelier hanging from ceiling (small, yellow candles)

**Color palette — use ONLY these 16 colors:**

| Use | Hex |
|---|---|
| Wood floor light | `#b86f50` |
| Wood floor mid | `#733e39` |
| Wood floor dark | `#3e2731` |
| Stone floor light | `#c0cbdc` |
| Stone floor shadow | `#8b9bb4` |
| Stone floor very dark | `#3a4466` |
| Plaster white | `#ffffff` |
| Marble vein light | `#ead4aa` |
| Carpet / blanket red | `#e43b44` |
| Blanket blue | `#0099db` |
| Brick red-brown | `#a22633` |
| Fire / candle flame | `#feae34` |
| Fire glow yellow | `#fee761` |
| Cauldron green | `#63c74d` |
| Gold coin glint | `#feae34` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Output:** 1024 × 1024 PNG. Magenta between/around all tiles. No labels, no grid.

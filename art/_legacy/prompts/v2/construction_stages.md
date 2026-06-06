# `construction_stages` — building-under-construction visuals

Sprites for the Construction system's intermediate states. When an
agent places a blueprint and then advances it through 1, 2, 3, 4
hits, the world needs to show the visual progression — not just
"slab → finished cottage" at the last hit. This sheet provides 5
states (0%, 25%, 50%, 75%, 100% / scaffold removed) for a cottage
shape, plus a generic blueprint outline that overlays any building.

**Save the result to:**

```
~/projects/agent_sim/art/raw/construction_stages.png
```

---

Generate a single PNG: a top-down pixel-art **construction stages
sprite sheet** in the visual style of **Pokémon HeartGold / SoulSilver**
(Nintendo DS, 2009). 12 sprite cells in a 4×3 grid, each cell 256×256
px (representing 2×2 tile sprites at 16×16 native, 8× scale).

**Image size: 1024 × 768 px.** 4 columns × 3 rows.

**Background between cells:** solid magenta `#FF00FF`, ≥ 8 px of
magenta between every sprite.

**Style:** Crisp pixel art, no anti-aliasing, 1 px outline, slight 3/4
perspective. Each construction stage shows what would otherwise be a
hidden state — the player should be able to glance at a building and
say "ah, that's about halfway done."

**Sheet layout — 12 cells:**

**Row 1 — Cottage construction stages (cells 1–4):**
1. **Stage 0 — blueprint ghost.** A faint translucent outline of the
   finished cottage. Use very light blue + white outline at ~40%
   apparent alpha (achieved via pixel-level transparency — mostly
   magenta with a few light blue pixels showing the wall footprint).
   No roof yet. This represents "I just placed the blueprint."
2. **Stage 1 — foundation laid.** Just the stone foundation at the
   bottom 2/3 of where the cottage would be. Light gray stones, NO
   walls above. A small wheelbarrow of stones beside it.
3. **Stage 2 — walls partial.** Foundation done + the first ~1/2
   tile of wooden wall planks built on top. A wooden scaffold ladder
   leaning against the wall.
4. **Stage 3 — walls full, no roof.** Foundation + full wall height,
   doorway visible (open opening, no door yet), windows as open
   holes, but NO roof. Some scaffolding still on the front.

**Row 2 — Cottage construction stages continued (cells 5–8):**
5. **Stage 4 — roof going on.** Full walls + roof rafters visible
   (wooden cross-beams) + about half the red tile roof in place.
   Scaffolding on one side.
6. **Stage 5 — finished cottage.** Identical to the regular cottage
   sprite — this is the "construction complete" stage that gets
   swapped in. (Match `bld:000` cottage's visual.)
7. **Wreckage / demolished cottage.** Used when an agent demolishes
   a built cottage. Shows the foundation + scattered planks + small
   smoke puff.
8. **Wreckage / demolished blueprint.** A simpler ruin — just the
   foundation and a small pile of materials, for when an in-progress
   build is demolished.

**Row 3 — Universal overlays (cells 9–12):**
9. **Scaffolding sprite — small** (single tile). Wooden frame
   structure, can be overlaid on any building under construction.
10. **Scaffolding sprite — large** (2-tile-wide). Same style, wider.
11. **Construction progress bar** — a small horizontal bar at the
   base of any blueprint, showing 25/50/75% fill states. Provide a
   single sprite showing the BAR FRAME (empty rectangle); fill is
   rendered per-percent by the engine.
12. **Worker tools overlay** — a small pile of pickaxes / hammers /
    wooden planks resting next to a construction site (use as a
    decorative addition).

**Color palette — use ONLY the Endesga 32 colors. Common usages:**

| Use | Hex |
|---|---|
| Blueprint ghost light blue | `#2ce8f5` |
| Blueprint ghost white | `#ffffff` |
| Stone foundation light | `#c0cbdc` |
| Stone foundation shadow | `#8b9bb4` |
| Wood plank light (walls) | `#b86f50` |
| Wood plank dark | `#733e39` |
| Wood scaffolding | `#b86f50` |
| Roof tile red | `#e43b44` |
| Roof tile shadow | `#a22633` |
| Roof rafter dark wood | `#3e2731` |
| Smoke puff | `#c0cbdc` |
| Doorway dark interior | `#181425` |
| Outline | `#181425` |
| Background | `#FF00FF` |

**Important:** the stage progression should be VISUALLY MONOTONIC —
each stage clearly shows MORE construction than the previous. A
viewer pausing mid-build should immediately understand "ok this is
~50% done." The scaffolding sprites in row 3 are reusable overlays
the engine can composite onto any in-progress building.

**Output:** 1024 × 768 PNG, strong magenta between cells.

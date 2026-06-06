#!/usr/bin/env python3
"""D7 — promote scattered item DECORATIONS in worlds/eldoria/world.json
to actual item ENTITIES so they appear in agent observations + can be
picked up + can be looted.

The pre-D8 scatter (`scatter_items_eldoria.py`) wrote items as
DECORATIONS with `sprite: "item:apple"`. Decorations are visual-only
and don't appear in visible_items[] or accept the pickup verb. This
script:

  1. Walks `worlds/eldoria/world.json`.
  2. For every decoration with sprite starting with "item:", REMOVES
     it from `decorations` and PROMOTES it to an entity in
     `entities` with archetype="item".
  3. Synthesizes a unique entity_id per item ("item_<n>") and writes
     the sprite + quantity into the entity's Extras.
  4. Adds D7's extra wealth scatter — concentrated gold/gem piles
     clustered around the experiment hub (Crossroads market) and a
     thinner spread across the wider map. This is the "world seeds
     wealth, agents circulate" half of D7.

Deterministic — fixed seed (different from scatter_items_eldoria's so
the two scripts don't shadow each other).
"""
from __future__ import annotations
import json, random, sys
from pathlib import Path

WORLD_PATH = Path("worlds/eldoria/world.json")

# D5 — experiment hub.
HUB = (772, 894)
HUB_RADIUS = 40   # gold concentrated near hub

# Wealth piles to scatter beyond what scatter_items_eldoria already
# placed (which was mostly mixed weapons/food/etc). These are the
# pure wealth piles for D7's "first to find them gets them" dynamic.
NEW_HUB_PILES = [
    ("coin_single",       12),
    ("coins_small_pile",  18),
    ("coins_large_pile",   8),
    ("gem_emerald",        3),
    ("gem_ruby",           3),
    ("gem_sapphire",       3),
    ("chalice_gold",       2),
]

# Sparser wilderness scatter — treasure to discover further afield.
WIDER_PILES = [
    ("coin_single",       30),
    ("coins_small_pile",  20),
    ("gem_emerald",        5),
    ("gem_ruby",           5),
    ("gem_sapphire",       5),
]

random.seed(0xD7E1D)


def main() -> int:
    if not WORLD_PATH.exists():
        print(f"error: {WORLD_PATH} not found (run from repo root)", file=sys.stderr)
        return 1
    world = json.loads(WORLD_PATH.read_text())
    decos = world.get("decorations", [])
    width, height = world["width_tiles"], world["height_tiles"]
    tiles = world["tiles"]
    legend = world["tiles_legend"]
    walkable_glyphs = {g for g, name in legend.items() if name in {"grass", "dirt", "path", "sand"}}

    def tile_walkable(x: int, y: int) -> bool:
        if not (0 <= x < width and 0 <= y < height):
            return False
        return tiles[y][x] in walkable_glyphs

    # Build occupancy set so we don't drop new items on top of buildings.
    occupied: set[tuple[int, int]] = set()
    for d in decos:
        if d.get("walkable", False):
            continue
        bx, by = d["x"], d["y"]
        fw = d.get("footprint_w", 1)
        fh = d.get("footprint_h", 1)
        for dx in range(fw):
            for dy in range(fh):
                occupied.add((bx + dx, by + dy))

    entities = world.get("entities", [])
    if not isinstance(entities, list):
        entities = []
    seq = 1

    def next_eid() -> str:
        nonlocal seq
        eid = f"item_{seq}"
        seq += 1
        return eid

    def add_item_entity(x: int, y: int, kind: str, quantity: int = 1):
        sprite = f"item:{kind}"
        eid = next_eid()
        entities.append({
            "entity_id": eid,
            "archetype": "item",
            "pos": [x, y],
            "facing": "S",
            "display_name": kind,
            "extras": {"sprite": sprite, "quantity": quantity, "source": "world_seed"},
        })

    # ── Pass 1 — promote existing scattered item decorations to entities ──
    kept_decos = []
    promoted = 0
    for d in decos:
        sprite = d.get("sprite", "")
        if sprite.startswith("item:"):
            kind = sprite[len("item:"):]
            add_item_entity(d["x"], d["y"], kind)
            promoted += 1
            continue
        kept_decos.append(d)

    # ── Pass 2 — concentrated hub wealth ──
    hub_added = 0
    for kind, count in NEW_HUB_PILES:
        for _ in range(count):
            for _attempt in range(20):
                dx = random.randint(-HUB_RADIUS, HUB_RADIUS)
                dy = random.randint(-HUB_RADIUS, HUB_RADIUS)
                x = HUB[0] + dx
                y = HUB[1] + dy
                if not tile_walkable(x, y):
                    continue
                if (x, y) in occupied:
                    continue
                # Quantity for coin piles = real numeric amount.
                qty = 1
                if "coins_large_pile" in kind:
                    qty = random.randint(30, 80)
                elif "coins_small_pile" in kind:
                    qty = random.randint(5, 20)
                add_item_entity(x, y, kind, qty)
                hub_added += 1
                break

    # ── Pass 3 — wider scatter ──
    wide_added = 0
    for kind, count in WIDER_PILES:
        for _ in range(count):
            for _attempt in range(50):
                x = random.randint(0, width - 1)
                y = random.randint(0, height - 1)
                if not tile_walkable(x, y):
                    continue
                if (x, y) in occupied:
                    continue
                qty = 1
                if "coins_small_pile" in kind:
                    qty = random.randint(5, 20)
                add_item_entity(x, y, kind, qty)
                wide_added += 1
                break

    world["decorations"] = kept_decos
    world["entities"] = entities
    WORLD_PATH.write_text(json.dumps(world))
    print(f"promoted {promoted} item decorations → entities")
    print(f"added {hub_added} hub-clustered wealth piles around {HUB} r={HUB_RADIUS}")
    print(f"added {wide_added} wider-scatter wealth piles")
    print(f"total item entities now: {sum(1 for e in entities if e.get('archetype') == 'item')}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

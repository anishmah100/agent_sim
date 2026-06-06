#!/usr/bin/env python3
"""Scatter v2 item-master sprites across Eldoria as walkable decorations.

Each placement is a decoration record with sprite="item:NAME" — the
frontend renderer maps `item:NAME` through the sprite catalog to
art/processed/v2_items_master_v2/NAME.png.

We cluster items near existing building decorations (towns, cottages,
stalls) so they look like dropped loot / wares / debris rather than
floating in empty fields, and pick tile categories that fit each item
(food near markets, weapons near guards, tools near builders).

Deterministic — fixed seed — so a re-run produces the same world.
"""
from __future__ import annotations
import json, random, sys
from pathlib import Path

WORLD_PATH = Path("worlds/eldoria/world.json")

# Item sprite cohorts. Each cohort picks roughly the kinds of items
# that would naturally appear near a particular building archetype.
COHORTS = {
    "market":  ["apple", "bread_loaf", "cheese_wheel", "fish_cooked", "fish_raw",
                "bottle_empty", "bucket_empty", "bucket_water", "chalice_gold"],
    "smith":   ["axe", "hammer", "sword_short", "dagger", "chestplate_iron",
                "helmet_iron", "club_wood", "wood_log"],
    "scout":   ["bow", "crossbow", "boots_leather", "chestplate_leather",
                "helmet_leather", "cloak_folded", "fishing_rod", "compass"],
    "loot":    ["coin_single", "coins_small_pile", "coins_large_pile",
                "gem_emerald", "gem_ruby", "gem_sapphire"],
    "misc":    ["coal"],
}

# Building sprite → cohort. Best guesses from the bld:* names that
# appear in worlds/eldoria/world.json decorations.
BUILDING_TO_COHORT = {
    "bld:granary":      "market",
    "bld:stall_red_bread_open": "market",
    "bld:stall_blue_fruit_open": "market",
    "bld:stall_green_meat_open": "market",
    "bld:well":         "market",
    "bld:blacksmith":   "smith",
    "bld:watchtower":   "scout",
    "bld:town_hall":    "loot",
}

# Building names whose name starts with one of these → cohort. Catches
# stall_* variants not explicitly listed above.
PREFIX_COHORT = [("bld:stall_", "market")]

random.seed(0xE1D0)

def cohort_for(sprite: str) -> str | None:
    if sprite in BUILDING_TO_COHORT:
        return BUILDING_TO_COHORT[sprite]
    for pfx, cohort in PREFIX_COHORT:
        if sprite.startswith(pfx):
            return cohort
    return None

def main() -> int:
    if not WORLD_PATH.exists():
        print(f"error: {WORLD_PATH} not found (run from repo root)", file=sys.stderr)
        return 1

    world = json.loads(WORLD_PATH.read_text())
    decos = world.get("decorations", [])
    if not decos:
        print("error: world has no decorations to anchor items to", file=sys.stderr)
        return 1

    width = world["width_tiles"]
    height = world["height_tiles"]
    tiles = world["tiles"]
    legend = world["tiles_legend"]
    # Walkable glyphs in this bundle: grass / dirt / path / sand. Stone
    # ('#') and water ('W') are off-limits for items.
    walkable_glyphs = {g for g, name in legend.items() if name in {"grass", "dirt", "path", "sand"}}

    def tile_at(x: int, y: int) -> str:
        if not (0 <= x < width and 0 <= y < height):
            return "#"
        return tiles[y][x]

    # Build an occupancy set of decoration footprints so items don't
    # land on top of buildings.
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

    placed: list[dict] = []
    item_seen: set[tuple[int, int]] = set()

    def place(x: int, y: int, sprite: str) -> bool:
        if (x, y) in occupied or (x, y) in item_seen:
            return False
        if tile_at(x, y) not in walkable_glyphs:
            return False
        item_seen.add((x, y))
        placed.append({
            "x": x, "y": y,
            "sprite": f"item:{sprite}",
            "height_tiles": 1,
            "footprint_w": 1, "footprint_h": 1,
            "walkable": True,
        })
        return True

    # Pass 1 — cluster items around cohort buildings. Sample a
    # fraction of each cohort and place 1–2 items each, capped to
    # avoid flooding (Eldoria has 1000+ buildings and we want items
    # to feel curated, not blanket the whole map).
    PER_COHORT_CAP = {"market": 80, "smith": 30, "scout": 30, "loot": 30}
    placed_by_cohort: dict[str, int] = {k: 0 for k in PER_COHORT_CAP}
    cohort_targets = [(d, cohort_for(d.get("sprite", ""))) for d in decos]
    cohort_targets = [(d, c) for d, c in cohort_targets if c]
    random.shuffle(cohort_targets)
    for d, cohort in cohort_targets:
        if placed_by_cohort.get(cohort, 0) >= PER_COHORT_CAP.get(cohort, 0):
            continue
        names = COHORTS[cohort]
        # 1–2 items per chosen building.
        for _ in range(random.randint(1, 2)):
            for _attempt in range(6):
                dx = random.randint(-3, 3 + d.get("footprint_w", 1))
                dy = random.randint(-3, 3 + d.get("footprint_h", 1))
                if place(d["x"] + dx, d["y"] + dy, random.choice(names)):
                    placed_by_cohort[cohort] += 1
                    break

    # Pass 2 — sprinkle ~40 items in the wilderness so distant tiles
    # have something to discover too. Pick random tiles, accept the
    # first ~40 that are walkable + unoccupied.
    wilderness_pool = COHORTS["scout"] + COHORTS["loot"] + COHORTS["misc"] + ["wood_log", "apple"]
    target = 40
    tries = 0
    while target > 0 and tries < 5000:
        tries += 1
        x = random.randint(0, width - 1)
        y = random.randint(0, height - 1)
        if place(x, y, random.choice(wilderness_pool)):
            target -= 1

    # Append, don't replace — preserve every existing decoration.
    world["decorations"] = decos + placed
    WORLD_PATH.write_text(json.dumps(world))
    print(f"scattered {len(placed)} items across Eldoria "
          f"(near-building: {len(placed) - (40 - target)}, wilderness: {40 - target})")
    return 0

if __name__ == "__main__":
    sys.exit(main())

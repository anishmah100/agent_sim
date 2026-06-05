"""Wilderness ring around Oak Hollow.

Procedural fill of a larger world: dense forest belt around the central
town area, scattered berry bushes + mushroom patches (forageables),
monster spawn tiles, and a dungeon mouth (portal tile that warps to the
dungeon interior map).

Run:
  python worlds/_design/wilderness.py
Writes:
  worlds/dev_wilderness.json — extended Oak Hollow at 200×120 with the
  wilderness ring.
"""

import json
import math
import random
from pathlib import Path

W = 200
H = 120

# Center the town inside the larger world.
TOWN_CX, TOWN_CY = W // 2, H // 2
TOWN_W, TOWN_H = 60, 40

GRASS, DIRT, PATH, WATER, STONE, WALL, FLOOR, VOID = "g", "d", "p", "w", "s", "W", "f", "."


def make_grid():
    return [[GRASS for _ in range(W)] for _ in range(H)]


def fill_rect(g, x0, y0, x1, y1, t):
    for y in range(max(0, y0), min(H, y1 + 1)):
        for x in range(max(0, x0), min(W, x1 + 1)):
            g[y][x] = t


def fill_circle(g, cx, cy, r, t):
    for y in range(max(0, int(cy - r - 1)), min(H, int(cy + r + 2))):
        for x in range(max(0, int(cx - r - 1)), min(W, int(cx + r + 2))):
            if (x - cx) ** 2 + (y - cy) ** 2 <= r * r:
                g[y][x] = t


def build_wilderness():
    g = make_grid()
    rng = random.Random(42)

    # Central river flows W → E through the middle of the world.
    river_y = H // 2
    for x in range(W):
        for dy in range(-3, 4):
            if 0 <= river_y + dy < H:
                g[river_y + dy][x] = WATER

    # Scattered ponds in the wilderness.
    for _ in range(8):
        cx = rng.randint(10, W - 10)
        cy = rng.randint(10, H - 10)
        if abs(cy - river_y) < 8:
            continue
        fill_circle(g, cx, cy, rng.randint(3, 7), WATER)

    # Roads radiating out from town to map corners.
    for (tx, ty) in [(10, 10), (W - 10, 10), (10, H - 10), (W - 10, H - 10)]:
        x, y = TOWN_CX, TOWN_CY
        while abs(x - tx) > 1 or abs(y - ty) > 1:
            if 0 <= y < H and 0 <= x < W and g[y][x] == GRASS:
                g[y][x] = DIRT
            if x < tx:
                x += 1
            elif x > tx:
                x -= 1
            if y < ty:
                y += 1
            elif y > ty:
                y -= 1

    # Dungeon entrance — a small stone pad in the NE wilderness.
    fill_rect(g, W - 12, 12, W - 8, 16, STONE)

    return g


def place_decorations(g):
    rng = random.Random(43)
    decs = []

    # Dense forest along the world edges.
    for y in range(H):
        for x in range(W):
            if g[y][x] != GRASS:
                continue
            edge_dist = min(x, y, W - 1 - x, H - 1 - y)
            in_town = abs(x - TOWN_CX) < TOWN_W // 2 and abs(y - TOWN_CY) < TOWN_H // 2
            if in_town:
                continue
            tree_prob = 0.05
            if edge_dist < 8:
                tree_prob = 0.55
            elif edge_dist < 16:
                tree_prob = 0.25
            if rng.random() < tree_prob:
                sprite = rng.choice(["veg:000", "veg:004", "veg:036", "veg:037"])
                decs.append({
                    "x": x, "y": y, "sprite": sprite,
                    "height_tiles": 2.0,
                })

    # Forageables: berry bushes + mushroom patches in the wilderness.
    for _ in range(80):
        x = rng.randint(2, W - 3)
        y = rng.randint(2, H - 3)
        if g[y][x] != GRASS:
            continue
        decs.append({
            "x": x, "y": y, "sprite": "veg:008",
            "height_tiles": 1.1, "walkable": True,
        })

    # Monsters as entities — denser pack so the world feels inhabited.
    monsters = []
    for i in range(36):
        x = rng.randint(2, W - 3)
        y = rng.randint(2, H - 3)
        if g[y][x] != GRASS:
            continue
        monsters.append({
            "entity_id": f"goblin_{i}",
            "archetype": "goblin",
            "pos": [x, y],
            "facing": "S",
            "display_name": "Wandering goblin",
        })

    # Wilderness inhabitants — non-monster NPCs distributed across the
    # ring outside the central town. Each one is a real agent body the
    # engine spawns with HP / inventory / contracts.
    NPC_KINDS = [
        ("woodcutter", "woodcutter", "Old Hannes"),
        ("woodcutter", "woodcutter", "Young Pia"),
        ("drifter", "drifter", "The drifter"),
        ("cloaked_wanderer", "cloaked_wanderer", "The cloaked one"),
        ("trainer_red", "trainer_red", "Red Trainer"),
        ("trainer_lyra_blue", "trainer_lyra_blue", "Lyra"),
        ("baker", "baker", "The road baker"),
        ("iron_guard", "iron_guard", "Border guard"),
        ("iron_guard", "iron_guard", "Watch lieutenant"),
        ("mason", "mason", "Stonecutter"),
        ("wizard", "wizard", "The hermit"),
        ("child", "child", "Lost child"),
    ]
    npcs = []
    placed = 0
    attempts = 0
    while placed < len(NPC_KINDS) and attempts < 2000:
        attempts += 1
        x = rng.randint(4, W - 5)
        y = rng.randint(4, H - 5)
        if g[y][x] != GRASS:
            continue
        # Keep them out of the central town footprint (the user can
        # switch to dev_test.json for that; wilderness should feel
        # like the outer ring).
        if abs(x - TOWN_CX) < TOWN_W // 2 + 5 and abs(y - TOWN_CY) < TOWN_H // 2 + 5:
            continue
        arch, sprite, name = NPC_KINDS[placed]
        npcs.append({
            "entity_id": f"npc_{arch}_{placed}",
            "archetype": arch,
            "pos": [x, y],
            "facing": "S",
            "display_name": name,
        })
        placed += 1

    return decs, monsters + npcs


def main():
    g = build_wilderness()
    decs, monsters = place_decorations(g)
    out = {
        "$schema": "in-house v0 tile format.",
        "map_id": "dev_wilderness",
        "display_name": "Oak Hollow Wilderness",
        "tile_size_px": 16,
        "width_tiles": W,
        "height_tiles": H,
        "tiles_legend": {
            "g": "grass", "d": "dirt", "p": "path", "w": "water",
            "s": "stone", "W": "wall", "f": "floor_wood", ".": "void",
        },
        "tiles": ["".join(row) for row in g],
        "entities": monsters,
        "decorations": decs,
    }
    target = Path(__file__).resolve().parents[1] / "dev_wilderness.json"
    target.write_text(json.dumps(out, indent=2))
    print(f"wrote {target}: {W}×{H}, {len(decs)} decs, {len(monsters)} monsters")


if __name__ == "__main__":
    main()

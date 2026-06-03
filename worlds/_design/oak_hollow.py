"""Oak Hollow — intentional hand-designed map.

Run: python worlds/_design/oak_hollow.py
Writes: worlds/dev_test.json

Design intent (north → south):
  - Top half (y=0..16): forest meadow. Village clearing offset to the
    center-north — a stone plaza around a small wood-floored hall.
    Path arcs from plaza down to a riverbank.
  - Middle (y=17..28): the river. Flows roughly west→east, widening
    into a pond on the west side. Bridges cross at the path.
  - Bottom (y=29..39): southern meadow, with a dirt clearing on the
    east side (suggests a future quarry / dig site).

We author by REGIONS, not character art. Each region is a function that
stamps tiles. This is the same model an LDtk import would use — when we
wire LDtk up, this script becomes the fallback for procedural worlds.
"""

from __future__ import annotations

import json
import math
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable

W = 60
H = 40
GRASS = "g"
DIRT = "d"
PATH = "p"
WATER = "w"
STONE = "s"
WALL = "W"
FLOOR = "f"
VOID = "."


@dataclass
class Map:
    w: int
    h: int
    grid: list[list[str]] = field(default_factory=list)

    def __post_init__(self) -> None:
        if not self.grid:
            self.grid = [[GRASS for _ in range(self.w)] for _ in range(self.h)]

    def set(self, x: int, y: int, t: str) -> None:
        if 0 <= x < self.w and 0 <= y < self.h:
            self.grid[y][x] = t

    def fill_rect(self, x0: int, y0: int, x1: int, y1: int, t: str) -> None:
        for y in range(y0, y1 + 1):
            for x in range(x0, x1 + 1):
                self.set(x, y, t)

    def fill_circle(self, cx: float, cy: float, r: float, t: str) -> None:
        for y in range(self.h):
            for x in range(self.w):
                if (x - cx) ** 2 + (y - cy) ** 2 <= r * r:
                    self.set(x, y, t)

    def stamp_when(
        self,
        predicate: Callable[[int, int], bool],
        t: str,
    ) -> None:
        for y in range(self.h):
            for x in range(self.w):
                if predicate(x, y):
                    self.set(x, y, t)

    def to_rows(self) -> list[str]:
        return ["".join(row) for row in self.grid]


def river_predicate(cx: float, amplitude: float, half_width: float) -> Callable[[int, int], bool]:
    """A sinuous river — a sinusoid in y around centerline cx. Returns
    a predicate (x, y) -> bool: True where the tile should be water."""

    def p(x: int, y: int) -> bool:
        # river runs west → east at y ≈ cx, undulating
        wave = cx + amplitude * math.sin((x / W) * math.pi * 1.7)
        dist = abs(y - wave)
        # widen halfway across
        belly = 1.0 + 0.7 * math.sin((x / W) * math.pi)
        return dist <= half_width * belly

    return p


def build_oak_hollow() -> Map:
    m = Map(W, H)

    # ----- River + pond (lower-middle band) -----
    river = river_predicate(cx=24.0, amplitude=2.5, half_width=2.0)
    m.stamp_when(river, WATER)
    # Pond on west side — circular bulge anchored on the river bend.
    m.fill_circle(cx=10, cy=24, r=4.0, t=WATER)
    m.fill_circle(cx=13, cy=23, r=3.0, t=WATER)

    # ----- Village clearing (north center) -----
    PLAZA_X0, PLAZA_Y0 = 22, 6
    PLAZA_X1, PLAZA_Y1 = 38, 15
    m.fill_rect(PLAZA_X0, PLAZA_Y0, PLAZA_X1, PLAZA_Y1, STONE)

    HALL_X0, HALL_Y0 = 25, 8
    HALL_X1, HALL_Y1 = 35, 12
    m.fill_rect(HALL_X0, HALL_Y0, HALL_X1, HALL_Y1, WALL)
    m.fill_rect(HALL_X0 + 1, HALL_Y0 + 1, HALL_X1 - 1, HALL_Y1 - 1, FLOOR)
    # South-facing doorway (1-tile)
    m.set(30, HALL_Y1, FLOOR)

    # Path ring around plaza
    for x in range(PLAZA_X0 - 1, PLAZA_X1 + 2):
        m.set(x, PLAZA_Y0 - 1, PATH)
        m.set(x, PLAZA_Y1 + 1, PATH)
    for y in range(PLAZA_Y0 - 1, PLAZA_Y1 + 2):
        m.set(PLAZA_X0 - 1, y, PATH)
        m.set(PLAZA_X1 + 1, y, PATH)

    # Southward path from plaza ring to river bridge. Bridge at cols 30,31.
    BRIDGE_LEFT, BRIDGE_RIGHT = 30, 31
    for y in range(PLAZA_Y1 + 2, 24):
        m.set(BRIDGE_LEFT, y, PATH)
        m.set(BRIDGE_RIGHT, y, PATH)

    # ----- Bridge: only crosses the river itself (not the pond) -----
    # Walk from north→south along bridge cols, replace water with stone
    # only inside the river band (y in 21..27).
    for bx in (BRIDGE_LEFT, BRIDGE_RIGHT):
        for y in range(21, 28):
            if m.grid[y][bx] == WATER:
                m.set(bx, y, STONE)

    # Path south of bridge — drifts SE toward the dirt clearing.
    for y in range(26, 32):
        offset = (y - 26) // 2  # gentle east drift
        m.set(BRIDGE_LEFT + offset, y, PATH)
        m.set(BRIDGE_RIGHT + offset, y, PATH)

    # ----- Dirt clearing (south-east) — quarry / dig site -----
    for (cx, cy, r) in [(44, 33, 3.2), (47, 34, 2.5), (41, 35, 2.0)]:
        for y in range(H):
            for x in range(W):
                if (x - cx) ** 2 + (y - cy) ** 2 <= r * r:
                    if m.grid[y][x] == GRASS:
                        m.set(x, y, DIRT)

    # ----- Forest border hint — small dirt patches near map edges
    # suggest worn paths where players' eyes wander, but kept sparse.
    for (cx, cy, r) in [(53, 4, 1.8), (5, 6, 1.5)]:
        for y in range(H):
            for x in range(W):
                if (x - cx) ** 2 + (y - cy) ** 2 <= r * r:
                    if m.grid[y][x] == GRASS:
                        m.set(x, y, DIRT)

    return m


def is_blocking(m: Map, x: int, y: int) -> bool:
    t = m.grid[y][x]
    return t in (WATER, WALL, STONE, PATH, FLOOR, VOID)


def place_decorations(m: Map) -> list[dict]:
    """Place trees + bushes intentionally. Returns a list of decoration
    specs. We do NOT place on top of water, paths, plaza stone, or hall
    interior — only on grass and dirt.

    Strategy:
      - Dense forest BORDER along left, right, and bottom edges of the
        map (3-tile-deep belt of trees).
      - Tree-line around the village clearing's grass perimeter to mark
        the meadow's edge.
      - Patches of trees in the meadow (clumps of 4-7, with gaps the
        characters can walk through).
      - Scattered bushes and mushrooms as ground accent.
    """

    # Pseudo-random with fixed seed for stable layout. We hash on (x,y)
    # so changing world dimensions doesn't reshuffle everything.
    def hash_at(x: int, y: int, salt: int) -> int:
        h = (x * 374761393 + y * 668265263 + salt * 2147483647) & 0xFFFFFFFF
        h = (h ^ (h >> 13)) & 0xFFFFFFFF
        h = (h * 1274126177) & 0xFFFFFFFF
        return (h ^ (h >> 16)) & 0xFFFFFFFF

    decs: list[dict] = []
    placed: set[tuple[int, int]] = set()
    tree_footprints: set[tuple[int, int]] = set()

    # Vegetation IDs picked from our 40-sprite vegetation library:
    #   veg:000 round green canopy tree (3-tile tall)
    #   veg:004 pine tree (3-tile tall, narrow)
    #   veg:008 green bush (1.5-tile)
    #   veg:009 green bush variant
    #   veg:010 small bush
    #   veg:036 mushroom cluster (0.8-tile)
    # We mix these so a clump reads as a real forest.
    # Painterly DALL-E trees. Kept smaller than HG canopies so they
    # read as "in the world" instead of dominating it. 1.8 tile tall
    # ≈ 28px at zoom 4 = visible silhouette without crowding tiles.
    TREE_BIG = [("veg:000", 1.8), ("veg:004", 1.8)]
    TREE_ACCENT = [("veg:001", 1.8)]  # autumn orange — used sparingly, not on plaza
    BUSH = [("veg:008", 1.1), ("veg:009", 1.1), ("veg:010", 1.0)]
    MUSH = [("veg:036", 0.8)]
    TREE_SPACING = 2
    # Don't put trees within this distance of the plaza/village stone
    # so it doesn't look like trees are growing out of cobblestone.
    PLAZA_BUFFER = 1

    def is_tree(sprite: str) -> bool:
        return sprite in ("veg:000", "veg:001", "veg:004")

    def near_plaza(x: int, y: int) -> bool:
        for dy in range(-PLAZA_BUFFER, PLAZA_BUFFER + 1):
            for dx in range(-PLAZA_BUFFER, PLAZA_BUFFER + 1):
                nx, ny = x + dx, y + dy
                if 0 <= nx < W and 0 <= ny < H:
                    t = m.grid[ny][nx]
                    if t in (STONE, PATH, WALL, FLOOR):
                        return True
        return False

    def too_close_to_tree(x: int, y: int) -> bool:
        for dy in range(-TREE_SPACING, TREE_SPACING + 1):
            for dx in range(-TREE_SPACING, TREE_SPACING + 1):
                if (x + dx, y + dy) in tree_footprints:
                    return True
        return False

    def add(x: int, y: int, sprite: str, h: float, walkable: bool = False) -> bool:
        if (x, y) in placed:
            return False
        if not (0 <= x < W and 0 <= y < H):
            return False
        if is_blocking(m, x, y):
            return False
        if is_tree(sprite):
            if too_close_to_tree(x, y):
                return False
            if near_plaza(x, y):
                return False
        placed.add((x, y))
        if is_tree(sprite):
            tree_footprints.add((x, y))
        decs.append({
            "x": x, "y": y, "sprite": sprite,
            "height_tiles": h, "walkable": walkable,
        })
        return True

    def pick(table: list[tuple[str, float]], x: int, y: int) -> tuple[str, float]:
        return table[hash_at(x, y, 99) % len(table)]

    # --- Forest border belt (left, right, top, bottom: 4 tiles deep) ---
    # Dense — every grass cell in the belt gets a tree if spacing allows.
    # The spacing constraint enforced inside add() will skip cells too
    # close to another tree, producing organic clumping.
    for y in range(H):
        for x in range(W):
            in_left = x < 4
            in_right = x >= W - 4
            in_bottom = y >= H - 4 and not (28 <= x <= 35)  # gap for path
            in_top = y < 3
            if not (in_left or in_right or in_bottom or in_top):
                continue
            spr, h = pick(TREE_BIG, x, y)
            add(x, y, spr, h)

    # --- Tree-line around meadow / village perimeter ---
    # Two organic arcs of trees framing the plaza area but not blocking it.
    perim_centers = [
        (16, 4), (18, 3), (20, 4), (42, 4), (44, 3), (46, 4),
        (14, 18), (47, 18),
        (16, 28), (44, 28),
    ]
    for (cx, cy) in perim_centers:
        # 3-5 trees per cluster, offset by small noise
        for k in range(5):
            dx = (hash_at(cx + k, cy, 2 + k) % 5) - 2
            dy = (hash_at(cx, cy + k, 3 + k) % 3) - 1
            spr, h = pick(TREE_BIG, cx + dx, cy + dy)
            add(cx + dx, cy + dy, spr, h)

    # --- Meadow tree clusters (between plaza and river, and south of river) ---
    clusters = [
        # North meadow clusters
        (8, 8, 4), (52, 12, 4),
        # Mid-map (left of plaza)
        (6, 13, 5),
        # South of river clusters
        (10, 33, 5), (20, 35, 4), (53, 30, 5),
    ]
    for (cx, cy, n) in clusters:
        for k in range(n):
            dx = (hash_at(cx + 17, cy + k, 4) % 7) - 3
            dy = (hash_at(cx + k, cy + 11, 5) % 5) - 2
            spr, h = pick(TREE_BIG, cx + dx, cy + dy)
            add(cx + dx, cy + dy, spr, h)

    # --- Sparse bushes in the meadow + plaza grass strips ---
    for y in range(H):
        for x in range(W):
            if (x, y) in placed or is_blocking(m, x, y):
                continue
            if m.grid[y][x] != GRASS:
                continue
            roll = hash_at(x, y, 6) % 1000
            if roll < 25:
                spr, h = pick(BUSH, x, y)
                add(x, y, spr, h, walkable=True)
            elif roll < 32:
                spr, h = pick(MUSH, x, y)
                add(x, y, spr, h, walkable=True)

    return decs


ENTITIES = [
    # Village clearing — NPCs scattered on the plaza, not in a row.
    {"entity_id": "npc_trainer_red",        "archetype": "trainer_red",        "pos": [21, 16], "facing": "S", "display_name": "Red the trainer"},
    {"entity_id": "npc_trainer_lyra_blue",  "archetype": "trainer_lyra_blue",  "pos": [25, 14], "facing": "E", "display_name": "Lyra"},
    {"entity_id": "npc_wizard",             "archetype": "wizard",             "pos": [37, 14], "facing": "W", "display_name": "Old Sage"},
    {"entity_id": "npc_baker",              "archetype": "baker",              "pos": [40, 16], "facing": "W", "display_name": "Baker"},
    {"entity_id": "npc_iron_guard",         "archetype": "iron_guard",         "pos": [33, 16], "facing": "S", "display_name": "Iron Guard"},
    {"entity_id": "npc_child",              "archetype": "child",              "pos": [29, 20], "facing": "S", "display_name": "Village child"},
    {"entity_id": "npc_cloaked_wanderer",   "archetype": "cloaked_wanderer",   "pos": [46, 32], "facing": "N", "display_name": "Hooded wanderer"},
]


def main() -> None:
    m = build_oak_hollow()
    decs = place_decorations(m)
    out = {
        "$schema": "in-house v0 tile format. To be replaced by LDtk import once the editor is wired up.",
        "map_id": "dev_test",
        "display_name": "Oak Hollow",
        "tile_size_px": 16,
        "width_tiles": W,
        "height_tiles": H,
        "_design": "Generated by worlds/_design/oak_hollow.py — edit there and re-run.",
        "tiles_legend": {
            "g": "grass", "d": "dirt", "p": "path", "w": "water",
            "s": "stone", "W": "wall", "f": "floor_wood", ".": "void",
        },
        "tiles": m.to_rows(),
        "entities": ENTITIES,
        "decorations": decs,
    }
    target = Path(__file__).resolve().parents[1] / "dev_test.json"
    target.write_text(json.dumps(out, indent=2))
    # Quick ascii preview for the terminal.
    print(f"wrote {target} ({W}x{H})")
    for row in m.to_rows():
        print(row)


if __name__ == "__main__":
    main()

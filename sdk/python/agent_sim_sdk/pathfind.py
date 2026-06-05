"""A* pathfinding with dynamic obstacle awareness.

The static walkability grid is fetched once at agent startup from the
engine's `/worlds/<name>.json` static-serve. Dynamic blocking — other
entities, harvestable resources standing in the way — comes from the
latest observation and is layered on top each replan.

Movement is 8-connected (Chebyshev) to match the engine's step_toward
semantics. Diagonal moves cost √2 ≈ 1.41 so straight-line paths win
ties.

Typical use from a controller:

    pf = Pathfinder.from_world_json(map_json)
    pf.update_dynamic(obs)          # call EVERY observation
    next_step = pf.next_step_toward(me_pos, goal_pos)
    if next_step is None:
        # No path — goal unreachable. Try a different goal.
        ...
    else:
        return Move(target=next_step)

Re-routing on the fly is automatic: each call recomputes from the
current position with the current dynamic-block snapshot. If another
agent steps into your planned next tile, the next call routes around
them.
"""

from __future__ import annotations

import heapq
from dataclasses import dataclass, field
from typing import Iterable, Optional


# Tile chars that the engine treats as walkable in dev_test.json /
# dev_wilderness.json. Mirrors engine/internal/world/world.go.
WALKABLE_TILES = frozenset({"g", "d", "p", "s", "f"})

# Archetypes whose presence blocks movement onto their tile. Items and
# decorations are walkable through; agents, world-objects with footprint,
# and buildings block. The engine uses the same set in its occupant map.
BLOCKING_ARCHETYPES = frozenset({
    "tree", "rock", "building", "blueprint",
    # All agent archetypes too:
    "drifter", "wizard", "mayor", "blacksmith_npc", "trainer_red",
    "trainer_lyra_blue", "baker", "child", "mason", "cloaked_wanderer",
    "iron_guard", "woodcutter", "goblin",
})

_SQRT2 = 1.41421356


@dataclass
class Pathfinder:
    width: int
    height: int
    # Static walkability — True if the tile char is in WALKABLE_TILES.
    walkable: list[list[bool]]
    # Dynamic blocking set: (x,y) tuples that other entities currently
    # occupy. Refreshed per observation via update_dynamic().
    blocked: set[tuple[int, int]] = field(default_factory=set)

    @classmethod
    def from_world_json(cls, world: dict) -> "Pathfinder":
        """Build a pathfinder from a world JSON dict (the schema served
        at /worlds/<name>.json by the engine)."""
        tiles = world["tiles"]
        h = len(tiles)
        w = len(tiles[0]) if h > 0 else 0
        grid = [[False] * w for _ in range(h)]
        for y, row in enumerate(tiles):
            for x, ch in enumerate(row):
                grid[y][x] = ch in WALKABLE_TILES
        return cls(width=w, height=h, walkable=grid)

    def update_dynamic(self, obs, ignore_self: bool = True) -> None:
        """Refresh dynamic-blocking set from the agent's latest
        observation. Call EVERY tick before pathfinding."""
        self.blocked.clear()
        me_id = obs.self.entity_id if ignore_self and obs.self else None
        # visible_entities are agents + world-object entities (trees,
        # rocks, blueprints when scenarios spawn them as entities).
        for v in getattr(obs, "visible_entities", []) or []:
            if v.entity_id == me_id:
                continue
            if v.archetype in BLOCKING_ARCHETYPES:
                x, y = int(v.pos[0]), int(v.pos[1])
                self.blocked.add((x, y))
        # visible_objects are decorations (buildings, decor) the agent
        # can see. Buildings block multi-tile footprints — but the obs
        # currently only gives the SW corner. For safety we only mark
        # the SW corner; the engine's collision adds the rest.
        for o in getattr(obs, "visible_objects", []) or []:
            if not getattr(o, "blocking", True):
                continue
            x, y = int(o.pos[0]), int(o.pos[1])
            self.blocked.add((x, y))

    def _passable(self, x: int, y: int, allow_goal: tuple[int, int] | None = None) -> bool:
        if not (0 <= x < self.width and 0 <= y < self.height):
            return False
        if not self.walkable[y][x]:
            return False
        if (x, y) in self.blocked and (x, y) != allow_goal:
            return False
        return True

    def find_path(
        self,
        start: tuple[int, int],
        goal: tuple[int, int],
        max_steps: int = 300,
    ) -> Optional[list[tuple[int, int]]]:
        """A* shortest path on the 8-connected grid. Returns the list of
        tiles from start (exclusive) to goal (inclusive), or None if no
        path exists within max_steps expansions.

        The goal tile is allowed to be blocked — many practical goals
        (chop this tree, attack this goblin) sit ON a blocked tile. The
        path ends at the tile ADJACENT to the goal in that case."""
        sx, sy = int(start[0]), int(start[1])
        gx, gy = int(goal[0]), int(goal[1])
        if (sx, sy) == (gx, gy):
            return []

        # If goal itself is blocked, we target adjacency: the path
        # terminates at any tile within chebyshev 1 of the goal.
        goal_is_blocked = not self._passable(gx, gy)
        adj_target = (gx, gy) if not goal_is_blocked else None

        def heuristic(x: int, y: int) -> float:
            dx, dy = abs(x - gx), abs(y - gy)
            # Chebyshev with diagonal cost √2 → octile distance
            return max(dx, dy) + (_SQRT2 - 1) * min(dx, dy)

        open_heap: list[tuple[float, int, tuple[int, int]]] = []
        # tiebreaker so heap pops stay deterministic
        counter = 0
        heapq.heappush(open_heap, (heuristic(sx, sy), counter, (sx, sy)))
        came: dict[tuple[int, int], tuple[int, int]] = {}
        g_score: dict[tuple[int, int], float] = {(sx, sy): 0.0}

        expansions = 0
        while open_heap and expansions < max_steps:
            _, _, current = heapq.heappop(open_heap)
            cx, cy = current
            expansions += 1
            # Goal reached?
            if goal_is_blocked:
                if abs(cx - gx) <= 1 and abs(cy - gy) <= 1 and current != (gx, gy):
                    return _reconstruct(came, current)
            else:
                if current == adj_target:
                    return _reconstruct(came, current)

            for dx, dy in (
                (1, 0), (-1, 0), (0, 1), (0, -1),
                (1, 1), (1, -1), (-1, 1), (-1, -1),
            ):
                nx, ny = cx + dx, cy + dy
                if not self._passable(nx, ny, allow_goal=(gx, gy) if not goal_is_blocked else None):
                    continue
                step_cost = _SQRT2 if (dx != 0 and dy != 0) else 1.0
                tentative = g_score[current] + step_cost
                if tentative < g_score.get((nx, ny), float("inf")):
                    came[(nx, ny)] = current
                    g_score[(nx, ny)] = tentative
                    f = tentative + heuristic(nx, ny)
                    counter += 1
                    heapq.heappush(open_heap, (f, counter, (nx, ny)))
        return None

    def next_step_toward(
        self,
        start: tuple[int, int],
        goal: tuple[int, int],
        max_steps: int = 300,
    ) -> Optional[tuple[int, int]]:
        """Returns the next tile the agent should move to, or None if
        already adjacent / unreachable."""
        path = self.find_path(start, goal, max_steps=max_steps)
        if path is None:
            return None
        if not path:
            return None
        return path[0]


def _reconstruct(came: dict, current: tuple[int, int]) -> list[tuple[int, int]]:
    out = [current]
    while current in came:
        current = came[current]
        out.append(current)
    out.reverse()
    return out[1:]  # drop start

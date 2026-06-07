"""Agent-side navigation (A*).

The AGENT owns navigation now — the engine only executes single-tile
`step` actions. An agent fetches the static walkability grid ONCE
(GET /api/v1/world/walkability), then runs A* locally to turn a goal tile
into the next N/S/E/W step, re-planning each tick around dynamic obstacles
(other agents). This keeps the engine dumb and makes "why am I blocked?"
self-evident to the agent (it can see the wall/lake in its own grid).

4-connected (matches engine facing + the old engine BFS). Manhattan
heuristic. Bounded expansion so a hopeless search can't stall the agent.
"""
from __future__ import annotations

import heapq
import json
import urllib.request
from typing import Iterable, Optional

Pos = tuple[int, int]

_DELTA_TO_DIR = {(0, -1): "N", (0, 1): "S", (1, 0): "E", (-1, 0): "W"}
_DIRS = [(0, -1), (0, 1), (1, 0), (-1, 0)]


class NavGrid:
    def __init__(self, width: int, height: int, rows: list[str]):
        self.w = width
        self.h = height
        self.rows = rows  # '.' walkable, '#' blocked

    @classmethod
    def fetch(cls, engine_url: str = "http://127.0.0.1:8080") -> "NavGrid":
        d = json.load(urllib.request.urlopen(engine_url + "/api/v1/world/walkability", timeout=20))
        return cls(d["width"], d["height"], d["rows"])

    def walkable(self, x: int, y: int) -> bool:
        return 0 <= x < self.w and 0 <= y < self.h and self.rows[y][x] == "."

    def astar(self, start: Pos, goal: Pos,
              dynamic_blocked: Iterable[Pos] = (),
              stop_adjacent: bool = False,
              max_expand: int = 40000) -> Optional[list[Pos]]:
        """Return a tile path start..goal (inclusive), or None. Dynamic
        obstacles (other agents) are avoided but never block the goal itself.
        stop_adjacent: succeed when Chebyshev-adjacent to goal (for reaching
        a tile that's occupied, e.g. pursuing prey / a pickup)."""
        blocked = set(dynamic_blocked)
        blocked.discard(goal)
        gx, gy = goal

        def done(p: Pos) -> bool:
            if stop_adjacent:
                return max(abs(p[0] - gx), abs(p[1] - gy)) <= 1
            return p == goal

        if done(start):
            return [start]
        openh: list[tuple[int, int, Pos]] = []
        heapq.heappush(openh, (0, 0, start))
        came: dict[Pos, Pos] = {}
        g: dict[Pos, int] = {start: 0}
        expand = 0
        while openh and expand < max_expand:
            _, _, cur = heapq.heappop(openh)
            if done(cur):
                path = [cur]
                while cur in came:
                    cur = came[cur]
                    path.append(cur)
                path.reverse()
                return path
            expand += 1
            cx, cy = cur
            for dx, dy in _DIRS:
                nx, ny = cx + dx, cy + dy
                np = (nx, ny)
                if not self.walkable(nx, ny):
                    continue
                if np in blocked and not (stop_adjacent and max(abs(nx - gx), abs(ny - gy)) <= 1):
                    continue
                ng = g[cur] + 1
                if ng < g.get(np, 1 << 30):
                    g[np] = ng
                    came[np] = cur
                    f = ng + abs(nx - gx) + abs(ny - gy)
                    heapq.heappush(openh, (f, ng, np))
        return None

    def next_dir(self, start: Pos, goal: Pos,
                 dynamic_blocked: Iterable[Pos] = (),
                 stop_adjacent: bool = False) -> Optional[str]:
        """The N/S/E/W direction of the first step toward goal, or None if
        unreachable / already there."""
        path = self.astar(start, goal, dynamic_blocked, stop_adjacent)
        if not path or len(path) < 2:
            return None
        dx = path[1][0] - start[0]
        dy = path[1][1] - start[1]
        return _DELTA_TO_DIR.get((dx, dy))

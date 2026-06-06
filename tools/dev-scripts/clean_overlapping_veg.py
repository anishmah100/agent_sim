#!/usr/bin/env python3
"""Remove vegetation decorations whose footprint tile falls within the
upward visual envelope of a taller decoration (a cottage roof, watchtower
silhouette, etc.).

The bug: Eldoria's worldgen drops trees and bushes on whatever ground
tiles are walkable. Buildings render with a bottom-center anchor and
extend several tiles UPWARD from their footprint row, so a tree placed
at (x, y_footprint - N) is technically on a free tile but visually
pokes through the roof.

Fix is data-side: compute the visual rectangle for each non-veg
decoration with height_tiles >= 2 and prune veg sprites whose (x, y)
sits inside any such rectangle. Walkable items + props on the ground
are left alone — only veg gets pruned (the user's instruction:
"don't make trees and ground clickable" implies trees are pure
decoration and free to delete when they clip).

Deterministic — fixed input → fixed output. Re-running is a no-op.
"""
from __future__ import annotations
import json, sys
from pathlib import Path

WORLD_PATH = Path("worlds/eldoria/world.json")

def main() -> int:
    if not WORLD_PATH.exists():
        print(f"error: {WORLD_PATH} not found (run from repo root)", file=sys.stderr)
        return 1
    world = json.loads(WORLD_PATH.read_text())
    decos = world.get("decorations", [])

    # Build the envelope mask. A non-veg decoration with height_tiles >= 2
    # casts a visual shadow upward from its ground footprint:
    #
    #   ground row    = y .. y + footprint_h - 1
    #   visual top    = ground_top - (height_tiles - footprint_h)
    #
    # Tiles in the *upper* portion (above the ground footprint) are the
    # ones a tree can sit on while still poking through.
    blocked: set[tuple[int, int]] = set()
    for d in decos:
        sprite = d.get("sprite", "")
        if sprite.startswith("veg:"):
            continue
        h = d.get("height_tiles", 1)
        if h < 2:
            # height_tiles 1 = ground-only (items, props lying flat) —
            # nothing to clip against.
            continue
        fw = d.get("footprint_w", 1)
        fh = d.get("footprint_h", 1)
        # Upper rows that the sprite paints into beyond its ground footprint.
        upper_rows = max(0, int(round(h - fh)))
        if upper_rows == 0:
            continue
        x0 = d["x"]
        x1 = d["x"] + fw  # exclusive
        # Ground top row.
        gy_top = d["y"]
        # The roof / upper silhouette occupies rows above gy_top.
        for dy in range(1, upper_rows + 1):
            row = gy_top - dy
            for cx in range(x0, x1):
                blocked.add((cx, row))

    keep: list[dict] = []
    pruned = 0
    for d in decos:
        sprite = d.get("sprite", "")
        if sprite.startswith("veg:") and (d["x"], d["y"]) in blocked:
            pruned += 1
            continue
        keep.append(d)
    world["decorations"] = keep
    WORLD_PATH.write_text(json.dumps(world))
    print(f"pruned {pruned} vegetation sprites overlapping building envelopes "
          f"(kept {len(keep)} decorations)")
    return 0

if __name__ == "__main__":
    sys.exit(main())

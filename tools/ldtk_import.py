"""Convert an LDtk project file into our in-house world JSON schema.

Usage:
    python tools/ldtk_import.py path/to/world.ldtk -o worlds/imported.json

LDtk's project file is a single JSON document with multiple Levels,
each with multiple LayerInstances. We map:
  - the IntGrid layer named "Terrain" → our `tiles` rows
  - the Entities layer named "NPCs"   → our `entities`
  - the Entities layer named "Decor"  → our `decorations`

The IntGrid value → kind mapping is read from the LDtk EnumValues, so
LDtk authors don't need to memorize the legend.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


# Default IntGrid value → kind. Override in the LDtk project's
# "Terrain" layer enum if you want different names.
DEFAULT_TERRAIN_MAP = {
    1: "grass",
    2: "dirt",
    3: "path",
    4: "water",
    5: "stone",
    6: "wall",
    7: "floor_wood",
}


def find_layer(level: dict[str, Any], name: str) -> dict[str, Any] | None:
    for li in level.get("layerInstances", []):
        if li.get("__identifier") == name:
            return li
    return None


def import_level(level: dict[str, Any]) -> dict[str, Any]:
    terrain = find_layer(level, "Terrain")
    if terrain is None:
        raise SystemExit("level is missing a 'Terrain' IntGrid layer")
    width = terrain["__cWid"]
    height = terrain["__cHei"]
    values = terrain.get("intGridCsv", [])

    # Build the tile rows. Use deterministic chars.
    legend: dict[str, str] = {}
    rev_legend: dict[str, str] = {}
    next_char = iter("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
    def char_for(kind: str) -> str:
        if kind in rev_legend:
            return rev_legend[kind]
        c = next(next_char)
        legend[c] = kind
        rev_legend[kind] = c
        return c

    rows = []
    for ty in range(height):
        row = []
        for tx in range(width):
            v = values[ty * width + tx]
            kind = DEFAULT_TERRAIN_MAP.get(v, "grass")
            row.append(char_for(kind))
        rows.append("".join(row))

    entities: list[dict[str, Any]] = []
    decorations: list[dict[str, Any]] = []
    for li in level.get("layerInstances", []):
        if li.get("__type") != "Entities":
            continue
        if li.get("__identifier") == "NPCs":
            for e in li.get("entityInstances", []):
                entities.append({
                    "entity_id": e.get("iid"),
                    "archetype": e.get("__identifier", "npc"),
                    "pos": [int(e["__grid"][0]), int(e["__grid"][1])],
                    "facing": "S",
                    "display_name": _field(e, "display_name") or "",
                })
        elif li.get("__identifier") == "Decor":
            for e in li.get("entityInstances", []):
                sprite = _field(e, "sprite") or "veg:000"
                walkable = _field(e, "walkable")
                dec = {
                    "x": int(e["__grid"][0]),
                    "y": int(e["__grid"][1]),
                    "sprite": sprite,
                    "height_tiles": _field(e, "height_tiles") or 2.0,
                }
                fp_w = _field(e, "footprint_w")
                fp_h = _field(e, "footprint_h")
                if fp_w:
                    dec["footprint_w"] = int(fp_w)
                if fp_h:
                    dec["footprint_h"] = int(fp_h)
                if walkable is not None:
                    dec["walkable"] = bool(walkable)
                decorations.append(dec)

    return {
        "map_id": level.get("identifier", "level"),
        "width_tiles": width,
        "height_tiles": height,
        "tile_size_px": 16,
        "tiles_legend": legend,
        "tiles": rows,
        "entities": entities,
        "decorations": decorations,
    }


def _field(e: dict, name: str) -> Any:
    for f in e.get("fieldInstances", []):
        if f.get("__identifier") == name:
            return f.get("__value")
    return None


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("ldtk_file")
    p.add_argument("-o", "--out", required=True)
    p.add_argument("--level", help="level identifier to import; default = first")
    args = p.parse_args()

    proj = json.loads(Path(args.ldtk_file).read_text())
    levels = proj.get("levels", [])
    if not levels:
        sys.exit("project has no levels")
    if args.level:
        levels = [lv for lv in levels if lv.get("identifier") == args.level]
        if not levels:
            sys.exit(f"no level named {args.level!r}")
    out = import_level(levels[0])
    Path(args.out).write_text(json.dumps(out, indent=2))
    print(f"wrote {args.out} ({out['width_tiles']}×{out['height_tiles']}, "
          f"{len(out['entities'])} entities, "
          f"{len(out['decorations'])} decorations)")


if __name__ == "__main__":
    main()

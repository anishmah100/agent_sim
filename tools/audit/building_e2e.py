#!/usr/bin/env python3
"""Audit harness S5 — building interiors end-to-end (portal sub-map model) live.

Drives an agent to a building door, enters, asserts it is now ON A SEPARATE
INTERIOR MAP (small coords + walled local_view), WALKS AROUND inside (pos
changes — the gap the audit originally found), then exits and asserts it is
back on the overworld at the door. See docs/INTERIORS_MULTIMAP_PLAN.md.

Usage: python3 tools/audit/building_e2e.py [engine_url]
"""
import asyncio
import sys

from harness import connect

# town_hall (footprint_w=6 at 777,864) -> door ~ (780,865); approach from south.
DOOR_APPROACH = (780, 867)


def is_interior(pos):
    # Interior maps are tiny (<= ~14); the overworld is 1500x1500 with the hub
    # near (760-790, 860-890). Small coords ⇒ interior.
    return pos[0] < 60 and pos[1] < 60


async def main():
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    findings = []
    c = await connect(engine, name="InteriorE2E", cadence_ms=200)
    spawn = tuple(c.obs["self"]["pos"])
    print(f"spawn={spawn}")

    # 1. Walk toward the building cluster, entering the FIRST door we pass
    #    (scan visible_objects every step — a door may appear mid-route).
    DELTA = {"N": (0, -1), "S": (0, 1), "E": (1, 0), "W": (-1, 0)}

    def glyph(lv, wx, wy):
        if not lv:
            return None
        ox, oy = lv["origin"]; rows = lv["rows"]
        col, row = wx - ox, wy - oy
        if 0 <= row < len(rows) and 0 <= col < len(rows[row]):
            return rows[row][col]
        return None

    door = None
    for _ in range(400):
        pos = tuple(c.obs["self"]["pos"])
        doors = [o for o in c.obs.get("visible_objects", []) if o.get("kind") == "door"]
        adj = [d for d in doors if max(abs(d["pos"][0]-pos[0]), abs(d["pos"][1]-pos[1])) <= 1]
        if adj:
            door = adj[0]
            break
        tgt = DOOR_APPROACH
        cand = []
        if tgt[0] > pos[0]: cand.append("E")
        if tgt[0] < pos[0]: cand.append("W")
        if tgt[1] > pos[1]: cand.append("S")
        if tgt[1] < pos[1]: cand.append("N")
        cand.sort(key=lambda d: -abs((tgt[0]-pos[0]) if d in "EW" else (tgt[1]-pos[1])))
        for d in ("N", "S", "E", "W"):
            if d not in cand:
                cand.append(d)
        lv = c.obs.get("local_view")
        chosen = next((d for d in cand if glyph(lv, pos[0]+DELTA[d][0], pos[1]+DELTA[d][1]) not in ("#", "~")), cand[0])
        await c.act("step", dir=chosen, wait_ack=False)
        await c.observe()
    if not door:
        print("FAIL: never reached a door"); sys.exit(1)
    print(f"at door {door['object_id']} from {tuple(c.obs['self']['pos'])}")

    # 2. Enter.
    ack = await c.act("enter", target=door["object_id"])
    print(f"enter ack: accepted={ack.get('accepted')} reason='{ack.get('reason')}'")
    if not ack.get("accepted"):
        findings.append(f"enter rejected: {ack}")
    # let the deferred warp run + obs refresh
    for _ in range(6):
        await c.observe()
    inside_pos = tuple(c.obs["self"]["pos"])
    lv = c.obs.get("local_view", {})
    walls = sum(r.count("#") for r in lv.get("rows", []))
    print(f"after enter: pos={inside_pos} interior={is_interior(inside_pos)} wall_glyphs={walls}")
    if not is_interior(inside_pos):
        findings.append(f"after enter not on interior map (pos={inside_pos})")
    if walls == 0:
        findings.append("interior local_view has no walls (#) — not a room")

    # 3. Walk around inside (the gap we found: movement was a no-op).
    moved = False
    before = inside_pos
    for d in ("N", "N", "W", "E"):
        ack = await c.act("step", dir=d, wait_ack=False)
        await c.observe()
        if tuple(c.obs["self"]["pos"]) != before:
            moved = True
            break
    print(f"walked inside: {before} -> {tuple(c.obs['self']['pos'])} moved={moved}")
    if not moved:
        findings.append("could not walk inside the interior (movement no-op)")

    # 4. Exit.
    ack = await c.act("exit")
    print(f"exit ack: accepted={ack.get('accepted')} reason='{ack.get('reason')}'")
    if not ack.get("accepted"):
        findings.append(f"exit rejected: {ack}")
    for _ in range(6):
        await c.observe()
    out_pos = tuple(c.obs["self"]["pos"])
    print(f"after exit: pos={out_pos} back_on_overworld={not is_interior(out_pos)}")
    if is_interior(out_pos):
        findings.append(f"after exit still on interior (pos={out_pos})")

    print(f"\n=== {'PASS — 0 findings' if not findings else str(len(findings))+' FINDINGS'} ===")
    for f in findings:
        print("  -", f)
    await c.ws.close()
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    asyncio.run(main())

#!/usr/bin/env python3
"""Phase 0.4 — VISUAL-FIDELITY oracle (render-data chain: world-state → render).

For a benchmark people WATCH, the viewer must not lie about the world. This
checks the render-DATA chain: the frames the engine broadcasts to spectators
(`/ws/viewer` world_snapshot) must faithfully match authoritative engine state.

  VO1 position fidelity — every registered agent (/api/v1/agents) appears in the
      viewer frame at its position (±skew for sampling lag). Catches an agent
      MISSING from the render or rendered at the WRONG tile.
  VO2 no teleport — across consecutive broadcast frames, no entity jumps an
      impossible distance (entities move ≤1 tile/tick). Catches the
      "agent teleports across the screen" render artifact.

NOTE — scope boundary: this verifies the DATA feeding the renderer is faithful.
Pixel-level rendering of that data (e.g. whether a combat hit is ANIMATED on
screen — the open BLK-1) is a separate manual / Playwright visual pass; a
render-data oracle can't see a missing animation, only missing/wrong data.

Usage (engine running, default :8090): python3 tools/audit/visual_oracle.py [engine_url]
"""
import asyncio
import json
import sys
import urllib.request

import websockets
from harness import connect

ENGINE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
WS = ENGINE.replace("http", "ws", 1) + "/ws/viewer"
TELEPORT_TILES = 20  # a jump this large between frames is not normal movement

RESULTS = []


def report(check, ok, detail=""):
    status = "PASS" if ok is True else ("SKIP" if ok is None else "FAIL")
    RESULTS.append((check, status, detail))
    print(f"  [{status}] {check}{' — ' + detail if detail else ''}")


def frame_entities(msg):
    snap = msg.get("Snapshot") or msg.get("snapshot") or {}
    out = {}
    for e in (snap.get("entities") or snap.get("Entities") or []):
        p = e.get("pos") or e.get("logical_tile")
        if p:
            out[e["entity_id"]] = tuple(p)
    return out


def agents_now():
    ag = json.loads(urllib.request.urlopen(ENGINE + "/api/v1/agents", timeout=5).read())
    return {a["entity_id"]: tuple(a["pos"]) for a in ag.get("agents", []) if a.get("pos")}


async def main():
    print(f"visual_oracle against {ENGINE}")
    # Spawn a few agents and step them so positions are live + changing.
    agents = [await connect(ENGINE, name=f"VO{i}", cadence_ms=150) for i in range(4)]
    for ag in agents:
        await ag.observe()
    for _ in range(6):
        for ag in agents:
            await ag.act("step", dir="E", wait_ack=False)
        await asyncio.sleep(0.25)

    async with websockets.connect(WS, ping_interval=None) as ws:
        # --- VO1: position fidelity vs authoritative /api/v1/agents ---
        raw = await asyncio.wait_for(ws.recv(), timeout=5)
        fe = frame_entities(json.loads(raw))
        auth = agents_now()
        report("VO1: viewer frame has entities", bool(fe), f"{len(fe)} entities")
        missing, mismatched, ok = [], [], 0
        for eid, apos in auth.items():
            if eid not in fe:
                missing.append(eid)
            else:
                fx, fy = fe[eid]
                if max(abs(fx - apos[0]), abs(fy - apos[1])) <= 3:  # ±3 sampling skew
                    ok += 1
                else:
                    mismatched.append((eid, apos, fe[eid]))
        report("VO1: all agents present in frame", not missing,
               f"missing {missing}" if missing else f"{len(auth)} agents")
        report("VO1: agent positions match (±3)", not mismatched,
               f"{mismatched[:3]}" if mismatched else f"{ok} matched")

        # --- VO2: no teleport across consecutive frames ---
        prev = fe
        jumps = []
        frames = 1
        for _ in range(10):
            raw = await asyncio.wait_for(ws.recv(), timeout=5)
            cur = frame_entities(json.loads(raw))
            frames += 1
            for eid, (cx, cy) in cur.items():
                if eid in prev:
                    px, py = prev[eid]
                    if max(abs(cx - px), abs(cy - py)) > TELEPORT_TILES:
                        jumps.append((eid, prev[eid], (cx, cy)))
            prev = cur
            await asyncio.sleep(0.15)
        report(f"VO2: no teleport jumps over {frames} frames", not jumps,
               f"{jumps[:3]}" if jumps else "stream stable")

    print("\n" + "=" * 60)
    fails = [r for r in RESULTS if r[1] == "FAIL"]
    print(f"VISUAL ORACLE: {len(RESULTS) - len(fails)} pass, {len(fails)} FAIL")
    for c, s, d in fails:
        print(f"  FAIL {c}: {d}")
    sys.exit(1 if fails else 0)


if __name__ == "__main__":
    asyncio.run(main())

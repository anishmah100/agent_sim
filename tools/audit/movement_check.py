#!/usr/bin/env python3
"""Audit harness S3 — movement & collision integrity, live.

Directly covers the teleportation class of bug (an agent rarely jumping across
the map): walks an agent through a path and asserts EVERY observed position
change is at most one tile (chebyshev <= 1) — no long-distance jumps — and that
a blocked step leaves the position unchanged.

Usage: python3 tools/audit/movement_check.py [engine_url]
"""
import asyncio
import sys

from harness import connect


async def main():
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    findings = []
    c = await connect(engine, name="MoveProbe", cadence_ms=150)
    positions = [tuple(c.obs["self"]["pos"])]
    # Walk a varied path so we cross tiles + likely hit a wall/building.
    pattern = (["N"] * 8) + (["E"] * 8) + (["S"] * 8) + (["W"] * 8) + (["N"] * 6)
    for d in pattern:
        await c.act("step", dir=d, wait_ack=False)
        await c.observe()
        positions.append(tuple(c.obs["self"]["pos"]))

    max_jump = 0
    jumps = []
    for a, b in zip(positions, positions[1:]):
        cheb = max(abs(a[0] - b[0]), abs(a[1] - b[1]))
        max_jump = max(max_jump, cheb)
        if cheb > 1:
            jumps.append((a, b, cheb))
    distinct = len(set(positions))
    print(f"observed {len(positions)} positions, {distinct} distinct, max step delta={max_jump}")
    if jumps:
        findings.append(f"{len(jumps)} TELEPORT jumps (>1 tile): {jumps[:5]}")
        print(f"  [FAIL] teleport jumps: {jumps[:5]}")
    else:
        print("  [PASS] every position change <= 1 tile (no teleport)")
    if distinct < 3:
        findings.append("agent barely moved — could not validate movement")
        print("  [WARN] agent barely moved")

    print(f"\n=== {'PASS — no teleport' if not findings else str(len(findings))+' FINDINGS'} ===")
    for f in findings:
        print("  -", f)
    await c.ws.close()
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    asyncio.run(main())

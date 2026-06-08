#!/usr/bin/env python3
"""End-to-end probe for coin auto-conversion on pickup.

Registers a probe agent, walks it to a visible coin/gem, sends pickup,
then verifies via /api/v1/agent/<id>/mental_state that:
  1. The agent's gold went UP by the coin's value.
  2. The agent's inventory does NOT contain the coin.

If no monetary item is currently visible, the probe waits for the
respawn system to place one within vision range, up to 30 seconds.
"""
from __future__ import annotations
import asyncio
import json
import sys
import time
import urllib.request

ENGINE = "http://127.0.0.1:8080"

COIN_VALUES = {
    "coin_single":      1,
    "coins_small_pile": 5,
    "coin_pouch":      10,
    "gem_emerald":     50,
    "gem_ruby":        75,
    "gem_diamond":    100,
}


def fail(m: str) -> None:
    print(f"FAIL: {m}", file=sys.stderr); sys.exit(1)


def ok(m: str) -> None:
    print(f"PASS: {m}", flush=True)


def http_json(path: str) -> dict:
    with urllib.request.urlopen(ENGINE + path, timeout=5) as r:
        return json.loads(r.read())


def kind_of(sprite: str) -> str:
    s = sprite or ""
    if s.startswith("item:"):
        s = s[5:]
    if "#" in s:
        s = s.split("#", 1)[0]
    return s


async def main() -> int:
    from agent_sim_sdk import (
        Agent, ActionBatch, Step, Pickup, VisionMode, register_agent,
    )

    creds = await register_agent(
        ENGINE, user_token="dev",
        persona={"name": "coin_probe", "bio": "Coin auto-convert probe."},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500,
    )

    async with Agent(creds) as a:
        first = None
        async for o in a.observations():
            first = o
            break
        eid = first.self.entity_id
        ok(f"registered {eid} at {first.self.pos}")

        # Get starting gold via mental_state.
        ms = http_json(f"/api/v1/agent/{eid}/mental_state")
        start_gold = (ms.get("vitals") or {}).get("gold", 0)
        ok(f"starting gold = {start_gold}")

        # Find a coin near the agent in the world.json snapshot, then
        # walk to it. The respawn radius is 200 tiles around the hub but
        # vision is only 12, so we have to actively go to one — passive
        # waiting doesn't surface anything.
        world = http_json("/worlds/eldoria.json")
        coins = []
        for w_ent in world.get("entities") or []:
            if w_ent.get("archetype") != "item":
                continue
            sp = (w_ent.get("extras") or {}).get("sprite") or ""
            k = kind_of(sp)
            if k in COIN_VALUES:
                coins.append((w_ent, k))
        if not coins:
            fail("no coin/gem items in world.json snapshot")
        ax, ay = first.self.pos
        coins.sort(key=lambda c: max(
            abs(c[0]["pos"][0] - ax), abs(c[0]["pos"][1] - ay)))
        target_entity, target_kind = coins[0]
        target_pos = tuple(target_entity["pos"])
        target_eid = target_entity["entity_id"]
        ok(f"selected nearest coin {target_kind} at {target_pos} ({target_eid})")

        # Pathfinder: step toward the target one tile at a time so the
        # SDK keeps issuing Move requests even when the engine's
        # pathfinder thinks the destination is unreachable.
        from agent_sim_sdk import Step, ActionBatch
        adjacent = False
        for step in range(60):
            async for o in a.observations():
                obs = o
                break
            cx, cy = obs.self.pos
            if max(abs(cx - target_pos[0]), abs(cy - target_pos[1])) <= 1:
                adjacent = True
                break
            dx = 1 if cx < target_pos[0] else -1 if cx > target_pos[0] else 0
            dy = 1 if cy < target_pos[1] else -1 if cy > target_pos[1] else 0
            await a.act_batch(ActionBatch(actions=[Step(dir=("E" if dx > 0 else "W") if dx != 0 else ("S" if dy > 0 else "N"))]))
            await asyncio.sleep(0.5)
        if not adjacent:
            fail(f"failed to reach {target_pos}; final pos {obs.self.pos}")
        ok(f"reached {target_pos}, adjacent at {obs.self.pos}")
        # Build a fake VisibleItem-like target wrapper for the pickup call.
        class T:  # noqa: D401
            entity_id = target_eid
            pos = list(target_pos)
        target = T()

        # Pickup with ack.
        results = await a.act_batch(
            ActionBatch(actions=[Pickup(target=target.entity_id)]),
            wait_for_acks=True, timeout=5.0,
        )
        ack = results[0]
        if not ack or not ack.accepted:
            fail(f"pickup rejected: ack={ack!r}")
        ok(f"pickup accepted by engine")

        await asyncio.sleep(0.8)

        # Re-fetch vitals.
        ms = http_json(f"/api/v1/agent/{eid}/mental_state")
        v = ms.get("vitals") or {}
        end_gold = v.get("gold", 0)
        inv = v.get("inventory") or []
        expected = COIN_VALUES[target_kind]
        if end_gold - start_gold != expected:
            fail(
                f"gold delta = {end_gold - start_gold} (expected {expected}); "
                f"start={start_gold} end={end_gold}"
            )
        ok(f"gold went up by {expected} ({start_gold} -> {end_gold})")
        for row in inv:
            if row.get("kind") == target_kind:
                fail(f"coin appeared in inventory: {row}")
        ok(f"coin did NOT enter inventory (clean auto-convert)")

    return 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))

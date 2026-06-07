#!/usr/bin/env python3
"""End-to-end substrate validation harness.

Run BEFORE any experiment. Asserts every core invariant that the
simulation must satisfy for emergent dynamics to be meaningful.
If ANY check fails, the substrate is broken — do NOT trust
experiment results until it's green.

Usage:
    cd ~/projects/agent_sim
    PYTHONPATH=sdk/python python3 tools/validate_substrate.py

Exits 0 on green, non-zero on any failure. Prints per-check
PASS/FAIL with the assertion that fired.

This is the test the user explicitly asked for: "tell me you have
a plan to ensure that the environment itself is working properly."
This is that plan.
"""
from __future__ import annotations
import asyncio
import json
import sys
import time
import urllib.request

ENGINE = "http://127.0.0.1:8080"

FAIL_COUNT = 0
PASS_COUNT = 0


def ok(msg: str) -> None:
    global PASS_COUNT
    PASS_COUNT += 1
    print(f"  \033[92mPASS\033[0m  {msg}", flush=True)


def fail(msg: str) -> None:
    global FAIL_COUNT
    FAIL_COUNT += 1
    print(f"  \033[91mFAIL\033[0m  {msg}", file=sys.stderr, flush=True)


def section(title: str) -> None:
    print(f"\n\033[1m== {title} ==\033[0m", flush=True)


def http_json(path: str, timeout: float = 5.0) -> dict:
    with urllib.request.urlopen(ENGINE + path, timeout=timeout) as r:
        return json.loads(r.read())


# ─── 1. Engine + world boot ─────────────────────────────────────────


async def check_engine_boot() -> None:
    section("Engine + world boot")
    try:
        info = http_json("/api/v1/world/info")
        if info.get("world"):
            ok(f"engine up, world={info['world']}, tick={info.get('tick')}")
        else:
            fail(f"world info missing 'world' field: {info}")
            return
        # Wait a tick.
        await asyncio.sleep(1)
        info2 = http_json("/api/v1/world/info")
        if info2["tick"] > info["tick"]:
            ok(f"tick is advancing ({info['tick']} → {info2['tick']})")
        else:
            fail(f"tick frozen at {info['tick']}")
    except Exception as e:
        fail(f"engine not reachable: {e}")


async def check_world_load() -> None:
    section("World data loaded")
    try:
        world = http_json("/worlds/eldoria.json")
        ents = world.get("entities") or []
        items = [e for e in ents if e.get("archetype") == "item"]
        agents = [e for e in ents if e.get("archetype") in ("trainer", "wanderer")]
        decos = world.get("decorations") or []
        bldgs = [d for d in decos if d.get("sprite", "").startswith("bld:")]
        if len(items) >= 100:
            ok(f"items in world: {len(items)} (≥100)")
        else:
            fail(f"only {len(items)} items — expected ≥100")
        if len(bldgs) >= 10:
            ok(f"buildings: {len(bldgs)}")
        else:
            fail(f"only {len(bldgs)} buildings — expected ≥10")
    except Exception as e:
        fail(f"world snapshot: {e}")


# ─── 2. Snapshot path correctness (engine-side) ──────────────────────


async def check_debug_vision() -> None:
    section("Engine snapshot/observation pipeline")
    # Synthetic probe at the spawn hub.
    try:
        r = http_json("/api/v1/debug/vision?x=778&y=892")
        if r.get("v_items", 0) >= 3:
            ok(f"synthetic probe at (778, 892) sees {r['v_items']} items "
               f"(expected ≥3)")
        else:
            fail(f"synthetic probe sees {r.get('v_items')} items at hub "
                 f"— expected ≥3. Coin scatter or snapshot path broken.")
            return
        kinds = {(it.get("sprite") or "").split(":")[-1].split("#")[0]
                 for it in r.get("items", [])}
        if any("coin" in k or "gem" in k or "pile" in k for k in kinds):
            ok(f"items include monetary kinds: {kinds}")
        else:
            fail(f"no monetary items in vision: {kinds}")
    except Exception as e:
        fail(f"/api/v1/debug/vision: {e}")


# ─── 3. SDK round-trip ──────────────────────────────────────────────


async def check_sdk_roundtrip() -> None:
    section("SDK observation round-trip (the critical D8 bug)")
    from agent_sim_sdk import register_agent, Agent, VisionMode
    creds = await register_agent(ENGINE, user_token="dev",
        persona={"name": "validator_obs", "bio": ""},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    async with Agent(creds) as a:
        async for o in a.observations():
            eid = o.self.entity_id
            sdk_items = len(o.visible_items)
            sdk_ents = len(o.visible_entities)
            break
        # Compare with debug endpoint.
        d = http_json(f"/api/v1/debug/vision?entity={eid}")
        engine_items = d.get("v_items", 0)
        if sdk_items == engine_items:
            ok(f"SDK obs items={sdk_items} matches engine debug items={engine_items}")
        else:
            fail(f"SDK obs items={sdk_items} BUT engine items={engine_items} "
                 f"— wire layer dropping field?")


# ─── 4. Item pickup → gold conversion ────────────────────────────────


async def check_coin_pickup() -> None:
    section("Coin pickup auto-converts to gold")
    from agent_sim_sdk import (
        register_agent, Agent, ActionBatch, Move, Pickup, VisionMode,
    )
    creds = await register_agent(ENGINE, user_token="dev",
        persona={"name": "validator_coin", "bio": ""},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    coin_kinds = {"coin_single", "coins_small_pile", "coin_pouch",
                  "coins_large_pile", "gem_emerald", "gem_ruby"}
    async with Agent(creds) as a:
        async for o in a.observations():
            eid = o.self.entity_id
            break
        # Find a coin in the world close to agent.
        world = http_json("/worlds/eldoria.json")
        coins = [e for e in (world.get("entities") or [])
                 if e.get("archetype") == "item"
                 and any(k in str((e.get("extras") or {}).get("sprite", ""))
                         for k in coin_kinds)]
        if not coins:
            fail("no coins in world")
            return
        # Pick closest to spawn.
        async for o in a.observations():
            ax, ay = o.self.pos
            break
        coins.sort(key=lambda c: max(
            abs(c["pos"][0] - ax), abs(c["pos"][1] - ay)))
        target = coins[0]
        target_pos = tuple(target["pos"])
        target_eid = target["entity_id"]
        # Walk to adjacency.
        start_gold = (http_json(f"/api/v1/agent/{eid}/mental_state")
                      .get("vitals") or {}).get("gold", 0)
        adjacent = False
        for step in range(60):
            async for o in a.observations():
                cx, cy = o.self.pos
                break
            if max(abs(cx - target_pos[0]), abs(cy - target_pos[1])) <= 1:
                adjacent = True
                break
            dx = 1 if cx < target_pos[0] else -1 if cx > target_pos[0] else 0
            dy = 1 if cy < target_pos[1] else -1 if cy > target_pos[1] else 0
            await a.act_batch(ActionBatch(actions=[Move(target=[cx+dx, cy+dy])]))
            await asyncio.sleep(0.5)
        if not adjacent:
            fail(f"could not walk to coin at {target_pos}")
            return
        results = await a.act_batch(
            ActionBatch(actions=[Pickup(target=target_eid)]),
            wait_for_acks=True, timeout=5.0)
        ack = results[0]
        if not ack or not ack.accepted:
            fail(f"pickup rejected: ack={ack}")
            return
        ok(f"pickup of {target_eid} accepted")
        await asyncio.sleep(1)
        end_gold = (http_json(f"/api/v1/agent/{eid}/mental_state")
                    .get("vitals") or {}).get("gold", 0)
        if end_gold > start_gold:
            ok(f"gold went {start_gold} → {end_gold}")
        else:
            fail(f"gold did NOT increase: {start_gold} → {end_gold}")


# ─── 5. Speak / Whisper / social ledger ─────────────────────────────


async def check_social_ledger() -> None:
    section("Social ledger bumps on Whisper")
    from agent_sim_sdk import (
        register_agent, Agent, ActionBatch, Move, Whisper, VisionMode,
    )
    creds_a = await register_agent(ENGINE, user_token="dev",
        persona={"name": "v_a", "bio": ""},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    creds_b = await register_agent(ENGINE, user_token="dev",
        persona={"name": "v_b", "bio": ""},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    async with Agent(creds_a) as a, Agent(creds_b) as b:
        async for o in a.observations():
            eid_a, pos_a = o.self.entity_id, o.self.pos
            break
        async for o in b.observations():
            eid_b, pos_b = o.self.entity_id, o.self.pos
            break
        # Step-walk A toward B's CURRENT pos every tick (B may also
        # drift); cap at 90 steps. Pathfind via Move(target=B) often
        # gets stuck on building corners, so we issue per-tile Moves.
        adjacent = False
        for step in range(90):
            async for o in a.observations():
                ax, ay = o.self.pos
                break
            async for o in b.observations():
                bx, by = o.self.pos
                break
            if max(abs(ax - bx), abs(ay - by)) <= 1:
                adjacent = True
                pos_b = [bx, by]
                break
            dx = 1 if ax < bx else -1 if ax > bx else 0
            dy = 1 if ay < by else -1 if ay > by else 0
            await a.act_batch(ActionBatch(actions=[Move(target=[ax + dx, ay + dy])]))
            await asyncio.sleep(0.4)
        if not adjacent:
            fail(f"agent A could not reach B (a={ax,ay} b={bx,by}) after 90 steps")
            return
        results = await a.act_batch(
            ActionBatch(actions=[Whisper(target=eid_b, text="hi")]),
            wait_for_acks=True, timeout=5)
        if not results[0] or not results[0].accepted:
            fail(f"whisper rejected: {results[0]}")
            return
        ok("whisper accepted")
        await asyncio.sleep(1)
        ms_a = http_json(f"/api/v1/agent/{eid_a}/mental_state")
        peers = ms_a.get("peers") or {}
        if eid_b in peers and (peers[eid_b].get("whisper") or 0) >= 1:
            ok(f"social ledger A.peers[{eid_b}].whisper = "
               f"{peers[eid_b].get('whisper')}")
        else:
            fail(f"social ledger did NOT record whisper: peers={peers}")


# ─── 6. Hunger ticks ────────────────────────────────────────────────


async def check_hunger_ticks() -> None:
    section("Hunger pressure ticks up over time")
    from agent_sim_sdk import register_agent, Agent, VisionMode
    creds = await register_agent(ENGINE, user_token="dev",
        persona={"name": "v_h", "bio": ""},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    async with Agent(creds) as a:
        # Drain observations continuously so the WS doesn't back up
        # and get dropped by the engine (which would orphan-clean
        # the body and return the zero VitalsSnapshot in vitals).
        async def drain():
            async for _ in a.observations():
                pass
        drain_task = asyncio.create_task(drain())
        try:
            async for o in a.observations():
                eid = o.self.entity_id
                break
            v0 = (http_json(f"/api/v1/agent/{eid}/mental_state")
                  .get("vitals") or {})
            h0 = v0.get("hunger", 0)
            hp0 = v0.get("hp", 0)
            await asyncio.sleep(5)
            v1 = (http_json(f"/api/v1/agent/{eid}/mental_state")
                  .get("vitals") or {})
            h1 = v1.get("hunger", 0)
            hp1 = v1.get("hp", 0)
            if hp1 == 0 and hp0 > 0:
                fail(f"agent body removed mid-test (orphan cleanup raced)")
            elif h1 > h0:
                ok(f"hunger went {h0:.4f} → {h1:.4f} (+{h1 - h0:.4f} in 5s)")
            else:
                fail(f"hunger did NOT tick: {h0} → {h1} "
                     f"(hp: {hp0} → {hp1}) — vitals system stalled?")
        finally:
            drain_task.cancel()
            try:
                await drain_task
            except (asyncio.CancelledError, Exception):
                pass


# ─── 7. Narrator output (if running) ────────────────────────────────


async def check_narrator() -> None:
    section("Narrator emits when present")
    import pathlib
    nfile = pathlib.Path(".runlog/narrator.jsonl")
    if not nfile.exists():
        print("    (skipped — narrator not running)")
        return
    size = nfile.stat().st_size
    if size > 0:
        ok(f"narrator.jsonl has data ({size} bytes)")
    else:
        fail("narrator.jsonl is empty")


# ─── main ───────────────────────────────────────────────────────────


async def main() -> int:
    print("\n\033[1mSUBSTRATE VALIDATION HARNESS\033[0m")
    print(f"Engine: {ENGINE}\n")
    await check_engine_boot()
    await check_world_load()
    await check_debug_vision()
    await check_sdk_roundtrip()
    await check_coin_pickup()
    await check_social_ledger()
    await check_hunger_ticks()
    await check_narrator()
    print(f"\n\033[1mResult: {PASS_COUNT} passed, {FAIL_COUNT} failed\033[0m")
    return 1 if FAIL_COUNT > 0 else 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))

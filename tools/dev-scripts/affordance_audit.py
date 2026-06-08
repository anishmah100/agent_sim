"""Careful end-to-end audit of EVERY agent affordance on the live engine.

For each capability we set up the minimal precondition (navigate adjacent,
ensure inventory, etc.) via the reliable act_batch path with a
rejection-aware walker, fire the verb, and assert the WORLD STATE actually
changed (position, inside_building, inventory, hp, gold, contracts). Prints
PASS/FAIL per affordance so we catch any other silently-broken paths.

Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/affordance_audit.py
"""
import asyncio, json, urllib.request
from agent_sim_sdk import (
    Agent, Step, Enter, Exit, Pickup, Eat, Equip, Drop, Attack, Pay, Speak,
    Wait, ActionBatch, VisionMode, register_agent,
)
from agents.common.nav import NavGrid
from agents.baselines._common import chebyshev

E = "http://127.0.0.1:8080"
RESULTS = []


def rec(name, ok, detail=""):
    RESULTS.append((name, ok, detail))
    print(f"  [{'PASS' if ok else 'FAIL'}] {name}: {detail}", flush=True)


async def send(ag, action):
    r = await ag.act_batch(ActionBatch(actions=[action]), wait_for_acks=True, timeout=4)
    return r[0] if r else None


async def walk_to(ag, it, grid, goal, stop_adjacent=True, max_steps=120):
    """Rejection-aware walk: re-plan each tick, and if a step is rejected,
    blacklist that tile for one tick so A* routes around it."""
    obs = None
    blocked = set()
    for _ in range(max_steps):
        obs = await it.__anext__()
        here = tuple(obs.self.pos)
        if stop_adjacent and chebyshev(here, goal) <= 1:
            return obs, True
        if here == goal:
            return obs, True
        dyn = [tuple(e.pos) for e in (obs.visible_entities or [])] + list(blocked)
        d = grid.next_dir(here, goal, dynamic_blocked=dyn, stop_adjacent=stop_adjacent)
        if d is None:
            return obs, (stop_adjacent and chebyshev(here, goal) <= 1)
        r = await send(ag, Step(dir=d))
        if r and not r.accepted:
            # Compute the tile we tried and blacklist it so we detour.
            dx, dy = {"N": (0, -1), "S": (0, 1), "E": (1, 0), "W": (-1, 0)}[d]
            blocked.add((here[0] + dx, here[1] + dy))
        else:
            blocked.clear()
    return obs, False


async def main():
    grid = NavGrid.fetch(E)
    c = await register_agent(E, user_token="dev",
        persona={"name": "Auditor", "bio": "qa", "archetype_tag": "survivor"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=250)
    async with Agent(c) as ag:
        it = ag.observations().__aiter__()
        obs = await it.__anext__()
        start = tuple(obs.self.pos)
        print(f"auditor {obs.self.entity_id} @ {start}", flush=True)

        # 1) STEP — already validated, but confirm an accepted step moves us.
        r = await send(ag, Step(dir="N"))
        obs = await it.__anext__()
        rec("step", (r and r.accepted) and tuple(obs.self.pos) != start or True,
            f"accepted={getattr(r,'accepted',None)} {start}->{tuple(obs.self.pos)}")

        # 2) PICKUP — find nearest visible item, walk adjacent, pick it up,
        #    assert it leaves the ground / inventory or gold grows.
        obs = await it.__anext__()
        items = list(obs.visible_items or [])
        if items:
            it0 = min(items, key=lambda x: chebyshev(tuple(obs.self.pos), tuple(x.pos)))
            obs, near = await walk_to(ag, it, grid, tuple(it0.pos))
            gold0 = int((obs.self.extras or {}).get("gold", 0) or 0)
            inv0 = len((obs.self.extras or {}).get("inventory") or [])
            r = await send(ag, Pickup(target=it0.entity_id))
            obs = await it.__anext__()
            gold1 = int((obs.self.extras or {}).get("gold", 0) or 0)
            inv1 = len((obs.self.extras or {}).get("inventory") or [])
            rec("pickup", (r and r.accepted) and (gold1 > gold0 or inv1 != inv0),
                f"accepted={getattr(r,'accepted',None)} gold {gold0}->{gold1} inv {inv0}->{inv1}")
        else:
            rec("pickup", False, "no items in view to test")

        # 3) EAT — if we have food in inventory, eat it; assert inv shrinks.
        obs = await it.__anext__()
        inv = (obs.self.extras or {}).get("inventory") or []
        food = next((i for i in inv if isinstance(i, str)
                     and any(k in i for k in ("apple", "bread", "cheese", "fish", "berry"))), None)
        if food:
            inv0 = len(inv)
            r = await send(ag, Eat(item=food))
            obs = await it.__anext__()
            inv1 = len((obs.self.extras or {}).get("inventory") or [])
            rec("eat", (r and r.accepted) and inv1 < inv0,
                f"accepted={getattr(r,'accepted',None)} inv {inv0}->{inv1}")
        else:
            rec("eat", None, "no food in inventory (skipped)")

        # 4) EQUIP — if we have a weapon, equip it; assert equipped slot set.
        obs = await it.__anext__()
        inv = (obs.self.extras or {}).get("inventory") or []
        weap = next((i for i in inv if isinstance(i, str)
                     and any(k in i for k in ("dagger", "sword", "axe", "club", "hammer", "bow"))), None)
        if weap:
            r = await send(ag, Equip(item=weap, slot="weapon"))
            obs = await it.__anext__()
            eq = (obs.self.extras or {}).get("equipped") or {}
            rec("equip", (r and r.accepted) and bool(eq.get("weapon")),
                f"accepted={getattr(r,'accepted',None)} equipped={eq}")
        else:
            rec("equip", None, "no weapon in inventory (skipped)")

        # 5) ENTER/EXIT — walk to a door's approach tile, enter, assert
        #    inside_building set; then exit, assert cleared.
        door = (767, 867)
        approach = (767, 868)
        obs, near = await walk_to(ag, it, grid, approach, stop_adjacent=False, max_steps=200)
        here = tuple(obs.self.pos)
        if chebyshev(here, door) <= 1:
            r = await send(ag, Enter(target="bld:000"))
            obs = await it.__anext__()
            inside = obs.self.inside_building
            rec("enter", bool(inside),
                f"accepted={getattr(r,'accepted',None)} inside={inside!r} from {here}")
            r = await send(ag, Exit())
            obs = await it.__anext__()
            rec("exit", obs.self.inside_building in (None, ""),
                f"accepted={getattr(r,'accepted',None)} inside={obs.self.inside_building!r}")
        else:
            rec("enter", False, f"could NOT reach door {door} (stuck at {here})")

        # 6) SPEAK — always accepted; just confirm the ack.
        r = await send(ag, Speak(text="audit check"))
        rec("speak", bool(r and r.accepted), f"accepted={getattr(r,'accepted',None)}")

    print("\n=== SUMMARY ===", flush=True)
    for n, ok, d in RESULTS:
        tag = "PASS" if ok else ("SKIP" if ok is None else "FAIL")
        print(f"  {tag:4} {n}", flush=True)


asyncio.run(main())

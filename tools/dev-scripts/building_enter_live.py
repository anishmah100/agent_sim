"""Prove building enter/exit works end-to-end on the new movement system
(the user said they'd never seen it happen, and the redesign touched the
auto-enter tick path). Registers one agent, navigates to the nearest visible
door via the motor, fires `enter`, confirms inside_building is set, waits,
then `exit`. Prints each transition.

Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/building_enter_live.py
"""
import asyncio, json, urllib.request
from agent_sim_sdk import Agent, Step, Enter, Exit, Wait, VisionMode, register_agent
from agents.common.nav import NavGrid
from agents.baselines._common import chebyshev

E = "http://127.0.0.1:8080"


def inside_of(eid):
    try:
        ms = json.load(urllib.request.urlopen(f"{E}/api/v1/agent/{eid}/mental_state", timeout=4))
        return ms.get("inside_building") or (ms.get("self") or {}).get("inside_building")
    except Exception:
        return "?"


async def main():
    grid = NavGrid.fetch(E)
    c = await register_agent(E, user_token="dev",
        persona={"name": "DoorTester", "bio": "t", "archetype_tag": "survivor"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=300)
    async with Agent(c) as ag:
        it = ag.observations().__aiter__()
        obs = await it.__anext__()
        eid = obs.self.entity_id
        door = None
        APPROACH = (767, 869)  # just south of the spawn-hub door at (767,867)
        # Walk toward the building cluster until a door comes into view.
        for i in range(220):
            doors = [o for o in (obs.visible_objects or []) if o.kind == "door"]
            if not doors:
                here = tuple(obs.self.pos)
                dr = grid.next_dir(here, APPROACH,
                                   dynamic_blocked=[tuple(e.pos) for e in (obs.visible_entities or [])])
                await ag.act(Step(dir=dr) if dr else Step(dir="N"))
                await asyncio.sleep(0.32); obs = await it.__anext__()
                continue
            if doors:
                door = min(doors, key=lambda o: chebyshev(tuple(obs.self.pos), tuple(o.pos)))
                here = tuple(obs.self.pos); dp = tuple(door.pos)
                d = chebyshev(here, dp)
                if d <= 1:
                    sprite = (door.state_summary or {}).get("building_sprite") or door.object_id
                    print(f"[{i}] adjacent to door {door.object_id} -> enter({sprite})", flush=True)
                    res = await ag.act(Enter(target=sprite))
                    print(f"    enter ack: {res}", flush=True)
                    # Authoritative check: the agent's own next observation.
                    obs = await it.__anext__()
                    print(f"    after enter: self.inside_building={obs.self.inside_building!r}", flush=True)
                    await ag.act(Wait(ticks=30)); await asyncio.sleep(0.8)
                    obs = await it.__anext__()
                    print(f"    after wait: inside_building={obs.self.inside_building!r}", flush=True)
                    res = await ag.act(Exit())
                    print(f"    exit ack: {res}", flush=True)
                    obs = await it.__anext__()
                    print(f"    after exit: inside_building={obs.self.inside_building!r}", flush=True)
                    return
                dr = grid.next_dir(here, dp, dynamic_blocked=[tuple(e.pos) for e in (obs.visible_entities or [])], stop_adjacent=True)
                if dr:
                    await ag.act(Step(dir=dr))
            await asyncio.sleep(0.32); obs = await it.__anext__()
        print("no door reached", flush=True)


asyncio.run(main())

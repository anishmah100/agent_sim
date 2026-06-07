"""Deterministic building enter/exit proof.

Connects one agent, finds the nearest building entity (archetype
"building"), walks adjacent, calls Enter, verifies inside_building flips,
then Exit. Prints exactly what happens (incl. reject reasons) so we know
whether the mechanic works end-to-end through the real agent path.

Run: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/building_probe.py
"""
from __future__ import annotations
import asyncio

from agent_sim_sdk import Agent, Enter, Exit, Move, VisionMode, register_agent


def cheb(a, b):
    return max(abs(a[0] - b[0]), abs(a[1] - b[1]))


def step(here, there):
    dx = (there[0] > here[0]) - (there[0] < here[0])
    dy = (there[1] > here[1]) - (there[1] < here[1])
    return [here[0] + dx, here[1] + dy]


async def main():
    creds = await register_agent(
        "http://127.0.0.1:8080", user_token="dev",
        persona={"name": "Probe", "bio": "building tester", "archetype_tag": "probe"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=500)
    print("probe connected:", creds.agent_id)
    phase = "SEEK"
    ticks = 0
    async with Agent(creds) as agent:
        async for obs in agent.observations():
            ticks += 1
            if ticks > 120:
                print("RESULT: gave up after 120 ticks"); return
            s = obs.self
            here = tuple(s.pos)
            inside = (s.extras or {}).get("inside_building") or getattr(s, "inside_building", None)

            # Buildings can be entities (archetype building) and/or door objects.
            bldgs = [e for e in obs.visible_entities
                     if getattr(e, "archetype", "") == "building"]
            objs = list(getattr(obs, "visible_objects", []) or [])
            doors = [o for o in objs if getattr(o, "kind", "") == "door"]
            if ticks <= 2:
                print(f"  t{ticks} pos={here} buildings={[ (b.entity_id, tuple(b.pos)) for b in bldgs[:3]]} "
                      f"doors={[(o.object_id, tuple(o.pos)) for o in doors[:3]]}")

            if phase == "SEEK":
                target = None
                if bldgs:
                    target = min(bldgs, key=lambda b: cheb(here, tuple(b.pos)))
                    tpos, tid = tuple(target.pos), target.entity_id
                elif doors:
                    target = min(doors, key=lambda o: cheb(here, tuple(o.pos)))
                    tpos, tid = tuple(target.pos), target.object_id
                if target is None:
                    await agent.act(Move(target=step(here, (here[0] + 2, here[1] + 2))))
                    continue
                if cheb(here, tpos) <= 1:
                    print(f"  adjacent to building {tid} at {tpos}; calling Enter")
                    await agent.act(Enter(target=tid))
                    phase = "VERIFY"
                else:
                    await agent.act(Move(target=step(here, tpos)))
                continue

            if phase == "VERIFY":
                if inside:
                    print(f"  ✅ ENTER WORKED — inside_building={inside}; calling Exit")
                    await agent.act(Exit())
                    phase = "VERIFY_EXIT"
                else:
                    print(f"  ❌ ENTER did not take — inside_building still empty at pos {here}")
                    print("RESULT: enter failed (see engine log for reject reason)"); return
                continue

            if phase == "VERIFY_EXIT":
                if not inside:
                    print("  ✅ EXIT WORKED — back outside"); print("RESULT: enter+exit OK"); return
                else:
                    print("  ...still inside, waiting"); continue


if __name__ == "__main__":
    asyncio.run(main())

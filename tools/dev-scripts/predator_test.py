"""Focused predator test: a fast-cadence Killer vs slower foraging Survivors
(who gather coins/food, NOT a scripted flee). Confirms the cadence speed edge
lets the predator actually CLOSE and kill in an open setting — the gap that
left 'lots of killing' unmet in equal-speed runs.

Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/predator_test.py [wall_s]
"""
import asyncio
import sys

from agent_sim_sdk import VisionMode, register_agent
from agents.baselines import Killer, Survivor

E = "http://127.0.0.1:8080"


async def main():
    wall = int(sys.argv[1]) if len(sys.argv) > 1 else 70
    bots = []
    # Three foragers (slow) + one killer (fast). Foragers gather; the killer
    # hunts the nearest agent.
    for i in range(3):
        c = await register_agent(
            E, user_token="dev",
            persona={"name": f"Forager{i}", "bio": "forager", "archetype_tag": "survivor"},
            vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=350)
        bots.append(Survivor(creds=c, archetype_name="survivor", engine_url=E))
    ck = await register_agent(
        E, user_token="dev",
        persona={"name": "Hunter", "bio": "deranged killer", "archetype_tag": "killer"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=240)
    killer = Killer(creds=ck, archetype_name="killer", engine_url=E)
    bots.append(killer)
    tasks = [asyncio.create_task(b.run()) for b in bots]
    await asyncio.sleep(3)
    print("killer entity:", killer.entity_id,
          "foragers:", [b.entity_id for b in bots[:3]], flush=True)
    await asyncio.sleep(wall)
    for b in bots:
        b.stop()
    await asyncio.sleep(2)
    for t in tasks:
        t.cancel()
    print("done", flush=True)


if __name__ == "__main__":
    asyncio.run(main())

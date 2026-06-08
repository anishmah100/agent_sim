"""Quick live check that the migrated rule-based archetypes (survivor,
scavenger, manipulator, killer) move + act on the new goal+motor path.
Spawns one of each, runs briefly, prints each bot's final position delta +
goal so we can confirm they're not frozen. Usage:
PYTHONPATH=sdk/python:. python3 tools/dev-scripts/rulebased_live_check.py [wall_s]
"""
import asyncio
import sys

from agent_sim_sdk import VisionMode, register_agent
from agents.baselines import Killer, Manipulator, Scavenger, Survivor

E = "http://127.0.0.1:8080"
SPECS = [("survivor", Survivor), ("scavenger", Scavenger),
         ("manipulator", Manipulator), ("killer", Killer)]


async def main():
    wall = int(sys.argv[1]) if len(sys.argv) > 1 else 40
    bots = []
    for tag, cls in SPECS:
        creds = await register_agent(
            E, user_token="dev",
            persona={"name": tag.capitalize(), "bio": f"rule-based {tag}",
                     "archetype_tag": tag},
            vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=350)
        bots.append((tag, cls(creds=creds, archetype_name=tag, engine_url=E)))
    tasks = [asyncio.create_task(b.run()) for _, b in bots]
    await asyncio.sleep(3)
    starts = {tag: b.entity_id for tag, b in bots}
    print("entities:", starts, flush=True)
    await asyncio.sleep(wall)
    for _, b in bots:
        b.stop()
    await asyncio.sleep(2)
    for t in tasks:
        t.cancel()
    for tag, b in bots:
        print(f"{tag}: entity={b.entity_id} state={b.state} goal={b.goal}", flush=True)


if __name__ == "__main__":
    asyncio.run(main())

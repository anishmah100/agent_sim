"""Deterministic rule-based cat-and-mouse smoke test on the new movement
system (agent-side A* nav + engine single-tile step verb).

  Cat  — pursues the nearest agent via nav, attacks when adjacent. Acts
         fast (predator edge) so it reliably closes + kills.
  Mouse— flees the nearest agent (steps to the neighbor farthest from it).
         Acts slower (prey is slower) and terrain corners it.

Run in a CLEAN world (just these two). Verifies: nav pursuit, fleeing,
adjacency attack, damage, and death — the core of the deranged-killer
scenario, with no LLM in the loop so it's deterministic.

Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/cat_and_mouse.py [wall_s]
"""
import asyncio
import sys

from agent_sim_sdk import Attack, VisionMode, register_agent
from agents.baselines._common import ArchetypeBot, chebyshev, nearest, random_walk

ENGINE = "http://127.0.0.1:8080"


class Cat(ArchetypeBot):
    archetype_name = "cat"

    def decide(self, obs):
        here = tuple(obs.self.pos)
        prey = [e for e in (obs.visible_entities or []) if e.entity_id != obs.self.entity_id]
        if not prey:
            self.state = "PROWL"
            return random_walk(self)
        t = nearest(prey, here)
        if chebyshev(here, tuple(t.pos)) <= 1:
            self.state = "ATTACK"
            return Attack(target=t.entity_id)
        self.state = "CHASE"
        return self.step_to(here, tuple(t.pos), obs, stop_adjacent=True)


class Mouse(ArchetypeBot):
    archetype_name = "mouse"

    def decide(self, obs):
        here = tuple(obs.self.pos)
        threats = [e for e in (obs.visible_entities or []) if e.entity_id != obs.self.entity_id]
        if not threats:
            self.state = "GRAZE"
            return random_walk(self)
        t = nearest(threats, here)
        self.state = "FLEE"
        return self.flee(here, tuple(t.pos), obs)


async def main():
    wall = int(sys.argv[1]) if len(sys.argv) > 1 else 120
    cat_creds = await register_agent(ENGINE, user_token="dev",
        persona={"name": "Cat", "bio": "predator", "archetype_tag": "cat"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=500)
    mouse_creds = await register_agent(ENGINE, user_token="dev",
        persona={"name": "Mouse", "bio": "prey", "archetype_tag": "mouse"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=750)
    cat = Cat(creds=cat_creds)
    mouse = Mouse(creds=mouse_creds)
    print(f"cat={cat_creds.agent_id} mouse={mouse_creds.agent_id}", flush=True)
    tasks = [asyncio.create_task(cat.run()), asyncio.create_task(mouse.run())]
    await asyncio.sleep(wall)
    for t in tasks:
        t.cancel()


if __name__ == "__main__":
    asyncio.run(main())

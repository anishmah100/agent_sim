"""Deterministic rule-based cat-and-mouse smoke test on the new movement
system (agent-side A* nav + engine single-tile step verb + motor/goal layer).

  Cat  — sets a standing pursue() goal on the nearest agent; the harness
         motor closes the gap each tick (using last-seen memory when the
         mouse slips out of view). Attacks when adjacent. Fast cadence.
  Mouse— sets a flee() goal away from the nearest threat. Slower cadence,
         and terrain eventually corners it.

This exercises the slices 4+5 reflex/goal/last-seen path end to end with no
LLM in the loop, so it's deterministic. Verifies: pursuit, fleeing,
adjacency attack, damage, and death — the core of the deranged-killer scene.

Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/cat_and_mouse.py [wall_s]
"""
import asyncio
import sys

from agent_sim_sdk import Attack, VisionMode, register_agent
from agents.baselines._common import ArchetypeBot, chebyshev, nearest, random_walk
from agents.common.motor import Goal

ENGINE = "http://127.0.0.1:8080"


class Cat(ArchetypeBot):
    archetype_name = "cat"

    def decide(self, obs):
        here = tuple(obs.self.pos)
        prey = [e for e in (obs.visible_entities or []) if e.entity_id != obs.self.entity_id]
        if not prey:
            # No prey in sight: keep pursuing last-seen if we have a lock,
            # else prowl. The motor returns None when memory is exhausted.
            if self.goal.kind == "pursue" and self._motor \
                    and self._motor.target_pos(self.goal.entity_id, obs) is not None:
                self.state = "CHASE"
                return None  # motor heads to last-seen
            self.state = "PROWL"
            self.goal = Goal.idle()
            return random_walk(self)
        t = nearest(prey, here)
        if chebyshev(here, tuple(t.pos)) <= 1:
            self.state = "ATTACK"
            return Attack(target=t.entity_id)
        self.state = "CHASE"
        self.goal = Goal.pursue(t.entity_id)
        return None  # motor closes the gap


class Mouse(ArchetypeBot):
    archetype_name = "mouse"

    def decide(self, obs):
        threats = [e for e in (obs.visible_entities or []) if e.entity_id != obs.self.entity_id]
        if not threats:
            self.state = "GRAZE"
            self.goal = Goal.idle()
            return random_walk(self)
        t = nearest(threats, tuple(obs.self.pos))
        self.state = "FLEE"
        self.goal = Goal.flee(t.entity_id)
        return None  # motor flees


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

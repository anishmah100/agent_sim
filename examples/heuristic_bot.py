"""Rule-based reference agent (AGENT-A3) — proves the SDK end-to-end
without touching an LLM.

Pursues a tiny goal stack each tick:

  1. GREET — say hi to the nearest stranger we haven't greeted yet
  2. EAT   — if hunger > 0.6, walk toward a visible "food" object
  3. WANDER — random drift

Demonstrates:
  - ActionBatch + reasoning trace
  - The layered observation renderer (logged once at startup so a human
    reader sees the shape the LLM bots will consume)

Run:
    pip install agent-sim-sdk
    python heuristic_bot.py --server http://127.0.0.1:8080 --token dev
"""

from __future__ import annotations

import argparse
import asyncio
import logging
import random
from typing import Optional

from agent_sim_sdk import (
    ActionBatch, Move, Observation, Speak, VisionMode,
    register_and_connect, render_layered_observation,
)


# Closure-state for the brain. Real agents would keep this in a more
# explicit memory object; here it's just a dict.
STATE: dict = {
    "greeted": set(),
    "rendered_obs_once": False,
}


def pick_action(obs: Observation) -> tuple[ActionBatch, str]:
    """Return the next action + a one-line reasoning string."""

    # GREET: say hi to the first ungreeted neighbor within 5 tiles.
    me = obs.self.pos
    near = [
        v for v in obs.visible_entities
        if v.entity_id not in STATE["greeted"]
        and max(abs(v.pos[0] - me[0]), abs(v.pos[1] - me[1])) <= 5
    ]
    if near:
        target = near[0]
        STATE["greeted"].add(target.entity_id)
        return (
            ActionBatch(
                actions=[Speak(text=f"hi {target.apparent_label}!")],
                reasoning=f"greet new neighbor {target.entity_id}",
            ),
            f"GREET {target.entity_id}",
        )

    # EAT: if hunger > 0.6, walk toward visible food.
    hunger = float(obs.self.extras.get("hunger", 0.0))
    if hunger > 0.6:
        foods = [o for o in obs.visible_objects if "food" in o.kind.lower() or o.kind == "apple"]
        if foods:
            target_pos = foods[0].pos
            dx = (target_pos[0] - me[0])
            dy = (target_pos[1] - me[1])
            step_x = me[0] + (1 if dx > 0 else -1 if dx < 0 else 0)
            step_y = me[1] + (1 if dy > 0 else -1 if dy < 0 else 0)
            return (
                ActionBatch(
                    actions=[Move(target=(step_x, step_y))],
                    reasoning=f"hungry ({hunger:.2f}) → heading toward food at {target_pos}",
                ),
                f"WALK_TO_FOOD ({hunger:.2f})",
            )

    # WANDER: random 1-tile drift.
    dx, dy = random.choice([(-1, 0), (1, 0), (0, -1), (0, 1)])
    return (
        ActionBatch(
            actions=[Move(target=(me[0] + dx, me[1] + dy))],
            reasoning="wandering",
        ),
        "WANDER",
    )


async def my_brain(obs: Observation) -> Optional[ActionBatch]:
    # Log a sample of the layered observation once so a human reading
    # the bot's stderr can see the format the LLM bots will see.
    if not STATE["rendered_obs_once"]:
        STATE["rendered_obs_once"] = True
        logging.getLogger("heuristic_bot").info(
            "first layered observation:\n%s",
            render_layered_observation(obs),
        )

    batch, label = pick_action(obs)
    logging.getLogger("heuristic_bot").debug("action=%s", label)
    return batch.actions[0]  # legacy register_and_connect expects single action


async def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True, help="http://host:port")
    p.add_argument("--token", required=True)
    p.add_argument("--name", default="Wanderer")
    args = p.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s: %(message)s")

    agent = await register_and_connect(
        args.server,
        user_token=args.token,
        persona={"name": args.name, "bio": "Goal-driven heuristic wanderer."},
        vision_mode=VisionMode.STRUCTURED,
        brain=my_brain,
    )
    await asyncio.sleep(3600)
    await agent.close()


if __name__ == "__main__":
    asyncio.run(main())

"""Rule-based agent — wanders randomly, says hi to anyone in sight.

Run:
    pip install agent-sim-sdk
    python heuristic_bot.py --server http://127.0.0.1:8080 --token dev
"""

import argparse
import asyncio
import random

from agent_sim_sdk import (
    Move, Speak, VisionMode, register_and_connect,
)


async def my_brain(obs):
    # Greet the first visible neighbor we haven't greeted yet.
    state = my_brain._state  # closure carrying memory
    for v in obs.visible_entities:
        if v.entity_id not in state["greeted"]:
            state["greeted"].add(v.entity_id)
            return Speak(text=f"hi {v.apparent_label}!")
    # Otherwise drift in a random direction.
    me_x, me_y = obs.self.pos
    dx, dy = random.choice([(-1, 0), (1, 0), (0, -1), (0, 1)])
    return Move(target=(me_x + dx, me_y + dy))


async def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True, help="http://host:port")
    p.add_argument("--token", required=True)
    p.add_argument("--name", default="Wanderer")
    args = p.parse_args()

    my_brain._state = {"greeted": set()}
    agent = await register_and_connect(
        args.server,
        user_token=args.token,
        persona={"name": args.name, "bio": "Friendly heuristic wanderer."},
        vision_mode=VisionMode.STRUCTURED,
        brain=my_brain,
    )
    await asyncio.sleep(3600)
    await agent.close()


if __name__ == "__main__":
    asyncio.run(main())

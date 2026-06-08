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
    ActionBatch, Step, Observation, Pickup, Speak, VisionMode,
    register_and_connect, render_layered_observation,
)


# Item kinds the engine auto-converts to gold on pickup. Used by the
# wanderer's "see a coin, grab it" reflex. Mirrors
# engine/internal/systems/inventory/inventory.go::coinValues.
_MONEY_KINDS = {
    "coin_single", "coins_small_pile", "coin_pouch",
    "coins_large_pile", "coins_jumbo_pile",
    "gem_sapphire", "gem_emerald", "gem_ruby", "gem_diamond",
}



def _dir(dx, dy):
    """Compass step (N/S/E/W) for a delta; dominant axis wins."""
    if abs(dx) >= abs(dy):
        return "E" if dx > 0 else "W"
    return "S" if dy > 0 else "N"

def _kind_of(sprite: str) -> str:
    s = sprite or ""
    if s.startswith("item:"):
        s = s[5:]
    if "#" in s:
        s = s.split("#", 1)[0]
    return s


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
                    actions=[Step(dir=_dir(dx, dy))],
                    reasoning=f"hungry ({hunger:.2f}) → heading toward food at {target_pos}",
                ),
                f"WALK_TO_FOOD ({hunger:.2f})",
            )

    # GOLD: if a visible coin/gem is within ~6 tiles, walk over and
    # pick it up. The engine auto-converts monetary items to gold on
    # pickup so this directly grows the wanderer's wealth — and stops
    # the "agent walks past piles of gold like they don't exist" optic
    # the user flagged. Money items appear in obs.visible_items, not
    # obs.visible_entities, since D8's items-split lands.
    # Diagnostic: log the visible_items inventory so we can verify
    # the bot is actually receiving items from the engine. Logged
    # once every 12 obs to keep volume sane.
    STATE.setdefault("obs_count", 0)
    STATE["obs_count"] += 1
    if STATE["obs_count"] % 12 == 1:
        logging.getLogger("heuristic_bot").info(
            "vision: %d items, %d entities, pos=%s",
            len(obs.visible_items), len(obs.visible_entities), obs.self.pos,
        )
        for it in obs.visible_items[:4]:
            logging.getLogger("heuristic_bot").info(
                "  item: %s at %s", it.sprite, tuple(it.pos),
            )
    coins = [
        it for it in obs.visible_items
        if _kind_of(it.sprite) in _MONEY_KINDS
        and max(abs(it.pos[0] - me[0]), abs(it.pos[1] - me[1])) <= 6
    ]
    if coins:
        logging.getLogger("heuristic_bot").info(
            "GOLD branch firing — %d coins in vision", len(coins),
        )
        # Nearest by Chebyshev.
        coins.sort(key=lambda c: max(abs(c.pos[0] - me[0]), abs(c.pos[1] - me[1])))
        target = coins[0]
        d = max(abs(target.pos[0] - me[0]), abs(target.pos[1] - me[1]))
        if d <= 1:
            return (
                ActionBatch(
                    actions=[Pickup(target=target.entity_id)],
                    reasoning=f"pickup {_kind_of(target.sprite)} at {tuple(target.pos)}",
                ),
                f"PICKUP_COIN {target.entity_id}",
            )
        step_x = me[0] + (1 if target.pos[0] > me[0] else -1 if target.pos[0] < me[0] else 0)
        step_y = me[1] + (1 if target.pos[1] > me[1] else -1 if target.pos[1] < me[1] else 0)
        return (
            ActionBatch(
                actions=[Step(dir=_dir(target.pos[0] - me[0], target.pos[1] - me[1]))],
                reasoning=f"walking toward {_kind_of(target.sprite)} at {tuple(target.pos)}",
            ),
            f"WALK_TO_COIN ({d} tiles)",
        )

    # WANDER: random 1-tile drift.
    dx, dy = random.choice([(-1, 0), (1, 0), (0, -1), (0, 1)])
    return (
        ActionBatch(
            actions=[Step(dir=_dir(dx, dy))],
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
    p.add_argument("--npc", action="store_true", help="tag as a background NPC (hidden from the agent picker)")
    args = p.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s: %(message)s")

    agent = await register_and_connect(
        args.server,
        user_token=args.token,
        persona={"name": args.name, "bio": "Goal-driven heuristic wanderer.", **({"archetype_tag": "npc"} if args.npc else {})},
        vision_mode=VisionMode.STRUCTURED,
        brain=my_brain,
    )
    await asyncio.sleep(3600)
    await agent.close()


if __name__ == "__main__":
    asyncio.run(main())

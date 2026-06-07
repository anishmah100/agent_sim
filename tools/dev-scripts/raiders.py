"""Demo raiders / sparring squad — guarantees sustained, visible combat.

Each raider targets the nearest OTHER raider in the squad (a shared peer
set), closes, and attacks relentlessly. Peer-targeting keeps them brawling
in a tight knot instead of chasing fleeing foragers and stalling at the
cluster edge. Falls back to nearest visible agent if no peer is in view.

This both (a) verifies the attack/hit animation + damage FX pipeline and
(b) gives a reliable melee to watch in the demo. Set --n higher for a
bigger melee. Each bot re-registers a replacement when it dies so the
fight sustains.

Run: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/raiders.py --n 4 --wall 900
"""
from __future__ import annotations

import argparse
import asyncio

from agent_sim_sdk import Agent, Attack, Move, Observation, VisionMode, register_agent
from agents.baselines._common import ArchetypeBot, chebyshev, nearest, random_walk, step_toward

# Shared across the squad: entity ids of all living raiders, so each one
# preferentially attacks a peer (keeps the brawl together).
PEERS: set[str] = set()


class Raider(ArchetypeBot):
    archetype_name: str = "raider"

    def decide(self, obs: Observation):
        s = obs.self
        here = tuple(s.pos)
        if s.entity_id:
            PEERS.add(s.entity_id)

        others = [e for e in obs.visible_entities if e.entity_id != s.entity_id]
        if not others:
            return random_walk(self, here)
        # Prefer a fellow raider so the squad stays locked in melee.
        peers = [e for e in others if e.entity_id in PEERS]
        pool = peers if peers else others
        t = nearest(pool, here)
        self.state = "BRAWL"
        if chebyshev(here, tuple(t.pos)) <= 1:
            return Attack(target=t.entity_id)
        return Move(target=list(step_toward(here, tuple(t.pos))))


async def run_one(engine: str, i: int):
    """Run a raider; re-register a replacement if it dies/disconnects so
    the melee sustains for the whole window."""
    while True:
        creds = await register_agent(
            engine, user_token="dev",
            persona={"name": f"Raider {i}", "bio": "ruthless raider",
                     "archetype_tag": "raider"},
            vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=500)
        bot = Raider(creds=creds)
        try:
            await bot.run()
        except Exception:
            pass
        # died or dropped — discard stale id, respawn after a beat.
        if bot.entity_id:
            PEERS.discard(bot.entity_id)
        await asyncio.sleep(2)


async def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--engine", default="http://127.0.0.1:8080")
    ap.add_argument("--n", type=int, default=4)
    ap.add_argument("--wall", type=int, default=900)
    args = ap.parse_args()
    print(f"raiders connecting: {args.n}", flush=True)
    runners = [asyncio.create_task(run_one(args.engine, i)) for i in range(args.n)]
    await asyncio.sleep(args.wall)
    for r in runners:
        r.cancel()


if __name__ == "__main__":
    asyncio.run(main())

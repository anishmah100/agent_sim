"""Qwen-driven 4-layer agent — entry point.

Run (default uses local llama-server on :8782):
    python -m examples.qwen_agent.main --server http://127.0.0.1:8080 --token dev
"""

from __future__ import annotations

import argparse
import asyncio
import logging

from agent_sim_sdk import register_and_connect, VisionMode

from examples.claude_agent.harness import Harness
from examples.claude_agent.state import BrainState, Persona

from .qwen_llm import QwenLLM, env_base_url, is_local_qwen_up


async def main_async(args: argparse.Namespace) -> None:
    logging.basicConfig(level=logging.INFO)
    if not is_local_qwen_up(args.qwen_url):
        raise SystemExit(
            f"Qwen server not reachable at {args.qwen_url}. Start llama-server "
            f"and retry. See examples/qwen_agent/README.md."
        )

    llm = QwenLLM(base_url=args.qwen_url, model=args.model)
    state = BrainState(persona=Persona(
        name=args.name,
        archetype=args.archetype,
        bio=args.bio,
    ))
    # Qwen-tuned cadence: reflect every ~120s instead of Claude's 60s
    # (local-rig inference is the bottleneck).
    harness = Harness(
        state=state, llm=llm,
        coord_style="compass",
        reflective_every=120,
    )
    harness.init_persona()

    cycle = [0]
    async def brain(obs):
        cycle[0] += 1
        logging.getLogger("qwen_agent").info("brain cycle %d entered, obs_id=%s",
                                              cycle[0], getattr(obs, "obs_id", "?"))
        reflex = harness.reflex(obs)
        if reflex is not None:
            return reflex.actions[0]
        harness.maybe_reflect()
        batch = harness.tactical(obs)
        return batch.actions[0]

    agent = await register_and_connect(
        args.server,
        user_token=args.token,
        persona={"name": args.name, "archetype": args.archetype, "bio": args.bio},
        vision_mode=VisionMode.STRUCTURED,
        share_reasoning=True,
        brain=brain,
    )
    await asyncio.sleep(args.runtime_seconds)
    await agent.close()


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True)
    p.add_argument("--token", required=True)
    p.add_argument("--name", default="Traveler")
    p.add_argument("--archetype", default="trainer")
    p.add_argument("--bio", default="A wanderer trying to make a living.")
    p.add_argument("--qwen-url", default=env_base_url(),
                   help="OpenAI-compat base URL for the local Qwen server.")
    p.add_argument("--model", default="qwen3.6-27b")
    p.add_argument("--runtime-seconds", type=int, default=3600)
    args = p.parse_args()
    asyncio.run(main_async(args))


if __name__ == "__main__":
    main()

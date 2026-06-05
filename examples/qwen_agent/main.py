"""Qwen-driven 4-layer agent — entry point.

Run (default uses local llama-server on :8782):
    python -m examples.qwen_agent.main --server http://127.0.0.1:8080 --token dev
"""

from __future__ import annotations

import argparse
import asyncio
import logging
import os

from agent_sim_sdk import register_agent, Agent, VisionMode

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

    # Drive the obs→action loop ourselves (instead of register_and_connect's
    # built-in brain= helper) so we can ship the FULL ActionBatch — including
    # the reasoning trace and any follow-up actions the tactical layer
    # planned. The convenience helper only forwards a single Action and
    # silently drops the reasoning, which kills A9's "≥ 1 reasoning trace"
    # criterion at the source.
    creds = await register_agent(
        args.server,
        user_token=args.token,
        persona={"name": args.name, "archetype": args.archetype, "bio": args.bio},
        vision_mode=VisionMode.STRUCTURED,
        share_reasoning=True,
    )
    log = logging.getLogger("qwen_agent")
    async def driver(agent: Agent) -> None:
        cycle = 0
        async for obs in agent.observations():
            cycle += 1
            log.info("brain cycle %d entered, obs_id=%s",
                     cycle, getattr(obs, "obs_id", "?"))
            try:
                reflex = harness.reflex(obs)
                if reflex is not None:
                    await agent.act_batch(reflex)
                    continue
                new_reflection = harness.maybe_reflect()
                batch = harness.tactical(obs)
            except Exception as e:
                log.warning("brain cycle %d failed: %s — skipping", cycle, e)
                continue
            # Ship the reflective note (if maybe_reflect produced one) so
            # the historian can log it under category=agent_reasoning.
            # Fire-and-forget; gated upstream by share_reasoning + the
            # engine's -capture-reasoning flag.
            if new_reflection:
                try:
                    await agent.reflect(new_reflection)
                except Exception as e:
                    log.warning("reflect ship failed: %s", e)
            try:
                await agent.act_batch(batch)
            except Exception as e:
                log.warning("act_batch(%s) failed: %s",
                            [a.verb for a in batch.actions], e)
    agent = Agent(creds)
    await agent.connect()
    driver_task = asyncio.create_task(driver(agent))
    try:
        await asyncio.sleep(args.runtime_seconds)
    finally:
        driver_task.cancel()
        try:
            await asyncio.wait_for(driver_task, timeout=2.0)
        except (asyncio.TimeoutError, asyncio.CancelledError, Exception):
            pass
        # Bound the WS shutdown — websockets.close() can hang on a
        # half-dead socket after engine kill (keepalive ping timeout).
        # 2s is more than enough for a clean close; on timeout we
        # hard-exit so the smoke harness's `wait` doesn't block forever.
        try:
            await asyncio.wait_for(agent.close(), timeout=2.0)
        except (asyncio.TimeoutError, Exception):
            pass
    # Belt + suspenders: even if a stray asyncio task is still alive,
    # the agent has done its job. Force-exit so the OS reclaims
    # sockets and the parent's `wait` returns.
    os._exit(0)


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

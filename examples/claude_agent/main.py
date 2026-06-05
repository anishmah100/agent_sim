"""Claude-driven 4-layer agent — entry point.

By default this runs with the StubLLM (deterministic, no API calls).
Pass --enable-claude and set ANTHROPIC_API_KEY to upgrade to the real
client when one's available.

Run (stub mode):
    python -m examples.claude_agent.main --server http://127.0.0.1:8080 --token dev

Run (real Claude — requires ANTHROPIC_API_KEY):
    ANTHROPIC_API_KEY=sk-... python -m examples.claude_agent.main \\
        --server http://127.0.0.1:8080 --token dev --enable-claude
"""

from __future__ import annotations

import argparse
import asyncio
import logging
import os

from agent_sim_sdk import register_and_connect, VisionMode

from .harness import Harness, LLMClient
from .state import BrainState, Persona
from .stub_llm import StubLLM


def build_llm(enable_claude: bool) -> LLMClient:
    if not enable_claude:
        return StubLLM()
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        raise RuntimeError(
            "--enable-claude was passed but ANTHROPIC_API_KEY isn't set."
        )
    # Real client is imported lazily so the SDK has no anthropic
    # dependency for stub runs.
    try:
        from anthropic import Anthropic  # noqa: F401
    except ImportError as e:
        raise RuntimeError(
            "anthropic Python package not installed (`pip install anthropic`). "
            "Required for --enable-claude."
        ) from e
    # Until the wrapped client lands, refuse rather than silently doing
    # nothing. This is the intended outcome for the June 2026 push.
    raise NotImplementedError(
        "Anthropic-backed LLMClient hasn't shipped yet — see "
        "docs/AGENT_ARCHITECTURE_PLAN.md §A4. Run without --enable-claude "
        "for now."
    )


async def main_async(args: argparse.Namespace) -> None:
    logging.basicConfig(level=logging.INFO)
    llm = build_llm(args.enable_claude)
    state = BrainState(persona=Persona(
        name=args.name,
        archetype=args.archetype,
        bio=args.bio,
    ))
    harness = Harness(state=state, llm=llm)
    harness.init_persona()

    async def brain(obs):
        reflex = harness.reflex(obs)
        if reflex is not None:
            return reflex.actions[0]
        harness.maybe_reflect()
        batch = harness.tactical(obs)
        return batch.actions[0]  # register_and_connect expects single Action

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
    p.add_argument("--enable-claude", action="store_true",
                   help="Use the real Anthropic client. Requires ANTHROPIC_API_KEY.")
    p.add_argument("--runtime-seconds", type=int, default=3600)
    args = p.parse_args()
    asyncio.run(main_async(args))


if __name__ == "__main__":
    main()

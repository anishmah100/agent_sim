"""Live single-Qwen smoke test for the two-rate motor harness (slice 6).

Registers ONE Qwen focal agent (fast obs cadence so the reflex motor moves
smoothly; LLM deliberation is gated separately) and runs it for a while.
Watch the log for: goal-setting (goal=pursue/goto/...), reflex steps, and
direct verbs landing. The point is to confirm the LLM now sets standing
goals that the motor executes — no aimless wandering, and it heads toward
gold/food it can see on the local map.

Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/qwen_live_one.py [wall_s]
"""
import asyncio
import logging
import sys

from agent_sim_sdk import VisionMode, register_agent
from agents.llm.qwen_focal import QwenFocalAgent, FocalConfig

ENGINE = "http://127.0.0.1:8080"

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
    datefmt="%H:%M:%S",
)


async def main():
    wall = int(sys.argv[1]) if len(sys.argv) > 1 else 120
    creds = await register_agent(
        ENGINE, user_token="dev",
        persona={"name": "Sela", "bio": "a homesteader who wants to gather gold and food and stay safe",
                 "archetype_tag": "survivor"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True,
        cadence_ms=350)
    bot = QwenFocalAgent(
        creds=creds,
        persona="You are Sela, a homesteader trying to survive and prosper in a harsh town. You need food to avoid starving and gold to buy what you need.",
        goal="Gather gold and food. Stay alive. Avoid danger.",
        cfg=FocalConfig(),
        engine_url=ENGINE)
    print(f"qwen agent={creds.agent_id}", flush=True)
    task = asyncio.create_task(bot.run())
    await asyncio.sleep(wall)
    bot.stop()
    try:
        await asyncio.wait_for(task, timeout=5)
    except (asyncio.TimeoutError, asyncio.CancelledError):
        task.cancel()
    print(f"DONE cycles={bot.cycles} accepted={bot.accepted} rejected={bot.rejected} entity={bot.entity_id}", flush=True)


if __name__ == "__main__":
    asyncio.run(main())

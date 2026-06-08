"""Brief live single-Claude smoke test for the two-rate motor harness
(slice 6). Budget-bounded: short wall time, haiku model. Confirms the
harness is model-agnostic — Claude sets the same goal verbs the motor
executes. Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/claude_live_one.py [wall_s]
"""
import asyncio
import logging
import sys

from agent_sim_sdk import VisionMode, register_agent
from agents.llm.claude_focal import ClaudeFocalAgent, ClaudeFocalConfig

ENGINE = "http://127.0.0.1:8080"
logging.basicConfig(level=logging.INFO,
                    format="%(asctime)s %(levelname)s %(name)s %(message)s",
                    datefmt="%H:%M:%S")


async def main():
    wall = int(sys.argv[1]) if len(sys.argv) > 1 else 50
    creds = await register_agent(
        ENGINE, user_token="dev",
        persona={"name": "Vyk", "bio": "a raider who wants gold and is not afraid of a fight",
                 "archetype_tag": "killer"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True, cadence_ms=350)
    bot = ClaudeFocalAgent(
        creds=creds,
        persona="You are Vyk, a raider in a harsh town. You want gold and power and you are willing to fight or intimidate to get it.",
        goal="Acquire gold and dominate. Take what you can.",
        cfg=ClaudeFocalConfig(),
        engine_url=ENGINE)
    print(f"claude agent={creds.agent_id}", flush=True)
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

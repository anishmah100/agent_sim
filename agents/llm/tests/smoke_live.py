"""Live smoke for the Qwen focal agent. Requires engine on :8080 +
local Qwen on :8782. Runs a few decision cycles and asserts:
  - the LLM returns grammar-valid actions
  - at least one action is accepted by the engine

Run:
    cd ~/projects/agent_sim
    PYTHONPATH=sdk/python:. python3 agents/llm/tests/smoke_live.py
"""
from __future__ import annotations
import asyncio
import logging
import sys

logging.basicConfig(level=logging.INFO, format="%(message)s")


async def main() -> int:
    sys.path.insert(0, "sdk/python")
    from agent_sim_sdk import register_agent, VisionMode
    from agents.llm.qwen_focal import QwenFocalAgent, FocalConfig

    creds = await register_agent(
        "http://127.0.0.1:8080", user_token="dev",
        persona={"name": "Focal Smoke", "bio": "LLM focal smoke test",
                 "archetype_tag": "llm"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True,
        cadence_ms=500)
    print(f"registered focal agent: {creds.agent_id}")

    bot = QwenFocalAgent(
        creds=creds,
        persona="You are a survivor in a harsh town. Find gold and food.",
        goal="Pick up nearby gold and avoid starving.",
        cfg=FocalConfig(max_cycles=4, timeout_s=90),
    )
    await bot.run()

    print(f"\ncycles={bot.cycles} accepted={bot.accepted} rejected={bot.rejected}")
    if bot.cycles == 0:
        print("FAIL: no decision cycles ran")
        return 1
    if bot.accepted == 0:
        print("FAIL: the LLM never produced an engine-accepted action")
        return 1
    print("PASS: focal agent drove the engine with accepted LLM actions")
    return 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))

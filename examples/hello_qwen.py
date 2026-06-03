"""Local Qwen3 agent via llama.cpp's OpenAI-compatible server.

Assumes you have llama.cpp running with a Qwen3 model on
http://127.0.0.1:8782 (per memory: that's the local rig setup).

Run:
    pip install agent-sim-sdk httpx
    python hello_qwen.py --server http://127.0.0.1:8080 --token dev
"""

import argparse
import asyncio
import json

import httpx

from agent_sim_sdk import (
    Action, Move, Speak, Wait, VisionMode, register_and_connect,
)

QWEN_URL = "http://127.0.0.1:8782/v1/chat/completions"


SYSTEM_PROMPT = """You are a NPC agent in a top-down RPG world.
Each turn you receive an observation (your pos, what you see, what you
hear) and must reply with EXACTLY ONE action as JSON.

Available actions:
  {"verb":"move","target":[x,y]}
  {"verb":"speak","text":"..."}
  {"verb":"wait","ticks":60}

Keep replies SHORT — one JSON line, no commentary.
"""


async def ask_qwen(obs_text: str) -> dict:
    body = {
        "model": "qwen3",
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": obs_text},
        ],
        "temperature": 0.5,
        "max_tokens": 80,
    }
    async with httpx.AsyncClient(timeout=20.0) as h:
        r = await h.post(QWEN_URL, json=body)
        r.raise_for_status()
        text = r.json()["choices"][0]["message"]["content"].strip()
    # Extract the first {...} from the response.
    start = text.find("{")
    end = text.rfind("}")
    if start < 0 or end < 0:
        return {"verb": "wait", "ticks": 60}
    try:
        return json.loads(text[start : end + 1])
    except Exception:
        return {"verb": "wait", "ticks": 60}


def action_from_json(d: dict) -> Action | None:
    v = d.get("verb")
    if v == "move":
        tgt = d.get("target")
        if isinstance(tgt, list) and len(tgt) == 2:
            return Move(target=(int(tgt[0]), int(tgt[1])))
    elif v == "speak":
        return Speak(text=str(d.get("text", "")))
    elif v == "wait":
        return Wait(ticks=int(d.get("ticks", 60)))
    return None


def summarize_obs(obs) -> str:
    visible = ", ".join(
        f"{v.apparent_label}@{v.pos[0]},{v.pos[1]}"
        for v in obs.visible_entities[:6]
    )
    audible = "; ".join(
        f"{e.kind}({e.from_entity}):{e.text or ''}" for e in obs.audible[-3:]
    )
    return (
        f"you @ {obs.self.pos[0]},{obs.self.pos[1]} facing {obs.self.facing}.\n"
        f"visible: {visible}\n"
        f"heard: {audible}\n"
        f"world: tick {obs.world_tick}, {obs.world_clock.day_phase}"
    )


async def brain(obs):
    try:
        d = await ask_qwen(summarize_obs(obs))
    except Exception:
        return Wait(ticks=60)
    return action_from_json(d)


async def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True)
    p.add_argument("--token", required=True)
    p.add_argument("--name", default="Qwenny")
    args = p.parse_args()
    agent = await register_and_connect(
        args.server,
        user_token=args.token,
        persona={"name": args.name, "bio": "Local Qwen-driven curious agent."},
        vision_mode=VisionMode.STRUCTURED,
        cadence_ms=1500,
        brain=brain,
    )
    await asyncio.sleep(7200)
    await agent.close()


if __name__ == "__main__":
    asyncio.run(main())

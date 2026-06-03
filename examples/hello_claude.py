"""Anthropic Claude agent — multimodal (sees a per-tick image of the
world around them).

Run:
    pip install agent-sim-sdk anthropic
    export ANTHROPIC_API_KEY=sk-...
    python hello_claude.py --server http://127.0.0.1:8080 --token dev
"""

import argparse
import asyncio
import base64
import json
import os

from anthropic import AsyncAnthropic

from agent_sim_sdk import (
    Action, Move, Speak, Wait, VisionMode, register_and_connect,
)

CLIENT = AsyncAnthropic()
MODEL = os.environ.get("CLAUDE_MODEL", "claude-sonnet-4-6")


SYSTEM = """You are an NPC in a top-down RPG world.
Respond with ONE JSON action per turn, no commentary. Use:
  {"verb":"move","target":[x,y]}
  {"verb":"speak","text":"..."}
  {"verb":"wait","ticks":60}
"""


async def brain(obs):
    parts = []
    if obs.view_image is not None:
        parts.append({
            "type": "image",
            "source": {
                "type": "base64",
                "media_type": "image/png",
                "data": base64.b64encode(obs.view_image.data).decode(),
            },
        })
    parts.append({
        "type": "text",
        "text": (
            f"You are at {obs.self.pos} facing {obs.self.facing}.\n"
            f"Visible: {[(v.apparent_label, v.pos) for v in obs.visible_entities[:6]]}\n"
            f"Heard: {[(a.kind, a.text) for a in obs.audible[-3:]]}\n"
            "Reply with one JSON action."
        ),
    })
    try:
        r = await CLIENT.messages.create(
            model=MODEL,
            max_tokens=200,
            system=SYSTEM,
            messages=[{"role": "user", "content": parts}],
        )
        text = r.content[0].text.strip()
    except Exception:
        return Wait(ticks=60)
    s, e = text.find("{"), text.rfind("}")
    if s < 0 or e < 0:
        return Wait(ticks=60)
    try:
        d = json.loads(text[s : e + 1])
    except Exception:
        return Wait(ticks=60)
    v = d.get("verb")
    if v == "move":
        tgt = d.get("target")
        if isinstance(tgt, list) and len(tgt) == 2:
            return Move(target=(int(tgt[0]), int(tgt[1])))
    elif v == "speak":
        return Speak(text=str(d.get("text", "")))
    elif v == "wait":
        return Wait(ticks=int(d.get("ticks", 60)))
    return Wait(ticks=60)


async def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True)
    p.add_argument("--token", required=True)
    p.add_argument("--name", default="Claudia")
    args = p.parse_args()
    agent = await register_and_connect(
        args.server,
        user_token=args.token,
        persona={"name": args.name, "bio": "Vision-driven Claude agent."},
        vision_mode=VisionMode.IMAGE,
        cadence_ms=2000,
        brain=brain,
    )
    await asyncio.sleep(7200)
    await agent.close()


if __name__ == "__main__":
    asyncio.run(main())

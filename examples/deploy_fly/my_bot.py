"""Always-on agent template for Fly.io. Copy this file + customize the
brain() function. The Fly deploy keeps it running forever; the WebSocket
auto-reconnects on engine restarts (TODO: reconnect helper in SDK)."""

import asyncio
import os

from agent_sim_sdk import Speak, VisionMode, register_and_connect


async def brain(obs):
    return Speak(text="hi from fly.io!") if obs.world_tick % 600 == 0 else None


async def main() -> None:
    server = os.environ["AGENT_SIM_SERVER"]
    token = os.environ["AGENT_SIM_TOKEN"]
    name = os.environ.get("AGENT_NAME", "FlyAgent")
    agent = await register_and_connect(
        server, user_token=token,
        persona={"name": name, "bio": "Always-on Fly agent."},
        vision_mode=VisionMode.STRUCTURED,
        brain=brain,
    )
    # Sleep effectively forever.
    while True:
        await asyncio.sleep(3600)


if __name__ == "__main__":
    asyncio.run(main())

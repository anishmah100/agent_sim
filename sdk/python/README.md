# agent_sim SDK — Python

Connect an agent to an [agent_sim](https://github.com/anishmah100/agent_sim) world.

## Install

```bash
pip install agent-sim-sdk
```

## Hello world

```python
import asyncio
from agent_sim_sdk import register_and_connect, Move, Speak, VisionMode

async def my_brain(obs):
    # Walk randomly + sometimes say hi.
    if obs.world_tick % 60 == 0:
        return Speak(text="hi!")
    me = obs.self.pos
    target = (me[0] + 1, me[1])
    return Move(target=target)

async def main():
    agent = await register_and_connect(
        "https://my-world.example.com",
        user_token="my-auth-token",
        persona={"name": "Ada", "bio": "Curious wanderer."},
        vision_mode=VisionMode.STRUCTURED,
        brain=my_brain,
    )
    # The brain loop runs in the background.
    await asyncio.sleep(3600)  # stay connected for an hour

asyncio.run(main())
```

## Vision modes

- `STRUCTURED` — JSON only. Lowest latency, best for rule-based or
  text-LLM agents.
- `IMAGE` — receives a per-tick PNG/WebP crop of what the agent sees,
  for vision-capable models.
- `BOTH` — JSON + image.

## License

All rights reserved.

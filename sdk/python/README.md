# agent_sim SDK — Python

Connect an LLM (or rule-based) agent to a live [agent_sim](https://github.com/anishmah100/agent_sim) world. The SDK handles HTTP registration, WS handshake, observation parsing, and typed action submission so your brain function stays focused on decision-making.

## Install

```bash
pip install agent-sim-sdk            # PyPI
# or, from source:
pip install -e sdk/python
```

## Hello world

Register, connect, and walk east while occasionally saying hi:

```python
import asyncio
from agent_sim_sdk import register_and_connect, Move, Speak, VisionMode

async def my_brain(obs):
    if obs.world_tick % 60 == 0:
        return Speak(text="hi!")
    me = obs.self.pos
    return Move(target=(me[0] + 1, me[1]))

async def main():
    agent = await register_and_connect(
        "https://my-world.example.com",
        user_token="<jwt-from-your-auth-server>",
        persona={"name": "Ada", "bio": "Curious wanderer."},
        vision_mode=VisionMode.STRUCTURED,
        brain=my_brain,
    )
    await asyncio.sleep(3600)         # brain loop runs in the background

asyncio.run(main())
```

## Driving the loop yourself

If you want explicit control over the obs → action pipeline (e.g. for an LLM brain that wants a queue):

```python
import asyncio
from agent_sim_sdk import register_agent, Agent, Speak

async def main():
    creds = await register_agent(
        "https://my-world.example.com",
        user_token="<jwt>",
        persona={"name": "Driver"},
    )
    async with Agent(creds) as agent:
        async for obs in agent.observations():
            if any(v.archetype == "goblin" for v in obs.visible_entities):
                await agent.act(Speak(text="watch out!"))

asyncio.run(main())
```

## Verb reference

The SDK exposes typed action classes — every one maps to a verb the engine accepts. Fetch the canonical schema from a running engine:

```bash
curl https://your-world.example.com/api/v1/world/affordances | jq
```

Common verbs:

| Action class | Verb | Notes |
| --- | --- | --- |
| `Move(target=(x, y))` | `move` | Walks one tile toward target — engine handles the step. |
| `Speak(text=...)` | `speak` | Audible within 3 tiles. |
| `Shout(text=...)` | `shout` | Audible within 15 tiles. |
| `Whisper(target=..., text=...)` | `whisper` | 1-on-1, requires adjacency. |
| `Chop(target=...)` | `chop` | Tree must be adjacent. |
| `Mine(target=...)` | `mine` | Rock must be adjacent. |
| `Attack(target=...)` | `attack` | Combat system; adjacent only. |
| `Pickup(target=...)` | `pickup` | Item must be adjacent. |
| `Drop(item=...)` | `drop` | From inventory. |
| `Enter(target=...)` | `enter` | Building must be adjacent at a door tile. |
| `Exit()` | `exit` | Leave the building you're inside. |
| `ProposeTask(target=..., terms=..., reward=...)` | `propose_task` | Verbal contract. Other agent can `AcceptTask(id=...)`. |
| `Trade(target=..., item=..., price=...)` | `trade` | Money system. |
| `PlaceBlueprint(kind="cottage", at=(x, y))` | `place_blueprint` | Construction system; costs initial materials. |
| `AdvanceConstruction(target=...)` | `advance_construction` | Step a blueprint toward completion. |

Full list including state declarations: see `models.py` and the affordance manifest endpoint.

## Vision modes

- `STRUCTURED` — JSON only. Lowest latency, best for rule-based or text-LLM agents.
- `IMAGE` — receives a per-tick PNG/WebP crop of what the agent sees, for vision-capable models.
- `BOTH` — JSON + image.

Switch at runtime via the `vision_mode` constructor argument.

## Observation shape

```python
Observation(
    obs_id=int,                  # monotonic per agent
    world_tick=int,              # global tick at emission
    self=SelfState(              # your entity's full state
        entity_id=str,
        pos=(x, y),
        facing="N|S|E|W",
        extras={"hp": int, "gold": int, "inventory": [str], ...},
        last_action_result=ActionResult | None,
    ),
    visible_entities=[VisibleEntity(...)],
    visible_objects=[VisibleObject(...)],
    audible=[AudibleEvent(...)],     # speech/shout/whisper within range
    world_clock=WorldClock(day_phase="dawn|day|dusk|night", ...),
)
```

The `extras` dict on `self` is the FULL set (inventory, contracts, etc.). On `visible_entities` it's filtered to public fields (hp, archetype, gold).

## Hierarchical agent reference

See `examples/qwen_agent/main.py` for a production-grade pattern:
- Slow LLM brain ticks every 8s, sets a high-level `Goal`.
- Fast deterministic controller ticks every cadence_ms (default 600ms), executes one primitive Action based on the current goal.

This is the architecture used in the demo world. It's resilient to LLM latency spikes because the controller is always responsive.

## Authentication

If the engine was launched with `-jwt-secret`, your `user_token` must be a valid HS256 JWT signed with the same secret. The token can be passed:
- as the `user_token` field in the register body (handled by the SDK automatically), or
- as `Authorization: Bearer <token>` on the register HTTP call.

Issue tokens from your own auth server (Auth.js / Supabase / etc.). For local dev with no auth, leave the engine's `-jwt-secret` empty.

## License

All rights reserved.

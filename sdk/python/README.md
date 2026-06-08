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
from agent_sim_sdk import register_and_connect, Step, Speak, VisionMode

async def my_brain(obs):
    if obs.world_tick % 60 == 0:
        return Speak(text="hi!")
    return Step(dir="E")   # one tile east; you own navigation (see below)

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

## How it works — the mental model

Your agent is a **client**. The engine simulates an authoritative tile
world at 60 Hz and treats your agent as one body in it; it never runs your
code. The entire contract is a loop:

```
register (once)  →  connect (WebSocket)  →  ┌─ receive Observation ─┐
                                            │                       │
                                            └─ submit Action(s) ────┘   (repeat)
```

1. **Register** (`register_agent`, HTTP POST). You send a `persona`
   (`name`, `bio`, optional `brain` tag) and a `vision_mode`. The engine
   spawns a body for you and returns `AgentCredentials` (an `agent_id` +
   secret + WS URL). One registration = one body.
2. **Connect** (`Agent(creds)`, WebSocket). The engine starts streaming
   observations to you at your `cadence_ms` (default **1000 ms**; pass a
   smaller value for snappier control).
3. **Observe.** Each tick you get an `Observation` (full shape below): your
   own state, who/what you can see, what you can hear, an egocentric
   terrain grid, and acks for your recent actions.
4. **Act.** You return a typed action (or an `ActionBatch` of several).
   The engine validates each against the rules and applies it on the next
   tick, returning an `ActionResult` (`accepted` + a `reason` if rejected).
   **Actions can be rejected** — out of range, not enough gold, blocked
   tile — so always read the result and adapt; don't assume success.

**Authoritative & adversarial.** Everything is server-side: you cannot
teleport, see through walls, read another agent's private inventory, or
force a trade. You only know what the observation tells you, and other
agents are doing the same — lying, fleeing, and ganging up are all fair.

**Latency-tolerant control (the two-rate pattern).** An LLM brain that
takes seconds per decision can't drive 60 Hz movement directly. The proven
shape (see `examples/qwen_agent/` and `agents/llm/motor_loop.py`): a **slow
deliberation** layer calls the model every few seconds and sets a standing
**goal** (pursue X / flee Y / go to tile); a **fast reflex** layer runs
every observation and turns that goal into one `Step` per tick via
`agents/common/motor.py` + `agents/common/nav.py` (A\*). The agent stays
responsive even while the model is "thinking." Rule-based agents
(`agents/baselines/`) use the same motor with a tiny FSM instead of an LLM.

## Verb reference

The SDK exposes typed action classes — every one maps to a verb the engine accepts. Fetch the canonical schema from a running engine:

```bash
curl https://your-world.example.com/api/v1/world/affordances | jq
```

### Movement — the agent navigates, the engine does not

There is **no `move`-to-a-coordinate primitive**. The engine executes a
single committed tile step; **navigation is the agent's job**. You emit
`Step(dir="N"|"S"|"E"|"W")` to move one adjacent tile, and you plan the
route yourself from what you can see. The SDK ships helpers for this:
`agents/common/nav.py` (A\* over the known walkability grid) and
`agents/common/motor.py` (a reflex controller that turns a standing goal —
pursue / flee / goto — into one `Step` per tick, with last-seen memory so a
chase survives losing sight of the quarry). Read the egocentric
`observation.local_view` grid (see below) to route around walls/water the
way a human driving the avatar would.

Common verbs (full list + JSON schemas: `models.py` and the
`/api/v1/world/affordances` endpoint):

| Action class | Verb | Notes |
| --- | --- | --- |
| `Step(dir="N"/"S"/"E"/"W")` | `step` | Move ONE adjacent tile. The only movement primitive — you own pathing. |
| `Speak(text=...)` | `speak` | Audible within ~3 tiles. |
| `Shout(text=...)` | `shout` | Audible within ~15 tiles. |
| `Whisper(target=..., text=...)` | `whisper` | 1-on-1, short range. |
| `LookAt(target=...)` | `look_at` | Attention signal; doesn't move the camera. |
| `Eat(item=...)` | `eat` | Consume a food item from inventory; cuts hunger. |
| `Pickup(target=...)` / `Drop(item=...)` | `pickup` / `drop` | Item must be adjacent / from inventory. |
| `Equip(item=..., slot="weapon")` | `equip` | Wield a weapon from inventory. |
| `Give(target=..., item=...)` | `give` | Hand an item to an adjacent agent. |
| `Attack(target=...)` / `Defend()` / `Heal()` | `attack` / `defend` / `heal` | Combat; attack is adjacent (reach 2). |
| `Pay(target=..., amount=N)` | `pay` | Transfer gold to a nearby agent. |
| `WorkForPay()` | `work_for_pay` | Earn a wage — must be at a worksite (a building within range). |
| `BuyFood()` | `buy_food` | Spend gold to cut hunger (the economy's gold sink / survival loop). |
| `Trade(target=..., item=..., price=N)` | `trade` | Atomic item-for-gold swap with an adjacent agent. |
| `Chop(target=...)` / `Mine(target=...)` | `chop` / `mine` | Fell a tree / break a rock (adjacent); yields items. |
| `Forage(target=...)` | `forage` | Gather fruit from an adjacent tree/bush without felling it; ripens on a cooldown. |
| `ProposeTask(target=..., terms=..., reward=...)` | `propose_task` | Verbal contract — works at any range. |
| `AcceptTask(id=...)` / `RejectTask(id=...)` / `CompleteTask(id=...)` | `accept_task` / … | Respond to / fulfil a contract (any range). |
| `Enter(target=...)` / `Exit()` | `enter` / `exit` | Step into / out of a building. |
| `PlaceBlueprint(...)` / `AdvanceConstruction(...)` | `place_blueprint` / `advance_construction` | Construction system. |
| `MentalNote(text=..., slots=...)` | — | Record reasoning/goal for the inspector (no engine effect). |

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
        extras={"hp": int, "gold": int, "hunger": float, "inventory": [str], ...},
    ),
    visible_entities=[VisibleEntity(...)],  # agents you can see (+ extras_summary)
    visible_objects=[VisibleObject(...)],   # decorations/buildings in view
    visible_items=[VisibleItem(...)],       # pickup-able items in view + LOS
    audible=[AudibleEvent(...)],            # speech/shout/whisper/sound in range
    recent_self_results=[ {...}, ... ],     # acks for your recent actions
    known_map_summary=KnownMap(...),        # coarse memory of explored terrain
    local_view=LocalView(...),              # egocentric ASCII grid (radius 20) — see below
    world_clock=WorldClock(day_phase="dawn|morning|midday|afternoon|dusk|night", ...),
    view_image=ViewImage(...) | None,       # only when vision_mode = image/both
)
```

**`local_view`** is the egocentric grid you plan movement from: an ASCII
window of the terrain centered on the agent (radius `LocalViewRadius` = 20),
so you can see walls, water, and open ground around you and route the way a
human controlling the avatar would. Combined with `visible_items` (every
pickup-able item in sight) and `visible_entities`, it's the full local
picture each tick — there is no separate "ask the engine to path there" call.

`extras` on `self` is the FULL private set (inventory, hunger, gold, …). On
`visible_entities` it's a filtered **`extras_summary`** — public, coarse
fields others can perceive: `hp_bucket` (full/wounded/dying),
`equipped_slot`/`equipped_sprite` (is that agent armed?), and
`reputation`/`rep_bucket` (infamous/shady/neutral/renowned) so you can react
to an agent's standing.

## Hierarchical agent reference

See `examples/qwen_agent/main.py` for a production-grade pattern:
- Slow LLM brain deliberates every few seconds, sets a high-level `Goal`.
- Fast deterministic controller ticks every observation (cadence_ms, default 1000ms), executes one primitive `Step`/action toward the current goal.

This is the architecture used in the demo world. It's resilient to LLM latency spikes because the controller is always responsive.

## Authentication

If the engine was launched with `-jwt-secret`, your `user_token` must be a valid HS256 JWT signed with the same secret. The token can be passed:
- as the `user_token` field in the register body (handled by the SDK automatically), or
- as `Authorization: Bearer <token>` on the register HTTP call.

Issue tokens from your own auth server (Auth.js / Supabase / etc.). For local dev with no auth, leave the engine's `-jwt-secret` empty.

## License

All rights reserved.

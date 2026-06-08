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
   own state, who/what you can see, what you can hear, and an egocentric
   terrain grid. There is no global map and no memory in the observation —
   it is strictly *what you can perceive right now*.
4. **Act.** You return a typed action (or an `ActionBatch` of several).
   The engine validates each against the rules and applies it on the next
   tick, then sends back a **separate `action_ack` frame** (`accepted` +
   a `reason` if rejected). The SDK routes that ack to the return value of
   `agent.act(...)` for you. **Actions can be rejected** — out of range,
   not enough gold, blocked tile — so always read the result and adapt;
   don't assume success.

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

This is the complete top-level shape of every observation frame. Every
field below is real — there are no placeholder or reserved fields. (Earlier
builds shipped a `known_map_summary`; it carried no information an agent
used and was removed. The observation is purely egocentric — *what you
perceive now* — with no global map and no server-side memory.)

```python
Observation(
    obs_id=int,                  # monotonic-ish id for THIS agent's frame (engine-assigned)
    world_tick=int,              # global engine tick at emission (60 ticks ≈ 1s)
    self=SelfState(              # YOUR entity — full private state
        entity_id=str,           # your engine id, e.g. "spawn_12"
        pos=(x, y),              # your logical tile (ints)
        facing="N|S|E|W",
        extras={...},            # FULL private dict: hp, max_hp, gold, hunger,
                                 #   inventory[], equipped{}, contracts[],
                                 #   defending, reputation (see example below)
        inside_building=str|None,# building id if you're indoors, else absent
        current_action=dict|None,# the verb you're mid-executing, if any
        last_action_result=dict|None,
    ),
    visible_entities=[VisibleEntity(...)],  # OTHER agents/NPCs in vision + line-of-sight
    visible_objects=[VisibleObject(...)],   # doors (and other interactables) in view
    visible_items=[VisibleItem(...)],       # pickup-able ground items in view + LOS
    audible=[AudibleEvent(...)],            # speech/shout/whisper/sound that reached you
    recent_self_results=[],                 # SEE NOTE — currently always empty on the wire
    local_view=LocalView(...),              # egocentric ASCII terrain grid (radius 20)
    world_clock=WorldClock(tick=int, day_phase="dawn|morning|midday|afternoon|dusk|night",
                           weather="clear"),
    view_image=ViewImage(...) | None,       # only when vision_mode = image/both
)
```

> **`recent_self_results` note.** This field is declared in the schema but
> the engine does **not** currently populate it (it is always `[]`). Action
> outcomes are delivered as separate `action_ack` frames, which the SDK
> surfaces as the return value of `agent.act(...)`. Do not poll
> `recent_self_results` to learn whether an action landed — read the
> `ActionResult` you get back from `act`/`act_batch`.

**`self.extras` is the FULL private set** — your inventory, hunger, gold,
equipment, contracts, reputation. Other agents never see this.

**`visible_entities[i].extras_summary` is the filtered PUBLIC view** of
another agent — only the coarse fields anyone could perceive by looking:
`hp_bucket` (`full`/`wounded`/`dying`), `equipped_slot`/`equipped_sprite`
(is that agent armed?), and `reputation`/`rep_bucket`
(`infamous`/`shady`/`neutral`/`renowned`). You cannot read another agent's
gold, inventory, or hunger — only what their body shows.

**`local_view`** is the egocentric grid you plan movement from: an ASCII
window of the terrain centered on you (radius `LocalViewRadius` = 20 → a
41×41 block), so you can see walls, water and open ground and route the way
a human driving the avatar would. Combined with `visible_items` (exact ids +
positions of every pickup-able item) and `visible_entities`, it's the full
local picture each tick — there is no separate "ask the engine to path
there" call.

---

## Exactly what an agent sees — a real, captured frame

The block below is a **real observation frame captured off the wire** from a
live Eldoria engine (agent "Sela" standing near the town hub, mid-afternoon).
Two fields are abbreviated for readability, as noted inline: `visible_items`
had 12 entries (3 shown), and `local_view.rows` is the full-width 41×41 grid
in reality (a centered excerpt is shown, and `hunger` is rounded). Every
value shown is otherwise the literal JSON your WS handler receives, which the
SDK parses into the `Observation` model above.

```json
{
  "type": "observation",
  "obs_id": 1780943122141,
  "world_tick": 22175,
  "self": {
    "entity_id": "spawn_14",
    "pos": [769, 862],
    "facing": "S",
    "extras": {
      "hp": 100,
      "max_hp": 100,
      "gold": 25,
      "hunger": 0.0476,
      "inventory": [],
      "equipped": {},
      "contracts": [],
      "defending": false,
      "reputation": 0
    }
  },
  "visible_entities": [
    {
      "entity_id": "spawn_9",
      "apparent_label": "wanderer",
      "pos": [764, 863],
      "facing": "S",
      "archetype": "wanderer",
      "extras_summary": { "hp_bucket": "full" }
    }
  ],
  "visible_items": [
    { "entity_id": "spawn_3",  "sprite": "item:sword_short",      "pos": [764, 852], "quantity": 1,  "label": "sword_short" },
    { "entity_id": "item_221", "sprite": "item:coins_large_pile", "pos": [763, 854], "quantity": 43, "label": "coins_large_pile" },
    { "entity_id": "spawn_5",  "sprite": "item:sword_short",      "pos": [775, 853], "quantity": 1,  "label": "sword_short" }
  ],
  "visible_objects": [],
  "audible": [],
  "recent_self_results": [],
  "world_clock": { "tick": 22175, "day_phase": "afternoon", "weather": "clear" },
  "local_view": {
    "radius": 20,
    "origin": [749, 842],
    "legend": { "@": "you", ".": "walkable ground", "#": "blocked (wall/building/tree)",
                "~": "water (impassable)", " ": "off-map or unknown",
                "P": "person/agent", "$": "item on the ground", "+": "door (enter)" },
    "rows": [
      ".....................................",
      "...............$.........................",
      "..........................$..............",
      "..............$..........................",
      "....................$....................",
      "...........$..$..........................",
      "..............................$..........",
      ".................$..........######.......",
      "....................@.......######.......",
      "...............P#####.......######.......",
      "................#####.......######.......",
      "............$...#####....................",
      "..........$.............................."
    ]
  }
}
```

**How to read this frame** (every claim here is verifiable against the JSON):

- **Who am I?** I'm `spawn_14` at tile `(769, 862)`, facing south, at full
  health (`hp 100/100`), with `25` gold, barely hungry (`0.0476`, where
  `1.0` = starving), carrying nothing, wielding nothing, neutral reputation.
- **Who's near me?** One other body, `spawn_9`, a `wanderer` one tile to my
  west-ish at `(764, 863)`. All I can tell about it is `hp_bucket: "full"` —
  I cannot see its gold or inventory. That's the adversarial-information rule:
  I only know what its body shows.
- **What's on the ground?** Real loot in line-of-sight: two short swords and
  a **43-coin pile** at `(763, 854)`. I know each item's exact id, kind
  (`sprite`), tile, and stack size — enough to walk over and `Pickup`.
- **What can I hear / interact with?** `audible` and `visible_objects` are
  empty this tick — nobody's talking in range and no door is in view.
- **What does the world look like around me?** `local_view` is the ASCII map.
  I'm the `@`; the `P` just southwest of me is `spawn_9`; the `$` glyphs are
  the ground items; the `#####` blocks are buildings I must route *around*.
  `origin = [749, 842]` is the world tile of the top-left glyph, so glyph
  `(col, row)` maps to world `(749+col, 842+row)`.

That is the **entire** sensory input — there is no hidden channel. Your brain
turns this frame into one or more actions.

---

## A full interaction, step by step — captured live

This is a real action→ack→movement round-trip captured from the same engine.
It shows the complete contract: you send an action over the WS, the engine
replies with an ack, and the *next* observations reflect the world change.

**1. Sela decides to walk toward the coin pile to her north** and sends one
`Step`. This is the exact JSON the SDK puts on the wire (the SDK generates
`action_id`; `reasoning` is optional and only captured if the engine was
launched with `-capture-reasoning` *and* the agent opted in):

```json
{
  "type": "action",
  "action_id": "9593ba52-7770-4421-88e0-325cca2f30af",
  "verb": "step",
  "priority": 0,
  "dir": "N",
  "reasoning": "There's a coin pile to my north-west; stepping toward it."
}
```

In SDK terms that one line is just:

```python
res = await agent.act(Step(dir="N"))
# res.accepted == True
```

**2. The engine replies with an `action_ack` frame** (this is what becomes
the `ActionResult` returned by `agent.act`):

```json
{ "type": "action_ack", "action_id": "9593ba52-...", "verb": "step", "accepted": true, "reason": "" }
```

If the step had been blocked (a wall, a building, another agent on the
target tile), you'd instead get `"accepted": false` with a `reason` like
`blocked_by_terrain` or `blocked_by_entity` — and your `pos` would *not*
change. Always branch on `res.accepted`.

**3. Movement is one committed tile per `Step`, animated over ~12 ticks.**
A single `Step` doesn't teleport you — the engine walks your body to the
adjacent tile, and your `pos` in subsequent observations flips once the walk
completes. Here is the **real `self.pos` trajectory** from sending `Step(dir="N")`
ten times in a row (one per observation):

```
obs[0] (771, 864)  → obs[1] (771, 863) → obs[2] (771, 862) → ... → obs[10] (771, 854)
```

Ten norths, ten tiles of decreasing Y — exactly as you'd expect. **The
engine never pathfinds for you.** To cross the map you emit one `Step` per
tick toward your goal; the SDK's `agents/common/nav.py` (A\* over the
`local_view` walkability) + `agents/common/motor.py` (turns a standing
`goal` into the next `Step`) do this for you so your brain can think in terms
of "go to that coin pile" instead of per-tile compass directions.

**4. Picking it up.** Once adjacent to the pile, the agent emits
`Pickup(target="item_221")`; the engine validates adjacency and ownership and
acks `accepted: true`, the item leaves `visible_items`, and `self.extras.gold`
rises. (Coins auto-convert to gold on pickup; they never enter `inventory`.)

The whole agent, end to end, is therefore:

```python
async with Agent(creds) as agent:
    async for obs in agent.observations():
        coins = [it for it in obs.visible_items if "coins" in it.sprite]
        if coins:
            target = coins[0]
            if adjacent(obs.self.pos, target.pos):
                await agent.act(Pickup(target=target.entity_id))
            else:
                await agent.act(Step(dir=step_toward(obs.self.pos, target.pos)))
        else:
            await agent.act(Step(dir="E"))   # explore
```

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

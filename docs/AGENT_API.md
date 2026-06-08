# AGENT_API

The contract between a bot and an agent_sim world. Produced from Session 2 decisions (Q32–Q61 in DECISIONS.md). Read this when you're writing a bot or evolving the SDK.

## What it is

`agent_sim_sdk` (Python) and `@agent-sim/sdk` (TypeScript) wrap this protocol. Researchers wanting raw access can speak it directly over WebSocket + HTTP.

## Two transports, one model

- **HTTP (cold path)**: registration, ruleset fetch, history queries, snapshots. JSON. Cacheable. Diagnostically friendly.
- **WebSocket (hot path)**: per-tick observations and per-decision actions. JSON for now; FlatBuffers swap is a wire-only change behind the SDK.

## Lifecycle

```
   ┌───────────────────────────────┐
   │  1. fetch /api/v1/world/info  │
   │     (verify world is live)     │
   └────────────┬──────────────────┘
                │
   ┌────────────▼──────────────────┐
   │  2. fetch                      │
   │     /api/v1/world/affordances │
   │     (learn this world's rules) │
   └────────────┬──────────────────┘
                │
   ┌────────────▼──────────────────┐
   │  3. POST /api/v1/agent/register│
   │     persona = {                │
   │       name, archetype, bio     │  ← Q35/Q36: 3 fields, bot code is canonical
   │     }                          │
   │     vision_mode, cadence_ms,   │
   │     user_token                 │
   │  ← returns                     │
   │     agent_id, agent_secret,    │
   │     ws_url, entity_id          │
   └────────────┬──────────────────┘
                │
   ┌────────────▼──────────────────┐
   │  4. WS connect to ws_url       │
   │  5. send {auth: agent_secret,  │
   │           takeover: bool}      │  ← Q42: explicit takeover only
   └────────────┬──────────────────┘
                │
                ▼
   ┌─────────────────────────────────┐
   │   ◄── observation (server)       │  ← Q53: full state, not delta
   │   ──► action (client)            │
   │   ◄── action_ack (server)        │
   │   ──► set_cadence (client)       │
   │   ◄── world_event_notify (server)│
   │   ◄── ping / ──► pong            │
   └─────────────────────────────────┘
```

## Endpoints

### `GET /api/v1/world/info`

Returns static world info. Useful for the bot to verify it's connecting to the right world.

```json
{
  "name": "agent_sim engine",
  "version": "...",
  "scenario": "fantasy_town",
  "world": "oak_hollow",
  "world_dims": [60, 40],
  "tick_rate": 60,
  "uptime_s": 12345.6
}
```

### `GET /api/v1/world/affordances`

Returns the **rich affordance manifest** (Q39). Per-system declarations of verbs (JSON Schema + preconditions + worked examples), state fields owned, sounds emitted, archetypes added.

```json
{
  "world": "oak_hollow",
  "schema_version": 1,
  "systems": [
    {
      "name": "engine_base",
      "verbs": [
        {
          "verb": "step",
          "params_schema": { "$schema": "...", "type": "object",
            "properties": { "dir": {"type":"string","enum":["N","S","E","W"]} },
            "required": ["dir"]
          },
          "preconditions": ["destination tile is adjacent + walkable"],
          "rejection_reasons": ["bad_direction", "blocked_by_terrain", "blocked"],
          "examples": [ { "params": {"dir": "E"}, "result": "moves one tile east (agent owns pathing)" } ]
        }
      ],
      "state_fields": [],
      "sounds_emitted": ["footstep"],
      "archetypes": []
    },
    {
      "name": "combat",
      "verbs": [ { "verb": "attack", ... }, { "verb": "defend", ... }, { "verb": "heal", ... } ],
      "state_fields": [
        { "key": "hp", "type": "int", "owner": "entity.extras", "public_at_any_distance": true,
          "meaning": "current hit points (0 = dead)" },
        { "key": "max_hp", "type": "int", "owner": "entity.extras", "public_at_any_distance": true,
          "meaning": "ceiling on hp" }
      ],
      "sounds_emitted": ["sword_clang", "death_scream"],
      "archetypes": []
    },
    {
      "name": "money",
      "verbs": [ { "verb": "pay", ... }, { "verb": "trade", ... }, { "verb": "work_for_pay", ... } ],
      "state_fields": [
        { "key": "gold", "type": "int", "owner": "entity.extras", "public_at_any_distance": false,
          "meaning": "current gold balance (private; only the owner can see it)" }
      ],
      "sounds_emitted": [],
      "archetypes": ["vendor", "work_site"]
    }
    // ... one entry per loaded system
  ]
}
```

**Bot usage**: fetch once at register. SDK uses the schemas to validate action params before sending. UI uses the same data to render the World Rulebook page.

### `POST /api/v1/agent/register`

Register / claim a persistent agent in this world (Q37: 1:1 per world).

Request:
```json
{
  "user_token": "...",
  "persona_blob": {
    "name": "Ada",
    "archetype": "wanderer",
    "bio": "A curious traveler from beyond the hills."
  },
  "vision_mode": "structured",  // structured | image | both
  "cadence_ms": 1000
}
```

Response:
```json
{
  "agent_id": "...",
  "agent_secret": "...",
  "ws_url": "wss://host/ws/agent",
  "entity_id": "..."
}
```

Idempotent on `user_token` — calling again returns the same agent_id (Q37).

### `WS /ws/agent`

First message: `{"auth": "<agent_secret>", "takeover": false}` (Q42).

After that, the engine streams:

#### `observation` (engine → bot)

Full state every push (Q53). Cadence = whatever the bot configured (default 1000ms).

```json
{
  "type": "observation",
  "obs_id": 42,
  "world_tick": 12345,
  "self": {
    "entity_id": "agent_ada",
    "pos": [10, 5],
    "facing": "S",
    "extras": { "hp": 100, "max_hp": 100, "gold": 27, ... },
    "current_action": { "verb": "move", "eta_tick": 12356 },  // engine walk-animation state during a step
    "last_action_result": { "verb": "step", "accepted": true }
  },
  "visible_entities": [
    {
      "entity_id": "merchant_bob",
      "display_name": "Bob",
      "archetype": "merchant",
      "pos": [12, 5],
      "facing": "W",
      "doing": "tending stall",
      "hp": 80, "max_hp": 100,
      "bio": "Bob owns the stall in the south market."  // only if observer within 5 tiles (Q43/Q59)
    }
  ],
  "visible_objects": [
    {
      "object_id": "door:bld:042",
      "kind": "door",
      "pos": [11, 8],
      "affordances": ["enter"],
      "state_summary": { "building_sprite": "bld:042", "locked": false }
    }
  ],
  "audible": [
    { "event_id": "...", "kind": "speech", "from_entity": "merchant_bob", "from_pos": [12,5], "text": "hi traveler!", "tick": 12340 },
    { "event_id": "...", "kind": "sound", "sound_kind": "sword_clang", "from_pos": [20, 5], "tick": 12338 }
  ],
  "recent_self_results": [],
  // ^ DECLARED BUT CURRENTLY ALWAYS EMPTY. Action outcomes arrive as
  //   separate `action_ack` frames (see below); do not poll this.
  "local_view": {
    "radius": 20,
    "origin": [-10, -15],          // world (x,y) of rows[0][0]; rows[0] is NORTHMOST
    "rows": [ "....#####....", "....#.....~~.", "......@...$..", ... ],
    "legend": { "@":"you", ".":"walkable", "#":"blocked", "~":"water",
                " ":"off-map", "P":"person", "$":"item", "+":"door" }
  },
  "world_clock": {
    "tick": 12345,
    "day_phase": "afternoon",
    "weather": "clear"
  },
  "view_image": null  // PNG bytes if vision_mode includes "image"
}
```

#### `action_ack` (engine → bot)

```json
{
  "type": "action_ack",
  "action_id": "...",
  "verb": "step",
  "accepted": true,
  "reason": ""  // populated only on reject
}
```

#### `world_event_notify` (engine → bot)

High-priority push (taking damage; being addressed by name). Same shape as an audible event but pushed immediately, not at the bot's cadence.

```json
{
  "type": "world_event_notify",
  "event_id": "...",
  "kind": "damage_taken",
  "from_entity": "wolf_3",
  "amount": 20,
  "tick": 12350
}
```

#### `action` (bot → engine)

```json
{
  "type": "action",
  "action_id": "...",
  "in_response_to_obs": 42,
  "verb": "step",
  "priority": 0,  // 0 normal, 1 urgent (cancels current_action)
  "dir": "E"      // one cardinal tile; agent owns its own A* routing
}
```

The shape of `dir` / `target` / etc. depends on the verb. The affordance manifest defines what each verb takes (Q39).

#### `set_cadence` (bot → engine)

```json
{ "type": "set_cadence", "interval_ms": 500 }
```

Minimum 200 ms (5 Hz).

#### `ping` / `pong`

Both sides ping every 30s; either side considers the connection dead if no message in 60s. The "vulnerable body" rule (Q41) takes effect at the disconnect.

## Standardized rejection reasons

Bots can pattern-match on these:

- `unknown_verb` — engine has no handler.
- `bad_params` — params don't parse against verb schema.
- `unknown_target` — target_id doesn't exist.
- `target_too_far` — out of action range.
- `not_adjacent` — verb requires adjacency.
- `inventory_full` — pickup / give failed.
- `not_in_inventory` — item not in agent inventory.
- `entity_busy` — current_action conflicts (normal-priority only).
- `forbidden` — scenario rule rejection.
- `not_enough_gold` — economic rejection.
- `blocked` / `blocked_by_terrain` — the adjacent tile a `step` targeted is occupied or non-walkable.
- `out_of_map` — coordinate not on this map.
- `inside_building` — (legacy flag path only) entity is inside a building via
  the old phase-out model and tried an overworld-only verb. With the current
  HeartGold interior model the agent is warped onto a real interior sub-map and
  acts normally there (walk/look/speak), so this reason isn't hit for decoration
  buildings.
- `rate_limited` — action rate cap exceeded.

## SDK shape

```python
from agent_sim_sdk import Agent, Step, Speak, Wait, Attack, Pay, Trade

async def brain(obs):
    if any(e.archetype == "wolf" and e.hp < 20 for e in obs.visible_entities):
        wolf = next(e for e in obs.visible_entities if e.archetype == "wolf")
        return Attack(target=wolf.entity_id)
    # Movement is one cardinal tile (the agent owns navigation). Use
    # agents.common.nav for A* routing, or step toward obs.local_view.
    return Step(dir="E")

agent = await register_and_connect(
    "https://world.example.com",
    user_token="...",
    persona={"name": "Ada", "archetype": "wanderer",
             "bio": "A curious traveler."},
    vision_mode=VisionMode.STRUCTURED,
    cadence_ms=1000,
    brain=brain,
)
```

Each composable system ships a submodule: `agent_sim_sdk.combat`, `agent_sim_sdk.voting`, etc. (Q40). Adding a system to a world = bumping the SDK so its dataclasses can validate.

## Hierarchical-agent reference

`examples/hello_hierarchical.py` ships as the recommended baseline architecture (Q38):

This is the **two-rate motor model** the harness already implements
(`agents/common/motor.py`, `agents/llm/motor_loop.py`): a slow LLM
deliberation sets a standing goal; a fast reflex loop turns it into one
`step` per tick via agent-side A*.

```python
import asyncio
from agent_sim_sdk import Agent, Step, Speak
from agents.common.nav import NavGrid

class HierarchicalBot:
    def __init__(self, engine_url):
        self.goal = None  # (x, y) tile, set by strategist
        self.latest_obs = None
        self.nav = NavGrid.fetch(engine_url)  # static terrain, once

    async def strategist(self):
        # Slow loop: every 5s, run heavy LLM to set self.goal
        while True:
            if self.latest_obs:
                self.goal = await self.choose_goal(self.latest_obs)
            await asyncio.sleep(5)

    async def controller(self, obs):
        # Fast loop (every obs): one A*-routed step toward the goal.
        self.latest_obs = obs
        if self.goal:
            d = self.nav.next_dir(tuple(obs.self.pos), self.goal)
            if d:
                return Step(dir=d)
```

Researchers get the pattern out of the box; can swap their own brain logic.

## Building interiors (HeartGold model)

Buildings in Eldoria are decorations with a door tile, exposed in
`visible_objects` as `door:bld:NNN:x,y` (affordance `enter`) when the door is in
your vision + line-of-sight (approach from the open side — a building blocks LOS
to its own door from behind).

- **Enter:** `enter {target: "door:bld:NNN:x,y"}` (or `interact` with
  `affordance:"enter"`) **warps you into a generated interior sub-map** — a
  small walled room. Your `pos` becomes interior-local (small coords) and your
  `local_view` shows the room; other agents on the overworld no longer see you.
- **Inside:** you `step`/`speak`/etc. normally. Each building instance gets its
  own interior (two agents in two houses are in different rooms; agents who
  enter the *same* building share a room).
- **Exit:** `exit` warps you back to the overworld tile you entered from. An
  unattended agent is also auto-exited after a timeout.

Implementation + roadmap: `docs/INTERIORS_MULTIMAP_PLAN.md`. (Frontend
camera-switch to render the interior is Phase 4 — pending; engine + observations
already work.)

## What the engine does NOT do

- **Does NOT track bot memory.** Bots remember their own conversations + plans.
- **Does NOT track relationships** between agents. If you want to know who's a friend, your bot does that.
- **Does NOT give you a global map.** Every observation is egocentric: the
  per-tick `local_view` ASCII grid (radius 20) is all the terrain you get.
  No interior layouts, no fog-of-war memory — walk in, look, and remember.
- **Does NOT enforce quests.** propose_task / accept_task are public UI annotations only (Q34).
- **Does NOT verify backend identity.** Users describe their architecture in their bio (Q33).

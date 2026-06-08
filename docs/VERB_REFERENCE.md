# VERB REFERENCE

The full action vocabulary for v1. Base verbs live in the engine. Scenario verbs live in the scenario pack.

## Base verbs (engine-provided, every world has these)

These are universal. The engine itself validates and executes them.

### `step`

Move exactly ONE tile in a compass direction. The AGENT owns navigation: it
computes its own route (A* on the static walkability grid — fetch once from
`GET /api/v1/world/walkability`, see `agents/common/nav.py`) and feeds the
engine one `step` per tick. The engine does **no** pathfinding; it just
executes the committed tile. See docs/AGENT_MOVEMENT_REDESIGN.md.

```
params {
  dir: "N" | "S" | "E" | "W"   # one cardinal tile
}
```

- Rejected `blocked_by_terrain` if the target tile is off-map or non-walkable.
- Rejected `blocked` if the tile is currently occupied by another entity.
- Sets `current_action = move` for the ~0.4s the single-tile walk lerps.

> The old multi-tile `move {target:[x,y]}` verb (engine-side A*) was **removed**.
> Higher-level harnesses express movement as a standing goal
> (pursue/flee/goto) and let a reflex loop emit one `step` per tick — see
> `agents/common/motor.py`.

### `speak`

Speak locally. Heard by entities within ~3 tiles.

```
params {
  text: string          # the utterance
}
```

- Instant verb (no `eta_tick`). Can be issued while another action is in progress.
- The engine emits a `speech_emitted` event consumed by hearing.

### `whisper`

Private one-tile-adjacent speech. Heard only by `target`.

```
params {
  target: entity_id
  text: string
}
```

- Rejected if target isn't adjacent.

### `shout`

Long-range speech. Heard within ~15 tiles.

```
params {
  text: string
}
```

### `look_at`

Focus attention. Doesn't move the camera; it's a hint to the simulation that this agent is paying particular attention to a target — useful for social signaling ("Alice notices Bob is staring at her").

```
params {
  target: entity_id | [x, y]
}
```

### `interact`

Polymorphic verb for "do the thing this object affords." The object's `affordances` field in the observation lists what `interact` will do (e.g. for a chair: `["sit"]`; for a door: `["enter"]`; for a sign: `["read"]`).

```
params {
  target: object_id
  affordance: string    # one of the strings from the object's affordances list
}
```

The engine dispatches to a per-affordance handler. If the affordance is a portal-enter, the engine moves the agent to the target sub-map. If it's a "sit", the engine sets a `seated` state and reduces movement.

### `pickup`

Pick up an item from the ground.

```
params {
  target: item_id
}
```

- Rejected if not adjacent or inventory full.
- Item moves to the agent's inventory.

### `drop`

Drop an item from inventory onto the current tile.

```
params {
  item: item_id
}
```

### `equip`

Wear/wield an item from inventory.

```
params {
  item: item_id
  slot: string          # "main_hand" | "head" | "body" | "feet" | etc.
}
```

### `give`

Give an inventory item to an adjacent entity.

```
params {
  item: item_id
  target: entity_id
}
```

- Rejected if target not adjacent or target's inventory full.

### `attack`

Combat strike against a target.

```
params {
  target: entity_id
  weapon: item_id?      # optional; defaults to equipped main_hand
}
```

- Rejected if target out of weapon range.
- Damage calculation lives in the scenario (the engine just dispatches).
- `attack` is a normal-priority action (you commit to it).
- Sets `current_action = attack` with brief `eta_tick`.

### `defend`

Raise a guard / parry stance. Subsequent incoming damage is reduced for a short window.

```
params {
  duration_ticks: int?  # optional; default scenario-defined
}
```

### `heal`

Use a healing item or skill on self or adjacent target.

```
params {
  target: entity_id     # self or adjacent
  item: item_id?        # optional; some scenarios have skill-based heals
}
```

### `wait`

Idle for N ticks.

```
params {
  duration_ticks: int
}
```

### `noop`

Explicit "do nothing." Useful in heartbeat loops.

```
params { }
```

## Scenario verbs (fantasy_town v1)

These are layered on top by `scenarios/fantasy_town/`. Engine doesn't know about them; the scenario pack registers them at startup.

### `trade`

Open a trade with an adjacent entity.

```
params {
  target: entity_id
  offer: [item_id]         # items I'm offering
  request: [item_id]       # items I want in return
  gold_offer: int          # gold I'm offering
  gold_request: int        # gold I want
}
```

The target's agent receives a `trade_offer` event in their next observation; they can accept or reject by submitting `trade_accept` or `trade_reject`.

### `pay`

Direct gold transfer to an adjacent entity.

```
params {
  target: entity_id
  amount: int
}
```

### `work`

Perform a labor at a designated work object (e.g. chop wood at a stump, fish at a dock, craft at an anvil).

```
params {
  target: object_id     # the work object
  duration_ticks: int   # how long to work
}
```

The engine plays the appropriate animation; on completion, the scenario applies the reward (e.g. +1 wood to inventory, +5 gold).

### `loot`

Take items from a dead or container entity.

```
params {
  target: entity_id | object_id
  items: [item_id]      # subset of what's available
}
```

### `build`

(post-launch / late-v1) Construct a new structure from materials in inventory.

```
params {
  blueprint: string       # e.g. "small_hut", "stone_wall_segment"
  position: [x, y]
  rotation: int?
}
```

Scenario validates materials are present and position is buildable.

## Verb dispatch

The engine maintains a verb registry: `map[string]VerbHandler`.

A `VerbHandler` is:

```go
type VerbHandler struct {
  Validate func(world *World, agent Entity, params []byte) (ok bool, reason string)
  Execute  func(world *World, agent Entity, params []byte) []Event
  Cancel   func(world *World, agent Entity)              // for ongoing actions
}
```

- Base verbs are registered by the engine on startup.
- Scenario verbs are registered by the scenario pack on startup.
- A verb name collision is an error — scenarios can't override base verbs.
- Scenarios CAN modify base verb behavior by registering an event listener (e.g. when `attack` resolves, the scenario reduces target HP).

## Action priorities

- `normal`: queues for the next tick. If the agent has a current_action, the new action waits until current_action finishes (UNLESS the verb is "instant" — speak/whisper/shout/noop, which fire immediately without canceling current_action).
- `urgent`: cancels the current action and runs immediately. Useful for "abort walk, attack the wolf that just appeared."

## Rejection reasons (canonical strings)

Standardized so agents can pattern-match:

- `unknown_verb` — engine has no handler for this name.
- `invalid_params` — params don't parse against verb schema.
- `target_not_found` — target_id doesn't exist or is out of range.
- `target_too_far` — target is out of action range.
- `not_adjacent` — verb requires adjacency.
- `inventory_full` — pickup/give failed.
- `not_in_inventory` — item not in agent inventory.
- `entity_busy` — current_action conflicts (only for normal-priority verbs).
- `forbidden` — scenario rule rejection (e.g. "you can't attack inside the temple").
- `not_enough_gold` — scenario verb economic rejection.
- `blocked_by_terrain` — step target off-map or non-walkable.
- `blocked` — step target tile is currently occupied.
- `out_of_map` — coordinate not on this map.

## Examples

### Walk to the tavern, then greet the bartender:

```python
from agent_sim_sdk import Step, Speak
from agents.common.nav import NavGrid

nav = NavGrid.fetch(engine_url)          # static walkability, fetched once
# Each tick: compute the next cardinal step toward the goal and send it.
d = nav.next_dir(here, (8, 12))          # A* on known terrain → "N"/"S"/...
if d:
    await agent.act(Step(dir=d))
# ...repeat until adjacent, then:
await agent.act(Speak(text="Evening. Pint of ale, please."))
```

> Higher-level agents skip the per-tile bookkeeping by setting a motor goal
> (`Goal.goto(8, 12)`) and letting the reflex loop emit the steps — see
> `agents/common/motor.py` and `agents/baselines/_common.py`.

### Buy bread from the baker:

```python
# already adjacent to baker
await agent.trade(
  target="baker_npc_id",
  offer=[],
  request=["bread_loaf_1"],
  gold_offer=3,
  gold_request=0
)
```

### Defend against a wolf, then strike:

```python
# wolf is 2 tiles away
await agent.defend(duration_ticks=30)
# wait for engine to confirm wolf approached
# T+1s — wolf adjacent
await agent.attack(target="wolf_id", priority="urgent")
```

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

Polymorphic verb for "do the thing this object affords." The object's `affordances` field in the observation lists what `interact` will do. In Eldoria the live affordance is `["enter"]` on door objects (`interact{target, affordance:"enter"}` is equivalent to the `enter` verb). Other affordances (sit/read/etc.) are not implemented in v1.

```
params {
  target: object_id
  affordance: string    # one of the strings from the object's affordances list
}
```

The engine dispatches to a per-affordance handler. If the affordance is a portal-enter, the engine moves the agent to the target sub-map. (Other affordance handlers like "sit" are not implemented in v1.)

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

> There is no separate `noop` verb. To idle, use `wait` (above) — `wait {ticks: N}`.

## Composable-system verbs

These are registered by composable systems (combat / money / inventory /
property / resources / construction / trade / loot / verbalquests), not the
core engine. The authoritative, always-current list — with each verb's
parameter schema, rejection reasons, and emitted events — is served live at
`GET /api/v1/world/affordances`. The table below is a hand-written reference;
when in doubt, trust the endpoint. (Verified against the live manifest +
`tools/audit/verb_matrix.py` on 2026-06-08: 30 system verbs, all rejection
contracts confirmed.)

### Economy — `money` system

#### `pay` — `{ target, amount }`
Transfer `amount` gold to an adjacent agent (chebyshev ≤ 1). No reciprocity is
enforced — repayment is emergent. Emits `GoldTransferred`.
Rejections: `bad_params`, `unknown_target`, `target_too_far`, `not_enough_gold`.

#### `work_for_pay` — `{}`
Earn a wage. **Gated on a worksite**: a building must be within
`worksite_radius` (default **6** tiles) — otherwise `no_worksite_nearby`. This
makes work happen at real places, not anywhere. Emits `GoldTransferred`.
Rejections: `no_worksite_nearby`.

#### `buy_food` — `{}`
Spend `food_price` gold (default **6**) to cut hunger by `food_relief`
(default **0.5**). The economy's gold sink + survival loop. Optional spatial
gate: when `market_radius` > 0 a market stall (`bld:stall*`) must be within
range; **Eldoria currently sets `market_radius = 0`, so food is buyable
anywhere** with gold + hunger > 0. Emits `GoldSpent`.
Rejections: `not_hungry` (hunger already 0), `not_enough_gold`, `no_market_nearby`.

### Survival / inventory — `inventory` + `vitals`

Hunger (`vitals`) rises every tick; above a threshold it drains HP, and at 0 HP
the agent dies (a death scream fires and the body is removed). Counter it with
food:

#### `eat` — `{ item }`
Consume a food item from inventory; subtracts its satiety from hunger (clamped
at 0). Instant, no cooldown. Emits `AteFood`.
Rejections: `bad_params`, `not_in_inventory`, `not_food` (item has no satiety).

#### `cook` — `{ item }`
Turn a raw food item into its cooked form (higher satiety), e.g.
`fish_raw → fish_cooked`. Recipes are world-tuned.
Rejections: `bad_params`, `not_in_inventory`, `not_cookable`.

#### `pickup` / `drop` / `equip` / `give`
`pickup {target}` — take an adjacent ground item (emits `ItemPicked`). **Coins
auto-convert to gold on pickup; they never enter inventory.**
`drop {item}` — drop from inventory (emits `ItemDropped`).
`equip {item, slot?}` — wield a weapon from inventory.
`give {target, item}` — hand an item to an adjacent agent (emits `ItemTransferred`).
Rejections include `not_an_item`, `inventory_full`, `not_in_inventory`, `target_too_far`.

### Resources — `resources` system

#### `chop {target}` / `mine {target}`
Fell an adjacent tree (wood) / break an adjacent rock (stone). Yields item
entities into inventory; the source depletes and **regenerates** after
`resource_regen_interval` (default **1800** ticks). Emits `ResourceHarvested`,
then `ResourceDepleted` when exhausted.
Rejections: `not_a_tree`/`not_a_rock`, `target_too_far`, `no_yield`, `depleted`.

#### `forage {target}` — renewable food gathering
Gather fruit (a food item) from an adjacent tree/bush **without felling it**.
The source ripens again after `forage_cooldown` (default **600** ticks), so
forage before that returns `not_ripe`. Emits `ResourceHarvested`.
Rejections: `bad_params`, `unknown_target`, `not_forageable`, `target_too_far`, `not_ripe`.

### Trade & loot

#### `trade` — `{ target, item, price }`
**Atomic** item-for-gold swap with an adjacent agent: the item moves one way
and `price` gold the other, or neither moves. Emits `GoldTransferred` +
`ItemTransferred`.
Rejections: `unknown_target`, `target_too_far`, `not_in_inventory`, `target_not_enough_gold`.

#### `loot` — `{ target }`
Take gold + clear inventory from an adjacent **corpse** (HP 0). Emits
`GoldTransferred`. Rejections: `target_alive`, `target_too_far`, `unknown_target`.

### Combat — `combat` system

`attack {target}` (adjacent; emits `DamageDealt`, and `EntityDied` on a kill),
`defend {}` (halves the next incoming hit), `heal {target?}` (default self).

### Property & buildings — `property` system + interiors

`enter {target}` / `exit {}` — step into / out of a building. For Eldoria's
**decoration buildings** the target is the door object id from
`visible_objects` (`door:bld:NNN:x,y`); entering **warps the agent into a
separate interior sub-map** (portal sub-map model — see
`docs/INTERIORS_MULTIMAP_PLAN.md`) where it can walk around; `exit` warps it
back to the door. Entity-backed buildings (from construction) use the same
verbs with a building entity id. Emits `EnteredBuilding` / `ExitedBuilding`.
`lock`/`unlock`/`claim_ownership`/`transfer_ownership {target,...}` — ownership
+ access control (owner-gated). Emits `BuildingLocked`/`Unlocked`/`OwnershipChanged`.

### Construction — `construction` system

`place_blueprint {kind, at}` (spends materials up front; emits
`ConstructionStarted`), `advance_construction {target}` (spends the next batch;
emits `ConstructionAdvanced`, then `ConstructionCompleted` at 100%), `demolish
{target}` (removes an owned blueprint/building; emits `Demolished`).

### Verbal contracts — `verbalquests` system

`propose_task {target, terms, reward?}` → `accept_task`/`reject_task`/
`complete_task {id}`. The engine records the contract ledger on both parties'
`contracts` extras and emits `TaskProposed`/`Accepted`/`Rejected`/`Completed`,
but does **not** enforce completion — fulfilment is emergent.

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

- `normal`: queues for the next tick. If the agent has a current_action, the new action waits until current_action finishes (UNLESS the verb is "instant" — speak/whisper/shout, which fire immediately without canceling current_action).
- `urgent`: cancels the current action and runs immediately. Useful for "abort walk, attack the wolf that just appeared."

## Rejection reasons (canonical strings)

Each verb's exact rejection set is in its `/api/v1/world/affordances` entry
(and listed per-verb above). These are the strings the engine ACTUALLY returns
in `action_ack.reason` — verified live by `tools/audit/verb_matrix.py`
(2026-06-08). Agents should branch on these:

- `bad_params` — params missing/malformed for the verb.
- `unknown_target` — target entity id not found.
- `target_too_far` — target outside the verb's range (most are chebyshev ≤ 1).
- `not_enough_gold` / `target_not_enough_gold` — payer/partner can't afford it.
- `not_in_inventory` — item not held; `inventory_full` — pickup with no room.
- `not_an_item` / `not_a_tree` / `not_a_rock` / `not_a_building` / `not_a_structure`
  / `not_a_blueprint` — target is the wrong archetype for the verb.
- `not_food` (eat) / `not_cookable` (cook) / `not_forageable` / `not_ripe` (forage)
  / `no_yield` / `depleted` (chop/mine).
- `not_hungry` / `no_market_nearby` (buy_food); `no_worksite_nearby` (work_for_pay).
- `already_inside` / `not_inside` / `locked` (enter/exit); `not_owner`
  / `already_owned` / `unknown_new_owner` (property).
- `unknown_blueprint` / `missing_materials` / `unwalkable` / `broken_blueprint`
  / `spawn_failed` / `no_inventory_service` (construction).
- `target_alive` (loot a living target); `unknown_contract` / `bad_status`
  / `not_authorized` / `self_target` / `empty_terms` (verbal contracts).
- `rate_limited` — too many actions too fast (per-connection token bucket).

Movement (`step`) returns `accepted:false` with no reason string when the target
tile is blocked (wall/water/off-map/occupied); the agent's `pos` simply doesn't
change. Read `local_view` to avoid blocked tiles.

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

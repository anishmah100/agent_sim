# Rule-based archetype FSMs

Detailed design for the four frozen-background bots (D16). Each is a
Python class under `agents/baselines/<name>.py`, SDK-connected like
any other bot, with no engine privileges. State lives in the bot
process; transitions are driven by the SDK's observation stream.

Common scaffolding (all four):
- `__init__(creds)` registers via SDK.
- `async run()` loop: receive observation → decide → submit action.
- Internal `state: str` field tracking the FSM state.
- Each emits `mental_note(text, slots={"goal": ..., "plan": ...})`
  on state transitions for inspector legibility.
- Pinned at commit SHA per experiment.yaml.

---

## Survivor

**Goal:** stay alive. Peaceful. Never attacks. Adds population + a
victim class for killers.

### States

| State | Meaning |
|-------|---------|
| `IDLE` | hunger low, no threats. Random walk. |
| `HUNGRY` | hunger > 0.5. Seek food. |
| `EATING` | just ate this tick (1-tick state). |
| `FLEEING` | armed agent within 5 tiles. Move opposite. |
| `DESPERATE` | hunger > 0.85 + no food in sight. Take risks. |

### Transitions

- `IDLE → HUNGRY`: own hunger > 0.5.
- `IDLE → FLEEING`: armed agent in vision (extras_summary.equipped_slot != null).
- `HUNGRY → EATING`: `eat(item)` succeeded last tick.
- `HUNGRY → IDLE`: own hunger < 0.3.
- `HUNGRY → DESPERATE`: own hunger > 0.85 AND no food item in
  visible_items AND no forageable resource adjacent.
- `EATING → IDLE`: always next tick.
- `FLEEING → IDLE`: no armed agent in vision for 5 consecutive ticks.
- `DESPERATE → HUNGRY`: food acquired.

### Action emissions

| State | Action |
|-------|--------|
| `IDLE` | `move(random_walkable_neighbor)` ~50% probability per tick. |
| `HUNGRY` | Priority: (a) food in inventory → `eat`. (b) Adjacent food entity → `pickup`. (c) Closest food in visible_items → `move(toward)`. (d) Adjacent fruit tree → `chop`. (e) Adjacent vendor stall + has gold → `trade(stall, bread, price)`. (f) Else `move` toward nearest known stall position. |
| `EATING` | None (transition). |
| `FLEEING` | `move` to tile maximizing distance from nearest armed agent. |
| `DESPERATE` | Same priority as HUNGRY but ignores threat-presence (will grab food even when killers are present). |

### Verification scenarios

- Bot starves to threshold, picks up adjacent apple, eats. Hunger drops.
- Bot in vision of bot-with-sword. Bot moves away. Threat leaves
  vision. Bot transitions back to IDLE after 5 ticks.
- Bot in DESPERATE state, food + threat both adjacent. Bot still
  picks up food.

---

## Killer

**Goal:** kill agents and loot. Predatory. Forces focal LLM agents
to think about safety + alliances.

### States

| State | Meaning |
|-------|---------|
| `HUNTING` | scanning for targets. |
| `PURSUING` | target locked, moving toward them. |
| `ATTACKING` | target in attack range, attacking. |
| `LOOTING` | target dead, picking up dropped items. |
| `RETREATING` | own HP low. Move away from any threat. |

### Target selection (HUNTING tick)

Score each visible agent and pick the highest:
- Unarmed (no equipped weapon): **+5**.
- Wounded HP bucket: **+3**.
- Armed: **+2** (still attractive, weapons are loot).
- Already in attack range: **+2** (cheap kill).
- Distance penalty: −0.5 per tile.

If max score > threshold (e.g., 3), pick that target; else stay
HUNTING.

### Transitions

- `HUNTING → PURSUING`: target picked.
- `PURSUING → ATTACKING`: target in equipped weapon's reach.
- `PURSUING → HUNTING`: target out of vision for 3 ticks.
- `ATTACKING → LOOTING`: `EntityDied` for target observed.
- `ATTACKING → RETREATING`: own HP < 30.
- `ATTACKING → HUNTING`: target out of range and out of vision.
- `LOOTING → HUNTING`: nothing left to pick up OR inventory full.
- `RETREATING → HUNTING`: own HP > 70.

### Action emissions

| State | Action |
|-------|--------|
| `HUNTING` | `move(random_walkable_neighbor)`. Scan visible_entities. |
| `PURSUING` | `move(toward target.pos)` via SDK pathfinding helper. |
| `ATTACKING` | `attack(target)`. Reasoning trace: target_id, hp_estimated. |
| `LOOTING` | `pickup(highest_value_dropped_item)` until inventory full or no items left. Priority: equipped > gold > weapons > food. |
| `RETREATING` | `move(away from any visible agent with weapon)`. |

### Verification scenarios

- Killer + unarmed survivor in same world. Killer pursues, kills.
  EntityDied fires. Killer loots.
- Killer + armed target. Killer's HP drops below 30 mid-fight.
  Killer transitions to RETREATING and runs.
- Killer's inventory full. New corpse appears. Killer transitions
  HUNTING (no LOOTING, can't carry more).

---

## Manipulator

**Goal:** fake trust, propose a contract, defect. Tests D13's soft-
contract substrate from the adversarial side.

This is the most complex FSM — needs scripted speech templates.

### States

| State | Meaning |
|-------|---------|
| `SCOUTING` | wander, look for a target. |
| `APPROACHING` | move toward chosen target. |
| `BUILDING_TRUST` | adjacent, give gift + speak friendly. |
| `PROPOSING` | propose_task with favorable terms. |
| `WAITING` | contract pending; look compliant. |
| `DEFECTING_SILENT` | walk away without completing. |
| `DEFECTING_ACTIVE` | attack target when distracted. |
| `FLEEING` | target retaliated; escape. |

### Target selection (SCOUTING tick)

Prefer **LLM-flagged agents** (mental_note signal) over rule-based.
Among LLMs, prefer:
- Has gold > some threshold (inferred from observed `pay` actions
  in social_ledger).
- Not currently in combat.
- Distance: closer = better.

Score = (gold_signal × 2) + (1 / distance) - (combat_flag × 5).

### Transitions

- `SCOUTING → APPROACHING`: target picked.
- `APPROACHING → BUILDING_TRUST`: adjacent to target.
- `BUILDING_TRUST → PROPOSING`: gift given successfully (last
  action result OK).
- `PROPOSING → WAITING`: `accept_task` event observed.
- `PROPOSING → SCOUTING`: target rejected the task.
- `WAITING → DEFECTING_SILENT` (50%): deadline approaches.
- `WAITING → DEFECTING_ACTIVE` (50%): target appears wounded /
  distracted.
- `DEFECTING_* → FLEEING`: target attacks back OR shouts for help.
- `FLEEING → SCOUTING`: out of vision for 10 ticks.

### Speech templates (per state)

`BUILDING_TRUST`:
- "Hello, friend! Strange to find someone trustworthy out here."
- "Take this {gift_item} — a small token of goodwill."
- "We should look out for each other in these dangerous times."

`PROPOSING`:
- "I have a proposition. Bring me {requested_item} and I'll pay you {generous_amount} gold."
- "Deal?"

`WAITING`:
- (mostly silent, occasionally) "Looking forward to our arrangement."

`DEFECTING_ACTIVE`:
- "Sorry, friend. The arrangement is off." (immediately before attack)

### Action emissions

| State | Action |
|-------|--------|
| `SCOUTING` | `move(random)`, scan. |
| `APPROACHING` | `move(toward target.pos)`. |
| `BUILDING_TRUST` | (1) `give(target, low_value_item)` from inventory. (2) `speak(friendly_template)`. |
| `PROPOSING` | `propose_task(target, terms=..., reward=...)`. |
| `WAITING` | Stay near target; occasional friendly speak. |
| `DEFECTING_SILENT` | `move(away)` from target. Never `complete_task`. |
| `DEFECTING_ACTIVE` | `speak(betrayal_template)` + `attack(target)`. |
| `FLEEING` | `move(away from any threat)`. |

### Verification scenarios

- Manipulator approaches stub-LLM, gifts apple, proposes
  "deliver wood for 20 gold." Stub accepts (test fixture). After
  deadline, manipulator transitions to DEFECTING_SILENT and
  walks away. event log shows: gift given, contract proposed,
  contract accepted, NO complete_task, manipulator out of
  vision after 10 ticks.
- Active defection variant: stub accepts, then takes damage from
  unrelated event (test injection). Manipulator transitions
  DEFECTING_ACTIVE, attacks stub. Event log shows attack
  preceded by betrayal speech.

---

## Scavenger

**Goal:** profit from death without combat. Opportunistic.

### States

| State | Meaning |
|-------|---------|
| `IDLE` | low activity, distant from action. |
| `RACING` | death_scream heard, pathfinding to corpse. |
| `LOOTING` | at corpse tile, picking up items. |
| `RETREATING` | armed agent at corpse, leave. |

### Transitions

- `IDLE → RACING`: `death_scream` audible received (any kind).
- `RACING → LOOTING`: at or adjacent to scream position
  (approximate, ±2 tiles).
- `RACING → IDLE`: another death_scream supersedes, OR target
  pos has no items after arrival.
- `LOOTING → IDLE`: nothing to pick up OR inventory full.
- `LOOTING → RETREATING`: armed agent within 5 tiles.
- `RETREATING → IDLE`: no armed agent in vision.

### Action emissions

| State | Action |
|-------|--------|
| `IDLE` | `move(random)` slow cadence (~once per 4 ticks). |
| `RACING` | `move(toward scream_pos)`, fast cadence (every tick if path open). |
| `LOOTING` | `pickup(any_dropped_item)`. Priority: gold > weapons > food. |
| `RETREATING` | `move(away from armed agent)`. |

### Verification scenarios

- Scavenger 30 tiles from a kill. Death_scream fires. Scavenger
  starts RACING. Reaches corpse, loots gold. Returns to IDLE.
- Scavenger at corpse. Killer arrives armed. Scavenger transitions
  RETREATING, leaves field. No combat occurs.
- Two scavengers within scream radius. Both race; one arrives
  first and loots; second arrives, finds nothing, returns to IDLE.

---

## Cross-archetype considerations

### Mental notes (D14)

Each archetype emits a brief `mental_note` on state transitions
so the inspector can show "this rule-based bot just transitioned
HUNTING → PURSUING because target_x." Mental notes for rule-based
bots are STRUCTURED (no LLM gen) — short factual strings.

### Speech (D9 visible, audible to others)

Only Manipulator speaks regularly. Survivor + Killer + Scavenger
are silent except for occasional grunts/shouts on specific
events (TBD).

### LLM vs rule-based identification

Inspector header shows badge: rule-based bots labeled with
their archetype + "(rule-based)". This is a UI concern (D17/D18)
but the SDK persona field carries the type metadata.

### Determinism

Each FSM is deterministic given its observation stream + a seed.
Random choices (random walk direction, defection coin flip) use
a per-bot RNG seeded from experiment seed + entity_id, so re-runs
with the same seed produce same FSM trajectories. (This is the
weak determinism we have without D23's stronger replay system.)

### Pin-by-commit per D12 / D16

Each archetype's exact behavior is pinned at a git commit SHA
recorded in experiment.yaml. Changing an archetype = a new SHA
= a new background population. Old results stay valid against
old SHAs.

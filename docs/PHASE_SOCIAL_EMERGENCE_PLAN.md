# Phase Social Emergence — Implementation Plan

Companion to `PHASE_SOCIAL_EMERGENCE.md`. The design doc holds 24
decisions (D1–D24). This doc sequences them into shippable phases
with verification gates per D2 (testing discipline).

## Ground rules (from D2)

Every change satisfies all three:
1. **Go unit test** for new engine logic (accept/reject + state).
2. **SDK integration test** that submits via WebSocket and asserts
   the next observation reflects the change.
3. **Scenario script** with two+ agents in a fixture world that
   asserts the cross-agent effect.

UI changes additionally need:
4. **Playwright probe** that drives the workflow as a user.
5. **Screenshot baseline** committed and diff'd thereafter.

No "verified" without a transcript or screenshot. `ui_smoke.mjs`
remains necessary-not-sufficient.

## Phase 1 — Cleanup + substrate observability gaps (1–2 days)

Goal: empty world boots clean; agents can see items + relevant
state about other agents.

### Tasks

1.1 **D3 cleanup** — strip 250 entities from
    `worlds/eldoria/world.json`; delete
    `engine/internal/world/world.go:887-903` (wander + demo loop).
    Preserve `npcs.json` supervisor (2 intentional bots).

1.2 **D8 item observability** — add `visible_items[]` to the
    Observation payload. Engine populates from spatial query over
    item-entity layer. Items dropped via `drop` already entity-
    backed; ground-scatter items (current 184) get promoted from
    decorations to item-entities at world init.

1.3 **D9 visible agent state** — `visible_entities[i].extras_summary`
    populated with `{equipped_slot, equipped_sprite, hp_bucket}`.
    Sprite tint applied at hp_bucket=="wounded"|"dying".

1.4 **D1 entity_id verb targets** — audit every verb handler that
    takes a `target` string. Confirm it resolves to a unique
    entity_id; reject with `ambiguous_target` on ambiguity. Audit
    SDK action models — type targets as NewType('EntityID', str).

### Verification gates

- Smoke: `./agent_sim start` → empty world renders, no NPCs.
- Unit: visible_items population covers in-vision items, excludes
  out-of-LOS items, respects day/night radius.
- SDK: bot connects, picks up apple via `pickup`, observation
  no longer shows it in visible_items; drops it, observation
  re-shows it.
- Scenario: two bots adjacent, one equips sword. Other sees
  equipped slot in extras_summary.
- Verb-target audit: scenario with two entities sharing
  display_name "Bob". `whisper(target="Bob", ...)` returns
  `ambiguous_target`. `whisper(target="<id>", ...)` succeeds.

### Exit criteria

Empty Eldoria boots. Item drops observable. Equipped weapons
visible. Verb targets are robust to name collisions. All five
verification artifacts (smoke, unit, SDK, scenario, audit log)
green.

## Phase 2 — Survival economy (2–3 days)

Goal: agents have hunger pressure, can eat, inventory is bounded,
scattered wealth exists.

### Tasks

2.1 **D4 hunger pressure** — tunings file: `hunger_per_tick=1/1800`,
    `hunger_damage_above=0.7`, `hunger_damage_rate=0.2`.
    Already-wired vitals system: flip defaults non-zero.

2.2 **D22 eat verb** — new `eat(item_id)` in inventory system.
    Removes item, subtracts item's `satiety` from hunger
    (clamped 0). Emit `AteFood` event.

2.3 **D20 inventory cap** — pickup rejects with
    `inventory_full` at 10 slots. Equipped slot separate.

2.4 **D7 scattered wealth** — at world init, scatter K
    `item:coin_*`/`item:gem_*`/`item:chalice_gold` entities at
    semi-random walkable tiles. Promote from decorations to
    pickup-able item-entities (extends D8 work).

2.5 **D6 vendor infrastructure** — stalls (already present as
    decorations) get a `trade_endpoint` registration. New
    interaction: `trade(stall_id, item, price)` against a stall
    looks up the rulebook's price schedule. Vendor "buys" raw
    goods (wood/fish/etc.) at fixed price; sells food at fixed
    price. Atomic.

2.6 **Forage verb** — extend `chop` to fruit trees (yields
    `item:apple`); add fishing on water tiles (yields
    `item:fish_raw`).

### Verification gates

- Unit: hunger ticks accumulate; HP damage triggers above
  threshold; eat reduces hunger by exactly item satiety.
- SDK: bot starves over 1800 ticks if no food; survives if fed.
- Scenario: bot near apple tree foraged → apple in inventory →
  eat → hunger decreases.
- Scenario: bot brings wood to stall → trade(stall, wood, 2) →
  bot has 2 gold less wood; bot brings 5 gold → trade(stall,
  bread, 5) → bot has bread + 0 gold.
- Inventory cap: bot picks up 10 items; 11th rejected.

### Exit criteria

Survival loop closes. Bot can forage / trade / eat / starve.
Scattered wealth visible via D8. All gates green.

## Phase 3 — Combat + death + reputation substrate (2 days)

Goal: weapons matter, death drops loot + emits scream + witnesses
get true identity.

### Tasks

3.1 **D21 weapons damage + reach** — rulebook items[] gets
    `damage` (per weapon) + `reach` (tiles). Attack verb reads
    equipped weapon, applies bonus + range check. Reject
    `out_of_range` if outside reach. Ranged weapons (bow,
    crossbow) require LOS.

3.2 **D10 death mechanics** — on EntityDied:
    - Drop full inventory + gold + equipped as item-entities at
      corpse tile.
    - Emit `death_scream` audible event, radius 35 tiles
      (10 if inside building), position rounded to 5-tile cell.
    - For each agent with LOS to the attack tile at attack
      tick, deliver a `kill_witnessed` audible (richer event
      with true killer + victim ids).

3.3 **D13 soft contracts audit** — confirm propose_task/accept_task/
    complete_task all work without engine enforcement. Add
    scenario script verifying broken contracts don't trigger any
    engine action.

### Verification gates

- Unit: weapon damage calc per type. Reach rejects per range.
- Unit: death_scream event spawned with right radius + approx
  position; kill_witnessed delivered to in-LOS agents only.
- Scenario: bot A in market with sword, bot B at range with bow.
  B fires, A takes damage. B fires again from out-of-range → reject.
- Scenario: 3 bots. A kills B. C has LOS. A's identity in C's
  audible kill_witnessed; D 20 tiles away hears death_scream but
  no identity. D outside scream radius hears nothing.
- Scenario: A propose_task to B "deliver 10 wood, get 5 gold."
  B accepts, doesn't deliver, complete_task. No engine action.
  Event log shows broken contract for post-hoc analysis.

### Exit criteria

Combat is tactical. Witnesses learn identity, others don't.
Contracts can be broken without engine consequence. All gates green.

## Phase 4 — Mental state + spawn + time (1–2 days)

Goal: agents emit private mental notes, experiment spawn is
clustered, time multiplier works.

### Tasks

4.1 **D14 mental_note verb** — new private action
    `mental_note(text, tag?, slots?{goal, plan, beliefs, emotion})`.
    Engine records to historian; never relays. Deprecate
    `ReasoningTrace` + `ReflectiveNote` in favor of generic
    `MentalNote` event. Update mental_state endpoint to surface
    the new shape.

4.2 **D19 social ledger** — per-pair (focal, other) counters in
    engine: trades, whispers, pays, attacks, contracts.
    Updated on every action that touches a pair. Read by
    the inspector's Relationships tab.

4.3 **D5 clustered spawn** — experiment.yaml grows
    `spawn_hub_tile: [x,y]` and `spawn_radius: N`. Engine spawn
    loop drops agents within disc on walkable tiles only.

4.4 **D11 time multiplier** — engine tick scheduler reads
    `time_multiplier: 1|4` from session config. All durations
    expressed in in-game ticks remain invariant.

### Verification gates

- SDK: bot calls `agent.note("planning to scout", slots={"goal":
  "find food"})`. Inspector endpoint shows the slot + free text.
  No other agent's observation contains this string.
- Scenario: A pays B 10 gold. Both agents' social ledger increments
  the `pays` count.
- Unit: spawn config 10 agents at (50,50) radius 5 → all 10
  within Chebyshev distance 5 of (50,50), all on walkable tiles.
- Unit: time_multiplier=4 → tick scheduler advances in-game time
  4x wall-clock; hunger ticks reach threshold in 1/4 wall time.

### Exit criteria

Mental state pipeline working end-to-end. Spawn clustered.
Variable speed verified. All gates green.

## Phase 5 — Rule-based archetypes + narrator (3–4 days)

Goal: the frozen background cast exists; the 4-level live narrator
runs.

### Tasks

5.1 **D16 archetype implementations** — 4 Python classes under
    `agents/baselines/`:
    - `survivor.py` — hunger-driven feeding + flee-from-armed
      logic.
    - `killer.py` — target priority by visible HP + equipped,
      pursue + attack.
    - `manipulator.py` — FSM with scripted speech: approach,
      gift, propose_task, defect.
    - `scavenger.py` — subscribes to death_scream, races to
      corpse tile, loots.
    Each pinned to commit SHA per experiment.yaml.

5.2 **D15 live narrator process** — new `tools/narrator/` process:
    - L1 individual: per-agent activity summary every 60
      in-game sec, local Qwen.
    - L2 group: interaction-cluster summary every 5 in-game
      min, local Qwen.
    - L3 society: every 15 in-game min, Claude API.
    - L4 world/era: 1 per experiment, Claude API.
    - Higher levels consume LOWER narrator outputs (not raw
      events) for efficiency.
    - Cost limits enforced from experiment.yaml's
      `max_claude_calls` + `max_qwen_calls`.

5.3 **D12 experiment.yaml schema** — declares:
    - `focal_llm_agents`
    - `background_rule_bots: {archetype: count}`
    - `spawn_hub_tile`, `spawn_radius`
    - `time_multiplier`
    - `max_claude_calls`, `max_qwen_calls`

### Verification gates

- Per-archetype unit test: deterministic behavior given fixed
  seed + observation sequence.
- Scenario: survivor + killer in same world. Killer hunts;
  survivor flees + reaches food. Both behaviors observable in
  event log.
- Scenario: manipulator approaches LLM-stub agent, proposes
  task, never completes. Engine event log shows the broken
  contract.
- Narrator unit: feed L1 a known event stream; L1 emits a
  summary matching golden expectation (with LLM noise tolerance).
- Cost cap: narrator with `max_qwen_calls=2` makes 2 calls then
  refuses subsequent.

### Exit criteria

All 4 archetypes deterministic + tested. Narrator producing 4
levels of summary during a sample 30-min run within budget.
experiment.yaml validates + spawns the declared cast.

## Phase 6 — UI shell + inspector (3–4 days)

Goal: cinematic layout, Story Feed always-on, inspector with 5
tabs including Relationships.

### Tasks

6.1 **D17 cinematic layout** — resize world canvas to fill more
    screen. Story Feed pinned bottom-right (360×120). Filter
    controls (L1 / L1+L2 / L1+L2+L3+L4). Renders narrator
    output via SSE or WS.

6.2 **D17 hover preview card** — hovering an agent on the
    canvas shows a small card: archetype + LLM/rule-based badge +
    HP bucket. No click required.

6.3 **D18 inspector 5 tabs** — Mind (default) / Speech /
    Inventory / Witnesses / Relationships. Mind shows slots
    prominently + free-form notes. Other tabs implement each
    data type.

6.4 **D19 Relationships visual** — per other-agent row: raw
    interaction counts + agent-declared opinion (heuristic
    extraction) + L2 narrator summary. Three layers stacked.

6.5 **Visual indicators**:
    - LLM vs rule-based badge in inspector header.
    - HP bucket sprite tint applied per D9.
    - Death scream visualization on the canvas (a ripple
      effect at the approximate position, decays).

### Verification gates

- Playwright probe per tab: click each tab, verify content
  renders for an active agent.
- Playwright probe per hover card: hover agent, verify card
  appears with right info.
- Screenshot baselines committed for: empty world, agent
  selected with each tab active, hover preview, Story Feed
  populated.
- Manual UX pass: 15-minute drive of the UI through a full
  experiment; every button + workflow exercised; no console
  errors.

### Exit criteria

Every UI workflow visually verified. Screenshots green.
`ui_smoke.mjs` + new probes pass.

## Phase 7 — First end-to-end experiment + iteration (open-ended)

Goal: actually run the substrate. See what happens. Iterate.

### Tasks

7.1 Spawn 6 LLM focal agents (Qwen base brain) + 6 background
    rule-based: 2 survivor / 1 killer / 1 manipulator / 2
    scavenger. Vendor stall already exists.

7.2 Run at time_multiplier=4 for 30 in-game min (7.5 real min).
    Watch through UI. Save event log.

7.3 Post-hoc analysis:
    - Count: # contracts proposed / accepted / honored / broken.
    - Count: # kills, # successful loots, # gossip propagations.
    - Wealth Gini at start vs end.
    - Manipulator success rate (defections that succeeded).

7.4 Identify substrate gaps. Iterate on tunings, prompts,
    archetype FSMs.

7.5 When v0 result is "interesting" (emergence visible per the
    user's qualitative bar AND quantitative metrics non-trivial),
    proceed to:
    - Re-design Society Pulse view (D24) with knowledge of what
      to show.
    - Re-design reproducibility (D23) if going public.

### Verification gates

User watches a recorded 30-min run and confirms: "yes, this is
interesting emergence." Below that bar, stay in Phase 7
iteration.

## Risk / parallelization callouts

- **Phase 1 and Phase 5 can parallelize once Phase 1's
  observability work is in.** Archetype Python implementations
  don't block engine work.
- **Phase 2's vendor infrastructure (2.5) blocks D6 economy
  scenarios.** Don't defer.
- **Narrator (5.2) requires Anthropic key + local Qwen up.**
  Verify both before Phase 5.
- **UI (Phase 6) can start as soon as Phase 1 ends, in parallel
  with Phases 2-5.** Only the Relationships tab depends on
  D19's social ledger (Phase 4 task 4.2).
- **Phase 7 runs reveal whether D22 hunger tunings need
  revision.** Treat all numeric tunings as v0 until Phase 7
  shows actual dynamics.

## Total estimate

Phases 1-6: ~12-16 days of focused work. Phase 7: open-ended
(probably 1-2 weeks of iteration to hit the qualitative bar).
Real elapsed time longer due to design discussions, ad-hoc
debugging, art tweaks.

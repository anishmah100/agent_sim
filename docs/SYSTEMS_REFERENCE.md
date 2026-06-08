# Systems reference — the composable gameplay systems

The engine core knows nothing about money, combat, or hunger (see
`docs/ARCHITECTURE.md` §2). All gameplay lives in **composable systems**
registered by a scenario (Eldoria uses `fantasy_town`). Each system registers
verbs, tick hooks, event-bus subscriptions, manifest declarations, and optional
services on the `Registry`. This doc is the per-system reference: what each owns,
its verbs/events/state, key tunings, and the invariants verified by the
2026-06-08 audit (`docs/ENVIRONMENT_AUDIT_PLAN.md` + `AUDIT_FINDINGS`).

Authoritative live surface: `GET /api/v1/world/affordances`
(30 system verbs, 23 event types, 19 state fields, 6 archetypes).

---

## How a system plugs in

A system implements `Name()` + `RegisterWith(r Registry)`. In `RegisterWith` it
may call:
- `r.Verb(name, handler)` — an agent action verb. Handler returns an
  `ActionResult{Accepted, Reason}`; the reason MUST be in the manifest's
  declared `RejectionReasons` (the verb-matrix audit enforces this).
- `r.OnTick(fn)` — runs every tick under the world write lock.
- `r.OnEvent(kind, fn)` / `r.OnEntitySpawn(fn)` — bus + spawn hooks.
- `r.Service(name, impl)` — expose an interface other systems consume via
  `w.GetService(name)`.
- `r.Manifest(decl)` — declares verbs/events/state/archetypes for the
  affordance endpoint + rulebook.

Systems communicate ONLY through services + events, never direct imports of
each other's internals (one exception: `vitals` and `reputation` import
`combat` for its `CombatService`/event types — a deliberate, acyclic dep).

State lives in `entity.Extras` (a `map[string]any`). **JSON-loaded numbers are
`float64`**; always read numerics tolerantly (the audit fixed a bare
`hp.(int)` that silently disabled starvation for world.json entities).

---

## combat
- **Owns:** HP-based melee. Verbs `attack` (adjacent, weapon dmg+reach),
  `defend` (halves next hit), `heal` (default self).
- **Service:** `CombatService.DealDamage(w, target, amount, cause, killer)` —
  the canonical death path; `vitals` routes starvation through it.
- **Events:** `DamageDealt`, `EntityDied`.
- **On death** it drops the victim's inventory + equipped + **gold (as coin
  items)** at the corpse tile, then removes the body. Gold is conserved
  (audit [0]); coins decompose by denomination via `goldDropDenoms`.
- **State:** `hp`, `max_hp`, `defending`. **Invariant:** no self-attack
  (audit [23]).

## money
- **Owns:** gold balance + transfers. Verbs `pay` (range `pay_max_range_tiles`,
  default 1), `work_for_pay` (requires a worksite/building within
  `worksite_radius`=6), `buy_food` (spend `food_price`=6 to cut hunger by
  `food_relief`=0.5; spatial gate only when `market_radius`>0 — **Eldoria sets
  0, so food is buyable anywhere**).
- **Service:** `MoneyService` (Balance, Pay, Grant).
- **Events:** `GoldTransferred`, `GoldSpent`. **State:** `gold`.

## vitals
- **Owns:** hunger. No verbs — a tick hook raises `hunger` per
  `hunger_per_tick`; above `hunger_damage_above` it drains HP every
  `hunger_damage_interval_ticks`. **Death routes through `CombatService`** so
  it drops loot + emits `EntityDied` like combat (audit [17][18]).
- **Events:** `HungerSpike` (on threshold crossing only). **State:** `hunger`.

## inventory
- **Owns:** per-entity items + equip slots. Verbs `pickup`, `drop`, `equip`,
  `give`, `eat` (food → cuts hunger), `cook` (raw→cooked).
- **Cap:** `DefaultMaxSlots`=10 (D20), enforced on pickup/give/trade AND
  resources (audit [1][6][12] closed the bypasses). **Coins auto-convert to
  gold on pickup — never enter inventory.**
- **Item id format:** canonical `item:<kind>#<unique>` so kind resolution
  (eat/equip/render) works everywhere (audit [9][29]).
- **Events:** `ItemPicked`, `ItemDropped`, `ItemTransferred`, `AteFood`.
- **Service:** `InventoryService` (Items returns a COPY — audit [30]).

## resources
- **Owns:** harvestable nodes. Verbs `chop` (tree→wood), `mine` (rock→stone),
  `forage` (tree/bush→food, renewable after `forage_cooldown`=600). Nodes have
  `hardness` (honors `tree_hardness`/`rock_hardness` tunings — audit [33]),
  deplete, and **regenerate** every `resource_regen_interval`=1800.
- **Events:** `ResourceHarvested`, `ResourceDepleted`. Respects the 10-slot cap.

## property + interiors (HeartGold multi-map)
- **Owns:** buildings as owned/lockable spaces. Verbs `enter`, `exit`, `lock`,
  `unlock`, `claim_ownership`, `transfer_ownership` (new owner must be an agent
  — audit [32]).
- **Two enter paths:** (1) *entity-backed* buildings (from construction) use
  the property handler; (2) *decoration* buildings (Eldoria's `bld:NNN`) warp
  the agent into a **generated interior sub-map** (`interiors.go`
  `GenerateInterior`), where it walks around, then `exit` warps it back to the
  door. Interiors are per-building-instance, lazily created, GC'd when empty
  (`docs/INTERIORS_MULTIMAP_PLAN.md`).
- **Multi-map plumbing:** `MultiMapHub` holds the overworld + interiors;
  `Entity.CurrentMap` tracks each agent's map; the agent hub routes
  obs/actions per-map; the viewer broadcasts interior occupants so the
  frontend renders agents inside.
- **Events:** `EnteredBuilding`/`ExitedBuilding` (paired across both paths,
  audit [7] + phase 5), `BuildingLocked`/`Unlocked`, `OwnershipChanged`.
- **State:** `owner`, `locked`, `access` (no grant verb yet — owner-only,
  audit [8]).

## construction
- **Owns:** building blueprints → buildings. Verbs `place_blueprint`
  (occupancy-checked, audit [10]; unique id via counter, audit [2]),
  `advance_construction` (guarded service assertion, audit [11]), `demolish`
  (no material refund in v1). Materials resolved by item KIND (audit [3]).
- **Events:** `ConstructionStarted`/`Advanced`/`Completed`, `Demolished`.
- **State:** `progress`, `steps_done`, `steps_total`.

## trade
- **Owns:** atomic item↔gold swap with an adjacent partner (`trade`). Payment
  fails → item stays (atomic from caller PoV); respects the buyer's slot cap.
- **Events:** `GoldTransferred`, `ItemTransferred`.

## loot
- **Owns:** `loot` a corpse (HP 0). **Currently unreachable** — combat removes
  the corpse the same tick and drops loot to the ground instead (audit
  [13][36]); kept for future lingering-corpse worlds.

## verbalquests
- **Owns:** emergent verbal contracts. Verbs `propose_task` (any range — by
  design, audit [14]), `accept_task`, `reject_task`, `complete_task`. The
  engine records a ledger on both parties' `contracts` extra (bounded —
  audit [16]) and emits markers, but does **NOT** enforce completion or pay
  rewards. Agents can lie.
- **Events:** `TaskProposed`/`Accepted`/`Rejected`/`Completed`.

## quests
- **Owns:** declarative goals on `entity.quests` (kinds: `reach_tile`,
  `gather_gold`, `kill_target` [needs the target seen alive first — audit [4]],
  `walk_distance` [reads `steps`, written by the step handler — audit [20]]).
  Checked each second; pays gold/HP/item and emits `QuestRewarded` (audit [21]).

## reputation
- **Owns:** a per-agent standing scalar. No verbs — `onDeath` (combat kills
  only, audit [41]) and `onDamage` (non-fatal hits only, audit [39]) lower it;
  it decays toward 0. **Public** — surfaced to others as `reputation` +
  `rep_bucket` (audit [40]).

## respawn
- **Owns:** periodic world respawn — drops food + wealth + tools at walkable
  hub tiles so a long run doesn't strip bare. No verbs.

---

## Verifying it all

`tools/audit/run_all.py` boots a fresh engine and runs the live suites
(verbs / observation integrity / movement / combat-economy e2e / building
interiors / events census / security). Use `tools/audit/restart_sidecar.sh`
to (re)start a clean engine on :8090 first. See
`docs/ENVIRONMENT_AUDIT_PLAN.md` for the full plan + execution log.

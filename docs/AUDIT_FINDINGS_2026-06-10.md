# Substrate hardening — direct-path testing + adversarial audit (2026-06-10)

Two sources: (a) a new live direct-path chain suite (`tools/audit/paths_e2e.py`,
S9) that drives full multi-step flows and asserts acks AND emitted events;
(b) an adversarial audit workflow (per-subsystem hunters + 2-skeptic
verification). The workflow completed 3 of 16 subsystems before rate limits;
see "Remaining" below for the resume point.

## Fixed (merged via `audit-hardening`, each verified build+test+live)

1. **Inventory id rewriting broke every transfer verb** (root cause of the
   pilot study's 87/87 failed `give` attempts). Pickup stores
   `item:<kind>#<entity_id>` but give/drop/eat/equip/cook/trade demanded that
   exact form, while agents reference the ground-entity id they SAW.
   → `resolveItemRef` accepts canonical id / ground-entity id / bare kind;
   service `Resolve` added; all six verbs canonicalize. Unit tests in
   `resolve_test.go`; proven live (C2 give→ItemTransferred, drop→ItemDropped).
2. **pickup conflated unknown targets with non-items** — returned
   `not_an_item` (undeclared for that case) instead of `unknown_target`,
   misleading LLM agents ("no item here").
3. **wire: `rate_limited` acks carried no action_id** — could never resolve
   the SDK's pending future; wait-for-ack calls stalled the full timeout.
   Now echoes action_id/verb like every other rejection path.
4. **wire: same-secret reconnect was one-shot** — a stomped connection's
   teardown unconditionally purged the registry secret its live replacement
   was still using; the next reconnect got `auth_invalid` forever. Delete is
   now guarded by `stillOwner`.
5. **baselines (HIGH): bots froze at the 10-slot inventory cap** —
   non-monetary pickups were re-issued forever against `inventory_full`.
   The base bot now drops its lowest-priority slot (junk → weapons → food)
   to make room. Also: per-bot RNG now seeded via sha256 (built-in `hash()`
   is process-salted; documented determinism was silently broken).
6. **sdk: engine error frames were swallowed** — `auth_invalid` produced a
   silent empty observation stream. Now logged + raised from
   `observations()`. `BuyFood` added to `__all__`. Stale doc contracts fixed
   (ActionBatch does NOT drop the rest of the batch on failure; `act()`
   raises on timeout).

## Confirmed, NOT yet fixed (FSM behavior changes — do as one careful batch)

- Scavenger RACING: no timeout / unreachable-goal handling (scream pos can be
  a non-walkable tile → freeze).
- Survivor: FLEEING unconditionally preempts DESPERATE; threat trigger is
  whole-vision-radius, not the documented 5 tiles (starving survivors flee
  instead of eating).
- Manipulator: threat list excludes its own mark, so "target fights back →
  FLEEING" can never fire; APPROACHING parks at a stale last-seen tile up to
  ~30s; `_betrayal_spoken` latch never resets.

## Design call needed (the biggest finding)

**~⅓ of the declared action space is dead in the shipped world.** Eldoria has
zero `tree`/`rock`/`bush` and zero `building` ENTITIES (buildings are
decorations + a door registry; `enter` works via the door warp). Therefore
`chop`, `mine`, `forage`, `claim_ownership`, `lock`, `unlock` and the
construction material loop can NEVER succeed — yet the rulebook prompts
agents with them (persona "Sela the homesteader"'s standing goal is
literally impossible). Confirmed live: chain suite C4–C7 skip with "no
tree/building"; `ResourceHarvested` has never appeared in any run's events.
Options: (a) spawn real resource-node + claimable-building entities (richer
emergence: adds scarcity sites; needs render care to not double-draw over
decorations), or (b) strip the unsatisfiable verbs from eldoria's rulebook
(honest, simple, less depth). Taste call — not made unilaterally.

## Remaining

- Engine-core audit (13 subsystems: world-core, observation, concurrency,
  combat, money/trade, inventory/loot, resources/respawn, property/
  construction, quests, reputation/vitals, rules-main, wire-viewer,
  tools-metrics) never ran — rate limits. Resume:
  `Workflow({scriptPath: <session>/workflows/scripts/substrate-full-audit-wf_78151fe8-48c.js, resumeFromRunId: "wf_78151fe8-48c"})`
  (cached results return instantly; worktree `~/projects/agent_sim_audit`
  kept for this purpose).
- Disputed findings (verifiers split, no action): minimap stall glyph dead
  code; view_image omitted from the hand-rolled WS observation map (1/2
  said real); audible 240-tick reprocessing without event_id dedup;
  HungerSpike dropped when `hunger_damage_interval_ticks > 1` (verification
  never completed — re-verify).
- S9 chain suite is wired for the dead paths (C4–C7) — they unskip
  automatically once the world ships the entities.

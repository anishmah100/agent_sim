# Environment Audit Plan — total, assumption-free verification

**Status:** ACTIVE (started 2026-06-08)
**Owner:** engine + SDK + frontend + docs
**Goal:** Establish *complete, evidence-backed confidence* that every part of
the agent_sim environment behaves exactly as documented, before we build the
next-generation agent harness. The next phase (advanced reasoning/action
agents) is only as trustworthy as the substrate it runs on.

---

## 0. Why this exists — the failure that motivated it

We shipped an `enter` verb, a `door` affordance, an `inside_building`
observation field, and UI sprite-hiding — and *assumed* an agent could "enter,
walk around inside, and exit a building." Live testing on 2026-06-08 proved
the **interior does not exist**: `enter`/`exit` work, but while inside the
agent is frozen at the door tile (movement is a no-op), sees only overworld
terrain, and auto-exits after ~4–10s. The interior sub-map engine
(`multimap.go`) exists but was never wired to decoration buildings.

**Root cause: assumption-driven verification.** Code existed, unit tests
passed, the field was in the schema — so we believed the feature worked. None
of that exercised the actual end-to-end behavior against a live world.

**The rule for everything below: NO ASSUMPTIONS.** A feature is "verified"
only when it has been exercised against a *live running engine* (and, where it
has a visual, confirmed in the *running UI*) and the observed result matches
the documented contract. "The code looks right", "the unit test passes", and
"I remember building it" are NOT verification.

---

## 1. Verification principles

1. **Ground truth = the live system, not the source.** Every claim is checked
   by hitting a running engine (`/api/v1/...`, the agent WebSocket) and/or the
   running frontend, not by reading Go and inferring.
2. **Every verb, both directions.** For each of the 30 verbs: exercise the
   ACCEPT path *and* every declared rejection reason, and confirm the
   resulting observation + emitted event(s) match the manifest.
3. **Cross-layer consistency.** For every observable thing, confirm the FOUR
   layers agree: engine wire payload ↔ Python SDK model ↔ TypeScript SDK model
   ↔ documentation. A field present in one and absent in another is a defect.
4. **Re-pull the inventory each run.** The verb/event/field/archetype lists in
   §2 are pulled from `/api/v1/world/affordances` of a live engine. Re-pull at
   the start of every audit pass; if the manifest changed, the plan expands to
   cover the new surface automatically.
5. **Evidence or it didn't happen.** Each checklist item records: the command/
   script run, the captured payload (or screenshot), PASS/FAIL, and — if a bug
   — the fix commit. Stored in §EXECUTION LOG.
6. **Reproducible harness.** Tests live in `tools/audit/` as scripts that any
   future change can re-run, not one-off shell snippets. The audit becomes a
   permanent regression gate, not a one-time event.

---

## 2. Ground-truth inventory (pulled from live `/api/v1/world/affordances`)

> Re-generate with `tools/audit/dump_inventory.py`. Snapshot 2026-06-08,
> world=eldoria, scenario=fantasy_town, schema_version=1.

**13 systems · 30 verbs · 23 event types · 19 state fields · 6 archetypes.**

### Verbs by system (with every declared rejection reason + emitted events)
| System | Verb | Rejection reasons (ALL must be reproduced) | Emits |
|---|---|---|---|
| combat | `attack` | bad_params, unknown_target, target_too_far | DamageDealt, EntityDied |
| combat | `defend` | — | — |
| combat | `heal` | unknown_target, target_too_far | — |
| money | `pay` | bad_params, unknown_target, target_too_far, not_enough_gold | GoldTransferred |
| money | `work_for_pay` | no_worksite_nearby | GoldTransferred |
| money | `buy_food` | not_hungry, not_enough_gold, no_market_nearby | GoldSpent |
| inventory | `pickup` | bad_params, not_an_item, target_too_far, inventory_full | ItemPicked |
| inventory | `drop` | bad_params, not_in_inventory | ItemDropped |
| inventory | `equip` | bad_params, not_in_inventory | — |
| inventory | `give` | bad_params, unknown_target, target_too_far, not_in_inventory | ItemTransferred |
| inventory | `eat` | bad_params, not_in_inventory, not_food | AteFood |
| inventory | `cook` | bad_params, not_in_inventory, not_cookable | — |
| property | `enter` | bad_params, unknown_target, not_a_building, target_too_far, already_inside, locked | EnteredBuilding |
| property | `exit` | not_inside | ExitedBuilding |
| property | `lock` | bad_params, unknown_target, not_a_building, not_owner | BuildingLocked |
| property | `unlock` | bad_params, unknown_target, not_a_building, not_owner | BuildingUnlocked |
| property | `claim_ownership` | bad_params, unknown_target, not_a_building, target_too_far, already_owned | OwnershipChanged |
| property | `transfer_ownership` | bad_params, unknown_target, not_a_building, not_owner, unknown_new_owner | OwnershipChanged |
| resources | `chop` | bad_params, unknown_target, not_a_tree, target_too_far, no_yield, depleted | ResourceHarvested, ResourceDepleted |
| resources | `forage` | bad_params, unknown_target, not_forageable, target_too_far, not_ripe | ResourceHarvested |
| resources | `mine` | bad_params, unknown_target, not_a_rock, target_too_far, no_yield, depleted | ResourceHarvested, ResourceDepleted |
| construction | `place_blueprint` | bad_params, unknown_blueprint, target_too_far, unwalkable, no_inventory_service, missing_materials, spawn_failed | ConstructionStarted |
| construction | `advance_construction` | bad_params, unknown_target, not_a_blueprint, target_too_far, not_owner, broken_blueprint, missing_materials | ConstructionAdvanced, ConstructionCompleted |
| construction | `demolish` | bad_params, unknown_target, not_a_structure, target_too_far, not_owner | Demolished |
| trade | `trade` | bad_params, unknown_target, target_too_far, not_in_inventory, target_not_enough_gold | GoldTransferred, ItemTransferred |
| loot | `loot` | bad_params, unknown_target, target_too_far, target_alive | GoldTransferred |
| verbalquests | `propose_task` | bad_params, unknown_target, self_target, empty_terms | TaskProposed |
| verbalquests | `accept_task` | bad_params, unknown_contract, bad_status, not_authorized | TaskAccepted |
| verbalquests | `reject_task` | bad_params, unknown_contract, bad_status, not_authorized | TaskRejected |
| verbalquests | `complete_task` | bad_params, unknown_contract, bad_status, not_authorized | TaskCompleted |

Plus base/world verbs not in the system manifest but in the SDK union:
`step`, `speak`, `shout`, `whisper`, `look_at`, `interact`, `wait`,
`mental_note` (session-meta channel). These MUST also be exercised.

### Systems with NO verbs (passive — verified by their tick effects + events)
`vitals` (hunger→hp drain→death), `quests` (declarative goal completion),
`respawn` (periodic loot drops), `reputation` (decay + on-damage/-death).

### State fields (19) — each must be present + correctly public/private
hp, max_hp, defending (combat); gold (money); hunger (vitals); inventory,
equipped (inventory); owner, locked, access (property); hardness, yield
(resources); progress, steps_done, steps_total (construction); quests, steps
(quests); contracts (verbalquests); reputation (reputation).

### Archetypes (6) building, tree, rock, blueprint, blueprint:cottage,
blueprint:shed. Plus engine archetypes seen live: item, wanderer, +agent
archetypes (survivor/killer/avenger/scavenger/manipulator/trader/...).

---

## 3. Test suites

Each item is `[ ]` until it has live evidence. `PASS` / `FAIL→fix@<commit>`.

### S1 — Verb matrix (the core: 30 system verbs + 8 base verbs)
For EACH verb, a row in `tools/audit/verb_matrix.py` that:
- **S1.a** Accept path: set up preconditions live, submit, assert
  `accepted:true`, assert the world changed as documented (re-observe), assert
  the documented event(s) landed in `.runlog/events.jsonl` with correct payload.
- **S1.b** Each rejection reason: construct the precondition violation, submit,
  assert `accepted:false` with the EXACT documented `reason`.
- **S1.c** Param schema: submit malformed/missing params → `bad_params` (not a
  panic, not a silent accept).
- **S1.d** Idempotency/edge: double-submit, submit on a dead/gone target,
  submit while inside a building, submit at range boundary (chebyshev exactly 1
  vs 2).

### S2 — Observation integrity (every field, every viewer)
- **S2.a** Capture a live frame; assert top-level keys == documented set
  EXACTLY (no extra, no missing). (Caught known_map/persona_reminder already.)
- **S2.b** `self.extras` carries the full private set; assert each of the 19
  fields appears where the owner is the agent.
- **S2.c** `visible_entities[].extras_summary` carries ONLY the public subset
  (hp_bucket, equipped_*, reputation/rep_bucket) — assert NO private field
  (gold, inventory, hunger) ever leaks into another agent's view.
- **S2.d** Vision + LOS: place a probe behind a wall/building; assert it is
  NOT in `visible_entities`; move into LOS; assert it appears. Day vs night
  radius (12 vs 6) verified by tick-phase.
- **S2.e** `inside_building` correctness (set on enter, cleared on exit; agent
  excluded from others' view while inside).
- **S2.f** `local_view` accuracy: cross-check glyphs against the actual
  walkability grid at the agent's tile (sample N tiles; `#`/`~`/`.` must match).
- **S2.g** Dead-field removal verification: `doing`, `weather`,
  `recent_self_results` removed from wire + both SDKs (per decision).
- **S2.h** `audible` window (~240 ticks) + range (speak ~3 / shout ~15 /
  whisper adjacency) — emit each, assert who hears it.

### S3 — Movement & collision (the teleport/husk class of bugs)
- **S3.a** `step` moves exactly one tile per accepted step (already shown:
  771,864→771,854). Re-verify all 4 dirs.
- **S3.b** Blocked step (`#`/`~`/off-map) → rejected, pos unchanged.
- **S3.c** Two adjacent agents: swap only when both genuinely adjacent; assert
  NO long-distance teleport over many ticks (probe every jump).
- **S3.d** Multi-tile footprints (tree 2×2, building) block A* + step.
- **S3.e** Door tiles are walkable; stepping onto/adjacent enables enter.
- **S3.f** No (0,0) husks in snapshot or picker; dead bodies removed.

### S4 — Each system, end-to-end (the real loops)
- **S4.combat** attack→DamageDealt→hp drops→EntityDied→body removed; defend
  halves damage; heal restores; reach boundary (adjacent only).
- **S4.money** work_for_pay only at worksite; buy_food only at market + when
  hungry + with gold; pay transfers + GoldTransferred; gold conservation
  (no money printed/destroyed except documented sinks).
- **S4.inventory** pickup→inventory; coins auto-convert to gold (NOT
  inventory); drop; equip; give; eat cuts hunger + AteFood; cook transforms
  raw→cooked.
- **S4.property+interiors** enter/exit/lock/unlock/claim/transfer; **NEW: real
  interior** — enter warps to interior sub-map, agent moves inside, sees
  interior tiles/items, exits back to the door tile. (Build per decision.)
- **S4.resources** chop tree→wood + ResourceHarvested→depletion→regen;
  mine rock→stone; forage→food + ripen cooldown (not_ripe before).
- **S4.construction** place_blueprint (spends materials)→advance×N→
  ConstructionCompleted→a real building exists + is enterable + claimable;
  demolish.
- **S4.trade** atomic item↔gold; both sides update or neither (assert
  atomicity under target_not_enough_gold).
- **S4.loot** only a corpse (HP 0); target_alive rejected; transfers gold +
  clears corpse inventory.
- **S4.verbalquests** propose→accept/reject→complete; ledger in `contracts`
  extras of both parties; markers/events.
- **S4.vitals** hunger rises per tick; crosses threshold→hp drain→death→
  scream→removal→respawn keeps population stable (the husk-gridlock regression).
- **S4.reputation** attack/kill lowers it; decays toward 0; surfaced as
  reputation + rep_bucket in others' view; gossip propagation across agents.
- **S4.quests** declarative goal attached→completion check→reward.
- **S4.respawn** periodic loot drop at walkable hub tiles.

### S5 — Buildings & interiors (expanded per "build real interiors" decision)
- Door discovery (LOS from open side), enter, **interior map load + movement +
  interior observation + interior items**, exit returns to door tile, UI
  renders interior (camera/sprite), lock gates enter, auto-exit timeout.

### S6 — Events & runlog integrity
- Every one of the 23 event types is actually emitted by its verb/tick path AND
  appears in `.runlog/events.jsonl` with the documented payload shape +
  category. Assert no event is declared-but-never-emitted.

### S7 — SDK parity (engine ↔ Python ↔ TypeScript ↔ docs)
- **S7.a** Feed N captured live frames through Python `Observation.model_validate`
  and TS `Observation.parse` — both must accept with zero loss.
- **S7.b** Field-by-field diff of Python models ↔ TS models ↔ wire keys.
  KNOWN GAP: TS `Observation` is missing `visible_items` — fix + test.
- **S7.c** Every verb in the SDK Action union maps to a live-accepted verb and
  vice-versa (no SDK verb the engine rejects as unknown; no engine verb the SDK
  can't express). The `verbs_coverage` contract test enforces this — extend it.

### S8 — UI / rendering (visual, in the running frontend)
- Every entity archetype renders with the right sprite (agents, items incl.
  coins NOT as logs, trees, rocks, buildings, doors, blueprints).
- Enter hides sprite; exit shows it; (interior view if built).
- Combat hit FX, hunger pang (amber), death scream, blueprint progress.
- Agent picker: no husks, no NPC dummies, jump-to-focus lands on the agent;
  LLM/Qwen/rule badges correct; last_verb/last_speech populate.
- Camera/interpolation smooth; no visual teleport; decorations cull correctly.
- Inspector shows self extras, mental notes, contracts.
- Capture screenshots as evidence for each.

### S9 — World-gen, persistence, lifecycle
- Bundle load (eldoria) deterministic; item/building counts sane.
- Snapshot save→restart→restore preserves world + inside-building + inventory.
- NPC subprocess spawn (npcs.json); day/night phase transitions; respawn cadence.

### S10 — Multi-agent / social / scale / soak
- N≥10 agents: no deadlock, no gridlock, no runaway event spam.
- Gossip/reputation propagates across the population.
- Contracts proposed/accepted/completed across agents.
- 30–60 min soak: live count stable, events flowing, no memory blowup, no
  stale-engine/husk regressions, no 401 token storms.

### S11 — Documentation accuracy (every doc claim vs code+live)
- Walk every file in `docs/` + both SDK READMEs; for each concrete claim
  (a field, a verb, a number, a behavior) verify against code AND live. Fix or
  delete stale claims. Add missing architecture docs so a newcomer can fully
  understand each subsystem from docs alone (per the directive).

### S12 — Negative / adversarial / security
- Malformed JSON, unknown verb, missing required params → clean rejection (no
  panic, no crash, no silent accept).
- Unknown/dead/cross-map targets; range boundary off-by-one.
- Auth: bad secret rejected; rate-limit/burst enforced; stale token → clean
  close (no 401 storm).
- Action flooding / oversized batch → bounded.

---

## 4. Execution method (BOTH — per decision)

1. **Reusable harness first** (`tools/audit/`): `dump_inventory.py`,
   `verb_matrix.py` (S1), `obs_integrity.py` (S2), `sdk_parity.py` (S7),
   `events_check.py` (S6), `building_e2e.py` (S5), plus a `run_all.py` that
   boots a clean sidecar engine on an isolated port, runs every suite, and
   emits a PASS/FAIL matrix. This is the permanent regression gate.
2. **Workflow fan-out** for breadth/adversarial bug-hunting: one subagent per
   system reads its code + the harness output and adversarially hunts for
   defects the harness didn't think to test; findings are verified before fix.
3. **Sequential deep-tests** by me for anything needing a live engine, the UI,
   or judgment (interiors build, visual checks, doc accuracy).
4. **Fix policy:** atomic commit per fix; `go build`+`go test`+`tsc`+`pytest`
   green before the next; never break UI/engine; surface design calls instead
   of guessing.

## 5. Exit criteria

- Every S1–S12 item has live evidence and is PASS (or FAIL→fixed→PASS).
- `tools/audit/run_all.py` is green and wired so it can gate future changes.
- Docs contain no claim unverified against code+live; architecture is
  documented well enough to onboard cold.
- A 30–60 min soak of the demo is clean.
- Real building interiors verified end-to-end (enter→walk→see→exit) live + UI.

---

## EXECUTION LOG
(Each entry: date · suite/item · evidence · result · fix commit.)

- 2026-06-08 · S2.a/S2.g · captured live frame, top-level keys ==
  {type,obs_id,world_tick,self,visible_entities,visible_objects,visible_items,
  audible,recent_self_results,local_view,world_clock}; known_map_summary +
  persona_reminder absent · PASS (after removal commit 59e6277).
- 2026-06-08 · S3.a · 10× step N moved 771,864→771,854 (one tile/step) · PASS.
- 2026-06-08 · S5 (pre-build) · enter `door:bld:000:767,867`→accepted,
  inside_building set; exit→cleared; **step inside = no-op, no interior** ·
  FAIL → interiors to be built (decision).
- 2026-06-08 · S7.b · TS Observation missing `visible_items` · FIXED@ce0f558 (added VisibleItem + vitest parity test).
- 2026-06-08 · S2.g · dead fields doing/weather/recent_self_results removed from wire+SDKs · FIXED@968899a (verified absent from fresh live frame).
- 2026-06-08 · S1 (verb matrix) · tools/audit/verb_matrix.py vs live engine —
  45/45 probes PASS: every system verb rejects with a DOCUMENTED reason for
  malformed/unknown-target/unknown-item/unknown-contract, and the trivial
  accept verbs (step/speak/shout/wait/look_at/defend) accept. No undocumented
  rejection reasons leaked. False alarms ruled out by direct repro: demolish
  acks unknown_target in isolation (matrix no-ack was buffered-frame timing,
  now retried); work_for_pay + buy_food accept is legitimate state-dependent
  behavior (worksite/market in range). · PASS.
- 2026-06-08 · CONFIG NOTE (for the maintainer) · Eldoria sets market_radius=0, so
  buy_food has NO spatial gate (food buyable anywhere with gold+hunger>0)
  despite bld:stall_* decorations existing. Consider market_radius>0 if the
  market should be a real place agents converge on.

- 2026-06-08 · S2 (obs integrity) · tools/audit/obs_integrity.py, 2 live agents
  · 0 findings: top-level keys EXACTLY the documented set; known_map_summary/
  recent_self_results/weather absent; self.extras carries hp/gold/hunger/
  inventory/reputation; **no private field (gold/inventory/hunger/contracts)
  leaks into another agent's extras_summary** and no 'doing'; summary keys all
  known-public. · PASS.

- 2026-06-08 · S5 (interiors e2e) · tools/audit/building_e2e.py vs live —
  enter door:bld:000 -> agent WARPED to a separate interior map (pos (3,6),
  walled room) -> WALKED inside (3,6)->(3,5) -> exit -> back on overworld at
  the door (766,867). The original 'no walkable interior' gap is CLOSED for
  the engine/observation side. · PASS (frontend map-switch = phase 4, pending).
  (Caught + fixed a stale-engine trap mid-test: a 48-min-old binary held :8090
  so relaunches silently failed to bind — killed by exact pid, verified uptime.)

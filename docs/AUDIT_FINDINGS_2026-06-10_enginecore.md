# Engine-core adversarial audit — 2026-06-10 (run wf_7f280820-467)

16-subsystem hunt + 2-skeptic verification against current master. 31 candidates,
**11 confirmed** by both skeptics; ~20 more candidates whose verification was cut
off by session limits (votes 0/0) and must be **re-verified, not dismissed**.

## Confirmed — FIXED (11/11) ✓

| sev | area | fix commit |
|---|---|---|
| HIGH×3 | equip left item in BOTH inventory+equipped → item duplicated on death, phantom equipped weapon after drop/give, and `"hand"` slot gave zero combat benefit (combat reads `equipped["weapon"]`) | `5dc775f` — equip MOVES item to `equipped["weapon"]` (disjoint); conservation regression test |
| HIGH | stomped reconnect cleared `PlayerControlled` unconditionally → reconnected agent wandered autonomously instead of obeying its bot | `d52b045` |
| MED×2 | `trySend` TOCTOU could send on a closed channel and panic the SHARED observation loop (kills observations for every agent); same in viewer | `d52b045` (recover) |
| MED | `HungerSpike` crossing gated behind the damage-interval → ~99.7% of starvation-zone crossings dropped | `c994174` |
| MED | `catalog.py` shout_count queried a `Shout` kind the engine never emits (shouts = `Speech`+`Mode='shout'`); speech absorbed shouts | `c994174` |
| MED | dead `shout_muffle_radius` tuning (read by no system; falsely advertised garbling to agents) | `c994174` (removed + regen rulebook) |
| LOW | loot manifest under-declared `not_a_target` / `not_enough_gold` | `c994174` |
| LOW | world/info + startup log reported empty legacy `-scenario` flag, not the effective scenario | `c994174` |

## Confirmed — OPEN: none

- ~~MED `wire/agent.go:310` register-without-connect leak~~ — **FIXED `6866b95`**:
  `RegisteredAt` stamp + a 30s reaper evicts never-connected registrations past
  a 60s grace window and removes their leaked auto-spawned bodies. Unit-tested.

## From the cut-off batch — manually verified + FIXED (4)

The re-verification workflow was throttled (session, then transient server rate
limits) twice, so I hand-verified the highest-signal candidates from code:
- **HIGH `perception.go` audible-ring data race** — FIXED `67ffcdc` (RLock).
- **HIGH `perception.go` inside-building speech leak** (identity + door pos) —
  FIXED `67ffcdc` (walls block sound; regression test).
- **MED `quests.go` HP reward kills agent w/o max_hp** — FIXED `0b37614`.
- **LOW heal manifest reasons** — FIXED `0b37614`.

## From the backlog — hand-verified + FIXED (round 2, 7 more)

- **SDK `client.py` UnboundLocalError on the engine-error-frame branch** (the
  earlier loud-auth fix never actually fired) + drain loop swallowed
  `_fatal_error` — FIXED `9843117`.
- **MED `world.go:1041` auto-exit two-on-one-tile** — FIXED `870c127` (relocate to
  nearest free tile).
- **MED `resources.go` forage/harvest id collisions** (alias on give/trade) —
  FIXED `870c127` (actor-id suffix).
- **MED `aoi.go` SnapshotForChunks Extras race** — FIXED `870c127` (copyExtras).
- **MED `motor_loop.py:96` salient-audible re-fires → LLM cost** — FIXED `1135921`
  (de-dup by event_id).

## Remaining backlog (low-priority / inert in eldoria)

- **`construction.go:158`** (stale spatial index), **`action.go:208`**
  (`interact{enter}` legacy flag), runtime-building-unwalkable — all
  building/construction mechanics that **don't fire in eldoria** (no building
  entities). Tied to the **dead-verbs design call**; fix when that's resolved.
- **MED `agent.go:531`** — no WS read/write deadline / ping keepalive (half-open
  sockets accumulate over a 24/7 run). The reaper already bounds never-connected
  leaks; this is connected-then-half-open. Moderate socket-layer hardening.
- LOW: `reject_task` can't cancel a proposal; baseline-FSM sticky-state labels
  (Manipulator DEFECTING_SILENT, Killer RETREATING) — rule-bot behavior, not data
  fidelity.

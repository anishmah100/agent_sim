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

## Unverified — NEED RE-VERIFICATION (session-limit cutoff)

High-signal candidates the verifier never reached; re-run before trusting:
- **HIGH `perception.go:127`** — viewer broadcast reads `w.audible` with no lock
  while Tick mutates it under the write lock → data race on the audible ring.
- **HIGH `perception.go:146`** — inside-building agents are concealed from sight,
  but their speech/whisper/shout leaks identity + exact door position to the open
  world (perception/privacy-fidelity — directly relevant to the correctness pass).
- **HIGH `construction.go:158`** — `place_blueprint` occupancy check reads a stale
  spatial index never updated on movement.
- **HIGH `action.go:208`** — `interact{enter}` uses the legacy `InsideBuilding`
  flag instead of the portal warp → bot may get stuck inside.
- **MED `quests.go:163`** — quest HP reward clamps HP to 0 when no `max_hp` extra
  (reward kills the agent).
- **MED `world.go:1041`** — auto-exit can place two entities on one tile (occupants
  invariant) (1/2 split).
- **MED `agent.go:531`** — no WS read/write deadline or ping keepalive → half-open
  sockets leak the entity/slot.
- **MED `agents/llm/motor_loop.py:96`** — salient-audible deliberation re-fires for
  the full 240-tick window (LLM token waste).
- Plus several LOW (forage/harvest non-unique item IDs, runtime buildings not
  marking tiles unwalkable, reject_task can't cancel, FSM sticky-state labels).

Re-verify: `Workflow({scriptPath: <audit script>, resumeFromRunId: "wf_7f280820-467"})`
(cached confirmed agents return instantly; only the cut-off verifiers re-run).

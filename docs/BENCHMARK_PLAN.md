# Benchmark plan — living design doc (started 2026-06-10)

Direction: pivot from "one mega persistent world" to a **battery of small,
capability-isolating games** on one shared spatial substrate, used to evaluate
how frontier models play. Findings-first (not a benchmark *platform* — see the
landscape research: Kradle/Emergence already occupy the platform thesis; the
open wedge is sharp, reproducible, reasoning-trace-transparent findings).

## Decisions locked this session

- **Game battery = 4 capabilities:** power-acquisition/entrenchment (flagship,
  least-crowded), deception/social-deduction, negotiation/bargaining,
  cooperation-vs-defection (public goods / commons). Sequence them; don't build
  all at once, but the architecture must accommodate all four.
- **One spatial substrate, embed everything.** Even the non-spatial-ish game
  (social deduction) lives in the tile world — agents physically gather, deceive
  via existing whisper/contract channels, and can also hunt each other. Keeps the
  Pixi viewer + reasoning-trace overlay (the visual/clip moat) working for *every*
  game. "Werewolf where you can also physically kill the suspect" is a feature.
- **The bar = mechanism correctness + visual fidelity, NOT literal zero-bugs.**
  Two things the substrate must never lie about:
  1. **Data fidelity:** `action → world-effect → correct-perception-by-others →
     faithful event/metric`. The nightmare it guards against: a dropped whisper
     that *looks like* a real deception finding. This is the #1 priority.
  2. **Visual fidelity:** `world-state → render`. For a benchmark people watch as
     it runs, a misrendering (e.g. the known invisible-combat bug) is as fatal as
     a miscounted metric.
- **Sequencing:** core-correctness pass on the shared engine FIRST (game-agnostic,
  never wasted, targets the false-finding nightmare), THEN build game #2 as the
  extensibility proof + second finding.

## Extensibility seam — what "a game" is

A game = parts on top of the spatial engine: (1) a bundle (map + scenario setup),
(2) an enabled verb/system subset, (3) a set of installed **observers**
(measurement — see below), (4) OPTIONALLY a derived objective/win-condition,
(5) an agent action-menu + per-agent goals (goals may be heterogeneous). Making
these clean-to-define is the whole extensibility task. Test of the abstraction:
building game #2 — every place we're forced to touch engine-core instead of just
adding a system + bundle + observers is a logged abstraction leak. "Extensibility
you haven't exercised is extensibility you don't have."

### Measurement: observers, NOT a mandatory scorer (decided 2026-06-10)

Not every game has an explicit win condition, and agents may optimize different
objectives. So measurement is a **library of composable observers** — each a
read-only function over the action/event/perception stream — rather than one
per-game scalar score (which would hit the score-interpretability critique: "what
does winning in an open world even mean?"). Most observers are game-agnostic and
reusable: Gini, wealth, relationship-graph, communication-topology
(whisper/shout/broadcast ratio — the pilot's headline metric), combat, contract,
survival, soft-power (trust/favor-graph centrality). A game installs a subset +
optional game-specific observers. A win condition, when present, is just a derived
observer, never required. Agents' own goals live in the persona, decoupled from
measurement. (This generalizes what the pilot already did — the credible finding
was the multi-dimensional fingerprint, not a single number.) The **runtime referee
is a fidelity-observer in this same family** — observers measure behavior, the
referee checks faithfulness; both are read-only stream consumers.

## PARKED — determinism / fairness (revisit before first comparative run)

Decision: do **not** build a deterministic/lockstep engine speculatively.
Reasoning: reproducibility (bit-identical replay) genuinely doesn't matter much —
the LLM is stochastic, and run-many-and-average with confidence intervals is the
standard. The ONE thing averaging can't fix is **systematic bias** (≠ variance):
today same-tick contention is resolved by network *arrival order*, so a
faster-latency model could systematically win contested resources — a substrate
artifact masquerading as a capability finding. But this is plausible, not
measured. **Resolution: add a SYMMETRY TEST to the correctness pass** — same model
(or two identical models) in both competitive seats, many runs; if outcomes are
~50/50 the confound is absent and averaging suffices; if one seat/connection
systematically wins, fix with the *lightest* control (a deterministic in-world
tiebreak for same-tick conflicts + randomized seat assignment), not full lockstep.
Engine RNG is already fixed-seeded (`world.go` NewPCG(1,2), `respawn.go` (7,31)),
so randomness is not a source.

## Extensibility assessment (code-grounded, 2026-06-10)

**~70% there.** The hard part is built: a real `Scenario` interface
(`Name/Verbs/Handler/OnEntitySpawn/OnTick`) + truly composable systems
(`systems/<name>`, each scenario's `Install()` picks its system subset). So the
enabled-system set is already per-game (a negotiation game installs
money+trade+inventory+reputation, not combat+construction). New verb = ~one line
through the registry. Adding a spatial game = new bundle + scenario picking its
systems + optional new system module.

The missing 30% — exactly what a benchmark needs:
1. **Objective + scoring aren't first-class.** `Scenario` has no `WinCondition()`
   / `Score()`; scoring lives in `tools/metrics/score_run.py`, hardcoded to
   Eldoria metrics. Each capability-game's objective+metric is the point of the
   game and must become part of the game definition.
2. **Agent isn't game-aware.** `ACTION_MENU` in `agents/llm/prompt.py` is a
   hardcoded Eldoria verb list, not derived from the world's advertised
   `Verbs()`; personas/goals are Eldoria-shaped too. Agent must read menu+goal
   from the game.

Both gaps are fixed by building game #2 (power-acquisition): a `Game` =
Scenario + WinCondition + Scorer + agent-facing menu/goal. The abstraction is
"done" when game #2 drops in without touching engine core (also Phase-2 Starlark
scenarios were planned but not built — today a new game is a Go package + recompile).

## The tape (single source of truth) — decided 2026-06-10

All measurement (observers) and verification (referee) read one append-only
stream: `actions → world-effects → per-agent perceptions → events`. Stream-pure
consumers run LIVE (viewer + live observers + referee) and OFFLINE (analysis,
retroactive observers, replay) off the same record. **Gap found in code:** the
current tape is world EVENTS only (`-event-log` via the historian + reasoning
traces + narrator). Per-agent PERCEPTIONS are computed and sent over WS but NOT
recorded — so the referee cannot check "did B receive the whisper" today.
**Phase-0 task #1 = complete the tape: log each agent's delivered observation.**
The engine already computes it; just persist it. One add unlocks the
perception-fidelity gap + the referee + stream-pure observers.

## Phase 0 — correctness pass — COMPLETE ✓ (2026-06-10)

1. ✅ **Tape** — engine logs per-agent delivered perceptions (`-log-perceptions`).
2. ✅ **Chain matrix** (`tools/audit/chain_matrix.py`) — per-verb four-link
   fidelity incl. the perception link. 9/9 pass.
3. ✅ **Invariants** (`tools/audit/invariants.py`) — enter/exit pairing, gold
   source/sink integrity, no post-mortem activity. CLEAN.
4. ✅ **Visual oracle** (`tools/audit/visual_oracle.py`) — viewer frames match
   engine state (position fidelity + no teleport). All pass. (Pixel-level
   animation, e.g. BLK-1 combat, is a separate manual/Playwright pass.)
5. ✅ **Referee** (`tools/audit/referee.py`) — offline fidelity-observer:
   whisper delivery, perception provenance, confidentiality. CLEAN. The
   credibility instrument for published findings.
6. ✅ **Symmetry test** (`tools/audit/symmetry_test.py`) — SETTLED the parked
   fairness Q: two identical agents contend per-tick, A-first 0.53 → SYMMETRIC,
   no arrival-order bias. No lockstep needed; averaging suffices. (Normalizing
   for cross-model inference latency stays a benchmark-design choice for later.)
7. ✅ **Engine-core audit** — 22 bugs found + fixed (see
   `AUDIT_FINDINGS_2026-06-10_enginecore.md`).

## Open / next (Phase 1+, after correctness pass)
- Game #2 = power-acquisition (flagship). Doubles as the extensibility proof.
- Known fix backlog carried in: invisible combat (visual-fidelity break), 3 FSM
  mediums, dead verbs in Eldoria (chop/mine/forage/property have no entities).

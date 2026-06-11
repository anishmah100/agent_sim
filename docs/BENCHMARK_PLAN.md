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

A game = five parts on top of the spatial engine: (1) a bundle (map + scenario
setup), (2) an enabled verb/system subset, (3) a win condition, (4) a scoring
function, (5) an agent action-menu. Making those five clean-to-define is the whole
extensibility task. Test of the abstraction: building game #2 — every place we're
forced to touch engine-core instead of just adding a system + bundle is a logged
abstraction leak. "Extensibility you haven't exercised is extensibility you don't
have."

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

## Open / next

- Correctness-pass methodology (the two fidelity chains): per-verb live assertion
  matrix + a runtime "referee" + resume the 13-subsystem adversarial code audit
  (run id `wf_78151fe8-48c`) + the symmetry test above.
- Game #2 = power-acquisition (flagship). Doubles as the extensibility proof.
- Known fix backlog carried in: invisible combat (visual-fidelity break), 3 FSM
  mediums, dead verbs in Eldoria (chop/mine/forage/property have no entities).

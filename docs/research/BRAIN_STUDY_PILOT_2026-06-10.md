# Cross-brain pilot study — 2026-06-10

**Question:** do different agent brains produce measurably different social
behavior in the same world — i.e., can this substrate support a cross-model
behavioral-comparison finding?

**Design:** 3 arms × 2 reps × 20 wall-minutes (2x time-mult), fresh engine
boot per run from `worlds/eldoria`, identical tunings
(`respawn_cap=400,respawn_batch=16,respawn_radius=50,respawn_interval_ticks=120`... see
driver), identical personas, identical 6-bot rule background cast, population
held at 10 in every arm. Brain is the only variable:

- **claude** — 4 focal agents on `claude-haiku-4-5` (API)
- **qwen** — 4 focal agents on local Qwen3.6-27B (llama.cpp)
- **rule** — 0 LLM focal + 4 rule stand-ins (population control)

Driver: `tools/experiments/run_brain_study.py`. Runs interleaved
(c,q,r,c,q,r) so machine-load drift spreads across arms. All 6 runs completed
the full window (no early stops). Claude API cost: ~$4.1 total.

Raw outputs: `.runlog/brain_study/20260610_114426/` (per-run score.json /
summary.json / REPORT.md + comparison.{md,json}); raw event streams (with
reasoning traces) for rep-2 runs in `.runlog/brain_study/events_archive.jsonl`.

## Headline result: models have distinct social-strategy signatures

| metric (mean, per-run in parens) | claude | qwen | rule |
|---|---|---|---|
| whisper_count | **60.5** (49, 72) | 2 (3, 1) | 0 (0, 0) |
| shout_count | 21 (3, 39) | **70.5** (46, 95) | 0 (0, 0) |
| pay_transfers | **6.5** (9, 4) | 1 (0, 2) | 0 (0, 0) |
| contracts_accepted | 5.5 (6, 5) | 5 (7, 3) | 0 (0, 0) |
| contracts_completed | 1.5 (2, 1) | 2 (2, 2) | 0 (0, 0) |
| kills | 45 (49, 41) | 31 (31, 31) | 36 (22, 50) |
| gold_gini_end | 0.67 (.78, .56) | 0.55 (.60, .50) | 0.56 (.55, .57) |
| gold_total_end | 2072 | 1435 | 847 |

- **Claude runs a PRIVATE-CHANNEL strategy**: whisper-heavy 1:1 alliance pacts
  ("I'll share food and gold with you. Deal?"), pays gold to partners, goes to
  the aid of a dying contract ally, updates threat assessments after
  witnessing kills ("spawn_26 killed spawn_2 nearby — stay alert").
- **Qwen runs a BROADCAST strategy**: shouts public greetings to attract
  anyone, mass-proposes contracts, barely whispers, accepts deals on explicit
  profit logic ("free profit").
- **Rule-bot control: zero whispers, zero accepted contracts, zero pays** —
  the social signal in LLM arms is model cognition, not substrate noise.
- Signatures replicate across both reps of both models (whisper gap is 30x;
  shout gap inverse and consistent; Qwen kill count literally identical 31/31).

Communication topology (private vs broadcast) is the variable the
mechanism-design literature flags as deciding cooperation-vs-defection
emergence (GODS 2025; see `SOCIAL_EMERGENCE_LITERATURE.md` §4). The pilot
shows models intrinsically differ on that variable — the same world grows a
different society depending on which model populates it.

## Trace-level validation (social contingency, rep-2 archives)

Keyword classification of reasoning traces referencing other agents / social
context (alliances, threats, deals, named peers) vs pure item/navigation:

- claude_r2: **612 / 1212 traces (50%)** social
- qwen_r2 (partial archive): **285 / 628 (45%)** social

Threshold pre-committed before the study: ≥25%. Both clear it 2x.

## Validity notes / known artifacts

- **LLM action rejections** cluster on movement-timing mechanics
  (`pickup: target_too_far`, `inventory_full` retries), not social verbs;
  claude_r1's 638 rejections were one stuck pickup-retry loop. Social actions
  (speak/whisper/propose/accept/pay) land.
- **`item_transfers` = 0 in ALL arms** while claude_r2 alone attempted ~87
  `give` actions: the give flow appears un-completable in live play
  (`not_in_inventory` / `target_too_far`). Direct-path testing required
  (tools/audit/paths_e2e.py chain C2) — possible real substrate usability bug.
- Rule-arm kill counts vary widely between reps (22 vs 50) — combat volume
  has high run-to-run variance at n=2; treat kill/Gini differences as
  suggestive until n≥10.
- No bit-identical reproducibility (D23 stands); runs are procedurally
  comparable (same bundle/tunings/cast), not replayable.

## Decision (validated)

CONTINUE, scoped to the finding-first plan: scale to ~10 runs/arm, add 1-2
models (e.g. Sonnet vs Haiku for within-family), one mechanism-design knob
experiment (whisper range / scarcity), then public writeup with the
social-fingerprint finding as the headline.

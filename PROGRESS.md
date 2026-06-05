# PROGRESS — agent_sim v2 push (June 2026)

**Live execution state. Updated after every phase commit. If wifi drops
or session crashes, a new Claude session should read this file FIRST
to know where to resume.**

Last updated: 2026-06-05 (wave plan locked, execution not yet started)

## Source-of-truth docs

- `docs/WORLD_SYSTEM_PLAN.md` — phases 1–4
- `docs/EXPERIMENT_SYSTEM_PLAN.md` — phases 5–14
- `docs/AGENT_ARCHITECTURE_PLAN.md` — phases A1–A10

## Locked decisions (do NOT re-litigate — confirmed 2026-06-05)

| # | Decision | Choice |
|---|---|---|
| 1 | Anthropic key | NOT available. Build Claude harness behind feature flag; depth-test Qwen only. |
| 2 | Rule DSL | Starlark for everything (go.starlark.net). |
| 3 | Editor placement | Dev-mode panel on existing Pixi viewport (Cmd+E toggle). |
| 4 | UI style | Match existing FrontendV3. No shadcn. |
| 5 | First-order ToM | Baseline (in A4/A5), not optional. Second-order is A10. |
| 6 | Commit cadence | Phase-per-commit straight to main; push regularly. |
| 7 | Experiment layout | `experiments/<world>/<run-id>/` + top-level JOURNAL.md + per-world WORLD_JOURNAL.md. |
| 8 | Qwen rig | Full lifecycle control (start/stop/restart freely). |
| 9 | Visual iteration | Batch screenshots per WAVE, not per phase. |
| 10 | A9 pass bar | Zero crashes; all core verbs; ≥2 multi-turn dialogues; ≥1 trade; ≥1 building entry+exit; reflection learning; ToM non-default; p99 ≤ 3 s. PLUS user signs off on reasoning trace sample. |
| 11 | Check-in cadence | Silent inside a wave. End-of-wave: summary + screenshot batch + wait for redirect. |

## Eldoria "living world" target (added 2026-06-05)

When the user opens the frontend after the push completes, they should see:

1. **Interactable items scattered through Eldoria** — chests, food, tools,
   weapons, signs to read. Not just NPCs walking on empty tiles. Items
   are first-class entities the agents (and NPCs) can `look_at`, `pick_up`,
   `drop`, `give`, and interact with via world-novel verbs.
2. **Goal-driven NPCs (heuristic, fast)** — at least some NPCs running
   pursuit goals (head to market, sell wares, return home, etc.) so the
   world feels purposeful even before LLM agents fully load. These use
   the new heuristic-bot path (Phase A3) with simple goal stacks, NO
   LLM calls — they're cheap to run dozens of.
3. **Live LLM-backed agents (capped)** — a few (3–5) Qwen agents
   running real-time concurrently via the local rig (port 8782). Capacity
   sized so each agent's tactical cycle finishes within its target
   cadence (~1–3 s) without queuing latency cascades.

These land as part of **Wave 6** (Eldoria soak + A9 Qwen depth smoke).
A new bundle file `worlds/eldoria/agents.toml` will declare the live
agent roster (heuristic NPC specs + Qwen agent specs) — Phase A9 wires
this into the smoke run + the default `start.sh` launch.

**Qwen capacity estimate (to be measured in A5):**
- Qwen3.6-27B Q4_K_M on the 4090: ~30–50 tok/sec single-stream, with
  continuous batching across ~3–5 concurrent sequences without
  catastrophic per-stream slowdown.
- Tactical cycle: ~700–1400 tokens (in + out). Reflective cycle: ~2x.
- Realistic real-time concurrent LLM agents: **3–5**. Conservative
  default: 3. Measure first, tune via Phase A5 + A9.

## Wave plan

| Wave | Phases | Status | Notes |
|---|---|---|---|
| 1 | World 1–2 | done | Bundle refactor + Starlark DSL (c13baf4, 728ed24) |
| 2 | Substrate 5–6 + Agent A1 | done | 5cf3f05, 5df8609, 39763a4 |
| 3 | Agent A6, Substrate 7–8, Agent A2–A3 | done | 3d5d51c, 5318091, bbba8a0, e0d9c83, 4572455 |
| 4 | Agent A4–A5 + Substrate 9–10 + World 3 | done | 0385bdc, 09793e4, 909bce5, 9a0c970, ee72c24 |
| 5 | Agent A7–A8 + Substrate 11–13 | pending | |
| 6 | Agent A9 + Substrate 14 + World 4 | pending | Climactic Qwen depth smoke |
| 7 | Lint + Agent A10 | pending | Second-order ToM |

## Phase tracker

Updated after each phase commit. Format:
`[status] PHASE_ID — title — commit SHA (date)`

### Wave 1
- [done] WORLD-1 — World bundle data refactor + scenario relocation (c13baf4)
- [done] WORLD-2 — Starlark rule engine + RuleSet wiring (728ed24)

### Wave 2
- [done] SUB-5 — Layered verb refactor + per-world tuning access (5cf3f05)
- [done] SUB-6 — Stats schema + vitals/hunger system (5df8609)
- [done] AGENT-A1 — ActionBatch + ActionResult + ack futures (39763a4)

### Wave 3
- [done] AGENT-A6 — Rulebook MD+JSON pipeline (3d5d51c)
- [done] SUB-7 — Categorized JSONL logging + mute gate (5318091)
- [done] SUB-8 — Layered opt-in reasoning capture (bbba8a0)
- [done] AGENT-A2 — Layered observation renderer (e0d9c83)
- [done] AGENT-A3 — Goal-stack heuristic bot (4572455)

### Wave 4
- [done] AGENT-A4 — Claude harness skeleton (stubbed) (0385bdc)
- [done] AGENT-A5 — Qwen harness + GBNF grammars (09793e4)
- [done] SUB-9 — jsonl2sqlite derived view + indexes (909bce5)
- [done] SUB-10 — Mechanical metrics catalog (9a0c970)
- [done] WORLD-3 — Visual editor scaffold (ee72c24)

### Wave 5
- [pending] AGENT-A7 — Mental-state inspector UI (3-tab drawer + dev panel)
- [pending] AGENT-A8 — Agent tests tier 1+2 (unit + StubLLM integration)
- [pending] SUB-11 — LLM-as-judge stub (Anthropic flag off; structured summarizer)
- [pending] SUB-12 — Experiment framework + exp CLI
- [pending] SUB-13 — JOURNAL.md + INDEX.md maintenance pipeline

### Wave 6
- [pending] AGENT-A9 — Qwen depth smoke (10 agents × 30 min on Eldoria)
- [pending] SUB-14 — Iteration loop orchestrator (AlphaEvolve-style batches)
- [pending] WORLD-4 — Docs + soak verification on regenerated Eldoria

### Wave 7
- [pending] AGENT-LINT — No-engine-import lint on sdk/ + examples/
- [pending] AGENT-A10 — Second-order ToM extension (research)

## How a new session resumes

If you (a fresh Claude session) are reading this for the first time
after a crash/disconnect:

1. **Read this file end-to-end.** It is the resume contract.
2. **Read `docs/AGENT_ARCHITECTURE_PLAN.md` + `docs/WORLD_SYSTEM_PLAN.md` + `docs/EXPERIMENT_SYSTEM_PLAN.md`.** These are the locked plans.
3. **Read `RESUME_LOG.md`** (sibling file) for the per-phase commit log
   including any in-flight notes from the last session before the crash.
4. **Check `git log --oneline -20`** to confirm the commit history matches
   what `PROGRESS.md` says. If a phase is marked `[in_progress]` here but
   has a corresponding commit, mark it `[done]` first.
5. **Resume from the next `[pending]` phase** in wave order.

Do NOT re-litigate the 11 locked decisions above. If you genuinely
believe one needs revisiting, ASK the user — do not change course
unilaterally.

## Known constraints that bind every phase

- Tests pass before committing a phase.
- Each phase commit message uses imperative form, no Co-Authored-By
  trailer (see [[feedback_commit_after_big_changes]] in auto-memory).
- Author attribution: `GIT_AUTHOR_NAME=anishmah100`,
  `GIT_AUTHOR_EMAIL=anishmah100@users.noreply.github.com`.
- `start.sh` updated when ports / scenarios / bot types change.
- UI changes get a Playwright screenshot at the end of their WAVE,
  bundled into a single SendUserFile batch.
- Engine port: 8080. Vite dev: 5173. Qwen llama-server: 8782.

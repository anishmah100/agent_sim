# June 2026 v2 push — results

24-phase autonomous execution, June 5, 2026. See `PROGRESS.md` for
the live phase tracker and `RESUME_LOG.md` for per-phase notes.

## Three plan docs

- `docs/WORLD_SYSTEM_PLAN.md` — world bundle layout + Starlark DSL
- `docs/EXPERIMENT_SYSTEM_PLAN.md` — auto-research loop substrate
- `docs/AGENT_ARCHITECTURE_PLAN.md` — 4-layer LLM-backed agent baseline

## What landed

### Worlds become bundles

```
worlds/<name>/
  bundle.toml      manifest (schema, scenario_pkg, art_pack, npcs, rules)
  world.json       map data (entities, tiles, decorations)
  rules.star       declarative ruleset (tunings, items, stats, novel verbs)
  npcs.json        process supervisor config (optional)
  design/          procedural generators (optional)
```

Engine takes `--bundle worlds/<name>/`; legacy `--world worlds/<name>.json`
still works for ad-hoc dev.

### Starlark rule engine

`engine/internal/world/rules/`: hermetic + deterministic (`go.starlark.net`).
Registers tunings, items, stats, and novel verbs from a single `rules.star`.
Eldoria's first ruleset: 13 tunings, 7 items, 3 stats, 1 demo novel verb.

### Layered verb catalog

`VerbCategoryCore | Common | Novel`. Engine systems (combat, money,
vitals) read scalar values from `World.Tuning(...)` instead of package
constants — per-world `attack_damage`, `starting_gold`, `hunger_per_tick`
all flow through the bundle's ruleset.

### Vitality (hunger) system

`engine/internal/systems/vitals` ticks hunger by `hunger_per_tick`,
drains hp when above `hunger_damage_above`. Eldoria's tuning: ~5 min
sated → starving, then 1 hp / tick.

### SDK (Python)

- `ActionBatch` + `ActionResult` + `ReasonCode` enum
- `act_batch(batch, timeout=...)` returns per-action results via futures
- `render_layered_observation(obs, coord_style="compass"|"absolute")` —
  salience-sorted nearby + audible buffer + 11×11 minimap
- `share_reasoning=True` on registration plumbs the per-action reasoning
  trace through (engine drops it unless `-capture-reasoning` is set,
  layered opt-in).

### Two LLM harnesses

- `examples/claude_agent/` — 4-layer brain (persona / reflective /
  tactical / reflex). Anthropic client behind a feature flag; refuses
  to fire without `--enable-claude`.
- `examples/qwen_agent/` — same Harness class, different cadence + GBNF
  grammars per layer (`persona.gbnf`, `reflective.gbnf`, `tactical.gbnf`).

Stub LLM ships in both; tests run against the stub.

### Rulebook pipeline

`worlds/<name>/RULEBOOK.md` + `worlds/<name>/rulebook.json` are
auto-rendered from bundle + ruleset + manifest. CLI:
`go run ./cmd/genrulebook -bundle worlds/eldoria`. Served live at
`GET /api/v1/world/rulebook.json`.

### Categorized JSONL logging

Every event in `historian` carries a `category` (system / movement /
combat / economy / social / agent_reasoning / world). `-event-mute=cat1,cat2`
drops noisy categories. Reasoning traces land under `agent_reasoning`
with first-class shape.

### Post-experiment toolchain

```
logs.jsonl  →  jsonl2sqlite  →  derived.sqlite
                                     ↓
                                tools/metrics/catalog → metrics.json
                                tools/judge/judge     → judge_report.md
                                tools/exp/cli         → REPORT.md
                                tools/journal/update  → JOURNAL.md + INDEX.md
                                tools/loop/orchestrator → next iteration
```

### Mental-state inspector UI

Inspector picks up a 3-tab drawer (Speech / Mind / Trace) gated by
`share_planner` + `share_reasoning`. Engine ships
`GET /api/v1/agent/<id>/mental_state`.

### Visual world editor scaffold

Cmd+E toggles a dev-mode panel on the existing Pixi viewport. Tile
palette + tool selector + Save button. Persistence to disk lands
later (panel UI surface ships in `WORLD-3`).

## Deferred (documented but not shipped)

- **Eldoria living world** (tasks #210/211/212/213): scatter items
  across Eldoria, swap wandering NPCs for goal-driven heuristic bots,
  spawn 3–5 live Qwen agents on boot. Needs real Qwen capacity
  measurement (`tools/dev-scripts/bench_qwen.mjs` to land).
- **AGENT-A9** smoke pass with real Qwen: blocked on the local rig
  not being verified during this push. Runbook in
  `examples/qwen_agent/README.md`.
- **AGENT-A10** second-order ToM: scaffolding exists in `BrainState.agent_register`;
  pending bandwidth.
- **WORLD-3 paint persistence**: editor UI ships but `Save` is a
  no-op. The HTTP endpoint `/api/v1/world/edit` lands when needed.

## How to verify

```
cd ~/projects/agent_sim

# Engine + tests.
(cd engine && go test ./...)

# Python SDK + tools.
(cd sdk/python && python -m pytest)
python -m pytest tools/

# Full smoke.
./start.sh
# Open http://127.0.0.1:5173 — click Eldoria, see the new editor button.
```

Author: anishmah100, June 2026.

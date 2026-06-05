# Experiment System — Plan (June 2026)

The companion to `WORLD_SYSTEM_PLAN.md`. Where that doc defines how
worlds are built and swapped, this one defines how we EXPERIMENT on
them — the iteration loop that produces emergent multi-agent
phenomena.

## Vision

agent_sim is a **substrate for emergent-behavior experiments**. The
visuals + map system + scaling were prerequisites; the actual product
is the EXPERIMENT LOOP — define rules, run a society of LLM agents,
observe emergent macro-phenomena (government formation, theft cascades,
scheming, enforcement, ad-hoc economies), tune rules, repeat.

Inspiration: ALife, Smallville, GovSim. The novel element is **scale +
tunability** — thousands of LLM agents in a rule-system that's
deliberately optimized for emergent behavior.

This is an **unverifiable-domain auto-research loop** (analogous to
AlphaEvolve, but where the scoring oracle is partly subjective):

```
                  ┌────────────────────────────────────┐
                  │ user describes target properties   │
                  │  ("scheming, governments, intrigue")│
                  └─────────────────┬──────────────────┘
                                    │
                                    ▼
        ┌─────────────────── JOURNAL ────────────────────┐
        │ "what we've learned so far" — stable findings, │
        │ open questions, rejected hypotheses, next bets │
        └─────────────────────┬──────────────────────────┘
                              │ read at iteration start
                              ▼
                    ┌──────────────────┐
                    │ propose ruleset  │ ← MINIMAL ruleset such that
                    │ + hypothesis     │   target arises (Occam)
                    └────────┬─────────┘
                             ▼
                    ┌──────────────────┐
                    │ run experiment   │  N agents, T minutes,
                    │                  │  full log capture
                    └────────┬─────────┘
                             ▼
                    ┌──────────────────┐
                    │ analyze:         │
                    │  • metrics       │  mechanical (deaths,
                    │                  │    transactions, money_gini)
                    │  • LLM judge     │  qualitative (did scheming
                    │                  │    happen? rate 1-5)
                    │  • Claude reads  │  reasoning traces, dialogue,
                    │    raw events    │    notable transitions
                    └────────┬─────────┘
                             ▼
                    ┌──────────────────┐
                    │ commit findings  │ ← was hypothesis right?
                    │ to JOURNAL       │   what's the next bet?
                    └────────┬─────────┘
                             │
                             └─────────► next iteration
```

## Decisions captured

| Decision | Choice |
|---|---|
| Engine verb depth | (γ) Layered. Core = physical+sensory+life/death+social primitives. Shared library of common-world plugins (attack, eat, pay, trade, vote, alliance). Worlds parameterize + add novel verbs/stats/items via predicate-and-effect DSL, Go escape hatch only when needed. |
| Logging shape | JSONL primary (durable, lossless, fast write path) + SQLite derived view post-experiment (queryable). Category-gated. |
| Reasoning trace format | Free-text `reasoning` string attached per submitted action. Cheap to produce, works with any LLM, sufficient for "why did X do Y" debugging. |
| Trace opt-in | Layered: experiment-level `capture_reasoning` toggle AND per-agent `share_reasoning` flag must BOTH be true. Privacy + bulk control. |
| Success criteria | Hybrid: mechanical metric catalog (auto) + LLM-as-judge for qualitative properties (auto) + Claude reads raw logs (manual). All three inform the report. |
| Experiment shape | Folder-per-experiment with `parent` + `diff_from_parent.yaml` evolution tree. Top-level `JOURNAL.md` + `INDEX.md` provide memory across runs. |
| Agency | Tiered batches: user sets a budget ("5 iterations toward X, then stop and synthesize"). Inside the batch I have full autonomy: propose, run, report, learn. Batch end → synthesis report → user redirects. |

## Architectural picture

```
┌──────────────────────────────────────────────────────────────────────┐
│ engine/                          ← runtime + core verbs              │
│  internal/                                                            │
│    world/      tick, snapshots, lock-free reads                       │
│    wire/       WS protocol                                            │
│    core/                                                              │
│      eventbus/   already exists; logging subscribes here              │
│      verbs/      core verb dispatch (move, look, speak, give, …)      │
│    genrules/   predicate+effect evaluator (placement + verb rules)    │
│    log/        JSONL writer, category filter, event-bus subscriber    │
│    metrics/    mechanical metric extractors                           │
│    judge/      LLM-as-judge API client (Anthropic)                    │
│    experiments/ experiment runner + folder structure                  │
│  cmd/                                                                 │
│    engine/     main; imports active world's scenario                  │
│    genworld/   declarative world generator (from WORLD_SYSTEM_PLAN)   │
│    exp/        CLI: `exp run`, `exp report`, `exp compare`, `exp diff`│
│    judge/      one-off CLI to re-run judge on a stored experiment     │
│                                                                       │
│ common-library/                  ← shared plugins (was internal/systems)│
│   combat/, money/, inventory/, trade/, quests/, …                     │
│   each is a verb-or-system plugin worlds can import or skip           │
│                                                                       │
│ worlds/                          ← swappable world bundles            │
│   eldoria/                                                            │
│     world.yaml                                                        │
│     rules.yaml      ← tunings + items + stats + novel verbs (NEW)    │
│     scenario/       ← Go code (was engine/internal/scenario/…)        │
│     …                                                                 │
│                                                                       │
│ experiments/                     ← growth-over-time research artefacts│
│   JOURNAL.md                                                          │
│   INDEX.md                                                            │
│   exp-001-baseline/                                                   │
│   exp-002-harder-hunger/                                              │
│   …                                                                   │
└──────────────────────────────────────────────────────────────────────┘
```

## World rule schema — extension of WORLD_SYSTEM_PLAN's predicate-and-effect DSL

The same DSL that handles placement (houses-can't-be-on-water) is
extended to cover **verb tunings + new verbs + items + stats**. One
language; one evaluator; one mental model.

```yaml
# worlds/eldoria/rules.yaml — TUNINGS + extensions

# === Stats every entity has ===
stats:
  hp:        { type: int,   min: 0, max: 100, default: 100 }
  hunger:    { type: float, min: 0, max: 100, default: 0 }
  satiety:   { type: float, min: 0, max: 100, default: 50 }
  money:     { type: int,   min: 0,             default: 25 }
  reputation:{ type: int,   default: 0 }

# === Periodic effects — applied per tick by the engine ===
tick_effects:
  hunger_rise:
    target: { archetype: agent_eligible }
    apply:  { hunger: +0.05 }      # per second; ~3.0/min
    when:   { state: { not_asleep: true } }
  hunger_death:
    target: { hunger: { gte: 100 } }
    apply:  { hp: -1 }
    when:   { tick: { every: 60 } }

# === Core verbs the engine ships — worlds tune them ===
verb_tunings:
  attack:
    cooldown_ticks: 18
    damage:                       # expression in the DSL
      base:        weapon.damage  # if no weapon: 1
      attacker_hp: { multiplier: 0.5, of: caster.hp_fraction }
      crit_chance: 0.05
    target_consequence:
      - { stat: hp, delta: -damage }
      - { event: combat, severity: damage, attacker: caster.id, victim: target.id }
    requires:
      - { distance: { max: 1 } }                   # melee by default
      - { life: { caster: alive, target: alive } }
  eat:
    requires:
      - { holder_has_item: { tag: edible } }
    apply:
      - { caster.satiety: +item.satiety_value }
      - { caster.hunger:  -item.satiety_value }
      - { consume_item: target_item }
  pay:
    requires:
      - { distance: { max: 1 } }
    apply:
      - { caster.money: -amount }
      - { target.money: +amount }
      - { event: payment, amount: amount, from: caster.id, to: target.id }

# === Items (what they do when worn / used / held) ===
items:
  sword:
    tags: [weapon]
    attrs: { damage: 12, range: 1 }
  bread:
    tags: [edible]
    attrs: { satiety_value: 30 }

  # LOTR-style world-specific mechanic
  ring_of_power:
    tags: [equipable, evil]
    attrs: { range: 5 }
    on_equip:
      - { event: corruption_began, holder: caster.id }
    while_equipped:
      - { stat: caster.sanity, delta_per_tick: -0.1 }
      - { unlocks_verb: command_thrall }
    on_use:
      apply:
        - { caster.corruption: +5 }

# === World-specific new verbs ===
verbs:
  command_thrall:
    description: "Force a target to execute a command on the next tick."
    requires:
      - { holder_has_item: ring_of_power }
      - { distance: { max: 5 } }
      - { line_of_sight: true }
    cooldown_ticks: 60
    apply:
      - { override_next_action: target, command: params.command }
      - { caster.sanity: -1 }
      - { event: mind_control, controller: caster.id, victim: target.id }
```

Schema highlights:

- **Predicates compose.** `AND/OR/NOT` already in our placement-rule
  DSL; just reused here.
- **Tunings ARE the iteration knobs.** "Hunger ticks too soft" =
  bump `tick_effects.hunger_rise.apply.hunger` from 0.05 to 0.12.
  Single-line YAML change.
- **Items declare their affordances** rather than the engine knowing.
  A new world adds new tags + new items without touching engine code.
- **Novel verbs are first-class.** The Ring's `command_thrall` is a
  full verb the engine routes; agents can perceive it in
  observations; the LLM can reason about whether to do it.
- **Go escape hatch.** When the DSL truly can't express something
  (custom event-bus integration, multi-step asynchronous mechanics),
  a world's `scenario/` Go package adds the verb directly. Same
  registry pattern; same observability.

## Logging architecture

Single event bus. JSONL writer subscribes to all categories and
writes one line per event. Filter on the way in (category toggle in
experiment config). Compress on close. Optionally post-process to
SQLite for queryability.

Categories:

| Category | Default | Purpose |
|---|---|---|
| `tick` | off | Per-tick heartbeat + active agent count |
| `action` | on | Every action the engine processes |
| `speech` | on | speak / shout / whisper events |
| `motion` | off | Step-by-step movement (often noisy) |
| `combat` | on | Attacks, damage, kills |
| `economy` | on | Pay / trade / inventory transfers |
| `spawn_despawn` | on | Births / deaths / item drops |
| `state_change` | off | Every stat change (very high volume) |
| `reasoning` | gated | LLM reasoning attached to actions |
| `observation` | off | Engine-built observation payloads (huge) |
| `validate_violation` | on | Rule predicate rejections |

Per-experiment YAML toggles flip these on/off:

```yaml
# in the experiment config
logging:
  categories: [action, speech, combat, economy, reasoning, spawn_despawn]
  reasoning_capture: true       # global gate
  rotate_at_mb: 500             # split into events.0001.jsonl.gz, …
```

Event shape:

```json
{
  "tick": 12345,
  "wall_ms": 1717589123456,
  "category": "combat",
  "actor": "agent_42",
  "verb": "attack",
  "targets": ["goblin_7"],
  "params": { "weapon": "sword" },
  "result": "hit",
  "consequences": [
    { "kind": "stat_delta", "entity": "goblin_7", "stat": "hp", "delta": -12 },
    { "kind": "emit_event", "name": "goblin_7_wounded" }
  ],
  "reasoning": "Goblin is alone and looks weak; I'm hungry and they have inventory.",
  "ruleset_hash": "abcd1234"
}
```

`ruleset_hash` is the SHA-256 of the world's rules.yaml at run time —
so we can NEVER mix up which ruleset produced which log line.

Post-experiment SQLite is a sidecar built by a small Go program that
streams `events.jsonl.gz` and inserts into typed tables (`actions`,
`combat`, `economy`, `state_changes`, `reasoning`, plus a generic
`events_raw` for anything that doesn't fit). Adds indexes on
`(actor, tick)` and `(category, tick)`. Cheap; runs in seconds for
typical experiments.

## Reasoning trace integration

SDK change: `Action` envelope grows an optional `reasoning: string`
field. Agent fills it when willing; engine logs it under category
`reasoning` IFF both:

- experiment config has `capture_reasoning: true`, AND
- agent's `share_reasoning: true` at register time

When ungated, the engine drops the field (does not log, does not echo).

For our Claude-API-backed agents, the Python SDK wrapper will:

- Capture the model's full assistant message (the rationale block) as
  the `reasoning` string.
- Default `share_reasoning: true` for local/development runs.
- Surface a `--no-share-reasoning` flag for production-style agents.

Future: more structured trace formats (raw chat transcript, decision
trees) can be added as new categories without breaking the simple
free-text path.

## Experiment folder layout

```
experiments/
  JOURNAL.md            ← cumulative synthesis (Claude maintains)
  INDEX.md              ← one line per run (id, hypothesis, judge, link)

  exp-001-baseline-fantasy/
    metadata.json       ← parent, ruleset_hash, agent_count, seed, duration
    config_snapshot/    ← frozen copy of worlds/eldoria/ at launch
    hypothesis.md       ← BEFORE running. What I expect + why.
    events.jsonl.gz     ← log
    events.sqlite       ← derived
    metrics.json        ← mechanical catalog output
    judge.md            ← LLM judge verdict + citations
    report.md           ← AFTER running. Was hypothesis right? Learnings.

  exp-002-harder-hunger/
    parent: exp-001-baseline-fantasy
    diff_from_parent.yaml   ← only the rules.yaml deltas
    …
```

Naming: `exp-NNN-<short-slug>` where NNN is a zero-padded sequence
across the project.

Everything except `JOURNAL.md` and `INDEX.md` is gitignored by default;
the journal is the long-term memory and ships with the repo.

## Mechanical metrics catalog

Each metric is a Go struct implementing `Compute(events) → value`.
First-pass catalog:

- `kill_rate_per_minute`
- `transaction_count`, `transaction_volume_money`
- `money_gini` (inequality at run end)
- `dialogue_words_per_minute`
- `alliance_stability` (rolling 5-min window of repeated cooperation pairs)
- `vote_count`, `policy_proposal_count`
- `conflict_density` (combat events per 100 ticks)
- `starvation_death_pct`
- `archetype_alive_at_end` (per archetype)
- `most_common_3grams_in_speech` (top topics)
- `mean_reasoning_length` (sanity check on trace capture)

Output is a flat JSON file per experiment + an aggregated row in
`experiments/INDEX.md` for quick scanning.

Adding new metrics = one Go file + register in a catalog. Each metric
should be pure: reads events, emits a value, no side effects.

## LLM-as-judge integration

After the run finishes:

1. Build a structured summary: top 50 events by category, all
   speech transcripts (or sampled if huge), all kills, all
   transactions, all alliances detected, all faction transitions.
2. Send to Anthropic API with a fixed judge prompt + the
   experiment's target properties.
3. Receive structured response: per-target-property score 1-5 +
   citations (event ids the judge based the score on) + a free-form
   "vibe" paragraph.
4. Write to `judge.md`. Include the model + tokens used so we can
   track judge drift if we change models.

The judge prompt is itself versioned (`prompts/judge_v1.md`) so we
can compare across runs without re-judging.

Cost: a long experiment summary might be 30-50k tokens; judge runs at
<$0.50 per experiment with Claude 4.6 Haiku. Budgetable.

## CLI surface

```
exp init <slug> --parent <id>      # scaffold a new experiment dir
exp run <id>                       # boot engine, run, write logs
exp report <id>                    # render mechanical metrics + judge
exp compare <a> <b>                # side-by-side metric diff
exp diff-config <a> <b>            # YAML diff of rulesets
exp judge <id> [--prompt v2]       # re-run judge (e.g. with new prompt)
exp journal-update                 # have Claude regenerate JOURNAL.md
                                   # from recent experiments
```

Backed by Go in `engine/cmd/exp/`. Single binary.

## Agency model (tiered guardrails) — workflow

```
1. User: "Run a batch of 5 iterations toward 'scheming + government'.
   Stop and synthesize."
   ↓
2. Claude reads JOURNAL.md, picks initial hypothesis informed by
   prior findings. Writes exp-NNN-1/hypothesis.md.
   ↓
3. Claude runs exp-NNN-1. Watches the run via WS metrics. Stops at
   the planned duration.
   ↓
4. Claude reads metrics + judge + raw events. Writes
   exp-NNN-1/report.md. Updates JOURNAL.md with stable findings.
   ↓
5. Claude proposes the next ruleset diff based on what the report
   identified ("attacks still rare → bump weapon damage by 2x, watch
   if combat density rises").
   ↓
6. Iterate steps 2-5 until budget exhausted.
   ↓
7. Claude writes batch synthesis: "Across 5 iterations toward
   scheming, the dominant blocker was X. Best result was exp-NNN-3
   which produced limited scheming but no government. Next bets:
   …" Awaits user redirect.
```

Budget can be expressed as: `iterations: 5`, `tokens: $50`, `wall_clock: 6h`.

## Phased execution

Adds to the WORLD_SYSTEM_PLAN phases:

| # | Phase | Depends on | Days |
|---|---|---|---|
| 5  | Layered verb refactor — split engine into core + common-library; expose plugin registry | clean engine module split | 1.5 |
| 6  | World rule schema (tunings + items + stats + novel verbs in YAML); generalize predicate-and-effect evaluator | 5 | 1.5 |
| 7  | Structured event-bus logging (JSONL writer, category filter, experiment config gate) | 5 | 1 |
| 8  | Reasoning trace SDK changes + layered opt-in | 7 | 0.5 |
| 9  | SQLite derived-view post-processor + indexes | 7 | 0.5 |
| 10 | Mechanical metrics catalog (10+ extractors, INDEX row aggregator) | 9 | 1 |
| 11 | LLM-as-judge: prompt v1, summarizer, API client | 10 | 0.5 |
| 12 | Experiment framework — folder layout, `exp` CLI, parent/diff tracking | 7, 10, 11 | 1 |
| 13 | JOURNAL.md + INDEX.md maintenance (Claude updates after each batch) | 12 | 0.5 |
| 14 | Iteration loop CLI / batch orchestrator (the AlphaEvolve loop) | 12, 13 | 1 |

Total: ~9 additional days on top of WORLD_SYSTEM_PLAN's ~5 days.

## What this plan does NOT do

- **Doesn't reach the agent-architecture redesign.** The user flagged
  that as a separate deep discussion that follows this one. The verb
  catalog above is the SCAFFOLD; the actual catalog of verbs (what
  even the engine offers as primitives) is settled in that next pass.
- **Doesn't ship a custom UI for experiment management.** CLI +
  Markdown reports. If a web UI for browsing experiments becomes
  important, it's a follow-up.
- **Doesn't auto-tune.** This is human-in-the-loop with tiered
  guardrails. No reinforcement-learning loop on top.
- **Doesn't multiplex experiments.** One experiment runs at a time,
  binding the engine. Parallel batches later if cost makes sense.
- **Doesn't ship a second world.** Eldoria is the substrate for the
  initial experiment series. Confirming everything works on one
  world is enough; a second world (e.g. Manhattan-finance) tests the
  layered-verb separation in a later pass.

## Open questions to revisit at phase boundaries

- **Judge model.** Start with Claude 4.6 Haiku (cheap, fast). Confirm
  it's discriminating enough by sampling judge agreement against
  human (your) scoring on the first 3 experiments. Promote to Sonnet
  if needed.
- **Replay tooling.** Should we be able to re-render an experiment's
  timeline visually after the fact (a "movie of the experiment")?
  Probably yes eventually. Cheap to add since we have the events.
  Defer until the iteration loop is producing real findings.
- **Cross-experiment metric trend dashboard.** When INDEX.md grows to
  100+ runs, a small static-site dashboard might help. Trivial
  follow-up.

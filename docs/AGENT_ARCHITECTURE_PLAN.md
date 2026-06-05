# Agent Architecture — Plan (June 2026)

The third sibling to `WORLD_SYSTEM_PLAN.md` and `EXPERIMENT_SYSTEM_PLAN.md`.
Where those define how worlds are built and how experiments iterate on
them, this doc defines **how an LLM-backed agent actually thinks, decides,
and acts** inside one — the baseline that has to be *good enough* that
emergent macro-phenomena can actually arise.

## Vision

This is **not** a pursuit of the strongest possible agent. It's a pursuit
of a **damn good baseline** — capable enough that:

- A society of thousands of these agents in a tuned world produces the
  emergent phenomena we care about (government formation, theft cascades,
  scheming, enforcement, ad-hoc economies).
- A reader who knows nothing about the engine internals can look at the
  agent code alone and grok how to build a competing one.
- The same harness runs against **local Qwen** (cheap, scale, the default
  for big societies) and **Anthropic Claude** (rich, the gold-standard
  comparator), with each backend's quirks accounted for in its own
  harness rather than papered over by a lowest-common-denominator
  abstraction.
- A zero-shot transfer from a custom researcher's bot (built for a
  different world) onto our world is *possible* via a standardized
  rulebook — even if the resulting play is bad, the bot loads, reads the
  rules, and acts.

Inspiration (background research, condensed):

- **Generative Agents (Park et al. 2023 / Smallville)**: hierarchical
  memory stream, reflection, day-planning, dialogue. Validated that
  hybrid structured-plus-free-text memory scales to dozens of agents.
- **Voyager (Wang et al. 2023)**: skill library, self-curriculum, code
  as memory. We borrow the *idea* of an agent learning the world's
  affordances rather than being hand-coded for each.
- **ReAct (Yao et al. 2022)**: reason-act-observe interleave. The
  tactical-layer pattern.
- **MemGPT / extended-context tricks**: structured working-memory page
  with rolling free-text fragments. Our memory spine is a stripped-down
  version of this idea.
- **AlphaStar / OpenAI Five**: hierarchical control + action queues.
  Inspired the receding-horizon queue.
- **GovSim**: a small society of LLM agents will form alliances and
  break them; we're scaling that up by ~50–500×.

## Vision is non-negotiable

> "The most important part of this project is actually the world
> experiments themselves … the real hook is the emergent properties of
> a society of thousands of agents."

If a phase below seems too ambitious, **expand the timeline rather
than the scope**. The agent baseline has to be strong enough that when
something interesting happens, we believe it. A weak baseline lets us
explain anything away as "the agent was just dumb."

## Decisions captured

| Decision | Choice |
|---|---|
| Decision loop shape | **Hierarchical 4-layer**: persona (one-time) + reflective (~60–120 s, staggered) + tactical (~1 Hz, ReAct) + reflex (~10 Hz, code). Body drains a small action queue at engine tick rate (60 Hz). |
| Two harnesses | **Yes, intentionally**. `ClaudeHarness` and `QwenHarness` share an output API but use different prompts, different output formats (free-text JSON vs grammar-constrained), different memory rendering, different reflection cadences. No "polyfill" layer that masks the differences. |
| Memory | **Hybrid**: structured spine (`WorldMap`, `AgentRegister` sparse top-K, `DialogueLog`, `GoalStack`) + per-slot free-text notes. Reflection writes notes; tactical reads spine + recent notes. |
| Observation shape | **Layered: structured + sparse vision**. Self block, nearby-entities list within vision radius (≤ K=12, sorted by salience), `audible` ring buffer (30 s window of speech + non-speech sound events with localization hints), compact ASCII minimap centered on self (11×11). ~600–1200 tokens. |
| Verb catalog depth | **Layered**: ~11 core verbs from the engine (move_step, look_at, speak, shout, whisper, pick_up, drop, give, pay, wait, ponder) + ~15–20 common-library verbs available to most worlds (attack, eat, propose_trade, vote, …) + per-world novel verbs declared in the world's YAML config. |
| Action queue | **Receding-horizon batch of 1–3 actions** per LLM call, each with `precondition` + `interrupt_if`. First failure drops the rest and re-plans. Submitted as one unit; consumed action-by-action by the body. |
| Blocked-action feedback | **Structured triple**: `{reason_code: enum, context: object, human_text: string}`. Code is for the harness to branch on; text is for the LLM to read. Context contains the actor/object that caused the block. |
| Pathfinding | **Agent's job, not the engine's**. The engine exposes `move_step(dir)` and the agent's reflex layer runs A* on its known map. Blocked steps yield diagnostic context so the agent can re-plan socially (push past, ask, detour). |
| Rulebook | **YAML config is source-of-truth**; `RULEBOOK.md` (committed, human-read) and `rulebook.json` (served at `GET /api/v1/world/<name>/rulebook.json`, agent-read) are auto-rendered. Fixed table-of-contents schema across all worlds. |
| Code separation | **Hard wall**: `sdk/python/agent_sim_sdk/` is wire + types; `examples/` holds reference agents (`heuristic_bot.py`, `claude_agent/`, `qwen_agent/`, `tom_agent/`). A lint test bans engine-package imports from anywhere under `sdk/` or `examples/`. |
| Mental-state UI | **Click any entity → side drawer**. Three tabs: (1) Recent dialogue (always public, last 20 lines you said/heard). (2) Goal stack + last action (gated by `share_planner` flag). (3) Reasoning trace (gated by both experiment-level `capture_reasoning` AND per-agent `share_reasoning`). Inspector view (developer-mode) shows the raw observation, the raw LLM prompt, and the raw LLM response side-by-side for the last N ticks. |
| Testing strategy | Three-tier: (1) **Unit**: per-layer with mocked LLM (deterministic). (2) **Integration**: real engine + scripted "stub LLM" that replays canned responses — verifies dialogue/items/whisper/shout/building entry behave end-to-end. (3) **Smoke** with real LLM (Qwen): 10-agent 5-minute Eldoria run that never crashes, all verbs exercised at least once, all interiors entered. |

## Architectural picture

```
┌──────────────────────────────────────────────────────────────────────┐
│ engine/   (Go)                                                       │
│   - world tick (60 Hz), verbs, collisions, spawn/despawn             │
│   - exposes WS observation stream + HTTP rulebook/affordances        │
│   - HAS NO knowledge of "agents" beyond an opaque action stream      │
└────────────────────────┬─────────────────────────────────────────────┘
                         │ WebSocket (per-entity)
                         ▼
┌──────────────────────────────────────────────────────────────────────┐
│ sdk/python/agent_sim_sdk/   (wire types, transport, no policy)       │
│   - WSClient: connect, observe(), submit_actions(), get_rulebook()   │
│   - models.py: Observation, Action, ActionResult, RuleBook           │
│   - testing.py: StubLLM, scripted scenario harness                   │
└────────────────────────┬─────────────────────────────────────────────┘
                         │ import
                         ▼
┌──────────────────────────────────────────────────────────────────────┐
│ examples/   (the reference agents — copy-paste-fork friendly)        │
│                                                                      │
│   heuristic_bot.py     – ~50 LOC, no LLM, proves the SDK works       │
│   claude_agent/        – Claude harness + 4 brain layers             │
│   qwen_agent/          – Qwen harness + 4 brain layers (GBNF)        │
│   tom_agent/           – ToM extension on top of claude_agent        │
│                                                                      │
│   Each example has a README.md that reads alone — a researcher       │
│   can clone, swap in their own LLM, and run.                         │
└──────────────────────────────────────────────────────────────────────┘
```

## The 4-layer brain

Top-down, slowest to fastest. Each layer reads from layers above and
writes back its own state.

### Layer 1 — Persona (one-time, ~once per agent lifetime)

Run on agent registration. Input: persona fields (`name`, `archetype`,
`bio`). Output:

- **Long-term values**: a few sentences the agent commits to (e.g. "I
  distrust strangers", "I want to become rich"). Read by every reflective
  cycle as anchoring context.
- **Initial GoalStack**: 1–3 top-level goals derived from the persona
  ("get to Eldoria", "find work").

Not re-run unless persona is mutated by an event (e.g. trauma in some
worlds could trigger persona re-evaluation; baseline doesn't use this).

### First-order ToM is baseline, not an extension

Every reflective cycle updates `agent_register[other_id].theory_of_me`
for each top-K other based on:

- What that agent said/did to me recently.
- What I observed them saying/doing to others I know.
- The current state of the world (am I high-status now? broke?).

The tactical brain reads `theory_of_me` when planning social actions
("she probably still thinks I owe her — better pay first, ask after").

Second-order ToM ("what X thinks Y thinks about Z") is in Phase A10
as a research extension, since the local Qwen rig likely can't sustain
it well.

### Layer 2 — Reflective brain (~60–120 s of sim-time, staggered)

The Smallville-style "step back and think" layer. Runs roughly once a
sim-minute per agent, but **staggered** across the population — at
1000 agents, ~10/sec reflect, never a synchronized burst. Input:

- Long-term values (from persona).
- Current GoalStack.
- Last N reflective notes (from prior cycles).
- Last K tactical notes (the free-text shorthand the tactical brain
  jotted down).
- Recent dialogue summary.
- Rolled-up structured spine (no per-tile detail at this layer).

Output:

- Updated GoalStack (push/pop/edit).
- A new reflective note (free text, summarising what was learned this
  window).
- Optional updates to the AgentRegister entries for known others ("Cara
  hasn't paid me back, I now think she's unreliable").

This is the layer most affected by harness choice. Claude gets rich
free-text reasoning here; Qwen gets a flatter schema with discrete
slots.

### Layer 3 — Tactical brain (~1 Hz, ReAct interleave)

The workhorse. Runs every 1–3 seconds per agent. Input: the layered
observation (see "Observation shape" below). Output: a **batch of
1–3 actions** + a free-text `reasoning` field per batch.

Each action carries:

```jsonc
{
  "verb": "move_step",
  "args": { "dir": "N" },
  "precondition": { "i_am_at": [24, 17] },  // optional
  "interrupt_if":  { "saw_speaker": "*" }   // optional, halts queue
}
```

The body drains the queue, action-by-action, at engine tick rate.
On any failure, the queue is dropped and tactical re-plans next cycle.

### Layer 4 — Reflex (~10 Hz, code only)

Pure Python, no LLM. Handles:

- A* pathfinding over the agent's known map.
- Local collision avoidance (one-step sidestep around a stationary
  obstacle without bothering the tactical layer).
- Interrupt detection: if `interrupt_if` fires, halt the queue and bump
  tactical's cadence.
- Cheap "no-brainer" behaviors when tactical is mid-call (e.g. "I see a
  threat closing in" → step away).

### Layer 5 — Body (60 Hz)

Just drains the action queue against the engine's tick. Each tick:
pop the next action if its precondition holds, submit it, push the
result back up to the reflex layer.

## Two harnesses, intentionally different

```
                ┌──────────────────────────────────────┐
                │           BrainState (shared)        │
                │   GoalStack, AgentRegister, notes…   │
                └──────────────┬───────────────────────┘
                               │
       ┌───────────────────────┼───────────────────────┐
       ▼                                               ▼
┌──────────────┐                                ┌──────────────┐
│ ClaudeHarness│                                │ QwenHarness  │
├──────────────┤                                ├──────────────┤
│ - Free-text  │                                │ - GBNF-      │
│   reasoning  │                                │   constrained│
│ - Rich obs   │                                │   JSON only  │
│   rendering  │                                │ - Flat, deno-│
│ - 1 prompt   │                                │   ted schema │
│   per cycle  │                                │ - 1 grammar  │
│ - Tool-use   │                                │   per layer  │
│   for action │                                │ - Shorter    │
│   batch      │                                │   obs win-   │
│ - Reflection │                                │   dow (8 chat│
│   prompted   │                                │   lines vs   │
│   as essay   │                                │   20)        │
└──────────────┘                                └──────────────┘
       │                                               │
       └───────────────────┬───────────────────────────┘
                           ▼
                   common output API:
              ActionBatch + ReasoningTrace
```

**Why two**: Qwen 3.6-27B on the local rig (port 8782, `--reasoning-budget 0`)
has well-known failure modes when asked to "reason then JSON" without a
grammar; Claude has the opposite problem (over-eager prose, can be
unprompted into lengthy soliloquies on a cheap action). Each harness
adapts the prompt, the schema, and the layer cadence to its model's
quirks. The shared piece is the OUTPUT API — both produce the same
`ActionBatch` shape and `ReasoningTrace` shape, so the rest of the
plumbing (logging, UI, judge) is unified.

**Concretely**:

- `ClaudeHarness.tactical_cycle(state, obs) -> (ActionBatch, ReasoningTrace)`
- `QwenHarness.tactical_cycle(state, obs) -> (ActionBatch, ReasoningTrace)`

Both implement the `Harness` protocol declared in
`sdk/python/agent_sim_sdk/harness.py`.

## Memory: structured spine + free-form notes

```
BrainState
├── persona               ── set once
│   ├── name, archetype, bio
│   └── long_term_values: [str]
│
├── goal_stack            ── small (≤ 5 items)
│   └── [{goal, why, status}]
│
├── agent_register        ── sparse top-K (K=20), first-order ToM lives here
│   └── {entity_id: {
│         last_seen_pos,
│         disposition,         ── how I feel about them (-1..+1)
│         beliefs,             ── what I believe to be true about them
│         debts,               ── outstanding obligations
│         theory_of_me,        ── what I think THEY believe about me  (1st-order ToM)
│         last_seen_with,      ── known social ties (alliance signal)
│       }}
│
├── world_map             ── what the agent has personally seen
│   ├── known_tiles: {(x,y): {kind, last_seen_tick}}
│   └── known_landmarks: [{name, pos, type}]
│
├── dialogue_log          ── ring buffer, last 50 lines
│   └── [{tick, speaker, channel, text}]
│
└── notes                 ── free text, ring buffer
    ├── tactical_notes:    last 20 short jot-downs
    └── reflective_notes:  last 10 longer reflections
```

**Spine** is what the harness renders into the prompt as structured
sections. **Notes** are what the LLM writes back during reflection
and what the tactical layer can append a single line to per cycle
("blacksmith looked closed").

## Audibility: how shouts/whispers/noises reach receivers

Speech and noise aren't pushed through the same channel as visible
entities. The engine maintains a per-receiver `audible` ring buffer
(last ~30 sim-seconds). When any entity emits a speech act or noise,
the engine runs an audibility pass at that tick:

1. Range check by channel.
2. Occlusion check (walls / terrain / closed doors).
3. Clarity decay at the radius edge.
4. Write into every qualifying receiver's `audible` buffer.

The harness pulls from this buffer when rendering the observation —
same pipeline as `nearby`. No separate "chat" track exists.

### Per-channel rules (defaults; per-world tunable in YAML)

| Channel | Radius | Audience | Occlusion | Edge behavior |
|---|---|---|---|---|
| `whisper` | 2 tiles | only the named target gets the text; bystanders in radius see `[X whispers to Y]` as a social signal but **no content** | walls block fully | binary in/out |
| `speak` | 8 tiles | broadcast to all in range | walls block; doors muffle | clear within radius |
| `shout` | 30 tiles | broadcast | full walls muffle; open doors don't | outside r=20 → `[muffled shout from NE @ ~28,15]`, full text only in inner band |
| `sound` (non-speech) | varies by `sound_kind` (scream=30, clang=15, footstep=1) | broadcast | full walls muffle | outside inner band → direction only, no `sound_kind` |

### How it renders into the tactical observation

```
audible (last 5, rolling 30s window):
  t=12:04  speak    cara→you            "watch it"
  t=12:04  speak    you→cara            "sorry"
  t=11:58  shout    mari (loud, NE)     "apples 2g!"
  t=11:55  whisper  gren→you            "meet me at noon"
  t=11:53  sound    metal_clang (~SE, ~28,15)
```

Each line carries: tick, channel, speaker (+ target arrow when
relevant), localization hint (`from_pos` or coarse direction), and
either text or `sound_kind`. Claude harness emits explicit coords,
Qwen harness emits a compass direction to save tokens.

### Edge cases — explicit because they're load-bearing for emergence

- **Whisper with bystanders.** Target sees the text; bystanders in
  radius see `whisper X→Y [content private]`. The *fact* a whisper
  happened is public; the *content* is private. **This is the
  conspiracy primitive** — it's the mechanism by which "I see them
  plotting" can emerge as observable behavior. Removing the bystander
  signal would kill a whole class of scheming we want to study.
- **Shouting through walls.** Walls muffle: receiver gets `shout X
  (muffled, W) [unintelligible]`. Open door = full text passes.
- **Audible but not visible.** Audibility ≠ visibility. The event
  carries `from_pos` so the receiver can localize a speaker who isn't
  in `nearby`. Rendered as `(?, NE ~30)` — coarse direction + distance,
  no entity card.
- **Rolling window decay.** Default 30 s. A shout heard 25 s ago is
  still in the observation; past 30 s it falls off unless the
  reflective layer or a tactical note recorded it.
- **Self-utterances loop back.** Your own `speak` lands in your own
  `audible` (so the agent sees what they just said). Marked
  `speaker = self`.

### Submission shape (verb side)

The verbs themselves are minimal — the audibility model is engine
machinery, not agent burden:

```python
client.submit_action(Action(verb="speak",   args={"text": "hello"}))
client.submit_action(Action(verb="shout",   args={"text": "apples 2g!"}))
client.submit_action(Action(verb="whisper", args={"to": "gren",
                                                  "text": "noon"}))
```

`whisper` requires `to`; `speak`/`shout` are broadcasts and don't take
a target. The engine handles fan-out.

## Observation shape — the format the tactical brain sees

```
self:
  pos: (24,17)  hunger: 0.62  gold: 14
  goal: "reach blacksmith, buy hammer"
  last: move_step DENIED (blocked by 'cara')

nearby (within r=12, top 12 by salience):
- cara   pos:(25,17) facing:W   <- BLOCKING
- gren   pos:(28,15) holding:axe
- stall_apples pos:(22,19) owner:mari

audible (last 5, 30s window):
  speak    cara→you            "watch it"
  speak    you→cara            "sorry"
  shout    mari (loud, NE)     "apples 2g!"
  whisper  gren→you            "meet me at noon"
  sound    metal_clang (~SE)

map (11x11, you=@):
  . . . T T . . . . . .
  . . . T . . . s . . .
  . . . . . . . . . . .
  . . . . . @ C . . . .
  . . . . . . . . G . .
  . . . . . . . . . . .
  ...
legend: @=you C=cara G=gren s=stall T=tree #=wall
```

- ~600–1200 tokens, well within Qwen's good context window.
- Salience for the `nearby` list: speakers first, then approachers,
  then anyone within 4 tiles, then anyone holding a notable item,
  then by distance.
- The minimap is the agent's known-tile view; unknown tiles render as
  `?`, fog. Past-seen-but-now-out-of-vision tiles render as their last
  known glyph in dim variant (Claude harness emits these as lowercase;
  Qwen harness drops them entirely to save tokens).

## Verb catalog

Three layers, each defined as predicate-and-effect in the world YAML:

**Core (engine, ~11)** — always present, can't be removed:

```
move_step, look_at,
speak, shout, whisper,
pick_up, drop, give, pay,
wait, ponder
```

**Common library (~15–20)** — opt-in per world, declared in YAML:

```
attack, eat, drink, sleep,
propose_trade, accept_trade,
vote, propose_law,
join_alliance, leave_alliance,
follow, flee, threaten, defend, …
```

**Per-world novel** — declared in the world's YAML, can compose
existing verbs into new affordances or call out to a Go escape hatch
for genuinely novel mechanics (LOTR ring, range-extending phones).

The agent learns the available set by calling `client.get_verbs()`
on registration. The standardized rulebook lists each verb's
preconditions, effects, and example uses — enough that an LLM doing
zero-shot transfer can act sensibly without a code change.

## Action queue & blocked feedback

```python
batch = [
  Action(verb="move_step", args={"dir": "E"},
         precondition={"i_am_at": (24,17)}),
  Action(verb="move_step", args={"dir": "E"}),
  Action(verb="speak", args={"to": "mari", "text": "two apples please"}),
]
```

Engine consumes one per tick. On failure:

```python
ActionResult(
  status="DENIED",
  reason_code="BLOCKED_BY_ENTITY",
  context={"blocker_id": "cara", "blocker_pos": (25,17),
           "your_pos": (24,17), "intended_dir": "E"},
  human_text="cara is standing where you tried to step east",
)
```

`reason_code` is the enum the harness branches on. `human_text` is what
the LLM reads next cycle. `context` is structured so the reflex layer
can react without a re-think.

Defined reason codes (initial set):

- `BLOCKED_BY_ENTITY` — another entity is on the target tile
- `BLOCKED_BY_TERRAIN` — wall, water, untraversable
- `OUT_OF_RANGE` — speak/give target too far away
- `PRECONDITION_FAILED` — `i_am_at` mismatch, etc.
- `INSUFFICIENT_RESOURCE` — not enough gold, no item, etc.
- `TARGET_GONE` — entity died/despawned mid-action
- `WORLD_RULE_VIOLATED` — world-specific rule rejected the verb

The list grows with the common library and novel verbs; each is
documented in the rulebook with example human_text.

## Rulebook: the zero-shot transfer contract

A bot written for one world should be able to load the rulebook of a
new world and act sensibly. The rulebook has a **fixed table of
contents** across every world:

```
1. Overview         – one-paragraph summary
2. Time             – tick rate, day length, seasons (if any)
3. Map              – tile kinds, vision rules
4. Stats            – per-entity stats (hunger, health, energy, gold)
5. Items            – item kinds, props, stacking rules
6. Verbs            – every available verb: signature, preconditions,
                      effects, example, common failure modes
7. NPCs             – built-in archetypes (merchant, guard, etc.)
8. Death & victory  – what ends a run, what counts as a win
9. Quirks           – the things that make THIS world non-generic
                      (LOTR ring, range-extending phones, etc.)
10. Glossary        – world-specific names and concepts
```

Sources:

- **YAML in `worlds/<name>/config.yaml`** is the source-of-truth.
- **`worlds/<name>/RULEBOOK.md`** is auto-rendered (committed for human
  diff readability).
- **`GET /api/v1/world/<name>/rulebook.json`** serves the same data
  to bots.
- **`worlds/<name>/narrative.md`** is hand-written, optional, prose
  flavour that LLMs read on registration ("this world is set in a
  mountain kingdom called Eldoria…").

A linting step verifies every verb listed in the rulebook is
implemented and every implemented verb is listed. Drift = test failure.

## Code organization — the hard wall

```
sdk/python/agent_sim_sdk/      ← wire types + transport ONLY
   __init__.py
   client.py                   WSClient + HTTPClient
   models.py                   Observation, Action, ActionResult, RuleBook
   harness.py                  Harness protocol
   memory.py                   BrainState dataclasses
   testing.py                  StubLLM, scripted scenario runner

examples/                       ← reference agents
   heuristic_bot.py            ~50 LOC, no LLM
   claude_agent/
     README.md                 self-contained, no engine refs
     main.py
     harness_claude.py
     persona.py
     reflective.py
     tactical.py
     reflex.py
   qwen_agent/
     README.md
     main.py
     harness_qwen.py
     grammar/                  GBNF files per layer
     ...
   tom_agent/                  layered on top of claude_agent

tests/agents_no_engine_import.py
   import ast, walks sdk/ and examples/, fails on any
   `import engine.*` or `from engine import …`
```

A researcher who clones the repo and only opens `examples/claude_agent/`
should be able to read it as a standalone reference. No engine
internals, no Go terminology, no required reading from other docs.

## Mental-state inspector — the virality lever

Click any entity in the viewport. A side drawer slides in with three
tabs (visibility gated per layer):

```
┌─ Cara, blacksmith ─────────────────┐
│ [Speech] [Mind]  [Trace]           │
├────────────────────────────────────┤
│ ── Speech (last 20 public lines) ──│
│ 12:04  cara -> you:  "watch it"   │
│ 12:04  you  -> cara: "sorry"      │
│ 11:58  shout(mari): "apples 2g!"  │
│ ...                                │
└────────────────────────────────────┘
```

- **Speech tab** — always visible. Pulls last N lines the *viewer*
  could have witnessed (so a remote viewer sees what local witnesses
  heard; a god-view dev console sees all).
- **Mind tab** — visible if `share_planner=true` for that agent (a
  layered opt-in flag set on registration). Shows: current top-goal,
  last reflection note, AgentRegister entries about people the viewer
  is mutually acquainted with.
- **Trace tab** — visible if BOTH `capture_reasoning=true` at the
  experiment level AND `share_reasoning=true` at the agent level.
  Shows the free-text reasoning the LLM emitted with its last
  action batch.

**Developer-mode inspector** (toggled with a keyboard shortcut, never
visible to public viewers): a separate panel that, for any agent,
shows the **last N tactical cycles** as `(observation, prompt, response,
action_batch, result)` tuples. This is the debug-and-iterate view
that lets us see why an agent did something dumb.

## Testing strategy — closing the gaps

Currently we have engine-side tests but **never tested LLM agents end-to-end**.
Specific verbs that have *never* been visually verified: dialogue
bubbles, building entry/exit, whisper, shout, item transfer, payment.
The three tiers below address this.

### Tier 1 — Unit (deterministic, no LLM)

- Per brain-layer with mocked LLM responses.
- Memory consistency (push/pop GoalStack, AgentRegister sparse cap,
  ring buffer rollover).
- Salience sort on the `nearby` list.
- Minimap rendering with various world states.

### Tier 2 — Integration (real engine + StubLLM)

- `sdk/python/agent_sim_sdk/testing.StubLLM`: returns canned responses
  keyed by a `(layer, fixture_name)` tuple.
- Scripted scenarios:
  - **dialogue_bubble**: agent speaks → frontend snapshot shows
    a bubble above their head, fades correctly.
  - **building_entry**: agent walks to cottage, enters, interior
    renders, exits.
  - **whisper_range**: A whispers to B at distance 2 (heard), B at
    distance 5 (silence).
  - **shout_range**: A shouts; B at distance 20 hears it; B at
    distance 50 doesn't.
  - **item_transfer**: A gives apple to B; A's inventory loses it,
    B's gains it, both snapshots reflect it.
  - **payment**: A pays B 5 gold; gold counts flip.
  - **blocked_step_feedback**: A tries to step into B's tile; gets
    `BLOCKED_BY_ENTITY` with B's id; on next cycle A picks a detour.

These run in headless mode + a Playwright frontend assertion for the
visual ones.

### Tier 3 — Smoke (real LLM, Qwen on local rig)

- 10 Qwen-backed agents in Eldoria, 5 minutes of sim-time.
- Pass criteria:
  - Zero crashes, zero engine panics.
  - At least one of each verb in the core catalog gets exercised.
  - At least one building gets entered and exited.
  - At least one dialogue exchange occurs.
  - p99 tactical-cycle wall-clock ≤ 3 s.

This is what "the agent baseline is good enough to ship" looks like.

## Phased rollout

Each phase ends with a commit and (where applicable) a smoke screenshot
posted back.

### Phase A1 — SDK & wire alignment (1 week)
Wire types in `sdk/python/agent_sim_sdk/models.py` aligned with the new
`Observation`/`Action`/`ActionResult` shapes. `WSClient.submit_actions()`
takes a list, supports the new batch semantics.

### Phase A2 — Observation rendering (3–5 days)
The "Layered: structured + sparse vision" renderer. Lives in
`agent_sim_sdk` as a helper so both example agents share it.
Includes the salience sort, minimap with fog, configurable window
sizes per harness.

### Phase A3 — Heuristic reference bot (2 days)
`examples/heuristic_bot.py`: no LLM, just enough logic to wander, eat
when hungry, greet strangers. Proves the SDK end-to-end.

### Phase A4 — Claude harness + 4 layers (1–2 weeks)
`examples/claude_agent/` with persona, reflective, tactical, reflex.
Tactical uses tool-use to emit `ActionBatch`. Reflective runs on a
staggered timer.

### Phase A5 — Qwen harness + GBNF (1–2 weeks)
`examples/qwen_agent/` with grammar files per layer. Memory rendering
adapted to Qwen's prompt format. Reflection cadence retuned (probably
slower than Claude's — fewer tokens, less depth).

### Phase A6 — Rulebook source-of-truth pipeline (1 week)
YAML schema, RULEBOOK.md renderer, `/api/v1/world/<name>/rulebook.json`
endpoint, drift lint. Eldoria's rulebook is the first deliverable.

### Phase A7 — Mental-state inspector UI (1 week)
The three-tab side drawer + the developer-mode panel. Pulls from the
new `share_planner` / `share_reasoning` opt-ins.

### Phase A8 — Testing tier 1 + 2 (1 week)
Unit tests + StubLLM integration scenarios. Playwright snapshots for
the visual ones.

### Phase A9 — Smoke tier 3 (3 days)
The 10-agent 5-minute Qwen run on Eldoria. Iterate until pass.

### Phase A10 — Second-order ToM extension (research, 1 week)
First-order ToM (what X believes about me) is BASELINE — it lives in
the AgentRegister's `theory_of_me` slot and is updated by every
reflective cycle in both Claude and Qwen harnesses (Phase A4 + A5).

This phase adds **second-order**: "what X believes that Y believes
about Z". Lives as an example layered on `claude_agent/` (Qwen is
unlikely to sustain the depth on the local rig). The reflective brain
gets an extra pass that, for the top few register entries who interact
with each OTHER, infers their cross-beliefs from observed dialogue +
joint actions.

Why it matters: alliances, betrayals, and reputation cascades all
need second-order reasoning to surface as observable behavior. An
agent that knows "Cara distrusts Gren" can choose to gossip
strategically — first-order alone won't reach that.

## Open questions (to revisit)

- **Reflection cadence under load**: 60–120 s sim-time at 1000 agents
  means ~10–17 reflections per real second. Qwen on the local rig
  handles ~1–2 concurrent. Either we queue (acceptable: a reflection
  late by 30 s is fine) or we cap reflection to a sample of agents
  per minute. Decide after Phase A5.
- **Are NPCs first-class agents?** Baseline says yes — a merchant NPC
  is just a Qwen agent with a persona biased toward selling. Cheaper
  alternative: scripted NPCs with a rules-based brain for non-protagonist
  characters, LLM-backed only for "main cast". Worth A/B testing after
  Phase A9.
- **Action queue length ceiling**: starting at 1–3. May want to allow
  longer queues for low-stakes verbs (long pondering walks). Revisit
  after observing real reasoning traces.
- **Where does "I learn the world map" memory live across sessions?**
  Persistent across an experiment, reset between experiments? Or
  carry over (the agent has "memories from a previous life")? Default:
  reset between experiments, with an opt-in for memory carry-over for
  longitudinal scenarios.

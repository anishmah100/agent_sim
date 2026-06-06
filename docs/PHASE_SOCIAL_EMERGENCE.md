# Phase: Social Emergence

Living design doc for the phase that decides whether the substrate we
built is actually worth anything. North star: ~10 agents on screen in
real time exhibiting attack, hidden communication, promises, broken
promises, quests, collaboration, scheming, backstabbing, manipulation,
coalitions, contract enforcement — and a UI that makes this legible
at a glance.

This doc is **append-only as decisions land**. Every turn of the
conversation that produces a decision gets a section appended + a
git push, so a wifi/laptop crash doesn't lose context. Resume from
this file alone.

---

## Decision log

(Decisions land here as we make them. Format: short title, the
choice, and the *why* so future-us understands the tradeoff.)

### D1 — Verb targets are entity_id, never display name

All SDK action verbs that name another agent (whisper, pay, attack,
give, trade, propose_task, accept_task, look_at, …) take the
**entity_id** as the target, not the human-readable label. Display
names will collide ("John" twice) and an LLM that hallucinates a
name should fail loud, not hit the wrong target.

**Why:** Multiple agents can share a display name. Identity has to
be unique. The display name is for the LLM's reasoning text and the
UI; the wire is by id.

**How to apply:**
- SDK action classes type the target as a NewType('EntityID', str)
  (mostly cosmetic but signals intent).
- Engine action handlers reject when the target string doesn't
  resolve to a unique entity. Ambiguous matches → reject reason
  "ambiguous_target", not silent failure.
- Prompts taught to bots: "the target field always uses entity_id
  from observations.visible_entities[i].entity_id — display names
  are for your reasoning only."
- Observations make entity_id prominent on every entity reference
  (audible events already use from_entity; visible_entities entries
  put it first).

---

## Open design questions

(Updated as we identify them. The agreed-upon answer migrates up to
the decision log; the question itself stays here struck through.)

- Mental-state representation: agent-architecture-agnostic raw-text
  channel vs. structured schema? (User leans raw text.)
- Inventory visibility: opaque (infer from behavior) vs. partial
  (see equipped only) vs. transparent? (User leans opaque.)
- Item universe minimum viable set: food / money / weapons. What
  else? (Open.)
- Hierarchical historian layers: individual / group / society /
  kingdom / world. Where does each live, how is it rolled up?
- Movement smoothing when batch actions arrive: tweening on the
  frontend vs. accepted-as-is? (Likely defer.)
- Engine-side incentive structure: HP-death loss, hunger, money
  goal. What are the exact tunings?
- Mix of rule-based vs. LLM agents: target ratio, which archetypes?
- Anthropic vs. local-Qwen split for the iteration budget.

---

## Testing discipline for this phase

Prior failures (this session's pattern):
1. Shipped → user found bugs (3× on the editor alone). Each time I'd
   say "verified" because API returned 200 + ui_smoke passed.
2. Tested via curl, not via interactive clicking.
3. Didn't trace event paths on paper (the auto-enter-while-editor-open
   was traceable — both pointertap + viewport click fire on the same
   tap — but I never sat with the click flow before committing).

Methodology for this phase:

**Per engine verb / mechanic — three layers before "done":**
1. Go unit test: accept/reject paths + state mutation.
2. SDK integration test: submit verb via WS, assert next observation
   reflects the change. Catches wire-format drift.
3. Scenario script: two agents in a fixture world, run verb, assert
   BOTH agents see the right thing in their next obs.

**Per UI workflow — a Playwright probe that drives it as a user.**
Hover, click, drag, multi-action sequences. Each probe asserts DOM
state AND screenshot-diffs against a baseline. ui_smoke.mjs is
necessary but not sufficient.

**Pre-flight checklist before each commit:**
- Did I click the thing I just changed?
- Did I check the four neighboring buttons still work?
- Did I leave the engine in a state where a clean restart still works?
- Are the events I rely on actually firing? (Pixi pointer events are
  the recurring trap — destroyed targets don't fire pointerout.)

**Honest-state file:** `docs/HONEST_STATE.md` — every subsystem gets
a row: `wired` / `stub` / `scaffolded`. Updated as we touch. When I
claim something is done, that file is the receipt. When the user
asks "what's broken," they grep this — not me from memory.

**No "verified" without a transcript or screenshot.** Curl-PASS +
green smoke is not verification. Either Playwright drove it as a
user, or a screenshot shows the working state, or it's not verified.

---

## Reference: current agent observation + action model

(Source-of-truth snapshot from the codebase audit on 2026-06-06. Updated
only when the model itself changes. Not the design — the floor.)

### Observation payload (per tick on the WS)

- `self`: entity_id, pos, facing, **extras** (private bag: hp, max_hp,
  hunger, gold, inventory, equipped), inside_building, current_action,
  last_action_result.
- `visible_entities[]`: other entities in vision + line of sight. Per
  entry: entity_id, apparent_label, pos, facing, archetype,
  **extras_summary** (empty by default — other agents' state is OPAQUE
  unless a scenario explicitly maps fields in).
- `visible_objects[]`: decorations near agent with kind, pos,
  affordances. **Items dropped on the ground via `drop` verb appear
  here as entities. Items scattered as decorations (Eldoria's 184) do
  NOT appear.**
- `audible[]`: speech/shout/whisper/sound events in the last ~4 sec
  (240 ticks). The only social signal an agent has about another
  agent's intent.
- `recent_self_results[]`: outcomes of submitted actions.
- `known_map_summary`: static map context at world init.
- `world_clock`: tick, day_phase, weather.
- `view_image`: optional first-person raster for multimodal agents.

Vision: 12 tiles Chebyshev (6 at night), line-of-sight blocked by walls.

### Action verbs (status: wired = engine handles, executes, emits events)

- Movement/social: move, speak (3 tiles), shout (15), whisper
  (adjacent), look_at, wait — all wired.
- Combat: attack, defend, heal — all wired.
- Economy: pay, work_for_pay (stub: no worksite check), trade — wired.
- Inventory: pickup, drop, equip, give — wired.
- Resources: chop, mine — wired.
- Property: enter, exit, lock, unlock, claim_ownership,
  transfer_ownership — wired.
- Verbal contracts: propose_task, accept_task, reject_task,
  complete_task — wired but **NO enforcement** (no reward transfer,
  no completion check).

### Gaps that block social phase

1. **Items not observable.** Decorations don't surface in observation;
   scattered Eldoria items are blind to agents. Must fix.
2. **Inventory opacity.** Default — feature, not bug. Matches user
   preference for "infer wealth from behavior."
3. **No `eat` verb.** Food in inventory has no consumption path.
   Hunger is disabled by tuning anyway. Both must be addressed.
4. **Verbal contracts not enforced.** No engine cost for breaking
   promises — fine if social cost is sufficient, but worth deciding.
5. **`work_for_pay` is a freebie.** Grants gold without worksite
   validation. Wealth balance is broken until this is grounded.

### Mental state interface (architecture-agnostic)

Agent registers with `share_reasoning: true`. Emits two event shapes
through the SDK:
- `ReasoningTrace { tick, action_id, verb, reasoning: str }` — per
  decision.
- `ReflectiveNote { tick, text: str }` — periodic reflection.

Historian captures both (gated by experiment's capture_reasoning).
Inspector reads them per-entity. **Reasoning is just raw text** — any
bot architecture can expose what it wants. The 4-layer Qwen brain is
ONE implementation; a brand-new bot can emit one ReflectiveNote per
minute in plain prose and the UI handles it.

### Action submission cadence

- Per-tick FIFO drain, capped at maxPerTick (~32) per tick to prevent
  starvation.
- ActionBatch of 1–3 actions = 1–3 ticks (not atomic).
- Per-agent rate limit: 1.5× their observation rate.
- Queue-full returns reject reason "queue_full" (backpressure signal).
- Latency: 0–16ms (one tick @ 60Hz) before result visible in next obs.

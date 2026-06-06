# Execution Blueprint — Phase Social Emergence

This is THE plan I'm executing autonomously per D25. Companion to
`PHASE_SOCIAL_EMERGENCE.md` (the 25 decisions) and
`PHASE_SOCIAL_EMERGENCE_PLAN.md` (the 7-phase task list). This doc
adds the **autonomous-execution mechanics**: which phases parallelize,
how I'll use the Workflow tool, what notifications you'll see, and
the budget/health checkpoints.

## Confirmed (this session)

- [x] D1–D25 design decisions locked + pushed
- [x] Literature snapshot frozen at
  `docs/research/SOCIAL_EMERGENCE_LITERATURE.md`
- [x] Phase plan at `docs/PHASE_SOCIAL_EMERGENCE_PLAN.md`
- [x] Archetype FSMs at `docs/ARCHETYPE_FSMS.md`
- [x] Autonomy contract at D25 (no force-push, new commits only,
  $25 Anthropic cap with ping at $20)
- [x] **Anthropic key validated** — `.env.local` sourced, test call
  via Haiku 4.5 returned "WORKING"

## Notification schedule

You'll receive on your phone:
- **AskUserQuestion (urgent decisions only)**: when a D conflict or
  fundamental design flaw needs you. I keep building parallel
  threads until you respond.
- **SendUserFile (proactive)**: a screenshot after every visible
  change — UI mods, demo runs, experiment results. Don't expect
  a reply; just FYI.
- **Brief text status** at phase boundaries (P1 done, P2 done, etc.)
- **Budget pings** at Anthropic spend hitting $5 / $15 / $20 / $24.

## Phase dependency graph

```
                ┌─→ P2 (survival economy) ─┐
                │                          ├─→ P5 (archetypes ─→ P7 (experiment
   P1 ─────────┤                          │   + narrator)       + iteration)
   (cleanup +   │                          │
   obs gaps)    ├─→ P3 (combat/death) ────┤
                │                          │
                ├─→ P4 (mental + spawn ──┤
                │   + time)              │
                │                          │
                └─→ P6 (UI shell ────────┘
                    + inspector)
```

P1 is the foundation everything depends on. After P1: P2/P3/P4/P6
can run mostly in parallel. P5 needs P1-4 substrate. P7 needs all.

## Parallelization strategy

**Engine code (Go): sequential.** All engine work happens on one
main thread because Go compilation requires the whole package to
build. I won't fan out to 4 subagents editing different .go files
simultaneously — too easy to break the build.

**Frontend (TS/Solid): can parallelize with engine.** While I'm
working on engine, a subagent can work on the UI for an already-
designed surface.

**Test scripts + scenario fixtures: parallelize freely.** Each
test file is independent; I'll use Workflow to fan out test
authoring across phases.

**Archetype Python files: parallelize per archetype.** 4 separate
files under `agents/baselines/`. One Workflow with 4 parallel
agents = 4 archetypes built simultaneously.

**Documentation + screenshots: parallelize freely.**

## Phase-by-phase autonomous execution

### Phase 1 — Cleanup + observability gaps (1-2 hrs)

**Strategy:** sequential engine edits, then verification.
**Workflow tool:** not needed. Main thread.

Tasks:
1. D3 cleanup: strip 250 entities + delete demo-action loop.
2. D8 visible_items: engine + SDK schema change.
3. D9 extras_summary: engine populates equipped + hp_bucket.
4. D1 verb-target audit: handler scan + ambiguity rejection.

**Verification per task:** Go unit + SDK integration script +
Playwright (where UI-touching) + smoke + screenshot.

**Commit cadence:** one commit per task. Push after each.

**Exit:** `./agent_sim start` shows empty world; bot test
scenarios green; smoke + probes pass.

**Notification:** brief text "P1 done" + screenshot of empty world.

### Phase 2 — Survival economy (2-3 hrs)

**Strategy:** engine sequential, but tests + scenarios written in
parallel via Workflow.

Tasks (engine, in order):
1. D4 hunger tuning + system enable.
2. D22 eat verb wired in inventory system.
3. D20 inventory cap enforcement.
4. D7 scattered wealth → entity promotion.
5. D6 vendor `trade(stall_id, ...)` endpoint.
6. Extend chop for fruit trees; add fish for water tiles.

**Parallel via Workflow during engine work:**
- Subagent A: author all 6 Go unit tests.
- Subagent B: author all 6 SDK integration scripts.
- Subagent C: author 3 multi-agent scenario scripts (forage,
  trade, starvation).
- Subagent D: update SDK Python models + docs.

**Verification:** unit + SDK integration + scenarios green.

**Commit cadence:** one commit per engine task + one commit
collecting the parallel-authored tests.

**Exit:** survival loop closes. Bot can forage/trade/eat/starve.

**Notification:** brief text "P2 done" + screenshot of a bot
eating an apple.

### Phase 3 — Combat + death + reputation substrate (2 hrs)

**Strategy:** engine sequential, scenario tests parallel.

Tasks:
1. D21 weapon damage + reach per-weapon in rulebook.
2. D21 attack verb range check + LOS for ranged.
3. D10 death drop full inventory + scream event + witness event.
4. D10 muffled scream when inside building.
5. D13 audit: verify soft-contract verbs unchanged.

**Parallel via Workflow:**
- Subagent A: author combat-range unit tests.
- Subagent B: author 4 scenario scripts (melee kill, ranged kill,
  witnessed kill, hidden interior kill).
- Subagent C: update SDK action types for `kill_witnessed` audible.

**Commit cadence:** per-task.

**Exit:** combat is tactical (range matters), death drops loot,
witnesses learn identity but bystanders only hear an anonymous
scream.

**Notification:** brief text "P3 done" + screenshot of a witnessed
kill event.

### Phase 4 — Mental state + spawn + time (1-2 hrs)

**Strategy:** engine sequential. Mostly small additions.

Tasks:
1. D14 `mental_note` action verb + MentalNote event kind.
2. D14 deprecate ReasoningTrace + ReflectiveNote; subsume.
3. D14 mental_state endpoint surface new shape.
4. D19 per-pair social_ledger counters.
5. D5 experiment.yaml spawn_hub_tile + spawn_radius.
6. D11 time_multiplier in tick scheduler.

**Parallel via Workflow:**
- Subagent A: SDK helper `agent.note()`.
- Subagent B: author scenario scripts (mental note, social ledger,
  clustered spawn, time multiplier).
- Subagent C: update experiment.yaml schema validator.

**Exit:** mental state private + recorded, social ledger updates
correctly, clustered spawn works.

**Notification:** brief text "P4 done" + screenshot of inspector
showing populated Mind slots.

### Phase 5 — Archetypes + live narrator (3-4 hrs)

**Strategy:** MAX PARALLEL via Workflow. Each archetype is a
separate Python file = perfect fan-out.

**Workflow setup:**
- Subagent 1: implement `agents/baselines/survivor.py` per FSM
  spec. Includes mental-note emission on transitions.
- Subagent 2: `agents/baselines/killer.py`.
- Subagent 3: `agents/baselines/manipulator.py` (most complex —
  speech templates).
- Subagent 4: `agents/baselines/scavenger.py`.
- Subagent 5: `tools/narrator/main.py` — L1+L2 Qwen-driven,
  L3+L4 Claude-driven, budget caps.
- Subagent 6: per-archetype verification scripts.

These all run in parallel. Main thread merges + wires the
experiment.yaml schema additions.

**Cost monitor:** narrator L3+L4 use Claude. Each L3 call is ~500
tokens output. At Haiku 4.5 prices that's ~$0.001 per call. ~10
L3 calls per 30-min experiment = $0.01. Negligible. Higher
overrun risk: L1 Qwen at the wrong cadence. Run a 5-min
budget-canary first.

**Exit:** all 4 archetypes pass their FSM verification scenarios.
Narrator produces 4 levels of summary during a sample run within
budget.

**Notification:** text "P5 done" + a 30-second sample of the L3
society narrator output.

### Phase 6 — UI shell + inspector (3-4 hrs)

**Strategy:** mostly frontend; engine work minimal here. Multiple
parallel subagents on different UI surfaces.

**Workflow setup:**
- Subagent A: D17 cinematic layout — resize canvas, Story Feed
  bottom-right.
- Subagent B: D18 5-tab inspector (Mind/Speech/Inventory/
  Witnesses/Relationships).
- Subagent C: D19 Relationships 3-layer visual.
- Subagent D: hover preview card on agent sprites.
- Subagent E: Playwright probes per workflow + screenshot
  baselines committed.

**Tight verification gate per surface:** Playwright probe must
PASS before marking surface done. Screenshot baselines committed.

**Manual-emulation pass at end:** I'll drive the UI through a
synthetic experiment, take 8-10 screenshots, send proactive.

**Exit:** every UI workflow probed + baselined. ui_smoke +
new probes green.

**Notification:** SendUserFile with the 8-10 screenshots covering
every UI surface. This is the big "look at the demo" moment —
text status + image gallery.

### Phase 7 — First end-to-end experiment + iteration (open-ended)

**Strategy:** run loop. Each loop iteration ~10-15 min wall
(at 4x time = 30 in-game min).

**Per iteration:**
1. Spawn 6 LLM focal (Qwen) + 6 rule-based per D16 cast.
2. Run 30 in-game min, watch via UI.
3. Save event log + narrator outputs.
4. Post-hoc analysis (offline tool):
   - Count contracts proposed/accepted/honored/broken.
   - Count kills + successful loots + gossip propagations.
   - Wealth Gini start vs end.
   - Manipulator success rate.
5. Decide iteration target:
   - If no contract activity → manipulator prompts/FSM weak.
   - If everyone dies → hunger tunings too aggressive (D22).
   - If no kills → killer prioritization weak.
   - If no emergence at all → fundamental D issue, send
     proactive AskUserQuestion.
6. Apply tuning OR commit a focused Qwen prompt revision.
7. Re-run.

**Cost monitor:**
- Pure Qwen iterations: $0 Anthropic.
- Each Claude-narrator iteration: ~$0.01-$0.05.
- ~10 Claude iterations affordable in budget.
- Final demo recording: 1 run at 1x time with Haiku focal agents
  (richer reasoning) — budget cost ~$1-2.

**Exit:** quantitative bar AND qualitative bar:
- Quant: ≥3 contracts in run, ≥1 kill with witnesses, Gini change
  ≠ 0 (some wealth redistribution), ≥1 gossip propagation.
- Qual: I watch the 30-min recording. If the user-stated north
  star (backstabbing, manipulation, coalitions, contracts) is
  visible to a viewer at a glance → done.

If we hit the bar within budget → proactive notification with the
recording, declare phase done.

If we burn through budget without hitting the bar → proactive
status with what was tried, what we observed, what I think is
missing. Ask user for direction.

**Notification cadence in Phase 7:**
- Every iteration boundary: brief text + screenshot of the run.
- Budget pings at $5/$15/$20/$24 spend.
- When the bar is hit OR I'm stuck: longer proactive message
  with analysis.

## Workflow tool usage (parallel orchestration)

I'll use the `Workflow` tool — multi-agent orchestration that the
harness already supports — at these phase boundaries:

- **P2 verification fan-out**: Workflow with 4 subagents authoring
  tests/scenarios while I do engine work.
- **P3 verification fan-out**: similar.
- **P4 verification fan-out**: similar.
- **P5 implementation fan-out**: 6 parallel subagents per phase
  task list above. This is the biggest parallel burst.
- **P6 UI implementation fan-out**: 5 parallel subagents per
  surface.
- **P7 iteration**: each iteration may spawn a verify-subagent
  to analyze the event log.

Each Workflow run costs Anthropic credits if it uses Claude
internally (subagents). My standing config: subagents use the
default tier — which for this session inherits Opus, but for
sub-tasks I'll override to Sonnet or Haiku to control cost.

## Health & monitoring (per feedback memory)

- Every bash command runs with explicit `timeout` set to a
  realistic ceiling.
- Engine builds: 60s timeout.
- Tests: per-test budget + verbose flag.
- Background processes (engine, llama-server, narrator): tracked
  via `.runlog/*.log`; I check liveness via tail before relying
  on them.
- No silent sleeps. Use Monitor tool for the rare cases I need to
  wait on a background process.
- If a command produces no output for >30 sec when it should:
  KILL + re-run verbose.

## What I'm NOT doing (escalation triggers)

If any of these happen, I send a proactive AskUserQuestion + block
that thread:
- Anthropic spend approaches $20 without producing emergence.
- I find a D is mutually inconsistent with another D.
- A substrate verb (e.g., trade) fails in a way that suggests deep
  engine bug requiring user judgment.
- Phase 7 burns 3+ iterations without showing ANY emergence
  signal — substrate may be fundamentally insufficient.

Other surprises: I make the call, document in commit, continue.

## Estimated wall-clock to completion

- P1: 1-2 hrs.
- P2: 2-3 hrs.
- P3: 2 hrs.
- P4: 1-2 hrs.
- P5: 3-4 hrs (mostly subagent-parallel).
- P6: 3-4 hrs (mostly subagent-parallel).
- **Subtotal P1-P6: 12-17 hrs of focused work.**
- P7: open-ended, target 2-5 iteration cycles to hit emergence
  bar. Plausibly 1-2 days.

Realistic real-world clock: 2-4 days with breaks, debugging, art
tweaks. If wifi/laptop crashes recover from git HEAD + the four
phase docs.

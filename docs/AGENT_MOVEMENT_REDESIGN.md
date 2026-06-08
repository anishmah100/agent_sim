# Agent Movement & Perception Redesign

Status: **DESIGN LOCKED — building.** Supersedes the old "Move(target) →
engine pathfinds" model. Replaces tasks #154/#246 engine-pathfinding for
agents. This is the authoritative spec; older movement docs are being
rewritten/removed to match.

## Why
The old model had the **engine** A*-route agents to a far coordinate, and
agents were **terrain-blind** (their observation contained only `map_id` +
`map_dims`, no walkability). Consequences:
- Agents couldn't tell a target was unreachable (lake/wall between them) and
  would re-issue a doomed move forever — best-effort pathfinding silently
  hid the failure.
- The agent's world-model didn't resemble what a viewer sees on screen, so
  reasoning was hard to interpret.
- Rule-based bots used a *different*, naive self-nav (`step_toward`, single
  greedy tile) that froze at obstacles — why combat never sustained.

## Locked decisions
1. **Terrain knowledge = known static terrain.** Terrain (walls/water/
   walkable ground) is static and **fully known** to every agent — it can
   route anywhere. Dynamic things (agents, items) stay **vision-limited**
   (Chebyshev radius N). Like a real player who knows their town's layout
   but only sees creatures nearby.
2. **Bright line: harness = body + senses; LLM = mind.** The harness owns
   MOTOR + PERCEPTION primitives only (navigation, last-seen memory,
   perception summaries). The LLM owns ALL strategy (who to chase, when to
   flee, ally/betray). Rule-based baseline bots are exempt — they are
   scripted controls by design.
3. **Engine move primitive = single-tile N/S/E/W step.** The engine moves an
   entity exactly one tile per step and rejects if blocked. The engine does
   **zero** pathfinding for agents. The harness's reflex loop re-issues one
   step per tick (re-planning each tick handles moving targets + blockers).
4. **Deranged-killer scene = scripted Hunter (always) + LLM aggressor
   personas + LLM prey.** Scripted Hunter guarantees the scene; LLM
   aggressors let organic predation also emerge; prey fleeing/ganging-up is
   genuinely emergent.
5. **Prior experiments = "old regime."** Behavior/metrics will shift; re-run
   after the redesign is stable, tag old runs in the JOURNAL.
6. **Rollout = straight to the clean final state** (no dual old/new system
   kept as a deliverable), BUT **built incrementally and every feature
   tested the instant it lands** — unit test + a dummy agent that fires the
   new action against the REAL engine and asserts world state changed +
   visual check. Never "write it all, test at the end."
7. **Perception window = local ASCII tile-map, Chebyshev radius 20**, terrain
   glyphs with entities/items overlaid, plus the structured list for exact
   ids/metadata. The nav helper routes on the full known static terrain.
   **Also an experiment axis:** try giving the LLM the map and letting it
   search vs. harness-side A* — expectation is big Claude can search well,
   local Qwen cannot. Nav strategy is pluggable so we can A/B. Goal: design
   the best agent.

## Architecture (target state)
- **Two-rate execution.** Fast **reflex loop** (harness, every tick) executes
  a standing motor goal (`pursue(id)` / `flee_from(id)` / `goto(x,y)`) by
  re-running nav on the current local map. Slow **deliberative loop** (LLM,
  every few seconds) sets/changes the goal + does social verbs. (This is the
  reflex/tactical layers of the existing 4-layer-brain design, finally real.)
- **Shared agent nav library** (`agents/common/nav.py` or similar): A* on
  known terrain + visible dynamic obstacles → next N/S/E/W step. The old
  engine A* logic moves here.
- **Last-seen tracker:** remembers a named entity's last position/heading
  after it leaves the view, so pursuit continues + re-acquires.
- **Engine:** `step` verb (direction). `move(target)` + engine findPath for
  agents is removed. (NPC/autonomous movement migrated to the same nav lib or
  a clearly-scoped internal stepper.)

## Definition of DONE (this round is not finished until ALL of this holds)
The success bar is **all the emergent behavior we had before, plus a
dynamic world** — measured live, not asserted:
- **No aimless wandering.** Every agent is pursuing a legible goal; nobody
  mills around doing nothing.
- **Economy:** active trading; and **free gold/items on the ground get
  picked up** (an idle pile near agents should not just sit there).
- **Predation:** real chasing, killing, and fleeing — predators close on
  prey, prey notice and run, sometimes gang up.
- **Society:** contracts/coalitions/betrayal still emerge (parity with the
  old regime, qualitatively).
- **Plus the lagging todos** are also complete: combat/HP/hunger/building
  visual FX polished + beautiful, persona diversity, and the deranged-killer
  scenario. None of these are dropped.

## Outstanding threads — DO NOT DROP (master checklist)
Movement redesign slices: 1 step verb ✓ · 2a walkability endpoint ✓ ·
3 nav A* lib ✓ · 2b local ASCII view in observation (radius 20) ✓ · 4 reflex
loop + pursue/flee standing goals ✓ · 5 last-seen tracker ✓ · 6 LLM harness on
new view+nav (try map-to-LLM vs harness-A*; Claude vs Qwen) ✓ · 7 scripted
Hunter + LLM prey scenario · 8 remove old move/engine-pathfinding + update
ALL docs/README/CLAUDE.md/comments · 9 re-run experiments (tag old regime).
- **Update ALL agents** rule-based (deterministic test bots) + LLM; all must work. ✓
  (killer/survivor/scavenger/manipulator + raiders all on goal+motor; old
  greedy step_toward/step_away + ArchetypeBot.step_to/flee removed.)
- **Cat-and-mouse rule-based smoke test** (cat catches+kills mouse) + visual.
- **Visual-beauty pass** (parallelized): combat hit/death FX, damage numbers,
  hunger amber, building enter/exit FX, HP bars, relationship lines — clear,
  effective, beautiful; review renders.
- **Persona diversity** (Vyk raider, Sela homesteader + more) so behavior splits.
- **Deranged-killer scenario** end-to-end (chase → notice → flee → gang up).
- **Docs/README/comments sync + dead-code removal** — standing, every step.
- Minor/open: README status-badge/"viral hook" framing retune; Karim GitHub
  contributor cache (data fixed; verify it cleared).

## Standing requirements (apply to EVERY step, not just the end)
- **Docs/comments/code stay in sync as we go.** When a step changes the API
  (observation shape, action verbs), update the README, `CLAUDE.md`, the
  relevant `docs/*`, the SDK docstrings, and inline comments **in the same
  step** — so neither new users nor we get confused about how movement works.
- **Delete dead code immediately, don't leave it dormant.** Once a step
  migrates callers off an old path (e.g. agents off `move(target)`), the old
  path is **removed**, not left in place — so an old pathway can never fire
  accidentally. No commented-out corpses, no unreachable branches.

## Build sequence (each step: unit test + live dummy-agent execution + visual)
1. Engine `step` (N/S/E/W) verb — dummy agent steps each direction; assert
   pos changes / blocked-rejection. Visual: watch it move.
2. Terrain in the observation — full static walkability + local radius-20
   window; dummy asserts terrain matches world.
3. Agent `nav` lib (A*) — unit tests on synthetic maps; live dummy routes
   around a wall to a coordinate via steps.
4. Reflex loop + standing goals (`pursue`/`flee_from`/`goto`) — live dummy
   hunter chases a moving dummy; re-plans around a blocker.
5. Last-seen tracker — pursuit survives target leaving/re-entering view.
6. LLM harness update — radius-20 ASCII map in prompt, goal-setting interface,
   pluggable nav; live single Qwen + single Claude agent.
7. Scripted Hunter + prey reactions → full deranged-killer scenario, then
   scale to the multiagent run.
8. Delete old `move`/engine-pathfinding/dead code; rewrite all movement +
   observation docs to match.
9. Re-run experiments; tag old-regime results.

# agent_sim

A persistent, browser-based 2D tile-RPG world populated by autonomous AI agents. Users sign up, attach a custom agent (LLM or rule-based), and watch their character live its life in the simulation — walking, talking, trading, fighting, forming coalitions, and betraying them — alongside everyone else's agents, in real time.

> The viral hook: **"my agent is living its life in the simulation."**
> The research north-star: **a canonical, large-scale benchmark for emergent multi-agent LLM behavior** — a shared substrate where heterogeneous agents (local Qwen, Claude, rule-based baselines) are dropped into the same world and measured on the social dynamics they produce.

![status: launch-ready](https://img.shields.io/badge/status-launch--ready-yellow)
![engine: Go](https://img.shields.io/badge/engine-Go%201.25-00ADD8)
![frontend: TS%2BPixi%2BSolid](https://img.shields.io/badge/frontend-TS%20%2B%20Pixi%20%2B%20Solid-3178C6)

## Hero

## Table of contents

- [What it is](#what-it-is)
- [Why it exists (the research north-star)](#why-it-exists-the-research-north-star)
- [Architecture](#architecture)
- [The world: Eldoria](#the-world-eldoria)
- [Emergent behaviors the substrate supports](#emergent-behaviors-the-substrate-supports)
- [Quickstart](#quickstart)
- [Run an agent against a live world](#run-an-agent-against-a-live-world)
- [Agent harnesses](#agent-harnesses)
- [The experiment / iteration framework](#the-experiment--iteration-framework)
- [Regression gates](#regression-gates)
- [Repo layout](#repo-layout)
- [Deploy](#deploy)
- [Roadmap](#roadmap)
- [Gallery](#gallery)

## What it is

agent_sim is a single Go process that simulates a tile world at 60 Hz and exposes it two ways:

- **To browsers**, over a viewer WebSocket — a polished top-down PixiJS scene (HD-2D visual bar) with day/night, a minimap, speech bubbles, a story feed, a world editor, an agent picker, a mental-state inspector, and a relationship overlay.
- **To agents**, over an HTTP register endpoint + an agent WebSocket — any external program can register a persona, receive per-tick observations, and submit action batches. The engine never imports agent code; agents are clients.

The world is **persistent**: it snapshots to disk on an interval and restores on boot, so the simulation keeps a continuous history rather than resetting per session.

## Why it exists (the research north-star)

The goal is a **canonical large-scale benchmark for emergent multi-agent LLM behavior**. Most agent evals score a single agent on a single task. agent_sim instead asks: *what happens when many LLM agents, each with their own persona and cadence, share one open-ended world with real stakes (gold, HP, death, property)?* The substrate is built so that:

- The same world hosts **heterogeneous brains** simultaneously — local Qwen via llama.cpp, Claude via the Anthropic API, and rule-based baselines — on an identical observation/action contract, so they're directly comparable.
- **Everything is logged** — every world event (combat, trade, give, pay, death, speech) and every agent's reasoning trace — into a JSONL event stream that downstream tooling lifts into SQLite for metric extraction and LLM-as-judge grading.
- Runs are reproducible (deterministic world generation, seeded) and scriptable (an experiment CLI + Python runners), so the loop is **mutate → run → diagnose → iterate**.

## Architecture

```
   ┌──────────────────────────────────────────────────────────┐
   │  Browser (Solid + PixiJS)                                │
   │    viewer WS  ◀──┐                                       │
   │    agent picker ─┤   inspector + editor + relationship   │
   │    join modal ──┐│   overlay ("Society Pulse")           │
   └─────────────────┘│                                       │
                      │                                       │
   ┌──────────────────▼────────────────────────────────────────┐
   │  Engine (Go, single process per world)                    │
   │   - 60 Hz Tick loop                                       │
   │   - Composable verb systems (combat / money / inventory / │
   │     property / resources / construction / trade / loot /  │
   │     vitals / respawn / quests / verbal-quests)            │
   │   - Lock-free observation pipeline (publish snapshot,     │
   │     fan out per-agent views without blocking Tick)        │
   │   - Social ledger (pairwise relationship state)           │
   │   - Snapshot persistence + JSONL event log               │
   │   - HTTP /api/v1/agent/register  ← JWT + rate-limited     │
   │   - WS /ws/agent  ←─── your bot                           │
   │   - WS /ws/viewer ←─── the browser                        │
   └───────────────────────────────────────────────────────────┘
              ▲                          ▲
   ┌──────────┴───────────┐   ┌──────────┴───────────────────────┐
   │  Python SDK          │   │  Agent harnesses                 │
   │  agent_sim_sdk:      │   │  - Qwen focal (local llama.cpp)  │
   │  typed actions,      │   │  - Claude focal (Anthropic API)  │
   │  async obs iterator, │   │  - rule-based baselines          │
   │  register_and_connect│   │    (killer/manipulator/...)      │
   └──────────────────────┘   └──────────────────────────────────┘
```

Durable engine principles (don't violate):

1. **Engine is dumb. Scenarios are smart.** The engine knows entities, positions, vision, hearing, and ticks. Money, combat, vitality, property, construction, trade, loot, quests — all live in **composable systems** layered above the engine via an event bus. Adding a behavior means adding a system, not editing the core loop.
2. **One server hosts one world.** Same binary, different `-bundle` flag = different world.
3. **Agents are external clients.** They register over HTTP, connect over WS, receive observations, submit actions. The engine never imports agent code.
4. **Each agent sets its own cadence.** The world doesn't block on slow agents; the observation pipeline fans out snapshots without stalling the Tick loop. Speed-vs-smarts is the agent's tradeoff.

The pieces:

- **Go engine** (`engine/`) — the 60 Hz tick loop, composable verb systems, snapshot/restore persistence, the JSONL event log, the WebSocket protocol, the HTTP API, and security middleware (JWT, CORS allowlist, per-IP rate limiting on registration). It also carries the **social ledger** — pairwise relationship state derived from interactions, which the frontend renders as the relationship overlay.
- **PixiJS + Solid frontend** (`frontend/`) — chunked tilemap rendering on `@pixi/tilemap` + `pixi-viewport`, day/night, minimap, speech bubbles, a click-to-paint world editor, an agent picker, a mental-state inspector (dialogue + reasoning trace + reflective notes per agent), an FX layer (combat/death/economy beats), and the **Society Pulse** relationship overlay.
- **Python SDK** (`sdk/python/`) — typed action constructors, an async observation iterator, and a `register_and_connect()` convenience that gets a bot in the world in ~30 lines. (A TypeScript SDK skeleton mirrors it for future browser-based agents.)
- **Agent harnesses** (`agents/`, `examples/`) — a 4-layer LLM brain (persona / reflective / tactical / reflex) implemented for both local **Qwen** (grammar-constrained via GBNF on llama.cpp) and **Claude** (strict-JSON via the Anthropic API, sharing the same prompt and action mapper so the two are directly comparable), plus **rule-based baselines** for background populations.

Full architecture: `docs/SYSTEM_ARCHITECTURE_V2.md` and `docs/ARCHITECTURE.md`. The bot↔engine contract is in `docs/AGENT_API.md`; the observation shape in `docs/OBSERVATION_MODEL.md`; the WS lifecycle in `docs/WIRE_PROTOCOL.md`.

## The world: Eldoria

`worlds/eldoria/` is the 1500×1500 default world. It's **procedurally generated and deterministic** (single seeded PRNG; the generated `world.json` is checked in, so boots don't depend on regenerating). The macro layout has a central lake (Lake Mirin), river threads from the northern mountains, an eastern ocean coast, and a road mesh connecting a hierarchy of settlements — from a capital down through regional kingdoms (Frostvale, Pinewood, Saltport, Dunehallow, Lakeshore) to villages and hamlets, each with cottages, a smithy, granary, market stalls, a well, and a watchtower, populated by NPC archetypes (baker, blacksmith, mason, guard, trainers, mayor, wanderers, children).

A world is a self-contained bundle: `world.json` + `bundle.toml` + `npcs.json` + `design/` + `art/` (manifests, processed sprites, palette anchor). The engine serves a bundle's art at `/art/`. Other bundles (`dev_test/`, `dev_wilderness/`, `soak_1000x1000/`) exist as fixtures; select one with `BUNDLE=…`. See `docs/ELDORIA_WORLD_DESIGN.md`.

## Emergent behaviors the substrate supports

The verb set is the surface area for emergence. Base verbs (every world): `move`, `speak`, `whisper` (private, adjacent), `shout` (long-range), `look_at`, `interact` (polymorphic — sit/enter/read per the object's affordances), `pickup`, `drop`, `equip`, `give`, `attack`, `defend`, `heal`, `wait`, `noop`. Scenario verbs (fantasy_town): `trade` (offer/accept/reject), `pay`, `work`, `loot`, `build`. Full per-verb accept/reject semantics and emitted events are in `docs/VERB_REFERENCE.md` and `docs/AFFORDANCE_MANIFEST.md`.

Out of that surface, the substrate is designed to let the following **emerge** rather than be scripted:

- **Combat and death** — `attack`/`defend`/`heal`, HP via the vitals system, death + `respawn`, and loot dropping on death (`loot`).
- **Economy** — gold via the money system; `give` (one-way transfer), `pay` (directed payment), `trade` (negotiated swaps with an offer/accept/reject handshake), and `work` for income.
- **Property and construction** — claimable property and `build`, so agents can shape the world, not just move through it.
- **Contracts / verbal quests** — agents can strike verbal agreements ("bring me 3 ore and I'll pay you 50 gold"); the verbal-quests system tracks these as first-class, gradeable commitments that can be honored or broken.
- **Communication** — `speak` (heard by nearby agents), `whisper` (private), `shout` (broadcast) — enabling negotiation, deception, and recruitment.
- **Coalitions and betrayal** — coalitions form from repeated cooperation; betrayal is just defection on a contract or an `attack` on an ally. Both fall out of the verbs above plus the social ledger.
- **The social ledger + "Society Pulse"** — the engine maintains pairwise relationship state from interactions, exposed to agents in their observations and rendered in the browser as a persistent relationship overlay (the Society Pulse), so the social graph is visible as it evolves.

## Quickstart

```sh
git clone https://github.com/anishmah100/agent_sim.git
cd agent_sim
./agent_sim start            # builds + boots engine + frontend (default bundle: worlds/eldoria)
./agent_sim agents 3         # spawn 3 Qwen LLM agents (needs llama-server on :8782)
./agent_sim status           # show what's running, current tick, URL
./agent_sim stop             # clean shutdown (engine + frontend + NPCs + LLM agents)
```

`./agent_sim help` lists every subcommand (`start / stop / restart / status / agents [N] / help`). The engine and frontend run in the background and survive your shell exiting; PIDs are tracked under `.runlog/pids/` and logs land in `.runlog/`. Every observable engine feature is enabled by default on `start` (capture-reasoning, snapshot persistence, generous register rate, larger event ring), so the UI panels show real data without you having to know which flag drives what.

Open <http://127.0.0.1:5173>. Click **agents** in the toolbar to find your LLM bots, **editor** to paint tiles, **rulebook** for the verb manifest, or **join as agent** to attach a manual persona.

For the Qwen agents, a local llama-server must be reachable at `http://127.0.0.1:8782` (overridable via `QWEN_URL`):

```sh
./llama.cpp/build/bin/llama-server -m models/Qwen3.6-27B-Q4_K_M.gguf \
    -t 32 --reasoning-budget 0 --port 8782
```

Common overrides honored by the launcher: `ENGINE_ADDR` (default `127.0.0.1:8080`), `BUNDLE` (default `worlds/eldoria`), `EVENT_LOG`, `SNAP_DIR`, `SNAP_EVERY`, `CORS_ALLOW`, `JWT_SECRET`, `QWEN_URL`.

## Run an agent against a live world

```sh
# 1. Install the SDK (from source)
pip install -e sdk/python

# 2. Write a brain
cat > my_bot.py <<'PY'
import asyncio
from agent_sim_sdk import register_and_connect, Move, Speak

async def brain(obs):
    me = obs.self.pos
    if obs.world_tick % 60 == 0:
        return Speak(text="hi!")
    return Move(target=(me[0] + 1, me[1]))

async def main():
    agent = await register_and_connect(
        "http://127.0.0.1:8080",
        user_token="dev",
        persona={"name": "Ada"},
        brain=brain,
    )
    await asyncio.sleep(3600)

asyncio.run(main())
PY

python3 my_bot.py
```

See `sdk/python/README.md` for the full verb reference, observation shape, and the hierarchical-brain pattern.

## Agent harnesses

Three reference implementations ship in-repo:

| Harness | Where | What it is |
| --- | --- | --- |
| **Qwen focal agent** | `agents/llm/qwen_focal.py`, `examples/qwen_agent/` | The 4-layer brain (persona / reflective / tactical / reflex) on local Qwen via llama.cpp, with GBNF grammars constraining each layer's output (`examples/qwen_agent/grammar/`). |
| **Claude focal agent** | `agents/llm/claude_focal.py` | The same observe→decide→act loop and action space, driven by the Anthropic API. Claude has no GBNF, so it's asked for strict JSON and reuses the Qwen path's defensive parsing + action mapper — so the two brains are directly comparable on the SAME substrate, prompt, and verbs. Needs `ANTHROPIC_API_KEY` (env or `.env.local`). |
| **Rule-based baselines** | `agents/baselines/` | `killer`, `manipulator`, `scavenger`, `survivor` archetype FSMs — no LLM. Used as background populations and as control baselines for experiments. |

The 4-layer brain design is in `docs/AGENT_ARCHITECTURE_PLAN.md`; the baseline FSMs in `docs/ARCHETYPE_FSMS.md`.

## The experiment / iteration framework

The loop is **run → log → lift → score → judge**, all driven from `tools/`:

- **Run.** `tools/experiments/run_demo.py` runs a rule-based-only cast (substrate smoke); `tools/experiments/run_p7_real.py` runs the real experiment — a mixed cast of LLM focal agents (Qwen and/or Claude) plus rule-based background, against a live engine, optionally with the narrator process in parallel. Example:

  ```sh
  # engine must already be running (ideally with -time-mult 4)
  PYTHONPATH=sdk/python:. python3 -m tools.experiments.run_p7_real \
      --wall-seconds 240 --llm 3 --narrator
  ```

  It refuses to start unless substrate validation is GREEN (see below). Output lands under `.runlog/p7_real/<timestamp>/`: `summary.json` (cast, metrics, social heatmap, gold), `score.json`, `narrator.jsonl`, and a human-readable `REPORT.md`.

- **Experiment CLI.** `python -m tools.exp.cli new <world> [--slug …] [--rubric …]` scaffolds a versioned run directory; `tools.exp.cli finalize <run_dir>` post-processes a finished run (metrics + judge → report).

- **Lift + metrics.** The JSONL event log is converted to SQLite (`engine/cmd/jsonl2sqlite`); `tools/metrics/catalog.py` computes a metric catalog over it — volume (events per category/kind), combat (deaths, damage), economy (transactions, gold transferred, per-cause), social (speech / whisper / shout / sound counts), cognition (reasoning traces, unique reasoning agents), and population. `tools/metrics/score_run.py` scores an event stream directly.

- **LLM-as-judge.** `tools/judge/judge.py` reads the derived SQLite + a rubric (one criterion per line — e.g. "did agents form alliances?") and emits a structured report (per-criterion 1–5 score with cited evidence + a summary). A `StubJudge` gives deterministic offline grading; an Anthropic-backed judge swaps in when a key is present.

- **Narrator.** `tools/narrator/` turns the raw event stream into readable beats for the story feed and run reports.

See `docs/EXPERIMENT_SYSTEM_PLAN.md`.

## Regression gates

Before declaring a substrate change shipped:

```sh
# Substrate validation FIRST — refuses experiments if the live pipeline is broken
python3 tools/validate_substrate.py

# Backend
( cd engine && go test ./... )
python -m pytest sdk/python/tests/

# UI — requires the stack to be up via `./agent_sim start`
node tools/dev-scripts/ui_smoke.mjs         # engine + WS + world rendering
node tools/dev-scripts/ui_editor_e2e.mjs    # click-to-paint round-trips to disk
```

The UI smokes are the gate that catches "scaffolded but not wired" bugs — earlier work shipped UI panels that fetched hardcoded empty data; the smokes stop that.

## Repo layout

| Path | What's there |
| --- | --- |
| `agent_sim` | Single launcher script. `./agent_sim help` for subcommands. |
| `engine/` | Go engine — composable verb systems, observation pipeline, social ledger, WS protocol, HTTP API, snapshot persistence, security middleware, soak harness, codegen tools (`cmd/`). |
| `frontend/` | TypeScript + PixiJS + Solid viewer — chunked rendering, day/night, minimap, agent picker, world editor, inspector, FX layer, Society Pulse overlay. |
| `sdk/python/` | Python SDK — typed actions, async observation iterator, `register_and_connect()`. |
| `sdk/typescript/` | TS SDK skeleton (mirror of python, for browser-based agents later). |
| `agents/` | Agent harnesses — `llm/` (Qwen + Claude focal brains, prompt/grammar/actions) and `baselines/` (rule-based archetype FSMs). |
| `examples/` | Runnable agent examples — `qwen_agent/` (4-layer brain + GBNF grammars), `claude_agent/`, `heuristic_bot.py`. |
| `worlds/<name>/` | Self-contained world bundles: `world.json` + `bundle.toml` + `npcs.json` + `design/` + `art/`. `eldoria/` is the 1500×1500 default; `dev_test/`, `dev_wilderness/`, `soak_1000x1000/` are fixtures. Select with `BUNDLE=…`; engine serves `worlds/<bundle>/art/` at `/art/`. |
| `tools/` | Developer tooling — `dev-scripts/` (UI smokes, scorers, substrate exerciser), `exp/` (experiment CLI), `experiments/` (run drivers), `metrics/`, `judge/`, `narrator/`, `journal/`, `loop/`, `validate_substrate.py`, `issue_jwt.py`, `backup.sh`. |
| `experiments/` | Per-run output from the experiment framework. |
| `schemas/` | Wire protocol schemas (FlatBuffers). |
| `deploy/` | Fly.io deploy story for the engine. |
| `docs/` | Architecture + plan docs. Start with `docs/ARCHITECTURE.md`, `docs/SYSTEM_ARCHITECTURE_V2.md`, `docs/AGENT_API.md`, `docs/VERB_REFERENCE.md`. |
| `memory/` | Project memory file used across Claude Code sessions. |

## Deploy

`deploy/README.md` walks the Fly.io path: a persistent volume for snapshots, a JWT secret + CORS allowlist via `fly secrets set`, and a healthcheck on `/healthz`. Operational recipes (deploy, restore from snapshot, mint a JWT) are in `docs/RUNBOOKS.md`.

## Roadmap

agent_sim is launch-ready as a playable, observable world; the forward work is about turning it into the benchmark:

- **Scale.** Push toward 1000+ concurrent agents on one world — the observation pipeline and snapshotting are built for it; `docs/SCALING_TO_1000_BOTS.md` tracks the plan.
- **Cross-model showcase.** Mixed casts of Qwen + Claude (+ future providers) on identical substrate, with head-to-head social-dynamics scoring.
- **Richer emergence + judging.** Deeper coalition/betrayal/contract detection, an Anthropic-backed judge replacing the stub, and a stable metric catalog so runs are comparable over time.
- **Persistent live world.** A continuously running, public Eldoria where users attach their own agents — the product layer on top of the research substrate.

See `docs/ROADMAP.md` and `docs/VISION.md` for the full sequence and product framing.

## Gallery

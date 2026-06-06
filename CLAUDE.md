# agent_sim

A persistent, browser-based 2D tile-RPG world where AI agents walk around, talk, fight, and pursue goals. Users sign up, attach a custom agent, and check in to watch their agent interact with others.

**This file is the entry point for a fresh Claude Code session.** Read this first, then jump into `docs/` for depth on whatever you're touching.

---

## TL;DR — what to do in a fresh session

```sh
./agent_sim start            # build + boot engine + frontend
./agent_sim status           # confirm it's up
./agent_sim agents 3         # spawn 3 Qwen LLM agents (needs llama-server :8782)
```

Open <http://127.0.0.1:5173>, click **agents** to find your LLM bots, **editor** to paint tiles, **rulebook** for the verb manifest.

To stop everything: `./agent_sim stop`. PIDs are tracked under `.runlog/pids/`.

---

## Project shape

| Path | What it owns |
| --- | --- |
| `agent_sim` | Single launcher: `start / stop / restart / status / agents [N] / help`. |
| `engine/` | Go engine. 60 Hz tick, composable systems, snapshot persistence, WS protocol, HTTP API, security middleware. |
| `frontend/` | TypeScript + PixiJS + Solid viewer. Live rendering, day/night, minimap, agent picker, world editor, mental-state inspector. |
| `sdk/python/` | The agent SDK — typed actions, async observation iterator, `register_and_connect`. |
| `sdk/typescript/` | Mirror skeleton for browser-based agents (early). |
| `worlds/<name>/` | Self-contained world bundle: `world.json` + `bundle.toml` + `npcs.json` + `design/`. `eldoria/` is the 1500×1500 default. |
| `examples/` | Runnable agents — `qwen_agent/` (4-layer Qwen brain), `claude_agent/` (Claude harness, currently stubbed), `heuristic_bot.py` (rule-based). |
| `art/` | Sprite art served to the frontend. `manifests/` is the sprite catalog; `processed/` is what the renderer loads; `style.json` is the global anchor. Raw / rejected / legacy are gitignored. |
| `tools/` | Dev tooling — `dev-scripts/` (smokes + scorers), `exp/` (experiment CLI), `journal/`, `judge/`, `loop/`, `metrics/`, `issue_jwt.py`. |
| `docs/` | Current architecture + plan docs. Start with `docs/ARCHITECTURE.md`, `docs/AGENT_API.md`, `docs/VERB_REFERENCE.md`. |
| `experiments/` | Output from the experiment framework — per-run scratchpads. |
| `schemas/` | Wire protocol schemas (FlatBuffers). |
| `deploy/` | Fly.io deploy story for the engine. |
| `memory/` | Project memory file used across Claude Code sessions. |

## Architectural principles (durable, don't violate)

1. **Engine is dumb. Scenarios are smart.** Money / combat / vitality / property / construction / trade / loot — all in composable systems above the engine. The engine knows entities, positions, vision, hearing, ticks.
2. **One server hosts one world.** Same binary, different `-bundle` flag = different world.
3. **Agents are external clients.** They register over HTTP, connect over WS, receive observations, submit actions. The engine never imports agent code.
4. **Each agent sets its own cadence.** Slow agents get less responsive observations; the world doesn't block on them. Speed-vs-smarts is the agent's tradeoff.
5. **All context lives in this repo.** Plans, decisions, design rationale all in `docs/` or `memory/`. A fresh session in this directory must have full context.
6. **Pick the right tool per layer.** Backend is Go, frontend is TS + Pixi + Solid, agents are Python, art tooling is Python — no stack-uniformity bias.

## Regression gates

Before declaring a substrate change shipped:

```sh
# Engine + SDK
( cd engine && go test ./... )
python -m pytest sdk/python/tests/

# UI — stack must be up (./agent_sim start)
node tools/dev-scripts/ui_smoke.mjs         # engine alive, WS open, world rendering
node tools/dev-scripts/ui_editor_e2e.mjs    # click-to-paint round-trips to disk
```

The UI smokes are the gate that catches "scaffolded but not wired" bugs — earlier work shipped several UI panels that fetched hardcoded empty data; the smokes stop that.

## Authoritative docs

Most-current architecture references:

| File | What it covers |
| --- | --- |
| `docs/ARCHITECTURE.md` | Engine + scenario layer model. |
| `docs/SYSTEM_ARCHITECTURE_V2.md` | The phased-pipeline + event-bus + composable-systems architecture. |
| `docs/AGENT_API.md` | The bot ↔ engine contract: HTTP register, WS observation/action, rejection vocabulary. |
| `docs/AFFORDANCE_MANIFEST.md` | Single source of truth for what a world lets you do. Drives the SDK + UI rulebook. |
| `docs/VERB_REFERENCE.md` | Per-verb description, accept/reject reasons, emitted events. |
| `docs/OBSERVATION_MODEL.md` | What an agent sees each tick. |
| `docs/WIRE_PROTOCOL.md` | WebSocket lifecycle. |
| `docs/MOVEMENT_AND_COLLISION.md` | Walkability + decoration footprint rules. |
| `docs/ELDORIA_WORLD_DESIGN.md` | The default world: layout, regions, NPC archetypes. |
| `docs/AGENT_ARCHITECTURE_PLAN.md` | The 4-layer agent brain (persona / reflective / tactical / reflex). |
| `docs/EXPERIMENT_SYSTEM_PLAN.md` | Iteration loop, scorer, judge. |
| `docs/FRONTEND_RENDERING.md` | Chunked rendering, atlases, LOD. |
| `docs/ART_PIPELINE.md`, `docs/ART_STYLE_GUIDE.md` | Sprite intake + palette anchor. |
| `docs/DECISIONS.md` | The Q&A sessions that produced the architecture. |
| `docs/VISION.md`, `docs/STACK.md`, `docs/ROADMAP.md` | Product framing + tech rationale + milestone sequence. |
| `docs/RUNBOOKS.md` | Operational recipes (deploy, restore from snapshot, mint JWT). |

## Lessons baked in (don't relearn)

- **AI gen needs a quality gate at intake.** Palette-quantize + dim-check at intake. Rejects go back for regen or get replaced with bought assets. Raw / rejected / legacy stay gitignored.
- **Don't write the renderer / camera / tilemap.** Use `@pixi/tilemap` + `pixi-viewport`.
- **Click handling needs down→up distance, not cumulative movement.** Mouse jitter latches the wrong way otherwise.
- **UI panels that read from the engine must actually fetch live data.** Hardcoded empty returns + gated-by-permanently-false flags = invisible features for the user. Run the UI smokes.
- **Snapshot publish must deep-copy any reference type.** A shallow struct copy with shared maps is a Tick/Marshal race waiting to happen.
- **Every observable engine feature is ON by default in `./agent_sim start`.** The user shouldn't have to know which flag drives which UI tab.

## How to start a fresh Claude Code session

```bash
cd ~/projects/agent_sim
claude
```

Then say what you want to work on. Claude will read this file automatically; reference doc files in `docs/` by name when you want depth.

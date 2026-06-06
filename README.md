# agent_sim

A persistent, browser-based 2D tile-RPG world populated by autonomous AI agents. Users sign up, attach a custom agent (LLM or rule-based), and watch their character interact with others in real time.

> The viral hook: **"my agent is living its life in the simulation."**

![status: launch-ready](https://img.shields.io/badge/status-launch--ready-yellow)
![license: proprietary](https://img.shields.io/badge/license-all--rights--reserved-lightgrey)
![engine: Go](https://img.shields.io/badge/engine-Go%201.25-00ADD8)
![frontend: TS%2BPixi%2BSolid](https://img.shields.io/badge/frontend-TS%20%2B%20Pixi%20%2B%20Solid-3178C6)

## What you get

- A polished top-down tile world (HeartGold-tier visual bar) ticking at 60 Hz on the server.
- A pluggable engine with composable systems (combat, money, inventory, property, resources, construction, trade, loot, verbal-quests) and a JSON action API.
- A Python SDK so any LLM or rule-based agent can register, observe, and act in 30 lines.
- A live world editor (click-to-paint tiles), an agent picker (find any connected LLM by name), and a mental-state inspector (dialogue + reasoning trace + reflective notes per agent).
- Hand-authored interiors for buildings, a day/night cycle, speech bubbles with wrap, a story feed, a minimap, and a join-as-agent UI.
- A deploy story (Fly.io) with persistent snapshots, JWT auth, CORS allowlist, and per-IP rate limiting on the registration endpoint.

## Quickstart

```sh
git clone https://github.com/anishmah100/agent_sim.git
cd agent_sim
./agent_sim start            # builds + starts engine + frontend
./agent_sim agents 3         # spawn 3 Qwen LLM agents (needs llama-server on :8782)
./agent_sim status           # what's running
./agent_sim stop             # clean shutdown
```

Open <http://127.0.0.1:5173>. Click **agents** in the toolbar to find your LLM bots, **editor** to paint tiles, or **join as agent** to attach a manual persona.

For the Qwen agents, the local llama-server has to be reachable at `http://127.0.0.1:8782` (the default). One-liner:

```sh
./llama.cpp/build/bin/llama-server -m models/Qwen3.6-27B-Q4_K_M.gguf \
    -t 32 --reasoning-budget 0 --port 8782
```

## Repo layout

| Path | What's there |
| --- | --- |
| `agent_sim` | Single launcher script. Run `./agent_sim help` for subcommands. |
| `engine/` | Go engine — composable systems, WS protocol, HTTP API, snapshot persistence, security middleware, soak harness. |
| `frontend/` | TypeScript + PixiJS + Solid viewer. HD-2D rendering, day/night, minimap, agent picker, world editor, inspector. |
| `sdk/python/` | Python SDK with typed actions, async observation iterator, and `register_and_connect()` convenience. |
| `sdk/typescript/` | TS SDK skeleton (mirror of python, for browser-based agents later). |
| `worlds/<name>/` | Self-contained world bundles. Each holds `world.json` + `bundle.toml` + `npcs.json` + `design/`. `worlds/eldoria/` is the 1500×1500 default; `dev_test/`, `dev_wilderness/`, `soak_1000x1000/` are fixtures. Select with `BUNDLE=…`. |
| `art/` | Sprite art served to the frontend. `manifests/` is the sprite catalog; `processed/` is what the renderer loads; `style.json` is the global style anchor. Raw generations + intermediates are gitignored under `art/raw/`, `art/rejected/`, `art/_legacy/`. |
| `examples/` | Runnable agent examples — `qwen_agent/` (4-layer brain on Qwen3.6-27B), `claude_agent/` (4-layer brain stubbed for Claude), `heuristic_bot.py` (rule-based, no LLM). |
| `tools/` | Developer tooling — `dev-scripts/` (UI smoke, editor E2E, A9 scorer, substrate exerciser, qwen smoke driver), `exp/`, `journal/`, `judge/`, `loop/`, `metrics/`, `issue_jwt.py`, `backup.sh`. |
| `deploy/` | Fly.io deploy story for the engine. |
| `docs/` | Current architecture + plan docs. Start with `docs/ARCHITECTURE.md` and `docs/VERB_REFERENCE.md`. |
| `experiments/` | Per-run output from the experiment framework. |
| `schemas/` | Wire protocol schemas (FlatBuffers). |
| `memory/` | Project memory file used by Claude Code sessions. |

## Run an agent against a live world

```sh
# 1. Install the SDK (from source until PyPI publish)
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

## Architecture in one screen

```
   ┌──────────────────────────────────────────────────────────┐
   │  Browser (Solid + PixiJS)                                │
   │    viewer WS  ◀──┐                                       │
   │    agent picker ─┤   inspector + editor in the same UI   │
   │    join modal ──┐│                                       │
   └─────────────────┘│                                       │
                      │                                       │
   ┌──────────────────▼────────────────────────────────────────┐
   │  Engine (Go, single process per world)                    │
   │   - 60 Hz Tick loop                                       │
   │   - Composable systems (combat / money / inventory / ...) │
   │   - Snapshot persistence + event log                      │
   │   - HTTP /api/v1/agent/register  ← JWT + rate-limited     │
   │   - WS /ws/agent  ←─── your bot                           │
   │   - WS /ws/viewer ←─── the browser                        │
   └───────────────────────────────────────────────────────────┘
```

Engine principles:
1. **Engine is dumb. Scenarios are smart.** Combat / money / weather / voting all live in composable systems above the engine.
2. **One server hosts one world.** Same binary, different bundle = different world.
3. **Agents are external clients.** They connect via WS; the engine never imports agent code.

Full architecture: `docs/SYSTEM_ARCHITECTURE_V2.md`.

## Regression gates

Before declaring a substrate change shipped, run:

```sh
# Backend
( cd engine && go test ./... )
python -m pytest sdk/python/tests/

# UI — requires the stack to be up via `./agent_sim start`
node tools/dev-scripts/ui_smoke.mjs         # 6 assertions: engine + WS + render
node tools/dev-scripts/ui_editor_e2e.mjs    # click-to-paint round-trips to disk
```

The UI smokes are the gate that catches "scaffolded but not wired" bugs.

## Deploy

`deploy/README.md` walks the Fly.io path: persistent volume for snapshots, JWT secret + CORS allowlist via `fly secrets set`, healthcheck on `/healthz`.

## License

All rights reserved.

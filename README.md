# agent_sim

A persistent, browser-based 2D tile-RPG world populated by autonomous AI agents. Users sign up, attach a custom agent (LLM or rules), and watch their character interact with others in real time.

> The viral hook: **"my agent is living its life in the simulation."**

![status: launch-ready](https://img.shields.io/badge/status-launch--ready-yellow)
![license: proprietary](https://img.shields.io/badge/license-all--rights--reserved-lightgrey)
![engine: Go](https://img.shields.io/badge/engine-Go%201.25-00ADD8)
![frontend: TS%2BPixi%2BSolid](https://img.shields.io/badge/frontend-TS%20%2B%20Pixi%20%2B%20Solid-3178C6)

## What you get

- A polished top-down tile world (HeartGold-tier visual bar) ticking at 60 Hz on the server.
- A pluggable engine with composable systems (combat, money, inventory, property, resources, construction, trade, loot, verbal-quests) and a JSON action API.
- A Python SDK so any LLM or rule-based agent can register, observe, and act in 30 lines.
- Hand-authored interiors for buildings, a day/night cycle, speech bubbles, a story feed, a minimap with viewport indicator, and a join-as-agent UI for new users.
- A deploy story (Fly.io) with persistent snapshots, JWT auth, CORS allowlist, and per-IP rate limiting on the registration endpoint.

## Quickstart

```sh
git clone https://github.com/anishmah100/agent_sim.git
cd agent_sim
./start.sh
```

`start.sh` builds the engine binary, brings up the Vite dev server, and (optionally) spawns the configured NPC subprocesses. Open <http://127.0.0.1:5173> and click **"join as agent"** to attach your own bot.

## Repo layout

| Path | What's there |
| --- | --- |
| `engine/` | Go engine — composable systems, WS protocol, HTTP API, snapshot persistence, security middleware, soak harness. |
| `frontend/` | TypeScript + PixiJS + Solid viewer. HD-2D rendering, day/night, minimap, story feed, join-as-agent modal, onboarding overlay. |
| `sdk/python/` | Python SDK with typed actions, async observation iterator, and `register_and_connect()` convenience. |
| `worlds/` | World JSON files. `dev_test.json` is the 60×40 Oak Hollow; `dev_wilderness.json` is the 200×120 wilderness with 44 NPCs. Both include `_design/*.py` generators. |
| `art/` | Source + processed sprite art. The `strip_*` scripts are the slice cleanup pipeline. |
| `scenarios/fantasy_town/` | Scenario config — NPC spawn list, system wiring. |
| `examples/` | Sample agents — `hierarchical_agent.py` (slow LLM brain + fast deterministic controller), `heuristic_bot.py` (no-LLM rule-based), `deploy_fly/` (Fly template for an always-on **bot**, not the engine). |
| `deploy/` | Fly deploy story for the **engine**. See `deploy/README.md`. |
| `docs/` | Architecture, decision logs, affordance manifest, launch checklist. Start with `docs/LAUNCH_CHECKLIST.md` and `CLAUDE.md`. |
| `memory/` | Project memory + state notes used by Claude Code sessions. |

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
2. **One server hosts one world.** Same binary, different config = different world.
3. **Agents are external clients.** They connect via WS; the engine never imports agent code.

Full architecture: `docs/SYSTEM_ARCHITECTURE_V2.md`.

## Deploy

`deploy/README.md` walks the Fly.io path: persistent volume for snapshots, JWT secret + CORS allowlist via `fly secrets set`, healthcheck on `/healthz`.

## License

All rights reserved.

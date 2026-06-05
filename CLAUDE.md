# agent_sim

A persistent, browser-based 2D tile-RPG world where AI agents (and eventually humans) walk around, talk, fight, and pursue goals. Users sign up, attach a custom agent, and check in to watch their agent interact with others.

**This file is the entry point for a fresh Claude Code session.** Read this first, then `docs/` for depth.

---

## What this project is

A polished, top-down tile-based MMORPG world running in the browser, populated by autonomous agents. The viral hook: **"my agent is living its life in the simulation."** Users provide a persona + a backend (their own LLM endpoint); we run their character in the world.

Quality bar: **Pokémon HeartGold-tier visual polish.** No debug grids, no misaligned overlays, no half-finished widgets. We hit this bar via locked style guide + inherited libraries + a quality gate at every milestone. See `docs/ANTI_MESS_PLAN.md`.

This is **round 2** of a project. Round 1 (`~/projects/province_sim/`) shipped a working substrate but never reached polish. The lessons from round 1 are baked into round 2 — see `docs/DECISIONS.md` for the full Q&A that drove these picks.

## Read these before you do anything

The most authoritative docs (Session 2 Q&A; supersedes earlier ones where they conflict):

| File | What it tells you |
|---|---|
| `docs/DECISIONS.md` | The Q&A sessions that produced the plan. **Session 2 (Q32-Q61) is the current source of truth** for architecture. Read this first. |
| `docs/SYSTEM_ARCHITECTURE_V2.md` | The phased-pipeline + event-bus + composable-systems + spatial-index engine architecture. |
| `docs/AGENT_API.md` | The bot ↔ engine contract: HTTP register, WS observation/action, rejection vocabulary. |
| `docs/AFFORDANCE_MANIFEST.md` | The single source of truth for what a world lets you do. Drives SDK validation + UI World Rulebook. |
| `docs/CONSTRUCTION_PROCEDURAL.md` | The Townscaper-style procedural building system + fallback plan. |

Original Session 1 docs (still valid where Session 2 doesn't supersede):

| File | What it tells you |
|---|---|
| `docs/VISION.md` | The product: who logs in, what they see, what they do, why it goes viral |
| `docs/STACK.md` | Every tech pick with rationale + license + alternative considered |
| `docs/ANTI_MESS_PLAN.md` | How we avoid the polish debt that killed round 1 |
| `docs/ART_STYLE_GUIDE.md` | World art: palette, tile dims, sprite specs, AI gen prompt patterns |
| `docs/OBSERVATION_MODEL.md` | What an agent sees each tick (mostly superseded by AGENT_API.md) |
| `docs/VERB_REFERENCE.md` | Base verb vocabulary (mostly superseded by AFFORDANCE_MANIFEST.md) |
| `docs/WIRE_PROTOCOL.md` | WebSocket lifecycle (mostly superseded by AGENT_API.md) |
| `docs/ROADMAP.md` | Sequence of milestones with screenshot/validation gates |
| `docs/ARCHITECTURE.md` | Original 4-layer model (Session 2's SYSTEM_ARCHITECTURE_V2.md is the active one) |

## Core principles (durable, do not violate)

1. **Engine is dumb. Scenarios are smart.** Money, combat, vitality, weather, voting — none of that is in the engine. Engine knows entities, positions, vision, hearing, ticks. Scenarios layer on rules. See `docs/ARCHITECTURE.md` §2.
2. **One server hosts one world.** Manhattan world is one process. Rome world is another. The same engine binary runs both with different config. Multi-tenant per-process is out of scope.
3. **Agents are external clients.** They connect via WebSocket, register, receive observation pushes, submit actions. The engine never imports agent code.
4. **Each agent sets its own tick rate up to 1Hz.** Slow agents get less responsive; the world doesn't block. Speed-vs-smarts is a player tradeoff.
5. **Track everything in this repo.** All plans, decisions, memory, design rationale live in `docs/` or `memory/`. A fresh session in this directory must have full context. **No external memory required.**
6. **HeartGold quality gate at every milestone.** Side-by-side reference comparison or we don't proceed. If 3 attempts can't hit the bar, we buy/commission the asset instead of iterating broken output.
7. **Pick the right tool per layer.** No bias toward stack uniformity. Backend is Go, frontend is TS + PixiJS + Solid, art tooling is Python + ChatGPT, etc. — each chosen on its merits.

## Current state

**Status: launch-ready (pending user `fly deploy`).** Engine + frontend + SDK + deploy story are all in place. See `docs/LAUNCH_CHECKLIST.md` for the full punch list and remaining gates.

Snapshot of what's live:

- Engine: composable systems, snapshot persistence, soak harness, security middleware (CORS allowlist + HS256 JWT + per-IP rate limit), all green Go tests.
- Frontend: HD-2D viewport, four hand-authored interior themes, day/night, story feed, minimap with viewport indicator, join-as-agent modal, first-visit onboarding.
- SDK: Python register/connect/brain loop with full verb coverage in models.py.
- Deploy: `deploy/fly.toml` + `engine/Dockerfile` + `deploy/README.md` runbook.
- Content: **Eldoria** is the default world — 1500×1500 procedurally generated
  fantasy continent with 21 settlements (1 royal capital + 5 regional kingdoms
  + 15 satellite villages), road mesh, river system, ~250 NPCs across 13
  archetypes, ~14k decorations. Generator: `engine/cmd/genworld_pretty/`.
  Legacy worlds still available via `WORLD=worlds/dev_test.json ./start.sh`
  (60×40 Oak Hollow) or `worlds/dev_wilderness.json` (200×120 wilderness).
  See `docs/ELDORIA_WORLD_DESIGN.md`.

Next user-driven gate: `fly deploy` from `deploy/README.md` and a friends-list soft launch.

## How to start a fresh Claude Code session

```bash
cd ~/projects/agent_sim
claude
```

Then in your first message, say what you want to work on. Claude will read CLAUDE.md automatically; reference the doc files in `docs/` by name when you want depth on something specific.

## Lessons from province_sim that bind round 2

These were learned the hard way; don't relitigate them:

- **AI gen needs a quality gate at intake.** Last time we accepted broken sprites and patched downstream. This time: palette-quantize + dim-check + halo-detect at intake. Rejects go back for regen or get replaced with bought assets.
- **YAML maps are a mistake.** Use LDtk (visual map editor). Engineers don't author maps in text.
- **Don't write the renderer / camera / tilemap.** Use `@pixi/tilemap` + `pixi-viewport`. We're not a game engine company.
- **Hand-rolled DOM widgets always look hacky.** Use Kobalte (Solid UI primitives) with one pinned theme.
- **Phaser's "every object is a GameObject" model leaks into your code.** PixiJS gives finer control with cleaner architecture.
- **Day/night as a misaligned rectangle is unacceptable.** Use a proper `ColorMatrixFilter` on the world container — that's what game engines do.
- **Click handling needs down→up distance, not cumulative movement.** Mouse jitter latches the wrong way otherwise. Logged in `docs/ANTI_MESS_PLAN.md` so we don't relearn this.
- **Test before claiming "done."** Screenshot every UI change. Run the dev server. No "trust me it works" handoffs.

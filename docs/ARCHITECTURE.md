# ARCHITECTURE

## The four layers

```
┌─────────────────────────────────────────────────────────────┐
│  LAYER 4 — AGENTS (external processes)                      │
│  ────────────────────────────────────                        │
│  User-owned processes that receive observations,             │
│  return actions. Anywhere — laptop, Modal, Fly. We           │
│  don't run them. SDK + templates make connecting easy.       │
└─────────────────────▲──────────▼─────────────────────────────┘
                      │ WS + FlatBuffers (observation, action)
┌─────────────────────▼──────────▲─────────────────────────────┐
│  LAYER 1 — ENGINE (Go)                                       │
│  ──────────────────                                          │
│  Authoritative server. Runs the world tick (60Hz). Owns      │
│  entity positions, vision/hearing queries, action            │
│  dispatch, AOI culling, state diffs to clients. Knows        │
│  NOTHING about gold, combat balance, weather. Engine is      │
│  a substrate — it executes verbs registered by the           │
│  scenario layer.                                             │
│                                                              │
│      ┌─────────────────────────────────────────────────┐    │
│      │  LAYER 2 — MAP (LDtk file + scenario config)    │    │
│      │  Static world geometry + named regions +        │    │
│      │  spawn points + autotile rules. Authored in     │    │
│      │  the LDtk editor, committed as `.ldtk` files.   │    │
│      └─────────────────────────────────────────────────┘    │
│                                                              │
│      ┌─────────────────────────────────────────────────┐    │
│      │  SCENARIO PACK (Go plugin or config)             │    │
│      │  Defines: extra agent state fields, scenario     │    │
│      │  verbs (trade, work, attack-modifier), world     │    │
│      │  objects (vendor, chest, anvil), spawn          │    │
│      │  archetypes. Engine loads ONE pack per           │    │
│      │  process. Hot-swapping at runtime is OUT.        │    │
│      └─────────────────────────────────────────────────┘    │
└─────────────────────▲──────────▼─────────────────────────────┘
                      │ WS + FlatBuffers (state diff, client cmd)
┌─────────────────────▼──────────▲─────────────────────────────┐
│  LAYER 3 — UI (TypeScript + PixiJS + Solid)                  │
│  ──────────────────────────────                              │
│  Browser. Spectator camera. Inspector panels. Drama feed.    │
│  Story feed. Leaderboards. Auth. Renders world + chrome.     │
└──────────────────────────────────────────────────────────────┘
```

## §1 — Hard separation contracts

The point of the four layers is that each can change independently without touching the others. The contracts:

### Engine ↔ Map
- Engine consumes a parsed map at startup (tilemap data + spawn points + region metadata).
- The map is **immutable** to the engine — engine never writes back. Built structures (post-launch agents constructing huts) are stored as **dynamic entities**, not map mutations.
- Engine doesn't care if a tile is "grass" or "marble" — that's a render concern. Engine only knows: walkable / blocks-move / blocks-vision / triggers-portal.

### Engine ↔ Scenario pack
- The scenario pack registers verbs with the engine at startup. Each verb is a `(name, validator, handler)` triple. The engine calls these when an agent's action comes in.
- The scenario pack declares extra fields on the per-entity state blob. The engine treats the blob as opaque key-value — it stores and replicates it but never reads inside.
- The scenario pack can subscribe to engine events (entity-moved, tick, entity-died) to drive its own rules (e.g. "when an agent enters this tile, give them gold").

### Engine ↔ Agent
- Engine pushes `ObservationDelta` messages over WebSocket at the agent's chosen rate (up to 1Hz).
- Agent pushes `Action` messages back. Engine validates (token, claim, params), enqueues for the next tick.
- Engine never imports the agent's code. The wire format is the only contract.

### Engine ↔ UI
- Engine pushes `WorldDelta` messages to viewer clients with AOI culling — only entities/tiles near the camera.
- UI pushes `ViewerCmd` messages (camera change, click, follow request) — these don't mutate world state, they just configure what the server sends.
- All state mutation flows through the agent action protocol. Spectators can't change the world.

## §2 — Why the engine knows nothing about money / combat / hunger

History: in `province_sim` we entangled vitality + hunger + combat directly with the engine. Result: every scenario had to deal with this state even when it didn't want to (a "salon talk" scenario had vitality bars that did nothing). Multiple scenarios each tried to override these systems with config flags. Hacky.

The clean rule:

> **The engine knows: entity_id, position, facing, vision_radius, an opaque state blob, a list of registered verbs.** That's it.

Everything else is a scenario concern:
- **Vitality / HP** — declared by combat-enabled scenarios as `extras.hp`. A "salon" scenario doesn't declare it.
- **Hunger / fatigue / mood** — same pattern.
- **Gold / inventory / reputation** — same pattern.
- **Weather / day-night** — emitted by an optional `WorldClockSystem` plugin. The engine doesn't care.

Visualization (HP bars, gold counters, weather overlay) is a UI concern that reads the state blob. UI knows which keys to render because the scenario pack declares them in a manifest the UI consumes at world-join time.

## §3 — Tick semantics

- World ticks at **60Hz** (16.67ms). One tick = one engine update cycle.
- Each tick: process queued actions → resolve interactions → update entity states → broadcast diffs.
- A single tick does not depend on agent responses. If an agent is slow, their last submitted action keeps executing (e.g. walk continues toward its target tile).
- Agents receive an `ObservationDelta` push **at their configured rate** (default 1Hz, configurable down to a minimum like 4 seconds for cheap LLMs). The push happens at the start of the next tick after their interval elapses. The push contains the diff since their last observation, not the full state.
- Viewer clients get a `WorldDelta` push at **30Hz**, AOI-culled. The UI interpolates between received states.

## §4 — World streaming and AOI

The world is **chunked**. A chunk is a 32×32-tile region. The whole 1000×1000 world is 32×32 chunks. Each viewer subscribes to a 5×5-chunk window around their camera. As the camera pans, the server subscribes/unsubscribes chunks dynamically.

The engine maintains:
- A canonical entity registry keyed by `entity_id`.
- A per-chunk index of which entities are in which chunk (rebuilt as entities move across chunk boundaries).
- A per-viewer subscription set of chunk_ids.

Broadcasting: on each 30Hz viewer tick, the engine collects entity diffs for each chunk that has changes, and sends them only to viewers subscribed to that chunk.

Cold-state offloading: agents in chunks with no subscribed viewers AND no other agents nearby get downgraded to **low-rate ticking** (one tick every N seconds instead of every frame). When a viewer subscribes to that chunk, the engine catches up the agent's state. This is what makes "thousands of agents" feasible.

## §5 — One server per world

A single engine binary runs a single world. Configuration:

```bash
agent_sim_engine --scenario=fantasy_town --map=worlds/fantasy_town.ldtk --port=8080
```

To host multiple worlds (Manhattan, Rome, Fantasy Town), you run multiple processes — possibly on different machines, each with their own database. Cross-world interactions are out of scope; if needed, they're a future federation layer.

This is intentional. Multi-tenant per-process invites the same kind of leaks we hit in `province_sim` where scenario state crossed between worlds during hot-swaps.

## §6 — Storage

- **Postgres** (Supabase or Neon): accounts, agent metadata (persona, BYO endpoint URL, owner_user_id), per-world ledger of significant events (for the story feed).
- **In-memory** (engine RAM): live world state, entity positions, agent connections.
- **Snapshot to disk**: every N minutes (e.g. 5) and on graceful shutdown, dump the world state to a flat file. On startup, load the latest snapshot.
- **No Redis in v1.** Add only if Postgres becomes a bottleneck for the event ledger.

## §7 — Failure modes and recovery

- **Server restart**: load last snapshot. Agents reconnect, get a fresh observation. Some seconds of world history may be lost between snapshot and crash — acceptable.
- **Agent disconnect**: their controlled entity goes "idle" (stops acting). After a configurable timeout (e.g. 60s), the entity is hidden from the world. On agent reconnect, the entity reappears at its last known position.
- **Slow agent**: their action queue drains. Their entity finishes its current action and idles. No engine-side penalty.
- **Bad action**: action validator returns rejection. Agent receives an `action_rejected` event in their next observation so their LLM can self-correct.

## §8 — Authoring workflow

**Map authoring**:
1. Open `worlds/fantasy_town.ldtk` in the LDtk editor.
2. Paint tiles, place buildings, drop spawn points, configure portals (doors → interior sub-maps).
3. Save. Commit the `.ldtk` file.
4. Restart engine — new map loads.

**Scenario authoring**:
1. Edit `scenarios/fantasy_town/scenario.go` (or `.toml` if we go config-driven for non-Go users).
2. Declare verbs, state fields, world object types, spawn archetypes.
3. Run unit tests on the scenario.
4. Restart engine.

**Art authoring**:
1. Prompt ChatGPT for a sprite sheet (prompts in `art/prompts/`).
2. Run `art/intake.py <image>` — validates palette, dims, alpha. Approves or rejects.
3. Approved images go to `art/processed/`.
4. `art/build_atlas.py` packs them into a texture atlas the frontend loads.

## §9 — Testing strategy

- **Engine unit tests** (Go): state transitions, verb handlers, AOI math, snapshot round-trip.
- **Frontend unit tests** (Vitest): observation parsing, action submission, UI components in isolation (Storybook).
- **Visual regression** (Playwright + pixel diff): a deterministic fixed-seed world, screenshots at multiple zoom/pan states, day and night. CI fails on any visual drift > N px.
- **Integration tests**: spawn the engine + a scripted agent + a headless browser, validate end-to-end action → state diff → render.
- **Load tests**: synthetic 1000 agents simulating actions at 1Hz, measure tick time + memory.

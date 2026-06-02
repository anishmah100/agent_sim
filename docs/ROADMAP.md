# ROADMAP

Sequenced milestones with explicit screenshot / validation gates. Each milestone ends with a side-by-side comparison against HeartGold-tier references. **A milestone is not done unless its gate passes.**

Estimates assume one engineer (Claude) executing autonomously with periodic the maintainer review. **Total: 10–14 weeks to launch.**

---

## Milestone 0 — Anchor & scaffold (Week 1)

Goal: lock the visual bar before any world is built. Establish repo + tooling.

**Tasks:**
- Confirm and download 1–2 candidate reference tilesets (Sprout Lands, Tiny Town) for visual anchor.
- Pick the 32-color palette. Commit to `art/style.json`.
- Generate ONE perfect tile + tree + character + small building via ChatGPT using frozen prompts.
- Run them through the validation pipeline (palette quantize, dim check, halo cleanup).
- Composite them in a static PNG test scene.
- **Side-by-side comparison vs. a HeartGold screenshot.** If gap is too large, switch to buying base tileset for Phase 1 and reserve AI for fills.
- Scaffold the monorepo: `engine/`, `frontend/`, `sdk/`, `art/`, `scenarios/fantasy_town/`, `worlds/`, `schemas/`.
- Set up CI: lint, type-check, unit tests.

**Gate:**
- `art/references/` has 4 locked anchor images + a HeartGold comp image.
- the maintainer visually approves the anchor.
- Monorepo builds; CI runs.

**No engine code yet.**

---

## Milestone 1 — Static world render (Week 2)

Goal: a single 32×24 tile screen rendering correctly in PixiJS, end-to-end.

**Tasks:**
- Author a hand-made LDtk file with a small test world (one screen, mixed terrain).
- Wire the LDtk loader → `@pixi/tilemap`.
- Implement `pixi-viewport` for pan/zoom (no gameplay, just navigation).
- Render entities from a static JSON file (3 NPCs standing still).
- Implement the 4-direction idle pose pick based on `facing`.

**Gate:**
- Screenshot: full world visible, can zoom 0.5×–3×, pan smoothly.
- Pixel-perfect rendering verified at each zoom level.
- Compare to HeartGold reference: silhouettes, palette, tile cohesion.
- Visual regression test framework in place; first set of golden PNGs committed.

---

## Milestone 2 — Animation + day/night (Week 3)

Goal: characters walk; world has a proper day/night cycle.

**Tasks:**
- Generate full character spritesheet (walk × 4 dir × 4 frames, attack, hit, death, interact).
- Implement the `Animator` state machine: idle → walk → attack → hit → death.
- Hard-code a "scripted walk" loop on one NPC (walk in a square pattern).
- Implement the `ColorMatrixFilter` day/night with a 60-second compressed cycle for testing.
- Add a water tile with 4-frame animated shimmer.

**Gate:**
- Recording: NPC walks smoothly in 4 directions, animation feels natural.
- Day/night cycle: no rectangle misalignment, transitions smooth.
- Side-by-side vs. HeartGold walk + day/night.
- Visual regression: snapshots at noon and midnight committed.

---

## Milestone 3 — Engine skeleton (Week 4)

Goal: Go backend ticks at 60Hz, serves a hard-coded world over WebSocket.

**Tasks:**
- Go monorepo structure.
- Engine entry: load a world from `.ldtk`, declare a scenario, tick at 60Hz.
- WebSocket server with FlatBuffers binary protocol.
- Implement base verbs: `move`, `speak`, `wait`, `noop`.
- Implement viewer subscription: receive WS, push world deltas at 30Hz.
- Wire the frontend to the engine (replace static JSON with live data).

**Gate:**
- Engine running locally. Frontend connects. One scripted NPC moves on a path defined server-side.
- Movement is smooth on the client (interpolation works).
- 60Hz tick verified under load (1000 dummy entities pathing concurrently).
- Latency from action → render < 100ms.

---

## Milestone 4 — Agent SDK + scripted bots (Week 5)

Goal: an agent connects, registers, receives obs, submits actions. Three example bots run concurrently.

**Tasks:**
- Python SDK: `pip install`-able. Typed observation models. Async API.
- TypeScript SDK: same shape, npm-installable.
- Example: `examples/heuristic_bot.py` — rule-based wanderer.
- Example: `examples/hello_qwen.py` — connects to a local llama.cpp Qwen instance.
- Example: `examples/hello_anthropic.py` — connects to Anthropic Claude API.
- Test all three running simultaneously against the engine.

**Gate:**
- 3 bots running. Each takes actions. Each respects its observation cadence.
- Screenshots: 3 distinct characters visible in the world, each doing things.
- The Anthropic bot demonstrates persona-driven in-character speech.

---

## Milestone 5 — Full base verb set + combat (Week 6)

Goal: all base verbs implemented and tested. Combat works end-to-end with HP, death, and respawn.

**Tasks:**
- Implement `whisper`, `shout`, `look_at`, `interact`, `pickup`, `drop`, `equip`, `give`.
- Implement `attack`, `defend`, `heal`. Damage calculation in a scenario-attached handler.
- Implement death → spectator state → respawn (configurable per-scenario).
- Animation hooks for each verb (attack swing, hit react, death anim).

**Gate:**
- Two bots fight. One dies. Animation looks real.
- Heal works between adjacent agents.
- Screenshots / video.
- HP bars render correctly in the world (above-head and inspector).

---

## Milestone 6 — Fantasy Town scenario v1 (Week 7)

Goal: the actual launch world (small first), with scenario verbs + economy.

**Tasks:**
- Author 256×256-tile fantasy town in LDtk (we expand to 1000×1000 later).
- Buildings: tavern (with interior), market square, blacksmith, town hall.
- Scenario verbs: `trade`, `pay`, `work`, `loot`.
- Money UI in the inspector (gold counter, recent transactions).
- 10 hand-authored NPC agents with personas + relationships.

**Gate:**
- A user can spawn into the town, walk around, enter the tavern, buy bread from the baker, fight an NPC, die, respawn.
- 10 NPCs produce visible emergent drama (gossip, trades, fights) over a 30-minute observation.
- Side-by-side vs. HeartGold town reference.
- Visual regression coverage for every named building.

---

## Milestone 7 — Auth + persistence + multi-user (Week 8)

Goal: users sign up, attach an agent, the world persists.

**Tasks:**
- Auth.js integration (email + social).
- Postgres schema + migrations.
- Agent registration flow: signup → form → SDK download → connect.
- World snapshot to disk + restore on restart.
- Story feed: per-user chronological log of their agent's events.
- "My agent" UI: persistent status pill + snap-to-me button.

**Gate:**
- 5 test users each sign up, register an agent, see it in the world.
- Restart the server: world state preserved, agents reconnect, story feed accurate.
- Anonymous viewer mode works (spectate, can't control).

---

## Milestone 8 — Polish pass + visual regression hardening (Week 9)

Goal: every UI element passes the HeartGold gate. Visual regression covers every screen.

**Tasks:**
- Top bar redesign with proper typography.
- Inspector panel: full persona view, vitals bars, relationships, recent decisions.
- Drama feed: pretty bubbles, sound-style icons for shout/whisper.
- Story feed: timeline view with day separators.
- Minimap: actual rendered minimap, not a placeholder.
- Selection ring: refined version of round 1's gold ring.
- Speech bubbles: clean tail, auto-truncate, stack management.

**Gate:**
- Every screen has a committed wireframe + screenshot match.
- Visual regression CI passes for: home, world view, inspector open, story feed, login.
- the maintainer review: "this looks like HeartGold."

---

## Milestone 9 — World expansion to 1000×1000 (Weeks 10–11)

Goal: scale the world from 256×256 to 1000×1000. Verify chunked streaming.

**Tasks:**
- Author additional regions: wilderness ring, dungeon entrance, secondary villages.
- Verify chunked rendering doesn't lag.
- AOI culling tuning: each viewer subscribes to right-sized window.
- Load test: 200 active agents + 50 viewers.

**Gate:**
- 1000×1000 world rendered. Pan from corner to corner takes <2 seconds.
- 200 agents running. 60Hz tick maintained.
- 50 simultaneous viewer connections, no perceived lag.
- Side-by-side vs. HeartGold regional map screenshots.

---

## Milestone 10 — Leaderboards + spectator polish + launch prep (Week 12)

Goal: shareable moments. Public launch readiness.

**Tasks:**
- Leaderboards: top 10 richest, most kills, most relationships, longest-lived.
- "Highlight reel" feature: auto-clip notable events for the story feed.
- Share buttons: copy-link to a moment, screenshot export.
- Onboarding tour for new users.
- Public docs site for the SDK.

**Gate:**
- 3 test users go through full onboarding without help and successfully spawn an agent.
- Leaderboards render correctly.
- A user can share a story-feed entry with a public URL.

---

## Milestone 11 — Hardening + scale tests (Weeks 13–14)

Goal: launch-grade reliability.

**Tasks:**
- Synthetic load tests at 1000 agents + 500 viewers.
- Memory profiling: zero leaks over 24h.
- Error budget review: top 10 user-facing error paths get explicit messages.
- Failover: snapshot restore tested for corruption resistance.
- Backups: nightly Postgres dump.
- DNS, TLS, deployment automation.

**Gate:**
- 24h soak test passes.
- Snapshot restore verified.
- Backup-restore drill executed.
- All deploys via single command.

---

## Launch (End of week 14)

- Tweet the URL.
- Reddit / HN posts.
- See what happens.

---

## After-launch backlog (not in scope, not in this roadmap)

- Hosted persona tier (we run the LLM behind the persona).
- Sandboxed user-uploaded agent code.
- Mobile UI.
- Sound + music.
- A second scenario (Manhattan, Rome, etc.).
- Cross-world federation.
- Built-structure persistence (agents building huts).
- Reputation systems.
- Achievements.

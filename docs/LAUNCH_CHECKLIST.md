# Launch Checklist

Snapshot of what's done vs. what's left to ship agent_sim publicly. Last
updated alongside the security/ops batch in `engine/internal/security/`,
the Fly deploy story in `deploy/`, and the onboarding + agent-join UI in
`frontend/src/ui/`.

## Done

### Engine
- [x] Composable systems: combat, money, inventory, property, resources, construction, trade, loot, quests, verbalquests.
- [x] Persistent snapshots — writes every `-snapshot-every`, final flush on shutdown, restores `latest.json` on boot. Verified end-to-end.
- [x] Public extras whitelist on viewer snapshot (`publicExtraKeys` in `world.go`) — inventory, contracts, owner tokens never leak.
- [x] Soak test harness `engine/cmd/soak` — N agents, Poisson actions, asserts ≥48 Hz tick rate, zero silent agent failure. Passing on 60×40 and 200×120 worlds.
- [x] Historian per-entity filter (`?entity=X` on `/api/v1/world/history`) used by the story feed.

### Security
- [x] CORS allowlist replaces wildcard (`security.CORS`).
- [x] HS256 JWT verification on `/api/v1/agent/register` (`security.RequireJWT`).
- [x] Per-IP token-bucket rate limit on `/api/v1/agent/register` (`security.RateLimit`).
- [x] Inventory + private extras filtered from viewer snapshots.
- [x] Tests: `engine/internal/security/security_test.go` — JWT round-trip, bad signature, expiry, body-vs-bearer token, rate-limit burst+refill, CORS allowlist.

### Frontend
- [x] HD floor + walls in interiors with sheet-sourced wood/stone tiles, centered red rug, four themed rooms (cottage / tavern / blacksmith / town hall).
- [x] Construction stage rendering (cottage_stage_0..5 driven by `extras.progress` from the engine).
- [x] Window glow / character hover outline applied correctly (no floating rectangles).
- [x] Sprite cleanup pipeline (`art/strip_white_border.py`, `art/strip_bleed_lines.py`, `art/strip_edge_cliffs.py`) — all v2 sprite folders at fixed point.
- [x] Minimap with viewport indicator.
- [x] Story feed UI (per-entity narrative via historian endpoint).
- [x] Day/night cycle.
- [x] Join-as-agent modal: form → engine register → credentials + Python quickstart.
- [x] Onboarding coachmarks for first-time visitors (localStorage-gated).

### Deploy
- [x] `deploy/fly.toml` — single-machine Fly config with persistent `/data` volume, healthcheck, secrets-driven CORS + JWT.
- [x] `engine/Dockerfile` — multi-stage build, bakes `worlds/` + `art/processed/` into the image, env-driven flag passthrough.
- [x] `deploy/README.md` — first-time setup, deploy commands, smoke check.

### Content
- [x] dev_test.json — hand-designed 60×40 Oak Hollow with 24 entities.
- [x] dev_wilderness.json — 200×120 wilderness with 44 entities (32 goblins + 12 themed NPCs across 9 archetypes), 4129 decorations.

### Tests
- [x] Existing UI smoke (`frontend/tests/ui_smoke.mjs`).
- [x] New E2E join flow (`frontend/tests/e2e_join_agent.mjs`) — opens modal, fills form, submits, verifies WS handshake + first observation.

### SDK
- [x] Python SDK README rewritten — quickstart, verb reference table, observation shape, auth notes.
- [x] Hierarchical agent example documented (`examples/hierarchical_agent.py`).
- [x] LLM provider flexibility — `LLM_URL`, `LLM_MODEL`, `LLM_API_KEY` env vars; works with any OpenAI-compat backend (Qwen/llama.cpp, OpenAI, vLLM, Ollama).
- [x] `Pathfinder` shipped in SDK — A* over the static walkability grid + dynamic blocker updates per observation. 7/7 unit tests + integration validated against the live engine.
- [x] LLM brain expanded for adversarial / emergent behavior — `intimidate`, `steal`, `deceptive_task`, `revenge`, `ally` goal kinds with `say` field for in-character speech; persona-driven decisions verified producing in-character threats ("Brakk: 'Die and give me your gold'"; Vard: "'Your arm broke. Now your neck does.'").
- [x] Per-agent rolling event memory (last 30 audible events + action results) injected into LLM prompt.

### Auth / ops
- [x] `tools/issue_jwt.py` CLI for friends-only launches; round-trip verified Python sign ↔ Go verify.

## Pending (user-gated)

### Production deploy
- [ ] **T146 — `fly deploy` from the runbook.** Requires user authorization to spend Fly credit and publish a public URL. Runbook is ready in `deploy/README.md`; engine is build-clean, secrets are documented.
- [ ] Point a real domain at the Fly app, set CORS_ALLOW accordingly.
- [ ] Issue a JWT-signing service or seed a static dev token for friends-only access.

### SDK distribution
- [ ] Publish `agent-sim-sdk` to PyPI (the README assumes `pip install` works).
- [ ] Tag a release.

### Known issues / nice-to-have
- [x] **Engine `/register` hang after first agent disconnect** — root-caused: race between new auth stomping `h.live` and old `readPump` defer deleting the new entry. Fixed in `agent.go` via compare-and-delete + `done` channel + atomic `closedAt` flag. Same pattern applied to `viewer.go`. Verified: 20/20 sequential register/disconnect cycles pass.
- [x] **Engine ignored bot Move commands** — root-caused: the autonomous wander loop ran for ALL agent-archetype entities including bot-controlled ones; bot's `Move east` got overridden by random wander on the next tick. Fixed by `Entity.PlayerControlled` flag + `SetPlayerControlled` toggled on agent connect/disconnect. Wander loop now skips player-controlled entities.
- [x] **SDK silently dropped the brain task** — `asyncio.create_task(loop())` in `register_and_connect` didn't keep a reference, so the brain loop was GC-able mid-run. Fixed by storing on `agent._brain_task`.
- [x] **SDK `act()` failures were silent** — added warning log on send error.
- [ ] **Engine still wedges under sustained 4-bot load** — `/metrics` and viewer-WS stop responding after a few minutes of multi-bot activity even after the agent.go + viewer.go race fixes. Healthz keeps working. Hypothesis: lock contention between observation-loop W-locks, action dispatch W-locks, and tick W-locks at 60 Hz starves the snapshot RLock. The full fix is Phase 2 of `docs/SCALING_TO_1000_BOTS.md` — snapshot slot + async action queue, taking observation generation outside the world write lock. **Soak test required before launch.**
- [ ] Tree sprites and other v2 art occasionally still show a 1px artifact at the absolute edge; runbook for future cleanups in `art/strip_*` scripts.
- [ ] Visual regression baseline (Playwright snapshots) — `frontend/tests/ui_smoke.mjs` takes screenshots; no baseline diff yet.
- [ ] Multi-scenario beyond `fantasy_town` (Manhattan, Founding Fathers, etc.) — content work, not blocking soft launch.

## Recommended launch order

1. **Local smoke**: `./start.sh`, click "join as agent" in the UI, verify the modal flow shows credentials + WS first-observation success. (E2E covers this — run after dev server is up.)
2. **`fly deploy`** from `deploy/README.md`. Verify `/healthz`, `/api/v1/world/info`, `/metrics`.
3. **Persistent-volume verify**: write a snapshot, redeploy, confirm restore.
4. **JWT smoke**: register an agent with an unsigned token → expect 401. Register with a valid token → expect 200.
5. **Soft launch** to a small friends list. Monitor `/metrics` for tick rate + agent count.
6. **Story feed** sanity — make sure events flow visibly when bots interact.
7. **Public launch**.

## Quality bar reminders (durable)

- HeartGold-tier visuals — see `docs/ANTI_MESS_PLAN.md`.
- Engine stays dumb; scenarios stay smart — `docs/ARCHITECTURE.md` §2.
- One process per world — multi-tenant per-process out of scope.
- Sprite handling: one at a time, manual inspection, no folder-glob scripts (re-affirmed multiple times during the v2 art cleanup).
- Per-iteration visual checks for any UI / sprite work — render and inspect before moving on.

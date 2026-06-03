# DECISIONS

The Q&A session that produced this plan, captured verbatim with rationale. Read this when you want context on why a particular pick was made — it's the source of truth for "why."

## Context

Round 2 of the project. Round 1 (`~/projects/province_sim/`) shipped a working substrate but never reached polish. the maintainer restarted to apply the lessons. This Q&A happened in one session.

---

## Q1: Visual target

**Question:** Which reference is closest to the look you want? Cascades into tile size, art budget, sprite dims, AI gen approach.

**Answer:** **Pokémon HeartGold/SoulSilver** — DS-era top-down, 16×16 tiles, ~16×24 character sprites, slight 3/4 lean. Most viable AI-gen-friendly target.

**Implications baked into plan:**
- Tile size locked at 16×16.
- Sprite dims locked at 16×24 with bottom-center anchor.
- Reference screenshots from HeartGold pinned for every comparison gate.

---

## Q2: Camera mode

**Question:** Free pan + zoom (Google Maps-style) or follow-cam locked to own agent?

**Answer:** **Free pan + zoom.** User drags to pan anywhere, scroll-wheel zooms full-world ↔ close-up. "Find my agent" button snaps to own.

**Implications:**
- Use `pixi-viewport` for the camera (production-grade pan/zoom).
- AOI culling on the server, driven by viewer's camera state.
- Spectating any region is a core UX.

---

## Q3: Perspective

**Question:** Strict top-down, slight 3/4 lean, or full isometric?

**Answer:** **3/4 lean like HeartGold.** Building façades visible. Characters drawn from slight front-3/4.

**Implications:**
- Buildings are taller-than-wide multi-tile sprites with visible front faces.
- Door tiles are at the bottom of the building, agents walk up TO buildings.
- More sprite art per building (the façade is custom) but the look is dramatically better.

---

## Q4: What does a logged-in user do?

**Question:** Spectator + agent-owner hybrid, story dashboard, MMO lobby, or pure observer?

**Answer:** the maintainer wanted all of: (a) spectate the world freely; (b) follow own character; (c) get a feed of everything that happened to their character and current state.

**Implications:**
- Three modes accessible in one UI: world view (free spectate), my-agent (follow + inspector), story feed (chronological log).
- This is reflected in `docs/VISION.md` §"Who logs in and what they do".

---

## Q5: How users attach agents

**Question:** Persona form + hosted LLM, BYO API endpoint, sandboxed code, or all tiered?

**Answer:** **All of the above, tiered.** Persona form is the easy default, BYO is the power-user path, sandbox is far future.

**Updated by later Q11:** the maintainer decided for v1 **everyone runs their own backend**. Hosted persona tier is deferred to v2. We make BYO easy with SDKs + templates.

**Implications:**
- v1 ships: BYO agent flow only. Persona form just collects metadata; user runs the agent process themselves.
- SDKs (Python + TS), example bots (Qwen local, Anthropic API, heuristic), and a "deploy on Fly.io" template ship at launch.
- No inference cost on us in v1.

---

## Q6: World cadence

**Question:** Persistent 24/7, scheduled seasons, on-demand sessions, or hybrid?

**Answer:** **Persistent 24/7.** Agents act when owners offline; story feed catches them up on login.

**Implications:**
- Snapshot-to-disk every N minutes for crash recovery.
- "Cold" agents (in unobserved chunks) tick at low rate.

---

## Q7: World topology

**Question:** One global world, many shards, one-per-scenario, or auto-forking lobbies?

**Answer:** **Start with one world. Eventually scale to very large worlds (>200×200). One server hosts one world.** Multi-tenant per-process is out. the maintainer can run multiple servers for multiple worlds.

**Implications:**
- Engine binary takes a `--scenario` and `--map` flag at startup. Loads ONE world.
- Chunked map + AOI culling from day 1 (we'll scale to large worlds later).
- One Postgres database per world server.

---

## Q8: v1 scale

**Question:** 20 agents / 1k×1k, 50 agents / 2k×2k, 200 agents / 5k×5k, or variable?

**Answer:** **Eventually scale to thousands.**

**Implications:**
- Backend must be high-concurrency (Go was picked partly for this).
- AOI culling and chunked streaming from day 1.
- Inference batching, cold-state offloading designed in.
- v1 launches at smaller scale (200 active agents seems realistic for week-14 launch), but the architecture supports growth.

---

## Q9: v1 launch scenario

**Question:** Manhattan, fantasy town, office park, or custom?

**Answer:** **Fantasy town (Stardew/Rune-style).** Easiest AI gen quality, universal appeal.

**Implications:**
- v1 art generation focuses on fantasy: medieval buildings, forests, grass, dirt paths, swords, gold coins.
- Scenario verbs for v1: trade, pay, work, loot.

---

## Q10: Base verb set

**Question:** Minimal (move/speak/look/interact), +inventory, +combat, or just chat?

**Answer:** **All of them base.** move, speak, whisper, shout, look_at, interact, pickup, drop, equip, give, attack, defend, heal.

**Implications:**
- The base substrate supports all common game mechanics.
- Scenarios opt in to which they enable (a "salon" world disables combat verbs via config).
- See `docs/VERB_REFERENCE.md` §"Base verbs".

---

## Q11: Universal agent state

**Question:** Just pos + facing + inventory, or full vitality + hunger + fatigue, or pos+facing only?

**Answer:** **Vision radius around the agent (the maintainer asked me to pick X). Plus the agent's own HP, hunger. Plus a STATIC base map of the whole world so the agent can navigate. NEW BUILT structures should NOT be in the base map until the agent has personally seen them.**

**Implications:**
- Vision radius: 12 tiles default (with line-of-sight blocking), per scenario tunable.
- Static known map provided to every agent as part of observation.
- Per-agent "discovery set" tracks which dynamically-built structures the agent has seen.
- See `docs/OBSERVATION_MODEL.md` for the full schema.

---

## Q12: Agent decision cadence

**Question:** Uniform rate, tiered per agent class, event-driven, or hybrid?

**Answer:** **All agents pick their own rate up to 1Hz.** Slow agents update less often, fast agents more often. **The world never blocks on a slow agent.** This is part of the game's speed-vs-smarts tradeoff.

**Implications:**
- Engine pushes observations at each agent's configured interval, capped at 1Hz.
- Slow agents see less recent state; their actions execute as last-submitted.
- Documented as a player-facing design lever.

---

## Q13: Moderation

**Question:** Closed beta, classifier, reactive ban, or hybrid?

**Answer:** **No moderation. Launch first, deal with it later.**

**Implications:**
- v1 ships open. No automated filter on personas or speech.
- We accept that bad-actor content may appear.

---

## Q14: Inference cost

**Question:** Free tier with quota, pay-as-you-go, BYO only, or eat the cost?

**Answer:** **Everyone runs their own.** No hosted tier in v1. Make BYO easy.

**Implications:**
- No LLM inference bill for v1.
- SDKs + examples + Fly.io deploy template are first-class onboarding.

---

## Q15: Frontend engine

**Question:** PixiJS + custom, Phaser 3, Godot HTML5, or Defold?

**Answer:** **PixiJS** is OK, **but the maintainer is very worried** about writing too much from scratch and ending up with another mess. He demanded a clear plan to avoid endless UI iteration.

**Response — the anti-mess plan:**
- Inherit `@pixi/tilemap`, `pixi-viewport`, LDtk loader, Kobalte UI primitives.
- Lock palette + style guide BEFORE any art.
- Visual regression CI from day 1.
- Reference screenshot gate at every milestone.
- Buy/commission if 3 AI gen attempts don't pass.
- See `docs/ANTI_MESS_PLAN.md` for the full plan.

the maintainer approved this plan.

---

## Q16: UI overlay

**Question:** Sparse, dense (with persistent inventory), minimal, or Twitch-style sidebar?

**Answer:** **Sparse.** Top bar + minimap + drama feed always visible. Inspector on character click. Hotkey-toggled dev console.

**Implications:**
- Three persistent elements + click-to-open inspector + story feed in its own view.
- Reflected in the wireframe phase of Milestone 1/8.

---

## Q17: Combat lethality

**Question:** Lethal + spectator respawn, KO + respawn, restricted-zones, or none?

**Answer:** **Lethal combat.** Agents can be killed.

**Implications:**
- HP → 0 = death. Death animation. Body remains for loot window. Then despawns. Owner gets a story-feed entry, can respawn after cooldown.
- No engine-side PvP zones — combat works everywhere (per the "no moderation" call).

---

## Q18: Building interiors

**Question:** Sub-maps with portals, inline (Stardew model), hybrid, or seamless?

**Answer:** **Sub-maps; doors are portals.** HeartGold model.

**Implications:**
- LDtk's linked-levels feature is used.
- Walking onto a door tile triggers a sub-map load with fade transition.
- Interior maps can be larger than the building footprint.

---

## Q19: Backend language

**Question:** Go, Rust, Elixir, or Node?

**Answer:** **Go.**

---

## Q20: Wire protocol

**Question:** WS+FlatBuffers, WS+JSON, two protocols, or WebTransport?

**Answer:** **WebSocket + FlatBuffers binary.** Same protocol for browser and agents.

**Implications:**
- Schema-driven, language-neutral, hot-path messages.
- JSON used for cold-path (registration, auth).

---

## Q21: Persistence

**Question:** Postgres+snapshot, all-in-Postgres, Redis+Postgres, or in-memory only?

**Answer:** **Postgres for accounts/agents, periodic world snapshot to disk.**

**Implications:**
- Supabase or Neon hosted Postgres for v1.
- Snapshot files in a persistent volume on the engine machine.

---

## Q22: Monetary system (mid-session add)

**Question (from the maintainer):** We need a monetary system. Wealth tracked. Important for virality.

**Answer baked in:** Money is a **scenario-layer concept**, NOT engine. Engine has opaque `extras` blob per entity. Fantasy scenario adds `extras.gold int64`, verbs `pay/trade/work/loot`. Leaderboards read agent state via UI.

**Implications:**
- Engine stays universal.
- Other worlds without money just don't declare or use gold.

---

## Q23: Auth

**Question:** Clerk, Supabase Auth, Auth.js, or Discord only?

**Answer:** **Auth.js (self-hosted).**

**Implications:**
- No SaaS auth lock-in.
- Email + social providers (Google, GitHub, Discord).
- Session cookies; engine validates by calling Auth.js.

---

## Q24: v1 launch scope (the timeline call)

**Question:** All-in 10–14 weeks (1000×1000, all features), or trim (256×256, no combat, etc.)?

**Answer:** **All in. Everything. Big world. All features. Incrementally validated with screenshots along the way.** Cut line is launch with full feature set.

**Implications:**
- 10–14 week realistic timeline.
- Roadmap is structured with this scope (see `docs/ROADMAP.md`).
- Every milestone has a polish gate; we don't proceed if a milestone fails its visual bar.

---

## Q25: Default BYO bot template

**Question:** Local Qwen, Anthropic, OpenAI, or multi-template?

**Answer:** **Test with local llama.cpp (Qwen, what we had before) during development. Sample Anthropic bot too (the maintainer will provide a key eventually).**

**Implications:**
- Hello-world bot in the docs uses Qwen via llama.cpp.
- Anthropic example shipped for users with a Claude key.
- SDK is provider-agnostic; users plug in any backend.

---

## Q26: Art tooling

**Question:** ChatGPT/DALL-E, SDXL with LoRA, Midjourney, or commercial tile pack + AI fills?

**Answer:** **the maintainer generates spritesheets via ChatGPT. Claude does ALL post-processing (cutting, palette, alpha, sheet slicing). Prompts should be shorter than last time but still detailed enough to get full overworld richness — trees, food, apples, characters with full animation strips including moving, item interaction, fighting, taking damage.**

**Implications:**
- `art/prompts/` holds the frozen prompt templates per asset class.
- `art/intake.py` does ALL post-processing.
- Generation is prompt → eyeballed → intake → atlas. No manual Aseprite touch unless an asset is borderline.

---

## Q27: Repo location

**Question:** ~/projects/agent_sim, ~/Desktop/agent_sim, ~/agent_sim, or other?

**Answer:** **~/projects/agent_sim** (sibling of province_sim).

---

## Q28: Track everything in project dir

**Question (from the maintainer mid-session):** Keep all plan/state/memory in the new project dir so a fresh session has full context.

**Answer baked in:** Yes. CLAUDE.md is the entry point. Full docs/ folder. auto-memory dir gets thin pointers; the repo is the source of truth.

---

## Q29: Don't bias toward uniform stack

**Question (from the maintainer mid-session):** Pick the right tool per job. Don't lock everything into one ecosystem.

**Answer baked in:** Per-layer evaluation. Go for backend (concurrency). TS + PixiJS for frontend (browser perf). Python for art pipeline (Pillow + numpy). Auth.js (no SaaS). LDtk (visual editing).

---

## Q30: Polish guarantee

**Question (from the maintainer mid-session):** We need a CLEAR plan to reach HeartGold-tier quality. Round 1 had no path there.

**Answer:** Captured in full in `docs/ANTI_MESS_PLAN.md`. Key commitments:
1. Lock visual anchor with a real reference set BEFORE generating world art.
2. Style guide as code (palette + dims + animation specs) enforced by intake pipeline.
3. Generate ONE perfect asset, compare to reference, lock the bar, scale.
4. Mature autotile rules (LDtk's), not custom regex.
5. State-machine animations, not per-sprite logic.
6. Kobalte UI primitives, not hand-rolled widgets.
7. Visual regression CI from day 1.
8. Reference-comparison gate at every milestone.
9. Buy or commission if 3 generation attempts don't pass the bar.
10. The "does this look like HeartGold?" question is the gate.

---

---

## Q31: Multimodal / vision agents (mid-session add)

**Question (from the maintainer):** Some agents should operate on images instead
of structured observations. Add a path where an agent can ask for an
image of its surroundings (e.g. a 5×5 tile crop around its position) as
an alternative or supplement to the JSON observation. Crop size TBD.

**Answer baked in:**

- Image observations are a first-class observation mode, NOT a v2 add-on.
  See `docs/OBSERVATION_MODEL.md` §10b for full spec.
- Agents register with `vision: { mode: structured | image | both,
  image: { crop_tiles, render_scale, format, fog, ... } }`.
- The engine has an in-Go rasterizer that uses the SAME atlas the
  frontend uses — same pixels server-side and client-side, no
  divergence.
- The image rides in the same WS observation frame as structured data,
  not a separate fetch. Multimodal LLMs that want both spatial reasoning
  AND entity targeting use `mode: both`.
- Default crop size: open question, locked by experiment in Milestone 4
  (agent SDK + first multimodal example bot).

**Implications:**
- Wire schema reserves a `view_image` field in the observation message;
  absent for structured-only agents (zero overhead).
- Engine art atlas must be loadable in Go (`art/build_atlas.py` writes
  a format both Go and the frontend can consume).
- A multimodal example agent ships with the v1 SDK.

## What's NOT decided / open (after Session 1)

These came up but were deferred:

- **Specific palette pick** (Endesga 32 candidate, but final after Milestone 0 visual anchor).
- **Sound and music** — explicitly out of v1.
- **Mobile UI** — explicitly out of v1.
- **Multi-world federation** — out of v1.
- **Achievements / formal progression systems** — out of v1.
- **Hosted persona tier** — explicitly v2.

These are all in `docs/ROADMAP.md` "After-launch backlog."

---

# SESSION 2 — Feature-complete launch refactor

A second Q&A session after we'd built a working-but-half-wired prototype. the maintainer reframed the goal: "no v1/v2 staging — feature-complete launch." This session locked the architectural decisions that the rest of the build follows. Replaces / amends earlier docs where they conflict.

## Q32: Unit of composition for rulesets

**Question:** How granular is the unit of composition for higher-order rulesets (combat, money, voting, lineage, finance, kingdoms)?

**Answer:** **Composable systems.** Each ruleset is a standalone module. Worlds declare which modules to load via config. fantasy_town becomes a config file listing [Combat-v1, Money-v1, Inventory-v1, Construction-v1, ...]. Every world is a composition.

**Implications:**
- Hard separation between engine + base verbs + composable systems.
- Combat is NOT in the engine. Money is NOT in the engine. Engine knows only: entity / position / facing / movement / vision / hearing / base verbs (move, speak, whisper, shout, look_at, wait, interact).
- Each system is its own Go package under `engine/internal/systems/<name>/`.
- New systems plug in by registering verb handlers + event subscribers — never by touching engine core.

## Q33: Leaderboard purpose

**Question:** Is the leaderboard a public scorecard, an internal eval suite, or a competition platform?

**Answer:** **Internal-style, multi-dimensional, per-user.** Not a public scorecard ranking models. We track per-user metrics across many dimensions (wealth, kills, friends, governance, longevity, ...). Vision: researchers from labs drop bots into the world to see what their model is good at; the leaderboard surfaces capability profiles emergently.

**Implications:**
- No backend tagging. No "model A vs model B" infrastructure. Users describe their architecture in their bio if they want.
- Leaderboard aggregates per-user/per-agent (one user = one agent per world).
- Many parallel dimensions; each is a radar-chart axis.
- Aggregation per backend emerges naturally if (say) all top wealth agents have "Claude" in their bio.

## Q34: Quest abstraction

**Question:** Are quests engine primitives, a Tasks system, pure speech, or hybrid?

**Answer:** **Verbal contracts, no engine primitive.** Agent A speaks an offer. Agent B speaks acceptance. They go do the work. Either side can defect; emergent consequences. The ONLY engine support is a pair of LIGHTWEIGHT UI annotation verbs:
- `propose_task(recipient, title, description)` — creates a public marker
- `accept_task(task_id)` — links acceptance

The engine doesn't track completion, doesn't escrow rewards, doesn't enforce anything. The verbs exist solely so spectators can SEE "X is currently doing a task for Y" in the UI.

**Implications:**
- Agents that get exploited (do work, don't get paid) develop reputations. Reputation is the consequence layer.
- The UI renders the propose/accept pair as a connector between the two agents.
- No "quest log" engine state. Bot tracks its own commitments in its own memory.

## Q35: What "persona" means

**Question:** "Persona" was doing four jobs at once. Decompose: (1) display name + archetype, (2) public bio, (3) private system prompt, (4) self-declared relationships.

**Answer:** **Engine tracks only (1) name + archetype + (2) public bio.** Voice / system prompt + private opinions of others stay in the bot's own backend.

**Implications:**
- The docs' `apparent_label` derivation (from observer's relationship table) is **dead**. Engine just shows the entity's display_name to every observer.
- If an agent wants to call Bob "my wife" internally, that's the agent's brain's job, not engine state.
- Public bio is readable by other agents when within 5 tiles (see Q43).
- Persona registration: 3 fields, not 7.

## Q36: Persona authoring location

**Question:** Where is name + bio + archetype set — web form or bot code?

**Answer:** **Bot code is canonical.** Web is just login + API key issuance. Persona sits in the bot's source. SDK sends it on every register call. Researcher-friendly; Git-versioned; no "character creator" web flow.

**Implications:**
- Delete `frontend/src/auth/PersonaForm.tsx`. It's dead code.
- `/api/v1/agent/register` accepts persona inline.
- Editing your bio = edit source + reconnect; in-world bio updates.

## Q37: User → agent cardinality

**Question:** One user = many agents, or one user = one persistent agent?

**Answer:** **One user = one persistent agent per world.** Like a Pokemon save file. Reconnect picks up where you left off. Researchers wanting populations would need many user accounts (or future "researcher mode").

**Implications:**
- `/api/v1/agent/register` is idempotent: same user_token = same agent_id.
- Multi-world: same user can have separate characters in Fantasy Town vs Manhattan (each engine process = its own DB).
- Leaderboard slot per user.

## Q38: Execution model

**Question:** Real-time, turn-based, or hybrid?

**Answer:** **Pure real-time.** World ticks at 60Hz regardless. Agent observes at their own cadence. Slow agents miss things. "Snooze you lose" is realistic; speed-vs-quality is part of agent architecture.

**Implications:**
- Best agents will use hierarchical architectures: a fast low-level locomotion controller + a slow high-level strategist that share state. Ship a reference implementation: `examples/hello_hierarchical.py`.
- No benchmark "fair-comparison mode." Faster networks / smaller prompts are part of the bot's competitive surface.

## Q39: Affordance manifest

**Question:** How rich is the world's rules manifest, and how does a bot get it?

**Answer:** **Rich.** Per-system declarations of: verbs (with JSON Schema params + preconditions + worked examples), state fields owned (with shape + meaning), sounds emitted, archetypes added. Exposed at `/api/v1/world/affordances`. Bot fetches at register; UI renders the same data as the "World Rulebook" page. Single source of truth.

**Implications:**
- Each `engine/internal/systems/<name>/` package declares its manifest contribution.
- Engine aggregates all loaded systems' manifests at boot.
- UI consumes the same endpoint to render a beautiful World Rulebook page.
- Agents can validate actions client-side using the schemas.
- Codegen: SDK builds typed wrappers from the manifest at SDK release time.

## Q40: Action submission shape

**Question:** Typed dataclasses, generic act(), codegen, or method-style?

**Answer:** **Typed dataclasses per verb (current SDK style).** Core SDK ships Move/Speak/Wait/Interact/etc. Each composable system ships its own module: `agent_sim_sdk.combat`, `agent_sim_sdk.voting`, etc. Bot does `from agent_sim_sdk.combat import Attack`.

**Implications:**
- SDK versioning becomes important — adding a system = a release.
- Researchers get IDE auto-complete on every verb.
- Escape hatch: `agent.act_raw(verb_name, **params)` for advanced flexibility.

## Q41: Disconnect behavior

**Question:** Soft idle, grace + despawn, vulnerable body, or hybrid?

**Answer:** **Vulnerable body.** No invulnerability grace. Disconnected character is a stationary body that NPCs / other agents can attack, loot, or interact with. Reconnect any time; if you died, scenario respawn rules apply.

**Implications:**
- Disincentivizes parking bots.
- Reconnect within the body's lifetime = pick up where you left off.
- Need a clean visual indicator that an entity is currently disconnected (a small "Z" or grayed sprite).

## Q42: Concurrent connections for same user

**Question:** Last wins, first wins, explicit takeover, or shared body?

**Answer:** **Explicit takeover.** Second connection from the same user must send `takeover: true` in the auth frame. Without it, rejected with `already_connected`.

**Implications:**
- A buggy bot reconnecting in a loop doesn't thrash.
- Intentional restarts work: bot supplies `takeover: true` on startup.

## Q43: Vision shape + visible-entity fields

**Question:** Omnidirectional disk vs forward cone? What's in visible_entities?

**Answer:** **Omnidirectional 12-tile disk + bresenham LOS (walls + tall objects block).** Same in every direction (Pokemon-style). Each `visible_entities[i]` contains: entity_id + display_name + archetype + pos + facing + doing + HP / max_HP. **Bio is conditionally included** if observer is within 5 tiles. Gold + inventory stay private.

**Implications:**
- Observation builder checks distance per neighbor; includes bio if ≤5 tiles, omits otherwise.
- `apparent_label` field collapses to just `display_name`.
- HP visible to everyone in vision — Octopath-style health bars over heads in the UI.

## Q44: Day/night and vision

**Question:** Does day/night affect vision mechanically?

**Answer:** **Cosmetic only.** Day/night ColorMatrixFilter is the entire feature. Vision radius is a constant 12 tiles regardless of time-of-day. No light sources, no Lighting system, no night-vision reduction.

**Implications:**
- Lantern entities are decorative only.
- Simpler engine. Removes one composable system from the launch list.

## Q45: Sound blocking

**Question:** Do walls block sound?

**Answer:** **Walls fully block sound** via bresenham LOS (vision check is more permissive: it includes tall trees; hearing check only blocks for walls). Entering a building = real privacy. Whispered conversations actually matter.

**Implications:**
- The perception module gets two LOS variants: `seesEntity` (blocks on visionBlocks) and `hearsEntity` (blocks on walls only).
- Building interiors are an audio bubble — outside can't hear inside.

## Q46: Sound event emission

**Question:** Engine only emits speech, or any system can emit sounds?

**Answer:** **Any system can emit sounds.** Each composable system declares its sounds in the affordance manifest alongside its verbs + state. Combat declares `sword_clang` + `death_scream`. Door system declares `creak`. Wolf NPCs declare `howl`.

**Implications:**
- Engine API: `world.EmitSound(at, kind, options)` — propagates via the same audible event ring.
- Manifest schema now has four sections per system: verbs, state fields, sounds_emitted, archetypes.

## Q47: Buildings as entities

**Question:** Buildings as decorations, world objects, or first-class entities?

**Answer:** **First-class entities.** archetype="building". Full lifecycle: create (Construction system) / destroy (Combat / fire / siege) / inherit (Lineage system). Extras blob holds footprint, sprite, interior_map_id, owner, lock_state, etc. All systems touch buildings through normal entity channels.

**Implications:**
- Decorations stay for visual-only items (mushrooms, flowers, pebbles).
- Trees can stay decorations for now; promote to entities later if Forestry needs them.
- Building / Property system owns verbs: `enter`, `lock`, `unlock`, `place_sign`, `claim_ownership`, `transfer_ownership`.
- Building / Property system emits sounds: `door_open`, `door_close`, `lock_click`.

## Q48: Interior authoring + ticking

**Question:** Per-type templates, per-instance custom, or procedural? When are interiors loaded?

**Answer:** **Per-instance custom interiors. Eager-loaded at world boot. All interiors tick continuously.** Each building has its own hand-authored interior. World runs everywhere, always — NPCs inside the tavern keep gossiping whether anyone's watching or not.

**Implications:**
- Engine memory = sum of all loaded maps, all the time.
- Scales fine for a launch world (~10-30 named buildings).
- MultiMapHub.TickAll() handles per-tick across all maps.
- Hand-authored interiors are a content-cost line item.

## Q49: Base map content

**Question:** What does `GET /api/v1/world/map` return?

**Answer:** **Overworld only.** Terrain + named regions + building EXTERIORS (positions + display names like "tavern", "town hall"). NOT interior layouts. NOT current entity positions. NOT dynamic events. Agents must walk into interiors and remember what they saw.

**Implications:**
- Realism + benchmark fidelity.
- Static — same data for everyone.
- The docs' `known_map_summary.named_regions` populated from this; the interior portion in OBSERVATION_MODEL §5 is dead.

## Q50: Launch system scope

**Question:** Which systems are launch-required vs post-launch (but architecture-ready)?

**Answer:**
- **Launch-required**: Combat + Money + Inventory + Verbal-Quest UI + **Construction** (because building creation has engine implications that must be designed up front).
- **Post-launch, architecture-ready (no refactor needed)**: Relationships/Social, Voting/Governance, Lineage/Children, Financial markets, Kingdoms/regions.

**Implications:**
- Construction is a launch system, not post-launch.
- The composable-systems architecture must support all the post-launch systems plugging in without engine refactor — the central forcing function.

## Q51: Cross-system architecture

**Question:** Shared state, event bus, service interfaces, or hybrid?

**Answer:** **Hybrid: phased pipeline + typed event bus + service interfaces + grid spatial index.** Tick = 5 phases:

1. Drain action queue. Verb handlers run; may CALL other systems' services; may EMIT events.
2. System OnTick callbacks (deterministic order).
3. Drain event bus. Subscribers receive batched events from this tick.
4. Per-agent observation build (parallel goroutines).
5. Per-viewer broadcast (parallel, AOI-filtered).

Grid spatial index: O(1) "what's in radius R of (x,y)" via tile→entities map. All vision / hearing / AOI queries use it.

**Implications:**
- Significant refactor of `engine/internal/world/`: split into core (entities, tiles, movement, perception, spatial index) + base verbs + a System registry + an EventBus.
- Each system: registers verb handlers; declares emitted events + subscribed events; declares manifest contributions.
- Deterministic phase ordering enables replays + future benchmark replay.
- Naturally parallelizable per-entity within a phase.

## Q52: Visual feature tier

**Question:** Polished baseline, cinematic, Octopath HD-2D, or beyond?

**Answer:** **Octopath HD-2D tier.** Cinematic baseline + tilt-shift depth-of-field + dynamic point lights (lanterns cast soft circles at night) + water reflections of nearby sprites + wind-swayed grass/foliage shader + directional sun shadows + atmospheric perspective + bloom + vignette + LUT color grading + screen wipes + camera shake + weather particles.

**Implications:**
- PixiJS v8 with custom filter classes + shader passes. Possibly WebGPU compute.
- Multi-week shader/filter work. Real rendering pipeline upgrade, not just stacking filters.
- Per-feature flag — can ship base + bloom + vignette first, layer in shaders progressively.

## Q53: Delta encoding scope

**Question:** Where do we apply delta encoding?

**Answer:** **Delta for viewer broadcasts; full state for agent observations.** Bots get the complete snapshot every observation — no delta complexity in the SDK, no drift, easy benchmarking. Browser viewers use delta where bandwidth at 30Hz actually hurts.

**Implications:**
- Agent WS path: each obs is a full payload.
- Viewer WS path: first message full, then diffs against last sent.
- History persists indefinitely in an append-only event log; replay-independence frees us from delta-replay constraints.

## Q54: Historian

**Question:** How is the world's history exposed for summaries?

**Answer:** **Engine-side service. Free to anyone.** `/api/v1/history?since=T&until=T&about=entity_id&focus=...` returns an LLM-generated written narrative summary. Aggressive caching. Per-world LLM backend configurable. Powers story feed, spectator "what did I miss," world recap.

**Implications:**
- Append-only event log (Postgres) records every event the engine emits.
- Historian indexer caches summaries by query.
- Cost: we pay for LLM calls. Caching is essential for cost control.

## Q55: Agentic UI iteration loop

**Question:** How do I iterate on UI when the maintainer isn't watching?

**Answer:** **Wireframe-free, video-driven, design-critic loop.**

1. Implement the panel.
2. Playwright script records VIDEO of full interaction: clicks every button, hovers every element, triggers every state (loading / error / empty / full). Outputs video.mp4 + per-frame screenshots.
3. Spawn a "design critic" sub-agent with: the video, an Octopath/HG reference panel, the style guide. Critic returns gap analysis covering typography, color, alignment, density, polish, animation pacing, interaction feel.
4. Apply fixes; re-run.
5. Loop until critic clears.
6. Commit; send to the maintainer for human sign-off.

**Critical**: VIDEO not static frames. Static frames miss broken animations.

## Q56: Production quality scope

**Question:** What does "production quality" cover?

**Answer:** **All eight dimensions.** Reliability + Performance + Security + Observability + Code Quality + CI/CD + Data Integrity + Documentation. No compromises.

**Implications:**
- 99.9% uptime SLA target.
- Soak tests + crash recovery + snapshot validation.
- 60Hz at 1000 entities + 500 viewers benchmarked.
- Rate limits + auth + sanitization at every input.
- Prometheus + Grafana + alerting.
- 80% engine line coverage / 90% SDK.
- Zero-downtime deploys + rollback.
- Postgres backups + recovery drills.
- Public SDK docs site + runbooks.

## Q57: UI visual language

**Question:** Pokemon-style chunky bubbles or Octopath-style translucent dark + golden accents?

**Answer:** **Octopath / Square Enix HD-2D style.** Translucent dark panels, thin golden borders, serif accent font (EB Garamond) for headers, clean sans-serif (Inter) for body, subtle particle accents inside panels.

**Implications:**
- Lock the style guide. Color tokens, font tokens, panel-chrome tokens go into `art/ui_style.json`.
- Kobalte components themed with this palette + chrome.
- One style for everything. No splits between in-world and chrome.

## Q58: UI authoring approach

**Question:** Sketch / Figma / reference-screenshot / code-first?

**Answer:** **Code-first + design-critic loop.** No upfront wireframes. Iterate via Q55 loop until critic clears. Critic uses Octopath / Triangle Strategy reference panels as the comparison anchor.

**Implications:**
- We skip the wireframe step from ANTI_MESS_PLAN §4 for UI panels (it's substituted by the critic loop).
- The critic IS the discipline.

## Q59: Visible-entity field set

**Question:** What does an agent see about a visible neighbor at distance?

**Answer:** **entity_id + display_name + archetype + pos + facing + doing + HP/max_HP** always. **Bio included only when observer is within 5 tiles**. Gold + inventory always private.

**Implications:**
- Observation builder includes / omits bio per neighbor based on chebyshev distance ≤ 5.
- Default extras_summary visibility: scenario-defined. Combat exposes HP. Money does NOT expose gold publicly.

## Q60: Construction approach

**Question:** Blueprint / free-form / hybrid / collaborative?

**Answer:** **Townscaper / Manor Lords composable approach, time-boxed prototype.** 2-week sprint on one style (Cottage):

1. Hand-author the per-style component library: walls, windows, doors, roof tiles, chimney.
2. Procedural floor-plan generator: BSP partition of footprint into rooms; door placement; walls.
3. Auto-tile assembly: similar to terrain autotile, applied to building walls.
4. Build verb: `Build(style, footprint, room_count, target_pos)`. Validates materials + space + ownership. Spawns building entity + sub-map after N ticks.
5. Free-form furniture placement inside the empty interior: `Place_furniture(item, pos, rotation)`.

Design-critic loop. If critic clears: lock + add Manor + Watchtower + Tavern + Castle. If 2 weeks elapse without clearance: fall back to fixed blueprints + free-form interior.

**Implications:**
- Material sources: BOTH gather (Forestry chops trees → wood; Mining → stone) AND buy (markets). Two extra systems: Forestry + Mining (or one unified Resources system).
- Buildings spawn as entities with `extras.interior_map_id` pointing at a newly-generated sub-map.
- Multi-floor buildings = sub-map with a stair-tile portal to a floor-2 sub-map. Same Warp mechanism as exterior↔interior.

## Q61: NPC implementation

**Question:** Engine-internal Go, external SDK subprocesses, hybrid, or open-source reference?

**Answer:** **External Python subprocesses via the SDK.** Each NPC = a SDK process the engine spawns + supervises. Same code path as user bots. NPC code lives in `scenarios/fantasy_town/npcs/` — open + inspectable. Researchers can study NPC implementations as canonical examples.

**Implications:**
- Engine has an NPC supervisor: spawns N NPCs at boot, restarts on crash.
- We pay LLM cost for sophisticated NPCs (mayor, judge).
- The agent API is dogfooded by being our own consumer — its quality is a forcing function.
- Per-scenario NPC roster declared in scenario config.

---

# What's still open after Session 2

Detail-level questions deferred to implementation time:

- Race conditions for simultaneous tile claims
- Heartbeat / ping intervals (default: 30s ping, 60s timeout)
- SDK + manifest versioning policy
- Sound localization precision (precise from_pos vs direction-only)
- Multi-floor building portal mechanics
- Resource gathering specifics (per-tree wood yield, etc.)
- Furniture placement constraints
- Voting / Lineage / Finance / Kingdoms architecture stubs (don't paint into a corner)

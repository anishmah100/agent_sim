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

## What's NOT decided / open

These came up but were deferred:

- **Specific palette pick** (Endesga 32 candidate, but final after Milestone 0 visual anchor).
- **Sound and music** — explicitly out of v1.
- **Mobile UI** — explicitly out of v1.
- **Multi-world federation** — out of v1.
- **Achievements / formal progression systems** — out of v1.
- **Hosted persona tier** — explicitly v2.

These are all in `docs/ROADMAP.md` "After-launch backlog."

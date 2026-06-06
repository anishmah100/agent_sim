# STACK

Every pick justified, with license verified for commercial use.

## Backend (the world tick)

| Pick | License | Why |
|---|---|---|
| **Go 1.22+** | BSD-3 | Goroutines + channels give us a goroutine-per-connection model handling thousands of WS clients with zero callback hell. Single static binary deploys cleanly. Standard library covers TCP/HTTP/JSON. Faster to write than Rust, faster to run than Python/Node at this concurrency. |
| **`nbio`** (or `gorilla/websocket`) | MIT | High-performance WS library. `nbio` uses epoll/kqueue for true async I/O — needed at thousand-client scale. `gorilla/websocket` is older, simpler, sufficient up to ~1k clients. Start with `gorilla`, swap if profiling shows need. |
| **`flatbuffers`** for wire format | Apache-2 | Zero-copy parse. Schema-versioned. 5–10× smaller than JSON for typical state diffs. Critical when we broadcast at 30Hz to many clients. |
| **`pgx`** for Postgres | MIT | Most-performant Go Postgres driver. Async-friendly. Used in production by major Go shops. |
| **`pelletier/go-toml/v2`** | MIT | Scenario configs are TOML (more human-friendly than YAML for game data, simpler than JSON). |

Considered, rejected:
- **Rust** — best raw perf, longest iteration loop. Go is good enough at our scale.
- **Elixir** — strong concurrency primitives, niche tooling for binary game wire formats.
- **Node.js + uWebSockets.js** — single-threaded event loop caps us. Workable up to ~1k clients per process; we want to scale further.
- **Python** — what `province_sim` used. Can't do 60Hz tick at our entity counts reliably.

## Frontend (renderer + chrome)

| Pick | License | Why |
|---|---|---|
| **TypeScript 5+** | Apache-2 | Type-safe game state. The wire format types are auto-generated from FlatBuffers schemas, shared with the engine. |
| **PixiJS v8** | MIT | WebGL/WebGPU 2D renderer. Used by Discord (UI), Adventure Quest, etc. Lower-level than Phaser → cleaner architecture at scale. v8 added WebGPU support; we benefit from that on capable browsers. |
| **`@pixi/tilemap`** | MIT | Official PixiJS tile renderer extension. Production-grade. We do not write a tile renderer. |
| **`pixi-viewport`** | MIT | Production-grade pan/zoom/follow/inertia camera. ~3K stars, mature. We do not write a camera. |
| **`ldtk-ts`** (LDtk loader) | MIT | Reads `.ldtk` files into typed JS. Maintained by the LDtk maintainers. |
| **Solid.js** | MIT | Reactive UI for the DOM overlay. ~7KB. No virtual DOM. Fine-grained reactivity makes 30Hz state updates trivial without re-rendering panels. Faster than React for this use case. |
| **Kobalte** | MIT | Headless accessible UI components for Solid (buttons, dialogs, dropdowns, tooltips, etc.). We theme them once with a pixel-art look. We do not write widget primitives. |
| **Vite** | MIT | Build tool + dev server with HMR. The HMR mess we had with Phaser stays away because PixiJS scenes aren't HMR-survivable; we'll opt out of HMR for the canvas layer and accept full-reload, while keeping HMR for the DOM layer. |
| **Vitest** | MIT | Unit tests. Same engine as Vite. |
| **Playwright** | Apache-2 | E2E + visual regression. Pixel-diff CI gate. |

Considered, rejected:
- **Phaser 3** — we used it last time. Encourages God-object scene classes. Polish ceiling lower.
- **Godot HTML5** — full engine, but 30–50MB initial download, threading limited in web, unfamiliar deploy.
- **Defold** — great engine, Lua scripting adds a third language and a niche workflow.
- **PlayCanvas** — 3D-first.
- **React** for the DOM — VDOM overhead on 30Hz updates is wasted work. Solid eliminates it.

## Map editor

| Pick | License | Why |
|---|---|---|
| **LDtk** | MIT | Modern visual map editor with first-class **linked levels** (perfect for buildings-with-rooms). Visual autotile rules. Field editor for entities (NPC spawns, portal targets). Author by the maker of Dead Cells; designed for level designers not programmers. |

Considered, rejected:
- **Tiled (TMX/JSON)** — universal but older; nested levels are a bolt-on, not a primitive. Worse for our hierarchical-map use case.
- **Custom YAML** — what `province_sim` did. Hand-editing tile arrays is the wrong abstraction.

## Auth + identity

| Pick | License | Why |
|---|---|---|
| **Auth.js (NextAuth)** | ISC | Self-hosted, no vendor. Email + GitHub + Google + Discord providers ready. Session management built in. Free. Per the maintainer's call: control over user data, no SaaS lock-in. |
| **Resend** (transactional email) | Proprietary, free tier | For magic-link / verification emails. Generous free tier. Alternative: Postmark, AWS SES. |

Considered, rejected:
- **Clerk** — polished but vendor lock-in.
- **Supabase Auth** — bundled with Postgres but UI less polished.
- **Discord OAuth only** — too narrow.

## Database

| Pick | License | Why |
|---|---|---|
| **PostgreSQL 16** | PostgreSQL License (BSD-style) | Industry standard. Robust. We use it for accounts, agent metadata, story-feed event ledger. |
| **Supabase or Neon** (hosted) | Vendor | Hosted Postgres with backups. Free tier covers v1. Switchable to self-host any time. |

## Agent SDKs (we ship these)

| Language | Status | Notes |
|---|---|---|
| **Python** | v1 | `pip install agent-sim-sdk`. Typed observation/action models via pydantic. Async-first. |
| **TypeScript** | v1 | `npm install @agent-sim/sdk`. Auto-generated types from FlatBuffers schemas. |
| **Go** | post-v1 | For users who want raw performance. |

Example bots we ship:
- `examples/qwen_agent/` — local llama.cpp, no API key needed.
- `examples/hello_anthropic.py` — Anthropic Claude SDK, key from env.
- `examples/heuristic_bot.py` — rule-based, no LLM, useful for population fillers.

## Art pipeline

| Pick | License | Why |
|---|---|---|
| **ChatGPT / DALL-E 3 (image gen)** | Proprietary, OpenAI API | the maintainer's call. Generates spritesheets. We use detailed but concise prompts (a frozen template per asset class). |
| **Python + Pillow + numpy** | Various MIT/BSD | Local image-processing toolchain. Palette quantize, alpha cleanup, sheet slicing, atlas packing. We DO the post-processing — ChatGPT only generates raw sheets. |
| **Lospec Palette List** | CC0 | Library of pixel-art palettes. We pick one (e.g. "Endesga 32", "AAP-64") and lock. |
| **Aseprite** (optional, manual touch-up) | Source-available | If a sprite needs hand-tweaking, this is the tool. We try not to rely on it; if we do, we own the touched file. |

Asset fallback plan: if AI gen can't reach the quality bar after 3 attempts for a given asset, we **buy** a commercial pixel pack (Cup Nooble's "Sprout Lands" at $20, or Pixel Frog's "Tiny" packs at free CC0) for the base tileset, AI-gen the long tail (food, custom buildings, NPC variations).

## Hosting / deploy

| Layer | Pick | Why |
|---|---|---|
| Engine | **Fly.io** machine (1 GB+ RAM, persistent volume for snapshots) | Single binary deploy. Persistent volumes for snapshot files. Public WS endpoint. Good Go support. |
| Frontend | **Vercel** or **Cloudflare Pages** | Static site (Vite build output). Fast CDN. Free for our v1 traffic. |
| DB | **Supabase** or **Neon** | Hosted Postgres free tier. |
| Auth emails | **Resend** | Free tier. |

Per `docs/ARCHITECTURE.md` §5: **one server per world**. To host multiple worlds (e.g. Manhattan + Fantasy), spawn multiple Fly machines, each with its own scenario + map + database namespace.

## Per-layer language matrix (confirming "right tool per layer")

| Concern | Language | Why |
|---|---|---|
| World tick | Go | concurrency + perf |
| Frontend renderer | TS + PixiJS | browser-native, type-safe |
| Frontend chrome | TS + Solid | reactive, lightweight |
| Wire format | FlatBuffers schemas | language-neutral, auto-generated |
| Map editor | LDtk (a GUI) | designer-friendly |
| Art post-process | Python + Pillow | image-library ecosystem |
| Agent SDK | Python + TS (Go later) | meet users where they are |
| Scripts / dev tools | Python + bash | quick iteration |

No bias toward stack uniformity. Each pick stands on its own.

## License roll-up (every dep cleared for commercial use)

All MIT / BSD / Apache-2 / CC0. No GPL / AGPL / "non-commercial" / vendor lock-in surfaces in our runtime dependency graph. Auth.js (ISC) and PostgreSQL license are BSD-style. Vendor services (Fly, Vercel, Supabase, Resend, OpenAI) have commercial-permissive ToS for our use case. **Confirmed before commit.**

# Session Handoff — 2026-06-08 (resume point)

Snapshot of where we stopped so the next session picks up cleanly.
Companion to the auto-memory `project_agent_sim_v2_push` pointer.

## What this session shipped (all committed, author anishmah100)

Recent commits (newest first):

- `18e9445` Drop dead-agent husks from `/api/v1/agents` picker
- `1a03be4` Avenger archetype + deranged-killer set-piece (slice 7)
- `3557105` Fix `kill_witnessed` audible never reaching non-adjacent witnesses
- `1e63903` Don't emit `hunger_pang` FX for agents hidden inside buildings
- `dd777ab` Witnesses tab: surface kills an agent saw + screams it heard
- `0b49b84` Sync `DefaultAttackDamage` to `unarmedDmg=7`
- `021710a` Inspector/hover badge distinguishes Qwen vs Claude vs rule vs NPC

These finished batch items **5, 6, 7** and the **hunger-FX** half of item 8 from the
"keep going with 5-8" list:

1. **Agent-type badge (#5)** — inspector header + hover card now show
   `Qwen` (cyan) / `Claude` (orange) / `LLM` / `rule` / `NPC`. Brain string
   plumbed runner → engine (`agents_list.go` Brain field, from
   `persona["brain"]`) → UI (`AgentHoverCard.Badge`, `BadgeKind`).
2. **Witnesses tab (#6)** — replaced the Trace tab. Engine keeps a per-entity
   witness ring (`world.WitnessRecord`, `World.WitnessedBy`) populated in
   `EmitDeathScream` (`kill_witnessed` with killer/victim for LOS witnesses,
   `scream_heard` for those merely in earshot). Surfaced on the
   `mental_state` response. UI: `WitnessesTab` in `Inspector.tsx`.
   - Found+fixed a real bug: `kill_witnessed` audible had `radius:1` so LOS
     witnesses beyond one tile never received it → widened to `witnessRadius`.
3. **Deranged-killer set-piece (#7)** — new rule-based `Avenger`
   (`agents/baselines/avenger.py`): forages until it witnesses a kill, then
   arms up and mobs the named killer; grudge decays (`GRUDGE_TTL=80`) if the
   killer escapes sight. `--setpiece deranged_killer` in `run_p7_real.py`
   (1 hunter + 5 avengers + 3 survivors + 3 Qwen prey). Unit tests added.
   - **Validated live**: hunter `spawn_8` killed two prey, then avenger
     `spawn_13` killed the hunter back — the chase→notice→flee→gang-up arc
     fired. 25 kills / 381 hits in the watch window.
4. **Hunger-FX fix (part of #8)** — `hunger_pang` no longer emitted for agents
   inside buildings (was floating "hungry" over empty footprint tiles).

## Tests / build state

- Engine: `go build ./...` clean; `go test ./...` green (fixed two stale
  combat tests via the `DefaultAttackDamage` sync commit).
- Frontend: `npx tsc --noEmit` clean. No vitest files.
- Agents: `pytest agents/baselines/tests/` → 21 passed (4 new Avenger tests).

## OPEN / NEXT (tracked as tasks #258–#260, plus #259 = item-8 remainder)

- **#260 — restart engine + verify picker fix; chase remaining teleport.**
  The dead-agent-husk picker fix (`18e9445`) is committed but the RUNNING
  engine is the pre-fix binary. **Rebuild + restart the engine** to apply it,
  then confirm clicking any persona centers on a LIVE hub agent, never (0,0).
  The "occasional teleport across the screen while watching" is still open —
  likely viewer WS frame-drops letting an agent jump >`SNAP_PX` (4 tiles) so
  the renderer snaps; consider hide+fade on big jumps instead of snapping.
- **#258 — rendering robustness audit (DO THIS; user asked explicitly).**
  Buildings vanished from a long-open browser tab, but the fresh page-load
  Playwright capture (`/tmp/big_2.png`) rendered them fine. Root cause: stale
  Vite **HMR** state in the Pixi decoration layer — NOT data/textures (map
  serves 1479 `bld:` decorations near hub at 764,864; textures return 200).
  Make the Pixi render modules (`Decoration.ts`, `Tilemap.ts`, `Entity.ts`,
  `PixiApp.tsx`) HMR-safe (dispose/rebuild on hot reload, or force full reload
  for those modules); re-check the viewport cull rect for large footprints;
  log when the deco layer is empty while decorations are in view.
  **Immediate user workaround: hard-refresh (Ctrl+Shift+R) restores buildings.**
- **#259 — economy depth / gold sinks (item 8 remainder).** Add a gold sink
  (buy food from stalls → offset hunger) so the survivor's gold matters.
- **#232 — budget tracker** (older pending), **#253 — sustain loop** (older).

## How to resume the live world

```bash
cd ~/projects/agent_sim
# rebuild engine (picks up 18e9445 picker fix)
(cd engine && go build -o ../.runlog/engine ./cmd/engine)
# start engine (same flags as start.sh / last run)
./.runlog/engine -addr 127.0.0.1:8080 -bundle worlds/eldoria \
  -event-log .runlog/events.jsonl -snapshot-dir .runlog/snapshots \
  -snapshot-every 600s -cors-allow http://127.0.0.1:5173,http://localhost:5173 \
  -register-rate 200 -register-burst 200 -time-mult 1.0 \
  -npc-config worlds/eldoria/npcs.json &
# frontend (vite) — if not already running
(cd frontend && npm run dev &)   # http://127.0.0.1:5173
# the deranged-killer set-piece (Qwen local on :8782)
python -m tools.experiments.run_p7_real --setpiece deranged_killer \
  --wall-seconds 1800 --out .runlog/p7_real/setpiece &
```
Note: the engine restores entities from `.runlog/snapshots/latest.json` on
boot. Snapshots store entities only (decorations/tiles come from the bundle),
so buildings are unaffected by restore.

## Standing constraints (do not drift)

- Commit author `anishmah100 <anishmah100@users.noreply.github.com>`; NEVER add a
  Co-Authored-By Claude trailer. Every change = a new commit. No force-push.
- Keep `.env.local` gitignored. Stay under the $25 Anthropic spend cap
  (prefer Qwen local on :8782 for runs; reserve Claude brain for showcases).
- Agent art: one sprite at a time, manual inspect, no batch glob scripts.
- Send a render image each visual iteration; the human makes the taste call.

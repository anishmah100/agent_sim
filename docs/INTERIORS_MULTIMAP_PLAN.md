# Building Interiors — HeartGold multi-map model (implementation plan)

**Status:** ACTIVE (started 2026-06-08)
**Decision:** Pokémon HeartGold model — doors are portals to *separate* interior
sub-maps. Enter warps the agent into the interior map; the camera shows only
the current map; the agent walks around inside; an exit tile warps it back to
the overworld door. (Ref: `docs/DECISIONS.md` Q "Sub-maps; doors are portals.")

**Why this plan exists:** the audit (2026-06-08) found `enter`/`exit` work but
there is no interior to walk around in, AND that the `MultiMapHub` is *dormant*
— never instantiated. The engine is strictly single-world. So this is a
from-scratch multi-map integration, not a wire-up. It must be built in small,
individually-tested, non-breaking steps because the live single-world demo
depends on the very code paths being changed.

---

## Current reality (verified in code, 2026-06-08)

- `engine/cmd/engine/main.go`: loads ONE `world.World w`; the loop calls
  `w.Tick()` (one map). `wire.NewViewerHub(ctx, w)` + `wire.NewAgentHub(ctx, w)`
  each hold that single `w`.
- `engine/internal/wire/agent.go`: `AgentHub.w *world.World` — every
  `BuildObservationFor` / `SubmitAction` / `SpawnAgentEntity` /
  `SetPlayerControlled` / `EntityIDs` call targets that single world.
- `engine/internal/world/multimap.go`: `MultiMapHub` + `LoadInterior` + `Warp`
  + `TickAll` exist but are referenced by NO production code (only a test).
- `Entity` has NO `CurrentMap` field (the multimap comment lies).
- Decoration `enter` (`world/action.go: tryEnterDecorationBuilding`) sets
  `e.InsideBuilding = sprite` + `e.insideTicks` (the phase-out flag) — no warp.
- Frontend renders one map: it consumes `/api/v1/world/snapshot` + the viewer
  WS, which serialize a single world. No notion of "which map".

## Target behavior (HeartGold)

1. Agent stands on/adjacent to a `door:bld:NNN:x,y` and submits `enter`.
2. Engine lazily creates (or reuses) the **interior map instance for that
   specific building**, warps the agent to the interior entrance, records the
   overworld return tile.
3. The agent's observations now come from the interior map (its own terrain,
   walls, items, and any other agents who entered the same building). It walks
   around with normal `step`.
4. Agent submits `exit` (or steps on the interior exit mat) → warp back to the
   overworld door tile; interior is GC'd when empty.
5. The frontend, when following an agent, renders whichever map that agent is
   on; entering/exiting switches the rendered tilemap + camera.

## Design decisions

- **Per-building-instance interiors, lazily created, GC'd when empty.** Keyed
  by the door/building id, not the sprite type — so two agents in two different
  houses are NOT in the same room (privacy + correct emergence). Created on
  first enter; removed from the hub when the last occupant exits, to bound
  memory across 1200+ buildings.
- **Procedural interiors** sized from the building footprint (a walled room with
  a floor, an exit mat tile at the south entrance, optional furniture/items by
  building type). Authoring 1200+ JSONs is untenable; a generator keyed on
  sprite type gives variety (blacksmith vs granary vs house) without files.
- **`map_id` scheme:** `interior:<building_id>` (e.g. `interior:bld:000@767,867`).
  The overworld keeps its bundle map id (`eldoria`).
- **Snapshot/observation already work per-World** — routing is the only change;
  `BuildObservationFor` runs on whatever World the entity lives on.

---

## Phased implementation (each phase: atomic commit, build+test green, demo unbroken)

### Phase 1 — Entity map-tracking foundation (NON-breaking, no behavior change)
- Add `CurrentMap string` to `Entity` (default = the world's `MapID` on spawn).
- Populate it in `SpawnEntity`/load so every entity has its overworld map id.
- No routing change yet; single world still authoritative. Verify obs frame
  unchanged + all tests green. (Pure additive field.)

### Phase 2 — Hub instantiation, single-map no-op
- `main.go`: `hub := world.NewMultiMapHub(w)`; replace `w.Tick()` with
  `hub.TickAll()`. With one map this is identical behavior.
- Give `AgentHub` + `ViewerHub` the hub; add `AgentHub.worldFor(entityID)` that
  returns the World currently holding the entity (overworld for now).
- Route the agent obs/action calls through `worldFor(...)` instead of `h.w`.
  Still single map → identical behavior. Verify live demo unchanged.

### Phase 3 — Interior generation + warp on enter/exit
- `interiors.go`: `GenerateInterior(buildingID, sprite, footprint) *World` — a
  walled room, floor tiles, an exit-mat tile, type-flavored furniture/items.
- Rewrite the decoration `enter` path: instead of the InsideBuilding flag,
  `hub.LoadGenerated(interior)` + `hub.Warp(entity, overworld, interior, entrance)`;
  store the overworld return tile on the entity.
- `exit` (and stepping on the exit mat): `hub.Warp(...)` back to the return tile;
  if the interior is now empty, `hub.Unload(interiorID)`.
- Keep the auto-exit timeout as a safety net (warp out if stuck).
- Verify live with `tools/audit/building_e2e.py`: enter → pos is interior →
  `step` MOVES inside → see interior items → exit → back at overworld door.

### Phase 4 — Frontend map-switching
- Viewer/snapshot expose the set of maps + each entity's `current_map`.
- When the followed agent is on an interior map, render that map's tiles +
  entities and switch the camera; on exit, switch back to the overworld.
- Verify visually: follow an agent through a door, watch it walk around inside,
  come back out. Screenshot evidence.

### Phase 5 — Polish + audit
- Lock/unlock gates interior entry; multiple agents meeting inside; NPCs inside;
  interior in the runlog/events; soak with agents entering/leaving; doc update
  (AGENT_API/OBSERVATION_MODEL/MOVEMENT_AND_COLLISION reflect real interiors).

## Risks & guards
- **Breaking the single-world demo:** Phases 1–2 are behavior-preserving;
  prove each with the audit harness + a live demo check before Phase 3.
- **Concurrency:** each World has its own lock; `Warp` takes both. Tick order
  across maps is sequential (deterministic) per `TickAll`.
- **Memory:** GC interiors when empty (per-instance creation is bounded by
  concurrent occupied buildings, not total buildings).
- **Observation routing regressions:** `obs_integrity.py` + `verb_matrix.py`
  must stay green after every phase.

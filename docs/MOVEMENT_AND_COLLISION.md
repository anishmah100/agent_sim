# Movement, collision, and the move API

> Synthesized from field-tested patterns: Pokémon HeartGold (DS), Link to the Past (SNES), Stardew Valley, Tibia, and the cooperative-pathfinding literature (Silver's WHCA\*, conflict-based search). Sources at the bottom.

## Why a discrete model beats float positions

Three patterns from the literature converge on the same conclusion:

1. **Pokémon-class collision**: when each character occupies one cell, checking adjacent cell indices is the simplest correct answer. No bounding-box math, no fractional state to reason about.
2. **Half-tile subgrid (NES Zelda)**: pixel-by-pixel walking with snap-to-grid on turn. The grid is the SOURCE OF TRUTH; smoothness is a render concern.
3. **Link to the Past corner-feeler**: two "feelers" off the player's facing edge; if one hits, nudge sideways onto the grid line. Makes the player feel weightless without giving up tile-based collision.

All three keep the LOGICAL position discrete and the RENDER position continuous. **This is what we do.** Float-only positions (what we had before this rewrite) make collision fragile and pathfinding nonsensical.

The model: **logical position is discrete (integer tile), rendered position is continuous.** This gives us simple collision + AI + pathfinding (everything on the integer grid) while keeping the smooth Pokémon-style camera and animation that we'd lose with strict tile-snap movement.

## Entity position fields

```go
type Entity struct {
    LogicalTile   [2]int     // canonical "where I am right now" — single source of truth for collision
    WalkFromTile  [2]int     // tile we started this walk-step from (= LogicalTile when not walking)
    WalkProgress  float64    // 0..1 along the current walk step
    WalkPath      [][2]int   // remaining tiles in the queued move target; empty when idle
    WalkETATick   uint64     // tick at which we reach LogicalTile from WalkFromTile
    Facing        Facing
    CurrentAction string     // "" | "move" | "attack" | "interact" | "hit"
    ...
}
```

Render position the frontend uses:
```
pos = lerp(WalkFromTile, LogicalTile, WalkProgress)
```

The engine NEVER carries a continuous position. `LogicalTile` is what every collision query reads.

## Tile occupancy

Per chunk (or per world for small maps), maintain:

```go
type TileOccupancy struct {
    static  map[Tile]ObjectID    // trees, walls, building footprints
    agents  map[Tile]EntityID    // characters' LogicalTile AND WalkFromTile during transit
}
```

A character mid-walk occupies BOTH WalkFromTile and LogicalTile — this prevents a second character from stepping into the tile a walker just vacated until the walker has fully arrived. (Otherwise you'd get "teleport through" glitches.)

A tile is **walkable** for entity E iff:
- `world.tile_kind(t)` is in `WALKABLE_KINDS` (grass, dirt, path, floor_wood, stone, sand)
- AND `occupancy.static[t]` is unset
- AND `occupancy.agents[t]` is unset OR equals E

`WALKABLE_KINDS` is a scenario-tunable constant (some scenarios let agents walk into water — boat scenario).

## The `move` verb

```
verb:   "move"
params: { target_tile: [x, y] }
```

### Engine handler — on submission

1. **Validate target**: is `[x, y]` on the map, and is `world.tile_kind([x, y])` in `WALKABLE_KINDS`?
   - No → `action_rejected: { reason: "cannot_path", detail: "target not on walkable terrain" }`
2. **A\* search** from `self.LogicalTile` to `target_tile`. Cost = 1 per step, all-Manhattan. Heuristic = Manhattan distance. Treat any tile with static occupancy as blocked. Dynamic occupancy (other agents) is treated AS-OF-NOW for the heuristic — we don't reserve their tiles for future ticks since they're moving too.
   - No path exists → `action_rejected: { reason: "cannot_path", detail: "no route to target" }`
3. **Set the entity's walk state**: `WalkPath = path[1:]` (drop current tile), `CurrentAction = "move"`, `WalkProgress = 0`, `WalkFromTile = LogicalTile`, `LogicalTile = path[1]`, occupancy map updated to mark both tiles.
   - Wait — committing `LogicalTile = path[1]` immediately means we "claim" the next tile at submission time. If another agent already has it, we should reject FIRST.
4. **Re-check `LogicalTile = path[1]` is unoccupied**. If a different agent owns it, reject `target_occupied`.
5. **Emit `move_started`** event.

### Engine handler — each tick

```
on tick():
  for entity with CurrentAction == "move":
    e.WalkProgress += STEP_PER_TICK   # ~0.04 = ~25 ticks per step ≈ ~0.4 s
    if e.WalkProgress < 1.0:
      continue                         # still mid-step, render-only
    # We've completed a step. Snap.
    e.WalkProgress = 0
    e.WalkFromTile = e.LogicalTile
    occupancy.agents.remove(e.WalkFromTile, e)
    if len(e.WalkPath) == 0:
      # We've arrived at the original target.
      e.CurrentAction = ""
      emit move_completed { entity_id, at_tile }
      continue
    # Look at next tile in the path.
    next := e.WalkPath[0]
    if !walkable(next, for_entity=e):
      e.CurrentAction = ""
      e.WalkPath = nil
      emit path_obstructed { entity_id, at_tile: e.LogicalTile, blocker_tile: next }
      continue
    # Commit to the next step.
    e.WalkPath = e.WalkPath[1:]
    e.LogicalTile = next
    occupancy.agents.add(next, e)
```

### What the agent sees

In every observation while moving:
- `self.current_action`: `{ verb: "move", target_tile, eta_tick }`
- `self.pos`: their logical tile (continuous render is the frontend's job)

When something happens to the move:
- Successful arrival → next observation has `current_action: null`, and `audible.recent_self_events` includes `move_completed`
- Blocked mid-path → next observation has `current_action: null` AND a `path_obstructed` event with the obstructing tile
- Rejection at submission → `last_action_result: { accepted: false, reason: "cannot_path" | "target_occupied" | "out_of_map" }`

Agent decides what to do — re-route, wait, attack the blocker, etc. **Engine never auto-replans.**

## Why this model

| Question | Answer |
|---|---|
| Can two agents walk through each other? | **No** — both their logical tile AND walk-from-tile are reserved during transit. |
| Can an agent walk into a tree? | **No** — tree's footprint marks every covered tile as statically occupied; A* won't route through it. |
| What if I path to X but agent Y steps into my path mid-walk? | I stop at my current logical tile; I get a `path_obstructed` event. My LLM decides whether to re-route, wait, etc. |
| What about LLM agents that are slow to respond? | Their walk continues server-side at engine rate. The LLM just issues high-level intents — the per-tick locomotion is the engine's job. |
| Smooth render? | Frontend interpolates between `WalkFromTile` and `LogicalTile` using `WalkProgress`. Same as Pokémon — feels analog, computes digital. |
| Can the agent pick any visible tile as target? | Yes. The observation includes `known_map_summary` (tile kinds + walkability) so the agent knows what's reachable. |
| What about diagonal moves? | v1: orthogonal only (4-direction). Diagonal is a future option — adds the corner-cutting question. |

## Static walkability table (v1)

| Tile kind | Walkable |
|---|---|
| `grass` | yes |
| `dirt` | yes |
| `path` | yes |
| `floor_wood` | yes |
| `stone` | yes |
| `sand` | yes |
| `water` | NO |
| `wall` | NO |
| `void` | NO |

Per-scenario override available (a "sailor" scenario could allow water).

## Multi-tile footprints

A tree with footprint `[2, 2]` at LogicalTile `[5, 5]` claims tiles `[5,5]`, `[5,6]`, `[6,5]`, `[6,6]` in `occupancy.static`. A* sees those tiles as blocked and routes around.

A building with footprint `[3, 3]` at `[10, 10]` claims all 9 tiles. The door is a single tile inside the footprint marked as a **portal**:
- Walkability: the door tile IS walkable
- Action: stepping onto a portal tile auto-triggers the `interact` action with `affordance="enter"`, which transports the agent to the linked interior sub-map.

(Buildings in the manifest declare `interactable_tile: [dx, dy]` relative to footprint origin.)

## Coordinate space

- Logical tiles are integers, addressable up to map size (1000×1000 in v1).
- A* on a 1000×1000 map is fine — we never path the full map (cap path length at e.g. 100 tiles; longer requests get rejected with `path_too_long`).
- The agent's known_map_summary is delivered ONCE at register-time and on scenario change. Subsequent observation pushes only carry deltas.

## What the frontend renders

For every visible entity:
- `pos = lerp(WalkFromTile, LogicalTile, WalkProgress)` — supplied by the engine in each observation push
- Facing comes from the engine
- Walk animation plays while `CurrentAction == "move"`
- Idle animation when `CurrentAction == ""` and not moving
- Action animation (`attack`, `interact`, `hit`) plays its frames once when `CurrentAction` is set to that value

## Y-sort (depth ordering for the 3/4 view)

Top-down RPGs use **Y-sort**: the higher an object's foot pixel on screen, the earlier it draws. The canonical implementation:

1. Each rendered object's `position.y` is its **foot pixel** (where it touches the ground), not its center.
2. The scene container has `sortableChildren = true` (PixiJS) with `zIndex = foot_y`.

Effect: a character standing south of a tree (foot pixel further down the screen) draws on top of the tree's sprite. Walking north of the tree, the character draws behind. This is the entire "character disappears behind the building" illusion — no extra layer logic needed.

Multi-tile objects (trees, buildings) follow the same rule: their `zIndex` is the foot Y of their footprint's bottom row. The sprite extends UP into pixel space above the footprint; that's where the canopy / roof shows. Characters north of the footprint draw below the sprite; characters south draw above.

## Pathfinding strategy

**v1 (now): plain BFS per-request.** No future-tile reservation, no cooperative planning. Why:
- BFS on a 32×20 dev_test world is sub-millisecond.
- Even on a 1000×1000 map, a single A\* under a 100-step path budget is well under one frame.
- Multi-agent conflict resolution: we use the **"replan-on-block"** strategy from the literature — at submission time we plan a path; at execution time, if the next step is blocked we stop and emit `path_obstructed`. The agent decides what to do next.

**v2 (when crowds get dense): cooperative pathfinding.**
- Switch to Silver's **WHCA\*** (Windowed Hierarchical Cooperative A*) — a single shared **space-time reservation table** prevents head-on collisions and corridor deadlocks.
- This requires planning paths over **(tile, future_tick)** pairs instead of just tiles.
- Worth it ONLY if v1 produces visible "agents stuck against each other" issues. Until then, simpler is better.

**Conflict resolution between two agents wanting the same tile in the same tick** (the only really tricky case in v1):
- Use deterministic ordering by `entity_id` lexicographic. Loser gets `target_occupied`.

## Interior transitions

From the literature: "Players prefer exploring maps without excessive transition effects, with transitions typically reserved for major transitions between maps or levels." Small huts can be open-roof inline; big interiors are separate sub-maps with a fade.

**Our rule:**
- A building's manifest declares `interactable_tile: [dx, dy]` relative to its footprint origin — that's the door tile.
- The door tile is marked as a `portal` in the engine's tile data. `portal_target = "interior_map_id"`.
- When an agent's logical tile becomes the portal tile, the engine auto-triggers `interact { affordance: "enter" }`:
  - Save the agent's outdoor position into `last_overworld_pos` extras.
  - Move the agent to the interior map at the linked spawn point.
  - Frontend receives a `map_changed` event and fades the canvas.
- Walking onto the interior's "exit" tile (also a portal, with `portal_target = "fantasy_town"` and a destination spawn) reverses the process.

Interiors are full maps in the same engine. The `WalkProgress` model applies inside buildings exactly as outside.

## Pre-cache hint for v2

When the agent's logical tile enters the chunk **containing** a portal, the engine pre-fetches the interior's tile data to its viewer subscribers in the background. By the time the agent walks ONTO the portal, the interior map is already in the client's cache and the fade-out can be instant.

## Static + dynamic occupancy summary

The walkability oracle answers: "Can entity E enter tile T right now?"

```
walkable_for(E, T) =
  IsOnMap(T)
  AND TerrainKindAllowsWalking(world.tile_kind(T))    // grass, dirt, path, etc.
  AND ObjectFootprint(T) is empty                      // no tree / wall
  AND OccupantAgent(T) is None OR equals E             // no character collision
```

Each layer is keyed by tile, O(1) lookup. The engine's `IsWalkable(T)` checks the static layers; `CanEnter(E, T)` adds the dynamic agent check.

## Sources

- Jonathan Whiting — [2D Tilemap Collision](https://jonathanwhiting.com/tutorial/collision/) — rectangle vs. tile, axis-separated movement, sub-step at high speed
- Oraqia — [Tricks for 2D grid-based character collision](https://oraqia.wordpress.com/2014/07/05/tricks-for-2d-grid-based-character-collision-that-can-work-in-3d-too/) — Pokémon vs. Zelda vs. Link to the Past corner feelers
- Godot 4 Recipes — [Using Y-Sort](https://kidscancode.org/godot_recipes/4.x/2d/using_ysort/index.html) — pivot at bottom of sprite for depth ordering
- David Silver — [Cooperative Pathfinding](https://cdn.aaai.org/ojs/18726/18726-52-22369-1-10-20210928.pdf) — space-time reservation table, WHCA*
- [Pathfinding in Video Games (UDIT)](https://www.udit.es/en/pathfinding-en-videojuegos-a-a-estrella-dijkstra-y-navmesh-con-ejemplos-paso-a-paso/) — A* as the standard for 2D grid games
- GameDev.net — [How To Do RPG Interiors](https://www.gamedev.net/forums/topic/668452-how-to-do-rpg-interiors/) — separate sub-maps vs. transparent roof
- [Multi-agent Pathfinding Collision Detection (arXiv 1908.09707)](https://arxiv.org/pdf/1908.09707) — efficient conflict detection methods
- [ChunkMap (Phaser)](https://phaser.io/news/2016/01/chunkmap) and [GameDev.net 2D Tilemaps "Chunk Theory"](https://gamedev.net/forums/topic/653120-2d-tilemaps-chunk-theory/) — chunked streaming pattern for >100×100 maps

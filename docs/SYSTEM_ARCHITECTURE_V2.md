# SYSTEM_ARCHITECTURE — Session 2

The engine architecture committed in Session 2 (Q32, Q51). Supersedes `ARCHITECTURE.md` §1–§5 where they conflict.

## TL;DR

The engine is a **substrate**. Real gameplay lives in **composable systems** registered at boot. Each tick runs a **5-phase pipeline**. Systems talk to each other via a **typed event bus** + optional **service interfaces**. All spatial queries go through a **grid spatial index**.

## Layer diagram (Session 2)

```
┌────────────────────────────────────────────────────────────┐
│  LAYER 5 — UI (TypeScript + PixiJS + Solid)                │
│  Spectator + my-agent + story-feed modes; World Rulebook;  │
│  HUD; Dialogue; Leaderboards; Historian queries.           │
└──────────────────▲─────────────────▼───────────────────────┘
              WS + JSON delta (Q53)
┌──────────────────▼─────────────────▲───────────────────────┐
│  LAYER 4 — AGENTS (external WS bots, including NPCs)       │
│  Python / TS SDK over WebSocket. Full-state observations,  │
│  typed actions. NPCs run as subprocesses on engine host    │
│  (Q61). Same protocol for everyone.                        │
└──────────────────▲─────────────────▼───────────────────────┘
        WS + JSON full obs (Q53) / typed action
┌──────────────────▼─────────────────▲───────────────────────┐
│  LAYER 3 — COMPOSABLE SYSTEMS                              │
│                                                            │
│  Each system is a Go package: registers verbs + state +    │
│  sounds in the affordance manifest; subscribes/emits       │
│  events on the bus; optionally exposes service interface.  │
│                                                            │
│  Launch-required (Q50):                                    │
│    - Combat                                                │
│    - Money                                                 │
│    - Inventory                                             │
│    - VerbalQuests (propose_task / accept_task UI markers)  │
│    - Construction                                          │
│    - Building / Property (door / lock / ownership)         │
│    - Forestry + Mining (material gathering, Q60)           │
│                                                            │
│  Post-launch, architecture-ready (Q50):                    │
│    - Relationships / Social                                │
│    - Voting / Governance                                   │
│    - Lineage / Children                                    │
│    - Financial markets                                     │
│    - Kingdoms / regions                                    │
└──────────────────▲─────────────────▼───────────────────────┘
        Verb register / event pub-sub / service call
┌──────────────────▼─────────────────▲───────────────────────┐
│  LAYER 2 — ENGINE CORE (Go)                                │
│                                                            │
│  Owns:                                                     │
│    - Entity registry                                       │
│    - Tile maps (multi-map; sub-maps per interior)          │
│    - Movement + collision (BFS pathfinding, occupant grid) │
│    - Perception (LOS, hearing, audible ring buffer)        │
│    - Spatial index (grid: tile → entities)                 │
│    - Event bus (typed publish + batched subscribe)         │
│    - System registry                                       │
│    - 5-phase tick pipeline                                 │
│    - Manifest aggregator                                   │
│    - Snapshot writer (Postgres + disk)                     │
│    - Event-log writer (history)                            │
│    - WS hubs (agent + viewer)                              │
│    - Rasterizer (per-agent N×N PNG, multimodal)            │
│    - NPC subprocess supervisor                             │
│                                                            │
│  Base verbs (the universal vocabulary):                    │
│    move, speak, whisper, shout, look_at, wait, interact    │
│                                                            │
│  Knows NOTHING about gold, HP, voting, marriage, kingdoms. │
└──────────────────▲─────────────────▼───────────────────────┘
                LDtk map files + scenario config
┌──────────────────▼─────────────────▲───────────────────────┐
│  LAYER 1 — STATIC MAP + SCENARIO CONFIG                    │
│  LDtk .ldtk files + scenarios/<name>/config.toml that      │
│  declares which systems to load.                           │
└────────────────────────────────────────────────────────────┘
```

## The 5-phase tick (Q51)

```
TICK START
─────────────────────────────────────────────────────────
Phase 1: ACTION DISPATCH
  Drain the per-agent action queue.
  For each action:
    handler = world.VerbRegistry[action.verb]
    result := handler(world, entity, action)
    if result.emits_events: world.EventBus.queue(events...)
    ack := build_ack(result)
    queue_for_send(agent, ack)

Phase 2: SYSTEM TICK
  For each system in deterministic registration order:
    system.OnTick(world, current_tick)
    (May emit events too.)

Phase 3: EVENT DRAIN
  For each event queued during Phase 1 + 2:
    for each subscriber to this event_kind:
      subscriber.OnEvent(world, event)

Phase 4: OBSERVATION BUILD (parallel goroutines)
  For each agent due for an observation this tick:
    obs := world.BuildObservation(agent, opts)
    queue_for_send(agent, obs)

Phase 5: VIEWER BROADCAST (parallel)
  For each viewer:
    diff := world.BuildDeltaForChunks(viewer.subscribed_chunks)
    queue_for_send(viewer, diff)
─────────────────────────────────────────────────────────
TICK END (flush all queued sends)
```

Determinism = phases are ordered. Within a phase, work is parallelizable per-entity.

## Event bus (Q51)

```go
package eventbus

type Event interface { Kind() string }

type EntityDied struct { EntityID, Killer, Cause string }
func (EntityDied) Kind() string { return "EntityDied" }

type StructureBuilt struct { BuilderID, BuildingID, Sprite string }
func (StructureBuilt) Kind() string { return "StructureBuilt" }

type RuleChanged struct { Rule string; Enabled bool; PassedBy string }
func (RuleChanged) Kind() string { return "RuleChanged" }

// Bus interface:
type Bus interface {
    Subscribe(kind string, h func(world *World, ev Event))
    Queue(ev Event)
    Drain(world *World) // called in Phase 3
}
```

Typed events. Subscribers registered at system init. Batched drain at end of tick.

## Service interfaces (Q51)

For when one system needs to call into another synchronously:

```go
type CombatService interface {
    DealDamage(target string, amount int, cause string) (newHP int, died bool)
}

// Combat system registers:
world.RegisterService("combat", CombatService(&combatImpl{...}))

// Construction system calls:
hp, dead := world.GetService("combat").(CombatService).DealDamage("wolf_3", 5, "trap")
```

Synchronous, in-process. Used when an action in one system needs to mutate state owned by another (Construction's "destroy this building" → Combat's "deal damage to the structure"). Discouraged for cross-cutting facts; events are preferred for those.

## Spatial index (Q51)

```go
type SpatialIndex struct {
    grid map[Tile][]string  // tile → entity IDs
}

func (s *SpatialIndex) EntitiesInRadius(center Tile, r int) []string
func (s *SpatialIndex) EntitiesInRect(x0, y0, x1, y1 int) []string
func (s *SpatialIndex) Add(entityID string, t Tile)
func (s *SpatialIndex) Remove(entityID string, t Tile)
func (s *SpatialIndex) Move(entityID string, from, to Tile)
```

All perception (vision, hearing), AOI culling, observation building, leaderboards-by-region — use this. O(1) "what's near here" is the load-bearing primitive.

## System registration

```go
// engine/internal/systems/combat/combat.go
package combat

func New() *System { ... }

func (s *System) Name() string { return "combat" }

func (s *System) Manifest() SystemDeclaration {
    return SystemDeclaration{
        Name: "combat",
        Verbs: []VerbDeclaration{
            {Verb: "attack", ParamsSchema: ..., Examples: ...},
            {Verb: "defend", ...},
            {Verb: "heal",   ...},
        },
        StateFields: []StateFieldDeclaration{
            {Key: "hp",     Type: "int", PublicAtAnyDistance: true, ...},
            {Key: "max_hp", Type: "int", PublicAtAnyDistance: true, ...},
        },
        SoundsEmitted: []SoundDeclaration{
            {Kind: "sword_clang", EmittedBy: "attack verb"},
            {Kind: "death_scream", EmittedBy: "death event"},
        },
    }
}

func (s *System) RegisterWith(reg Registry) {
    reg.Verb("attack",  s.handleAttack)
    reg.Verb("defend",  s.handleDefend)
    reg.Verb("heal",    s.handleHeal)
    reg.OnEntitySpawn(s.spawnInit)  // give every spawn hp+max_hp
    reg.OnTick(s.tickRegen)         // slow HP regen
    reg.Service(CombatService(s))   // expose service interface
}
```

This is the contract every system implements. The engine just calls these.

## Goroutine concurrency model

- **Main tick goroutine**: drives the 5-phase pipeline. Holds the world write-lock during Phases 1-3.
- **Per-agent worker pool**: Phase 4 (observation build) runs in parallel. Each worker holds the world read-lock; observation building doesn't mutate world state.
- **Per-viewer worker pool**: Phase 5 same shape.
- **WS read goroutines**: one per connection, push to action queue (mutex-protected).
- **WS write goroutines**: one per connection, drain send queue.
- **Snapshot goroutine**: runs every N minutes; takes read-lock on world; serializes to Postgres + disk.
- **Event-log goroutine**: drains a small event-log buffer to Postgres asynchronously.
- **NPC supervisor goroutine**: spawns + restarts NPC subprocesses; doesn't touch world directly.

## Per-world process model (recall Q97 / earlier session)

One engine binary = one world. `fantasy_town` is its own process. `manhattan_modern` another. Cross-world interaction is out of scope. Each gets its own Postgres database namespace.

## What changes from Session 1's architecture

- `engine/internal/world/world.go` splits into core + perception + spatial index + event bus + system registry + pipeline.
- The `scenario` interface (Session 1) is replaced with the System interface above.
- Combat / Money / Inventory move from `engine/internal/scenario/fantasy_town/` to `engine/internal/systems/{combat,money,inventory}/`.
- `fantasy_town` becomes a **config file**, not a Go package — `scenarios/fantasy_town/scenario.toml` lists the systems to load.
- Observation builder uses the spatial index instead of iterating all entities.
- The viewer broadcast becomes AOI-filtered.
- The agent WS hub uses event-log push for `world_event_notify`.

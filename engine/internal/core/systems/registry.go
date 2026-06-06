// Package systems is the registry contract — the interface every
// composable system in engine/internal/systems/<name>/ implements.
//
// Locked by docs/DECISIONS.md Q32 + Q51 + docs/SYSTEM_ARCHITECTURE_V2.md.
//
// A System contributes verb handlers, event subscribers, optional
// service interfaces, per-tick logic, per-entity-spawn logic, and the
// affordance manifest entries. The engine's Registry wires all of it.
package systems

import (
	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
)

// World is what systems see. Concrete implementation lives in the
// engine core; this interface keeps systems decoupled from world
// internals. We'll grow this surface as systems need more capability.
type World interface {
	eventbus.WorldCtx

	EntityByID(string) Entity
	EntityIDs() []string
	MutateEntity(id string, f func(Entity))
	SpawnEntity(Entity) error
	RemoveEntity(id string) error

	// SpawnEntityFromSpec creates a fresh world entity from the given
	// spec and adds it at the spec's tile. Used by systems that need to
	// produce new entities (construction → blueprint/building, loot →
	// dropped item) without knowing the concrete *world.Entity type.
	SpawnEntityFromSpec(spec EntitySpec) (Entity, error)

	EmitSound(at [2]int, kind string)
	// EmitDeathScream — D10. Emits an anonymous wide-radius scream +
	// targeted kill_witnessed audibles to LOS witnesses. Killer ""
	// means non-combat death (no witness event). Muffled=true for
	// kills inside buildings (narrows scream radius).
	EmitDeathScream(at [2]int, victimID, killerID string, muffled bool)
	QueueEvent(eventbus.Event)
	GetService(name string) any
	RegisterService(name string, svc any)

	// Spatial index access.
	EntitiesInRadius(center [2]int, r int) []string

	// Tile + walkability for verb validation.
	IsWalkable(t [2]int) bool
	Chebyshev(a, b [2]int) int

	// Building interior membership. EnterBuilding marks the entity as
	// being inside the named building for up to `maxTicks` (the engine
	// auto-exits after the countdown so an offline agent doesn't get
	// stuck inside forever). ExitBuilding clears membership early.
	// Building systems orchestrate the verb-level rules (ownership,
	// locks, observers); the world owns the field.
	EnterBuilding(entityID, buildingID string, maxTicks int) bool
	ExitBuilding(entityID string) bool
	InsideBuilding(entityID string) string

	// Declarative-ruleset access. Tunings live in worlds/<name>/rules.star
	// and are loaded into World.Rules at bundle load. The methods are
	// nil-safe — a world without a ruleset returns the supplied default,
	// which keeps engine code working when run via the legacy --world flag.
	//
	// Systems should ALWAYS prefer these over package-level constants so
	// that per-world tunings (eldoria's hunger_per_tick = 0.0008 vs another
	// world's higher value) take effect without code changes.
	Tuning(name string, defaultValue float64) float64
	TuningInt(name string, defaultValue int) int
	TuningBool(name string, defaultValue bool) bool
}

// EntitySpec describes a not-yet-spawned entity. Pass to
// World.SpawnEntityFromSpec; the world allocates a real entity at the
// given tile with these fields.
type EntitySpec struct {
	ID          string
	Archetype   string
	Pos         [2]int
	DisplayName string
	Extras      map[string]any
}

// Entity is the opaque handle systems mutate. The concrete world
// type is engine/internal/world/Entity; this interface forces systems
// to go through the documented mutation API instead of poking fields.
type Entity interface {
	ID() string
	Archetype() string
	Pos() [2]int
	SetExtra(key string, value any)
	GetExtra(key string) (any, bool)
}

// ActionEnvelope is the wire shape of an action. Verb handlers
// receive the JSON-raw and decide how to parse + validate.
type ActionEnvelope struct {
	ActionID string
	Verb     string
	Priority int
	Raw      []byte
}

// ActionResult is the engine-facing response.
type ActionResult struct {
	ActionID string
	Verb     string
	Accepted bool
	Reason   string
}

// VerbHandler is the function signature a system registers per verb.
type VerbHandler func(w World, e Entity, env *ActionEnvelope) ActionResult

// Registry is what systems hand themselves to at boot. The engine
// owns it; systems just call its methods.
type Registry interface {
	// Verb registers a handler for `verb`. Panics on collision (two
	// systems can't own the same verb name).
	Verb(verb string, h VerbHandler)

	// OnEvent subscribes a system to a typed event kind.
	OnEvent(kind string, h eventbus.Handler)

	// OnTick registers a per-tick callback. Multiple systems can
	// register; called in registration order during Phase 2.
	OnTick(h func(w World, tick uint64))

	// OnEntitySpawn registers a callback fired for every entity at
	// world boot AND on every SpawnEntity call. Used to seed extras.
	OnEntitySpawn(h func(w World, e Entity))

	// Service exposes a system's service interface under a name.
	// Other systems retrieve it via World.GetService(name).
	Service(name string, svc any)

	// Manifest contributes the system's manifest section.
	Manifest(decl manifest.SystemDeclaration)
}

// System is the contract every composable system implements.
type System interface {
	Name() string
	RegisterWith(r Registry)
}

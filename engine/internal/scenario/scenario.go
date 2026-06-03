// Package scenario defines the interface a fantasy/contemporary/etc
// world plugs into. The engine is dumb — it knows ticks, positions,
// vision, hearing, base verbs. The scenario adds rules: HP, gold,
// monsters, quests, weather. One scenario per running engine process.
package scenario

import (
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// VerbHandler is invoked by the engine when an action whose verb is
// scenario-bound arrives. The handler decides if it's accepted and may
// mutate world state. The world write-lock is HELD when this is
// called.
type VerbHandler func(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult

// Scenario is the contract.
type Scenario interface {
	// Name returns a stable identifier (e.g. "fantasy_town"). Reported
	// in /api/v1/world/info.
	Name() string

	// Verbs returns the additional verbs this scenario adds beyond the
	// base set in docs/VERB_REFERENCE.md. Used to advertise the
	// vocabulary in world info.
	Verbs() []string

	// Handler returns a VerbHandler for the given verb. Returns nil if
	// the verb isn't handled here (engine falls back to defaults or
	// rejects). Both engine-base verbs (e.g. attack, pickup) AND
	// scenario-custom verbs (trade, pay) can be routed here.
	Handler(verb string) VerbHandler

	// OnEntitySpawn is called once per entity at world load. Lets the
	// scenario seed HP, gold, inventory etc. into entity.Extras.
	OnEntitySpawn(e *world.Entity)

	// OnTick is called every engine tick AFTER the base movement
	// update, so the scenario can resolve damage timers, regenerate
	// HP, advance quest state, etc.
	OnTick(w *world.World, tick uint64)
}

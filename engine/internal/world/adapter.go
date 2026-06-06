package world

// Adapter — bridges the existing *World to the new core/systems
// interfaces (engine/internal/core/systems/registry.go).
//
// Existing world code keeps working as-is. New composable systems
// receive WorldAdapter / EntityAdapter wrappers and use the documented
// minimal surface.

import (
	"sync/atomic"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/spatial"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

// WorldAdapter wraps *World and satisfies systems.World.
type WorldAdapter struct {
	W       *World
	Bus     *eventbus.Bus
	Spatial *spatial.Index
	// Service registry keyed by name. Systems register here via
	// Registry.Service; consumers look up via World.GetService.
	services map[string]any
}

func NewWorldAdapter(w *World, bus *eventbus.Bus, sp *spatial.Index) *WorldAdapter {
	return &WorldAdapter{
		W: w, Bus: bus, Spatial: sp,
		services: make(map[string]any),
	}
}

// Tick — eventbus.WorldCtx requirement.
func (a *WorldAdapter) Tick() uint64 { return a.W.CurrentTick() }

func (a *WorldAdapter) EntityByID(id string) syscore.Entity {
	e := a.W.EntityByIDUnlocked(id)
	if e == nil {
		return nil
	}
	return &EntityAdapter{E: e, W: a.W}
}

func (a *WorldAdapter) EntityIDs() []string { return a.W.EntityIDsUnlocked() }

func (a *WorldAdapter) MutateEntity(id string, f func(syscore.Entity)) {
	a.W.MutateEntity(id, func(real *Entity) {
		f(&EntityAdapter{E: real, W: a.W})
	})
}

func (a *WorldAdapter) SpawnEntity(e syscore.Entity) error {
	ent, ok := e.(*EntityAdapter)
	if !ok {
		return ErrBadEntityType
	}
	a.W.SpawnEntity(ent.E)
	a.Spatial.Add(ent.E.EntityID, ent.E.LogicalTile)
	return nil
}

// SpawnEntityFromSpec builds a real *Entity from the spec, adds it to
// the world + spatial index, and returns an Entity handle bound to the
// fresh underlying.
func (a *WorldAdapter) SpawnEntityFromSpec(spec syscore.EntitySpec) (syscore.Entity, error) {
	e := &Entity{
		EntityID:    spec.ID,
		Archetype:   spec.Archetype,
		LogicalTile: spec.Pos,
		DisplayName: spec.DisplayName,
		Extras:      spec.Extras,
	}
	if e.Extras == nil {
		e.Extras = map[string]any{}
	}
	a.W.SpawnEntity(e)
	a.Spatial.Add(spec.ID, spec.Pos)
	return &EntityAdapter{E: e, W: a.W}, nil
}

func (a *WorldAdapter) RemoveEntity(id string) error {
	a.W.RemoveEntity(id)
	a.Spatial.Remove(id)
	return nil
}

func (a *WorldAdapter) EmitSound(at [2]int, kind string) {
	a.W.audibleAppend(AudibleEvent{
		EventID:    nextEventID(&a.W.eventSeq),
		Kind:       "sound",
		SoundKind:  kind,
		FromPos:    at,
		Tick:       a.W.tick,
		radius:     8,
	})
}

// EmitDeathScream — D10. When an agent dies, emit a wide-radius
// anonymous death scream + a targeted kill_witnessed audible to each
// agent within line-of-sight of the killing tile. Non-witnesses hear
// "a scream from somewhere" but don't learn killer/victim identity;
// witnesses get the truth and can choose to gossip about it.
//
// at        — killing tile (rounded to 5-tile cell for the
//              anonymous scream to obfuscate exact position).
// victimID  — the dead entity's id; carried in the witness event only.
// killerID  — the attacker's id; carried in the witness event only.
//              Empty for non-combat deaths (starvation, etc).
// muffled   — true when the kill occurred inside a building. Reduces
//              scream radius to ~10 tiles, simulating muffled sound.
func (a *WorldAdapter) EmitDeathScream(at [2]int, victimID, killerID string, muffled bool) {
	// Round position to 5-tile cell for anonymity.
	approxX := (at[0] / 5) * 5
	approxY := (at[1] / 5) * 5
	radius := 35
	if muffled {
		radius = 10
	}
	a.W.audibleAppend(AudibleEvent{
		EventID:   nextEventID(&a.W.eventSeq),
		Kind:      "sound",
		SoundKind: "death_scream",
		FromPos:   [2]int{approxX, approxY},
		Tick:      a.W.tick,
		radius:    radius,
		// FromEntity intentionally empty — anonymous.
	})
	if killerID == "" {
		return // starvation etc: no witness event
	}
	// Witness events: deliver a targeted "kill_witnessed" audible to
	// every entity with line-of-sight to the killing tile.
	// LOS check uses the existing lineOfSight helper. Witnesses must
	// also be within their own vision radius of the kill (12 tiles
	// day, 6 night — for simplicity here use 12; observation builder
	// filters by per-agent day_phase if needed).
	const witnessRadius = 12
	// Build the witness event payload once.
	witnessText := `{"killer":"` + killerID + `","victim":"` + victimID + `"}`
	for id, ent := range a.W.entities {
		if ent.InsideBuilding != "" && muffled {
			// Inside-building observers can't witness an outdoor kill.
			continue
		}
		if id == victimID || id == killerID {
			continue
		}
		// Chebyshev + LOS.
		dx := at[0] - ent.LogicalTile[0]
		if dx < 0 {
			dx = -dx
		}
		dy := at[1] - ent.LogicalTile[1]
		if dy < 0 {
			dy = -dy
		}
		if dx > witnessRadius || dy > witnessRadius {
			continue
		}
		if !a.W.lineOfSight(ent.LogicalTile, Tile{at[0], at[1]}) {
			continue
		}
		a.W.audibleAppend(AudibleEvent{
			EventID:   nextEventID(&a.W.eventSeq),
			Kind:      "sound",
			SoundKind: "kill_witnessed",
			FromPos:   at, // witnesses see the true position
			Text:      witnessText,
			Tick:      a.W.tick,
			radius:    1,
			whisperTo: id, // only this witness receives it
		})
	}
}

func (a *WorldAdapter) QueueEvent(ev eventbus.Event) { a.Bus.Queue(ev) }

func (a *WorldAdapter) GetService(name string) any   { return a.services[name] }
func (a *WorldAdapter) RegisterService(name string, svc any) {
	a.services[name] = svc
}

func (a *WorldAdapter) EntitiesInRadius(center [2]int, r int) []string {
	return a.Spatial.EntitiesInRadius(center, r)
}

func (a *WorldAdapter) IsWalkable(t [2]int) bool { return a.W.IsWalkable(Tile(t)) }

func (a *WorldAdapter) Chebyshev(a1, b [2]int) int {
	return chebyshev(Tile(a1), Tile(b))
}

// Building-interior membership. These manipulate the engine-private
// InsideBuilding + insideTicks fields directly so composable systems
// don't have to know about the existing tick-decay semantics.
func (a *WorldAdapter) EnterBuilding(entityID, buildingID string, maxTicks int) bool {
	e := a.W.EntityByIDUnlocked(entityID)
	if e == nil {
		return false
	}
	a.W.MutateEntity(entityID, func(real *Entity) {
		real.InsideBuilding = buildingID
		real.insideTicks = maxTicks
	})
	return true
}

func (a *WorldAdapter) ExitBuilding(entityID string) bool {
	e := a.W.EntityByIDUnlocked(entityID)
	if e == nil || e.InsideBuilding == "" {
		return false
	}
	a.W.MutateEntity(entityID, func(real *Entity) {
		real.InsideBuilding = ""
		real.insideTicks = 0
	})
	return true
}

func (a *WorldAdapter) InsideBuilding(entityID string) string {
	e := a.W.EntityByIDUnlocked(entityID)
	if e == nil {
		return ""
	}
	return e.InsideBuilding
}

// Tuning / TuningInt / TuningBool — declarative-ruleset access. Reads
// from World.Rules with nil-safe defaults so legacy bundles that don't
// declare [rules.file] keep working with the supplied defaults.
func (a *WorldAdapter) Tuning(name string, defaultValue float64) float64 {
	return a.W.Rules.GetFloat(name, defaultValue)
}
func (a *WorldAdapter) TuningInt(name string, defaultValue int) int {
	return a.W.Rules.GetInt(name, defaultValue)
}
func (a *WorldAdapter) TuningBool(name string, defaultValue bool) bool {
	return a.W.Rules.GetBool(name, defaultValue)
}

// LockWrite / UnlockWrite / LockRead / UnlockRead expose locking for
// the pipeline. World uses sync.RWMutex internally.
func (a *WorldAdapter) LockWrite()   { a.W.mu.Lock() }
func (a *WorldAdapter) UnlockWrite() { a.W.mu.Unlock() }
func (a *WorldAdapter) LockRead()    { a.W.mu.RLock() }
func (a *WorldAdapter) UnlockRead()  { a.W.mu.RUnlock() }
func (a *WorldAdapter) IncrementTick() uint64 {
	return atomic.AddUint64(&a.W.tick, 1)
}

// === EntityAdapter ===

type EntityAdapter struct {
	E *Entity
	W *World
}

func (a *EntityAdapter) ID() string         { return a.E.EntityID }
func (a *EntityAdapter) Archetype() string  { return a.E.Archetype }
func (a *EntityAdapter) Pos() [2]int        { return a.E.LogicalTile }
func (a *EntityAdapter) SetExtra(k string, v any) {
	if a.E.Extras == nil {
		a.E.Extras = map[string]any{}
	}
	a.E.Extras[k] = v
}
func (a *EntityAdapter) GetExtra(k string) (any, bool) {
	if a.E.Extras == nil {
		return nil, false
	}
	v, ok := a.E.Extras[k]
	return v, ok
}

// Underlying — escape hatch for systems that need to touch full Entity.
func (a *EntityAdapter) Underlying() *Entity { return a.E }

var ErrBadEntityType = adapterErr("entity must be *EntityAdapter")

type adapterErr string

func (e adapterErr) Error() string { return string(e) }

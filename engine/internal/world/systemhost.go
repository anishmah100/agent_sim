package world

// SystemHost — installs a composable-systems ConcreteRegistry into a
// live *World via the existing InstallScenario API. This is the single
// bridge between the new core/systems architecture (typed event bus,
// service interfaces, manifest aggregator) and the legacy world.go
// dispatch path. The legacy path stays; what it dispatches changes.
//
// Why InstallScenario instead of replacing world.go dispatch outright:
// the current SubmitAction / Dispatch flow already handles locking,
// observation invalidation, and built-in verbs (move, speak, look).
// Slotting registry-driven verbs into that pipe is one line of code
// per verb. A full ripout of world.go's dispatcher would be a much
// larger change for no architectural gain — once verbs all route
// through the registry, the legacy switch in action.go becomes empty
// and can be deleted in a later sweep.

import (
	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/core/spatial"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

// ActionAccepted is queued onto the bus once for every action whose
// dispatcher returned Accepted=true. Lets the historian + smoke scorer
// see native engine verbs (move, speak, shout, …) that don't otherwise
// generate per-system events.
type ActionAccepted struct {
	EntityID string
	Verb     string
	Tick     uint64
}

func (ActionAccepted) Kind() string { return "ActionAccepted" }

var _ eventbus.Event = ActionAccepted{}

// SystemHost owns the bus + spatial index + adapter + registry for a
// world. Construct one per world; call Install with each composable
// system before InstallInto.
type SystemHost struct {
	World      *World
	Bus        *eventbus.Bus
	Spatial    *spatial.Index
	Adapter    *WorldAdapter
	Registry   *syscore.ConcreteRegistry
	Aggregator *manifest.Aggregator
}

// NewSystemHost builds the supporting infrastructure for a *World.
// Spatial is seeded from the world's current entities.
func NewSystemHost(w *World, agg *manifest.Aggregator) *SystemHost {
	bus := eventbus.New()
	spat := spatial.New()
	for _, id := range w.EntityIDsUnlocked() {
		e := w.EntityByIDUnlocked(id)
		if e != nil {
			spat.Add(e.EntityID, e.LogicalTile)
		}
	}
	adapter := NewWorldAdapter(w, bus, spat)
	reg := syscore.NewConcreteRegistry(bus, agg)
	return &SystemHost{
		World:      w,
		Bus:        bus,
		Spatial:    spat,
		Adapter:    adapter,
		Registry:   reg,
		Aggregator: agg,
	}
}

// Install registers a system. Wraps BeginSystem/EndSystem so manifest
// attribution is automatic.
func (h *SystemHost) Install(s syscore.System) {
	h.Registry.BeginSystem(s.Name())
	s.RegisterWith(h.Registry)
	h.Registry.EndSystem()
}

// InstallInto wires the accumulated verb / tick / spawn hooks into the
// underlying *World via its legacy InstallScenario API. Idempotent
// against the World — call once after all systems are Install()ed.
func (h *SystemHost) InstallInto() {
	h.Registry.InstallServicesInto(h.Adapter)

	// Bridge each verb. We capture the adapter once; the closure wraps
	// the concrete *Entity into an EntityAdapter on every call.
	verbs := make(map[string]func(*World, *Entity, *ActionEnvelope) ActionResult)
	for _, verb := range h.Registry.Verbs() {
		v := verb
		verbs[v] = func(w *World, e *Entity, env *ActionEnvelope) ActionResult {
			r := h.Registry.Handle(h.Adapter, &EntityAdapter{E: e, W: w}, &syscore.ActionEnvelope{
				ActionID: env.ActionID,
				Verb:     env.Verb,
				Priority: env.Priority,
				Raw:      env.Raw,
			})
			return ActionResult{
				ActionID: r.ActionID,
				Verb:     r.Verb,
				Accepted: r.Accepted,
				Reason:   r.Reason,
			}
		}
	}

	onTick := func(w *World, tick uint64) {
		h.Registry.RunOnTickAll(h.Adapter, tick)
		// Drain any events systems queued this tick so subscribers run
		// inside the same write-locked window. Events queued during a
		// drain land in the next tick (Bus contract).
		h.Bus.Drain(h.Adapter)
	}

	onSpawn := func(e *Entity) {
		h.Registry.RunOnEntitySpawn(h.Adapter, &EntityAdapter{E: e, W: h.World})
	}

	h.World.InstallScenario(verbs, onTick, onSpawn)

	// Wire the action-accepted hook so native engine verbs (move, speak,
	// shout, whisper, …) land in the historian. Without this, the
	// only events on the bus come from systems that explicitly Queue();
	// movement + speech were silent — the smoke scorer couldn't see them.
	h.World.SetOnActionAccepted(func(entityID, verb string, raw []byte) {
		h.Bus.Queue(ActionAccepted{
			EntityID: entityID,
			Verb:     verb,
			Tick:     h.World.CurrentTick(),
		})
	})
}


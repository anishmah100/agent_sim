// Package pipeline runs the 5-phase tick loop.
//
// Locked by docs/DECISIONS.md Q51 + docs/SYSTEM_ARCHITECTURE_V2.md.
//
//   Phase 1: Action dispatch.
//   Phase 2: System OnTick callbacks.
//   Phase 3: Event bus drain.
//   Phase 4: Observation build (parallel goroutines).
//   Phase 5: Viewer broadcast (parallel, AOI-filtered).
//
// All five phases run on each Tick() call from the engine's main loop.
// Phases 4 + 5 are parallel within themselves; phases 1-3 hold the
// world write-lock and run sequentially for deterministic ordering.
package pipeline

import (
	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

// World is the runtime surface the pipeline needs.
type World interface {
	systems.World

	// LockWrite / UnlockWrite are explicit here so the pipeline can
	// hold the write-lock across phases 1-3 and release for phase 4-5.
	LockWrite()
	UnlockWrite()
	LockRead()
	UnlockRead()
	IncrementTick() uint64
}

// VerbDispatcher routes incoming actions to the right system's handler.
type VerbDispatcher interface {
	Handle(w systems.World, e systems.Entity, env *systems.ActionEnvelope) systems.ActionResult
}

// ActionQueueReader exposes the per-tick action drain.
type ActionQueueReader interface {
	DrainActions() []QueuedAction
}

type QueuedAction struct {
	EntityID string
	Envelope *systems.ActionEnvelope
	OnResult func(systems.ActionResult)
}

// TickHooks aggregates the per-tick callbacks systems register.
type TickHooks interface {
	RunOnTickAll(w systems.World, tick uint64)
}

// ObservationDispatcher is the thing that decides which agents are
// due for an observation this tick and pushes to their WS hubs.
type ObservationDispatcher interface {
	BuildAndPushReady(w systems.World, tick uint64)
}

// ViewerBroadcaster sends AOI-filtered diffs to subscribed viewers.
type ViewerBroadcaster interface {
	BroadcastTick(w systems.World, tick uint64)
}

// Pipeline runs the phased tick.
type Pipeline struct {
	world      World
	actions    ActionQueueReader
	dispatcher VerbDispatcher
	hooks      TickHooks
	bus        *eventbus.Bus
	observer   ObservationDispatcher
	broadcast  ViewerBroadcaster
}

func New(
	w World,
	actions ActionQueueReader,
	dispatcher VerbDispatcher,
	hooks TickHooks,
	bus *eventbus.Bus,
	observer ObservationDispatcher,
	broadcast ViewerBroadcaster,
) *Pipeline {
	return &Pipeline{
		world: w, actions: actions, dispatcher: dispatcher,
		hooks: hooks, bus: bus, observer: observer, broadcast: broadcast,
	}
}

// Tick runs the 5-phase pipeline once. Caller (engine main loop)
// invokes at engine tick rate (60Hz).
func (p *Pipeline) Tick() {
	tick := p.world.IncrementTick()

	// Phases 1-3: mutate world state. Hold write-lock.
	p.world.LockWrite()
	p.phase1Actions()
	p.phase2SystemTicks(tick)
	p.phase3EventDrain()
	p.world.UnlockWrite()

	// Phases 4-5: read-only; can parallelize.
	p.world.LockRead()
	p.phase4Observations(tick)
	p.phase5Broadcast(tick)
	p.world.UnlockRead()
}

func (p *Pipeline) phase1Actions() {
	queued := p.actions.DrainActions()
	for _, q := range queued {
		e := p.world.EntityByID(q.EntityID)
		if e == nil {
			res := systems.ActionResult{
				ActionID: q.Envelope.ActionID, Verb: q.Envelope.Verb,
				Accepted: false, Reason: "unknown_entity",
			}
			if q.OnResult != nil {
				q.OnResult(res)
			}
			continue
		}
		res := p.dispatcher.Handle(p.world, e, q.Envelope)
		if q.OnResult != nil {
			q.OnResult(res)
		}
	}
}

func (p *Pipeline) phase2SystemTicks(tick uint64) {
	p.hooks.RunOnTickAll(p.world, tick)
}

func (p *Pipeline) phase3EventDrain() {
	p.bus.Drain(p.world)
}

func (p *Pipeline) phase4Observations(tick uint64) {
	if p.observer != nil {
		p.observer.BuildAndPushReady(p.world, tick)
	}
}

func (p *Pipeline) phase5Broadcast(tick uint64) {
	if p.broadcast != nil {
		p.broadcast.BroadcastTick(p.world, tick)
	}
}

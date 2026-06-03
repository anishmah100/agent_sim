package systems

import (
	"fmt"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
)

// ConcreteRegistry is the in-process implementation of Registry. It's
// also the source of the VerbDispatcher / TickHooks the pipeline uses.
type ConcreteRegistry struct {
	verbs        map[string]VerbHandler
	tickHooks    []func(w World, tick uint64)
	spawnHooks   []func(w World, e Entity)
	services     map[string]any
	manifestAgg  *manifest.Aggregator
	bus          *eventbus.Bus
	currentSys   string
}

func NewConcreteRegistry(bus *eventbus.Bus, agg *manifest.Aggregator) *ConcreteRegistry {
	return &ConcreteRegistry{
		verbs:       make(map[string]VerbHandler),
		services:    make(map[string]any),
		manifestAgg: agg,
		bus:         bus,
	}
}

// BeginSystem starts a system's registration block. All subsequent
// calls (Verb, OnEvent, OnTick, ...) are attributed to this system in
// the manifest. Called by the engine before invoking system.RegisterWith.
func (r *ConcreteRegistry) BeginSystem(name string) {
	r.currentSys = name
}

func (r *ConcreteRegistry) EndSystem() {
	r.currentSys = ""
}

// Verb registers a handler. Panics on collision (per design — two
// systems cannot own the same verb).
func (r *ConcreteRegistry) Verb(verb string, h VerbHandler) {
	if _, exists := r.verbs[verb]; exists {
		panic(fmt.Sprintf("verb collision: %q already registered", verb))
	}
	r.verbs[verb] = h
}

func (r *ConcreteRegistry) OnEvent(kind string, h eventbus.Handler) {
	r.bus.Subscribe(kind, h)
}

func (r *ConcreteRegistry) OnTick(h func(w World, tick uint64)) {
	r.tickHooks = append(r.tickHooks, h)
}

func (r *ConcreteRegistry) OnEntitySpawn(h func(w World, e Entity)) {
	r.spawnHooks = append(r.spawnHooks, h)
}

func (r *ConcreteRegistry) Service(name string, svc any) {
	if _, exists := r.services[name]; exists {
		panic(fmt.Sprintf("service collision: %q already registered", name))
	}
	r.services[name] = svc
}

func (r *ConcreteRegistry) Manifest(decl manifest.SystemDeclaration) {
	r.manifestAgg.Add(decl)
}

// === Pipeline-facing accessors ===

// Handle dispatches an action to its verb's handler.
// Implements pipeline.VerbDispatcher.
func (r *ConcreteRegistry) Handle(w World, e Entity, env *ActionEnvelope) ActionResult {
	h, ok := r.verbs[env.Verb]
	if !ok {
		return ActionResult{
			ActionID: env.ActionID, Verb: env.Verb,
			Accepted: false, Reason: "unknown_verb",
		}
	}
	return h(w, e, env)
}

// RunOnTickAll fires every per-tick hook. Implements pipeline.TickHooks.
func (r *ConcreteRegistry) RunOnTickAll(w World, tick uint64) {
	for _, h := range r.tickHooks {
		h(w, tick)
	}
}

// RunOnEntitySpawn fires spawn hooks against the given entity. Called
// at world boot per existing entity and on each SpawnEntity.
func (r *ConcreteRegistry) RunOnEntitySpawn(w World, e Entity) {
	for _, h := range r.spawnHooks {
		h(w, e)
	}
}

// ServiceLookup retrieves a registered service by name.
func (r *ConcreteRegistry) ServiceLookup(name string) any {
	return r.services[name]
}

// InstallServicesInto copies every service registered with this
// registry into the given World, so that verb handlers and other
// systems can reach them via w.GetService(name). Call once after all
// systems have registered, before the first action is dispatched.
func (r *ConcreteRegistry) InstallServicesInto(w World) {
	for name, svc := range r.services {
		w.RegisterService(name, svc)
	}
}

// VerbCount returns how many verbs are registered (observability).
func (r *ConcreteRegistry) VerbCount() int {
	return len(r.verbs)
}

// Verbs returns the registered verb names (observability + tests).
func (r *ConcreteRegistry) Verbs() []string {
	out := make([]string, 0, len(r.verbs))
	for v := range r.verbs {
		out = append(out, v)
	}
	return out
}

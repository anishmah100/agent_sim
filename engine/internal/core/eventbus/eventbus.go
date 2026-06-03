// Package eventbus is the typed event channel between composable systems.
//
// Locked by docs/DECISIONS.md Q51 + docs/SYSTEM_ARCHITECTURE_V2.md.
//
// Usage model:
//   1. System.RegisterWith calls bus.Subscribe("EventKind", handler) at boot.
//   2. During a tick, action handlers + system OnTicks call bus.Queue(event).
//   3. At Phase 3 of the pipeline, bus.Drain runs every queued event past
//      every subscriber to that event's Kind.
//
// Events are typed structs that implement the Event interface. Subscribers
// receive them as `Event` and type-assert. The bus itself doesn't know any
// concrete types — systems own their event vocabulary.
package eventbus

import (
	"sync"
)

// Event — anything a system might emit. Concrete types must declare
// their kind string via Kind(). We use the kind for fast subscriber
// dispatch instead of reflecting on the Go type.
type Event interface {
	Kind() string
}

// Handler — the callback signature for subscribers. WorldCtx is opaque
// here so the bus stays decoupled from World; the pipeline passes the
// real world in when it drains.
type Handler func(world WorldCtx, ev Event)

// WorldCtx is the bare bones the bus exposes to handlers. The real
// engine World implements this; tests can pass a stub.
type WorldCtx interface {
	Tick() uint64
}

// Bus is the publish-subscribe channel for typed events.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]Handler
	queue       []Event
}

func New() *Bus {
	return &Bus{
		subscribers: make(map[string][]Handler),
	}
}

// Subscribe registers a handler for a specific event Kind. Called at
// system init, never during a running tick.
func (b *Bus) Subscribe(kind string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[kind] = append(b.subscribers[kind], h)
}

// Queue records an event for the next Drain. Safe to call from any
// goroutine; events are processed in the order they were queued.
func (b *Bus) Queue(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queue = append(b.queue, ev)
}

// QueueAll batches a slice of events.
func (b *Bus) QueueAll(evs []Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queue = append(b.queue, evs...)
}

// Drain runs every queued event past every subscriber to its Kind.
// Called once per tick (Phase 3 of the pipeline). After drain the
// queue is reset; events emitted DURING drain go into the next tick's
// queue, preventing infinite loops.
//
// Order: events are drained in queue order. Within an event, handlers
// fire in subscription order.
func (b *Bus) Drain(world WorldCtx) int {
	b.mu.Lock()
	pending := b.queue
	b.queue = nil
	subs := b.subscribers
	b.mu.Unlock()

	for _, ev := range pending {
		for _, h := range subs[ev.Kind()] {
			h(world, ev)
		}
	}
	return len(pending)
}

// QueueDepth returns the current pending count. Useful for tests +
// observability.
func (b *Bus) QueueDepth() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.queue)
}

// SubscriberCount returns the number of subscribers for a kind.
func (b *Bus) SubscriberCount(kind string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[kind])
}

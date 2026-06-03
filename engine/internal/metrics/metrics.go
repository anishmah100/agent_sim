// Package metrics — hand-rolled Prometheus text exposition.
//
// Avoids the prometheus client_golang dependency: we already have a
// historian + tick counter, so emitting their state in the exposition
// format is ~50 lines. Lets the engine remain a single Go binary.
//
// Endpoint shape (served by the engine at /metrics):
//
//   # HELP agentsim_tick The current world tick.
//   # TYPE agentsim_tick counter
//   agentsim_tick 1234
//
//   # HELP agentsim_events_emitted_total Total events on the event bus.
//   ...
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// Source describes the data the metrics handler reads. The engine wires
// a concrete impl in main.go; tests can pass a stub.
type Source interface {
	Tick() uint64
	UptimeSeconds() float64
	EntityCount() int
	ViewerCount() int
	AgentCount() int
	EventsEmitted() uint64
	NPCRestarts() int
}

// Handler returns an http.HandlerFunc that emits the Prometheus
// text-exposition format. Caller is expected to register at /metrics.
func Handler(src Source) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; version=0.0.4")
		rw.Header().Set("Cache-Control", "no-store")
		emit(rw, src)
	}
}

func emit(w io.Writer, src Source) {
	fmt.Fprint(w, "# HELP agentsim_tick The current world tick.\n")
	fmt.Fprint(w, "# TYPE agentsim_tick counter\n")
	fmt.Fprintf(w, "agentsim_tick %d\n", src.Tick())

	fmt.Fprint(w, "# HELP agentsim_uptime_seconds Engine uptime since process start.\n")
	fmt.Fprint(w, "# TYPE agentsim_uptime_seconds gauge\n")
	fmt.Fprintf(w, "agentsim_uptime_seconds %f\n", src.UptimeSeconds())

	fmt.Fprint(w, "# HELP agentsim_entities Number of entities in the world.\n")
	fmt.Fprint(w, "# TYPE agentsim_entities gauge\n")
	fmt.Fprintf(w, "agentsim_entities %d\n", src.EntityCount())

	fmt.Fprint(w, "# HELP agentsim_viewers Number of connected viewer clients.\n")
	fmt.Fprint(w, "# TYPE agentsim_viewers gauge\n")
	fmt.Fprintf(w, "agentsim_viewers %d\n", src.ViewerCount())

	fmt.Fprint(w, "# HELP agentsim_agents Number of connected agent clients.\n")
	fmt.Fprint(w, "# TYPE agentsim_agents gauge\n")
	fmt.Fprintf(w, "agentsim_agents %d\n", src.AgentCount())

	fmt.Fprint(w, "# HELP agentsim_events_emitted_total Total events drained on the event bus.\n")
	fmt.Fprint(w, "# TYPE agentsim_events_emitted_total counter\n")
	fmt.Fprintf(w, "agentsim_events_emitted_total %d\n", src.EventsEmitted())

	fmt.Fprint(w, "# HELP agentsim_npc_restarts_total Total restarts across all supervised NPC processes.\n")
	fmt.Fprint(w, "# TYPE agentsim_npc_restarts_total counter\n")
	fmt.Fprintf(w, "agentsim_npc_restarts_total %d\n", src.NPCRestarts())
}

// CounterFromAtomic is a small helper for engine code that owns a
// uint64 counter via the atomic package — wraps it as a Source field.
type CounterFromAtomic struct{ ptr *atomic.Uint64 }

func WrapAtomic(p *atomic.Uint64) CounterFromAtomic { return CounterFromAtomic{p} }
func (c CounterFromAtomic) Value() uint64           { return c.ptr.Load() }

// UptimeSince returns a function that, when called, gives the seconds
// since the given start time. Useful for the Source.UptimeSeconds.
func UptimeSince(t time.Time) func() float64 {
	return func() float64 { return time.Since(t).Seconds() }
}

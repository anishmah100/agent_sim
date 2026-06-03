// Package historian subscribes to every event on the world's bus,
// keeps an in-memory ring of recent events, and (optionally) appends
// them as JSONL to disk for the autoresearch loop's offline analysis.
//
// Per the autoresearch-loop memory (north star): "substrate must log
// EVERYTHING (world + bot reasoning)". This package is the world half
// of that contract. Bot reasoning lives in the SDK side.
//
// The Historian is NOT a system — it consumes the bus, doesn't add
// verbs or state. It's attached to a *world.SystemHost after install.
package historian

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
)

const DefaultRingCapacity = 4096

// Record is a single archived event. Kind + Tick let consumers filter
// without parsing Payload; Payload is the JSON-marshaled event struct.
type Record struct {
	Tick    uint64          `json:"tick"`
	Seq     uint64          `json:"seq"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// Historian collects every event flowing through the bus.
type Historian struct {
	mu       sync.RWMutex
	cap      int
	ring     []Record
	head     int  // next write index
	full     bool // ring has wrapped at least once
	seq      uint64
	logFile  *os.File
}

// New returns a Historian with the given ring capacity (events held in
// memory). diskPath, when non-empty, opens a JSONL file for append and
// every event is written there too. The caller closes the Historian to
// flush the file.
func New(capacity int, diskPath string) (*Historian, error) {
	if capacity <= 0 {
		capacity = DefaultRingCapacity
	}
	h := &Historian{
		cap:  capacity,
		ring: make([]Record, capacity),
	}
	if diskPath != "" {
		if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(diskPath), err)
		}
		f, err := os.OpenFile(diskPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", diskPath, err)
		}
		h.logFile = f
	}
	return h, nil
}

// Attach wires this Historian into a bus. Call once at engine boot,
// after all systems have registered (the bus is fully populated by
// then, so wildcard subscribers can't miss the very first emission).
func (h *Historian) Attach(bus *eventbus.Bus) {
	bus.SubscribeAll(h.observe)
}

func (h *Historian) observe(world eventbus.WorldCtx, ev eventbus.Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		// Don't crash the tick over a marshal failure; record the kind
		// at least so the gap is visible.
		payload = []byte(`{"_marshal_error":"` + err.Error() + `"}`)
	}
	h.mu.Lock()
	rec := Record{
		Tick:    world.Tick(),
		Seq:     h.seq,
		Kind:    ev.Kind(),
		Payload: json.RawMessage(payload),
	}
	h.seq++
	h.ring[h.head] = rec
	h.head = (h.head + 1) % h.cap
	if h.head == 0 {
		h.full = true
	}
	logFile := h.logFile
	h.mu.Unlock()

	if logFile != nil {
		line, _ := json.Marshal(rec)
		line = append(line, '\n')
		_, _ = logFile.Write(line)
	}
}

// Recent returns up to `limit` most-recent records, optionally
// filtered to events with Tick >= sinceTick.
func (h *Historian) Recent(sinceTick uint64, limit int) []Record {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if limit <= 0 {
		limit = h.cap
	}
	if limit > h.cap {
		limit = h.cap
	}
	// Walk backwards from head.
	out := make([]Record, 0, limit)
	pos := h.head - 1
	end := -1
	if h.full {
		end = (h.head - h.cap + h.cap) % h.cap
		_ = end
	}
	count := h.cap
	if !h.full {
		count = h.head
	}
	for i := 0; i < count && len(out) < limit; i++ {
		if pos < 0 {
			pos += h.cap
		}
		rec := h.ring[pos]
		if rec.Tick >= sinceTick {
			out = append(out, rec)
		}
		pos--
	}
	// Reverse so result is chronological ascending.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Stats returns observability counts.
func (h *Historian) Stats() Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	stored := h.head
	if h.full {
		stored = h.cap
	}
	return Stats{
		TotalEmitted: h.seq,
		InRing:       stored,
		Capacity:     h.cap,
	}
}

type Stats struct {
	TotalEmitted uint64 `json:"total_emitted"`
	InRing       int    `json:"in_ring"`
	Capacity     int    `json:"capacity"`
}

// Close flushes + closes the on-disk log if one was opened.
func (h *Historian) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.logFile != nil {
		err := h.logFile.Close()
		h.logFile = nil
		return err
	}
	return nil
}

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

// Record is a single archived event. Kind + Tick + Category let
// consumers filter without parsing Payload; Payload is the JSON-marshaled
// event struct.
type Record struct {
	Tick     uint64          `json:"tick"`
	Seq      uint64          `json:"seq"`
	Kind     string          `json:"kind"`
	Category string          `json:"category,omitempty"`
	Payload  json.RawMessage `json:"payload"`
}

// Category is the coarse-grained tag used to gate logging. See
// docs/EXPERIMENT_SYSTEM_PLAN.md §7 — long-running experiments often
// want to drop noisy categories (movement, system) without losing the
// signal categories (combat, economy, social).
const (
	CategorySystem    = "system"
	CategoryMovement  = "movement"
	CategoryCombat    = "combat"
	CategoryEconomy   = "economy"
	CategorySocial    = "social"
	CategoryReasoning = "agent_reasoning"
	CategoryWorld     = "world" // catch-all
)

// CategoryFilter controls which categories are written. A category in
// Disabled is dropped from BOTH the in-memory ring and the JSONL log.
// Empty filter (all-on) is the default — engines run with full
// observability unless an experiment configures otherwise.
type CategoryFilter struct {
	Disabled map[string]bool
}

// classify maps a known event Kind onto a Category. Unknown kinds get
// CategoryWorld so they're never silently dropped. Update this map
// when a new system emits a new event Kind.
var categoryByKind = map[string]string{
	// Combat (combat package).
	"DamageDealt":  CategoryCombat,
	"EntityDied":   CategoryCombat,
	// Economy (money, trade, loot).
	"GoldTransferred":  CategoryEconomy,
	"TradeCompleted":   CategoryEconomy,
	"ItemLooted":       CategoryEconomy,
	"ItemPicked":       CategoryEconomy,
	"ItemDropped":      CategoryEconomy,
	"ItemTransferred":  CategoryEconomy,
	"ResourceHarvested": CategoryEconomy,
	"ResourceDepleted":  CategoryEconomy,
	// Social (audibility).
	"Speech":  CategorySocial,
	"Whisper": CategorySocial,
	"Shout":   CategorySocial,
	"Sound":   CategorySocial,
	// Movement.
	"EntityMoved":   CategoryMovement,
	"FacingChanged": CategoryMovement,
	// Vitality (vitals package).
	"HungerSpike": CategoryWorld,
	// Property — building enter/exit, lock/unlock, ownership.
	// Without explicit category these were silently routed to
	// CategoryWorld, which still wrote them to disk but made them
	// invisible to category-scoped consumers (and easier to lose
	// in noisy world-event streams).
	"EnteredBuilding":  CategoryWorld,
	"ExitedBuilding":   CategoryWorld,
	"BuildingLocked":   CategoryWorld,
	"BuildingUnlocked": CategoryWorld,
	"OwnershipChanged": CategoryWorld,
	// Construction.
	"ConstructionStarted":   CategoryWorld,
	"ConstructionAdvanced":  CategoryWorld,
	"ConstructionCompleted": CategoryWorld,
	"Demolished":            CategoryWorld,
	// System.
	"SystemBoot":     CategorySystem,
	"SystemShutdown": CategorySystem,
	// Generic accepted-action breadcrumb. Categorized by the bundled
	// verb in score_a9.py — at this layer we keep it under "world".
	"ActionAccepted": CategoryWorld,
}

// classify returns the category for an event Kind. Unknown kinds get
// CategoryWorld.
func classify(kind string) string {
	if c, ok := categoryByKind[kind]; ok {
		return c
	}
	return CategoryWorld
}

// ReasoningTrace is the historian's record shape for the per-action
// free-text reasoning emitted by the tactical brain. The historian
// records it in the same JSONL stream under category=agent_reasoning
// so analysis tools can replay (action, reasoning) pairs in one pass.
type ReasoningTrace struct {
	EntityID  string `json:"entity_id"`
	ActionID  string `json:"action_id"`
	Verb      string `json:"verb"`
	Reasoning string `json:"reasoning"`
}

// LogReasoning is the entry point main.go installs as wire.AgentHub.
// OnReasoning. Writes a record under category=agent_reasoning regardless
// of whether the agent_reasoning category is muted — the layered
// opt-in (experiment + agent flags) already gated upstream, so muting
// would be misleading at this point.
func (h *Historian) LogReasoning(currentTick uint64, t ReasoningTrace) {
	if h == nil {
		return
	}
	if h.filter.Disabled != nil && h.filter.Disabled[CategoryReasoning] {
		// Defensive: if the category is muted, drop anyway.
		return
	}
	payload, _ := json.Marshal(t)
	h.appendRecord(Record{
		Tick:     currentTick,
		Kind:     "ReasoningTrace",
		Category: CategoryReasoning,
		Payload:  json.RawMessage(payload),
	})
}

// ReflectiveNote is the slow-loop counterpart to ReasoningTrace.
// Emitted by the brain's reflective layer (~once per minute) when it
// updates goals or theory-of-mind. Stored under the same
// agent_reasoning category so a single jq query reads both.
type ReflectiveNote struct {
	EntityID string `json:"entity_id"`
	Note     string `json:"note"`
}

// LogReflection — symmetrical with LogReasoning. Gated by the same
// layered opt-in upstream.
func (h *Historian) LogReflection(currentTick uint64, n ReflectiveNote) {
	if h == nil {
		return
	}
	if h.filter.Disabled != nil && h.filter.Disabled[CategoryReasoning] {
		return
	}
	payload, _ := json.Marshal(n)
	h.appendRecord(Record{
		Tick:     currentTick,
		Kind:     "ReflectiveNote",
		Category: CategoryReasoning,
		Payload:  json.RawMessage(payload),
	})
}

// appendRecord — shared write path for LogReasoning + LogReflection.
// Assigns seq under the mutex, then writes the JSONL line outside the
// mutex (the file is append-only and Write is atomic on POSIX).
func (h *Historian) appendRecord(rec Record) {
	h.mu.Lock()
	rec.Seq = h.seq
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

// Historian collects every event flowing through the bus.
type Historian struct {
	mu      sync.RWMutex
	cap     int
	ring    []Record
	head    int  // next write index
	full    bool // ring has wrapped at least once
	seq     uint64
	logFile *os.File
	filter  CategoryFilter
}

// New returns a Historian with the given ring capacity (events held in
// memory). diskPath, when non-empty, opens a JSONL file for append and
// every event is written there too. The caller closes the Historian to
// flush the file.
func New(capacity int, diskPath string) (*Historian, error) {
	return NewWithFilter(capacity, diskPath, CategoryFilter{})
}

// NewWithFilter is the rich constructor: same as New, plus a category
// filter that drops events whose Category is in filter.Disabled. Used
// by experiments that want to silence noisy categories.
func NewWithFilter(capacity int, diskPath string, filter CategoryFilter) (*Historian, error) {
	if capacity <= 0 {
		capacity = DefaultRingCapacity
	}
	h := &Historian{
		cap:    capacity,
		ring:   make([]Record, capacity),
		filter: filter,
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
	kind := ev.Kind()
	category := classify(kind)
	// Category gate: if disabled, drop entirely (ring + disk).
	if h.filter.Disabled != nil && h.filter.Disabled[category] {
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		// Don't crash the tick over a marshal failure; record the kind
		// at least so the gap is visible.
		payload = []byte(`{"_marshal_error":"` + err.Error() + `"}`)
	}
	h.mu.Lock()
	rec := Record{
		Tick:     world.Tick(),
		Seq:      h.seq,
		Kind:     kind,
		Category: category,
		Payload:  json.RawMessage(payload),
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

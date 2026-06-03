package historian

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
)


// stubEvent + stubWorld are local fakes — historian must work with any
// Event/WorldCtx since it doesn't depend on the real *world.World.
type stubEvent struct {
	K    string
	Note string
}

func (s stubEvent) Kind() string { return s.K }

type stubWorld struct{ t uint64 }

func (s *stubWorld) Tick() uint64 { return s.t }

func TestObserveAndRecent(t *testing.T) {
	bus := eventbus.New()
	h, err := New(8, "")
	if err != nil {
		t.Fatal(err)
	}
	h.Attach(bus)

	w := &stubWorld{t: 0}
	for i := 0; i < 5; i++ {
		w.t = uint64(i + 1)
		bus.Queue(stubEvent{K: "Foo", Note: "n"})
		bus.Drain(w)
	}

	all := h.Recent(0, 0)
	if len(all) != 5 {
		t.Fatalf("expected 5 records, got %d", len(all))
	}
	// Chronological order.
	for i, r := range all {
		if r.Tick != uint64(i+1) {
			t.Fatalf("record %d tick=%d", i, r.Tick)
		}
	}

	// Filter by since.
	recent := h.Recent(3, 0)
	if len(recent) != 3 {
		t.Fatalf("since=3 expected 3, got %d", len(recent))
	}
	if recent[0].Tick != 3 {
		t.Fatalf("recent[0].Tick=%d", recent[0].Tick)
	}
}

func TestRingWraps(t *testing.T) {
	bus := eventbus.New()
	h, _ := New(4, "")
	h.Attach(bus)
	w := &stubWorld{}
	for i := 1; i <= 10; i++ {
		w.t = uint64(i)
		bus.Queue(stubEvent{K: "X"})
		bus.Drain(w)
	}
	stats := h.Stats()
	if stats.TotalEmitted != 10 {
		t.Fatalf("total=%d", stats.TotalEmitted)
	}
	if stats.InRing != 4 {
		t.Fatalf("in_ring=%d", stats.InRing)
	}
	recs := h.Recent(0, 0)
	if len(recs) != 4 {
		t.Fatalf("recent len=%d", len(recs))
	}
	// Oldest record in ring should be tick 7 (10-3).
	if recs[0].Tick != 7 {
		t.Fatalf("oldest tick=%d (expected 7)", recs[0].Tick)
	}
}

func TestDiskLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.jsonl")
	bus := eventbus.New()
	h, err := New(4, logPath)
	if err != nil {
		t.Fatal(err)
	}
	h.Attach(bus)
	w := &stubWorld{t: 5}
	bus.Queue(stubEvent{K: "Foo", Note: "first"})
	bus.Queue(stubEvent{K: "Bar", Note: "second"})
	bus.Drain(w)
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	// Two lines, both valid JSON Records.
	lines := 0
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var rec Record
		if err := dec.Decode(&rec); err != nil {
			t.Fatalf("decode line: %v", err)
		}
		lines++
	}
	if lines != 2 {
		t.Fatalf("expected 2 lines, got %d", lines)
	}
}

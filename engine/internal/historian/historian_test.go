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

func TestCategoryClassification(t *testing.T) {
	// Spot-check the classifier covers the known event kinds.
	cases := map[string]string{
		"DamageDealt":     CategoryCombat,
		"GoldTransferred": CategoryEconomy,
		"Whisper":         CategorySocial,
		"EntityMoved":     CategoryMovement,
		"SystemBoot":      CategorySystem,
		"HungerSpike":     CategoryWorld,
		"NeverHeardOf":    CategoryWorld, // default
	}
	for kind, want := range cases {
		if got := classify(kind); got != want {
			t.Errorf("classify(%q): want %q, got %q", kind, want, got)
		}
	}
}

func TestCategoryGate_DropsDisabled(t *testing.T) {
	// Create a historian that mutes Movement; emit one Combat + one
	// Movement event; only Combat should land in the ring + disk.
	bus := eventbus.New()
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "events.jsonl")
	h, err := NewWithFilter(8, logPath, CategoryFilter{
		Disabled: map[string]bool{CategoryMovement: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.Attach(bus)
	w := &stubWorld{t: 1}
	bus.Queue(stubEvent{K: "DamageDealt", Note: "hit"})
	bus.Queue(stubEvent{K: "EntityMoved", Note: "step"})
	bus.Drain(w)
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	recs := h.Recent(0, 0)
	if len(recs) != 1 {
		t.Fatalf("ring should hold 1 record (movement gated); got %d", len(recs))
	}
	if recs[0].Category != CategoryCombat {
		t.Fatalf("kept record category: want combat, got %q", recs[0].Category)
	}
	// Disk file also has just one line.
	data, _ := os.ReadFile(logPath)
	lines := bytes.Count(data, []byte("\n"))
	if lines != 1 {
		t.Fatalf("disk: want 1 line, got %d", lines)
	}
}

func TestLogReasoning_LandsInRing(t *testing.T) {
	h, err := New(8, "")
	if err != nil {
		t.Fatal(err)
	}
	h.LogReasoning(42, ReasoningTrace{
		EntityID:  "hero",
		ActionID:  "act-1",
		Verb:      "move",
		Reasoning: "heading to blacksmith to buy hammer",
	})
	recs := h.Recent(0, 0)
	if len(recs) != 1 {
		t.Fatalf("want 1 reasoning record, got %d", len(recs))
	}
	if recs[0].Category != CategoryReasoning {
		t.Fatalf("category: want %q, got %q", CategoryReasoning, recs[0].Category)
	}
	if recs[0].Tick != 42 {
		t.Fatalf("tick: %d", recs[0].Tick)
	}
}

func TestLogReasoning_HonorsMute(t *testing.T) {
	h, err := NewWithFilter(8, "", CategoryFilter{
		Disabled: map[string]bool{CategoryReasoning: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.LogReasoning(7, ReasoningTrace{EntityID: "hero", ActionID: "x", Verb: "wait"})
	if got := len(h.Recent(0, 0)); got != 0 {
		t.Fatalf("muted reasoning should drop; got %d records", got)
	}
}

func TestCategoryGate_NoFilterKeepsAll(t *testing.T) {
	bus := eventbus.New()
	h, err := New(8, "")
	if err != nil {
		t.Fatal(err)
	}
	h.Attach(bus)
	w := &stubWorld{t: 1}
	bus.Queue(stubEvent{K: "EntityMoved"})
	bus.Queue(stubEvent{K: "DamageDealt"})
	bus.Queue(stubEvent{K: "Whisper"})
	bus.Drain(w)
	if got := len(h.Recent(0, 0)); got != 3 {
		t.Fatalf("default historian keeps all; got %d", got)
	}
}

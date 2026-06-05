package world

import (
	"encoding/json"
	"testing"
)

// Tests for the Phase A async action queue + auto-spawn paths.

func tickN(w *World, n int) {
	for i := 0; i < n; i++ {
		w.Tick()
	}
}

func TestQueueAction_BasicEnqueueAndDrain(t *testing.T) {
	w := loadTestWorld(t)
	// Queue a "speak" action; this should be applied at the next Tick().
	env := &ActionEnvelope{
		ActionID: "act-1",
		Verb:     "speak",
		Raw:      json.RawMessage(`{"text":"hello"}`),
	}
	replyCh := w.QueueAction("a", env)
	// Before Tick(), the action sits in the queue — reply is pending.
	select {
	case <-replyCh:
		t.Fatal("reply received before Tick — action drained too early")
	default:
		// expected
	}
	w.Tick()
	select {
	case res := <-replyCh:
		if !res.Accepted {
			t.Fatalf("speak should be Accepted, got reason=%q", res.Reason)
		}
		if res.ActionID != "act-1" {
			t.Fatalf("ActionID round-trip: got %q", res.ActionID)
		}
	default:
		t.Fatal("Tick() did not drain the action")
	}
}

func TestQueueAction_UnknownEntityRejected(t *testing.T) {
	w := loadTestWorld(t)
	env := &ActionEnvelope{ActionID: "x", Verb: "speak", Raw: json.RawMessage(`{"text":"hi"}`)}
	ch := w.QueueAction("not-a-real-entity", env)
	w.Tick()
	res := <-ch
	if res.Accepted {
		t.Fatal("unknown entity should not be Accepted")
	}
	if res.Reason != "unknown_entity" {
		t.Fatalf("expected reason=unknown_entity, got %q", res.Reason)
	}
}

func TestQueueAction_FIFOOrdering(t *testing.T) {
	w := loadTestWorld(t)
	// Submit 3 speak actions; each carries a distinct ActionID. They
	// must apply in submission order (Go's channel FIFO guarantee).
	chs := make([]<-chan ActionResult, 0, 3)
	for i, id := range []string{"first", "second", "third"} {
		env := &ActionEnvelope{
			ActionID: id, Verb: "speak",
			Raw: json.RawMessage(`{"text":"x"}`),
		}
		chs = append(chs, w.QueueAction("a", env))
		_ = i
	}
	w.Tick()
	for i, ch := range chs {
		select {
		case res := <-ch:
			expected := []string{"first", "second", "third"}[i]
			if res.ActionID != expected {
				t.Fatalf("position %d: got ActionID %q, want %q", i, res.ActionID, expected)
			}
			if !res.Accepted {
				t.Fatalf("position %d: not Accepted (%s)", i, res.Reason)
			}
		default:
			t.Fatalf("position %d: reply never arrived", i)
		}
	}
}

func TestQueueAction_QueueFullBackpressure(t *testing.T) {
	w := loadTestWorld(t)
	// Replace the queue with a tiny one so we can easily fill it.
	w.actionQueue = make(chan *pendingAction, 2)
	mk := func(id string) *ActionEnvelope {
		return &ActionEnvelope{
			ActionID: id, Verb: "speak",
			Raw: json.RawMessage(`{"text":"x"}`),
		}
	}
	chA := w.QueueAction("a", mk("a"))
	chB := w.QueueAction("a", mk("b"))
	// Third action — queue is at capacity. Must be rejected with
	// queue_full immediately, without blocking.
	chC := w.QueueAction("a", mk("c"))
	select {
	case res := <-chC:
		if res.Accepted || res.Reason != "queue_full" {
			t.Fatalf("c: want queue_full reject, got accepted=%v reason=%q",
				res.Accepted, res.Reason)
		}
	default:
		t.Fatal("queue_full reject should be immediate (sync send on cap-1 reply chan)")
	}
	// Drain the two that did make it in.
	w.Tick()
	_ = <-chA
	_ = <-chB
}

func TestSpawnAgentEntity_LandsOnWalkableTile(t *testing.T) {
	w := loadTestWorld(t)
	id, err := w.SpawnAgentEntity("wanderer", "Test Wanderer")
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if id == "" {
		t.Fatal("spawned id is empty")
	}
	e := w.EntityByID(id)
	if e == nil {
		t.Fatal("EntityByID returned nil for the just-spawned entity")
	}
	if e.Archetype != "wanderer" {
		t.Fatalf("archetype: got %q", e.Archetype)
	}
	if e.DisplayName != "Test Wanderer" {
		t.Fatalf("display_name: got %q", e.DisplayName)
	}
	if !w.IsWalkable(e.LogicalTile) {
		t.Fatalf("spawn landed on unwalkable tile %v", e.LogicalTile)
	}
}

func TestSpawnAgentEntity_DefaultArchetype(t *testing.T) {
	w := loadTestWorld(t)
	// Empty archetype → defaults to "wanderer".
	id, err := w.SpawnAgentEntity("", "")
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	e := w.EntityByID(id)
	if e == nil {
		t.Fatal("entity gone")
	}
	if e.Archetype != "wanderer" {
		t.Fatalf("default archetype should be wanderer, got %q", e.Archetype)
	}
}

func TestTick_PublishesSnapshot(t *testing.T) {
	w := loadTestWorld(t)
	// Before any Tick(), snapshot pointer is nil.
	if snap := w.LoadSnapshot(); snap != nil {
		t.Fatal("snapshot should be nil before first Tick()")
	}
	w.Tick()
	snap := w.LoadSnapshot()
	if snap == nil {
		t.Fatal("Tick() did not publish a snapshot")
	}
	if snap.Tick != 1 {
		t.Fatalf("snap.Tick: want 1, got %d", snap.Tick)
	}
	if len(snap.Entities) != 2 {
		t.Fatalf("entities: want 2, got %d", len(snap.Entities))
	}
	if _, ok := snap.Entities["a"]; !ok {
		t.Fatal("entity 'a' missing from snapshot")
	}
}

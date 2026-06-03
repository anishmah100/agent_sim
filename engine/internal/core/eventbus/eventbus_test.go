package eventbus

import "testing"

type stubWorld struct{ tick uint64 }

func (s *stubWorld) Tick() uint64 { return s.tick }

type dummyEvent struct{ Name string }

func (dummyEvent) Kind() string { return "DummyEvent" }

func TestBus_SubscribeQueueDrain(t *testing.T) {
	b := New()
	got := []string{}
	b.Subscribe("DummyEvent", func(w WorldCtx, ev Event) {
		got = append(got, ev.(dummyEvent).Name)
	})
	b.Queue(dummyEvent{Name: "a"})
	b.Queue(dummyEvent{Name: "b"})
	if b.QueueDepth() != 2 {
		t.Fatalf("expected 2 queued, got %d", b.QueueDepth())
	}
	n := b.Drain(&stubWorld{tick: 1})
	if n != 2 {
		t.Fatalf("expected drain 2, got %d", n)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("delivery order wrong: %v", got)
	}
	if b.QueueDepth() != 0 {
		t.Fatalf("queue not reset after drain: %d", b.QueueDepth())
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	b := New()
	calls := 0
	for i := 0; i < 3; i++ {
		b.Subscribe("DummyEvent", func(w WorldCtx, ev Event) { calls++ })
	}
	b.Queue(dummyEvent{})
	b.Drain(&stubWorld{})
	if calls != 3 {
		t.Fatalf("3 subscribers should fire; got %d", calls)
	}
	if b.SubscriberCount("DummyEvent") != 3 {
		t.Fatalf("subscriber count wrong: %d", b.SubscriberCount("DummyEvent"))
	}
}

func TestBus_DrainDoesNotLoopOnInTickEmit(t *testing.T) {
	b := New()
	// A handler that emits MORE events when called. Those should land
	// in the next drain, not the current one. Prevents infinite loops.
	delivered := 0
	b.Subscribe("DummyEvent", func(w WorldCtx, ev Event) {
		delivered++
		if ev.(dummyEvent).Name == "trigger" {
			b.Queue(dummyEvent{Name: "from_handler"})
		}
	})
	b.Queue(dummyEvent{Name: "trigger"})
	b.Drain(&stubWorld{})
	if delivered != 1 {
		t.Fatalf("first drain should deliver only the trigger, got %d", delivered)
	}
	if b.QueueDepth() != 1 {
		t.Fatal("re-emitted event should be in next-tick queue")
	}
	b.Drain(&stubWorld{})
	if delivered != 2 {
		t.Fatalf("second drain should deliver the re-emitted event, got %d", delivered)
	}
}

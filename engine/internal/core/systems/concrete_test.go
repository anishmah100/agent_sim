package systems

import (
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
)

type stubEntity struct {
	id        string
	archetype string
	pos       [2]int
	extras    map[string]any
}

func (s *stubEntity) ID() string                    { return s.id }
func (s *stubEntity) Archetype() string              { return s.archetype }
func (s *stubEntity) Pos() [2]int                    { return s.pos }
func (s *stubEntity) SetExtra(k string, v any)       { s.extras[k] = v }
func (s *stubEntity) GetExtra(k string) (any, bool)  { v, ok := s.extras[k]; return v, ok }

type stubWorld struct {
	tick uint64
}

func (s *stubWorld) Tick() uint64                      { return s.tick }
func (s *stubWorld) EntityByID(string) Entity          { return nil }
func (s *stubWorld) EntityIDs() []string               { return nil }
func (s *stubWorld) MutateEntity(string, func(Entity)) {}
func (s *stubWorld) SpawnEntity(Entity) error          { return nil }
func (s *stubWorld) SpawnEntityFromSpec(EntitySpec) (Entity, error) { return nil, nil }
func (s *stubWorld) RemoveEntity(string) error         { return nil }
func (s *stubWorld) EmitSound([2]int, string)          {}
func (s *stubWorld) EmitDeathScream([2]int, string, string, bool) {}
func (s *stubWorld) QueueEvent(eventbus.Event)         {}
func (s *stubWorld) GetService(string) any             { return nil }
func (s *stubWorld) RegisterService(string, any)       {}
func (s *stubWorld) EntitiesInRadius([2]int, int) []string { return nil }
func (s *stubWorld) IsWalkable([2]int) bool            { return true }
func (s *stubWorld) EnterBuilding(string, string, int) bool { return false }
func (s *stubWorld) ExitBuilding(string) bool          { return false }
func (s *stubWorld) InsideBuilding(string) string      { return "" }
func (s *stubWorld) Tuning(_ string, d float64) float64 { return d }
func (s *stubWorld) TuningInt(_ string, d int) int      { return d }
func (s *stubWorld) TuningBool(_ string, d bool) bool   { return d }
func (s *stubWorld) Chebyshev(a, b [2]int) int         {
	dx, dy := a[0]-b[0], a[1]-b[1]
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

func TestRegistry_VerbCollisionPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on verb collision")
		}
	}()
	bus := eventbus.New()
	agg := manifest.NewAggregator("test", "test")
	r := NewConcreteRegistry(bus, agg)
	r.Verb("move", func(World, Entity, *ActionEnvelope) ActionResult { return ActionResult{} })
	r.Verb("move", func(World, Entity, *ActionEnvelope) ActionResult { return ActionResult{} })
}

func TestRegistry_HandleUnknownVerb(t *testing.T) {
	r := NewConcreteRegistry(eventbus.New(), manifest.NewAggregator("t", "t"))
	res := r.Handle(&stubWorld{}, &stubEntity{}, &ActionEnvelope{Verb: "nope", ActionID: "x"})
	if res.Accepted || res.Reason != "unknown_verb" {
		t.Fatalf("unknown verb should be rejected: %+v", res)
	}
}

func TestRegistry_HandleRegisteredVerb(t *testing.T) {
	r := NewConcreteRegistry(eventbus.New(), manifest.NewAggregator("t", "t"))
	r.Verb("speak", func(w World, e Entity, env *ActionEnvelope) ActionResult {
		return ActionResult{ActionID: env.ActionID, Verb: "speak", Accepted: true}
	})
	res := r.Handle(&stubWorld{}, &stubEntity{}, &ActionEnvelope{Verb: "speak", ActionID: "1"})
	if !res.Accepted {
		t.Fatalf("speak should be accepted: %+v", res)
	}
}

func TestRegistry_TickHooksRunInOrder(t *testing.T) {
	r := NewConcreteRegistry(eventbus.New(), manifest.NewAggregator("t", "t"))
	order := []string{}
	r.OnTick(func(World, uint64) { order = append(order, "a") })
	r.OnTick(func(World, uint64) { order = append(order, "b") })
	r.RunOnTickAll(&stubWorld{}, 5)
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("tick hooks order: %v", order)
	}
}

func TestRegistry_ServiceLookup(t *testing.T) {
	r := NewConcreteRegistry(eventbus.New(), manifest.NewAggregator("t", "t"))
	type combatSvc interface{ Hello() string }
	type combatImpl struct{}
	hi := func() string { return "boom" }
	r.Service("combat", &struct{ Hello func() string }{Hello: hi})
	got := r.ServiceLookup("combat")
	if got == nil {
		t.Fatal("service not registered")
	}
	_ = combatImpl{}
	_ = (*combatSvc)(nil)
}

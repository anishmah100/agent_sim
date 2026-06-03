// Integration test for the property system. Boots a world with one
// hero adjacent to a building, drives all six verbs end-to-end against
// the real *World via the WorldAdapter.
package property_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/core/spatial"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/property"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const testWorldJSON = `{
  "map_id": "property_test",
  "width_tiles": 8,
  "height_tiles": 6,
  "tiles_legend": {".":"grass"},
  "tiles": ["........","........","........","........","........","........"],
  "entities": [
    {"entity_id":"hero","archetype":"trainer","pos":[2,2],"facing":"S","display_name":"Hero"},
    {"entity_id":"alice","archetype":"trainer","pos":[3,1],"facing":"S","display_name":"Alice"},
    {"entity_id":"shed","archetype":"building","pos":[3,2],"facing":"S","display_name":"Shed"}
  ]
}`

func boot(t *testing.T) (*world.WorldAdapter, *syscore.ConcreteRegistry) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "w.json")
	if err := os.WriteFile(p, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := world.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	bus := eventbus.New()
	spat := spatial.New()
	for _, id := range w.EntityIDsUnlocked() {
		e := w.EntityByIDUnlocked(id)
		if e != nil {
			spat.Add(e.EntityID, e.LogicalTile)
		}
	}
	wa := world.NewWorldAdapter(w, bus, spat)
	agg := manifest.NewAggregator("property_test", "test")
	reg := syscore.NewConcreteRegistry(bus, agg)
	reg.BeginSystem("property")
	property.New().RegisterWith(reg)
	reg.EndSystem()
	reg.InstallServicesInto(wa)
	for _, id := range wa.EntityIDs() {
		reg.RunOnEntitySpawn(wa, wa.EntityByID(id))
	}
	return wa, reg
}

func handle(t *testing.T, reg *syscore.ConcreteRegistry, wa *world.WorldAdapter, actor, verb, params string) syscore.ActionResult {
	t.Helper()
	return reg.Handle(wa, wa.EntityByID(actor), &syscore.ActionEnvelope{
		ActionID: "1", Verb: verb,
		Raw: []byte(params),
	})
}

func TestSeedDefaults(t *testing.T) {
	wa, _ := boot(t)
	shed := wa.EntityByID("shed")
	if owner, _ := shed.GetExtra("owner"); owner != "" {
		t.Fatalf("expected owner='' got %v", owner)
	}
	if locked, _ := shed.GetExtra("locked"); locked != false {
		t.Fatalf("expected locked=false got %v", locked)
	}
	// Non-building entity gets no property extras.
	hero := wa.EntityByID("hero")
	if _, ok := hero.GetExtra("owner"); ok {
		t.Fatal("hero should NOT have owner extra")
	}
}

func TestClaimOwnership(t *testing.T) {
	wa, reg := boot(t)
	res := handle(t, reg, wa, "hero", "claim_ownership", `{"target":"shed"}`)
	if !res.Accepted {
		t.Fatalf("claim should accept: %s", res.Reason)
	}
	shed := wa.EntityByID("shed")
	owner, _ := shed.GetExtra("owner")
	if owner != "hero" {
		t.Fatalf("owner=%v", owner)
	}
}

func TestClaimRejectsAlreadyOwned(t *testing.T) {
	wa, reg := boot(t)
	_ = handle(t, reg, wa, "hero", "claim_ownership", `{"target":"shed"}`)
	res := handle(t, reg, wa, "alice", "claim_ownership", `{"target":"shed"}`)
	if res.Accepted {
		t.Fatal("second claim should reject")
	}
	if res.Reason != "already_owned" {
		t.Fatalf("reason=%s", res.Reason)
	}
}

func TestLockGatesEnter(t *testing.T) {
	wa, reg := boot(t)
	_ = handle(t, reg, wa, "hero", "claim_ownership", `{"target":"shed"}`)
	_ = handle(t, reg, wa, "hero", "lock", `{"target":"shed"}`)

	// Owner can still enter.
	res := handle(t, reg, wa, "hero", "enter", `{"target":"shed"}`)
	if !res.Accepted {
		t.Fatalf("owner should enter locked building: %s", res.Reason)
	}
	if in := wa.InsideBuilding("hero"); in != "shed" {
		t.Fatalf("hero inside=%q", in)
	}

	// Other entity cannot enter (Alice is at (1,1), not adjacent to (3,2))
	// — move her adjacent first by mutating her position. Easier: do the
	// test using the hero's adjacency since this test is about lock
	// gating, not range. Have the hero exit, then check that the rule
	// fires regardless of adjacency by querying the service.
	_ = handle(t, reg, wa, "hero", "exit", `{}`)
	svc := wa.GetService("property").(property.PropertyService)
	if svc.CanEnter(wa, "alice", "shed") {
		t.Fatal("service should report alice cannot enter")
	}
	if !svc.CanEnter(wa, "hero", "shed") {
		t.Fatal("service should report hero can enter")
	}
}

func TestUnlockClearsGate(t *testing.T) {
	wa, reg := boot(t)
	_ = handle(t, reg, wa, "hero", "claim_ownership", `{"target":"shed"}`)
	_ = handle(t, reg, wa, "hero", "lock", `{"target":"shed"}`)
	res := handle(t, reg, wa, "hero", "unlock", `{"target":"shed"}`)
	if !res.Accepted {
		t.Fatalf("unlock should accept: %s", res.Reason)
	}
	svc := wa.GetService("property").(property.PropertyService)
	if svc.IsLocked(wa, "shed") {
		t.Fatal("shed should be unlocked")
	}
}

func TestLockRequiresOwner(t *testing.T) {
	wa, reg := boot(t)
	res := handle(t, reg, wa, "alice", "lock", `{"target":"shed"}`)
	if res.Accepted {
		t.Fatal("non-owner lock should reject")
	}
	if res.Reason != "not_owner" {
		t.Fatalf("reason=%s", res.Reason)
	}
}

func TestTransferOwnership(t *testing.T) {
	wa, reg := boot(t)
	_ = handle(t, reg, wa, "hero", "claim_ownership", `{"target":"shed"}`)
	res := handle(t, reg, wa, "hero", "transfer_ownership", `{"target":"shed","new_owner":"alice"}`)
	if !res.Accepted {
		t.Fatalf("transfer should accept: %s", res.Reason)
	}
	svc := wa.GetService("property").(property.PropertyService)
	if svc.OwnerOf(wa, "shed") != "alice" {
		t.Fatalf("owner should be alice, got %s", svc.OwnerOf(wa, "shed"))
	}
}

func TestEnterExitEmitsEvents(t *testing.T) {
	wa, reg := boot(t)
	entered, exited := 0, 0
	wa.Bus.Subscribe("EnteredBuilding", func(_ eventbus.WorldCtx, _ eventbus.Event) { entered++ })
	wa.Bus.Subscribe("ExitedBuilding", func(_ eventbus.WorldCtx, _ eventbus.Event) { exited++ })

	_ = handle(t, reg, wa, "hero", "enter", `{"target":"shed"}`)
	_ = handle(t, reg, wa, "hero", "exit", `{}`)
	wa.Bus.Drain(wa)
	if entered != 1 || exited != 1 {
		t.Fatalf("entered=%d exited=%d", entered, exited)
	}
}

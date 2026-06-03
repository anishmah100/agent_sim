package construction_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/core/spatial"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/construction"
	"github.com/anishmah100/agent_sim/engine/internal/systems/inventory"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const testWorldJSON = `{
  "map_id": "construction_test",
  "width_tiles": 8,
  "height_tiles": 6,
  "tiles_legend": {".":"grass"},
  "tiles": ["........","........","........","........","........","........"],
  "entities": [
    {"entity_id":"builder","archetype":"trainer","pos":[2,2],"facing":"S","display_name":"Builder"}
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
	agg := manifest.NewAggregator("construction_test", "test")
	reg := syscore.NewConcreteRegistry(bus, agg)
	for _, s := range []syscore.System{inventory.New(), construction.New()} {
		reg.BeginSystem(s.Name())
		s.RegisterWith(reg)
		reg.EndSystem()
	}
	reg.InstallServicesInto(wa)
	for _, id := range wa.EntityIDs() {
		reg.RunOnEntitySpawn(wa, wa.EntityByID(id))
	}
	return wa, reg
}

// seedInventory drops `wood` woods and `stone` stones into the builder's
// inventory via direct mutation (Resources would mint these naturally).
func seedInventory(wa *world.WorldAdapter, woods, stones int) {
	wa.MutateEntity("builder", func(real syscore.Entity) {
		items := []string{}
		for i := 0; i < woods; i++ {
			items = append(items, fmt.Sprintf("wood_seed_%d", i))
		}
		for i := 0; i < stones; i++ {
			items = append(items, fmt.Sprintf("stone_seed_%d", i))
		}
		real.SetExtra("inventory", items)
	})
}

func handle(t *testing.T, reg *syscore.ConcreteRegistry, wa *world.WorldAdapter, verb, raw string) syscore.ActionResult {
	t.Helper()
	return reg.Handle(wa, wa.EntityByID("builder"), &syscore.ActionEnvelope{
		ActionID: "1", Verb: verb, Raw: []byte(raw),
	})
}

func TestPlaceBlueprintConsumesInitialMaterials(t *testing.T) {
	wa, reg := boot(t)
	seedInventory(wa, 5, 5)
	// Cottage: 2x wood + 1x stone initial.
	res := handle(t, reg, wa, "place_blueprint", `{"kind":"cottage","at":[2,3]}`)
	if !res.Accepted {
		t.Fatalf("place should accept: %s", res.Reason)
	}
	builder := wa.EntityByID("builder")
	v, _ := builder.GetExtra("inventory")
	inv, _ := v.([]string)
	// Started with 10; consumed 3 (2 wood + 1 stone).
	if len(inv) != 7 {
		t.Fatalf("expected 7 items left, got %d (%v)", len(inv), inv)
	}
}

func TestPlaceRejectsMissingMaterials(t *testing.T) {
	wa, reg := boot(t)
	seedInventory(wa, 1, 0) // only 1 wood, need 2 wood + 1 stone
	res := handle(t, reg, wa, "place_blueprint", `{"kind":"cottage","at":[2,3]}`)
	if res.Accepted {
		t.Fatal("should reject")
	}
	if res.Reason != "missing_materials" {
		t.Fatalf("reason=%s", res.Reason)
	}
}

func TestAdvanceCompletesAndSpawnsBuilding(t *testing.T) {
	wa, reg := boot(t)
	// Need 2 wood + 1 stone initial + 4 advance steps * (1 wood + 1 stone)
	// = 6 wood + 5 stone total. Give plenty.
	seedInventory(wa, 20, 20)
	res := handle(t, reg, wa, "place_blueprint", `{"kind":"cottage","at":[2,3]}`)
	if !res.Accepted {
		t.Fatalf("place failed: %s", res.Reason)
	}
	// Find the blueprint id by inspecting the world. Iterate entities,
	// pick the one with archetype="blueprint".
	bpID := ""
	for _, id := range wa.EntityIDs() {
		if e := wa.EntityByID(id); e != nil && e.Archetype() == "blueprint" {
			bpID = id
			break
		}
	}
	if bpID == "" {
		t.Fatal("no blueprint spawned")
	}
	// Advance 4 times.
	for i := 0; i < 4; i++ {
		r := handle(t, reg, wa, "advance_construction", `{"target":"`+bpID+`"}`)
		if !r.Accepted {
			t.Fatalf("advance %d: %s", i, r.Reason)
		}
	}
	// Blueprint should be gone; a building "bld_<bpID>" should exist.
	if wa.EntityByID(bpID) != nil {
		t.Fatal("blueprint should be removed after completion")
	}
	bldID := "bld_" + bpID
	bld := wa.EntityByID(bldID)
	if bld == nil {
		t.Fatalf("expected building %s", bldID)
	}
	if bld.Archetype() != "building" {
		t.Fatalf("archetype=%s", bld.Archetype())
	}
	owner, _ := bld.GetExtra("owner")
	if owner != "builder" {
		t.Fatalf("owner=%v", owner)
	}
}

func TestDemolishRequiresOwner(t *testing.T) {
	wa, reg := boot(t)
	seedInventory(wa, 5, 5)
	_ = handle(t, reg, wa, "place_blueprint", `{"kind":"cottage","at":[2,3]}`)
	var bpID string
	for _, id := range wa.EntityIDs() {
		if e := wa.EntityByID(id); e != nil && e.Archetype() == "blueprint" {
			bpID = id
			break
		}
	}
	// Builder demolishes own blueprint.
	r := handle(t, reg, wa, "demolish", `{"target":"`+bpID+`"}`)
	if !r.Accepted {
		t.Fatalf("demolish should succeed: %s", r.Reason)
	}
	if wa.EntityByID(bpID) != nil {
		t.Fatal("blueprint should be gone")
	}
}

func TestEventsEmitted(t *testing.T) {
	wa, reg := boot(t)
	seedInventory(wa, 20, 20)
	started, advanced, completed := 0, 0, 0
	wa.Bus.Subscribe("ConstructionStarted", func(_ eventbus.WorldCtx, _ eventbus.Event) { started++ })
	wa.Bus.Subscribe("ConstructionAdvanced", func(_ eventbus.WorldCtx, _ eventbus.Event) { advanced++ })
	wa.Bus.Subscribe("ConstructionCompleted", func(_ eventbus.WorldCtx, _ eventbus.Event) { completed++ })

	_ = handle(t, reg, wa, "place_blueprint", `{"kind":"shed","at":[2,3]}`)
	var bpID string
	for _, id := range wa.EntityIDs() {
		if e := wa.EntityByID(id); e != nil && e.Archetype() == "blueprint" {
			bpID = id
			break
		}
	}
	// Shed: 2 steps to complete.
	_ = handle(t, reg, wa, "advance_construction", `{"target":"`+bpID+`"}`)
	_ = handle(t, reg, wa, "advance_construction", `{"target":"`+bpID+`"}`)
	wa.Bus.Drain(wa)
	if started != 1 || advanced != 2 || completed != 1 {
		t.Fatalf("events: started=%d advanced=%d completed=%d", started, advanced, completed)
	}
}

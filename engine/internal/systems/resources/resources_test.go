package resources_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/core/spatial"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/resources"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const testWorldJSON = `{
  "map_id": "resources_test",
  "width_tiles": 6,
  "height_tiles": 4,
  "tiles_legend": {".":"grass"},
  "tiles": ["......","......","......","......"],
  "entities": [
    {"entity_id":"hero","archetype":"trainer","pos":[1,1],"facing":"S","display_name":"Hero"},
    {"entity_id":"oak","archetype":"tree","pos":[2,1],"facing":"S","display_name":"Oak"},
    {"entity_id":"granite","archetype":"rock","pos":[1,2],"facing":"S","display_name":"Granite"}
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
	agg := manifest.NewAggregator("resources_test", "test")
	reg := syscore.NewConcreteRegistry(bus, agg)
	reg.BeginSystem("resources")
	resources.New().RegisterWith(reg)
	reg.EndSystem()
	reg.InstallServicesInto(wa)
	for _, id := range wa.EntityIDs() {
		reg.RunOnEntitySpawn(wa, wa.EntityByID(id))
	}
	return wa, reg
}

func TestTreeSeed(t *testing.T) {
	wa, _ := boot(t)
	oak := wa.EntityByID("oak")
	h, _ := oak.GetExtra("hardness")
	if h != resources.DefaultTreeHardness {
		t.Fatalf("oak hardness=%v", h)
	}
}

func TestChopAddsWood(t *testing.T) {
	wa, reg := boot(t)
	res := reg.Handle(wa, wa.EntityByID("hero"), &syscore.ActionEnvelope{
		ActionID: "1", Verb: "chop",
		Raw: []byte(`{"target":"oak"}`),
	})
	if !res.Accepted {
		t.Fatalf("chop should accept: %s", res.Reason)
	}
	hero := wa.EntityByID("hero")
	v, _ := hero.GetExtra("inventory")
	inv, _ := v.([]string)
	if len(inv) != 1 {
		t.Fatalf("expected 1 item, got %v", inv)
	}
	if got := inv[0][:4]; got != "wood" {
		t.Fatalf("expected wood prefix, got %q", inv[0])
	}
}

func TestChopRejectsRock(t *testing.T) {
	wa, reg := boot(t)
	res := reg.Handle(wa, wa.EntityByID("hero"), &syscore.ActionEnvelope{
		ActionID: "1", Verb: "chop",
		Raw: []byte(`{"target":"granite"}`),
	})
	if res.Accepted {
		t.Fatal("chop should reject a rock")
	}
	if res.Reason != "not_a_tree" {
		t.Fatalf("reason=%s", res.Reason)
	}
}

func TestDepletionRemovesEntity(t *testing.T) {
	wa, reg := boot(t)
	for i := 0; i < resources.DefaultTreeHardness; i++ {
		_ = reg.Handle(wa, wa.EntityByID("hero"), &syscore.ActionEnvelope{
			ActionID: "1", Verb: "chop",
			Raw: []byte(`{"target":"oak"}`),
		})
	}
	if wa.EntityByID("oak") != nil {
		t.Fatal("oak should be removed after full depletion")
	}
}

func TestMineYieldsStone(t *testing.T) {
	wa, reg := boot(t)
	res := reg.Handle(wa, wa.EntityByID("hero"), &syscore.ActionEnvelope{
		ActionID: "1", Verb: "mine",
		Raw: []byte(`{"target":"granite"}`),
	})
	if !res.Accepted {
		t.Fatalf("mine should accept: %s", res.Reason)
	}
	hero := wa.EntityByID("hero")
	v, _ := hero.GetExtra("inventory")
	inv, _ := v.([]string)
	if len(inv) != 1 || inv[0][:5] != "stone" {
		t.Fatalf("expected stone item, got %v", inv)
	}
}

func TestDepletionEmitsEvent(t *testing.T) {
	wa, reg := boot(t)
	depleted := 0
	wa.Bus.Subscribe("ResourceDepleted", func(_ eventbus.WorldCtx, _ eventbus.Event) { depleted++ })
	for i := 0; i < resources.DefaultTreeHardness; i++ {
		_ = reg.Handle(wa, wa.EntityByID("hero"), &syscore.ActionEnvelope{
			ActionID: "1", Verb: "chop",
			Raw: []byte(`{"target":"oak"}`),
		})
	}
	wa.Bus.Drain(wa)
	if depleted != 1 {
		t.Fatalf("expected 1 ResourceDepleted, got %d", depleted)
	}
}

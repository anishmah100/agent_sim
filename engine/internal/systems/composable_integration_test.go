// Integration test: load Combat + Money + Inventory as composable
// systems against the real World via the adapter. Validates that the
// new architecture handles the same gameplay the old fantasy_town
// scenario did.

package systems_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/core/spatial"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/combat"
	"github.com/anishmah100/agent_sim/engine/internal/systems/inventory"
	"github.com/anishmah100/agent_sim/engine/internal/systems/money"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const testWorldJSON = `{
  "map_id": "integration_test",
  "width_tiles": 10,
  "height_tiles": 6,
  "tiles_legend": {".":"grass"},
  "tiles": ["..........","..........","..........","..........","..........",".........."],
  "entities": [
    {"entity_id":"hero","archetype":"trainer","pos":[2,2],"facing":"S","display_name":"Hero"},
    {"entity_id":"goblin","archetype":"goblin","pos":[3,2],"facing":"S","display_name":"Goblin"},
    {"entity_id":"merchant","archetype":"merchant","pos":[2,3],"facing":"S","display_name":"Merchant"}
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
	agg := manifest.NewAggregator("integration_test", "test_scenario")
	reg := syscore.NewConcreteRegistry(bus, agg)

	// Install three systems. Order matters for OnEntitySpawn ordering.
	systems := []syscore.System{combat.New(), money.New(), inventory.New()}
	for _, s := range systems {
		reg.BeginSystem(s.Name())
		s.RegisterWith(reg)
		reg.EndSystem()
	}
	reg.InstallServicesInto(wa)

	// Seed extras on existing entities (simulate engine-boot OnEntitySpawn).
	for _, id := range wa.EntityIDs() {
		reg.RunOnEntitySpawn(wa, wa.EntityByID(id))
	}
	return wa, reg
}

func TestComposableBoot(t *testing.T) {
	wa, reg := boot(t)
	if reg.VerbCount() < 8 {
		t.Fatalf("expected at least 8 verbs across 3 systems, got %d", reg.VerbCount())
	}
	hero := wa.EntityByID("hero")
	if _, ok := hero.GetExtra("hp"); !ok {
		t.Fatal("Combat OnEntitySpawn should have seeded hp")
	}
	if _, ok := hero.GetExtra("gold"); !ok {
		t.Fatal("Money OnEntitySpawn should have seeded gold")
	}
	if _, ok := hero.GetExtra("inventory"); !ok {
		t.Fatal("Inventory OnEntitySpawn should have seeded inventory")
	}
}

func TestComposable_AttackDamages(t *testing.T) {
	wa, reg := boot(t)
	res := reg.Handle(wa, wa.EntityByID("hero"), &syscore.ActionEnvelope{
		ActionID: "1", Verb: "attack",
		Raw: []byte(`{"target":"goblin"}`),
	})
	if !res.Accepted {
		t.Fatalf("attack should accept: %s", res.Reason)
	}
	hp, _ := wa.EntityByID("goblin").GetExtra("hp")
	if hp != combat.DefaultMaxHP-combat.DefaultAttackDamage {
		t.Fatalf("expected hp=%d got %v", combat.DefaultMaxHP-combat.DefaultAttackDamage, hp)
	}
}

func TestComposable_PayWithinRange(t *testing.T) {
	wa, reg := boot(t)
	// merchant at (2,3) is 1 tile from hero at (2,2).
	res := reg.Handle(wa, wa.EntityByID("hero"), &syscore.ActionEnvelope{
		ActionID: "1", Verb: "pay",
		Raw: []byte(`{"target":"merchant","amount":5}`),
	})
	if !res.Accepted {
		t.Fatalf("pay should accept: %s", res.Reason)
	}
	heroGold, _ := wa.EntityByID("hero").GetExtra("gold")
	merchGold, _ := wa.EntityByID("merchant").GetExtra("gold")
	if heroGold != money.DefaultStartingGold-5 {
		t.Fatalf("hero gold: %v", heroGold)
	}
	if merchGold != money.DefaultStartingGold+5 {
		t.Fatalf("merchant gold: %v", merchGold)
	}
}

func TestComposable_AttackEmitsEvents(t *testing.T) {
	wa, _ := boot(t)
	bus := wa.Bus

	deathCount := 0
	bus.Subscribe("EntityDied", func(w eventbus.WorldCtx, ev eventbus.Event) {
		deathCount++
	})

	svc := wa.GetService("combat").(combat.CombatService)
	// Lethal blow.
	svc.DealDamage(wa, "goblin", 999, "test", "hero")
	bus.Drain(wa)
	if deathCount != 1 {
		t.Fatalf("expected 1 EntityDied event, got %d", deathCount)
	}
}

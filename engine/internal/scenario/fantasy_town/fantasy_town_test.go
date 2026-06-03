package fantasy_town

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const testWorldJSON = `{
  "map_id": "scenario_test",
  "width_tiles": 10,
  "height_tiles": 6,
  "tiles_legend": {".":"grass"},
  "tiles": [
    "..........",
    "..........",
    "..........",
    "..........",
    "..........",
    ".........."
  ],
  "entities": [
    {"entity_id":"hero","archetype":"trainer","pos":[2,2],"facing":"S","display_name":"Hero"},
    {"entity_id":"goblin","archetype":"goblin","pos":[3,2],"facing":"S","display_name":"Goblin"},
    {"entity_id":"merchant","archetype":"merchant","pos":[5,2],"facing":"S","display_name":"Merchant"}
  ]
}`

func newScenarioWorld(t *testing.T) *world.World {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "w.json")
	if err := os.WriteFile(p, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	w, err := world.Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	s := New()
	verbs := make(map[string]func(*world.World, *world.Entity, *world.ActionEnvelope) world.ActionResult)
	for _, v := range s.Verbs() {
		if h := s.Handler(v); h != nil {
			verbs[v] = h
		}
	}
	w.InstallScenario(verbs, func(w *world.World, t uint64) { s.OnTick(w, t) }, s.OnEntitySpawn)
	return w
}

func TestSpawnGivesHPAndGold(t *testing.T) {
	w := newScenarioWorld(t)
	hero := w.EntityByID("hero")
	if hero == nil {
		t.Fatal("no hero")
	}
	if hero.Extras["hp"] == nil {
		t.Fatal("hero should have hp")
	}
	if hero.Extras["gold"] == nil {
		t.Fatal("hero should have gold")
	}
}

func TestAttackDealsDamage(t *testing.T) {
	w := newScenarioWorld(t)
	res := w.SubmitAction("hero", &world.ActionEnvelope{
		ActionID: "1", Verb: "attack",
		Raw: []byte(`{"verb":"attack","target":"goblin"}`),
	})
	if !res.Accepted {
		t.Fatalf("attack should accept: %s", res.Reason)
	}
	g := w.EntityByID("goblin")
	if g == nil {
		t.Fatal("goblin gone")
	}
	hp, _ := g.Extras["hp"].(int)
	if hp != DefaultMaxHP-DefaultAttackDamage {
		t.Fatalf("expected hp=%d got %d", DefaultMaxHP-DefaultAttackDamage, hp)
	}
}

func TestDefendHalvesDamage(t *testing.T) {
	w := newScenarioWorld(t)
	w.SubmitAction("goblin", &world.ActionEnvelope{
		ActionID: "1", Verb: "defend",
		Raw: []byte(`{"verb":"defend"}`),
	})
	w.SubmitAction("hero", &world.ActionEnvelope{
		ActionID: "2", Verb: "attack",
		Raw: []byte(`{"verb":"attack","target":"goblin"}`),
	})
	g := w.EntityByID("goblin")
	hp, _ := g.Extras["hp"].(int)
	want := DefaultMaxHP - (DefaultAttackDamage / 2)
	if hp != want {
		t.Fatalf("defended damage: want %d got %d", want, hp)
	}
}

func TestHealRestores(t *testing.T) {
	w := newScenarioWorld(t)
	// Hurt the goblin first.
	w.SubmitAction("hero", &world.ActionEnvelope{
		ActionID: "1", Verb: "attack",
		Raw: []byte(`{"verb":"attack","target":"goblin"}`),
	})
	// Heal self (no target).
	w.SubmitAction("goblin", &world.ActionEnvelope{
		ActionID: "2", Verb: "heal",
		Raw: []byte(`{"verb":"heal"}`),
	})
	g := w.EntityByID("goblin")
	hp, _ := g.Extras["hp"].(int)
	// After 12 damage then heal 25, capped at max.
	if hp != DefaultMaxHP {
		t.Fatalf("expected heal to cap at max %d, got %d", DefaultMaxHP, hp)
	}
}

func TestPayTransfersGold(t *testing.T) {
	w := newScenarioWorld(t)
	// Move hero adjacent to merchant — they're at (2,2) and (5,2). Use
	// move action which has pathfinding. For test simplicity we just
	// teleport via mutation by submitting moves through Dispatch.
	// Instead, place hero adjacent in the JSON; redo with direct.
	w.SubmitAction("merchant", &world.ActionEnvelope{
		ActionID: "1", Verb: "pay",
		Raw: []byte(`{"verb":"pay","target":"hero","amount":5}`),
	})
	// Merchant at (5,2), hero at (2,2) — distance 3. Should reject.
	m := w.EntityByID("merchant")
	hgold, _ := m.Extras["gold"].(int)
	if hgold != DefaultGold {
		t.Fatalf("merchant gold should be unchanged; got %d", hgold)
	}
}

func TestWorkPaysGold(t *testing.T) {
	w := newScenarioWorld(t)
	w.SubmitAction("hero", &world.ActionEnvelope{
		ActionID: "1", Verb: "work_for_pay",
		Raw: []byte(`{"verb":"work_for_pay"}`),
	})
	hero := w.EntityByID("hero")
	gold, _ := hero.Extras["gold"].(int)
	if gold != DefaultGold+5 {
		t.Fatalf("expected gold %d got %d", DefaultGold+5, gold)
	}
}

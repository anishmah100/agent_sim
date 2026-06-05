// Tests for the fantasy_town composable system set.
//
// These mirror the original scenario-as-monolith tests but route
// everything through the new SystemHost / ConcreteRegistry path —
// proving the composable systems handle the same gameplay through
// the live *World's SubmitAction surface.
package fantasy_town

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/systems/combat"
	"github.com/anishmah100/agent_sim/engine/internal/systems/money"
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
	Install(w)
	return w
}

// submit enqueues an action, drives one Tick to drain the queue, and
// returns the result. Phase A made SubmitAction blocking on a reply
// channel, so direct calls would deadlock without a tick goroutine —
// this helper is the test-side equivalent.
func submit(t *testing.T, w *world.World, entityID string, env *world.ActionEnvelope) world.ActionResult {
	t.Helper()
	ch := w.QueueAction(entityID, env)
	w.Tick()
	select {
	case res := <-ch:
		return res
	default:
		t.Fatalf("submit(%s, %s): Tick did not drain action", entityID, env.Verb)
		return world.ActionResult{}
	}
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
	res := submit(t, w, "hero", &world.ActionEnvelope{
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
	if hp != combat.DefaultMaxHP-combat.DefaultAttackDamage {
		t.Fatalf("expected hp=%d got %d", combat.DefaultMaxHP-combat.DefaultAttackDamage, hp)
	}
}

func TestDefendHalvesDamage(t *testing.T) {
	w := newScenarioWorld(t)
	submit(t, w, "goblin", &world.ActionEnvelope{
		ActionID: "1", Verb: "defend",
		Raw: []byte(`{"verb":"defend"}`),
	})
	submit(t, w, "hero", &world.ActionEnvelope{
		ActionID: "2", Verb: "attack",
		Raw: []byte(`{"verb":"attack","target":"goblin"}`),
	})
	g := w.EntityByID("goblin")
	hp, _ := g.Extras["hp"].(int)
	want := combat.DefaultMaxHP - (combat.DefaultAttackDamage / 2)
	if hp != want {
		t.Fatalf("defended damage: want %d got %d", want, hp)
	}
}

func TestHealRestores(t *testing.T) {
	w := newScenarioWorld(t)
	submit(t, w, "hero", &world.ActionEnvelope{
		ActionID: "1", Verb: "attack",
		Raw: []byte(`{"verb":"attack","target":"goblin"}`),
	})
	submit(t, w, "goblin", &world.ActionEnvelope{
		ActionID: "2", Verb: "heal",
		Raw: []byte(`{"verb":"heal"}`),
	})
	g := w.EntityByID("goblin")
	hp, _ := g.Extras["hp"].(int)
	if hp != combat.DefaultMaxHP {
		t.Fatalf("expected heal to cap at max %d, got %d", combat.DefaultMaxHP, hp)
	}
}

func TestPayRejectsOutOfRange(t *testing.T) {
	w := newScenarioWorld(t)
	// Merchant at (5,2), hero at (2,2) — distance 3. Should reject.
	res := submit(t, w, "merchant", &world.ActionEnvelope{
		ActionID: "1", Verb: "pay",
		Raw: []byte(`{"verb":"pay","target":"hero","amount":5}`),
	})
	if res.Accepted {
		t.Fatal("pay should reject across 3 tiles")
	}
	m := w.EntityByID("merchant")
	hgold, _ := m.Extras["gold"].(int)
	if hgold != money.DefaultStartingGold {
		t.Fatalf("merchant gold should be unchanged; got %d", hgold)
	}
}

func TestWorkPaysGold(t *testing.T) {
	w := newScenarioWorld(t)
	submit(t, w, "hero", &world.ActionEnvelope{
		ActionID: "1", Verb: "work_for_pay",
		Raw: []byte(`{"verb":"work_for_pay"}`),
	})
	hero := w.EntityByID("hero")
	gold, _ := hero.Extras["gold"].(int)
	if gold != money.DefaultStartingGold+money.WorkPayment {
		t.Fatalf("expected gold %d got %d", money.DefaultStartingGold+money.WorkPayment, gold)
	}
}

func TestLootRequiresDead(t *testing.T) {
	w := newScenarioWorld(t)
	// Place hero next to goblin (already at 2,2 and 3,2 — adjacent).
	res := submit(t, w, "hero", &world.ActionEnvelope{
		ActionID: "1", Verb: "loot",
		Raw: []byte(`{"verb":"loot","target":"goblin"}`),
	})
	if res.Accepted {
		t.Fatal("loot should reject a live target")
	}
	if res.Reason != "target_alive" {
		t.Fatalf("expected target_alive, got %s", res.Reason)
	}
}

func TestLootTransfersGold(t *testing.T) {
	w := newScenarioWorld(t)
	// Kill goblin via repeated attacks (or just drive HP to 0 via mutation
	// on the live world — simpler).
	w.MutateEntity("goblin", func(real *world.Entity) {
		real.Extras["hp"] = 0
	})
	res := submit(t, w, "hero", &world.ActionEnvelope{
		ActionID: "1", Verb: "loot",
		Raw: []byte(`{"verb":"loot","target":"goblin"}`),
	})
	if !res.Accepted {
		t.Fatalf("loot should accept on corpse: %s", res.Reason)
	}
	hero := w.EntityByID("hero")
	gob := w.EntityByID("goblin")
	heroGold, _ := hero.Extras["gold"].(int)
	gobGold, _ := gob.Extras["gold"].(int)
	if heroGold != money.DefaultStartingGold*2 {
		t.Fatalf("hero should have absorbed goblin gold; got %d", heroGold)
	}
	if gobGold != 0 {
		t.Fatalf("goblin gold should be 0; got %d", gobGold)
	}
}

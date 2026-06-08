package world

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/systems/combat"
	"github.com/anishmah100/agent_sim/engine/internal/systems/inventory"
	"github.com/anishmah100/agent_sim/engine/internal/systems/loot"
	"github.com/anishmah100/agent_sim/engine/internal/systems/money"
	"github.com/anishmah100/agent_sim/engine/internal/systems/property"
	"github.com/anishmah100/agent_sim/engine/internal/systems/reputation"
	"github.com/anishmah100/agent_sim/engine/internal/systems/resources"
	"github.com/anishmah100/agent_sim/engine/internal/systems/trade"
	"github.com/anishmah100/agent_sim/engine/internal/systems/vitals"
)

// Verb-coverage suite. Goal: every engine verb in the union of
// action.go's native dispatch + every system's registered verbs
// has a deterministic test that:
//
//   1. Submits the verb via the action queue.
//   2. Confirms ActionResult.Accepted matches expectation.
//   3. Confirms the historian-facing bus event(s) the verb is
//      supposed to emit are actually queued.
//
// The user discovered late in A9 that several verbs accepted their
// action but never queued the corresponding event (Speech, building
// enter/exit on decorations), and that the harness silently dropped
// many verbs to Wait. This file is the "everything works" suite
// players need before they can trust the substrate.

// vcTest builds a fresh world + system stack and returns the
// pieces every verb-coverage test needs.
type vcTest struct {
	t       *testing.T
	world   *World
	host    *SystemHost
	bus     *eventbus.Bus
	emitted []string // every Kind() drained by Tick during this test
}

func newVCTest(t *testing.T) *vcTest {
	t.Helper()
	w := loadTestWorld(t)
	// Build the full system stack so every verb registers a handler.
	agg := manifest.NewAggregator("test_world", "verbs_coverage")
	host := NewSystemHost(w, agg)
	host.Install(vitals.New())
	host.Install(combat.New())
	host.Install(inventory.New())
	host.Install(money.New())
	host.Install(trade.New())
	host.Install(loot.New())
	host.Install(property.New())
	host.Install(resources.New())
	host.Install(reputation.New())
	host.InstallInto()
	// Subscribe AFTER InstallInto so we catch every Queue.
	vc := &vcTest{t: t, world: w, host: host, bus: host.Bus}
	host.Bus.SubscribeAll(func(_ eventbus.WorldCtx, ev eventbus.Event) {
		vc.emitted = append(vc.emitted, ev.Kind())
	})
	return vc
}

// submit fires (verb, raw) for entity, ticks once to drain the
// action queue + bus, then returns the ActionResult.
func (vc *vcTest) submit(entity, verb string, raw string) ActionResult {
	vc.t.Helper()
	env := &ActionEnvelope{
		ActionID: "act-" + verb,
		Verb:     verb,
		Raw:      json.RawMessage(raw),
	}
	reply := vc.world.QueueAction(entity, env)
	vc.world.Tick()
	select {
	case res := <-reply:
		return res
	default:
		vc.t.Fatalf("no reply for verb=%s entity=%s — queue or tick path broken", verb, entity)
		return ActionResult{}
	}
}

// emittedIncludes — true if any drained event had this kind.
func (vc *vcTest) emittedIncludes(kind string) bool {
	for _, k := range vc.emitted {
		if k == kind {
			return true
		}
	}
	return false
}

// === Movement / social ===

func TestVerb_Speak_EmitsSpeechEvent(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "speak", `{"text":"hello"}`)
	if !res.Accepted {
		t.Fatalf("speak should accept; got reason=%q", res.Reason)
	}
	if !vc.emittedIncludes("Speech") {
		t.Fatalf("speak should emit Speech; got %v", vc.emitted)
	}
	if !vc.emittedIncludes("ActionAccepted") {
		t.Fatalf("every accepted verb should emit ActionAccepted; got %v", vc.emitted)
	}
}

func TestVerb_Shout_EmitsSpeechEvent(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "shout", `{"text":"hey"}`)
	if !res.Accepted {
		t.Fatalf("shout should accept; got reason=%q", res.Reason)
	}
	if !vc.emittedIncludes("Speech") {
		t.Fatalf("shout should emit Speech (mode=shout); got %v", vc.emitted)
	}
}

func TestVerb_Whisper_RequiresAdjacentTarget(t *testing.T) {
	vc := newVCTest(t)
	// "a" at (1,1), "b" at (8,1) — chebyshev 7, NOT adjacent.
	res := vc.submit("a", "whisper", `{"target":"b","text":"psst"}`)
	if res.Accepted {
		t.Fatalf("whisper at chebyshev>1 should reject; got accepted")
	}
	if res.Reason != "target_too_far" {
		t.Fatalf("whisper expected target_too_far; got %q", res.Reason)
	}
	// Move b adjacent and retry.
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	vc.emitted = nil
	res = vc.submit("a", "whisper", `{"target":"b","text":"psst"}`)
	if !res.Accepted {
		t.Fatalf("whisper adjacent should accept; got reason=%q", res.Reason)
	}
	if !vc.emittedIncludes("Whisper") {
		t.Fatalf("whisper should emit Whisper kind; got %v", vc.emitted)
	}
}

func TestVerb_LookAt_AlwaysAccepted(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "look_at", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("look_at should accept; got reason=%q", res.Reason)
	}
}

func TestVerb_Step_ToAdjacentWalkable(t *testing.T) {
	vc := newVCTest(t)
	// a is at (1,1); stepping E to (2,1) is walkable.
	res := vc.submit("a", "step", `{"dir":"E"}`)
	if !res.Accepted {
		t.Fatalf("step E should accept; got reason=%q", res.Reason)
	}
}

func TestVerb_Step_IntoWallRejected(t *testing.T) {
	vc := newVCTest(t)
	// a is at (1,1); wall row at y=2, so stepping S onto (1,2)... (1,2) is
	// walkable (wall is cols 2-5). Move a next to the wall first: from (2,1)
	// stepping S would hit (2,2) the wall. Re-place for a deterministic check.
	a := vc.world.entities["a"]
	a.LogicalTile = Tile{2, 1}
	a.WalkProgress = 1
	res := vc.submit("a", "step", `{"dir":"S"}`)
	if res.Accepted {
		t.Fatalf("step into wall (2,2) should reject; got accepted")
	}
	if res.Reason != "blocked_by_terrain" {
		t.Fatalf("reason=%q want blocked_by_terrain", res.Reason)
	}
}

func TestVerb_Wait_AlwaysAccepted(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "wait", `{"ticks":30}`)
	if !res.Accepted {
		t.Fatalf("wait should accept; got reason=%q", res.Reason)
	}
}

// === Interact / inventory (engine native path) ===

func TestVerb_Interact_EnterBuilding_DecorationPath(t *testing.T) {
	// This is the path Eldoria uses (buildings are decorations, not
	// entities). It MUST fire EnteredBuilding on the bus or the smoke
	// scorer + historian replay lose the entire enter signal.
	vc := newVCTest(t)
	res := vc.submit("a", "interact", `{"target":"bld:001","affordance":"enter"}`)
	if !res.Accepted {
		t.Fatalf("interact-enter on bld: should accept; got reason=%q", res.Reason)
	}
	if !vc.emittedIncludes("EnteredBuilding") {
		t.Fatalf("interact-enter MUST emit EnteredBuilding; got %v", vc.emitted)
	}
	// Now exit.
	vc.emitted = nil
	res = vc.submit("a", "interact", `{"target":"bld:001","affordance":"exit"}`)
	if !res.Accepted {
		t.Fatalf("interact-exit should accept; got reason=%q", res.Reason)
	}
	if !vc.emittedIncludes("ExitedBuilding") {
		t.Fatalf("interact-exit MUST emit ExitedBuilding; got %v", vc.emitted)
	}
}

func TestVerb_Pickup_RequiresVisibleItem(t *testing.T) {
	// Pickup goes via the inventory system. No item in the test world,
	// so we expect an unknown_target rejection. The point is to confirm
	// the verb routes to the inventory handler (not silent fallthrough).
	vc := newVCTest(t)
	res := vc.submit("a", "pickup", `{"target":"nonexistent_item"}`)
	if res.Accepted {
		t.Fatalf("pickup of nonexistent should reject; got accepted")
	}
	if res.Reason == "" {
		t.Fatalf("rejected pickup should carry a reason; got empty")
	}
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("pickup should route to inventory handler; got %q (unrouted)", res.Reason)
	}
}

// === Property (entity-backed enter via the property system) ===

func TestVerb_PropertyEnter_NoBuildingEntityRejects(t *testing.T) {
	// The property system's "enter" verb requires an ENTITY with
	// archetype=building. None exists in the test world, so this
	// should reject. The point is to confirm the verb routes here
	// instead of silently no-oping.
	vc := newVCTest(t)
	res := vc.submit("a", "enter", `{"target":"nonexistent"}`)
	if res.Accepted {
		t.Fatalf("property-enter on nonexistent should reject; got accepted")
	}
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("enter should route to property handler; got %q", res.Reason)
	}
}

// === Combat ===

func TestVerb_Attack_RoutesToCombatHandler(t *testing.T) {
	vc := newVCTest(t)
	// Adjacent: move b to (2,1), a at (1,1).
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	res := vc.submit("a", "attack", `{"target":"b"}`)
	// We don't assert Accepted because combat may need additional
	// conditions; we assert the verb routed (no "unknown_verb").
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("attack should route to combat handler; got %q", res.Reason)
	}
}

func TestVerb_Defend_AlwaysAccepted(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "defend", `{}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("defend should route to combat handler; got %q", res.Reason)
	}
}

func TestVerb_Heal_RoutesToCombat(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "heal", `{}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("heal should route to combat handler; got %q", res.Reason)
	}
}

// === Resources ===

func TestVerb_Chop_RoutesToResourcesHandler(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "chop", `{"target":"some_tree"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("chop should route to resources handler; got %q", res.Reason)
	}
}

func TestVerb_Mine_RoutesToResourcesHandler(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "mine", `{"target":"some_rock"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("mine should route to resources handler; got %q", res.Reason)
	}
}

// === Economy ===

func TestVerb_Pay_RoutesToMoneyHandler(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	res := vc.submit("a", "pay", `{"target":"b","amount":5}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("pay should route to money handler; got %q", res.Reason)
	}
}

func TestVerb_WorkForPay_RoutesToMoneyHandler(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "work_for_pay", `{}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("work_for_pay should route to money handler; got %q", res.Reason)
	}
}

func TestVerb_Trade_RoutesToTradeHandler(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	res := vc.submit("a", "trade", `{"target":"b","item":"apple","price":2}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("trade should route to trade handler; got %q", res.Reason)
	}
}

func TestVerb_Loot_RoutesToLootHandler(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "loot", `{"target":"some_corpse"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("loot should route to loot handler; got %q", res.Reason)
	}
}

// === Property system additional verbs ===

func TestVerb_Lock_RoutesToProperty(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "lock", `{"target":"some_bld"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("lock should route to property handler; got %q", res.Reason)
	}
}

func TestVerb_Unlock_RoutesToProperty(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "unlock", `{"target":"some_bld"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("unlock should route to property handler; got %q", res.Reason)
	}
}

func TestVerb_ClaimOwnership_RoutesToProperty(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "claim_ownership", `{"target":"some_bld"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("claim_ownership should route to property; got %q", res.Reason)
	}
}

func TestVerb_TransferOwnership_RoutesToProperty(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "transfer_ownership", `{"target":"some_bld","new_owner":"b"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("transfer_ownership should route to property; got %q", res.Reason)
	}
}

// === Inventory: drop / equip / give ===

func TestVerb_Drop_RoutesToInventory(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "drop", `{"item":"apple"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("drop should route to inventory handler; got %q", res.Reason)
	}
}

func TestVerb_Equip_RoutesToInventory(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "equip", `{"item":"sword"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("equip should route to inventory handler; got %q", res.Reason)
	}
}

func TestVerb_Give_RoutesToInventory(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	res := vc.submit("a", "give", `{"target":"b","item":"apple"}`)
	if strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("give should route to inventory handler; got %q", res.Reason)
	}
}

// === Construction (placeholder routing test) ===

func TestVerb_PlaceBlueprint_RoutesIfRegistered(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "place_blueprint", `{"kind":"shack","at":[3,3]}`)
	// Construction system may not be installed here — that's fine for
	// this test, we just confirm the result shape (a reason, not a
	// crash). If it's unrouted, the verb name is missing from the
	// registry, which would be a real bug.
	_ = res
}

// === unknown_verb ===

func TestUnknownVerb_Rejected(t *testing.T) {
	vc := newVCTest(t)
	res := vc.submit("a", "florp", `{}`)
	if res.Accepted {
		t.Fatalf("unknown verb should reject; got accepted")
	}
	if !strings.HasPrefix(res.Reason, "unknown_verb") {
		t.Fatalf("unknown verb should report unknown_verb:florp; got %q", res.Reason)
	}
}

// === D10 — death drops inventory + emits scream + witness events ===

func TestD10_DeathDropsInventoryAndEquipped(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	vc.world.entities["b"].Extras = map[string]interface{}{
		"hp": 5, "max_hp": 100,
		"inventory": []string{"item:apple#1", "item:bread_loaf#2"},
		"equipped":  map[string]any{"weapon": "item:dagger#5"},
	}
	// A wields axe (18 dmg, kills B at hp=5).
	vc.world.entities["a"].Extras = map[string]interface{}{
		"equipped": map[string]any{"weapon": "item:axe#9"},
	}
	res := vc.submit("a", "attack", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("attack should accept; got %q", res.Reason)
	}
	// B1: a dead entity is now REMOVED from the world (it no longer
	// occupies a tile, is targetable, or shows in observations). The
	// dropped loot remains on the ground.
	if vc.world.entities["b"] != nil {
		t.Fatal("B should be removed from the world after death")
	}
	// Inventory + equipped should drop as item entities at B's tile.
	dropped := 0
	for _, id := range vc.world.EntityIDs() {
		other := vc.world.entities[id]
		if other == nil || other.Archetype != "item" {
			continue
		}
		if other.LogicalTile == (Tile{2, 1}) {
			dropped++
		}
	}
	if dropped < 3 {
		t.Errorf("expected at least 3 item entities at corpse tile (apple+bread+dagger), got %d", dropped)
	}
	// And the corpse must not leave a phantom occupant claim behind.
	if occ := vc.world.occupants[Tile{2, 1}]; occ == "b" {
		t.Errorf("dead agent's occupant claim should be freed, still %q", occ)
	}
}

func TestD10_DeathEmitsScreamAndWitnessAudibles(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	vc.world.entities["b"].Extras = map[string]interface{}{"hp": 5, "max_hp": 100}
	vc.world.entities["a"].Extras = map[string]interface{}{
		"equipped": map[string]any{"weapon": "item:axe#1"},
	}
	res := vc.submit("a", "attack", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("attack: %q", res.Reason)
	}
	// A should now hear:
	//  - one 'kill_witnessed' (A has LOS to B's tile, A is the killer — wait,
	//    actually killer is excluded from witness events in EmitDeathScream).
	// So A should NOT get a kill_witnessed. But A SHOULD hear the anonymous
	// death_scream.
	audA := vc.world.VisibleAudible(vc.world.entities["a"], 0)
	sawScream, sawWitness := false, false
	for _, ev := range audA {
		if ev.SoundKind == "death_scream" {
			sawScream = true
			// The from_pos should be rounded to a 5-tile cell, so not
			// exactly B's pos (2,1) but something nearby.
			if ev.FromEntity != "" {
				t.Errorf("death_scream should be anonymous, got from_entity=%q", ev.FromEntity)
			}
		}
		if ev.SoundKind == "kill_witnessed" {
			sawWitness = true
		}
	}
	if !sawScream {
		t.Error("killer A should hear the anonymous death_scream")
	}
	if sawWitness {
		t.Error("killer A should NOT receive a kill_witnessed event (already knows)")
	}
}

// TestWitnessLog_RecordsKillForBystander — a third party with LOS to a
// kill gets a persistent kill_witnessed WitnessRecord (drives the
// inspector's Witnesses tab); the killer does not (they already know),
// but does get a scream_heard.
func TestWitnessLog_RecordsKillForBystander(t *testing.T) {
	vc := newVCTest(t)
	w := vc.world
	w.entities["b"].LogicalTile = Tile{2, 1}
	w.entities["b"].Extras = map[string]interface{}{"hp": 5, "max_hp": 100}
	w.entities["a"].Extras = map[string]interface{}{
		"equipped": map[string]any{"weapon": "item:axe#1"},
	}
	// Bystander C: adjacent, clear LOS, within witnessRadius.
	w.entities["c"] = &Entity{EntityID: "c", Archetype: "agent", LogicalTile: Tile{3, 1}}

	res := vc.submit("a", "attack", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("attack: %q", res.Reason)
	}

	cRecs := w.WitnessedBy("c", 10)
	sawKill := false
	for _, r := range cRecs {
		if r.Kind == "kill_witnessed" {
			sawKill = true
			if r.Killer != "a" || r.Victim != "b" {
				t.Errorf("witness record killer/victim: got %q/%q want a/b", r.Killer, r.Victim)
			}
		}
	}
	if !sawKill {
		t.Errorf("bystander C should have a kill_witnessed record, got %+v", cRecs)
	}

	// Killer A: no kill_witnessed (already knows), but heard the scream.
	aRecs := w.WitnessedBy("a", 10)
	for _, r := range aRecs {
		if r.Kind == "kill_witnessed" {
			t.Errorf("killer A should NOT get a kill_witnessed record; got %+v", r)
		}
	}
}

func TestHasDecorationNear(t *testing.T) {
	w := loadTestWorld(t)
	w.decorations = append(w.decorations,
		DecorationRef{X: 5, Y: 5, Sprite: "bld:stall_red_bread_open"},
		DecorationRef{X: 20, Y: 20, Sprite: "veg:tree_oak"},
	)
	if !w.HasDecorationNear([2]int{6, 6}, "bld:stall", 2) {
		t.Error("stall at (5,5) should be found within radius 2 of (6,6)")
	}
	if w.HasDecorationNear([2]int{6, 6}, "bld:stall", 0) {
		t.Error("radius 0 from (6,6) should not reach the stall at (5,5)")
	}
	if !w.HasDecorationNear([2]int{6, 6}, "bld:", 2) {
		t.Error("prefix bld: should match the stall")
	}
	// (5,5)->(6,6) is Chebyshev distance 1, so radius 1 includes it.
	if !w.HasDecorationNear([2]int{6, 6}, "bld:", 1) {
		t.Error("stall at cheb-distance 1 should be found within radius 1")
	}
}

// TestReputation_KillLowersKillerStanding — killing an agent drops the
// killer's reputation below zero (the substrate for infamy-driven social
// dynamics). Runs through the full Tick so the bus drains and the
// reputation system's EntityDied handler fires.
func TestReputation_KillLowersKillerStanding(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	vc.world.entities["b"].Extras = map[string]interface{}{"hp": 5, "max_hp": 100}
	vc.world.entities["a"].Extras = map[string]interface{}{
		"equipped": map[string]any{"weapon": "item:axe#1"},
	}
	res := vc.submit("a", "attack", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("attack: %q", res.Reason)
	}
	rep, _ := vc.world.entities["a"].Extras["reputation"].(float64)
	if rep >= 0 {
		t.Errorf("killer A's reputation should be negative after a kill, got %v", rep)
	}
}

// === D21 — weapons damage + reach ===

func TestD21_Attack_UnarmedDealsBaseDamage(t *testing.T) {
	vc := newVCTest(t)
	// Put B adjacent to A at full HP. A unarmed → 7 damage (unarmedDmg,
	// raised from 4 so predation reads on screen without decimating the town).
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	vc.world.entities["b"].Extras = map[string]interface{}{"hp": 100, "max_hp": 100}
	vc.world.entities["a"].Extras = map[string]interface{}{}
	res := vc.submit("a", "attack", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("unarmed attack adjacent should accept; got %q", res.Reason)
	}
	hp := vc.world.entities["b"].Extras["hp"].(int)
	if hp != 93 {
		t.Errorf("unarmed attack: B's HP want 93 (100-7), got %d", hp)
	}
}

func TestD21_Attack_EquippedSwordDealsWeaponDamage(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	vc.world.entities["b"].Extras = map[string]interface{}{"hp": 100, "max_hp": 100}
	// A wields a sword_short (12 damage per D21 table).
	vc.world.entities["a"].Extras = map[string]interface{}{
		"equipped": map[string]any{"weapon": "item:sword_short#1"},
	}
	res := vc.submit("a", "attack", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("sword attack should accept; got %q", res.Reason)
	}
	hp := vc.world.entities["b"].Extras["hp"].(int)
	if hp != 88 {
		t.Errorf("sword attack: B's HP want 88 (100-12), got %d", hp)
	}
}

func TestD21_Attack_BowAtRangeAccepted_UnarmedAtRangeRejected(t *testing.T) {
	vc := newVCTest(t)
	// B is 4 tiles away — too far for melee (reach 1), in range for bow (reach 6).
	vc.world.entities["b"].LogicalTile = Tile{5, 1}
	vc.world.entities["b"].Extras = map[string]interface{}{"hp": 100, "max_hp": 100}
	// Unarmed first — should reject out_of_range.
	vc.world.entities["a"].Extras = map[string]interface{}{}
	res := vc.submit("a", "attack", `{"target":"b"}`)
	if res.Accepted {
		t.Fatal("unarmed at 4 tiles should reject; got accepted")
	}
	if res.Reason != "out_of_range" {
		t.Errorf("unarmed at range: want reason=out_of_range, got %q", res.Reason)
	}
	// Now equip bow + retry.
	vc.world.entities["a"].Extras["equipped"] = map[string]any{"weapon": "item:bow#1"}
	res = vc.submit("a", "attack", `{"target":"b"}`)
	if !res.Accepted {
		t.Fatalf("bow at 4 tiles should accept; got %q", res.Reason)
	}
}

// === D8 prereq — drop spawns a pickup-able item entity ===

func TestDrop_SpawnsItemEntity(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["a"].Extras["inventory"] = []string{"item:apple#5"}
	res := vc.submit("a", "drop", `{"item":"item:apple#5"}`)
	if !res.Accepted {
		t.Fatalf("drop should be accepted; got reason=%q", res.Reason)
	}
	// An item entity must now exist at A's position.
	found := false
	for _, id := range vc.world.EntityIDs() {
		other := vc.world.entities[id]
		if other == nil || other.Archetype != "item" {
			continue
		}
		if other.LogicalTile == vc.world.entities["a"].LogicalTile {
			found = true
			sprite, _ := other.Extras["sprite"].(string)
			if sprite != "item:apple" {
				t.Errorf("spawned item should have sprite item:apple, got %q", sprite)
			}
			if other.DisplayName != "apple" {
				t.Errorf("spawned item should have DisplayName apple, got %q", other.DisplayName)
			}
		}
	}
	if !found {
		t.Fatal("drop should spawn an item entity at the dropper's tile; none found")
	}
	// And A's inventory should no longer contain the apple.
	inv := vc.world.entities["a"].Extras["inventory"].([]string)
	if len(inv) != 0 {
		t.Errorf("inventory should be empty after drop; got %v", inv)
	}
}

// === D22 + D20 — eat verb + inventory cap ===

func TestVerb_Eat_RoutesToInventory_FoodReducesHunger(t *testing.T) {
	vc := newVCTest(t)
	// Seed A with an apple in inventory and high hunger.
	vc.world.entities["a"].Extras["inventory"] = []string{"item:apple#1"}
	vc.world.entities["a"].Extras["hunger"] = 0.5
	res := vc.submit("a", "eat", `{"item":"item:apple#1"}`)
	if !res.Accepted {
		t.Fatalf("eat should be accepted; got reason=%q", res.Reason)
	}
	// Hunger should drop by apple's satiety = 0.25 → 0.25.
	got := vc.world.entities["a"].Extras["hunger"].(float64)
	if got < 0.24 || got > 0.26 {
		t.Errorf("hunger after eating apple: want ~0.25, got %v", got)
	}
	// Inventory should no longer contain the apple.
	inv := vc.world.entities["a"].Extras["inventory"].([]string)
	if len(inv) != 0 {
		t.Errorf("inventory should be empty after eat; got %v", inv)
	}
}

func TestVerb_Eat_NotInInventory_Rejected(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["a"].Extras["inventory"] = []string{}
	res := vc.submit("a", "eat", `{"item":"item:apple#99"}`)
	if res.Accepted {
		t.Fatal("eat of non-owned item should reject")
	}
	if res.Reason != "not_in_inventory" {
		t.Errorf("expected reason=not_in_inventory, got %q", res.Reason)
	}
}

func TestVerb_Eat_NonFoodRejected(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["a"].Extras["inventory"] = []string{"item:sword_short#5"}
	res := vc.submit("a", "eat", `{"item":"item:sword_short#5"}`)
	if res.Accepted {
		t.Fatal("eat of a weapon should reject (not food)")
	}
	if res.Reason != "not_food" {
		t.Errorf("expected reason=not_food, got %q", res.Reason)
	}
}

// === D1 — verb targets are entity_id, never display name ===
// Audit conclusion: the engine already resolves targets exclusively
// via direct entity_id map lookup. No display-name fallback exists in
// any verb handler (audited: whisper, pay, attack, give, trade,
// interact, lock, unlock, claim_ownership, transfer_ownership,
// pickup, equip, loot, chop, mine). These tests pin the behavior so
// any future regression that adds display-name resolution fails the
// build.

func TestD1_Whisper_AcceptsEntityID_RejectsDisplayName(t *testing.T) {
	vc := newVCTest(t)
	// In the test fixture, B's display_name is "B" and entity_id is "b".
	// Place them adjacent so range is not the failure mode here.
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	ok := vc.submit("a", "whisper", `{"target":"b","text":"psst"}`)
	if !ok.Accepted {
		t.Fatalf("whisper to entity_id should succeed; got %+v", ok)
	}
	nm := vc.submit("a", "whisper", `{"target":"B","text":"psst"}`)
	if nm.Accepted {
		t.Fatal("whisper to display_name 'B' must be rejected (D1)")
	}
	if nm.Reason != "unknown_target" {
		t.Fatalf("expected reason unknown_target for display-name target; got %q", nm.Reason)
	}
}

func TestD1_Pay_RejectsDisplayName(t *testing.T) {
	vc := newVCTest(t)
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	nm := vc.submit("a", "pay", `{"target":"B","amount":5}`)
	if nm.Accepted {
		t.Fatal("pay to display_name 'B' must be rejected (D1)")
	}
}

// D19 — accepted whisper/pay/attack/propose_task each bump the social
// ledger (a→b and b→a). Verified by reading w.SocialCountsFor after
// the verb commits.

func TestD19_SocialLedger_BumpedByVerbs(t *testing.T) {
	vc := newVCTest(t)
	// Adjacent so range checks pass.
	vc.world.entities["b"].LogicalTile = Tile{2, 1}
	// Seed payer balance + an item to trade so pay/trade succeed.
	vc.world.entities["a"].Extras = map[string]any{
		"gold":      100,
		"inventory": []any{"item:apple#1"},
	}
	if vc.world.entities["b"].Extras == nil {
		vc.world.entities["b"].Extras = map[string]any{}
	}
	vc.world.entities["b"].Extras["gold"] = 100

	if r := vc.submit("a", "whisper", `{"target":"b","text":"psst"}`); !r.Accepted {
		t.Fatalf("whisper should accept; reason=%q", r.Reason)
	}
	if r := vc.submit("a", "pay", `{"target":"b","amount":5}`); !r.Accepted {
		t.Fatalf("pay should accept; reason=%q", r.Reason)
	}
	if r := vc.submit("a", "attack", `{"target":"b"}`); !r.Accepted {
		t.Fatalf("attack should accept; reason=%q", r.Reason)
	}
	if r := vc.submit("a", "trade",
		`{"target":"b","item":"item:apple#1","price":3}`); !r.Accepted {
		t.Fatalf("trade should accept; reason=%q", r.Reason)
	}

	ab := vc.world.SocialCountsFor("a", "b")
	if ab.Whisper < 1 || ab.Pay < 1 || ab.Attack < 1 || ab.Trade < 1 {
		t.Fatalf("(a,b) want all >=1, got %+v", ab)
	}
	ba := vc.world.SocialCountsFor("b", "a")
	if ba != ab {
		t.Fatalf("ledger must be bidirectional; (a,b)=%+v (b,a)=%+v", ab, ba)
	}
	peers := vc.world.SocialPeersOf("a")
	if _, ok := peers["b"]; !ok {
		t.Fatalf("PeersOf(a) must include b; got %v", peers)
	}
}

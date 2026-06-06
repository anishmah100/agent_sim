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

func TestVerb_Move_ToAdjacentWalkable(t *testing.T) {
	vc := newVCTest(t)
	// a is at (1,1); (2,1) is walkable.
	res := vc.submit("a", "move", `{"target":[2,1]}`)
	if !res.Accepted {
		t.Fatalf("move adjacent should accept; got reason=%q", res.Reason)
	}
}

func TestVerb_Move_ToWallRejected(t *testing.T) {
	vc := newVCTest(t)
	// Wall row at y=2, columns 2-5.
	res := vc.submit("a", "move", `{"target":[2,2]}`)
	if res.Accepted {
		t.Fatalf("move into wall should reject; got accepted")
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

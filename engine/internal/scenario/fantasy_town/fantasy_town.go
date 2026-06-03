// Fantasy town scenario.
//
// Rules layer on top of the engine:
//   - HP: every agent has hp + max_hp in extras.
//   - Combat: attack (adjacent melee), defend (next hit halved), heal.
//   - Money: every agent has gold in extras.
//   - Inventory: list of item IDs in extras.inventory.
//   - Items: items in the world are entities of archetype="item".
//   - Verbs: trade, pay, work_for_pay, loot — add gold/items dynamics.
package fantasy_town

import (
	"encoding/json"
	"github.com/anishmah100/agent_sim/engine/internal/scenario"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const (
	DefaultMaxHP   = 100
	DefaultGold    = 10
	DefaultAttackDamage = 12
	DefaultHealAmount   = 25
)

type FantasyTown struct{}

func New() *FantasyTown { return &FantasyTown{} }

func (s *FantasyTown) Name() string { return "fantasy_town" }

func (s *FantasyTown) Verbs() []string {
	return []string{
		// engine-defined but scenario-handled:
		"attack", "defend", "heal",
		"pickup", "drop", "equip", "give",
		// scenario-custom:
		"trade", "pay", "loot", "work_for_pay",
	}
}

func (s *FantasyTown) OnEntitySpawn(e *world.Entity) {
	if e.Extras == nil {
		e.Extras = map[string]any{}
	}
	if e.Archetype == "item" {
		// Items don't get HP / gold.
		return
	}
	if _, ok := e.Extras["hp"]; !ok {
		e.Extras["hp"] = DefaultMaxHP
		e.Extras["max_hp"] = DefaultMaxHP
	}
	if _, ok := e.Extras["gold"]; !ok {
		e.Extras["gold"] = DefaultGold
	}
	if _, ok := e.Extras["inventory"]; !ok {
		e.Extras["inventory"] = []string{}
	}
	if _, ok := e.Extras["defending"]; !ok {
		e.Extras["defending"] = false
	}
}

func (s *FantasyTown) OnTick(w *world.World, tick uint64) {
	// Slow HP regen: +1 every 5 sec when not in combat.
	if tick%300 != 0 {
		return
	}
	for _, id := range w.EntityIDsUnlocked() {
		e := w.EntityByIDUnlocked(id)
		if e == nil || e.Archetype == "item" {
			continue
		}
		hp := extrasInt(e.Extras, "hp")
		maxHP := extrasInt(e.Extras, "max_hp")
		if hp > 0 && hp < maxHP {
			w.MutateEntity(id, func(real *world.Entity) {
				real.Extras["hp"] = hp + 1
			})
		}
	}
}

func (s *FantasyTown) Handler(verb string) scenario.VerbHandler {
	switch verb {
	case "attack":
		return handleAttack
	case "defend":
		return handleDefend
	case "heal":
		return handleHeal
	case "pickup":
		return handlePickup
	case "drop":
		return handleDrop
	case "give":
		return handleGive
	case "equip":
		return handleEquip
	case "trade":
		return handleTrade
	case "pay":
		return handlePay
	case "loot":
		return handleLoot
	case "work_for_pay":
		return handleWork
	}
	return nil
}

func handleAttack(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct{ Target string `json:"target"` }
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	other := w.EntityByIDUnlocked(p.Target)
	if other == nil {
		res.Reason = "unknown_target"
		return res
	}
	// Adjacent melee only — Chebyshev distance ≤ 1.
	if chebyshev(e.LogicalTile, other.LogicalTile) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	dmg := DefaultAttackDamage
	if defending, _ := other.Extras["defending"].(bool); defending {
		dmg /= 2
	}
	newHP := extrasInt(other.Extras, "hp") - dmg
	if newHP < 0 {
		newHP = 0
	}
	w.MutateEntity(other.EntityID, func(real *world.Entity) {
		real.Extras["hp"] = newHP
		real.Extras["defending"] = false
	})
	res.Accepted = true
	return res
}

func handleDefend(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		real.Extras["defending"] = true
	})
	return world.ActionResult{ActionID: env.ActionID, Verb: env.Verb, Accepted: true}
}

func handleHeal(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct{ Target string `json:"target"` }
	_ = json.Unmarshal(env.Raw, &p)
	tid := p.Target
	if tid == "" {
		tid = e.EntityID
	}
	target := w.EntityByIDUnlocked(tid)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if target.EntityID != e.EntityID && chebyshev(e.LogicalTile, target.LogicalTile) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	hp := extrasInt(target.Extras, "hp")
	maxHP := extrasInt(target.Extras, "max_hp")
	newHP := hp + DefaultHealAmount
	if newHP > maxHP {
		newHP = maxHP
	}
	w.MutateEntity(target.EntityID, func(real *world.Entity) {
		real.Extras["hp"] = newHP
	})
	res.Accepted = true
	return res
}

func handlePickup(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct{ Target string `json:"target"` }
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	item := w.EntityByIDUnlocked(p.Target)
	if item == nil || item.Archetype != "item" {
		res.Reason = "not_an_item"
		return res
	}
	if chebyshev(e.LogicalTile, item.LogicalTile) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		inv := extrasStringSlice(real.Extras, "inventory")
		inv = append(inv, p.Target)
		real.Extras["inventory"] = inv
	})
	w.RemoveEntity(p.Target)
	res.Accepted = true
	return res
}

func handleDrop(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct{ Item string `json:"item"` }
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	inv := extrasStringSlice(e.Extras, "inventory")
	idx := -1
	for i, id := range inv {
		if id == p.Item {
			idx = i
			break
		}
	}
	if idx < 0 {
		res.Reason = "not_in_inventory"
		return res
	}
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		cur := extrasStringSlice(real.Extras, "inventory")
		out := make([]string, 0, len(cur)-1)
		for i, id := range cur {
			if i != idx {
				out = append(out, id)
			}
		}
		real.Extras["inventory"] = out
	})
	// Spawn back into the world at the entity's tile.
	w.SpawnEntity(&world.Entity{
		EntityID:    p.Item,
		Archetype:   "item",
		LogicalTile: e.LogicalTile,
		Extras:      map[string]any{},
	})
	res.Accepted = true
	return res
}

func handleGive(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
		Item   string `json:"item"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	target := w.EntityByIDUnlocked(p.Target)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if chebyshev(e.LogicalTile, target.LogicalTile) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	myInv := extrasStringSlice(e.Extras, "inventory")
	idx := -1
	for i, id := range myInv {
		if id == p.Item {
			idx = i
			break
		}
	}
	if idx < 0 {
		res.Reason = "not_in_inventory"
		return res
	}
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		cur := extrasStringSlice(real.Extras, "inventory")
		out := make([]string, 0, len(cur)-1)
		for i, id := range cur {
			if i != idx {
				out = append(out, id)
			}
		}
		real.Extras["inventory"] = out
	})
	w.MutateEntity(p.Target, func(real *world.Entity) {
		inv := extrasStringSlice(real.Extras, "inventory")
		inv = append(inv, p.Item)
		real.Extras["inventory"] = inv
	})
	res.Accepted = true
	return res
}

func handleEquip(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Item string `json:"item"`
		Slot string `json:"slot"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	inv := extrasStringSlice(e.Extras, "inventory")
	found := false
	for _, id := range inv {
		if id == p.Item {
			found = true
			break
		}
	}
	if !found {
		res.Reason = "not_in_inventory"
		return res
	}
	slot := p.Slot
	if slot == "" {
		slot = "hand"
	}
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		equipped, _ := real.Extras["equipped"].(map[string]any)
		if equipped == nil {
			equipped = map[string]any{}
		}
		equipped[slot] = p.Item
		real.Extras["equipped"] = equipped
	})
	res.Accepted = true
	return res
}

func handlePay(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
		Amount int    `json:"amount"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil || p.Amount <= 0 {
		res.Reason = "bad_params"
		return res
	}
	target := w.EntityByIDUnlocked(p.Target)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if chebyshev(e.LogicalTile, target.LogicalTile) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	gold := extrasInt(e.Extras, "gold")
	if gold < p.Amount {
		res.Reason = "insufficient_gold"
		return res
	}
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		real.Extras["gold"] = extrasInt(real.Extras, "gold") - p.Amount
	})
	w.MutateEntity(p.Target, func(real *world.Entity) {
		real.Extras["gold"] = extrasInt(real.Extras, "gold") + p.Amount
	})
	res.Accepted = true
	return res
}

func handleTrade(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	// Trade is a paired pay+give. v1 treats it as: I give the target
	// my item; the target pays me their offer.
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
		Item   string `json:"item"`
		Price  int    `json:"price"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	target := w.EntityByIDUnlocked(p.Target)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if chebyshev(e.LogicalTile, target.LogicalTile) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	tgold := extrasInt(target.Extras, "gold")
	if tgold < p.Price {
		res.Reason = "target_insufficient_gold"
		return res
	}
	// Transfer item e → target.
	inv := extrasStringSlice(e.Extras, "inventory")
	idx := -1
	for i, id := range inv {
		if id == p.Item {
			idx = i
			break
		}
	}
	if idx < 0 {
		res.Reason = "not_in_inventory"
		return res
	}
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		cur := extrasStringSlice(real.Extras, "inventory")
		out := make([]string, 0, len(cur)-1)
		for i, id := range cur {
			if i != idx {
				out = append(out, id)
			}
		}
		real.Extras["inventory"] = out
		real.Extras["gold"] = extrasInt(real.Extras, "gold") + p.Price
	})
	w.MutateEntity(p.Target, func(real *world.Entity) {
		inv := extrasStringSlice(real.Extras, "inventory")
		inv = append(inv, p.Item)
		real.Extras["inventory"] = inv
		real.Extras["gold"] = extrasInt(real.Extras, "gold") - p.Price
	})
	res.Accepted = true
	return res
}

func handleLoot(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	// Loot = take from a target with HP=0 (dead). Empties their gold +
	// inventory into the looter.
	res := world.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct{ Target string `json:"target"` }
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	target := w.EntityByIDUnlocked(p.Target)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if chebyshev(e.LogicalTile, target.LogicalTile) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	if extrasInt(target.Extras, "hp") > 0 {
		res.Reason = "target_alive"
		return res
	}
	w.MutateEntity(p.Target, func(real *world.Entity) {
		looterGold := extrasInt(real.Extras, "gold")
		real.Extras["gold"] = 0
		real.Extras["inventory"] = []string{}
		// Lock in the transfer.
		w.MutateEntity(e.EntityID, func(me *world.Entity) {
			me.Extras["gold"] = extrasInt(me.Extras, "gold") + looterGold
		})
	})
	res.Accepted = true
	return res
}

func handleWork(w *world.World, e *world.Entity, env *world.ActionEnvelope) world.ActionResult {
	// Stub: the entity gets 5 gold for clocking in. Real version
	// requires standing on a work_site object.
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		real.Extras["gold"] = extrasInt(real.Extras, "gold") + 5
	})
	return world.ActionResult{ActionID: env.ActionID, Verb: env.Verb, Accepted: true}
}

// === helpers ===

func chebyshev(a, b [2]int) int {
	dx := a[0] - b[0]
	if dx < 0 {
		dx = -dx
	}
	dy := a[1] - b[1]
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

func extrasInt(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func extrasStringSlice(m map[string]any, k string) []string {
	switch v := m[k].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

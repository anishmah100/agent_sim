// Package inventory — composable Inventory system.
//
// Verbs: pickup / drop / equip / give.
// State: extras.inventory ([]string of item IDs), extras.equipped (map[slot]item_id).
// Service: InventoryService for cross-system access (Construction consumes; Loot fills).
package inventory

import (
	"encoding/json"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const DefaultMaxSlots = 20

// ItemPicked / ItemDropped / ItemTransferred events.
type ItemPicked struct{ Picker, Item string }

func (ItemPicked) Kind() string { return "ItemPicked" }

type ItemDropped struct{ Dropper, Item string; At [2]int }

func (ItemDropped) Kind() string { return "ItemDropped" }

type ItemTransferred struct{ From, To, Item string }

func (ItemTransferred) Kind() string { return "ItemTransferred" }

var (
	_ eventbus.Event = ItemPicked{}
	_ eventbus.Event = ItemDropped{}
	_ eventbus.Event = ItemTransferred{}
)

type InventoryService interface {
	Has(world syscore.World, entityID, itemID string) bool
	HasAll(world syscore.World, entityID string, itemIDs []string) bool
	Consume(world syscore.World, entityID string, itemIDs []string) (ok bool, reason string)
	Items(world syscore.World, entityID string) []string
}

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "inventory" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("pickup", s.handlePickup)
	r.Verb("drop", s.handleDrop)
	r.Verb("equip", s.handleEquip)
	r.Verb("give", s.handleGive)
	r.OnEntitySpawn(s.seedSpawn)
	r.Service("inventory", InventoryService(&service{}))
	r.Manifest(s.manifest())
}

func (s *System) seedSpawn(w syscore.World, e syscore.Entity) {
	if !syscore.IsAgentArchetype(e.Archetype()) {
		return
	}
	if _, ok := e.GetExtra("inventory"); !ok {
		e.SetExtra("inventory", []string{})
	}
	if _, ok := e.GetExtra("equipped"); !ok {
		e.SetExtra("equipped", map[string]any{})
	}
}

func (s *System) handlePickup(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	item := w.EntityByID(p.Target)
	if item == nil || item.Archetype() != "item" {
		res.Reason = "not_an_item"
		return res
	}
	if w.Chebyshev(e.Pos(), item.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	inv := extrasStrSlice(e, "inventory")
	if len(inv) >= DefaultMaxSlots {
		res.Reason = "inventory_full"
		return res
	}
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		cur := extrasStrSlice(real, "inventory")
		cur = append(cur, p.Target)
		real.SetExtra("inventory", cur)
	})
	w.RemoveEntity(p.Target)
	w.QueueEvent(ItemPicked{Picker: e.ID(), Item: p.Target})
	res.Accepted = true
	return res
}

func (s *System) handleDrop(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Item string `json:"item"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	inv := extrasStrSlice(e, "inventory")
	idx := indexOf(inv, p.Item)
	if idx < 0 {
		res.Reason = "not_in_inventory"
		return res
	}
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		cur := extrasStrSlice(real, "inventory")
		real.SetExtra("inventory", removeAt(cur, idx))
	})
	w.QueueEvent(ItemDropped{Dropper: e.ID(), Item: p.Item, At: e.Pos()})
	res.Accepted = true
	return res
}

func (s *System) handleEquip(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Item string `json:"item"`
		Slot string `json:"slot"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	inv := extrasStrSlice(e, "inventory")
	if indexOf(inv, p.Item) < 0 {
		res.Reason = "not_in_inventory"
		return res
	}
	slot := p.Slot
	if slot == "" {
		slot = "hand"
	}
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		eq, _ := real.GetExtra("equipped")
		m, _ := eq.(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		m[slot] = p.Item
		real.SetExtra("equipped", m)
	})
	res.Accepted = true
	return res
}

func (s *System) handleGive(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
		Item   string `json:"item"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	target := w.EntityByID(p.Target)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if !syscore.IsAgentArchetype(target.Archetype()) {
		res.Reason = "not_a_target"
		return res
	}
	if w.Chebyshev(e.Pos(), target.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	myInv := extrasStrSlice(e, "inventory")
	idx := indexOf(myInv, p.Item)
	if idx < 0 {
		res.Reason = "not_in_inventory"
		return res
	}
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		cur := extrasStrSlice(real, "inventory")
		real.SetExtra("inventory", removeAt(cur, idx))
	})
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		cur := extrasStrSlice(real, "inventory")
		cur = append(cur, p.Item)
		real.SetExtra("inventory", cur)
	})
	w.QueueEvent(ItemTransferred{From: e.ID(), To: p.Target, Item: p.Item})
	res.Accepted = true
	return res
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "inventory",
		Description: "Per-entity inventory of items + equip slots. Items are first-class entities of archetype='item' in the world.",
		Verbs: []manifest.VerbDeclaration{
			{Verb: "pickup", Description: "Pick up an adjacent item.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target is archetype=item", "target within 1 tile", "self has free slot"},
				RejectionReasons: []string{"bad_params", "not_an_item", "target_too_far", "inventory_full"},
				EmitsEvents:      []string{"ItemPicked"},
			},
			{Verb: "drop", Description: "Drop an item from inventory.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"item":{"type":"string"}},"required":["item"]}`),
				RejectionReasons: []string{"bad_params", "not_in_inventory"},
				EmitsEvents:      []string{"ItemDropped"},
			},
			{Verb: "equip", Description: "Wear / wield an inventory item.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"item":{"type":"string"},"slot":{"type":"string"}},"required":["item"]}`),
				RejectionReasons: []string{"bad_params", "not_in_inventory"},
			},
			{Verb: "give", Description: "Give an inventory item to an adjacent target.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"item":{"type":"string"}},"required":["target","item"]}`),
				RejectionReasons: []string{"bad_params", "unknown_target", "target_too_far", "not_in_inventory"},
				EmitsEvents:      []string{"ItemTransferred"},
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "inventory", Type: "list", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "list of item entity IDs the owner is carrying (private to owner)"},
			{Key: "equipped", Type: "object", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "map of slot -> item_id; visible to observers (you can see what someone is wearing)"},
		},
	}
}

// === Service implementation ===

type service struct{}

func (s *service) Has(w syscore.World, entityID, itemID string) bool {
	e := w.EntityByID(entityID)
	if e == nil {
		return false
	}
	return indexOf(extrasStrSlice(e, "inventory"), itemID) >= 0
}

func (s *service) HasAll(w syscore.World, entityID string, itemIDs []string) bool {
	e := w.EntityByID(entityID)
	if e == nil {
		return false
	}
	inv := extrasStrSlice(e, "inventory")
	for _, want := range itemIDs {
		if indexOf(inv, want) < 0 {
			return false
		}
	}
	return true
}

func (s *service) Consume(w syscore.World, entityID string, itemIDs []string) (bool, string) {
	if !s.HasAll(w, entityID, itemIDs) {
		return false, "missing_items"
	}
	w.MutateEntity(entityID, func(real syscore.Entity) {
		cur := extrasStrSlice(real, "inventory")
		for _, id := range itemIDs {
			cur = removeAt(cur, indexOf(cur, id))
		}
		real.SetExtra("inventory", cur)
	})
	return true, ""
}

func (s *service) Items(w syscore.World, entityID string) []string {
	e := w.EntityByID(entityID)
	if e == nil {
		return nil
	}
	return extrasStrSlice(e, "inventory")
}

// === helpers ===

func extrasStrSlice(e syscore.Entity, k string) []string {
	v, ok := e.GetExtra(k)
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func indexOf(s []string, item string) int {
	for i, x := range s {
		if x == item {
			return i
		}
	}
	return -1
}

func removeAt(s []string, idx int) []string {
	if idx < 0 || idx >= len(s) {
		return s
	}
	out := make([]string, 0, len(s)-1)
	out = append(out, s[:idx]...)
	out = append(out, s[idx+1:]...)
	return out
}

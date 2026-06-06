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

// MoneyGranter — the slice of money's MoneyService that inventory
// uses to auto-credit gold on coin/gem pickup. Kept as a local
// interface so inventory doesn't import the money package (cycle).
type MoneyGranter interface {
	Grant(world syscore.World, entityID string, amount int, cause string)
}

// coinValues — monetary item kinds (extracted from item id via
// itemKindFromID) and the gold they're worth on pickup. When a
// pickup target's kind is in this map, the item is destroyed +
// the value is credited via the money service; nothing lands in
// the player's inventory. This matches the user's mental model
// (coins are wealth, not carryable bags-of-stuff) and frees up
// the 10-slot inventory cap for items the player actually carries.
var coinValues = map[string]int{
	"coin_single":     1,
	"coins_small_pile": 5,
	"coin_pouch":      10,
	"gem_emerald":     50,
	"gem_ruby":        75,
	"gem_diamond":    100,
}

// DefaultMaxSlots — D20. Hard cap at 10 slots. Each item (including
// coin piles + equipped weapon slots NOT counted; equipped is in a
// separate Extras["equipped"] map) takes one slot. pickup rejects
// with reason="inventory_full" once 10 items are carried.
const DefaultMaxSlots = 10

// ItemPicked / ItemDropped / ItemTransferred / AteFood events.
type ItemPicked struct{ Picker, Item string }

func (ItemPicked) Kind() string { return "ItemPicked" }

type ItemDropped struct{ Dropper, Item string; At [2]int }

func (ItemDropped) Kind() string { return "ItemDropped" }

type ItemTransferred struct{ From, To, Item string }

func (ItemTransferred) Kind() string { return "ItemTransferred" }

// AteFood — D22. Emitted when an entity eats a food item via the
// `eat` verb. Hunger is the entity's NEW hunger value after eating.
type AteFood struct {
	Eater   string
	Item    string  // inventory item id consumed
	Satiety float64 // satiety subtracted from hunger
	Hunger  float64 // post-eat hunger value, clamped [0, 1]
}

func (AteFood) Kind() string { return "AteFood" }

var (
	_ eventbus.Event = ItemPicked{}
	_ eventbus.Event = ItemDropped{}
	_ eventbus.Event = ItemTransferred{}
	_ eventbus.Event = AteFood{}
)

// foodSatiety — D22 starting calibration. Maps food kind (the
// "<kind>" part of an item id "item:<kind>#<seq>") to the hunger
// reduction on eat. v1 hardcoded; should migrate to rulebook lookup
// when ItemKindProp accessor lands. Satiety values match the
// rulebook items + ARCHETYPE_FSMS expectations.
var foodSatiety = map[string]float64{
	"apple":        0.25,
	"bread_loaf":   0.5,
	"cheese_wheel": 0.7,
	"fish_cooked":  0.55,
	"fish_raw":     0.2,
}

// satietyForItem looks up the satiety for an inventory item id.
// Returns (value, true) if the item is recognized food, else (0,
// false). Strips the "item:" prefix + "#suffix" — accepts both
// "item:apple", "item:apple#42", and bare "apple" formats.
func satietyForItem(id string) (float64, bool) {
	// Strip "item:" prefix.
	if len(id) > 5 && id[:5] == "item:" {
		id = id[5:]
	}
	// Strip "#suffix".
	for i := 0; i < len(id); i++ {
		if id[i] == '#' {
			id = id[:i]
			break
		}
	}
	v, ok := foodSatiety[id]
	return v, ok
}

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
	r.Verb("eat", s.handleEat) // D22
	r.OnEntitySpawn(s.seedSpawn)
	r.Service("inventory", InventoryService(&service{}))
	r.Manifest(s.manifest())
}

// handleEat — D22. Consume a food item from inventory; subtract its
// satiety from hunger (clamped at 0). Instant; no action cooldown.
// Emits AteFood for the historian. Reasons:
//   - bad_params: malformed payload
//   - not_in_inventory: item id not in the eater's inventory
//   - not_food: item kind has no satiety value (not a food)
func (s *System) handleEat(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
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
	sat, isFood := satietyForItem(p.Item)
	if !isFood {
		res.Reason = "not_food"
		return res
	}
	// Subtract satiety from hunger, clamp at 0. Hunger may be missing
	// if the world has no vitals system; default to 0 (no-op effect).
	hungerRaw, _ := e.GetExtra("hunger")
	hunger, _ := hungerRaw.(float64)
	next := hunger - sat
	if next < 0 {
		next = 0
	}
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		real.SetExtra("hunger", next)
		cur := extrasStrSlice(real, "inventory")
		real.SetExtra("inventory", removeAt(cur, idx))
	})
	w.QueueEvent(AteFood{
		Eater:   e.ID(),
		Item:    p.Item,
		Satiety: sat,
		Hunger:  next,
	})
	res.Accepted = true
	return res
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
	// Coins + gems auto-convert to gold. They never enter inventory.
	// The kind is extracted from the item's sprite (the spawn pipeline
	// always sets sprite="item:<kind>"); fall back to the id parse
	// for legacy items that were spawned without an explicit sprite.
	kind := monetaryKindOf(item, p.Target)
	if value, ok := coinValues[kind]; ok {
		svc, _ := w.GetService("money").(MoneyGranter)
		if svc == nil {
			res.Reason = "money_service_missing"
			return res
		}
		svc.Grant(w, e.ID(), value, "pickup_coin")
		w.RemoveEntity(p.Target)
		w.QueueEvent(ItemPicked{Picker: e.ID(), Item: p.Target})
		res.Accepted = true
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

// monetaryKindOf — extract the canonical item kind for the coin
// table lookup. Priority: sprite extra (set by spawn pipelines, e.g.
// "item:coin_pouch") then the id parse (for items whose sprite extra
// was never set).
func monetaryKindOf(item syscore.Entity, fallbackID string) string {
	if s, ok := item.GetExtra("sprite"); ok {
		if str, ok := s.(string); ok && len(str) > 5 && str[:5] == "item:" {
			k := str[5:]
			for i := 0; i < len(k); i++ {
				if k[i] == '#' {
					k = k[:i]
					break
				}
			}
			return k
		}
	}
	return itemKindFromID(fallbackID)
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
	// D8/D10 prereq: spawn an actual item entity at the dropper's tile
	// so the dropped item is observable + pickup-able by other agents.
	// The pre-D8 implementation only emitted an event; the item
	// disappeared from the world. Sprite is derived from the item id
	// using the standard "item:<kind>#<seq>" → "item:<kind>" mapping
	// (so "item:apple#42" → sprite "item:apple"). Spawn-failure is
	// non-fatal — the drop still completes from the holder's PoV.
	_, _ = w.SpawnEntityFromSpec(syscore.EntitySpec{
		Archetype:   "item",
		Pos:         e.Pos(),
		DisplayName: itemKindFromID(p.Item),
		Extras: map[string]any{
			"sprite": spriteFromItemID(p.Item),
			"source": "drop",
		},
	})
	w.QueueEvent(ItemDropped{Dropper: e.ID(), Item: p.Item, At: e.Pos()})
	res.Accepted = true
	return res
}

// spriteFromItemID — mirror of the same convention used by D9's
// extras_summary builder. "item:apple#42" → "item:apple". Bare ids
// get the "item:" prefix. Used by drop to label the spawned item
// entity for downstream visibility (D8 visible_items + frontend).
func spriteFromItemID(id string) string {
	if id == "" {
		return ""
	}
	for i := 0; i < len(id); i++ {
		if id[i] == '#' {
			id = id[:i]
			break
		}
	}
	hasColon := false
	for i := 0; i < len(id); i++ {
		if id[i] == ':' {
			hasColon = true
			break
		}
	}
	if !hasColon {
		return "item:" + id
	}
	return id
}

// itemKindFromID strips "item:" prefix and "#suffix" from an id to
// recover the bare kind. "item:apple#42" → "apple". Used as the
// DisplayName of the spawned item entity.
func itemKindFromID(id string) string {
	if len(id) > 5 && id[:5] == "item:" {
		id = id[5:]
	}
	for i := 0; i < len(id); i++ {
		if id[i] == '#' {
			id = id[:i]
			break
		}
	}
	return id
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

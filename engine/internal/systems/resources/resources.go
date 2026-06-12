// Package resources — composable Resources system.
//
// Verbs: chop (trees), mine (rocks). Each takes a target entity. On
// success, the target's "yield" extras flow into the actor's inventory
// as item entity IDs, the target's "hardness" counter ticks down, and
// when fully depleted the resource entity is removed from the world.
//
// Resource entities live as archetype="tree" or archetype="rock" in
// the world JSON. They start with:
//   - hardness: int — hits remaining before depletion.
//   - yield:    []string — item IDs minted on each successful hit.
//
// Construction's MaterialsService will consume these item IDs through
// the standard InventoryService.
package resources

import (
	"encoding/json"
	"fmt"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const (
	DefaultTreeHardness = 3
	DefaultRockHardness = 5
	// forage: gather fruit from a tree without felling it — a renewable
	// food source. A tree can be foraged again only after this many ticks
	// (ripening), so it isn't an infinite apple spigot.
	DefaultForageCooldown = 600 // ~10s at 60Hz
	// Resource regrowth: partially-harvested trees/rocks slowly recover
	// hardness so a forest isn't permanently stripped. +1 hardness every
	// this many ticks (~30s), capped at the archetype's max. 0 disables.
	DefaultResourceRegenInterval = 1800
	// maxInventorySlots mirrors inventory.DefaultMaxSlots (D20). Harvest/
	// forage enforce it so they can't bypass the cap that pickup enforces.
	maxInventorySlots = 10
)

// === Events ===

type ResourceHarvested struct {
	By, Source string
	YieldItem  string
}

func (ResourceHarvested) Kind() string { return "ResourceHarvested" }

type ResourceDepleted struct {
	By, Source string
}

func (ResourceDepleted) Kind() string { return "ResourceDepleted" }

var (
	_ eventbus.Event = ResourceHarvested{}
	_ eventbus.Event = ResourceDepleted{}
)

// === System ===

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "resources" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("chop", s.handleChop)
	r.Verb("mine", s.handleMine)
	r.Verb("forage", s.handleForage)
	r.OnTick(s.regen)
	r.OnEntitySpawn(s.seedSpawn)
	r.Manifest(s.manifest())
}

func (s *System) seedSpawn(w syscore.World, e syscore.Entity) {
	// AUDIT FIX (low/[33]): honor per-world hardness tunings at seed time, the
	// same tunings regen already reads — otherwise seeded trees/rocks ignored
	// the world's tree_hardness/rock_hardness and regen could push them past
	// their (default) seed value inconsistently.
	switch e.Archetype() {
	case "tree":
		if _, ok := e.GetExtra("hardness"); !ok {
			e.SetExtra("hardness", w.TuningInt("tree_hardness", DefaultTreeHardness))
		}
		if _, ok := e.GetExtra("yield"); !ok {
			e.SetExtra("yield", []string{"wood"})
		}
	case "rock":
		if _, ok := e.GetExtra("hardness"); !ok {
			e.SetExtra("hardness", w.TuningInt("rock_hardness", DefaultRockHardness))
		}
		if _, ok := e.GetExtra("yield"); !ok {
			e.SetExtra("yield", []string{"stone"})
		}
	}
}

func (s *System) handleChop(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	return s.harvest(w, e, env, "tree")
}

func (s *System) handleMine(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	return s.harvest(w, e, env, "rock")
}

// handleForage — gather fruit from an adjacent tree/bush without felling
// it. Yields a food item (apple) into the actor's inventory and arms a
// ripening cooldown on the source so it can't be spammed. A renewable
// food source that deepens the survival economy. Reasons:
//   - bad_params / unknown_target / target_too_far
//   - not_forageable: target isn't a tree/bush
//   - not_ripe: foraged too recently; wait for it to bear fruit again
// regen slowly regrows hardness on partially-harvested (but not fully
// depleted/removed) tree and rock nodes, capped at the archetype max, so a
// forest recovers instead of being permanently stripped. 0 interval = off.
func (s *System) regen(w syscore.World, tick uint64) {
	interval := w.TuningInt("resource_regen_interval", DefaultResourceRegenInterval)
	if interval < 1 || tick%uint64(interval) != 0 {
		return
	}
	for _, id := range w.EntityIDs() {
		e := w.EntityByID(id)
		if e == nil {
			continue
		}
		var max int
		switch e.Archetype() {
		case "tree":
			max = w.TuningInt("tree_hardness", DefaultTreeHardness)
		case "rock":
			max = w.TuningInt("rock_hardness", DefaultRockHardness)
		default:
			continue
		}
		if h := intExtra(e, "hardness"); h > 0 && h < max {
			w.MutateEntity(id, func(re syscore.Entity) { re.SetExtra("hardness", h+1) })
		}
	}
}

func (s *System) handleForage(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	src := w.EntityByID(p.Target)
	if src == nil {
		res.Reason = "unknown_target"
		return res
	}
	if src.Archetype() != "tree" && src.Archetype() != "bush" {
		res.Reason = "not_forageable"
		return res
	}
	if w.Chebyshev(e.Pos(), src.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	tick := w.Tick()
	if ready := intExtra(src, "forage_ready_tick"); uint64(ready) > tick {
		res.Reason = "not_ripe"
		return res
	}
	// AUDIT FIX (high/[1]): enforce the 10-slot cap (D20) — forage was an
	// unbounded item spigot (renewable, no cost) that bypassed the cap.
	if len(stringSlice(e, "inventory")) >= maxInventorySlots {
		res.Reason = "inventory_full"
		return res
	}
	cooldown := w.TuningInt("forage_cooldown_ticks", DefaultForageCooldown)
	// Suffix with the ACTOR id (not just tick) so two agents foraging the same
	// kind on the same tick don't mint colliding ids — which would alias on
	// give/trade (audit). Matches pickup's #<entityid> uniqueness convention.
	itemID := fmt.Sprintf("item:apple#%s_%d", e.ID(), tick)
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		cur := stringSlice(real, "inventory")
		real.SetExtra("inventory", append(cur, itemID))
	})
	w.MutateEntity(src.ID(), func(real syscore.Entity) {
		real.SetExtra("forage_ready_tick", int(tick)+cooldown)
	})
	w.QueueEvent(ResourceHarvested{By: e.ID(), Source: src.ID(), YieldItem: itemID})
	w.SetEntityAction(e.ID(), "interact", 18)
	res.Accepted = true
	return res
}

func (s *System) harvest(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope, wantArch string) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	src := w.EntityByID(p.Target)
	if src == nil {
		res.Reason = "unknown_target"
		return res
	}
	if src.Archetype() != wantArch {
		res.Reason = fmt.Sprintf("not_a_%s", wantArch)
		return res
	}
	if w.Chebyshev(e.Pos(), src.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	yields := stringSlice(src, "yield")
	if len(yields) == 0 {
		res.Reason = "no_yield"
		return res
	}
	hardness := intExtra(src, "hardness")
	if hardness <= 0 {
		res.Reason = "depleted"
		return res
	}
	// AUDIT FIX (high/[1]): enforce the 10-slot inventory cap (D20) that
	// pickup enforces — harvest previously appended with no bound, an
	// unbounded-inventory exploit. Reject the whole hit if it wouldn't fit.
	if len(stringSlice(e, "inventory"))+len(yields) > maxInventorySlots {
		res.Reason = "inventory_full"
		return res
	}

	// Mint a fresh item entity per yield kind, drop into actor inventory.
	// AUDIT FIX (medium/[9]): use the canonical "item:<kind>#<unique>" id
	// format. The old "<kind>_<tick>_<i>" form had no "item:" prefix and
	// used "_" not "#", so eat/equip/render kind-resolution (which strips
	// "item:" + "#") couldn't read it — harvested wood/stone was unusable.
	tick := w.Tick()
	mintedIDs := make([]string, 0, len(yields))
	for i, kind := range yields {
		id := fmt.Sprintf("item:%s#%s_%d_%d", kind, e.ID(), tick, i) // actor id → cross-entity uniqueness (audit)
		mintedIDs = append(mintedIDs, id)
		w.QueueEvent(ResourceHarvested{By: e.ID(), Source: src.ID(), YieldItem: id})
	}

	// Append minted IDs to the actor's inventory directly. InventoryService
	// doesn't expose an Add primitive (its mutations are verb-driven); the
	// shape of the inventory extras is locked by docs/AFFORDANCE_MANIFEST.md.
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		cur := stringSlice(real, "inventory")
		cur = append(cur, mintedIDs...)
		real.SetExtra("inventory", cur)
	})

	newHardness := hardness - 1
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		real.SetExtra("hardness", newHardness)
	})
	if newHardness <= 0 {
		w.QueueEvent(ResourceDepleted{By: e.ID(), Source: src.ID()})
		w.RemoveEntity(src.ID())
	}
	res.Accepted = true
	return res
}

// === Manifest ===

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "resources",
		Description: "Chop trees, mine rocks. Each hit yields item entities into the actor's inventory; entity is removed when hardness reaches 0. Feeds Construction.",
		Verbs: []manifest.VerbDeclaration{
			{Verb: "chop", Description: "Chop an adjacent tree. Yields wood item IDs; depletes after N hits.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target is archetype=tree", "target within 1 tile", "target hardness > 0"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_tree", "target_too_far", "no_yield", "depleted", "inventory_full"},
				EmitsEvents:      []string{"ResourceHarvested", "ResourceDepleted"},
			},
			{Verb: "forage", Description: "Gather fruit (apple) from an adjacent tree/bush without felling it. Renewable food source; the source ripens again after forage_cooldown_ticks.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target is archetype=tree or bush", "target within 1 tile", "target is ripe (cooldown elapsed)"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_forageable", "target_too_far", "not_ripe", "inventory_full"},
				EmitsEvents:      []string{"ResourceHarvested"},
			},
			{Verb: "mine", Description: "Mine an adjacent rock. Yields stone item IDs; depletes after N hits.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target is archetype=rock", "target within 1 tile", "target hardness > 0"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_rock", "target_too_far", "no_yield", "depleted", "inventory_full"},
				EmitsEvents:      []string{"ResourceHarvested", "ResourceDepleted"},
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "hardness", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "hits remaining before this resource is depleted and removed"},
			{Key: "yield", Type: "list", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "kinds of items minted per successful harvest"},
		},
		Archetypes: []manifest.ArchetypeDecl{
			{Archetype: "tree", Description: "Choppable resource node. Yields wood item entities.", DefaultVerbsUsed: []string{"chop"}},
			{Archetype: "rock", Description: "Mineable resource node. Yields stone item entities.", DefaultVerbsUsed: []string{"mine"}},
		},
	}
}

// === helpers ===

func intExtra(e syscore.Entity, k string) int {
	v, ok := e.GetExtra(k)
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

func stringSlice(e syscore.Entity, k string) []string {
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

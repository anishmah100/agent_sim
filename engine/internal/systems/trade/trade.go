// Package trade — composable Trade system.
//
// One verb: trade. Depends on MoneyService + InventoryService. The
// engine treats trade as a primitive (single action that succeeds or
// fails atomically) because emergent verbal-contract trade lives one
// layer up — see VerbalQuests + the propose_task / accept_task verbs.
// This primitive exists so adjacent NPCs can settle "item for gold"
// without an AI agent on both sides; player merchants use it.
package trade

import (
	"encoding/json"

	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/inventory"
	"github.com/anishmah100/agent_sim/engine/internal/systems/money"
)

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "trade" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("trade", s.handleTrade)
	r.Manifest(s.manifest())
}

func (s *System) handleTrade(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
		Item   string `json:"item"`
		Price  int    `json:"price"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil || p.Price < 0 {
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
	if target.ID() == e.ID() {
		res.Reason = "self_target" // B15
		return res
	}
	if w.Chebyshev(e.Pos(), target.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}

	inv := w.GetService("inventory").(inventory.InventoryService)
	mon := w.GetService("money").(money.MoneyService)

	if !inv.Has(w, e.ID(), p.Item) {
		res.Reason = "not_in_inventory"
		return res
	}
	// AUDIT FIX (medium/[12]): respect the buyer's 10-slot cap (D20) BEFORE
	// any gold moves — trade could push a recipient past the cap. Checked
	// before payment so a failed trade leaves both sides untouched.
	if len(inv.Items(w, target.ID())) >= inventory.DefaultMaxSlots {
		res.Reason = "inventory_full"
		return res
	}
	if mon.Balance(w, target.ID()) < p.Price {
		res.Reason = "target_not_enough_gold"
		return res
	}

	// Atomic-from-the-caller's-PoV: payment fails → item stays.
	ok, reason := mon.Pay(w, target.ID(), e.ID(), p.Price, "trade")
	if !ok {
		res.Reason = reason
		return res
	}
	// Move the item via direct mutation (the InventoryService doesn't
	// expose a Move primitive; trade is the only caller that needs one).
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		cur := inv.Items(w, real.ID())
		out := make([]string, 0, len(cur))
		removed := false
		for _, id := range cur {
			if !removed && id == p.Item {
				removed = true
				continue
			}
			out = append(out, id)
		}
		real.SetExtra("inventory", out)
	})
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		cur := inv.Items(w, real.ID())
		cur = append(cur, p.Item)
		real.SetExtra("inventory", cur)
	})
	w.QueueEvent(inventory.ItemTransferred{From: e.ID(), To: p.Target, Item: p.Item})
	w.BumpSocial(e.ID(), p.Target, "trade")
	w.EmitSound(target.Pos(), "item_trade")   // FX: visible item handoff
	w.SetEntityAction(e.ID(), "interact", 18) // use animation
	res.Accepted = true
	return res
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "trade",
		Description: "Atomic item-for-gold swap with an adjacent partner. Higher-order verbal contracts use propose_task instead.",
		Verbs: []manifest.VerbDeclaration{
			{
				Verb:         "trade",
				Description:  "Give an item to an adjacent target in exchange for gold (target pays).",
				ParamsSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"item":{"type":"string"},"price":{"type":"integer","minimum":0}},"required":["target","item","price"]}`),
				Preconditions: []string{
					"target within 1 tile",
					"self has `item` in inventory",
					"target has at least `price` gold",
				},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_target", "self_target", "target_too_far", "not_in_inventory", "target_not_enough_gold", "inventory_full"},
				EmitsEvents:      []string{"GoldTransferred", "ItemTransferred"},
			},
		},
	}
}

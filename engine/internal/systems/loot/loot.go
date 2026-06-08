// Package loot — composable Loot system.
//
// One verb: loot. Take everything from an adjacent dead body. Requires
// Combat (HP check) + Money + Inventory services.
//
// AUDIT NOTE (findings [13]/[36]): in the current engine this verb is
// effectively UNREACHABLE. Combat (and starvation, via the same path) drop
// the victim's gold + inventory + equipped to the ground AND remove the
// corpse on the same tick a death occurs, so by the time an agent could loot,
// EntityByID(target) is nil → unknown_target. Death loot is therefore
// recovered by walking over the ground drops (pickup), not by this verb. The
// verb is kept for worlds that DON'T remove corpses immediately (a future
// "corpses linger N ticks" option) and to keep the service wiring intact.
package loot

import (
	"encoding/json"

	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/money"
)

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "loot" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("loot", s.handleLoot)
	r.Manifest(s.manifest())
}

func (s *System) handleLoot(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
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
	hpV, _ := target.GetExtra("hp")
	if asInt(hpV) > 0 {
		res.Reason = "target_alive"
		return res
	}

	mon := w.GetService("money").(money.MoneyService)
	if bal := mon.Balance(w, p.Target); bal > 0 {
		// AUDIT FIX (low/[37]): honor Pay's (ok, reason) — previously ignored,
		// so a failed transfer still reported Accepted=true (looted nothing).
		if ok, reason := mon.Pay(w, p.Target, e.ID(), bal, "loot"); !ok {
			res.Reason = reason
			return res
		}
	}
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		real.SetExtra("inventory", []string{})
	})
	res.Accepted = true
	return res
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "loot",
		Description: "Take gold + clear inventory from an adjacent corpse (target with HP=0).",
		Verbs: []manifest.VerbDeclaration{
			{
				Verb:             "loot",
				Description:      "Strip gold and inventory from an adjacent dead entity.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target within 1 tile", "target has hp == 0"},
				RejectionReasons: []string{"bad_params", "unknown_target", "target_too_far", "target_alive"},
				EmitsEvents:      []string{"GoldTransferred"},
			},
		},
	}
}

func asInt(v any) int {
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

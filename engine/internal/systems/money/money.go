// Package money — composable Money system.
//
// Adds gold balance to every spawn + verbs: pay, work_for_pay.
// (Trade lives partially here; the give-an-item half lives in the
// inventory system; trade itself orchestrates both.)
package money

import (
	"encoding/json"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const (
	DefaultStartingGold = 10
	WorkPayment         = 5
)

// GoldTransferred — emitted whenever gold moves between two entities.
type GoldTransferred struct {
	From   string
	To     string
	Amount int
	Cause  string // "pay", "trade", "work_for_pay", "loot"
}

func (GoldTransferred) Kind() string { return "GoldTransferred" }

var _ eventbus.Event = GoldTransferred{}

// MoneyService — exposed for other systems (construction consumes
// inventory items, but it might also be paid via gold).
type MoneyService interface {
	Balance(world syscore.World, entityID string) int
	Pay(world syscore.World, from, to string, amount int, cause string) (ok bool, reason string)
	Grant(world syscore.World, entityID string, amount int, cause string)
}

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "money" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("pay", s.handlePay)
	r.Verb("work_for_pay", s.handleWork)
	r.OnEntitySpawn(s.seedSpawn)
	r.Service("money", MoneyService(&service{}))
	r.Manifest(s.manifest())
}

func (s *System) seedSpawn(w syscore.World, e syscore.Entity) {
	if !syscore.IsAgentArchetype(e.Archetype()) {
		return
	}
	if _, ok := e.GetExtra("gold"); !ok {
		e.SetExtra("gold", w.TuningInt("starting_gold", DefaultStartingGold))
	}
}

func (s *System) handlePay(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
		Amount int    `json:"amount"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil || p.Amount <= 0 {
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
	svc := w.GetService("money").(MoneyService)
	ok, reason := svc.Pay(w, e.ID(), target.ID(), p.Amount, "pay")
	if !ok {
		res.Reason = reason
		return res
	}
	w.BumpSocial(e.ID(), target.ID(), "pay")
	res.Accepted = true
	return res
}

func (s *System) handleWork(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	svc := w.GetService("money").(MoneyService)
	svc.Grant(w, e.ID(), w.TuningInt("work_payment", WorkPayment), "work_for_pay")
	return syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb, Accepted: true}
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "money",
		Description: "Gold balance + transfers. Drives the economy. Doesn't enforce reciprocity — that's emergent (Q34 verbal contracts).",
		Verbs: []manifest.VerbDeclaration{
			{
				Verb:        "pay",
				Description: "Transfer gold to an adjacent entity.",
				ParamsSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"amount":{"type":"integer","minimum":1}},"required":["target","amount"]}`),
				Preconditions:    []string{"target within 1 tile", "self has at least `amount` gold"},
				RejectionReasons: []string{"bad_params", "unknown_target", "target_too_far", "not_enough_gold"},
				EmitsEvents:      []string{"GoldTransferred"},
			},
			{
				Verb:         "work_for_pay",
				Description:  "Perform labor (stub: just credits gold). Real version will validate a work-site.",
				ParamsSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "gold", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "current gold balance (private — only the owner sees it via self.extras)"},
		},
		SoundsEmitted: []manifest.SoundDecl{
			{Kind: "coin_clink", Description: "Gold transfer.", EmittedBy: "pay verb"},
		},
	}
}

// === Service implementation ===

type service struct{}

func (s *service) Balance(w syscore.World, entityID string) int {
	e := w.EntityByID(entityID)
	if e == nil {
		return 0
	}
	return extrasInt(e, "gold")
}

func (s *service) Pay(w syscore.World, fromID, toID string, amount int, cause string) (bool, string) {
	from := w.EntityByID(fromID)
	to := w.EntityByID(toID)
	if from == nil || to == nil {
		return false, "unknown_target"
	}
	bal := extrasInt(from, "gold")
	if bal < amount {
		return false, "not_enough_gold"
	}
	w.MutateEntity(fromID, func(real syscore.Entity) {
		real.SetExtra("gold", extrasInt(real, "gold")-amount)
	})
	w.MutateEntity(toID, func(real syscore.Entity) {
		real.SetExtra("gold", extrasInt(real, "gold")+amount)
	})
	w.QueueEvent(GoldTransferred{From: fromID, To: toID, Amount: amount, Cause: cause})
	w.EmitSound(from.Pos(), "coin_clink")
	return true, ""
}

func (s *service) Grant(w syscore.World, entityID string, amount int, cause string) {
	w.MutateEntity(entityID, func(real syscore.Entity) {
		real.SetExtra("gold", extrasInt(real, "gold")+amount)
	})
	w.QueueEvent(GoldTransferred{From: "", To: entityID, Amount: amount, Cause: cause})
}

func extrasInt(e syscore.Entity, k string) int {
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

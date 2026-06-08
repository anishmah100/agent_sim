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
	// Gold sink: buying a meal at the market. food_price gold buys
	// food_relief hunger reduction. Gives gold a survival purpose so a
	// hoard isn't inert wealth — the loop is earn/loot → buy food → live
	// longer. Tunable per world (food_price / food_relief).
	DefaultFoodPrice  = 6
	DefaultFoodRelief = 0.5
	// work_for_pay must happen at a worksite (any building) within this
	// Chebyshev radius — otherwise it's free gold minted from nowhere,
	// which wrecks the wealth distribution. Tunable via worksite_radius;
	// set to 0 to disable the gate.
	DefaultWorksiteRadius = 6
)

// GoldSpent — emitted when gold leaves the economy (a sink), e.g. buying
// a meal. Distinct from GoldTransferred (agent→agent), so the economy
// metrics can tell circulation apart from sinks.
type GoldSpent struct {
	Entity string
	Amount int
	On     string // "food"
}

func (GoldSpent) Kind() string { return "GoldSpent" }

var _ eventbus.Event = GoldSpent{}

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
	r.Verb("buy_food", s.handleBuyFood)
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
	if target.ID() == e.ID() {
		res.Reason = "self_target" // B15: no self-pay loops in the ledger
		return res
	}
	// Paying gold is a social transaction, not a physical grapple — allow
	// it at a short configurable range so a slow LLM brain doesn't have to
	// land on the exact adjacent tile of a moving target (the coordination
	// wall that left contract rewards/bribes unpaid: agents accepted deals
	// but could never get adjacent to honor them). attack stays strict.
	payRange := w.TuningInt("pay_max_range_tiles", 1)
	if payRange < 1 {
		payRange = 1
	}
	if w.Chebyshev(e.Pos(), target.Pos()) > payRange {
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
	w.SetEntityAction(e.ID(), "interact", 18) // use animation
	res.Accepted = true
	return res
}

// handleBuyFood — the economy's gold sink + survival loop. Spend
// food_price gold to relieve food_relief hunger (a meal at the market).
// Gold-gated only: the world interface doesn't expose decoration
// proximity to verb handlers yet, so a market-stall adjacency check is a
// future enhancement (the agent author already steers buyers toward the
// market hub). Reasons:
//   - not_hungry:     hunger already 0, nothing to buy
//   - not_enough_gold: balance < food_price
func (s *System) handleBuyFood(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	price := w.TuningInt("food_price", DefaultFoodPrice)
	relief := w.Tuning("food_relief", DefaultFoodRelief)

	goldRaw, _ := e.GetExtra("gold")
	gold := toInt(goldRaw)
	hungerRaw, _ := e.GetExtra("hunger")
	hunger, _ := hungerRaw.(float64)

	if hunger <= 0 {
		res.Reason = "not_hungry"
		return res
	}
	if gold < price {
		res.Reason = "not_enough_gold"
		return res
	}
	// Optional spatial gate: when market_radius > 0, food can only be
	// bought standing near a market stall, which makes the market a real
	// place agents converge on. Default 0 keeps the gold-only "rations"
	// model so a world without stalls still works.
	if mr := w.TuningInt("market_radius", 0); mr > 0 &&
		!w.HasDecorationNear(e.Pos(), "bld:stall", mr) {
		res.Reason = "no_market_nearby"
		return res
	}
	next := hunger - relief
	if next < 0 {
		next = 0
	}
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		real.SetExtra("gold", gold-price)
		real.SetExtra("hunger", next)
	})
	w.QueueEvent(GoldSpent{Entity: e.ID(), Amount: price, On: "food"})
	w.EmitSound(e.Pos(), "coin_clink")
	w.SetEntityAction(e.ID(), "interact", 18)
	res.Accepted = true
	return res
}

// toInt coerces a JSON-decoded number (int or float64) to int.
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func (s *System) handleWork(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	// Must be at a worksite (any building) — no minting gold from thin
	// air in an empty field. worksite_radius=0 disables the gate.
	if radius := w.TuningInt("worksite_radius", DefaultWorksiteRadius); radius > 0 &&
		!w.HasDecorationNear(e.Pos(), "bld:", radius) {
		res.Reason = "no_worksite_nearby"
		return res
	}
	svc := w.GetService("money").(MoneyService)
	svc.Grant(w, e.ID(), w.TuningInt("work_payment", WorkPayment), "work_for_pay")
	res.Accepted = true
	return res
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "money",
		Description: "Gold balance + transfers. Drives the economy. Doesn't enforce reciprocity — that's emergent (Q34 verbal contracts).",
		Verbs: []manifest.VerbDeclaration{
			{
				Verb:             "pay",
				Description:      "Transfer gold to an adjacent entity.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"amount":{"type":"integer","minimum":1}},"required":["target","amount"]}`),
				Preconditions:    []string{"target within pay_max_range_tiles (default 1) chebyshev", "self has at least `amount` gold"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_target", "self_target", "target_too_far", "not_enough_gold"},
				EmitsEvents:      []string{"GoldTransferred"},
			},
			{
				Verb:             "work_for_pay",
				Description:      "Perform labor at a worksite (any building within worksite_radius) for a wage.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{}}`),
				Preconditions:    []string{"a building within worksite_radius tiles"},
				RejectionReasons: []string{"no_worksite_nearby"},
				EmitsEvents:      []string{"GoldTransferred"},
			},
			{
				Verb:             "buy_food",
				Description:      "Buy a meal: spend food_price gold to reduce hunger by food_relief. The economy's gold sink + survival loop. With market_radius>0, must be at a market stall.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{}}`),
				Preconditions:    []string{"hunger > 0", "self has at least food_price gold", "(if market_radius>0) a market stall within range"},
				RejectionReasons: []string{"not_hungry", "not_enough_gold", "no_market_nearby"},
				EmitsEvents:      []string{"GoldSpent"},
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "gold", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "current gold balance (private — only the owner sees it via self.extras)"},
		},
		SoundsEmitted: []manifest.SoundDecl{
			{Kind: "coin_clink", Description: "Gold transfer / purchase.", EmittedBy: "pay + buy_food verbs"},
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

// Package combat — composable Combat system.
//
// Per docs/SYSTEM_ARCHITECTURE_V2.md, this is the new home for the
// combat ruleset (previously in engine/internal/scenario/fantasy_town).
// It registers Attack / Defend / Heal verbs, declares HP / max_hp /
// defending state on every spawn, emits EntityDied events, and
// exposes a CombatService for trap/structure/system-driven damage.
package combat

import (
	"encoding/json"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const (
	DefaultMaxHP        = 100
	DefaultAttackDamage = 12
	DefaultHealAmount   = 25
)

// EntityDied — emitted when an entity's HP reaches 0.
type EntityDied struct {
	EntityID string
	Killer   string // empty if cause was non-combat
	Cause    string // "attack", "trap", "fire", ...
}

func (EntityDied) Kind() string { return "EntityDied" }

// DamageDealt — emitted on every successful damage application.
type DamageDealt struct {
	Target   string
	Killer   string
	Amount   int
	NewHP    int
	Cause    string
}

func (DamageDealt) Kind() string { return "DamageDealt" }

// CombatService — exposed for other systems (traps, fire spread).
type CombatService interface {
	DealDamage(world syscore.World, targetID string, amount int, cause string, killer string) (newHP int, died bool)
}

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "combat" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("attack", s.handleAttack)
	r.Verb("defend", s.handleDefend)
	r.Verb("heal", s.handleHeal)
	r.OnEntitySpawn(s.seedSpawn)
	r.OnTick(s.tickRegen)
	r.Service("combat", CombatService(&service{}))
	r.Manifest(s.manifest())
}

func (s *System) seedSpawn(w syscore.World, e syscore.Entity) {
	if !syscore.IsAgentArchetype(e.Archetype()) {
		return
	}
	if _, ok := e.GetExtra("hp"); !ok {
		e.SetExtra("hp", DefaultMaxHP)
		e.SetExtra("max_hp", DefaultMaxHP)
	}
	if _, ok := e.GetExtra("defending"); !ok {
		e.SetExtra("defending", false)
	}
}

func (s *System) tickRegen(w syscore.World, tick uint64) {
	// HP regen +1 every 5 sec (300 ticks @ 60Hz) for entities not at 0.
	if tick%300 != 0 {
		return
	}
	for _, id := range w.EntityIDs() {
		e := w.EntityByID(id)
		if e == nil {
			continue
		}
		hp := extrasInt(e, "hp")
		maxHP := extrasInt(e, "max_hp")
		if hp <= 0 || hp >= maxHP {
			continue
		}
		w.MutateEntity(id, func(real syscore.Entity) {
			real.SetExtra("hp", hp+1)
		})
	}
}

func (s *System) handleAttack(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	other := w.EntityByID(p.Target)
	if other == nil {
		res.Reason = "unknown_target"
		return res
	}
	if !syscore.IsAgentArchetype(other.Archetype()) {
		res.Reason = "not_a_target"
		return res
	}
	if w.Chebyshev(e.Pos(), other.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	dmg := DefaultAttackDamage
	defending, _ := other.GetExtra("defending")
	if d, _ := defending.(bool); d {
		dmg /= 2
	}
	svc := w.GetService("combat").(CombatService)
	svc.DealDamage(w, other.ID(), dmg, "attack", e.ID())
	w.EmitSound(e.Pos(), "sword_clang")
	res.Accepted = true
	return res
}

func (s *System) handleDefend(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	w.MutateEntity(e.ID(), func(real syscore.Entity) {
		real.SetExtra("defending", true)
	})
	return syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb, Accepted: true}
}

func (s *System) handleHeal(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	_ = json.Unmarshal(env.Raw, &p)
	tid := p.Target
	if tid == "" {
		tid = e.ID()
	}
	target := w.EntityByID(tid)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if !syscore.IsAgentArchetype(target.Archetype()) {
		res.Reason = "not_a_target"
		return res
	}
	if target.ID() != e.ID() && w.Chebyshev(e.Pos(), target.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	hp := extrasInt(target, "hp")
	maxHP := extrasInt(target, "max_hp")
	newHP := hp + DefaultHealAmount
	if newHP > maxHP {
		newHP = maxHP
	}
	w.MutateEntity(target.ID(), func(real syscore.Entity) {
		real.SetExtra("hp", newHP)
	})
	res.Accepted = true
	return res
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "combat",
		Description: "HP-based melee combat. Attack damages adjacent targets, defend halves incoming damage, heal restores HP.",
		Verbs: []manifest.VerbDeclaration{
			{
				Verb:        "attack",
				Description: "Damage an adjacent target.",
				ParamsSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target must be within 1 tile (chebyshev)"},
				RejectionReasons: []string{"bad_params", "unknown_target", "target_too_far"},
				EmitsEvents:      []string{"DamageDealt", "EntityDied"},
				Examples: []manifest.VerbExample{
					{Params: json.RawMessage(`{"target":"goblin_3"}`), Result: "deals 12 dmg to goblin_3 (or 6 if defending)"},
				},
			},
			{
				Verb:             "defend",
				Description:      "Raise guard; halves the next incoming damage.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
				Preconditions:    []string{},
				RejectionReasons: []string{},
			},
			{
				Verb:         "heal",
				Description:  "Restore HP on self or adjacent target.",
				ParamsSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}}}`),
				Preconditions:    []string{"if target != self, target must be within 1 tile"},
				RejectionReasons: []string{"unknown_target", "target_too_far"},
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "hp", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "current hit points (0 = dead)"},
			{Key: "max_hp", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "ceiling on hp"},
			{Key: "defending", Type: "bool", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "guard stance — halves next incoming damage"},
		},
		SoundsEmitted: []manifest.SoundDecl{
			{Kind: "sword_clang", Description: "Attack lands.", EmittedBy: "attack verb"},
			{Kind: "death_scream", Description: "Entity dies.", EmittedBy: "EntityDied event"},
		},
	}
}

// === Service implementation ===

type service struct{}

func (s *service) DealDamage(w syscore.World, targetID string, amount int, cause string, killer string) (int, bool) {
	target := w.EntityByID(targetID)
	if target == nil {
		return 0, false
	}
	hp := extrasInt(target, "hp")
	newHP := hp - amount
	if newHP < 0 {
		newHP = 0
	}
	died := hp > 0 && newHP == 0
	w.MutateEntity(targetID, func(real syscore.Entity) {
		real.SetExtra("hp", newHP)
		real.SetExtra("defending", false)
	})
	w.QueueEvent(DamageDealt{Target: targetID, Killer: killer, Amount: amount, NewHP: newHP, Cause: cause})
	if died {
		// Credit the killer with a kill so leaderboards / inspector
		// have something to show. Non-combat causes (trap, fire) have
		// killer == "" and don't credit anyone.
		if killer != "" {
			w.MutateEntity(killer, func(real syscore.Entity) {
				real.SetExtra("kills", extrasInt(real, "kills")+1)
			})
		}
		w.QueueEvent(EntityDied{EntityID: targetID, Killer: killer, Cause: cause})
		w.EmitSound(target.Pos(), "death_scream")
	}
	return newHP, died
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

// Implement eventbus.WorldCtx for type compatibility.
var _ eventbus.Event = EntityDied{}
var _ eventbus.Event = DamageDealt{}

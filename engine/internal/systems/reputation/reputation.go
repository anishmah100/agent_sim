// Package reputation — a per-agent standing scalar that turns isolated
// violent/cooperative acts into a persistent social signal.
//
// Why it matters for emergence: without reputation, an agent's history is
// invisible — every encounter is a blank slate, so coalitions and revenge
// are random. With it, a killer accrues infamy that others can perceive
// and react to (flee the notorious, mob the murderer, trust the
// renowned). It's the load-bearing substrate for reputation-driven social
// dynamics.
//
// Model: each agent carries a "reputation" float (extras), 0 at spawn.
//   - killing drops it sharply (rep_kill_penalty, default 5)
//   - landing an attack drops it a little (rep_attack_penalty, default 0.5)
//   - it decays back toward 0 over time (rep_decay_step every
//     rep_decay_interval ticks) so infamy fades if you reform
//
// Reputation is surfaced to other agents via buildExtrasSummary (raw value
// + a coarse rep_bucket), so both rule-based and LLM agents can act on it.
//
// Updates run inside the bus drain, which the system host executes under
// the world write lock — so the handlers use the lock-free MutateEntity /
// EntityByID exactly like verb handlers do.
package reputation

import (
	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/combat"
)

const (
	DefaultKillPenalty   = 5.0
	DefaultAttackPenalty = 0.5
	DefaultDecayStep     = 0.05
	DefaultDecayInterval = 120 // ticks (~2s at 60Hz)
	RepFloor             = -50.0
	RepCeil              = 50.0
)

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "reputation" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.OnEntitySpawn(s.seed)
	r.OnEvent("EntityDied", s.onDeath)
	r.OnEvent("DamageDealt", s.onDamage)
	r.OnTick(s.decay)
	r.Manifest(s.manifest())
}

func (s *System) seed(w syscore.World, e syscore.Entity) {
	if !syscore.IsAgentArchetype(e.Archetype()) {
		return
	}
	if _, ok := e.GetExtra("reputation"); !ok {
		e.SetExtra("reputation", 0.0)
	}
}

func (s *System) onDeath(ctx eventbus.WorldCtx, ev eventbus.Event) {
	d, ok := ev.(combat.EntityDied)
	if !ok || d.Killer == "" {
		return
	}
	if w, ok := ctx.(syscore.World); ok {
		adjust(w, d.Killer, -w.Tuning("rep_kill_penalty", DefaultKillPenalty))
	}
}

func (s *System) onDamage(ctx eventbus.WorldCtx, ev eventbus.Event) {
	d, ok := ev.(combat.DamageDealt)
	if !ok || d.Killer == "" || d.Cause != "attack" {
		return
	}
	if w, ok := ctx.(syscore.World); ok {
		adjust(w, d.Killer, -w.Tuning("rep_attack_penalty", DefaultAttackPenalty))
	}
}

// decay nudges every agent's reputation toward 0 on a slow cadence, so a
// reformed killer eventually sheds infamy and one bad day doesn't brand an
// agent forever.
func (s *System) decay(w syscore.World, tick uint64) {
	interval := w.TuningInt("rep_decay_interval", DefaultDecayInterval)
	if interval < 1 {
		interval = 1
	}
	if tick%uint64(interval) != 0 {
		return
	}
	step := w.Tuning("rep_decay_step", DefaultDecayStep)
	for _, id := range w.EntityIDs() {
		e := w.EntityByID(id)
		if e == nil || !syscore.IsAgentArchetype(e.Archetype()) {
			continue
		}
		cur, ok := numeric(e.GetExtra("reputation"))
		if !ok || cur == 0 {
			continue
		}
		next := cur
		if cur > 0 {
			next = cur - step
			if next < 0 {
				next = 0
			}
		} else {
			next = cur + step
			if next > 0 {
				next = 0
			}
		}
		w.MutateEntity(id, func(re syscore.Entity) { re.SetExtra("reputation", next) })
	}
}

func adjust(w syscore.World, id string, delta float64) {
	e := w.EntityByID(id)
	if e == nil {
		return
	}
	cur, _ := numeric(e.GetExtra("reputation"))
	next := cur + delta
	if next < RepFloor {
		next = RepFloor
	}
	if next > RepCeil {
		next = RepCeil
	}
	w.MutateEntity(id, func(re syscore.Entity) { re.SetExtra("reputation", next) })
}

func numeric(v any, ok bool) (float64, bool) {
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "reputation",
		Description: "Per-agent standing scalar. Killing/attacking lowers it; it decays toward 0. Surfaced to others as reputation + rep_bucket so agents can react to infamy/renown.",
		StateFields: []manifest.StateFieldDecl{
			{Key: "reputation", Type: "float", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "social standing; negative = infamous (killer/aggressor), positive = renowned"},
		},
	}
}

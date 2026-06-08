// Package vitals — composable hunger / vitality system.
//
// Adds the "hunger" stat to every agent spawn and:
//   - increments hunger by w.Tuning("hunger_per_tick") each sim-tick,
//   - clamps to [0, 1],
//   - when hunger > w.Tuning("hunger_damage_above"), drains hp by
//     w.TuningInt("hunger_damage_rate") per tick.
//
// Eldoria's rules.star declares hunger_per_tick = 0.0008 (~5 min from
// 0 → 1 at 60 Hz), hunger_damage_above = 0.9, hunger_damage_rate = 1.
// A world that omits those tunings sees a no-op (defaults are 0 / 1).
//
// Note: this is intentionally minimal. Eat / drink verbs land here in
// a follow-up phase (Wave 6) when items get scattered through Eldoria.
package vitals

import (
	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const (
	DefaultHungerPerTick           = 0.0
	DefaultHungerDamageAbove       = 1.0 // never deal damage by default
	DefaultHungerDamageRate        = 0
	DefaultHungerDamageIntervalTks = 1 // every-tick damage (compat default)
)

// HungerSpike — emitted each time an entity crosses into starvation.
type HungerSpike struct {
	EntityID string
	Hunger   float64
}

func (HungerSpike) Kind() string { return "HungerSpike" }

var _ eventbus.Event = HungerSpike{}

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "vitals" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.OnEntitySpawn(s.seedSpawn)
	r.OnTick(s.tickHunger)
	r.Manifest(s.manifest())
}

func (s *System) seedSpawn(w syscore.World, e syscore.Entity) {
	if !syscore.IsAgentArchetype(e.Archetype()) {
		return
	}
	if _, ok := e.GetExtra("hunger"); !ok {
		e.SetExtra("hunger", 0.0)
	}
}

func (s *System) tickHunger(w syscore.World, tick uint64) {
	per := w.Tuning("hunger_per_tick", DefaultHungerPerTick)
	if per <= 0 {
		return // no hunger model in this world
	}
	above := w.Tuning("hunger_damage_above", DefaultHungerDamageAbove)
	rate := w.TuningInt("hunger_damage_rate", DefaultHungerDamageRate)
	// D4 — apply damage at intervals (not every tick) so an int rate
	// like 1 corresponds to ~1 HP per N seconds, not 60 HP/sec at
	// 60 Hz. Default interval = 1 preserves backwards compatibility.
	interval := w.TuningInt("hunger_damage_interval_ticks", DefaultHungerDamageIntervalTks)
	if interval < 1 {
		interval = 1
	}
	damageThisTick := tick%uint64(interval) == 0
	for _, id := range w.EntityIDs() {
		e := w.EntityByID(id)
		if e == nil {
			continue
		}
		if !syscore.IsAgentArchetype(e.Archetype()) {
			continue
		}
		raw, _ := e.GetExtra("hunger")
		curr, _ := raw.(float64)
		next := curr + per
		if next > 1 {
			next = 1
		}
		w.MutateEntity(id, func(real syscore.Entity) {
			real.SetExtra("hunger", next)
		})
		// Damage path. Only apply on tick boundaries of `interval`,
		// so 60 Hz × interval 324 ≈ 1 HP per 5.4 sec at rate=1.
		if next > above && rate > 0 && damageThisTick {
			hpRaw, _ := e.GetExtra("hp")
			hp, _ := hpRaw.(int)
			if hp > 0 {
				w.MutateEntity(id, func(real syscore.Entity) {
					real.SetExtra("hp", hp-rate)
				})
				// Distinct visual beat for starvation damage so the UI can
				// show a hunger pang (amber) rather than a red combat hit.
				// Fires at most once per damage interval (~5s), not spammy.
				// Skip agents hidden inside a building: they aren't rendered
				// on the overworld, so the "hungry" float would otherwise
				// appear over an empty tile (the building's footprint) with
				// no visible character beneath it.
				if w.InsideBuilding(id) == "" {
					w.EmitSound(e.Pos(), "hunger_pang")
				}
				// Only emit on the CROSSING (prev below → now above). The
				// previous code fired every tick the entity was above the
				// threshold — at scale (250 entities in Eldoria) this
				// drowned every other event in the historian (25,250 of
				// 25,303 records in the A9 smoke). Crossings are the
				// signal; sustained starvation is implicit in the hp
				// trajectory.
				if curr <= above {
					w.QueueEvent(HungerSpike{EntityID: id, Hunger: next})
				}
			}
		}
	}
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "vitals",
		Description: "Hunger drives over time and drains hp once it crosses a threshold. Tuned per world.",
		StateFields: []manifest.StateFieldDecl{
			{
				Key:                  "hunger",
				Type:                 "float",
				Owner:                "entity",
				PublicAtAnyDistance:  false,
				PublicWithinDistance: 0,
				Meaning:              "0 = sated, 1 = starving. Grows per tick; high values drain hp.",
			},
		},
	}
}

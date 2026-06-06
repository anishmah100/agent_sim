// Package respawn — periodic world spawning of food + small wealth.
//
// User feedback during P4 build: "food seems really rare. Maybe make
// it more abundant, and have some process for items to spawn randomly
// in the world rather than just being statically fixed."
//
// This system runs every Tick. Every `respawn_interval_ticks` ticks
// (default 1800 = 30 in-game sec at 60Hz), it spawns one item from a
// rotation of food + small wealth at a random walkable tile within
// the configured radius of a respawn hub. Spawning stops once the
// total count of item entities exceeds `respawn_cap` to prevent
// runaway accumulation.
//
// All tunings live in rules.star:
//   respawn_interval_ticks  (default 1800)
//   respawn_cap             (default 600)
//   respawn_radius          (default 200)
//   respawn_hub_x           (default 750)
//   respawn_hub_y           (default 750)
package respawn

import (
	"math/rand/v2"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const (
	DefaultRespawnIntervalTicks = 1800
	DefaultRespawnCap           = 600
	DefaultRespawnRadius        = 200
	DefaultRespawnHubX          = 750
	DefaultRespawnHubY          = 750
)

// Spawnable — one of these is picked uniformly at random each
// respawn tick. Sized so food is ~50% of spawns, wealth ~25%, tools
// ~12%, weapons ~12%. The weapon slice lets armed archetypes
// (killers) actually equip + deal real damage during a run; without
// weapons in the world the killers would only ever do 4 dmg/sec
// (unarmed) which is barely above hp regen, so kills wouldn't land
// within demo-scale runs.
var spawnable = []struct {
	kind string
}{
	// Food (×6 = ~50% probability)
	{"apple"}, {"apple"}, {"bread_loaf"}, {"cheese_wheel"}, {"fish_cooked"}, {"fish_raw"},
	// Wealth (×3 = ~25%)
	{"coin_single"}, {"coins_small_pile"}, {"gem_emerald"},
	// Tools / misc (×2 = ~17%)
	{"wood_log"}, {"bucket_water"},
	// Weapons (×2 = ~17%)
	{"dagger"}, {"sword_short"},
}

// Spawned — emitted on each successful spawn so the historian can
// log world dynamism.
type Spawned struct {
	EntityID string
	Sprite   string
	At       [2]int
}

func (Spawned) Kind() string { return "Spawned" }

var _ eventbus.Event = Spawned{}

type System struct {
	rng *rand.Rand
}

func New() *System {
	// Seeded from a fixed source for now; respawn deterministic if
	// seed propagation lands (D23 deferred).
	return &System{rng: rand.New(rand.NewPCG(7, 31))}
}

func (s *System) Name() string { return "respawn" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.OnTick(s.tick)
	r.Manifest(s.manifest())
}

func (s *System) tick(w syscore.World, tick uint64) {
	interval := w.TuningInt("respawn_interval_ticks", DefaultRespawnIntervalTicks)
	if interval <= 0 {
		return
	}
	if tick%uint64(interval) != 0 {
		return
	}
	cap := w.TuningInt("respawn_cap", DefaultRespawnCap)
	// Count current items in the world. EntitiesInRadius is cheaper
	// than walking every entity but we don't have a per-archetype
	// index, so just walk EntityIDs.
	ids := w.EntityIDs()
	count := 0
	for _, id := range ids {
		e := w.EntityByID(id)
		if e == nil {
			continue
		}
		if e.Archetype() == "item" {
			count++
		}
	}
	if count >= cap {
		return
	}
	// Pick a random walkable tile within the radius of the hub.
	radius := w.TuningInt("respawn_radius", DefaultRespawnRadius)
	hubX := w.TuningInt("respawn_hub_x", DefaultRespawnHubX)
	hubY := w.TuningInt("respawn_hub_y", DefaultRespawnHubY)
	var pos [2]int
	found := false
	for attempt := 0; attempt < 32; attempt++ {
		dx := s.rng.IntN(2*radius+1) - radius
		dy := s.rng.IntN(2*radius+1) - radius
		t := [2]int{hubX + dx, hubY + dy}
		if w.IsWalkable(t) {
			pos = t
			found = true
			break
		}
	}
	if !found {
		return
	}
	pick := spawnable[s.rng.IntN(len(spawnable))]
	sprite := "item:" + pick.kind
	spawned, err := w.SpawnEntityFromSpec(syscore.EntitySpec{
		Archetype:   "item",
		Pos:         pos,
		DisplayName: pick.kind,
		Extras: map[string]any{
			"sprite": sprite,
			"source": "respawn",
		},
	})
	if err != nil || spawned == nil {
		return
	}
	w.QueueEvent(Spawned{
		EntityID: spawned.ID(),
		Sprite:   sprite,
		At:       pos,
	})
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "respawn",
		Description: "Periodic world respawn — drops food + wealth + tools at random walkable tiles within a hub radius. Caps total item count to prevent runaway.",
	}
}

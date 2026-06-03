// Package fantasy_town — the v0 fantasy town scenario.
//
// Post Session-2 refactor, this package is intentionally tiny: it
// declares which composable systems to install. Every rule lives in
// engine/internal/systems/<name>. Adding a new ruleset to this world
// is a one-line change here.
//
// When the TOML scenario loader lands (task #109 part 2) this whole
// file becomes a scenario.toml manifest plus a generic loader.
package fantasy_town

import (
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/systems/combat"
	"github.com/anishmah100/agent_sim/engine/internal/systems/inventory"
	"github.com/anishmah100/agent_sim/engine/internal/systems/loot"
	"github.com/anishmah100/agent_sim/engine/internal/systems/money"
	"github.com/anishmah100/agent_sim/engine/internal/systems/property"
	"github.com/anishmah100/agent_sim/engine/internal/systems/quests"
	"github.com/anishmah100/agent_sim/engine/internal/systems/trade"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const Name = "fantasy_town"

// Install creates a SystemHost, registers the fantasy_town system set,
// and wires it into the live World via InstallScenario. Returns the
// host so the caller can serve the aggregated manifest / inspect the
// bus from HTTP handlers.
func Install(w *world.World) *world.SystemHost {
	agg := manifest.NewAggregator(w.MapID, Name)
	host := world.NewSystemHost(w, agg)

	// Order matters for OnEntitySpawn: combat seeds hp first, money
	// seeds gold, inventory seeds inventory[]. Quests + trade + loot
	// register no spawn hooks; they consume what's already there.
	host.Install(combat.New())
	host.Install(money.New())
	host.Install(inventory.New())
	host.Install(property.New())
	host.Install(trade.New())
	host.Install(loot.New())
	host.Install(quests.New())

	host.InstallInto()
	return host
}

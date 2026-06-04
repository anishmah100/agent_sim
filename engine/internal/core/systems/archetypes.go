package systems

// Single source of truth for the engine's archetype taxonomy.
//
// Before this file existed, multiple systems (combat, money, inventory)
// each maintained their own ad-hoc skip lists like
//
//   if archetype == "item" || archetype == "building" || archetype == "decoration"
//
// These lists drifted apart. Resources added `tree` and `rock` as
// first-class entities so they could be targets of chop/mine, but the
// other skip lists were never updated — so trees ended up with HP,
// gold, and inventory. An agent could attack a tree, pay gold to a
// rock, heal a blueprint.
//
// The taxonomy:
//
//   AGENT archetype    — something with a brain / behavior. NPCs,
//                        players, hostile creatures. Eligible for
//                        agent binding, combat, economy, social
//                        verbs (pay, give, trade, propose_task).
//                        Gets HP, gold, inventory seeded on spawn.
//
//   OBJECT archetype   — world furniture. Trees, rocks, items,
//                        buildings, blueprints, decorations. Does
//                        NOT get HP / gold / inventory seeded.
//                        Cannot be attacked, paid, healed. CAN be
//                        chopped / mined / picked up / entered /
//                        claimed via its own system's verbs.
//
// The OBJECT set is the small, enumerable one. Everything else is
// treated as an agent. Adding a new NPC kind (e.g. "elf_archer") is
// zero-config; adding a new world-object kind (e.g. "trap") goes here.

// objectArchetypes is the closed set of archetypes that are world
// objects, not agents. ANY archetype not in this set is considered
// agent-eligible.
var objectArchetypes = map[string]bool{
	"item":       true,
	"tree":       true,
	"rock":       true,
	"building":   true,
	"blueprint":  true,
	"decoration": true,
}

// IsObjectArchetype reports whether the given archetype is a world
// object (vs. an agent-like entity).
func IsObjectArchetype(a string) bool {
	return objectArchetypes[a]
}

// IsAgentArchetype is the negation. Use this when the caller cares
// about "can this entity be controlled / attacked / paid?" — the
// closed object set says no, everything else says yes.
func IsAgentArchetype(a string) bool {
	return !objectArchetypes[a]
}

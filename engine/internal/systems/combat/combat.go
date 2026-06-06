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
	// DefaultAttackDamage — D21 changed this from 12 to 4 to match the
	// unarmed weapon stat (weaponStats's unarmedDmg). Verbs that don't
	// resolve a weapon still see this default via the fallback path.
	DefaultAttackDamage = 4
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
	maxHP := w.TuningInt("max_hp", DefaultMaxHP)
	if _, ok := e.GetExtra("hp"); !ok {
		e.SetExtra("hp", maxHP)
		e.SetExtra("max_hp", maxHP)
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
	// D21 — weapon damage + reach. Read attacker's equipped weapon, look
	// up its damage + reach. Unarmed = base damage 4, reach 1.
	wepDmg, wepReach := weaponStats(e)
	dmg := wepDmg
	if dmg == 0 {
		// Fallback: legacy world tuning if the weapon table doesn't
		// know this weapon. Keeps backwards compat for any rule-based
		// scenario that relied on attack_damage.
		dmg = w.TuningInt("attack_damage", DefaultAttackDamage)
	}
	if w.Chebyshev(e.Pos(), other.Pos()) > wepReach {
		res.Reason = "out_of_range"
		return res
	}
	defending, _ := other.GetExtra("defending")
	if d, _ := defending.(bool); d {
		// Tuned ratio (0..1). Default 0.5 matches the legacy "halve damage"
		// behaviour. Eldoria's rules.star declares defend_damage_mul=0.5.
		mul := w.Tuning("defend_damage_mul", 0.5)
		dmg = int(float64(dmg) * mul)
	}
	svc := w.GetService("combat").(CombatService)
	svc.DealDamage(w, other.ID(), dmg, "attack", e.ID())
	w.EmitSound(e.Pos(), "sword_clang")
	res.Accepted = true
	return res
}

// weaponStats — D21 starting calibration. Reads attacker's
// extras["equipped"]["weapon"] (item id like "item:sword_short#42")
// and looks up damage + reach. Returns (4, 1) for unarmed: base
// 4 HP melee. Hardcoded table; will migrate to rulebook
// ItemKindProp when that accessor lands.
//
// Reach is a Chebyshev tile radius. Bow/crossbow ranged weapons
// have reach > 1; LOS check is NOT yet enforced (future P3 polish).
func weaponStats(e syscore.Entity) (damage, reach int) {
	const (
		unarmedDmg   = 4
		unarmedReach = 1
	)
	eqRaw, ok := e.GetExtra("equipped")
	if !ok {
		return unarmedDmg, unarmedReach
	}
	eq, ok := eqRaw.(map[string]any)
	if !ok || len(eq) == 0 {
		return unarmedDmg, unarmedReach
	}
	weaponRaw, ok := eq["weapon"]
	if !ok {
		return unarmedDmg, unarmedReach
	}
	wid, _ := weaponRaw.(string)
	if wid == "" {
		return unarmedDmg, unarmedReach
	}
	// Strip "item:" + "#suffix" to recover kind.
	kind := wid
	if len(kind) > 5 && kind[:5] == "item:" {
		kind = kind[5:]
	}
	for i := 0; i < len(kind); i++ {
		if kind[i] == '#' {
			kind = kind[:i]
			break
		}
	}
	if w, ok := weaponTable[kind]; ok {
		return w.damage, w.reach
	}
	return unarmedDmg, unarmedReach
}

type weaponStat struct {
	damage int
	reach  int
}

// weaponTable — D21 reference. Matches the ARCHETYPE_FSMS doc + the
// rulebook item kinds. Damage is final HP loss before defend
// multiplier. Reach is Chebyshev tiles.
var weaponTable = map[string]weaponStat{
	"dagger":       {damage: 10, reach: 1},
	"sword_short":  {damage: 12, reach: 1},
	"sword_long":   {damage: 16, reach: 1},
	"axe":          {damage: 18, reach: 1},
	"club_wood":    {damage: 6, reach: 1},
	"hammer":       {damage: 12, reach: 1},
	"bow":          {damage: 10, reach: 6},
	"crossbow":     {damage: 14, reach: 6},
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
	newHP := hp + w.TuningInt("heal_amount", DefaultHealAmount)
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
		// D10 — drop full inventory + gold + equipped at corpse tile so
		// loot is recoverable. Iterate inventory + spawn an entity per
		// item (mirrors what handleDrop does for a single item).
		inv, _ := target.GetExtra("inventory")
		if items, ok := inv.([]string); ok {
			for _, iid := range items {
				_, _ = w.SpawnEntityFromSpec(syscore.EntitySpec{
					Archetype:   "item",
					Pos:         target.Pos(),
					DisplayName: itemKindFromID(iid),
					Extras: map[string]any{
						"sprite": spriteFromItemID(iid),
						"source": "death_drop",
					},
				})
			}
			w.MutateEntity(targetID, func(real syscore.Entity) {
				real.SetExtra("inventory", []string{})
			})
		}
		// Equipped weapon also drops.
		if eqRaw, ok := target.GetExtra("equipped"); ok {
			if eq, ok2 := eqRaw.(map[string]any); ok2 {
				for _, raw := range eq {
					iid, _ := raw.(string)
					if iid == "" {
						continue
					}
					_, _ = w.SpawnEntityFromSpec(syscore.EntitySpec{
						Archetype:   "item",
						Pos:         target.Pos(),
						DisplayName: itemKindFromID(iid),
						Extras: map[string]any{
							"sprite": spriteFromItemID(iid),
							"source": "death_drop",
						},
					})
				}
				w.MutateEntity(targetID, func(real syscore.Entity) {
					real.SetExtra("equipped", map[string]any{})
				})
			}
		}
		// D10 audible: anonymous scream + targeted witness events.
		muffled := false
		if ib, ok := target.GetExtra("inside_building"); ok {
			if s, ok2 := ib.(string); ok2 && s != "" {
				muffled = true
			}
		}
		w.EmitDeathScream(target.Pos(), targetID, killer, muffled)
	}
	return newHP, died
}

// spriteFromItemID + itemKindFromID — D10 death drop helpers. Mirror
// of the inventory system's same-name helpers (kept duplicated to
// avoid creating a circular import: combat doesn't import inventory).
// "item:sword_short#42" → sprite "item:sword_short", kind "sword_short".
func spriteFromItemID(id string) string {
	if id == "" {
		return ""
	}
	for i := 0; i < len(id); i++ {
		if id[i] == '#' {
			id = id[:i]
			break
		}
	}
	hasColon := false
	for i := 0; i < len(id); i++ {
		if id[i] == ':' {
			hasColon = true
			break
		}
	}
	if !hasColon {
		return "item:" + id
	}
	return id
}

func itemKindFromID(id string) string {
	if len(id) > 5 && id[:5] == "item:" {
		id = id[5:]
	}
	for i := 0; i < len(id); i++ {
		if id[i] == '#' {
			id = id[:i]
			break
		}
	}
	return id
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

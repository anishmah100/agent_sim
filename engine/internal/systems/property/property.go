// Package property — composable Property / Building system.
//
// Buildings are first-class entities (archetype="building"). This
// system layers ownership, locking, and explicit enter/exit verbs on
// top of the existing engine-level interior-membership machinery.
//
// Verbs:
//   - enter:               step inside an adjacent building.
//   - exit:                leave the current building.
//   - lock / unlock:       toggle the building's locked state.
//   - claim_ownership:     take ownership of an unowned building.
//   - transfer_ownership:  give ownership to another entity.
//
// State on building entities:
//   - owner:    entity id of the owner, "" if unowned.
//   - locked:   bool. Only the owner can lock/unlock or enter when locked.
//   - access:   []string of entity ids the owner has granted entry to.
//
// Per Q15-Q17 from session-2 decisions: buildings are first-class
// entities with per-instance interiors, always-tick. Construction
// produces these via SpawnEntity + claim_ownership.
package property

import (
	"encoding/json"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const DefaultInteriorTicks = 600 // 10s @ 60Hz — auto-exit window

// === Events ===

type EnteredBuilding struct{ Entity, Building string }

func (EnteredBuilding) Kind() string { return "EnteredBuilding" }

type ExitedBuilding struct{ Entity, Building string }

func (ExitedBuilding) Kind() string { return "ExitedBuilding" }

type BuildingLocked struct{ Building, By string }

func (BuildingLocked) Kind() string { return "BuildingLocked" }

type BuildingUnlocked struct{ Building, By string }

func (BuildingUnlocked) Kind() string { return "BuildingUnlocked" }

type OwnershipChanged struct {
	Building string
	From, To string
}

func (OwnershipChanged) Kind() string { return "OwnershipChanged" }

var (
	_ eventbus.Event = EnteredBuilding{}
	_ eventbus.Event = ExitedBuilding{}
	_ eventbus.Event = BuildingLocked{}
	_ eventbus.Event = BuildingUnlocked{}
	_ eventbus.Event = OwnershipChanged{}
)

// === Service ===

// PropertyService — exposed for other systems (construction registers
// new buildings; trade/loot might check ownership before mutating).
type PropertyService interface {
	OwnerOf(w syscore.World, buildingID string) string
	IsLocked(w syscore.World, buildingID string) bool
	CanEnter(w syscore.World, entityID, buildingID string) bool
}

// === System ===

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "property" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("enter", s.handleEnter)
	r.Verb("exit", s.handleExit)
	r.Verb("lock", s.handleLock)
	r.Verb("unlock", s.handleUnlock)
	r.Verb("claim_ownership", s.handleClaim)
	r.Verb("transfer_ownership", s.handleTransfer)
	r.OnEntitySpawn(s.seedSpawn)
	r.Service("property", PropertyService(&service{}))
	r.Manifest(s.manifest())
}

func (s *System) seedSpawn(w syscore.World, e syscore.Entity) {
	if e.Archetype() != "building" {
		return
	}
	if _, ok := e.GetExtra("owner"); !ok {
		e.SetExtra("owner", "")
	}
	if _, ok := e.GetExtra("locked"); !ok {
		e.SetExtra("locked", false)
	}
	if _, ok := e.GetExtra("access"); !ok {
		e.SetExtra("access", []string{})
	}
}

// === Verb handlers ===

func (s *System) handleEnter(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	b := w.EntityByID(p.Target)
	if b == nil {
		res.Reason = "unknown_target"
		return res
	}
	if b.Archetype() != "building" {
		res.Reason = "not_a_building"
		return res
	}
	if w.Chebyshev(e.Pos(), b.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	if w.InsideBuilding(e.ID()) != "" {
		res.Reason = "already_inside"
		return res
	}
	if locked, _ := boolExtra(b, "locked"); locked {
		if !hasEntryRight(b, e.ID()) {
			res.Reason = "locked"
			return res
		}
	}
	if !w.EnterBuilding(e.ID(), p.Target, DefaultInteriorTicks) {
		res.Reason = "enter_failed"
		return res
	}
	w.QueueEvent(EnteredBuilding{Entity: e.ID(), Building: p.Target})
	res.Accepted = true
	return res
}

func (s *System) handleExit(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	in := w.InsideBuilding(e.ID())
	if in == "" {
		res.Reason = "not_inside"
		return res
	}
	if !w.ExitBuilding(e.ID()) {
		res.Reason = "exit_failed"
		return res
	}
	w.QueueEvent(ExitedBuilding{Entity: e.ID(), Building: in})
	res.Accepted = true
	return res
}

func (s *System) handleLock(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	return s.setLocked(w, e, env, true)
}

func (s *System) handleUnlock(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	return s.setLocked(w, e, env, false)
}

func (s *System) setLocked(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope, locked bool) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	b := w.EntityByID(p.Target)
	if b == nil {
		res.Reason = "unknown_target"
		return res
	}
	if b.Archetype() != "building" {
		res.Reason = "not_a_building"
		return res
	}
	if owner, _ := stringExtra(b, "owner"); owner != e.ID() {
		res.Reason = "not_owner"
		return res
	}
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		real.SetExtra("locked", locked)
	})
	if locked {
		w.QueueEvent(BuildingLocked{Building: p.Target, By: e.ID()})
	} else {
		w.QueueEvent(BuildingUnlocked{Building: p.Target, By: e.ID()})
	}
	res.Accepted = true
	return res
}

func (s *System) handleClaim(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	b := w.EntityByID(p.Target)
	if b == nil {
		res.Reason = "unknown_target"
		return res
	}
	if b.Archetype() != "building" {
		res.Reason = "not_a_building"
		return res
	}
	if w.Chebyshev(e.Pos(), b.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	if owner, _ := stringExtra(b, "owner"); owner != "" {
		res.Reason = "already_owned"
		return res
	}
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		real.SetExtra("owner", e.ID())
	})
	w.QueueEvent(OwnershipChanged{Building: p.Target, From: "", To: e.ID()})
	res.Accepted = true
	return res
}

func (s *System) handleTransfer(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target   string `json:"target"`
		NewOwner string `json:"new_owner"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	b := w.EntityByID(p.Target)
	if b == nil {
		res.Reason = "unknown_target"
		return res
	}
	if b.Archetype() != "building" {
		res.Reason = "not_a_building"
		return res
	}
	owner, _ := stringExtra(b, "owner")
	if owner != e.ID() {
		res.Reason = "not_owner"
		return res
	}
	// AUDIT FIX (low/[32]): the new owner must be an AGENT — previously any
	// entity (a building, an item, a tree) could be set as a building's owner,
	// producing nonsensical ownership the access/lock checks then trusted.
	newOwner := w.EntityByID(p.NewOwner)
	if newOwner == nil || !syscore.IsAgentArchetype(newOwner.Archetype()) {
		res.Reason = "unknown_new_owner"
		return res
	}
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		real.SetExtra("owner", p.NewOwner)
	})
	w.QueueEvent(OwnershipChanged{Building: p.Target, From: e.ID(), To: p.NewOwner})
	res.Accepted = true
	return res
}

// === Manifest ===

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "property",
		Description: "Buildings as owned, lockable entities. Drives ownership, access, enter/exit semantics; Construction registers buildings here.",
		Verbs: []manifest.VerbDeclaration{
			{Verb: "enter", Description: "Step inside an adjacent building.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target within 1 tile", "target is archetype=building", "self not already inside a building", "not locked or self has entry right"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_building", "target_too_far", "already_inside", "locked", "enter_failed"},
				EmitsEvents:      []string{"EnteredBuilding"},
			},
			{Verb: "exit", Description: "Leave the current building.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{}}`),
				Preconditions:    []string{"self is currently inside a building"},
				RejectionReasons: []string{"not_inside", "exit_failed"},
				EmitsEvents:      []string{"ExitedBuilding"},
			},
			{Verb: "lock", Description: "Lock an owned building.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"self is the owner of target"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_building", "not_owner"},
				EmitsEvents:      []string{"BuildingLocked"},
			},
			{Verb: "unlock", Description: "Unlock an owned building.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"self is the owner of target"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_building", "not_owner"},
				EmitsEvents:      []string{"BuildingUnlocked"},
			},
			{Verb: "claim_ownership", Description: "Take ownership of an unowned adjacent building.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target within 1 tile", "target has no owner"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_building", "target_too_far", "already_owned"},
				EmitsEvents:      []string{"OwnershipChanged"},
			},
			{Verb: "transfer_ownership", Description: "Give ownership of an owned building to another entity.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"new_owner":{"type":"string"}},"required":["target","new_owner"]}`),
				Preconditions:    []string{"self is the current owner", "new_owner is a known entity"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_building", "not_owner", "unknown_new_owner"},
				EmitsEvents:      []string{"OwnershipChanged"},
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "owner", Type: "string", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "entity id of the owner of this building, or empty if unowned"},
			{Key: "locked", Type: "bool", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "if true, only owner or entities in access[] can enter"},
			// AUDIT NOTE (medium/[8]): access[] is read by enter (hasEntryRight)
			// but there is NO verb to populate it yet — a locked building is
			// currently enterable ONLY by its owner. A grant_access verb is a
			// planned feature (flagged For the maintainer in SESSION_HANDOFF).
			{Key: "access", Type: "list", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "entity ids the owner has granted entry rights to (private; no grant verb yet — owner-only for now)"},
		},
		Archetypes: []manifest.ArchetypeDecl{
			{Archetype: "building", Description: "A first-class structure entity. Owned via claim_ownership; constructed via the construction system; enterable as an interior tile space.",
				DefaultVerbsUsed: []string{"enter", "exit", "lock", "unlock", "claim_ownership", "transfer_ownership"},
			},
		},
	}
}

// === Service ===

type service struct{}

func (svc *service) OwnerOf(w syscore.World, buildingID string) string {
	e := w.EntityByID(buildingID)
	if e == nil {
		return ""
	}
	v, _ := stringExtra(e, "owner")
	return v
}

func (svc *service) IsLocked(w syscore.World, buildingID string) bool {
	e := w.EntityByID(buildingID)
	if e == nil {
		return false
	}
	v, _ := boolExtra(e, "locked")
	return v
}

func (svc *service) CanEnter(w syscore.World, entityID, buildingID string) bool {
	e := w.EntityByID(buildingID)
	if e == nil {
		return false
	}
	if locked, _ := boolExtra(e, "locked"); !locked {
		return true
	}
	return hasEntryRight(e, entityID)
}

// === helpers ===

func boolExtra(e syscore.Entity, k string) (bool, bool) {
	v, ok := e.GetExtra(k)
	if !ok {
		return false, false
	}
	b, _ := v.(bool)
	return b, true
}

func stringExtra(e syscore.Entity, k string) (string, bool) {
	v, ok := e.GetExtra(k)
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, true
}

func hasEntryRight(building syscore.Entity, entityID string) bool {
	if owner, _ := stringExtra(building, "owner"); owner == entityID {
		return true
	}
	v, ok := building.GetExtra("access")
	if !ok {
		return false
	}
	switch x := v.(type) {
	case []string:
		for _, id := range x {
			if id == entityID {
				return true
			}
		}
	case []any:
		for _, item := range x {
			if s, ok := item.(string); ok && s == entityID {
				return true
			}
		}
	}
	return false
}

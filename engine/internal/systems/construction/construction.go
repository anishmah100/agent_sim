// Package construction — composable Construction system.
//
// Minimal viable engine-side construction. The user-facing
// Townscaper-style procedural geometry lives in the Pixi renderer +
// component library (separate work). This system owns the *rules*:
// blueprints exist as entities of archetype="blueprint" until built,
// then transform into archetype="building" owned by the constructor.
//
// Verbs:
//   - place_blueprint:  place a blueprint at an adjacent free tile,
//     pays the initial-materials cost up front.
//   - advance_construction: spend the next batch of materials to push
//     progress toward 100. Completes the blueprint
//     into a building when it hits 100.
//   - demolish:         remove an owned blueprint OR building. (No material
//     refund in v1 — audit [34] corrected this doc; the
//     handler removes the entity without refunding.)
//
// State on blueprint entities:
//   - kind:        which blueprint shape ("cottage" / "shed" / ...).
//   - owner:       entity id of the constructor.
//   - progress:    0..100 percent complete.
//   - cost_per_step: int — wood + stone consumed per advance.
//   - materials_remaining: estimate of total materials still needed.
//
// Once progress hits 100, the entity's archetype flips to "building"
// and the property system's seed defaults apply on the next observation.
package construction

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/inventory"
)

// Blueprint catalog — engine-level, hand-coded for v0. A scenario can
// declare its own blueprints by registering more entries here.
type BlueprintDef struct {
	Kind             string   `json:"kind"`
	Description      string   `json:"description"`
	InitialMaterials []string `json:"initial_materials"`
	AdvanceMaterials []string `json:"advance_materials"`
	StepsToComplete  int      `json:"steps_to_complete"`
	FootprintW       int      `json:"footprint_w,omitempty"`
	FootprintH       int      `json:"footprint_h,omitempty"`
}

var defaultBlueprints = map[string]BlueprintDef{
	"cottage": {
		Kind:             "cottage",
		Description:      "Small one-room dwelling.",
		InitialMaterials: []string{"wood", "wood", "stone"},
		AdvanceMaterials: []string{"wood", "stone"},
		StepsToComplete:  4,
		FootprintW:       2, FootprintH: 2,
	},
	"shed": {
		Kind:             "shed",
		Description:      "Storage hut. Cheap, fast.",
		InitialMaterials: []string{"wood"},
		AdvanceMaterials: []string{"wood"},
		StepsToComplete:  2,
		FootprintW:       1, FootprintH: 1,
	},
}

// === Events ===

type ConstructionStarted struct {
	Builder, Blueprint string
	BlueprintKind      string
}

func (ConstructionStarted) Kind() string { return "ConstructionStarted" }

type ConstructionAdvanced struct {
	Builder, Blueprint string
	NewProgress        int
}

func (ConstructionAdvanced) Kind() string { return "ConstructionAdvanced" }

type ConstructionCompleted struct {
	Builder, BuildingID string
	BlueprintKind       string
}

func (ConstructionCompleted) Kind() string { return "ConstructionCompleted" }

type Demolished struct {
	By, Target string
	WasBuilt   bool
}

func (Demolished) Kind() string { return "Demolished" }

var (
	_ eventbus.Event = ConstructionStarted{}
	_ eventbus.Event = ConstructionAdvanced{}
	_ eventbus.Event = ConstructionCompleted{}
	_ eventbus.Event = Demolished{}
)

// === System ===

type System struct {
	bp        map[string]BlueprintDef
	idCounter uint64
}

func New() *System {
	return &System{bp: defaultBlueprints}
}

func (s *System) Name() string { return "construction" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("place_blueprint", s.handlePlace)
	r.Verb("advance_construction", s.handleAdvance)
	r.Verb("demolish", s.handleDemolish)
	r.Manifest(s.manifest())
}

// === Verb handlers ===

func (s *System) handlePlace(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Kind string `json:"kind"`
		At   [2]int `json:"at"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	def, ok := s.bp[p.Kind]
	if !ok {
		res.Reason = "unknown_blueprint"
		return res
	}
	if w.Chebyshev(e.Pos(), p.At) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	if !w.IsWalkable(p.At) {
		res.Reason = "unwalkable"
		return res
	}
	// AUDIT FIX (medium/[10]): reject placing on a tile already held by a
	// non-item entity (another blueprint/building/agent). IsWalkable doesn't
	// catch blueprints (they don't mark the tile unwalkable), so blueprints
	// could stack on each other / on agents.
	for _, oid := range w.EntitiesInRadius(p.At, 0) {
		if o := w.EntityByID(oid); o != nil && o.Archetype() != "item" {
			res.Reason = "occupied"
			return res
		}
	}

	inv, ok := w.GetService("inventory").(inventory.InventoryService)
	if !ok {
		res.Reason = "no_inventory_service"
		return res
	}
	// Resolve abstract material kinds (e.g. "wood") to actual item IDs
	// in the actor's inventory.
	mintedKinds := def.InitialMaterials
	itemIDs, ok := resolveMaterials(inv, w, e.ID(), mintedKinds)
	if !ok {
		res.Reason = "missing_materials"
		return res
	}
	if ok, reason := inv.Consume(w, e.ID(), itemIDs); !ok {
		res.Reason = reason
		return res
	}

	// AUDIT FIX (high/[2]): use a monotonic counter, not kind+tick+builder.
	// Two place_blueprint actions of the same kind by the same agent in one
	// tick produced identical ids; SpawnEntity has no collision guard, so the
	// second silently clobbered the first (materials paid for both, one lost).
	id := fmt.Sprintf("bp_%s_%d", p.Kind, atomic.AddUint64(&s.idCounter, 1))
	_, err := w.SpawnEntityFromSpec(syscore.EntitySpec{
		ID:        id,
		Archetype: "blueprint",
		Pos:       p.At,
		Extras: map[string]any{
			"kind":              p.Kind,
			"owner":             e.ID(),
			"progress":          0,
			"steps_total":       def.StepsToComplete,
			"steps_done":        0,
			"advance_materials": def.AdvanceMaterials,
		},
	})
	if err != nil {
		res.Reason = "spawn_failed"
		return res
	}
	w.QueueEvent(ConstructionStarted{Builder: e.ID(), Blueprint: id, BlueprintKind: p.Kind})
	res.Accepted = true
	return res
}

func (s *System) handleAdvance(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	bp := w.EntityByID(p.Target)
	if bp == nil {
		res.Reason = "unknown_target"
		return res
	}
	if bp.Archetype() != "blueprint" {
		res.Reason = "not_a_blueprint"
		return res
	}
	if w.Chebyshev(e.Pos(), bp.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	owner, _ := stringExtra(bp, "owner")
	if owner != e.ID() {
		res.Reason = "not_owner"
		return res
	}

	advanceKinds := stringSlice(bp, "advance_materials")
	stepsTotal := intExtra(bp, "steps_total")
	if stepsTotal <= 0 {
		res.Reason = "broken_blueprint"
		return res
	}

	// AUDIT FIX (medium/[11]): guard the service assertion (was an unchecked
	// type assertion → nil-deref panic if inventory isn't registered).
	inv, ok := w.GetService("inventory").(inventory.InventoryService)
	if !ok {
		res.Reason = "no_inventory_service"
		return res
	}
	itemIDs, ok := resolveMaterials(inv, w, e.ID(), advanceKinds)
	if !ok {
		res.Reason = "missing_materials"
		return res
	}
	if ok, reason := inv.Consume(w, e.ID(), itemIDs); !ok {
		res.Reason = reason
		return res
	}

	stepsDone := intExtra(bp, "steps_done") + 1
	progress := (stepsDone * 100) / stepsTotal
	if progress > 100 {
		progress = 100
	}
	w.MutateEntity(p.Target, func(real syscore.Entity) {
		real.SetExtra("steps_done", stepsDone)
		real.SetExtra("progress", progress)
		if progress >= 100 {
			// Transform into a building. Property system seeds owner/locked/
			// access on its next OnEntitySpawn opportunity; the engine
			// doesn't re-fire OnEntitySpawn on archetype change, so we
			// seed the property extras inline here.
			real.SetExtra("locked", false)
			real.SetExtra("access", []string{})
			// Owner already set on the blueprint; preserve it.
		}
	})
	w.QueueEvent(ConstructionAdvanced{Builder: e.ID(), Blueprint: p.Target, NewProgress: progress})
	if progress >= 100 {
		// We can't change archetype via the syscore.Entity interface;
		// drop the blueprint and spawn a building entity at the same tile.
		kind, _ := stringExtra(bp, "kind")
		pos := bp.Pos()
		buildingID := "bld_" + p.Target
		_ = w.RemoveEntity(p.Target)
		_, _ = w.SpawnEntityFromSpec(syscore.EntitySpec{
			ID:        buildingID,
			Archetype: "building",
			Pos:       pos,
			Extras: map[string]any{
				"owner":    e.ID(),
				"locked":   false,
				"access":   []string{},
				"kind":     kind,
				"built_at": w.Tick(),
			},
		})
		w.QueueEvent(ConstructionCompleted{Builder: e.ID(), BuildingID: buildingID, BlueprintKind: kind})
	}
	res.Accepted = true
	return res
}

func (s *System) handleDemolish(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	target := w.EntityByID(p.Target)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if target.Archetype() != "blueprint" && target.Archetype() != "building" {
		res.Reason = "not_a_structure"
		return res
	}
	if w.Chebyshev(e.Pos(), target.Pos()) > 1 {
		res.Reason = "target_too_far"
		return res
	}
	owner, _ := stringExtra(target, "owner")
	if owner != e.ID() {
		res.Reason = "not_owner"
		return res
	}
	wasBuilt := target.Archetype() == "building"
	_ = w.RemoveEntity(p.Target)
	w.QueueEvent(Demolished{By: e.ID(), Target: p.Target, WasBuilt: wasBuilt})
	res.Accepted = true
	return res
}

// === Manifest ===

func (s *System) manifest() manifest.SystemDeclaration {
	bps := make([]manifest.ArchetypeDecl, 0, len(s.bp)+1)
	bps = append(bps, manifest.ArchetypeDecl{
		Archetype:        "blueprint",
		Description:      "Construction-in-progress entity. Owned by the builder, becomes a 'building' when progress hits 100.",
		DefaultVerbsUsed: []string{"advance_construction", "demolish"},
	})
	for kind, def := range s.bp {
		extras, _ := json.Marshal(map[string]any{
			"steps_to_complete": def.StepsToComplete,
			"initial_materials": def.InitialMaterials,
			"advance_materials": def.AdvanceMaterials,
			"footprint":         [2]int{def.FootprintW, def.FootprintH},
		})
		bps = append(bps, manifest.ArchetypeDecl{
			Archetype:     "blueprint:" + kind,
			Description:   def.Description,
			DefaultExtras: extras,
		})
	}
	return manifest.SystemDeclaration{
		Name:        "construction",
		Description: "Build blueprints into buildings by spending materials over time. Pairs with resources (materials) + property (post-build ownership).",
		Verbs: []manifest.VerbDeclaration{
			{Verb: "place_blueprint",
				Description:      "Place a blueprint at an adjacent walkable tile. Pays the initial-materials cost up front.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string"},"at":{"type":"array","items":{"type":"integer"},"minItems":2,"maxItems":2}},"required":["kind","at"]}`),
				Preconditions:    []string{"`at` within 1 tile of self", "`at` is walkable", "self has the initial_materials for this kind"},
				RejectionReasons: []string{"bad_params", "unknown_blueprint", "target_too_far", "unwalkable", "occupied", "no_inventory_service", "missing_materials", "spawn_failed"},
				EmitsEvents:      []string{"ConstructionStarted"},
			},
			{Verb: "advance_construction",
				Description:      "Advance an owned adjacent blueprint by one step; consumes one advance_materials batch. Completes the blueprint when progress reaches 100.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target within 1 tile", "target is a blueprint owned by self", "self has the advance_materials"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_blueprint", "target_too_far", "not_owner", "broken_blueprint", "missing_materials", "no_inventory_service"},
				EmitsEvents:      []string{"ConstructionAdvanced", "ConstructionCompleted"},
			},
			{Verb: "demolish",
				Description:      "Remove an owned blueprint OR building.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}`),
				Preconditions:    []string{"target within 1 tile", "self is the owner"},
				RejectionReasons: []string{"bad_params", "unknown_target", "not_a_structure", "target_too_far", "not_owner"},
				EmitsEvents:      []string{"Demolished"},
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "progress", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "0-100 percent complete on a blueprint entity"},
			{Key: "steps_done", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "internal counter — incremented per advance_construction"},
			{Key: "steps_total", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: true, Meaning: "total steps from blueprint to completion"},
		},
		Archetypes: bps,
	}
}

// === helpers ===

// resolveMaterials walks the actor's inventory matching item-IDs by their
// KIND against the requested material kinds. AUDIT FIX (high/[3]): the old
// version matched by raw string prefix, which only worked for the (since-
// removed) "wood_<tick>" resource ids — it could NOT match canonical
// "item:wood#<id>" items (picked up, given, or — after fix [9] — harvested),
// so construction silently ignored most wood/stone in inventory. Now both the
// canonical and any legacy id resolve to their kind first.
func resolveMaterials(inv inventory.InventoryService, w syscore.World, entityID string, want []string) ([]string, bool) {
	have := inv.Items(w, entityID)
	used := make(map[int]bool)
	out := make([]string, 0, len(want))
	for _, kind := range want {
		found := -1
		for i, id := range have {
			if used[i] {
				continue
			}
			if kindOfItem(id) == kind {
				found = i
				break
			}
		}
		if found < 0 {
			return nil, false
		}
		used[found] = true
		out = append(out, have[found])
	}
	return out, true
}

// kindOfItem extracts the canonical kind from an inventory id. Handles the
// canonical "item:<kind>#<unique>" form and any legacy "<kind>_<...>" form.
func kindOfItem(id string) string {
	if len(id) > 5 && id[:5] == "item:" {
		id = id[5:]
	}
	for i := 0; i < len(id); i++ {
		if id[i] == '#' || id[i] == '_' {
			return id[:i]
		}
	}
	return id
}

func stringExtra(e syscore.Entity, k string) (string, bool) {
	v, ok := e.GetExtra(k)
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, true
}

func intExtra(e syscore.Entity, k string) int {
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

func stringSlice(e syscore.Entity, k string) []string {
	v, ok := e.GetExtra(k)
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

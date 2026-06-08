// Package quests — composable Quest / objective system.
//
// Quests are declarative goals attached to an entity. A scenario (or
// another system) writes them into extras.quests; this system checks
// them every QuestCheckInterval ticks, applies rewards on completion,
// and prunes done quests.
//
// Quest shape:
//
//	{
//	  "id":       "tutorial_walk",
//	  "title":    "Take ten steps.",
//	  "kind":     "walk_distance" | "reach_tile" | "gather_gold" | "kill_target",
//	  "target":   quest-specific param (tile / amount / entity id)
//	  "progress": current value (set by this system)
//	  "goal":     required value
//	  "reward":   {"gold": int, "item": string, "hp": int}
//	  "done":     bool
//	}
//
// The verbal-contract layer (propose_task / accept_task) is separate;
// see the verbalquests system once it lands.
package quests

import (
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

const QuestCheckInterval = 60 // ticks (1s @ 60Hz)

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "quests" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.OnTick(s.tick)
	r.Manifest(s.manifest())
}

func (s *System) tick(w syscore.World, tick uint64) {
	if tick%QuestCheckInterval != 0 {
		return
	}
	for _, id := range w.EntityIDs() {
		e := w.EntityByID(id)
		if e == nil {
			continue
		}
		raw, ok := e.GetExtra("quests")
		if !ok {
			continue
		}
		qs := asQuests(raw)
		if len(qs) == 0 {
			continue
		}
		anyDone := false
		for _, q := range qs {
			if asBool(q["done"]) {
				continue
			}
			s.advance(w, e, q)
			if asBool(q["done"]) {
				anyDone = true
				s.reward(w, e.ID(), asMap(q["reward"]))
			}
		}
		if anyDone {
			fresh := make([]any, 0, len(qs))
			for _, q := range qs {
				if !asBool(q["done"]) {
					fresh = append(fresh, q)
				}
			}
			w.MutateEntity(e.ID(), func(real syscore.Entity) {
				real.SetExtra("quests", fresh)
			})
		}
	}
}

func (s *System) advance(w syscore.World, e syscore.Entity, q map[string]any) {
	switch q["kind"] {
	case "reach_tile":
		tgt := asIntPair(q["target"])
		pos := e.Pos()
		if tgt[0] == pos[0] && tgt[1] == pos[1] {
			q["done"] = true
		}
	case "gather_gold":
		need := asInt(q["goal"])
		v, _ := e.GetExtra("gold")
		have := asInt(v)
		q["progress"] = have
		if have >= need {
			q["done"] = true
		}
	case "kill_target":
		// AUDIT FIX (high/[4]): absence is ambiguous — a corpse is removed
		// on death (legit kill) BUT a bogus/never-spawned target id is also
		// absent. The old code completed on ANY absence, handing out a free
		// reward for a nonexistent target. Only credit a kill once the target
		// has been observed ALIVE at least once (target_seen): "gone after
		// being seen" = killed, while "never existed" never completes.
		tid, _ := q["target"].(string)
		other := w.EntityByID(tid)
		if other == nil {
			if seen, _ := q["target_seen"].(bool); seen {
				q["done"] = true
			}
			return
		}
		q["target_seen"] = true
		hpV, _ := other.GetExtra("hp")
		if asInt(hpV) <= 0 {
			q["done"] = true
		}
	case "walk_distance":
		v, _ := e.GetExtra("steps")
		steps := asInt(v)
		need := asInt(q["goal"])
		q["progress"] = steps
		if steps >= need {
			q["done"] = true
		}
	}
}

func (s *System) reward(w syscore.World, entityID string, r map[string]any) {
	if r == nil {
		return
	}
	w.MutateEntity(entityID, func(real syscore.Entity) {
		if g := asInt(r["gold"]); g > 0 {
			cur, _ := real.GetExtra("gold")
			real.SetExtra("gold", asInt(cur)+g)
		}
		if hp := asInt(r["hp"]); hp > 0 {
			curV, _ := real.GetExtra("hp")
			maxV, _ := real.GetExtra("max_hp")
			cur, max := asInt(curV), asInt(maxV)
			n := cur + hp
			if n > max {
				n = max
			}
			real.SetExtra("hp", n)
		}
		if item, ok := r["item"].(string); ok && item != "" {
			invV, _ := real.GetExtra("inventory")
			inv := asStrings(invV)
			inv = append(inv, item)
			real.SetExtra("inventory", inv)
		}
	})
}

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "quests",
		Description: "Declarative goals attached to an entity. Checked every second; rewards gold/HP/item on completion.",
		StateFields: []manifest.StateFieldDecl{
			{Key: "quests", Type: "list", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "list of pending quest objects on this entity (kind/goal/progress/reward)"},
			{Key: "steps", Type: "int", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "steps walked counter; read by walk_distance quests"},
		},
	}
}

// === helpers ===

func asQuests(v any) []map[string]any {
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, q := range x {
			if m, ok := q.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func asInt(v any) int {
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

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asIntPair(v any) [2]int {
	switch x := v.(type) {
	case []any:
		if len(x) >= 2 {
			return [2]int{asInt(x[0]), asInt(x[1])}
		}
	case [2]int:
		return x
	}
	return [2]int{}
}

func asStrings(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, s := range x {
			if str, ok := s.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

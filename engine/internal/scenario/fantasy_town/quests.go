package fantasy_town

import (
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// Quest / objective system.
//
// Quests are declarative goals attached to an entity. The scenario
// checks them every Nth tick. When a quest completes, the entity is
// rewarded (gold/item/HP) and the quest is removed from extras.quests.
//
// A Quest is a map with shape:
//   {
//     "id":        "tutorial_walk",
//     "title":     "Take ten steps.",
//     "kind":      "walk_distance" | "reach_tile" | "deliver_item"
//                  | "kill_target" | "gather_gold",
//     "target":    quest-specific param (tile / item / amount / entity)
//     "progress":  current
//     "goal":      required
//     "reward":    {"gold": 10, "item": "..."}
//     "done":      bool
//   }

const QuestCheckInterval = 60 // ticks (1 sec)

func (s *FantasyTown) tickQuests(w *world.World, tick uint64) {
	if tick%QuestCheckInterval != 0 {
		return
	}
	for _, id := range w.EntityIDsUnlocked() {
		e := w.EntityByIDUnlocked(id)
		if e == nil {
			continue
		}
		quests := extrasMaps(e.Extras, "quests")
		if len(quests) == 0 {
			continue
		}
		anyDone := false
		for _, q := range quests {
			if asBool(q["done"]) {
				continue
			}
			s.advanceQuest(w, e, q)
			if asBool(q["done"]) {
				anyDone = true
				s.applyReward(w, e, asMap(q["reward"]))
			}
		}
		if anyDone {
			// Prune completed quests.
			fresh := make([]map[string]any, 0, len(quests))
			for _, q := range quests {
				if !asBool(q["done"]) {
					fresh = append(fresh, q)
				}
			}
			w.MutateEntity(e.EntityID, func(real *world.Entity) {
				real.Extras["quests"] = anySliceOfMaps(fresh)
			})
		}
	}
}

func (s *FantasyTown) advanceQuest(w *world.World, e *world.Entity, q map[string]any) {
	kind, _ := q["kind"].(string)
	switch kind {
	case "reach_tile":
		tgt := asIntPair(q["target"])
		if tgt[0] == e.LogicalTile[0] && tgt[1] == e.LogicalTile[1] {
			q["done"] = true
		}
	case "gather_gold":
		need := asInt(q["goal"])
		have := extrasInt(e.Extras, "gold")
		q["progress"] = have
		if have >= need {
			q["done"] = true
		}
	case "kill_target":
		tgt, _ := q["target"].(string)
		other := w.EntityByIDUnlocked(tgt)
		if other == nil || extrasInt(other.Extras, "hp") <= 0 {
			q["done"] = true
		}
	case "walk_distance":
		// Tracks a "steps" counter the scenario bumps on each completed
		// move. Stored on the entity as `extras.steps`.
		steps := extrasInt(e.Extras, "steps")
		need := asInt(q["goal"])
		q["progress"] = steps
		if steps >= need {
			q["done"] = true
		}
	}
}

func (s *FantasyTown) applyReward(w *world.World, e *world.Entity, r map[string]any) {
	if r == nil {
		return
	}
	w.MutateEntity(e.EntityID, func(real *world.Entity) {
		if g := asInt(r["gold"]); g > 0 {
			real.Extras["gold"] = extrasInt(real.Extras, "gold") + g
		}
		if hp := asInt(r["hp"]); hp > 0 {
			cur := extrasInt(real.Extras, "hp")
			max := extrasInt(real.Extras, "max_hp")
			new := cur + hp
			if new > max {
				new = max
			}
			real.Extras["hp"] = new
		}
		if item, ok := r["item"].(string); ok && item != "" {
			inv := extrasStringSlice(real.Extras, "inventory")
			inv = append(inv, item)
			real.Extras["inventory"] = inv
		}
	})
}

// === helpers ===

func extrasMaps(m map[string]any, k string) []map[string]any {
	v, ok := m[k]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, e := range x {
			if mm, ok := e.(map[string]any); ok {
				out = append(out, mm)
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

func anySliceOfMaps(ms []map[string]any) []any {
	out := make([]any, len(ms))
	for i, m := range ms {
		out[i] = m
	}
	return out
}

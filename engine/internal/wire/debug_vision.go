package wire

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// DebugVisionHandler — /api/v1/debug/vision?x=X&y=Y&entity=ID.
// Returns what an observation builder produces for a synthetic
// probe placed at (X, Y), OR (when entity= is set) what THAT
// entity's actual obs returns. Used to diagnose D8 routing live
// without registering an SDK agent and risking that the SDK
// itself drops items.
func DebugVisionHandler(w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		q := r.URL.Query()
		if eid := q.Get("entity"); eid != "" {
			obs := w.BuildObservationFor(eid, 1, nil)
			if obs == nil {
				http.Error(rw, `{"error":"unknown entity"}`, 404)
				return
			}
			_ = json.NewEncoder(rw).Encode(map[string]any{
				"source":     "entity",
				"entity":     eid,
				"pos":        obs.Self.Pos,
				"v_items":    len(obs.VisibleItems),
				"v_entities": len(obs.VisibleEntities),
				"items":      obs.VisibleItems,
				"entities_archetypes": archetypeCounts(obs.VisibleEntities),
			})
			return
		}
		xs := q.Get("x")
		ys := q.Get("y")
		if xs == "" || ys == "" {
			http.Error(rw, `{"error":"x and y required"}`, 400)
			return
		}
		x, err1 := strconv.Atoi(xs)
		y, err2 := strconv.Atoi(ys)
		if err1 != nil || err2 != nil {
			http.Error(rw, `{"error":"x/y must be integers"}`, 400)
			return
		}
		obs := w.DebugObsAtTile([2]int{x, y})
		if obs == nil {
			http.Error(rw, `{"error":"no snapshot"}`, 503)
			return
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"source":     "synthetic",
			"pos":        obs.Self.Pos,
			"v_items":    len(obs.VisibleItems),
			"v_entities": len(obs.VisibleEntities),
			"items":      obs.VisibleItems,
			"entities_archetypes": archetypeCounts(obs.VisibleEntities),
		})
	}
}

func archetypeCounts(ents []world.VisibleEntityState) map[string]int {
	out := map[string]int{}
	for _, e := range ents {
		out[e.Archetype]++
	}
	return out
}

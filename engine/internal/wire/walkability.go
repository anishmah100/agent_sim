package wire

import (
	"encoding/json"
	"net/http"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// WalkabilityHandler — GET /api/v1/world/walkability. Returns the static
// walkability grid so agents can run their own A* navigation (the agent
// owns navigation; the engine executes only single-tile steps). Fetched
// ONCE at agent startup — terrain is static.
func WalkabilityHandler(w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		width, height, rows := w.WalkabilityRows()
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"width": width, "height": height, "rows": rows,
		})
	}
}

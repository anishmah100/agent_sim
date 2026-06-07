package wire

import (
	"encoding/json"
	"net/http"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// SocialHandler — GET /api/v1/social. Returns the full social-ledger
// graph (every interaction pair once) for the frontend's Society-Pulse
// relationship overlay. Cheap; the ledger is small (pairs that have
// actually interacted).
func SocialHandler(w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		edges := w.SocialEdges()
		if edges == nil {
			edges = []world.SocialEdge{}
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{"edges": edges})
	}
}

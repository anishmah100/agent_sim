package wire

import (
	"encoding/json"
	"net/http"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// AffordanceManifestHandler serves the aggregated affordance manifest
// at /api/v1/world/affordances.
//
// Single source of truth for: which verbs exist, what their params
// look like, which states they read/write, which sounds they emit.
// Bots fetch this at register time (validation + discovery); the UI's
// World Rulebook page renders directly from it. The aggregator is
// frozen by the time engine boot completes, so this handler is
// lock-free reads of immutable data.
func AffordanceManifestHandler(host *world.SystemHost) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Cache-Control", "public, max-age=60")
		if host == nil || host.Aggregator == nil {
			http.Error(rw, `{"error":"no scenario installed"}`, http.StatusServiceUnavailable)
			return
		}
		enc := json.NewEncoder(rw)
		enc.SetIndent("", "  ")
		_ = enc.Encode(host.Aggregator.Build())
	}
}

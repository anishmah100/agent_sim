package historian

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler serves /api/v1/world/history. Query params:
//   - since (uint): only return events with tick >= since (default 0).
//   - limit (int):  cap the response size (default = ring capacity).
//
// Response shape:
//
//	{
//	  "stats":  {"total_emitted": ..., "in_ring": ..., "capacity": ...},
//	  "events": [{"tick": ..., "seq": ..., "kind": "...", "payload": {...}}, ...]
//	}
//
// Events are returned in chronological ascending order. Cold consumers
// (the autoresearch loop, the historian summarizer) page through by
// polling with `since` set to the last seen tick+1.
func Handler(h *Historian) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		if h == nil {
			http.Error(rw, `{"error":"no historian"}`, http.StatusServiceUnavailable)
			return
		}
		since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		events := h.Recent(since, limit)
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"stats":  h.Stats(),
			"events": events,
		})
	}
}

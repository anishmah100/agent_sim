package wire

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

type LeaderRow struct {
	EntityID string `json:"entity_id"`
	Label    string `json:"label"`
	Metric   int    `json:"metric"`
}

// LeaderboardsHandler serves /api/v1/leaderboards?board=richest|kills|relationships.
func LeaderboardsHandler(w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		board := r.URL.Query().Get("board")
		if board == "" {
			board = "richest"
		}
		var rows []LeaderRow
		for _, id := range w.EntityIDs() {
			e := w.EntityByID(id)
			if e == nil {
				continue
			}
			var metric int
			switch board {
			case "richest":
				if v, ok := e.Extras["gold"].(int); ok {
					metric = v
				}
			case "kills":
				if v, ok := e.Extras["kills"].(int); ok {
					metric = v
				}
			case "relationships":
				if v, ok := e.Extras["relationships"].(map[string]any); ok {
					metric = len(v)
				}
			}
			rows = append(rows, LeaderRow{
				EntityID: e.EntityID,
				Label:    labelOf(e),
				Metric:   metric,
			})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Metric > rows[j].Metric })
		if len(rows) > 50 {
			rows = rows[:50]
		}
		rw.Header().Set("content-type", "application/json")
		rw.Header().Set("access-control-allow-origin", "*")
		_ = json.NewEncoder(rw).Encode(rows)
	}
}

func labelOf(e *world.Entity) string {
	if e.DisplayName != "" {
		return e.DisplayName
	}
	return e.EntityID
}

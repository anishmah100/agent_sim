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

// LeaderboardsHandler serves /api/v1/leaderboards?board=<name>.
//
// Boards:
//   - richest:   extras.gold (money system)
//   - kills:     extras.kills (combat — credited on EntityDied)
//   - buildings: count of buildings where extras.owner == this entity
//                (property system)
//   - contracts: count of completed verbal contracts this entity took
//                part in (verbalquests system)
//
// These map 1:1 to the systems documented in the affordance manifest,
// so a researcher comparing models can read "grok pulls ahead on
// buildings" or "claude leads on contracts" and trace it back to the
// system rules.
func LeaderboardsHandler(w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		board := r.URL.Query().Get("board")
		if board == "" {
			board = "richest"
		}
		rows := computeBoard(w, board)
		sort.Slice(rows, func(i, j int) bool { return rows[i].Metric > rows[j].Metric })
		if len(rows) > 50 {
			rows = rows[:50]
		}
		rw.Header().Set("content-type", "application/json")
		rw.Header().Set("access-control-allow-origin", "*")
		_ = json.NewEncoder(rw).Encode(rows)
	}
}

func computeBoard(w *world.World, board string) []LeaderRow {
	ids := w.EntityIDs()
	switch board {
	case "buildings":
		// Count buildings per owner. Skip the buildings themselves.
		count := make(map[string]int)
		ownerLabel := make(map[string]string)
		for _, id := range ids {
			e := w.EntityByID(id)
			if e == nil {
				continue
			}
			if e.Archetype != "building" {
				continue
			}
			owner, _ := e.Extras["owner"].(string)
			if owner == "" {
				continue
			}
			count[owner]++
		}
		// Resolve labels.
		for ownerID := range count {
			if e := w.EntityByID(ownerID); e != nil {
				ownerLabel[ownerID] = labelOf(e)
			} else {
				ownerLabel[ownerID] = ownerID
			}
		}
		out := make([]LeaderRow, 0, len(count))
		for owner, c := range count {
			out = append(out, LeaderRow{EntityID: owner, Label: ownerLabel[owner], Metric: c})
		}
		return out

	case "contracts":
		// Number of "completed" contracts an entity participated in
		// (proposer or target). Each contract sits on BOTH parties'
		// ledgers, so we count once per entity ledger.
		var rows []LeaderRow
		for _, id := range ids {
			e := w.EntityByID(id)
			if e == nil {
				continue
			}
			cs, _ := e.Extras["contracts"].([]any)
			n := 0
			for _, c := range cs {
				m, _ := c.(map[string]any)
				if m == nil {
					continue
				}
				if s, _ := m["status"].(string); s == "completed" {
					n++
				}
			}
			if n == 0 && e.Extras["contracts"] == nil {
				// Entity doesn't participate in the contract system at
				// all — skip rather than emit a 0-metric row.
				continue
			}
			rows = append(rows, LeaderRow{EntityID: id, Label: labelOf(e), Metric: n})
		}
		return rows

	default: // "richest" + "kills" + any future int-extras-keyed board.
		var rows []LeaderRow
		for _, id := range ids {
			e := w.EntityByID(id)
			if e == nil {
				continue
			}
			var metric int
			switch board {
			case "richest":
				metric = asInt(e.Extras["gold"])
			case "kills":
				metric = asInt(e.Extras["kills"])
			}
			rows = append(rows, LeaderRow{EntityID: id, Label: labelOf(e), Metric: metric})
		}
		return rows
	}
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

func labelOf(e *world.Entity) string {
	if e.DisplayName != "" {
		return e.DisplayName
	}
	return e.EntityID
}

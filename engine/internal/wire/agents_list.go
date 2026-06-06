package wire

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// AgentsListHandler — GET /api/v1/agents
//
// Returns the list of LLM/SDK-connected agents in the world, with the
// info the UI's agent-picker drawer needs to render + jump-to-focus:
//   - entity_id (engine id)
//   - persona name (from the registration blob)
//   - bound entity archetype + display_name
//   - current pos (from the latest snapshot)
//   - ms_connected (uptime)
//
// "Connected agents" means anything that registered via
// /api/v1/agent/register AND currently has an open WS. The 250
// background NPCs the engine ships in Eldoria don't show up here —
// only the agents YOU spawned (Qwen, heuristic_bot, manual joins).
func AgentsListHandler(hub *AgentHub, w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		hub.mu.Lock()
		conns := make([]*agentConn, 0, len(hub.live))
		for _, c := range hub.live {
			conns = append(conns, c)
		}
		hub.mu.Unlock()

		snap := w.Snapshot()
		entityPos := make(map[string][2]int, len(snap.Entities))
		entityKind := make(map[string]string, len(snap.Entities))
		entityName := make(map[string]string, len(snap.Entities))
		for _, e := range snap.Entities {
			entityPos[e.EntityID] = [2]int{e.LogicalTile[0], e.LogicalTile[1]}
			entityKind[e.EntityID] = e.Archetype
			entityName[e.EntityID] = e.DisplayName
		}

		now := nowMs()
		out := make([]agentInfo, 0, len(conns))
		for _, c := range conns {
			info := agentInfo{
				AgentID:      c.rec.AgentID,
				EntityID:     c.rec.EntityID,
				MsConnected:  now - c.rec.ConnectedAt,
				Archetype:    entityKind[c.rec.EntityID],
				DisplayName:  entityName[c.rec.EntityID],
			}
			if p := c.rec.Persona; p != nil {
				if name, ok := p["name"].(string); ok {
					info.PersonaName = name
				}
				if bio, ok := p["bio"].(string); ok {
					info.Bio = bio
				}
			}
			if pos, ok := entityPos[c.rec.EntityID]; ok {
				info.Pos = pos
			}
			out = append(out, info)
		}
		// Sort by persona name then entity id so the picker is stable.
		sort.Slice(out, func(i, j int) bool {
			if out[i].PersonaName != out[j].PersonaName {
				return out[i].PersonaName < out[j].PersonaName
			}
			return out[i].EntityID < out[j].EntityID
		})

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(agentsListResp{Agents: out})
	}
}

type agentInfo struct {
	AgentID     string `json:"agent_id"`
	EntityID    string `json:"entity_id"`
	PersonaName string `json:"persona_name,omitempty"`
	Bio         string `json:"bio,omitempty"`
	Archetype   string `json:"archetype,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Pos         [2]int `json:"pos"`
	MsConnected int64  `json:"ms_connected"`
}

type agentsListResp struct {
	Agents []agentInfo `json:"agents"`
}

func nowMs() int64 { return time.Now().UnixMilli() }

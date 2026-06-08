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
			// Skip husks: a connection whose bound entity is no longer in
			// the world snapshot is a dead/removed agent. Its entity was
			// dropped on death, so it reports pos (0,0) — and the picker's
			// jump-to-focus would fling the camera to the world's top-left
			// corner ("Bram the cautious takes me to a weird spawn at 0,0").
			// The supervisor respawns the persona as a fresh live entity,
			// which DOES appear in the snapshot, so the live one still
			// lists. Inside-building agents stay in the snapshot, so they
			// are not affected.
			if _, alive := entityPos[c.rec.EntityID]; !alive {
				continue
			}
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
				// IsLLM was declared but never set, so the frontend badge
				// showed every agent as rule-based. The registrar tags an
				// LLM brain with persona.archetype_tag == "llm"; rule-based
				// bots tag their archetype name (survivor/killer/…). Treat
				// anything explicitly tagged non-"llm" as rule-based, and
				// default unknown/manual joins to LLM (the original intent:
				// "anything not a known heuristic bot is assumed LLM").
				if tag, ok := p["archetype_tag"].(string); ok {
					info.IsLLM = tag == "llm"
				} else {
					info.IsLLM = true
				}
				// Brain — which model drives an LLM agent (qwen / claude),
				// or "rule" for heuristic bots. The runner stamps
				// persona["brain"] on register; fall back sensibly when
				// it's absent so older runs still render a badge.
				if brain, ok := p["brain"].(string); ok && brain != "" {
					info.Brain = brain
				} else if info.IsLLM {
					info.Brain = "llm"
				} else {
					info.Brain = "rule"
				}
			}
			if pos, ok := entityPos[c.rec.EntityID]; ok {
				info.Pos = pos
			}
			// Copy the picker telemetry (set under infoMu on the action
			// path). These were declared but never copied here, so the UI
			// always saw empty last_verb/last_speech and couldn't show what
			// an agent was doing — making slow LLM agents look frozen.
			c.rec.infoMu.Lock()
			info.LastVerb = c.rec.LastVerb
			info.LastSpeech = c.rec.LastSpeech
			c.rec.infoMu.Unlock()
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
	// IsLLM — heuristic-bot agents register with a known bio.
	// Anything else is assumed LLM-driven.
	IsLLM       bool   `json:"is_llm"`
	// Brain — model driving the agent: "qwen" / "claude" for LLM
	// agents, "rule" for heuristic bots, "llm" when an LLM agent
	// didn't stamp a specific model. The UI badge reads this.
	Brain       string `json:"brain,omitempty"`
	// LastVerb — last action verb the agent submitted (any verb,
	// accepted or not). Surfaces what the agent is currently
	// trying to do; helps the user see at-a-glance that an agent
	// is alive vs. stuck.
	LastVerb    string `json:"last_verb,omitempty"`
	// LastSpeech — most recent speech/shout/whisper text the
	// agent emitted. Limited to a sensible length in the renderer.
	LastSpeech  string `json:"last_speech,omitempty"`
}

type agentsListResp struct {
	Agents []agentInfo `json:"agents"`
}

func nowMs() int64 { return time.Now().UnixMilli() }

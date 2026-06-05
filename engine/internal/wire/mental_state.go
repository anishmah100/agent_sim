package wire

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anishmah100/agent_sim/engine/internal/historian"
)

// MentalStateHandler serves /api/v1/agent/<id>/mental_state — the
// Phase AGENT-A7 inspector reads this when the user clicks an entity.
//
// Response shape:
//
//	{
//	  "entity_id":                 "...",
//	  "capture_reasoning_enabled": bool,
//	  "dialogue":  [{tick, speaker, channel, text}],
//	  "mind":      {share_planner, top_goal, last_reflection, goal_stack_size},
//	  "traces":    [{tick, action_id, verb, reasoning}]
//	}
//
// `share_planner` (and the always-private `top_goal`/`last_reflection`)
// hang off agent registration in Wave 5 follow-up. Right now they're
// false / empty placeholders so the UI shows the gated state.
//
// Traces come from the historian's reasoning_traces ring buffer,
// filtered to this entity. capture_reasoning_enabled mirrors the
// engine-level flag so the frontend can show the right gating hint.
func MentalStateHandler(hist *historian.Historian, captureReasoning bool) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		// Cache off — the data is per-tick.
		rw.Header().Set("Cache-Control", "no-store")

		// Parse /api/v1/agent/<id>/mental_state — the URL path leaves
		// the id segment between two known prefixes/suffixes. The mux
		// registers /api/v1/agent/ as a prefix so this handler also
		// receives the /register POSTs; bounce anything that doesn't
		// end in /mental_state straight back to 404.
		if !strings.HasSuffix(r.URL.Path, "/mental_state") {
			http.NotFound(rw, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/")
		path = strings.TrimSuffix(path, "/mental_state")
		entityID := strings.Trim(path, "/")
		if entityID == "" {
			http.Error(rw, `{"error":"missing entity id"}`, http.StatusBadRequest)
			return
		}

		body := mentalStateResponse{
			EntityID:                entityID,
			CaptureReasoningEnabled: captureReasoning,
			Dialogue:                []dialogueLine{},
			Mind:                    mindSnapshot{ShareReasoning: false},
			Traces:                  []traceLine{},
		}
		if hist != nil {
			body.Traces = collectTraces(hist, entityID, 20)
		}
		enc := json.NewEncoder(rw)
		enc.SetIndent("", "  ")
		_ = enc.Encode(body)
	}
}

type mentalStateResponse struct {
	EntityID                string         `json:"entity_id"`
	CaptureReasoningEnabled bool           `json:"capture_reasoning_enabled"`
	Dialogue                []dialogueLine `json:"dialogue"`
	Mind                    mindSnapshot   `json:"mind"`
	Traces                  []traceLine    `json:"traces"`
}

type dialogueLine struct {
	Tick    uint64 `json:"tick"`
	Speaker string `json:"speaker"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

type mindSnapshot struct {
	SharePlanner    bool   `json:"share_planner"`
	TopGoal         string `json:"top_goal"`
	LastReflection  string `json:"last_reflection"`
	GoalStackSize   int    `json:"goal_stack_size"`
	ShareReasoning  bool   `json:"share_reasoning,omitempty"`
}

type traceLine struct {
	Tick      uint64 `json:"tick"`
	ActionID  string `json:"action_id"`
	Verb      string `json:"verb"`
	Reasoning string `json:"reasoning"`
}

// collectTraces pulls the most recent N reasoning records for an
// entity out of the historian's ring.
func collectTraces(hist *historian.Historian, entityID string, limit int) []traceLine {
	out := []traceLine{}
	if hist == nil {
		return out
	}
	for _, rec := range hist.Recent(0, 1024) {
		if rec.Kind != "ReasoningTrace" {
			continue
		}
		var rp historian.ReasoningTrace
		if err := json.Unmarshal(rec.Payload, &rp); err != nil {
			continue
		}
		if rp.EntityID != entityID {
			continue
		}
		out = append(out, traceLine{
			Tick:      rec.Tick,
			ActionID:  rp.ActionID,
			Verb:      rp.Verb,
			Reasoning: rp.Reasoning,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

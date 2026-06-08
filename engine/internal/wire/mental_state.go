package wire

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anishmah100/agent_sim/engine/internal/historian"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// SocialPeersReader — D19. Just the subset of *world.World the mental
// state handler needs to surface per-pair social counts AND per-entity
// vitals snapshot. Kept as an interface so tests can supply a fixture
// instead of a full world.
type SocialPeersReader interface {
	SocialPeersOf(entityID string) map[string]world.SocialCounts
	// VitalsOf returns hp/hunger/gold/inventory/equipped for the inspector.
	// Returns zero VitalsSnapshot when the entity is unknown.
	VitalsOf(entityID string) world.VitalsSnapshot
	// WitnessedBy returns the most recent things the entity perceived
	// (kills it saw, death screams it heard) for the Witnesses tab.
	WitnessedBy(entityID string, limit int) []world.WitnessRecord
}

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
func MentalStateHandler(hist *historian.Historian, captureReasoning bool, social SocialPeersReader) http.HandlerFunc {
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
			Peers:                   map[string]world.SocialCounts{},
			Witnesses:               []world.WitnessRecord{},
		}
		if social != nil {
			if peers := social.SocialPeersOf(entityID); len(peers) > 0 {
				body.Peers = peers
			}
			body.Vitals = social.VitalsOf(entityID)
			if ws := social.WitnessedBy(entityID, 20); len(ws) > 0 {
				body.Witnesses = ws
			}
		}
		if hist != nil {
			body.Traces = collectTraces(hist, entityID, 20)
			body.Dialogue = collectDialogue(hist, entityID, 20)
			body.Mind.LastReflection = collectLastReflection(hist, entityID)
			// D14 — populate per-slot fields from MentalNote records.
			// goal/plan/beliefs/emotion are independent latest-non-empty.
			slots := collectMentalSlots(hist, entityID)
			if v, ok := slots["goal"]; ok {
				body.Mind.TopGoal = v
			}
			if v, ok := slots["plan"]; ok {
				body.Mind.Plan = v
			}
			if v, ok := slots["beliefs"]; ok {
				body.Mind.Beliefs = v
			}
			if v, ok := slots["emotion"]; ok {
				body.Mind.Emotion = v
			}
		}
		enc := json.NewEncoder(rw)
		enc.SetIndent("", "  ")
		_ = enc.Encode(body)
	}
}

type mentalStateResponse struct {
	EntityID                string                        `json:"entity_id"`
	CaptureReasoningEnabled bool                          `json:"capture_reasoning_enabled"`
	Dialogue                []dialogueLine                `json:"dialogue"`
	Mind                    mindSnapshot                  `json:"mind"`
	Traces                  []traceLine                   `json:"traces"`
	// D19 — per-pair social counters for this entity. Empty map when
	// no interactions have been logged yet. Surfaced to the inspector's
	// Relationships tab.
	Peers map[string]world.SocialCounts `json:"peers"`
	// Live vitals snapshot: hp, hunger, gold, inventory, equipped.
	// Surfaced to the inspector's identity block + inventory display.
	Vitals world.VitalsSnapshot `json:"vitals"`
	// Witnesses — recent kills this agent saw + screams it heard,
	// newest first. Surfaced to the inspector's Witnesses tab.
	Witnesses []world.WitnessRecord `json:"witnesses"`
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
	// D14 — recommended slots from MentalNote. Each independently
	// holds the latest non-empty value across the historian ring.
	// Inspector renders these prominently at the top of the Mind
	// tab when populated.
	Plan    string `json:"plan,omitempty"`
	Beliefs string `json:"beliefs,omitempty"`
	Emotion string `json:"emotion,omitempty"`
}

type traceLine struct {
	Tick      uint64 `json:"tick"`
	ActionID  string `json:"action_id"`
	Verb      string `json:"verb"`
	Reasoning string `json:"reasoning"`
}

// collectDialogue scans the historian for Speech/Whisper events the
// agent emitted (Speaker == entityID) and returns the most recent N
// in newest-first order.
//
// Earlier version of the handler returned []. Now sources from the
// same ring buffer the smoke scorer uses.
func collectDialogue(hist *historian.Historian, entityID string, limit int) []dialogueLine {
	out := []dialogueLine{}
	if hist == nil {
		return out
	}
	for _, rec := range hist.Recent(0, 2048) {
		if rec.Kind != "Speech" && rec.Kind != "Whisper" {
			continue
		}
		var payload struct {
			Speaker string `json:"Speaker"`
			Text    string `json:"Text"`
			Mode    string `json:"Mode"`
			Target  string `json:"Target"`
		}
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			continue
		}
		if payload.Speaker != entityID {
			continue
		}
		channel := payload.Mode
		if channel == "" {
			if rec.Kind == "Whisper" {
				channel = "whisper"
			} else {
				channel = "speak"
			}
		}
		out = append(out, dialogueLine{
			Tick:    rec.Tick,
			Speaker: payload.Speaker,
			Channel: channel,
			Text:    payload.Text,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// collectLastReflection — most recent ReflectiveNote OR MentalNote
// for this entity. Falls through to ReflectiveNote for backwards
// compat. Empty when share_reasoning is off (the layered opt-in
// upstream prevents the historian from logging at all).
func collectLastReflection(hist *historian.Historian, entityID string) string {
	if hist == nil {
		return ""
	}
	for _, rec := range hist.Recent(0, 2048) {
		// D14 — prefer MentalNote when present; it's the new
		// architecture-agnostic shape.
		if rec.Kind == "MentalNote" {
			var p struct {
				EntityID string `json:"entity_id"`
				Text     string `json:"text"`
			}
			if err := json.Unmarshal(rec.Payload, &p); err == nil &&
				p.EntityID == entityID && p.Text != "" {
				return p.Text
			}
		}
		if rec.Kind != "ReflectiveNote" {
			continue
		}
		var p struct {
			EntityID string `json:"entity_id"`
			Note     string `json:"note"`
		}
		if err := json.Unmarshal(rec.Payload, &p); err != nil {
			continue
		}
		if p.EntityID != entityID {
			continue
		}
		return p.Note
	}
	return ""
}

// collectMentalSlots — D14. Most recent populated slots {goal, plan,
// beliefs, emotion} from this entity's MentalNote records. Each
// slot independently keeps the LATEST non-empty value across the
// ring. Empty map when no notes / opt-in off.
func collectMentalSlots(hist *historian.Historian, entityID string) map[string]string {
	out := map[string]string{}
	if hist == nil {
		return out
	}
	wanted := map[string]bool{
		"goal": true, "plan": true, "beliefs": true, "emotion": true,
	}
	for _, rec := range hist.Recent(0, 2048) {
		if rec.Kind != "MentalNote" {
			continue
		}
		var p struct {
			EntityID string            `json:"entity_id"`
			Slots    map[string]string `json:"slots"`
		}
		if err := json.Unmarshal(rec.Payload, &p); err != nil {
			continue
		}
		if p.EntityID != entityID || len(p.Slots) == 0 {
			continue
		}
		for k, v := range p.Slots {
			if !wanted[k] || v == "" {
				continue
			}
			if _, already := out[k]; !already {
				out[k] = v
			}
		}
	}
	return out
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

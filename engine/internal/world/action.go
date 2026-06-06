package world

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Action is the wire-level shape of an action submitted by an agent.
// We unmarshal into ActionEnvelope first, then dispatch by Verb.
type ActionEnvelope struct {
	ActionID         string          `json:"action_id"`
	InResponseToObs  uint64          `json:"in_response_to_obs,omitempty"`
	Verb             string          `json:"verb"`
	Priority         int             `json:"priority,omitempty"`
	// Reasoning is a free-text trace the agent's tactical brain
	// emits alongside the action. Captured only when the experiment's
	// capture_reasoning flag AND the agent's share_reasoning flag are
	// BOTH set — see docs/EXPERIMENT_SYSTEM_PLAN.md §8.
	Reasoning        string          `json:"reasoning,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

func (a *ActionEnvelope) UnmarshalJSON(data []byte) error {
	type alias ActionEnvelope
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*a = ActionEnvelope(aux)
	a.Raw = append([]byte(nil), data...)
	return nil
}

// ActionResult is what the engine returns on the engine-→agent WS
// channel after every action attempt.
type ActionResult struct {
	ActionID string `json:"action_id"`
	Verb     string `json:"verb"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

// Dispatch validates + executes an action for the given entity.
// Mutates world state; caller holds the lock.
func (w *World) Dispatch(e *Entity, env *ActionEnvelope) ActionResult {
	res := ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	if e.InsideBuilding != "" {
		// While inside a building, most "outside" verbs are nonsense
		// (move into a wall, attack someone you can't see). But there
		// are verbs an inside entity MUST be able to perform — most
		// critically `exit`, `interact{affordance=exit}`, and the
		// idle/social set so agents aren't bricks the moment they
		// step through a door. Without this allowlist the original
		// "always-reject-while-inside" rule trapped every agent that
		// successfully entered: their next action would be rejected
		// as "inside_building" forever, including exit.
		switch env.Verb {
		case "exit", "interact", "wait", "look_at", "speak",
			"shout", "whisper", "ponder", "drop", "equip":
			// fall through to normal dispatch — the verb's own
			// handler will decide whether the parameters make sense.
		default:
			res.Reason = "inside_building"
			return res
		}
	}
	// Scenario handlers take precedence — a scenario can override base
	// verb semantics (e.g. attack with HP rules) or implement entirely
	// scenario-custom verbs (trade, pay).
	if h := w.scenarioHandler(env.Verb); h != nil {
		return h(w, e, env)
	}
	switch env.Verb {
	case "move":
		var p struct {
			Target [2]int `json:"target"`
			Jog    bool   `json:"jog"`
		}
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		t := Tile{p.Target[0], p.Target[1]}
		if !w.IsWalkable(t) {
			res.Reason = "unreachable"
			return res
		}
		if !w.startMove(e, t) {
			res.Reason = "no_path"
			return res
		}
		res.Accepted = true
	case "speak":
		var p struct{ Text string `json:"text"` }
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		w.emitSpeech(e, "speech", p.Text, 3)
		res.Accepted = true
	case "shout":
		var p struct{ Text string `json:"text"` }
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		w.emitSpeech(e, "shout", p.Text, 15)
		res.Accepted = true
	case "whisper":
		var p struct {
			Target string `json:"target"`
			Text   string `json:"text"`
		}
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		target := w.entities[p.Target]
		if target == nil {
			res.Reason = "unknown_target"
			return res
		}
		if chebyshev(e.LogicalTile, target.LogicalTile) > 1 {
			res.Reason = "target_too_far"
			return res
		}
		w.emitWhisper(e, target, p.Text)
		// D19 — social ledger.
		if w.social != nil {
			w.social.Bump(e.EntityID, target.EntityID, SocialWhisper)
		}
		res.Accepted = true
	case "look_at":
		// look_at is a hint, not a state change. We just record it for
		// the social signal layer (future). Always accepted.
		res.Accepted = true
	case "wait":
		var p struct{ Ticks int `json:"ticks"` }
		_ = json.Unmarshal(env.Raw, &p)
		if p.Ticks <= 0 {
			p.Ticks = 60
		}
		e.CurrentAction = "wait"
		e.actionTicks = p.Ticks
		res.Accepted = true
	case "interact":
		var p struct {
			Target     string `json:"target"`
			Affordance string `json:"affordance"`
		}
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		// Affordances are scenario-defined; engine just routes to the
		// scenario handler when one's registered. For v1, "enter" on a
		// door warps the entity inside the building.
		if p.Affordance == "enter" {
			if strings.HasPrefix(p.Target, "bld:") {
				e.InsideBuilding = p.Target
				e.insideTicks = 240 + w.rng.IntN(360)
				// Fire EnteredBuilding so the historian sees it. The
				// property system emits this for entity-backed
				// buildings; this path covers decoration-backed
				// buildings (the `bld:` family that snapshot exposes
				// as visible doors) which never had an entity.
				if w.onBuildingEntered != nil {
					w.onBuildingEntered(e.EntityID, p.Target, w.tick)
				}
				res.Accepted = true
				return res
			}
		}
		// Also accept affordance="exit" from inside as a courtesy so
		// agents can be consistent: interact(target=<building>, affordance="exit").
		if p.Affordance == "exit" && e.InsideBuilding != "" {
			prev := e.InsideBuilding
			e.InsideBuilding = ""
			e.insideTicks = 0
			if w.onBuildingExited != nil {
				w.onBuildingExited(e.EntityID, prev, w.tick)
			}
			res.Accepted = true
			return res
		}
		res.Reason = "no_affordance_handler"
	case "pickup", "drop", "equip", "give":
		// Inventory verbs are handled by the inventory scenario layer.
		// Engine returns "scenario_required" if no scenario has bound
		// a handler.
		res.Reason = "scenario_required"
	case "attack", "defend", "heal":
		// Combat verbs: scenario-layer.
		res.Reason = "scenario_required"
	default:
		res.Reason = fmt.Sprintf("unknown_verb:%s", env.Verb)
	}
	return res
}

func chebyshev(a, b Tile) int {
	dx := a[0] - b[0]
	if dx < 0 {
		dx = -dx
	}
	dy := a[1] - b[1]
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

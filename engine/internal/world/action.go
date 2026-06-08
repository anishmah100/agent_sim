package world

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Action is the wire-level shape of an action submitted by an agent.
// We unmarshal into ActionEnvelope first, then dispatch by Verb.
type ActionEnvelope struct {
	ActionID        string `json:"action_id"`
	InResponseToObs uint64 `json:"in_response_to_obs,omitempty"`
	Verb            string `json:"verb"`
	Priority        int    `json:"priority,omitempty"`
	// Reasoning is a free-text trace the agent's tactical brain
	// emits alongside the action. Captured only when the experiment's
	// capture_reasoning flag AND the agent's share_reasoning flag are
	// BOTH set — see docs/EXPERIMENT_SYSTEM_PLAN.md §8.
	Reasoning string          `json:"reasoning,omitempty"`
	Raw       json.RawMessage `json:"-"`
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
	// Building entry for decoration-backed buildings. Eldoria's buildings
	// are decorations exposed to agents as visible doors
	// (object_id "door:bld:NNN:x,y"); they have no entity, so the property
	// system's entity-based `enter` handler rejects them as unknown_target.
	// Resolve such a target to its bld: id and enter via the same
	// InsideBuilding path interact{affordance=enter} uses. Falls through to
	// the scenario handler for genuine entity-backed buildings.
	if env.Verb == "enter" {
		if r, handled := w.tryEnterDecorationBuilding(e, env); handled {
			return r
		}
	}
	// Exit from a building interior (HeartGold model): the agent is on an
	// interior sub-map; warp it back to the overworld door tile. Interiors
	// carry no property system, so this is handled in core dispatch.
	if env.Verb == "exit" && w.hub != nil && strings.HasPrefix(w.MapID, "interior:") {
		return w.tryExitInterior(e, env)
	}
	// Scenario handlers take precedence — a scenario can override base
	// verb semantics (e.g. attack with HP rules) or implement entirely
	// scenario-custom verbs (trade, pay).
	if h := w.scenarioHandler(env.Verb); h != nil {
		return h(w, e, env)
	}
	switch env.Verb {
	case "step":
		// Single-tile compass step. The AGENT owns navigation (A* on its
		// known terrain); the engine just executes one committed tile.
		var p struct {
			Dir string `json:"dir"`
		}
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		d, ok := dirDelta(p.Dir)
		if !ok {
			res.Reason = "bad_direction"
			return res
		}
		next := Tile{e.LogicalTile[0] + d[0], e.LogicalTile[1] + d[1]}
		if !w.IsWalkable(next) {
			res.Reason = "blocked_by_terrain"
			return res
		}
		if !w.stepOneTile(e, next) {
			res.Reason = "blocked"
			return res
		}
		// AUDIT FIX (medium/[20]): track a per-entity step counter so
		// walk_distance quests (which read extras["steps"]) can complete —
		// nothing wrote this before, making that quest kind impossible.
		if sv, ok := e.Extras["steps"]; ok {
			if n, ok2 := sv.(int); ok2 {
				e.Extras["steps"] = n + 1
			} else {
				e.Extras["steps"] = 1
			}
		} else {
			if e.Extras == nil {
				e.Extras = map[string]any{}
			}
			e.Extras["steps"] = 1
		}
		res.Accepted = true
	case "speak":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		// Radius from rules.star (speak_radius), not a hardcoded 3 —
		// the tuning was declared (=8) but ignored, so speech carried
		// far less than intended and clustered agents (vision 12) often
		// couldn't hear each other.
		w.emitSpeech(e, "speech", p.Text, w.Rules.GetInt("speak_radius", 8))
		res.Accepted = true
	case "shout":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(env.Raw, &p); err != nil {
			res.Reason = "bad_params"
			return res
		}
		w.emitSpeech(e, "shout", p.Text, w.Rules.GetInt("shout_radius", 30))
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
		if chebyshev(e.LogicalTile, target.LogicalTile) > w.Rules.GetInt("whisper_radius", 2) {
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
		var p struct {
			Ticks int `json:"ticks"`
		}
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

// tryEnterDecorationBuilding handles `enter` for decoration-backed
// buildings (the bld:/door: family exposed as visible doors). Returns
// (result, true) when it took responsibility; (zero, false) when the
// target isn't a decoration building so the caller falls through to the
// scenario handler (entity-backed buildings).
func (w *World) tryEnterDecorationBuilding(e *Entity, env *ActionEnvelope) (ActionResult, bool) {
	res := ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		return res, false
	}
	bid := normalizeBuildingTarget(p.Target)
	if bid == "" {
		return res, false // not a decoration-building target
	}
	if e.InsideBuilding != "" {
		res.Reason = "already_inside"
		return res, true
	}
	hasBuilding, adjacent := false, false
	var doorTile Tile
	var doorRef buildingRef
	for tile, ref := range w.buildingDoors {
		if ref.Sprite != bid {
			continue
		}
		hasBuilding = true
		if chebyshev(e.LogicalTile, tile) <= 1 {
			adjacent = true
			doorTile = tile
			doorRef = ref
			break
		}
	}
	if !hasBuilding {
		res.Reason = "unknown_target"
		return res, true
	}
	if !adjacent {
		res.Reason = "target_too_far"
		return res, true
	}
	w.SetEntityAction(e.EntityID, "interact", 20)
	w.audibleAppend(AudibleEvent{
		EventID:   nextEventID(&w.eventSeq),
		Kind:      "sound",
		SoundKind: "building_enter",
		FromPos:   e.LogicalTile,
		Tick:      w.tick,
		radius:    8,
	})

	// HeartGold model: warp into a real interior sub-map when a hub is
	// present. Lazily generate the building's interior (keyed by its door
	// tile so each building instance is its own room), then queue the
	// cross-map move for after the tick (Warp can't run under this lock).
	if w.hub != nil {
		interiorID := InteriorMapID(bid, doorTile)
		iw := w.hub.Get(interiorID)
		if iw == nil {
			// Footprint dims aren't on buildingRef; use sensible defaults.
			gen, err := GenerateInterior(bid, doorTile, 4, 3)
			if err == nil {
				iw = gen
				w.hub.Add(iw)
			}
		}
		if iw != nil {
			e.interiorReturnMap = w.MapID
			e.interiorReturnTile = e.LogicalTile
			w.pendingWarps = append(w.pendingWarps, pendingWarp{
				EntityID: e.EntityID,
				ToMapID:  interiorID,
				Target:   iw.interiorEntrance(),
			})
			if w.onBuildingEntered != nil {
				w.onBuildingEntered(e.EntityID, bid, w.tick)
			}
			res.Accepted = true
			return res, true
		}
		_ = doorRef // (footprint lookup is a future refinement)
	}

	// Fallback (no hub, e.g. unit tests): the legacy phase-out flag.
	e.InsideBuilding = bid
	e.insideTicks = 240 + w.rng.IntN(360)
	if w.onBuildingEntered != nil {
		w.onBuildingEntered(e.EntityID, bid, w.tick)
	}
	res.Accepted = true
	return res, true
}

// tryExitInterior queues a warp back to the overworld door tile the agent
// entered from. Called from core Dispatch for `exit` on an interior map.
func (w *World) tryExitInterior(e *Entity, env *ActionEnvelope) ActionResult {
	res := ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	if e.interiorReturnMap == "" {
		res.Reason = "not_inside"
		return res
	}
	w.pendingWarps = append(w.pendingWarps, pendingWarp{
		EntityID:   e.EntityID,
		ToMapID:    e.interiorReturnMap,
		Target:     e.interiorReturnTile,
		unloadFrom: w.MapID,
	})
	// AUDIT/phase-5: emit ExitedBuilding so interior exits are paired with
	// their EnteredBuilding in the event log. This runs on the INTERIOR world
	// (no scenario/bus of its own), so route the hook through the overworld
	// (primary), whose bus the historian watches. The building sprite is
	// encoded in the interior map id.
	if w.hub != nil {
		if pw := w.hub.Primary(); pw != nil && pw.onBuildingExited != nil {
			sprite, _ := ParseInteriorMapID(w.MapID)
			pw.onBuildingExited(e.EntityID, sprite, w.tick)
		}
	}
	w.SetEntityAction(e.EntityID, "interact", 20)
	w.audibleAppend(AudibleEvent{
		EventID:   nextEventID(&w.eventSeq),
		Kind:      "sound",
		SoundKind: "building_enter",
		FromPos:   e.LogicalTile,
		Tick:      w.tick,
		radius:    4,
	})
	res.Accepted = true
	return res
}

// normalizeBuildingTarget maps an agent-supplied enter target to a bld:
// sprite id. Accepts "bld:NNN", "bld:NNN:x,y", or "door:bld:NNN:x,y".
// Returns "" if it doesn't look like a building target.
func normalizeBuildingTarget(t string) string {
	t = strings.TrimPrefix(t, "door:")
	if !strings.HasPrefix(t, "bld:") {
		return ""
	}
	parts := strings.Split(t, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return t
}

// Package verbalquests — composable VerbalQuests system.
//
// Per Q34 from session-2 decisions: quests in agent_sim are emergent
// from verbal contracts between agents, NOT engine-enforced. This
// system provides the bookkeeping ledger and the events that make
// contracts observable; it does NOT verify completion, enforce
// reciprocity, or auto-pay rewards. Agents can lie. The world is the
// world.
//
// Verbs:
//   - propose_task: actor proposes a contract to a nearby target.
//   - accept_task:  target accepts a previously-proposed contract.
//   - reject_task:  target rejects (or proposer cancels) a contract.
//   - complete_task: actor declares a contract done (just a marker).
//
// State on every spawned agent-like entity:
//   - contracts: []object — open contracts where this entity is the
//     proposer OR the target. Each has:
//       {id, proposer, target, terms, reward, status}
//     status in {"proposed","accepted","rejected","completed"}.
//
// The bookkeeping lives in extras.contracts on BOTH parties so both
// sides see the same record locally — no global ledger required.
package verbalquests

import (
	"encoding/json"
	"fmt"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
)

// === Events ===

type TaskProposed struct {
	ID, Proposer, Target string
	Terms                string
	Reward               string
}

func (TaskProposed) Kind() string { return "TaskProposed" }

type TaskAccepted struct {
	ID, Proposer, Target string
}

func (TaskAccepted) Kind() string { return "TaskAccepted" }

type TaskRejected struct {
	ID, Proposer, Target string
}

func (TaskRejected) Kind() string { return "TaskRejected" }

type TaskCompleted struct {
	ID, Proposer, Target string
}

func (TaskCompleted) Kind() string { return "TaskCompleted" }

var (
	_ eventbus.Event = TaskProposed{}
	_ eventbus.Event = TaskAccepted{}
	_ eventbus.Event = TaskRejected{}
	_ eventbus.Event = TaskCompleted{}
)

// === System ===

type System struct{}

func New() *System { return &System{} }

func (s *System) Name() string { return "verbalquests" }

func (s *System) RegisterWith(r syscore.Registry) {
	r.Verb("propose_task", s.handlePropose)
	r.Verb("accept_task", s.handleAccept)
	r.Verb("reject_task", s.handleReject)
	r.Verb("complete_task", s.handleComplete)
	r.OnEntitySpawn(s.seedSpawn)
	r.Manifest(s.manifest())
}

func (s *System) seedSpawn(w syscore.World, e syscore.Entity) {
	// Only agent-like archetypes carry contracts; items / buildings /
	// resources don't. We accept the default-empty fallback in the
	// helpers below rather than try to enumerate archetypes here.
	switch e.Archetype() {
	case "item", "building", "decoration", "tree", "rock":
		return
	}
	if _, ok := e.GetExtra("contracts"); !ok {
		e.SetExtra("contracts", []any{})
	}
}

// === Verb handlers ===

func (s *System) handlePropose(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		Target string `json:"target"`
		Terms  string `json:"terms"`
		Reward string `json:"reward"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil {
		res.Reason = "bad_params"
		return res
	}
	target := w.EntityByID(p.Target)
	if target == nil {
		res.Reason = "unknown_target"
		return res
	}
	if target.ID() == e.ID() {
		res.Reason = "self_target"
		return res
	}
	if p.Terms == "" {
		res.Reason = "empty_terms"
		return res
	}
	id := fmt.Sprintf("ct_%s_%s_%d", e.ID(), p.Target, w.Tick())
	contract := map[string]any{
		"id":       id,
		"proposer": e.ID(),
		"target":   p.Target,
		"terms":    p.Terms,
		"reward":   p.Reward,
		"status":   "proposed",
		"tick":     w.Tick(),
	}
	appendContract(w, e.ID(), contract)
	appendContract(w, p.Target, contract)
	w.QueueEvent(TaskProposed{ID: id, Proposer: e.ID(), Target: p.Target, Terms: p.Terms, Reward: p.Reward})
	res.Accepted = true
	return res
}

func (s *System) handleAccept(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	return s.transitionStatus(w, e, env, "proposed", "accepted", "TaskAccepted")
}

func (s *System) handleReject(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	return s.transitionStatus(w, e, env, "proposed", "rejected", "TaskRejected")
}

func (s *System) handleComplete(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope) syscore.ActionResult {
	return s.transitionStatus(w, e, env, "accepted", "completed", "TaskCompleted")
}

func (s *System) transitionStatus(w syscore.World, e syscore.Entity, env *syscore.ActionEnvelope, fromStatus, toStatus, eventKind string) syscore.ActionResult {
	res := syscore.ActionResult{ActionID: env.ActionID, Verb: env.Verb}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Raw, &p); err != nil || p.ID == "" {
		res.Reason = "bad_params"
		return res
	}
	contracts := readContracts(e)
	idx := indexContract(contracts, p.ID)
	if idx < 0 {
		res.Reason = "unknown_contract"
		return res
	}
	c := contracts[idx]
	if c["status"] != fromStatus {
		res.Reason = "bad_status"
		return res
	}
	proposer, _ := c["proposer"].(string)
	target, _ := c["target"].(string)

	// Authorization: accept/reject must come from target; complete must
	// come from proposer (the proposer declares the task done from their
	// PoV — Q34's "no engine enforcement" means whether it really is done
	// is for the agent to decide).
	authorized := false
	switch eventKind {
	case "TaskAccepted", "TaskRejected":
		authorized = e.ID() == target
	case "TaskCompleted":
		authorized = e.ID() == proposer
	}
	if !authorized {
		res.Reason = "not_authorized"
		return res
	}

	// Apply the same status flip on both parties' copies.
	mutateContract(w, proposer, p.ID, func(c map[string]any) { c["status"] = toStatus })
	mutateContract(w, target, p.ID, func(c map[string]any) { c["status"] = toStatus })

	switch eventKind {
	case "TaskAccepted":
		w.QueueEvent(TaskAccepted{ID: p.ID, Proposer: proposer, Target: target})
	case "TaskRejected":
		w.QueueEvent(TaskRejected{ID: p.ID, Proposer: proposer, Target: target})
	case "TaskCompleted":
		w.QueueEvent(TaskCompleted{ID: p.ID, Proposer: proposer, Target: target})
	}
	res.Accepted = true
	return res
}

// === Manifest ===

func (s *System) manifest() manifest.SystemDeclaration {
	return manifest.SystemDeclaration{
		Name:        "verbalquests",
		Description: "Verbal contracts between agents. Engine emits markers + maintains the contract ledger on both parties' extras. Does NOT enforce completion or pay rewards — those are emergent. Agents can lie.",
		Verbs: []manifest.VerbDeclaration{
			{Verb: "propose_task",
				Description:      "Propose a verbal contract to a known entity. Records the contract on both parties' extras.contracts.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"terms":{"type":"string"},"reward":{"type":"string"}},"required":["target","terms"]}`),
				RejectionReasons: []string{"bad_params", "unknown_target", "self_target", "empty_terms"},
				EmitsEvents:      []string{"TaskProposed"},
			},
			{Verb: "accept_task",
				Description:      "Accept a proposed contract addressed to you.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
				RejectionReasons: []string{"bad_params", "unknown_contract", "bad_status", "not_authorized"},
				EmitsEvents:      []string{"TaskAccepted"},
			},
			{Verb: "reject_task",
				Description:      "Reject a proposed contract addressed to you.",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
				RejectionReasons: []string{"bad_params", "unknown_contract", "bad_status", "not_authorized"},
				EmitsEvents:      []string{"TaskRejected"},
			},
			{Verb: "complete_task",
				Description:      "Mark an accepted contract as complete (from the proposer's PoV — no engine verification).",
				ParamsSchema:     json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
				RejectionReasons: []string{"bad_params", "unknown_contract", "bad_status", "not_authorized"},
				EmitsEvents:      []string{"TaskCompleted"},
			},
		},
		StateFields: []manifest.StateFieldDecl{
			{Key: "contracts", Type: "list", Owner: "entity.extras", PublicAtAnyDistance: false, Meaning: "ledger of open verbal contracts where this entity is proposer or target; private to the owner"},
		},
	}
}

// === helpers ===

func readContracts(e syscore.Entity) []map[string]any {
	v, ok := e.GetExtra("contracts")
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func indexContract(contracts []map[string]any, id string) int {
	for i, c := range contracts {
		if cid, _ := c["id"].(string); cid == id {
			return i
		}
	}
	return -1
}

func appendContract(w syscore.World, entityID string, contract map[string]any) {
	w.MutateEntity(entityID, func(real syscore.Entity) {
		cur := readContracts(real)
		// Materialize as []any so JSON round-trips match the seeded shape.
		out := make([]any, 0, len(cur)+1)
		for _, c := range cur {
			out = append(out, c)
		}
		out = append(out, contract)
		real.SetExtra("contracts", out)
	})
}

func mutateContract(w syscore.World, entityID, id string, mutate func(map[string]any)) {
	w.MutateEntity(entityID, func(real syscore.Entity) {
		cur := readContracts(real)
		idx := indexContract(cur, id)
		if idx < 0 {
			return
		}
		mutate(cur[idx])
		out := make([]any, 0, len(cur))
		for _, c := range cur {
			out = append(out, c)
		}
		real.SetExtra("contracts", out)
	})
}

package world

import "testing"

// A contract proposed to an agent is stored in entity.Extras["contracts"]
// by the verbalquests system. For the agent to ACCEPT it, the contract
// must reach the agent's observation. This test pins that the
// observation's SelfState.Extras carries the contracts list, so an
// LLM/bot can see a pending contract and accept_task it.
//
// The first real P7 experiment showed 9 contracts proposed, 0 accepted
// — because nothing surfaced pending contracts to the target. This is
// the regression guard for the fix.
func TestObservation_SelfCarriesContracts(t *testing.T) {
	w := loadTestWorld(t)
	target := w.entities["b"]
	if target == nil {
		t.Fatal("fixture missing entity b")
	}
	if target.Extras == nil {
		target.Extras = map[string]any{}
	}
	// Simulate verbalquests appending a proposed contract to b.
	target.Extras["contracts"] = []any{
		map[string]any{
			"id":       "ct_a_b_1",
			"proposer": "a",
			"target":   "b",
			"terms":    "bring me 3 apples",
			"reward":   "20 gold",
			"status":   "proposed",
		},
	}

	obs := w.BuildObservationFor("b", 1, nil)
	if obs == nil {
		t.Fatal("nil observation for b")
	}
	raw, ok := obs.Self.Extras["contracts"]
	if !ok {
		t.Fatalf("obs.Self.Extras has no 'contracts' key; keys=%v",
			keysOf(obs.Self.Extras))
	}
	list, ok := raw.([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("contracts not a 1-element list: %#v", raw)
	}
	c, _ := list[0].(map[string]any)
	if c["id"] != "ct_a_b_1" || c["status"] != "proposed" {
		t.Fatalf("contract content wrong: %#v", c)
	}
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

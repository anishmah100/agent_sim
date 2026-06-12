package wire

import "testing"

// The reaper must evict ONLY registrations that never connected and have
// aged past the grace window — keeping within-grace and connected ones.
// Victims here are not auto-spawned (remove=false), so no world is needed.
func TestReapStaleRegistrations(t *testing.T) {
	h := &AgentHub{
		registry: map[string]*agentRecord{},
		live:     map[string]*agentConn{},
	}
	now := nowMs()
	h.registry["stale"] = &agentRecord{AgentID: "a1", Secret: "stale", RegisteredAt: now - 120_000, ConnectedAt: 0}
	h.registry["recent"] = &agentRecord{AgentID: "a2", Secret: "recent", RegisteredAt: now - 1_000, ConnectedAt: 0}
	h.registry["connected"] = &agentRecord{AgentID: "a3", Secret: "connected", RegisteredAt: now - 120_000, ConnectedAt: now - 100_000}

	h.reapStaleRegistrations(60_000)

	if _, ok := h.registry["stale"]; ok {
		t.Error("stale never-connected registration should have been reaped")
	}
	if _, ok := h.registry["recent"]; !ok {
		t.Error("within-grace registration must be kept (may still connect)")
	}
	if _, ok := h.registry["connected"]; !ok {
		t.Error("connected registration must be kept (disconnect path owns its cleanup)")
	}
}

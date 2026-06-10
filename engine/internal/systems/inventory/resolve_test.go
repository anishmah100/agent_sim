package inventory

import "testing"

// resolveItemRef must accept the canonical inventory id, the pre-pickup
// ground-entity id (the "#<seq>" suffix), and the bare kind — pickup
// rewrites ids, and agents reference items by what they saw.
func TestResolveItemRef(t *testing.T) {
	inv := []string{
		"item:sword_short#spawn_10",
		"item:apple#spawn_42",
		"item:apple#spawn_77",
	}
	cases := []struct {
		ref  string
		want int
	}{
		{"item:sword_short#spawn_10", 0}, // canonical id
		{"spawn_10", 0},                  // ground-entity id the agent saw
		{"spawn_42", 1},
		{"sword_short", 0},      // bare kind
		{"item:sword_short", 0}, // kind with prefix
		{"apple", 1},            // ambiguous kind -> first match
		{"spawn_99", -1},        // unknown
		{"", -1},
	}
	for _, c := range cases {
		if got := resolveItemRef(inv, c.ref); got != c.want {
			t.Errorf("resolveItemRef(%q) = %d, want %d", c.ref, got, c.want)
		}
	}
}

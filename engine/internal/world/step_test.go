package world

import "testing"

func TestDirDelta(t *testing.T) {
	cases := map[string]Tile{
		"N": {0, -1}, "S": {0, 1}, "E": {1, 0}, "W": {-1, 0},
		"north": {0, -1}, "EAST": {1, 0},
	}
	for dir, want := range cases {
		got, ok := dirDelta(dir)
		if !ok || got != want {
			t.Fatalf("dirDelta(%q)=%v,%v want %v", dir, got, ok, want)
		}
	}
	if _, ok := dirDelta("X"); ok {
		t.Fatal("bad direction should not be ok")
	}
}

func TestDispatch_StepMovesOneTile(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	start := a.LogicalTile // (1,1)
	env := &ActionEnvelope{ActionID: "s1", Verb: "step",
		Raw: []byte(`{"verb":"step","dir":"E"}`)}
	res := w.Dispatch(a, env)
	if !res.Accepted {
		t.Fatalf("step E should be accepted; reason=%q", res.Reason)
	}
	want := Tile{start[0] + 1, start[1]}
	if a.LogicalTile != want {
		t.Fatalf("after step E pos=%v want %v", a.LogicalTile, want)
	}
	if a.Facing != FacingE {
		t.Fatalf("facing=%v want E", a.Facing)
	}
}

func TestDispatch_StepOffMapBlocked(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	a.LogicalTile = Tile{0, 0}
	a.WalkProgress = 1
	env := &ActionEnvelope{ActionID: "s2", Verb: "step",
		Raw: []byte(`{"verb":"step","dir":"W"}`)}
	res := w.Dispatch(a, env)
	if res.Accepted {
		t.Fatal("step off the map should be rejected")
	}
	if res.Reason != "blocked_by_terrain" {
		t.Fatalf("reason=%q want blocked_by_terrain", res.Reason)
	}
}

func TestDispatch_StepBadDirection(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	env := &ActionEnvelope{ActionID: "s3", Verb: "step",
		Raw: []byte(`{"verb":"step","dir":"X"}`)}
	res := w.Dispatch(a, env)
	if res.Accepted || res.Reason != "bad_direction" {
		t.Fatalf("bad dir: accepted=%v reason=%q", res.Accepted, res.Reason)
	}
}

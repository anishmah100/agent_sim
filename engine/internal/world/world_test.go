package world

import (
	"os"
	"path/filepath"
	"testing"
)

// Tiny test world JSON. 10×6 grass with a single wall row and two
// entities. Used by several tests below.
const testWorldJSON = `{
  "map_id": "test_world",
  "width_tiles": 10,
  "height_tiles": 6,
  "tiles_legend": {".":"grass","#":"wall"},
  "tiles": [
    "..........",
    "..........",
    "..####....",
    "..........",
    "..........",
    ".........."
  ],
  "entities": [
    {"entity_id":"a","archetype":"trainer","pos":[1,1],"facing":"S","display_name":"A"},
    {"entity_id":"b","archetype":"trainer","pos":[8,1],"facing":"S","display_name":"B"}
  ]
}`

func writeTestWorld(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.json")
	if err := os.WriteFile(p, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func loadTestWorld(t *testing.T) *World {
	t.Helper()
	p := writeTestWorld(t)
	w, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return w
}

func TestLoad_Basic(t *testing.T) {
	w := loadTestWorld(t)
	if w.WidthTiles != 10 || w.HeightTiles != 6 {
		t.Fatalf("dims: got %dx%d", w.WidthTiles, w.HeightTiles)
	}
	if !w.IsWalkable(Tile{0, 0}) {
		t.Fatal("0,0 should be walkable (grass)")
	}
	if w.IsWalkable(Tile{2, 2}) {
		t.Fatal("2,2 should NOT be walkable (wall)")
	}
}

func TestLOS_WallBlocks(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	b := w.entities["b"]
	// A at (1,1), B at (8,1). Wall row at y=2. Same row, wall not on
	// the line of sight, so they should see each other (long range).
	if !w.lineOfSight(a.LogicalTile, b.LogicalTile) {
		t.Fatal("clear horizontal line should have LOS")
	}
	// Through-the-wall: 1,1 → 5,5. The straight line crosses y=2 in the
	// wall columns. Should be blocked.
	if w.lineOfSight(Tile{1, 1}, Tile{5, 5}) {
		t.Fatal("line through wall should be blocked")
	}
}

func TestSeesEntity_VisionRadius(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	b := w.entities["b"]
	// At default radius (12) and chebyshev distance 7 they see each other.
	if !w.SeesEntity(a, b, VisionRadius) {
		t.Fatal("a should see b within VisionRadius")
	}
	// At radius 3 they don't.
	if w.SeesEntity(a, b, 3) {
		t.Fatal("a should NOT see b at radius 3")
	}
}

func TestHearing_SpeakLocal(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	b := w.entities["b"]
	// Speak radius is 3. A and B are 7 apart so B does not hear.
	w.emitSpeech(a, "speech", "hi", 3)
	audB := w.VisibleAudible(b, 0)
	if len(audB) != 0 {
		t.Fatalf("B should not hear A's local speak; got %v", audB)
	}
	// Shout radius 15 — B hears.
	w.emitSpeech(a, "shout", "HEY", 15)
	audB = w.VisibleAudible(b, 0)
	found := false
	for _, e := range audB {
		if e.Text == "HEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("B should hear A's shout")
	}
}

func TestDispatch_Move(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	env := &ActionEnvelope{
		ActionID: "act_1",
		Verb:     "move",
		Raw:      []byte(`{"verb":"move","target":[3,1]}`),
	}
	res := w.Dispatch(a, env)
	if !res.Accepted {
		t.Fatalf("move should be accepted; got reason=%q", res.Reason)
	}
}

func TestDispatch_MoveIntoWallRejected(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	env := &ActionEnvelope{
		ActionID: "act_2",
		Verb:     "move",
		Raw:      []byte(`{"verb":"move","target":[2,2]}`),
	}
	res := w.Dispatch(a, env)
	if res.Accepted {
		t.Fatal("move into wall should be rejected")
	}
	if res.Reason != "unreachable" {
		t.Fatalf("unexpected reason %q", res.Reason)
	}
}

func TestObservationBuilder(t *testing.T) {
	w := loadTestWorld(t)
	obs := w.BuildObservationFor("a", 1, nil)
	if obs == nil {
		t.Fatal("nil observation")
	}
	if obs.Self.EntityID != "a" {
		t.Fatalf("self mismatch: %s", obs.Self.EntityID)
	}
	// B should be in visible_entities (clear LOS at distance 7).
	found := false
	for _, ve := range obs.VisibleEntities {
		if ve.EntityID == "b" {
			found = true
		}
	}
	if !found {
		t.Fatal("A's observation should include B")
	}
}
